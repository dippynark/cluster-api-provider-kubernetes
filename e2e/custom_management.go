package e2e

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/pkg/errors"

	"github.com/dippynark/cluster-api-provider-kubernetes/e2e/framework/management/kind"
	yaml "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
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
	fmt.Print(f.Name())
	//defer os.RemoveAll(f.Name())
	if _, err := f.Write(b); err != nil {
		return nil, errors.WithStack(err)
	}
	cluster, err := kind.NewClusterWithConfig(ctx, name, f.Name(), scheme, images...)
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

	restConfig, err := clientcmd.BuildConfigFromFlags(master, f.Name())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return c.ClientFromRestConfig(restConfig)
}
