package e2e

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeadmv1beta1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/types/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	capkv1 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha3"
	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
)

const (
	defaultVersion = "v1.17.0"
)

var _ = Describe("Kubernetes", func() {
	Describe("Cluster Creation", func() {
		var (
			namespace  string
			client     ctrlclient.Client
			clusterGen = &ClusterGenerator{}
			cluster    *capiv1.Cluster
		)
		SetDefaultEventuallyTimeout(5 * time.Minute)
		SetDefaultEventuallyPollingInterval(10 * time.Second)

		BeforeEach(func() {
			namespace = "default"
		})

		AfterEach(func() {
			//ensureDockerArtifactsDeleted(input)
		})

		Describe("One control-plane node and one worker node", func() {
			It("should deploy successfully", func() {
				controlPlaneReplias := 1
				workerReplicas := 1
				var (
					kubernetesCluster                     *capkv1.KubernetesCluster
					kubeadmControlPlane                   *controlplanev1.KubeadmControlPlane
					controlPlaneKubernetesMachineTemplate *capkv1.KubernetesMachineTemplate
					workerKubernetesMachineTemplate       *capkv1.KubernetesMachineTemplate
					machineDeployment                     *capiv1.MachineDeployment
					bootstrapTemplate                     *cabpkv1.KubeadmConfigTemplate
					err                                   error
				)

				cluster, kubernetesCluster, kubeadmControlPlane, controlPlaneKubernetesMachineTemplate = clusterGen.GenerateCluster(namespace, int32(controlPlaneReplias))
				machineDeployment, workerKubernetesMachineTemplate, bootstrapTemplate = GenerateMachineDeployment(cluster, int32(workerReplicas))

				// Set up the client to the management cluster
				client, err = mgmt.GetClient()
				Expect(err).NotTo(HaveOccurred())

				// Set up the cluster object
				createClusterInput := framework.CreateClusterInput{
					Creator:      client,
					Cluster:      cluster,
					InfraCluster: kubernetesCluster,
				}
				framework.CreateCluster(ctx, createClusterInput)

				// Set up the KubeadmControlPlane
				createKubeadmControlPlaneInput := framework.CreateKubeadmControlPlaneInput{
					Creator:         client,
					ControlPlane:    kubeadmControlPlane,
					MachineTemplate: controlPlaneKubernetesMachineTemplate,
				}
				framework.CreateKubeadmControlPlane(ctx, createKubeadmControlPlaneInput)

				// Wait for the cluster to provision.
				waitForClusterToProvisionInput := framework.WaitForClusterToProvisionInput{
					Getter:  client,
					Cluster: cluster,
				}
				framework.WaitForClusterToProvision(ctx, waitForClusterToProvisionInput)

				// Wait for control plane nodes to be ready
				waitForKubeadmControlPlaneMachinesToExistInput := framework.WaitForKubeadmControlPlaneMachinesToExistInput{
					Lister:       client,
					Cluster:      cluster,
					ControlPlane: kubeadmControlPlane,
				}
				framework.WaitForKubeadmControlPlaneMachinesToExist(ctx, waitForKubeadmControlPlaneMachinesToExistInput)

				// Install a networking solution on the workload cluster
				workloadClient, err := mgmt.GetWorkloadClient(ctx, cluster.Namespace, cluster.Name)
				Expect(err).ToNot(HaveOccurred())
				applyYAMLURLInput := framework.ApplyYAMLURLInput{
					Client:        workloadClient,
					HTTPGetter:    http.DefaultClient,
					NetworkingURL: "https://docs.projectcalico.org/manifests/calico.yaml",
					Scheme:        mgmt.Scheme,
				}
				framework.ApplyYAMLURL(ctx, applyYAMLURLInput)

				// Wait for the controlplane nodes to exist
				assertKubeadmControlPlaneNodesExistInput := framework.WaitForKubeadmControlPlaneMachinesToExistInput{
					Lister:       client,
					Cluster:      cluster,
					ControlPlane: kubeadmControlPlane,
				}
				framework.WaitForKubeadmControlPlaneMachinesToExist(ctx, assertKubeadmControlPlaneNodesExistInput, "10m")

				// Create the workload nodes
				createMachineDeploymentinput := framework.CreateMachineDeploymentInput{
					Creator:                 client,
					MachineDeployment:       machineDeployment,
					BootstrapConfigTemplate: bootstrapTemplate,
					InfraMachineTemplate:    workerKubernetesMachineTemplate,
				}
				framework.CreateMachineDeployment(ctx, createMachineDeploymentinput)

				// Wait for the workload nodes to exist
				waitForMachineDeploymentNodesToExistInput := framework.WaitForMachineDeploymentNodesToExistInput{
					Lister:            client,
					Cluster:           cluster,
					MachineDeployment: machineDeployment,
				}
				framework.WaitForMachineDeploymentNodesToExist(ctx, waitForMachineDeploymentNodesToExistInput)

				// Wait for the control plane to be ready
				waitForControlPlaneToBeReadyInput := framework.WaitForControlPlaneToBeReadyInput{
					Getter:       client,
					ControlPlane: kubeadmControlPlane,
				}
				framework.WaitForControlPlaneToBeReady(ctx, waitForControlPlaneToBeReadyInput)
			})
		})
	})
})

