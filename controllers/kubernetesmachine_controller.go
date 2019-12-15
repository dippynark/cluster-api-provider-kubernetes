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
	"fmt"

	capkv1 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
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
	defaultImageTag       = "v1.16.3"
)

// KubernetesMachineReconciler reconciles a KubernetesMachine object
type KubernetesMachineReconciler struct {
	client.Client
	*rest.RESTClient
	*rest.Config
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.lukeaddison.co.uk,resources=kubernetesmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.lukeaddison.co.uk,resources=kubernetesmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;create;delete

func (r *KubernetesMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {
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
	return r.reconcileNormal(cluster, machine, kubernetesMachine, log)
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

func (r *KubernetesMachineReconciler) reconcileNormal(cluster *clusterv1.Cluster, machine *clusterv1.Machine, kubernetesMachine *capkv1.KubernetesMachine, log logr.Logger) (ctrl.Result, error) {
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
		log.Info("Waiting for the Bootstrap provider controller to set bootstrap data")
		return ctrl.Result{}, nil
	}

	// Check if machine pod already exists
	machinePod := &corev1.Pod{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Namespace: machine.Namespace,
		Name:      machinePodName(cluster, kubernetesMachine),
	}, machinePod)
	if k8serrors.IsNotFound(err) {
		if util.IsControlPlaneMachine(machine) {
			return r.createControlPlaneMachinePod(cluster, kubernetesMachine)
		}
		return r.createWorkerMachinePod(cluster, kubernetesMachine)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	// Ensure machine pod is controlled by kubernetes machine
	if ref := metav1.GetControllerOf(machinePod); ref == nil || ref.UID != kubernetesMachine.UID {
		return ctrl.Result{}, errors.Errorf("expected Pod %s in Namespace %s to be controlled by KubernetsMachine %s", machinePod.Name, machinePod.Namespace, kubernetesMachine.Name)
	}

	// exec bootstrap
	// NB. this step is necessary to mimic the behaviour of cloud-init that is embedded in the base images
	// for other cloud providers
	if err := r.execBootstrap(machinePod, *machine.Spec.Bootstrap.Data); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to exec KubernetesMachine bootstrap")
	}

	// Set the provider ID on the Kubernetes node corresponding to the external machine
	// NB. this step is necessary because there is no a cloud controller for kubernetes that executes this step
	if err := r.setNodeProviderID(cluster, kubernetesMachine, machinePod, log); err != nil {
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

func (r *KubernetesMachineReconciler) setNodeProviderID(cluster *clusterv1.Cluster, kubernetesMachine *infrav1.KubernetesMachine, machinePod *corev1.Pod, log logr.Logger) error {

	providerID, err := providerID(machinePod)
	if err != nil {
		return err
	}
	patch := fmt.Sprintf(`{"spec": {"providerID": "%s"}}`, providerID)
	req := r.Post().
		Resource("pods").
		Name(machinePod.Name).
		Namespace(machinePod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "kind",
			Command: []string{
				"kubectl",
				"--kubeconfig", "/etc/kubernetes/admin.conf",
				"patch",
				"node", machinePodName(cluster, kubernetesMachine),
				"--patch", patch,
			},
			Stdin:  false,
			Stdout: true,
			Stderr: true,
			TTY:    false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(r.Config, "POST", req.URL())
	if err != nil {
		return err
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	sopt := remotecommand.StreamOptions{
		Stdout: stdout,
		Stderr: stderr,
		Tty:    false,
	}
	err = exec.Stream(sopt)
	if err != nil {
		return err
	}

	if stdout.String() != "" {
		return errors.Errorf("Pod %s/%s exec stderr: %s", machinePod.Name, machinePod.Namespace, stderr.String())
	}
	log.Info(fmt.Sprintf("Pod %s/%s exec stdout: %s", machinePod.Name, machinePod.Namespace, stdout.String()))

	return nil
}

func (r *KubernetesMachineReconciler) execBootstrap(machinePod *corev1.Pod, data string) error {

	/*cloudConfig, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return errors.Wrap(err, "failed to decode machine's bootstrap data")
	}

	lines, err := cloudinit.Run(cloudConfig, m.container.Cmder())
	if err != nil {
		m.log.Error(err, strings.Join(lines, "\n"))
		return errors.Wrap(err, "failed to join a control plane node with kubeadm")
	}

	req := r.Post().
		Resource("pods").
		Name(machinePod.Name).
		Namespace(machinePod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "kind",
			Command: []string{
				"bash", "-c", data,
			},
			Stdin:  false,
			Stdout: true,
			Stderr: true,
			TTY:    false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(r.Config, "POST", req.URL())
	if err != nil {
		return err
	}*/

	return nil
}

func (r *KubernetesMachineReconciler) reconcileDelete(cluster *clusterv1.Cluster, machine *clusterv1.Machine, kubernetesMachine *capkv1.KubernetesMachine) (ctrl.Result, error) {
	// if the deleted machine is a control-plane node, exec kubeadm reset so the etcd member hosted
	// on the machine gets removed in a controlled way
	/*if util.IsControlPlaneMachine(machine) {
		if err := externalMachine.KubeadmReset(); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to execute kubeadm reset")
		}
	}*/

	// Delete the machine
	machinePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machinePodName(cluster, kubernetesMachine),
			Namespace: machine.Namespace,
		},
	}
	err := r.Client.Delete(context.TODO(), machinePod)
	if err != nil && !k8serrors.IsNotFound(err) {
		return ctrl.Result{}, errors.Wrap(err, "failed to delete KubernetesMachine")
	}

	// Machine is deleted so remove the finalizer.
	kubernetesMachine.Finalizers = util.Filter(kubernetesMachine.Finalizers, capkv1.MachineFinalizer)

	return ctrl.Result{}, nil
}

func (r *KubernetesMachineReconciler) createControlPlaneMachinePod(cluster *clusterv1.Cluster, kubernetesMachine *infrav1.KubernetesMachine) (ctrl.Result, error) {
	machinePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machinePodName(cluster, kubernetesMachine),
			Namespace: kubernetesMachine.Namespace,
			Labels: map[string]string{
				clusterv1.MachineClusterLabelName:      cluster.Name,
				clusterv1.MachineControlPlaneLabelName: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "kind",
					Image: fmt.Sprintf("%s:%s", defaultImageName, defaultImageTag),
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
							Name:      "tmp",
							MountPath: "/tmp",
						},
						{
							Name:      "run",
							MountPath: "/run",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "lib-modules",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/lib/modules",
						},
					},
				},
				{
					Name: "tmp",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium: "Memory",
						},
					},
				},
				{
					Name: "run",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium: "Memory",
						},
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

func (r *KubernetesMachineReconciler) createWorkerMachinePod(cluster *clusterv1.Cluster, kubernetesMachine *infrav1.KubernetesMachine) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func machinePodName(cluster *clusterv1.Cluster, kubernetesMachine *infrav1.KubernetesMachine) string {
	return fmt.Sprintf("%s-%s", cluster.Name, kubernetesMachine.Name)
}

func providerID(machinePod *corev1.Pod) (string, error) {
	uid := machinePod.GetUID()
	if uid == "" {
		return "", errors.Errorf("Pod %s/%s UID is empty", machinePod.Namespace, machinePod.Name)
	}
	return fmt.Sprintf("kubernetes:////%s", uid), nil
}
