package e2e

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeadmv1beta1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/types/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"

	capkv1 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha3"
	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
)

var _ = Describe("Kubernetes", func() {
	Describe("Cluster Creation", func() {
		var (
			namespace  string
			input      *framework.ControlplaneClusterInput
			clusterGen = &ClusterGenerator{}
			nodeGen    = &NodeGenerator{}
		)

		BeforeEach(func() {
			namespace = "default"
		})

		AfterEach(func() {
			//ensureDockerArtifactsDeleted(input)
		})

		Context("One node cluster", func() {
			It("should create a single node cluster", func() {
				cluster, infraCluster := clusterGen.GenerateCluster(namespace)
				nodes := make([]framework.Node, 1)
				for i := range nodes {
					nodes[i] = nodeGen.GenerateNode(cluster.GetName())
				}
				input = &framework.ControlplaneClusterInput{
					Management:    mgmt,
					Cluster:       cluster,
					InfraCluster:  infraCluster,
					Nodes:         nodes,
					CreateTimeout: 5 * time.Minute,
				}
				input.ControlPlaneCluster()

				input.CleanUpCoreArtifacts()
			})
		})

		Context("Multi-node controlplane cluster", func() {
			It("should create a multi-node controlplane cluster", func() {
				cluster, infraCluster := clusterGen.GenerateCluster(namespace)
				nodes := make([]framework.Node, 3)
				for i := range nodes {
					nodes[i] = nodeGen.GenerateNode(cluster.Name)
				}

				input = &framework.ControlplaneClusterInput{
					Management:    mgmt,
					Cluster:       cluster,
					InfraCluster:  infraCluster,
					Nodes:         nodes,
					CreateTimeout: 5 * time.Minute,
				}
				input.ControlPlaneCluster()

				input.CleanUpCoreArtifacts()
			})
		})
	})
})

type ClusterGenerator struct {
	counter int
}

func (c *ClusterGenerator) GenerateCluster(namespace string) (*capiv1.Cluster, *capkv1.KubernetesCluster) {
	generatedName := fmt.Sprintf("test-%d", c.counter)
	c.counter++

	kubernetesCluster := &capkv1.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      generatedName,
		},
	}

	cluster := &capiv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      generatedName,
		},
		Spec: capiv1.ClusterSpec{
			ClusterNetwork: &capiv1.ClusterNetwork{
				Services: &capiv1.NetworkRanges{CIDRBlocks: []string{}},
				Pods:     &capiv1.NetworkRanges{CIDRBlocks: []string{"192.168.0.0/16"}},
			},
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: capkv1.GroupVersion.String(),
				Kind:       framework.TypeToKind(kubernetesCluster),
				Namespace:  kubernetesCluster.GetNamespace(),
				Name:       kubernetesCluster.GetName(),
			},
		},
	}
	return cluster, kubernetesCluster
}

type NodeGenerator struct {
	counter int
}

func (n *NodeGenerator) GenerateNode(clusterName string) framework.Node {
	namespace := "default"
	version := "v1.17.0"
	generatedName := fmt.Sprintf("controlplane-%d", n.counter)
	n.counter++
	kubernetesMachine := &capkv1.KubernetesMachine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      generatedName,
		},
		Spec: capkv1.KubernetesMachineSpec{
			PodSpec: corev1.PodSpec{
				Containers: []corev1.Container{},
			},
		},
	}

	bootstrapConfig := &cabpkv1.KubeadmConfig{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      generatedName,
		},
		Spec: cabpkv1.KubeadmConfigSpec{
			ClusterConfiguration: &kubeadmv1beta1.ClusterConfiguration{
				APIServer: kubeadmv1beta1.APIServer{
					// Darwin support
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
	}

	machine := &capiv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      generatedName,
			Labels: map[string]string{
				capiv1.MachineControlPlaneLabelName: "",
				capiv1.ClusterLabelName:             clusterName,
			},
		},
		Spec: capiv1.MachineSpec{
			Bootstrap: capiv1.Bootstrap{
				ConfigRef: &corev1.ObjectReference{
					APIVersion: cabpkv1.GroupVersion.String(),
					Kind:       framework.TypeToKind(bootstrapConfig),
					Namespace:  bootstrapConfig.GetNamespace(),
					Name:       bootstrapConfig.GetName(),
				},
			},
			InfrastructureRef: corev1.ObjectReference{
				APIVersion: capkv1.GroupVersion.String(),
				Kind:       framework.TypeToKind(kubernetesMachine),
				Namespace:  kubernetesMachine.GetNamespace(),
				Name:       kubernetesMachine.GetName(),
			},
			Version:     &version,
			ClusterName: clusterName,
		},
	}
	return framework.Node{
		Machine:         machine,
		InfraMachine:    kubernetesMachine,
		BootstrapConfig: bootstrapConfig,
	}
}