type ClusterGenerator struct {
	counter int
}

func (c *ClusterGenerator) GenerateCluster(namespace string, replicas int32) (*capiv1.Cluster, *capkv1.KubernetesCluster, *controlplanev1.KubeadmControlPlane, *capkv1.KubernetesMachineTemplate) {
	generatedName := fmt.Sprintf("test-%d", c.counter)
	c.counter++
	version := defaultVersion

	kubernetesCluster := &capkv1.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      generatedName,
		},
		Spec: capkv1.KubernetesClusterSpec{
			ControlPlaneServiceType: corev1.ServiceTypeClusterIP,
		},
	}

	kubernetesMachineTemplate := &capkv1.KubernetesMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      fmt.Sprintf("%s-control-plane", generatedName),
		},
		Spec: capkv1.KubernetesMachineTemplateSpec{
			Template: capkv1.KubernetesMachineTemplateResource{
				Spec: capkv1.KubernetesMachineSpec{
					PodSpec: corev1.PodSpec{
						// Necessary to avoid API errors
						// TODO: avoid needing to set an empty array here
						Containers: []corev1.Container{},
					},
				},
			},
		},
	}

	kubeadmControlPlane := &controlplanev1.KubeadmControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      generatedName,
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Replicas: &replicas,
			Version:  version,
			InfrastructureTemplate: corev1.ObjectReference{
				Kind:       framework.TypeToKind(kubernetesMachineTemplate),
				Namespace:  kubernetesMachineTemplate.GetNamespace(),
				Name:       kubernetesMachineTemplate.GetName(),
				APIVersion: capkv1.GroupVersion.String(),
			},
			KubeadmConfigSpec: cabpkv1.KubeadmConfigSpec{
				ClusterConfiguration: &kubeadmv1beta1.ClusterConfiguration{
					APIServer: kubeadmv1beta1.APIServer{
						// For NodePort connections
						CertSANs: []string{"127.0.0.1"},
					},
					ControllerManager: kubeadmv1beta1.ControlPlaneComponent{
						ExtraArgs: map[string]string{
							"enable-hostpath-provisioner": "true",
						},
					},
				},
				InitConfiguration: &kubeadmv1beta1.InitConfiguration{
					NodeRegistration: kubeadmv1beta1.NodeRegistrationOptions{
						KubeletExtraArgs: map[string]string{
							"eviction-hard":            "nodefs.available<0%,nodefs.inodesFree<0%,imagefs.available<0%",
							"cgroups-per-qos":          "false",
							"enforce-node-allocatable": "",
						},
					},
				},
				JoinConfiguration: &kubeadmv1beta1.JoinConfiguration{
					NodeRegistration: kubeadmv1beta1.NodeRegistrationOptions{
						KubeletExtraArgs: map[string]string{
							"eviction-hard":            "nodefs.available<0%,nodefs.inodesFree<0%,imagefs.available<0%",
							"cgroups-per-qos":          "false",
							"enforce-node-allocatable": "",
						},
					},
				},
			},
		},
	}

	cluster := &capiv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      generatedName,
		},
		Spec: capiv1.ClusterSpec{
			ClusterNetwork: &capiv1.ClusterNetwork{
				Pods: &capiv1.NetworkRanges{CIDRBlocks: []string{"192.168.0.0/16"}},
			},
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: capkv1.GroupVersion.String(),
				Kind:       framework.TypeToKind(kubernetesCluster),
				Namespace:  kubernetesCluster.GetNamespace(),
				Name:       kubernetesCluster.GetName(),
			},
			ControlPlaneRef: &corev1.ObjectReference{
				APIVersion: controlplanev1.GroupVersion.String(),
				Kind:       framework.TypeToKind(kubeadmControlPlane),
				Namespace:  kubeadmControlPlane.GetNamespace(),
				Name:       kubeadmControlPlane.GetName(),
			},
		},
	}

	return cluster, kubernetesCluster, kubeadmControlPlane, kubernetesMachineTemplate
}

