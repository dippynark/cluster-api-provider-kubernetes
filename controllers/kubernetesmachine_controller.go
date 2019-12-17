/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	capkv1 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha1"
	"github.com/dippynark/cluster-api-provider-kubernetes/pkg/cloudinit"
	"github.com/dippynark/cluster-api-provider-kubernetes/pkg/pod"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	coreV1Client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha1"
)

const (
	machineControllerName = "KubernetesMachine-controller"
	defaultImageName      = "kindest/node"
	kindContainerName     = "kind"
)

// KubernetesMachineReconciler reconciles a KubernetesMachine object
type KubernetesMachineReconciler struct {
	client.Client
	*coreV1Client.CoreV1Client
	*rest.Config
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.lukeaddison.co.uk,resources=kubernetesmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.lukeaddison.co.uk,resources=kubernetesmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;watch;create

func (r *KubernetesMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {
	// TODO: what does ctx do
	ctx := context.Background()
	log := r.Log.WithName(machineControllerName).WithValues("kubernetes-machine", req.NamespacedName)

	// Fetch the KubernetesMachine instance.
	kubernetesMachine := &capkv1.KubernetesMachine{}
	if err := r.Client.Get(ctx, req.NamespacedName, kubernetesMachine); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, kubernetesMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("Waiting for Machine Controller to set OwnerRef on KubernetesMachine")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("machine", machine.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Info("KubernetesMachine owner Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info(fmt.Sprintf("Please associate this machine with a cluster using the label %s: <name of cluster>", clusterv1.MachineClusterLabelName))
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Make sure infrastructure is ready
	if !cluster.Status.InfrastructureReady {
		log.Info("Waiting for KubernetesCluster Controller to create cluster infrastructure")
		return ctrl.Result{}, nil
	}

	// Fetch the Kubernetes Cluster.
	kubernetesCluster := &capkv1.KubernetesCluster{}
	kubernetesClusterName := types.NamespacedName{
		Namespace: kubernetesMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, kubernetesClusterName, kubernetesCluster); err != nil {
		log.Info("KubernetesCluster is not available yet")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("kubernetes-cluster", kubernetesCluster.Name)

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(kubernetesMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to Patch the KubernetesMachine object and status after each reconciliation.
	defer func() {
		if err := patchHelper.Patch(ctx, kubernetesMachine); err != nil {
			log.Error(err, "failed to patch KubernetesMachine")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// Handle deleted machines
	if !kubernetesMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(cluster, machine, kubernetesMachine)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(cluster, machine, kubernetesMachine)
}

// SetupWithManager adds watches for this controller
func (r *KubernetesMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.KubernetesMachine{}).
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("KubernetesMachine")),
			},
		).
		Watches(
			&source.Kind{Type: &infrav1.KubernetesCluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(r.KubernetesClusterToKubernetesMachines),
			},
		).
		Watches(
			&source.Kind{Type: &corev1.Pod{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(r.PodToKubernetesMachine),
			},
		).
		Complete(r)
}

// KubernetesClusterToKubernetesMachines is a handler.ToRequestsFunc to be used
// to enqeue requests for reconciliation of KubernetesMachines.
func (r *KubernetesMachineReconciler) KubernetesClusterToKubernetesMachines(o handler.MapObject) []ctrl.Request {
	result := []ctrl.Request{}
	c, ok := o.Object.(*infrav1.KubernetesCluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a KubernetesCluster but got a %T", o.Object), "failed to get KubernetesMachine for KubernetesCluster")
		return nil
	}
	log := r.Log.WithValues("KubernetesCluster", c.Name, "Namespace", c.Namespace)

	cluster, err := util.GetOwnerCluster(context.TODO(), r.Client, c.ObjectMeta)
	switch {
	case k8serrors.IsNotFound(err) || cluster == nil:
		return result
	case err != nil:
		log.Error(err, "failed to get owning cluster")
		return result
	}

	labels := map[string]string{clusterv1.MachineClusterLabelName: cluster.Name}
	machineList := &clusterv1.MachineList{}
	if err := r.Client.List(context.TODO(), machineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
		log.Error(err, "failed to list KubernetesMachines")
		return nil
	}
	for _, m := range machineList.Items {
		if m.Spec.InfrastructureRef.Name == "" {
			continue
		}
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Name}
		result = append(result, ctrl.Request{NamespacedName: name})
	}

	return result
}

// PodToKubernetesMachine is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of KubernetesMachines
func (r *KubernetesMachineReconciler) PodToKubernetesMachine(o handler.MapObject) []ctrl.Request {
	result := []ctrl.Request{}
	s, ok := o.Object.(*corev1.Pod)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Service but got a %T", o.Object), "failed to get KubernetesMachine for Pod")
		return nil
	}
	log := r.Log.WithValues("Pod", s.Name, "Namespace", s.Namespace)

	// Only watch pods owned by a kubernetesmachine
	ref := metav1.GetControllerOf(s)
	if ref == nil || (ref.Kind != "KubernetesMachine" || ref.APIVersion != infrav1.GroupVersion.String()) {
		return nil
	}
	log.Info(fmt.Sprintf("Found Pod owned by KubernetesMachine %s/%s", s.Namespace, ref.Name))
	name := client.ObjectKey{Namespace: s.Namespace, Name: ref.Name}
	result = append(result, ctrl.Request{NamespacedName: name})

	return result
}

func (r *KubernetesMachineReconciler) reconcileNormal(cluster *clusterv1.Cluster, machine *clusterv1.Machine, kubernetesMachine *capkv1.KubernetesMachine) (ctrl.Result, error) {
	// If the KubernetesMachine doesn't have finalizer, add it.
	if !util.Contains(kubernetesMachine.Finalizers, capkv1.MachineFinalizer) {
		kubernetesMachine.Finalizers = append(kubernetesMachine.Finalizers, capkv1.MachineFinalizer)
	}

	// if the machine is already provisioned, return
	if kubernetesMachine.Spec.ProviderID != nil {
		return ctrl.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	if machine.Spec.Bootstrap.Data == nil {
		r.Log.Info("Waiting for the Bootstrap provider controller to set bootstrap data")
		return ctrl.Result{}, nil
	}

	// Check if machine pod already exists
	machinePod := &corev1.Pod{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Namespace: machine.Namespace,
		Name:      machinePodName(cluster, machine),
	}, machinePod)
	if k8serrors.IsNotFound(err) {
		return r.createMachinePod(cluster, machine, kubernetesMachine)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	// Ensure machine pod is controlled by kubernetes machine
	if ref := metav1.GetControllerOf(machinePod); ref == nil || ref.UID != kubernetesMachine.UID {
		return ctrl.Result{}, errors.Errorf("expected Pod %s in Namespace %s to be controlled by KubernetsMachine %s", machinePod.Name, machinePod.Namespace, kubernetesMachine.Name)
	}

	// Check if machine pod is running
	if machinePod.Status.Phase != corev1.PodRunning {
		r.Log.Info(fmt.Sprintf("Pod %s/%s is not running", machinePod.Namespace, machinePod.Name))
		return ctrl.Result{}, nil
	}

	// exec bootstrap
	// NB. this step is necessary to mimic the behaviour of cloud-init that is embedded in the base images
	// for other cloud providers
	if err := r.execBootstrap(machinePod, *machine.Spec.Bootstrap.Data); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to exec KubernetesMachine bootstrap")
	}

	// Set the provider ID on the Kubernetes node corresponding to the external machine
	// NB. this step is necessary because there is no a cloud controller for kubernetes that executes this step
	if err := r.setNodeProviderID(cluster, machine, machinePod); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to patch the Kubernetes node with the machine providerID")
	}

	// Set ProviderID so the Cluster API Machine Controller can pull it
	providerID, err := providerID(machinePod)
	if err != nil {
		return ctrl.Result{}, err
	}
	kubernetesMachine.Spec.ProviderID = &providerID

	// Mark the kubernetesMachine ready
	kubernetesMachine.Status.Ready = true

	return ctrl.Result{}, nil
}

func (r *KubernetesMachineReconciler) execBootstrap(machinePod *corev1.Pod, data string) error {

	cloudConfig, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return errors.Wrap(err, "failed to decode machine's bootstrap data")
	}

	r.Log.Info("Running machine bootstrap scripts")
	machinePodKindCmder := pod.ContainerCmder(r.CoreV1Client, r.Config, machinePod.Name, machinePod.Namespace, "kind")
	lines, err := cloudinit.Run(cloudConfig, machinePodKindCmder)
	if err != nil {
		r.Log.Error(err, strings.Join(lines, "\n"))
		return errors.Wrap(err, "failed to join node with kubeadm")
	}

	return nil
}

func (r *KubernetesMachineReconciler) setNodeProviderID(cluster *clusterv1.Cluster, machine *clusterv1.Machine, machinePod *corev1.Pod) error {

	// Find controller pod
	labels := map[string]string{
		clusterv1.MachineClusterLabelName:      cluster.Name,
		clusterv1.MachineControlPlaneLabelName: "true",
	}
	podList := &corev1.PodList{}
	if err := r.Client.List(context.TODO(), podList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
		r.Log.Error(err, fmt.Sprintf("failed to list controller Pods for Cluster %s/%s", cluster.Namespace, cluster.Name))
		return err
	}
	if len(podList.Items) == 0 {
		return errors.New("Unable to find controller Pod for Cluster")
	}
	// TODO: consider other controller pods
	controllerPod := podList.Items[0]

	r.Log.Info("Setting node provider ID")
	machinePodKindCmder := pod.ContainerCmder(r.CoreV1Client, r.Config, controllerPod.Name, machine.Namespace, "kind")

	providerID, err := providerID(machinePod)
	if err != nil {
		return err
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	machinePodKindCmd := machinePodKindCmder.Command("kubectl",
		"--kubeconfig", "/etc/kubernetes/admin.conf",
		"patch",
		"node", machinePodName(cluster, machine),
		"--patch", fmt.Sprintf(`{"spec": {"providerID": "%s"}}`, providerID))
	machinePodKindCmd.SetStdout(stdout)
	machinePodKindCmd.SetStderr(stderr)

	err = machinePodKindCmd.Run()
	if err != nil {
		if stderr.String() != "" {
			return errors.Errorf("Pod %s/%s exec stderr: %s", machinePod.Name, machinePod.Namespace, stderr.String())
		}
		return errors.Errorf("Pod %s/%s exec stdout: %s", machinePod.Name, machinePod.Namespace, stdout.String())
	}
	r.Log.Info(fmt.Sprintf("Pod %s/%s exec stdout: %s", machinePod.Name, machinePod.Namespace, stdout.String()))

	return nil
}

func (r *KubernetesMachineReconciler) reconcileDelete(cluster *clusterv1.Cluster, machine *clusterv1.Machine, kubernetesMachine *capkv1.KubernetesMachine) (ctrl.Result, error) {
	// if the deleted machine is a control-plane node, exec kubeadm reset so the
	// etcd member hosted on the machine gets removed in a controlled way
	if util.IsControlPlaneMachine(machine) {
		if err := r.kubeadmReset(cluster, machine); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to execute kubeadm reset")
		}
	}

	// Machine is deleted so remove the finalizer.
	kubernetesMachine.Finalizers = util.Filter(kubernetesMachine.Finalizers, capkv1.MachineFinalizer)

	return ctrl.Result{}, nil
}

// kubeadmReset will run `kubeadm reset` on the machine
func (r *KubernetesMachineReconciler) kubeadmReset(cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	machinePodKindCmder := pod.ContainerCmder(r.CoreV1Client, r.Config, machinePodName(cluster, machine), machine.Namespace, "kind")
	machinePodKindCmd := machinePodKindCmder.Command("kubeadm",
		"reset",
		"--force")
	machinePodKindCmd.SetStdout(stdout)
	machinePodKindCmd.SetStderr(stderr)

	r.Log.Info("Running kubeadm reset on the machine")
	err := machinePodKindCmd.Run()
	if err != nil {
		if stderr.String() != "" {
			return errors.Errorf("Pod %s/%s exec stderr: %s", machinePodName(cluster, machine), machine.Namespace, stderr.String())
		}
		return errors.Errorf("Pod %s/%s exec stdout: %s", machinePodName(cluster, machine), machine.Namespace, stdout.String())
	}
	r.Log.Info(fmt.Sprintf("Pod %s/%s exec stdout: %s", machinePodName(cluster, machine), machine.Namespace, stdout.String()))

	return nil
}

func (r *KubernetesMachineReconciler) createMachinePod(cluster *clusterv1.Cluster, machine *clusterv1.Machine, kubernetesMachine *infrav1.KubernetesMachine) (ctrl.Result, error) {
	if util.IsControlPlaneMachine(machine) {
		return r.createControlPlaneMachinePod(cluster, machine, kubernetesMachine)
	}
	return r.createWorkerMachinePod(cluster, machine, kubernetesMachine)
}

func (r *KubernetesMachineReconciler) createControlPlaneMachinePod(cluster *clusterv1.Cluster, machine *clusterv1.Machine, kubernetesMachine *infrav1.KubernetesMachine) (ctrl.Result, error) {
	directory := corev1.HostPathDirectory
	machinePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machinePodName(cluster, machine),
			Namespace: machine.Namespace,
			Labels: map[string]string{
				clusterv1.MachineClusterLabelName:      cluster.Name,
				clusterv1.MachineControlPlaneLabelName: "true",
			},
		},
		Spec: corev1.PodSpec{
			DNSPolicy: "None",
			DNSConfig: &corev1.PodDNSConfig{
				Nameservers: []string{"8.8.8.8", "8.8.4.4"},
			},
			Containers: []corev1.Container{
				{
					Name:  kindContainerName,
					Image: machinePodImage(machine),
					Ports: []corev1.ContainerPort{
						{
							Name:          "https",
							Protocol:      "TCP",
							ContainerPort: 6443,
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: pointer.BoolPtr(true),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "lib-modules",
							MountPath: "/lib/modules",
							ReadOnly:  true,
						},
						{
							Name:      "var-lib-containerd",
							MountPath: "/var/lib/containerd",
						},
						{
							Name:      "var-lib-etcd",
							MountPath: "/var/lib/etcd",
						},
					},
					Resources: kubernetesMachine.Spec.Resources,
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "lib-modules",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/lib/modules",
							Type: &directory,
						},
					},
				},
				{
					Name: "var-lib-containerd",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "var-lib-etcd",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(kubernetesMachine, machinePod, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, r.Create(context.TODO(), machinePod)
}

func (r *KubernetesMachineReconciler) createWorkerMachinePod(cluster *clusterv1.Cluster, machine *clusterv1.Machine, kubernetesMachine *infrav1.KubernetesMachine) (ctrl.Result, error) {
	directory := corev1.HostPathDirectory
	machinePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machinePodName(cluster, machine),
			Namespace: machine.Namespace,
			Labels: map[string]string{
				clusterv1.MachineClusterLabelName: cluster.Name,
			},
		},
		Spec: corev1.PodSpec{
			DNSPolicy: "None",
			DNSConfig: &corev1.PodDNSConfig{
				Nameservers: []string{"8.8.8.8", "8.8.4.4"},
			},
			Containers: []corev1.Container{
				{
					Name:  kindContainerName,
					Image: machinePodImage(machine),
					SecurityContext: &corev1.SecurityContext{
						Privileged: pointer.BoolPtr(true),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "lib-modules",
							MountPath: "/lib/modules",
							ReadOnly:  true,
						},
						{
							Name:      "var-lib-containerd",
							MountPath: "/var/lib/containerd",
						},
					},
					Resources: kubernetesMachine.Spec.Resources,
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "lib-modules",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/lib/modules",
							Type: &directory,
						},
					},
				},
				{
					Name: "var-lib-containerd",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(kubernetesMachine, machinePod, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, r.Create(context.TODO(), machinePod)
}

func machinePodImage(machine *clusterv1.Machine) string {
	return fmt.Sprintf("%s:%s", defaultImageName, *machine.Spec.Version)
}

func machinePodName(cluster *clusterv1.Cluster, machine *clusterv1.Machine) string {
	return fmt.Sprintf("%s-%s", cluster.Name, machine.Name)
}

func providerID(machinePod *corev1.Pod) (string, error) {
	uid := machinePod.GetUID()
	if uid == "" {
		return "", errors.Errorf("Pod %s/%s UID is empty", machinePod.Namespace, machinePod.Name)
	}
	return fmt.Sprintf("kubernetes:////%s", uid), nil
}

func kubeconfigSecretName(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("%s-kubeconfig", cluster.Name)
}
