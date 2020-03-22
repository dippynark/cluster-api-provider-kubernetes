package e2e

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/pkg/errors"

	yaml "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/cluster-api/test/framework/exec"
	"sigs.k8s.io/cluster-api/test/framework/management/kind"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kindv1 "sigs.k8s.io/kind/pkg/apis/config/v1alpha3"
)

// Shells out to `docker`, `kind`, `kubectl`

// CAPKCluster wraps a Cluster and has custom logic for GetWorkloadClient and setup.
type CAPKCluster struct {
	*kind.Cluster
}

func NewClusterForCAPK(ctx context.Context, name string, scheme *runtime.Scheme, images ...string) (*CAPKCluster, error) {
	config := kindv1.Cluster{
		TypeMeta: kindv1.TypeMeta{
			APIVersion: "kind.sigs.k8s.io/v1alpha3",
			Kind:       "Cluster",
		},
		Nodes: []kindv1.Node{
			{
				Role: kindv1.ControlPlaneRole,
				ExtraPortMappings: []kindv1.PortMapping{
					{
						ContainerPort: 30000,
						HostPort:      30000,
						ListenAddress: "127.0.0.1",
					},
				},
			},
		},
	}

	b, err := yaml.Marshal(config)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	f, err := ioutil.TempFile("", "capi-test-framework")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer os.RemoveAll(f.Name())
	if _, err := f.Write(b); err != nil {
		return nil, errors.WithStack(err)
	}
	cluster, err := newClusterWithConfigCopy(ctx, name, f.Name(), scheme, images...)
	if err != nil {
		return nil, err
	}
	return &CAPKCluster{
		cluster,
	}, nil
}

// GetWorkloadClient uses some special logic for darwin architecture due to Docker for Mac limitations.
func (c *CAPKCluster) GetWorkloadClient(ctx context.Context, namespace, name string) (client.Client, error) {
	mgmtClient, err := c.GetClient()
	if err != nil {
		return nil, err
	}

	// Modify lb service to expose apiserver
	service := &corev1.Service{}
	key := client.ObjectKey{
		Name:      fmt.Sprintf("%s-lb", name),
		Namespace: namespace,
	}
	if err := mgmtClient.Get(ctx, key, service); err != nil {
		return nil, err
	}
	service.Spec.Type = corev1.ServiceTypeNodePort
	// TODO: check length
	service.Spec.Ports[0].NodePort = 30000
	if err := mgmtClient.Update(ctx, service); err != nil {
		return nil, err
	}

	config := &corev1.Secret{}
	key = client.ObjectKey{
		Name:      fmt.Sprintf("%s-kubeconfig", name),
		Namespace: namespace,
	}
	if err := mgmtClient.Get(ctx, key, config); err != nil {
		return nil, err
	}

	f, err := ioutil.TempFile("", "worker-kubeconfig")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if _, err := f.Write(config.Data["value"]); err != nil {
		return nil, errors.WithStack(err)
	}
	c.WorkloadClusterKubeconfigs[namespace+"-"+name] = f.Name()

	masterURL := &url.URL{
		Scheme: "https",
		Host:   "127.0.0.1:30000",
	}
	master := masterURL.String()

	// Wait for connectivity
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{Transport: transport, Timeout: 1 * time.Second}
	for {
		res, err := httpClient.Get("https://127.0.0.1:30000/healthz")
		if err != nil || res.StatusCode != 200 {
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}

	workloadRestConfig, err := clientcmd.BuildConfigFromFlags(master, f.Name())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return c.ClientFromRestConfig(workloadRestConfig)
}

// NewClusterWithConfig creates a kind cluster using a kind-config file.
func newClusterWithConfigCopy(ctx context.Context, name, configFile string, scheme *runtime.Scheme, images ...string) (*kind.Cluster, error) {
	cmd := exec.NewCommand(
		exec.WithCommand("kind"),
		exec.WithArgs("create", "cluster", "--name", name, "--config", configFile),
	)
	return create(ctx, cmd, name, scheme, images...)
}

func create(ctx context.Context, cmd *exec.Command, name string, scheme *runtime.Scheme, images ...string) (*kind.Cluster, error) {
	stdout, stderr, err := cmd.Run(ctx)
	if err != nil {
		fmt.Println(string(stdout))
		fmt.Println(string(stderr))
		return nil, err
	}
	kubeconfig, err := getKubeconfig(ctx, name)
	if err != nil {
		return nil, err
	}
	f, err := ioutil.TempFile("", "management-kubeconfig")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if _, err := f.Write(kubeconfig); err != nil {
		return nil, errors.WithStack(err)
	}

	c := &kind.Cluster{
		Name:                       name,
		KubeconfigPath:             f.Name(),
		Scheme:                     scheme,
		WorkloadClusterKubeconfigs: make(map[string]string),
	}
	for _, image := range images {
		fmt.Printf("Looking for image %q locally to load to the management cluster\n", image)
		if !c.ImageExists(ctx, image) {
			fmt.Printf("Did not find image %q locally, not loading it to the management cluster\n", image)
			continue
		}
		fmt.Printf("Loading image %q on to the management cluster\n", image)
		if err := c.LoadImage(ctx, image); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func getKubeconfig(ctx context.Context, name string) ([]byte, error) {
	getPathCmd := exec.NewCommand(
		exec.WithCommand("kind"),
		exec.WithArgs("get", "kubeconfig", "--name", name),
	)
	stdout, stderr, err := getPathCmd.Run(ctx)
	if err != nil {
		fmt.Println(string(stderr))
		return nil, err
	}
	return bytes.TrimSpace(stdout), nil
}