func GenerateMachineDeployment(cluster *capiv1.Cluster, replicas int32) (*capiv1.MachineDeployment, *capkv1.KubernetesMachineTemplate, *cabpkv1.KubeadmConfigTemplate) {
	namespace := cluster.GetNamespace()
	generatedName := cluster.GetName()
	version := defaultVersion

	kubernetesMachineTemplate := &capkv1.KubernetesMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      fmt.Sprintf("%s-worker", generatedName),
		},
		Spec: capkv1.KubernetesMachineTemplateSpec{
			Template: capkv1.KubernetesMachineTemplateResource{
				Spec: capkv1.KubernetesMachineSpec{
					PodSpec: corev1.PodSpec{
						// Necessary to avoid API errors
						// TODO: avoid needing to set an empty array
						Containers: []corev1.Container{},
					},
				},
			},
		},
	}

	bootstrapTemplate := &cabpkv1.KubeadmConfigTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      generatedName,
		},
		Spec: cabpkv1.KubeadmConfigTemplateSpec{
			Template: cabpkv1.KubeadmConfigTemplateResource{
				Spec: cabpkv1.KubeadmConfigSpec{
					JoinConfiguration: &kubeadmv1beta1.JoinConfiguration{
						NodeRegistration: kubeadmv1beta1.NodeRegistrationOptions{
							KubeletExtraArgs: map[string]string{
								"eviction-hard":            "nodefs.available<0%,nodefs.inodesFree<0%,imagefs.available<0%",
								"cgroups-per-qos":          "false",
								"enforce-node-allocatable": "",
							},
						},
					},
				},
			},
		},
	}

	machineTemplate := capiv1.MachineTemplateSpec{
		ObjectMeta: capiv1.ObjectMeta{
			Namespace: namespace,
			Name:      generatedName,
		},
		Spec: capiv1.MachineSpec{
			ClusterName: cluster.GetName(),
			Bootstrap: capiv1.Bootstrap{
				ConfigRef: &corev1.ObjectReference{
					APIVersion: cabpkv1.GroupVersion.String(),
					Kind:       framework.TypeToKind(bootstrapTemplate),
					Namespace:  bootstrapTemplate.GetNamespace(),
					Name:       bootstrapTemplate.GetName(),
				},
			},
			InfrastructureRef: corev1.ObjectReference{
				APIVersion: capkv1.GroupVersion.String(),
				Kind:       framework.TypeToKind(kubernetesMachineTemplate),
				Namespace:  kubernetesMachineTemplate.GetNamespace(),
				Name:       kubernetesMachineTemplate.GetName(),
			},
			Version: &version,
		},
	}

	machineDeployment := &capiv1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      generatedName,
		},
		Spec: capiv1.MachineDeploymentSpec{
			ClusterName:             cluster.GetName(),
			Replicas:                &replicas,
			Template:                machineTemplate,
			Strategy:                nil,
			MinReadySeconds:         nil,
			RevisionHistoryLimit:    nil,
			Paused:                  false,
			ProgressDeadlineSeconds: nil,
		},
	}
	return machineDeployment, kubernetesMachineTemplate, bootstrapTemplate
}
