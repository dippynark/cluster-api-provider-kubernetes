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
	"context"
	"fmt"
	"path"
	"time"

	capkv1 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha1"
	utils "github.com/dippynark/cluster-api-provider-kubernetes/pkg/utils"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	coreV1Client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	machineControllerName              = "KubernetesMachine-controller"
	enableBootstrapProcessRequeueAfter = time.Second * 5
	setNodeProviderIDRequeueAfter      = time.Second * 5
	defaultImageName                   = "kindest/node"
	defaultImageTag                    = "v1.17.0"
	kindContainerName                  = "kind"
	defaultAPIServerPort               = 6443
	libModulesVolumeName               = "lib-modules"
	libModulesVolumeMountPath          = "/lib/modules"
	runVolumeName                      = "run"
	runVolumeMountPath                 = "/run"
	tmpVolumeName                      = "tmp"
	tmpVolumeMountPath                 = "/tmp"
	varLibVolumeName                   = "var-lib"
	varLibVolumeMountPath              = "/var/lib"
	varLogVolumeName                   = "var-log"
	varLogVolumeMountPath              = "/var/log"
	cloudInitScriptsVolumeName         = "cloud-init-scripts"
	cloudInitScriptsVolumeMountPath    = "/opt/cloud-init"
	cloudInitSystemdUnitsVolume        = "cloud-init-systemd-units"
	etcSystemdSystem                   = "/etc/systemd/system"
	cloudInitBootstrapScriptName       = "bootstrap.sh"
	cloudInitInstallScriptName         = "install.sh"
	cloudInitInstallScript             = `#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

systemctl enable --now cloud-init.path
`
	cloudInitSystemdServiceUnitName = "cloud-init.service"
	cloudInitSystemdServiceUnit     = `[Unit]
Description=Cloud-init bootstrap
After=network.target

[Service]
ExecStart=/opt/cloud-init/bootstrap.sh
`
	cloudInitSystemdPathUnitName = "cloud-init.path"
	cloudInitSystemdPathUnit     = `[Unit]
Description=Detect containerd socket creation

[Path]
PathExists=/var/run/containerd/containerd.sock

[Install]
WantedBy=multi-user.target
`
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
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=core,resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=list;watch;create
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create

func (r *KubernetesMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {
	// TODO: what does ctx do
	ctx := context.Background()
	log := log.Log.WithValues(namespaceLogName, req.Namespace, kubernetesMachineLogName, req.Name)

	// Fetch the KubernetesMachine instance.
	kubernetesMachine := &capkv1.KubernetesMachine{}
	if err := r.Client.Get(ctx, req.NamespacedName, kubernetesMachine); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(kubernetesMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to Patch the KubernetesMachine object and status after each reconciliation.
	defer func() {
		r.reconcilePhase(kubernetesMachine)

		if err := patchHelper.Patch(ctx, kubernetesMachine); err != nil {
			log.Error(err, "failed to patch KubernetesMachine")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, kubernetesMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("Waiting for Machine Controller to set OwnerRef on KubernetesMachine")
		return ctrl.Result{}, nil
	}

	log = log.WithValues(machineLogName, machine.Name)

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

	log = log.WithValues(clusterLogName, cluster.Name)

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

	log = log.WithValues(kubernetesClusterLogName, kubernetesCluster.Name)

	// Handle deleted machines
	if !kubernetesMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(machine, kubernetesMachine)
	}

	// Make sure infrastructure is ready
	if !cluster.Status.InfrastructureReady {
		log.Info("Waiting for KubernetesCluster Controller to create cluster infrastructure")
		return ctrl.Result{}, nil
	}

	// Handle non-deleted machines
	return r.reconcileNormal(cluster, machine, kubernetesMachine)
}

// SetupWithManager adds watches for this controller
func (r *KubernetesMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capkv1.KubernetesMachine{}).
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(capkv1.GroupVersion.WithKind("KubernetesMachine")),
			},
		).
		Watches(
			&source.Kind{Type: &capkv1.KubernetesCluster{}},
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
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(r.SecretToKubernetesMachine),
			},
		).
		Watches(
			&source.Kind{Type: &corev1.PersistentVolumeClaim{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(r.PersistentVolumeClaimToKubernetesMachine),
			},
		).
		Complete(r)
}

// KubernetesClusterToKubernetesMachines is a handler.ToRequestsFunc to be used
// to enqeue requests for reconciliation of KubernetesMachines.
func (r *KubernetesMachineReconciler) KubernetesClusterToKubernetesMachines(o handler.MapObject) []ctrl.Request {
	result := []ctrl.Request{}
	c, ok := o.Object.(*capkv1.KubernetesCluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a KubernetesCluster but got a %T", o.Object), "failed to get KubernetesMachine for KubernetesCluster")
		return nil
	}
	log := r.Log.WithValues(namespaceLogName, c.Namespace, clusterLogName, c.Name)

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
	p, ok := o.Object.(*corev1.Pod)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Pod but got a %T", o.Object), "failed to get KubernetesMachine for Pod")
		return nil
	}
	log := r.Log.WithValues(namespaceLogName, p.Namespace, podLogName, p.Name)

	// Only watch pods owned by a kubernetesmachine
	ref := metav1.GetControllerOf(p)
	if ref == nil || (ref.Kind != "KubernetesMachine" || ref.APIVersion != capkv1.GroupVersion.String()) {
		return nil
	}
	log = log.WithValues(kubernetesMachineLogName, ref.Name)
	log.Info("Found Pod owned by KubernetesMachine")
	name := client.ObjectKey{Namespace: p.Namespace, Name: ref.Name}
	result = append(result, ctrl.Request{NamespacedName: name})

	return result
}

// SecretToKubernetesMachine is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of KubernetesMachines
func (r *KubernetesMachineReconciler) SecretToKubernetesMachine(o handler.MapObject) []ctrl.Request {
	result := []ctrl.Request{}
	s, ok := o.Object.(*corev1.Secret)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Secret but got a %T", o.Object), "failed to get KubernetesMachine for Secret")
		return nil
	}
	log := r.Log.WithValues(namespaceLogName, s.Namespace, secretLogName, s.Name)

	// Only watch secrets owned by a kubernetesmachine
	ref := metav1.GetControllerOf(s)
	if ref == nil || (ref.Kind != "KubernetesMachine" || ref.APIVersion != capkv1.GroupVersion.String()) {
		return nil
	}
	log = log.WithValues(kubernetesMachineLogName, ref.Name)
	log.Info("Found Secret owned by KubernetesMachine")
	name := client.ObjectKey{Namespace: s.Namespace, Name: ref.Name}
	result = append(result, ctrl.Request{NamespacedName: name})

	return result
}

// PersistentVolumeClaimToKubernetesMachine is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of KubernetesMachines
func (r *KubernetesMachineReconciler) PersistentVolumeClaimToKubernetesMachine(o handler.MapObject) []ctrl.Request {
	result := []ctrl.Request{}
	p, ok := o.Object.(*corev1.PersistentVolumeClaim)
	if !ok {
		r.Log.Error(errors.Errorf("expected a PersistentVolumeClaim but got a %T", o.Object), "failed to get KubernetesMachine for Secret")
		return nil
	}
	log := r.Log.WithValues(namespaceLogName, p.Namespace, persistentVolumeClaimLogName, p.Name)

	// Only watch persistentvolumeclaim owned by a kubernetesmachine
	ref := metav1.GetControllerOf(p)
	if ref == nil || (ref.Kind != "KubernetesMachine" || ref.APIVersion != capkv1.GroupVersion.String()) {
		return nil
	}
	log = log.WithValues(kubernetesMachineLogName, ref.Name)
	log.Info("Found PersistentVolumeClaim owned by KubernetesMachine")
	name := client.ObjectKey{Namespace: p.Namespace, Name: ref.Name}
	result = append(result, ctrl.Request{NamespacedName: name})

	return result
}

func (r *KubernetesMachineReconciler) reconcileNormal(cluster *clusterv1.Cluster, machine *clusterv1.Machine, kubernetesMachine *capkv1.KubernetesMachine) (ctrl.Result, error) {
	log := r.Log.WithValues(namespaceLogName, cluster.Namespace, clusterLogName, cluster.Name, machineLogName, machine.Name, kubernetesClusterLogName, kubernetesMachine.Name)

	// If the kubernetesMachine is in an error state, return early
	if kubernetesMachine.Status.ErrorReason != nil || kubernetesMachine.Status.ErrorMessage != nil {
		return reconcile.Result{}, nil
	}

	// If the KubernetesMachine doesn't have finalizer, add it.
	if !util.Contains(kubernetesMachine.Finalizers, capkv1.KubernetesMachineFinalizer) {
		kubernetesMachine.Finalizers = append(kubernetesMachine.Finalizers, capkv1.KubernetesMachineFinalizer)
	}

	// If the KubernetesMachine doesn't have foregroundDeletion finalizer, add it.
	if !util.Contains(kubernetesMachine.Finalizers, metav1.FinalizerDeleteDependents) {
		kubernetesMachine.Finalizers = append(kubernetesMachine.Finalizers, metav1.FinalizerDeleteDependents)
	}

	// Make sure bootstrap data is available and populated.
	if machine.Spec.Bootstrap.Data == nil {
		log.Info("Waiting for the Bootstrap provider controller to set bootstrap data")
		return ctrl.Result{}, nil
	}

	// Get or create machine pod
	machinePod := &corev1.Pod{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Namespace: machine.Namespace,
		Name:      machinePodName(kubernetesMachine),
	}, machinePod)
	if k8serrors.IsNotFound(err) {
		if kubernetesMachine.Spec.ProviderID != nil {
			// Machine pod was previous created so something has deleted it
			// This could be due to the Node it was running on failing (for example)
			// We rely on a higher level object for recreation
			kubernetesMachine.Status.SetErrorReason(capierrors.UnsupportedChangeMachineError)
			kubernetesMachine.Status.SetErrorMessage(errors.New("Machine Pod cannot be found"))
			return ctrl.Result{}, nil
		}
		return r.createMachinePod(cluster, machine, kubernetesMachine)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	log = log.WithValues(podLogName, machinePod.Name)

	// Ensure machine pod is controlled by kubernetes machine
	if ref := metav1.GetControllerOf(machinePod); ref == nil || ref.UID != kubernetesMachine.UID {
		kubernetesMachine.Status.SetErrorReason(capierrors.UnsupportedChangeMachineError)
		kubernetesMachine.Status.SetErrorMessage(errors.Errorf("Machine Pod is not controlled by KubernetesMachine"))
		return ctrl.Result{}, nil
	}

	// Machine Pod has been created so update the providerID so the Cluster API Machine Controller
	// can pull it
	if kubernetesMachine.Spec.ProviderID == nil {
		providerID, err := getProviderIDFromPod(machinePod)
		if err != nil {
			return ctrl.Result{}, err
		}
		kubernetesMachine.Spec.ProviderID = &providerID
		return ctrl.Result{}, nil
	}

	// Make sure machine pod uid and providerID match
	if getPodUIDFromProviderID(*kubernetesMachine.Spec.ProviderID) != string(machinePod.UID) {
		kubernetesMachine.Status.SetErrorReason(capierrors.UnsupportedChangeMachineError)
		kubernetesMachine.Status.SetErrorMessage(errors.Errorf("Machine Pod UID has changed"))
		return ctrl.Result{}, nil
	}

	// Handle deleting machine pod
	if !machinePod.ObjectMeta.DeletionTimestamp.IsZero() {
		kubernetesMachine.Status.SetErrorReason(capierrors.UnsupportedChangeMachineError)
		kubernetesMachine.Status.SetErrorMessage(errors.Errorf("Machine Pod has been deleted unexpectedly"))
		return ctrl.Result{}, nil
	}

	// Generate cloud-init bootstrap script secret
	cloudInitScriptSecret, err := r.generatateCloudInitSecret(cluster, machine, kubernetesMachine, machine.Spec.Bootstrap.Data)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create secret
	// TODO: check whether secret needs to be updated
	err = r.Create(context.TODO(), cloudInitScriptSecret)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return ctrl.Result{}, err
	}
	// Ensure secret is controlled by kubernetes machine
	if ref := metav1.GetControllerOf(cloudInitScriptSecret); ref == nil || ref.UID != kubernetesMachine.UID {
		kubernetesMachine.Status.SetErrorReason(capierrors.UnsupportedChangeMachineError)
		kubernetesMachine.Status.SetErrorMessage(errors.Errorf("bootstrap Secret is not controlled by KubernetesMachine"))
		return ctrl.Result{}, nil
	}

	// Create persistent volume claims
	// TODO: clean up pvcs if they are removed from the list of templates
	// TODO: set error phase/reason/message if irrecoverable error occurs
	err = r.createPersistentVolumeClaims(kubernetesMachine)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create persistent volume claims")
	}

	// Check status of kind container
	kindContainerStatus, exists := utils.GetContainerStatus(machinePod.Status.ContainerStatuses, kindContainerName)
	if !exists {
		log.Info("Waiting for kind container status")
		return ctrl.Result{}, nil
	}
	if kindContainerStatus.State.Terminated != nil {
		kubernetesMachine.Status.SetErrorReason(capierrors.UnsupportedChangeMachineError)
		kubernetesMachine.Status.SetErrorMessage(errors.Errorf("kind container has terminated: %s", kindContainerStatus.State.Terminated.Reason))

		return ctrl.Result{}, nil
	}
	if kindContainerStatus.State.Running == nil {
		log.Info("Waiting for kind container to be running")
		return ctrl.Result{}, nil
	}

	// Enable bootstrap process
	// TODO: if this has already been done, don't enable it again?
	if err := r.enableBoostrapProcess(machinePod); err != nil {
		return ctrl.Result{RequeueAfter: enableBootstrapProcessRequeueAfter}, errors.Wrap(err, "failed to enable bootstrap process")
	}

	// Check kind container is ready before attempting to set providerID
	if !kindContainerStatus.Ready {
		log.Info("Waiting for kind container to be ready")
		return ctrl.Result{}, nil
	}

	// Set the provider ID on the Kubernetes node corresponding to the external machine
	// NB. this step is necessary because there is not a cloud controller for kubernetes that executes this step
	// TODO: if this has already been done, don't set it again?
	if err := r.setNodeProviderID(cluster, machinePod); err != nil {
		return ctrl.Result{RequeueAfter: setNodeProviderIDRequeueAfter}, errors.Wrap(err, "failed to patch the Kubernetes node with the machine providerID")
	}

	// Check Machine Pod is ready before marking kubernetesMachine ready
	// TODO: do we need this for worker Nodes?
	if !utils.IsPodReady(machinePod) {
		log.Info("Waiting for machine Pod to be ready")
		return ctrl.Result{}, nil
	}

	// Mark the kubernetesMachine ready
	// TODO: should ready ever go back to false?
	kubernetesMachine.Status.Ready = true

	return ctrl.Result{}, nil
}

func (r *KubernetesMachineReconciler) reconcileDelete(machine *clusterv1.Machine, kubernetesMachine *capkv1.KubernetesMachine) (ctrl.Result, error) {
	// If the deleted machine is a control-plane node, exec kubeadm reset so the
	// etcd member hosted on the machine gets removed in a controlled way
	if util.IsControlPlaneMachine(machine) && util.Contains(kubernetesMachine.Finalizers, capkv1.KubernetesMachineFinalizer) {
		// Check if machine pod exists
		machinePod := &corev1.Pod{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{
			Namespace: machine.Namespace,
			Name:      machinePodName(kubernetesMachine),
		}, machinePod)
		// Check we found a pod...
		if !k8serrors.IsNotFound(err) {
			// ...and we didn't encounter another error
			if err != nil {
				return ctrl.Result{}, err
			}
			// Ensure machine pod is controlled by kubernetes machine
			if ref := metav1.GetControllerOf(machinePod); ref != nil && ref.UID == kubernetesMachine.UID {
				if err := r.kubeadmReset(machinePod); err != nil {
					// TODO: if this happens too much should we raise a failure?
					return ctrl.Result{}, errors.Wrap(err, "failed to execute kubeadm reset")
				}
			}
		} else {
			// TODO: the pod is not found, do we want to hang here instead?
			kubernetesMachine.Finalizers = util.Filter(kubernetesMachine.Finalizers, capkv1.KubernetesMachineFinalizer)
			kubernetesMachine.Status.SetErrorReason(capierrors.UnsupportedChangeMachineError)
			kubernetesMachine.Status.SetErrorMessage(errors.New("Machine Pod cannot be found"))

			return ctrl.Result{}, nil
		}
	}

	// Machine is deleted so remove the finalizer.
	kubernetesMachine.Finalizers = util.Filter(kubernetesMachine.Finalizers, capkv1.KubernetesMachineFinalizer)

	return ctrl.Result{}, nil
}

func (r *KubernetesMachineReconciler) reconcilePhase(k *capkv1.KubernetesMachine) {
	// Set phase to "Pending" if nil
	if k.Status.Phase == "" {
		k.Status.Phase = capkv1.KubernetesMachinePhasePending
	}

	// Set phase to "Provisioning" if the providerID has been set
	if k.Spec.ProviderID != nil {
		k.Status.Phase = capkv1.KubernetesMachinePhaseProvisioning
	}

	// Set phase to "Running" if providerID has been set and the
	// KubernetesMachine is ready
	if k.Spec.ProviderID != nil && k.Status.Ready {
		k.Status.Phase = capkv1.KubernetesMachinePhaseRunning
	}

	// Set phase to "Failed" if any of Status.ErrorReason or Status.ErrorMessage
	// is not-nil
	if k.Status.ErrorReason != nil || k.Status.ErrorMessage != nil {
		k.Status.Phase = capkv1.KubernetesMachinePhaseFailed
	}

	// Set phase to "Deleting" if the deletion timestamp is set
	if !k.DeletionTimestamp.IsZero() {
		k.Status.Phase = capkv1.KubernetesMachinePhaseDeleting
	}
}

func (r *KubernetesMachineReconciler) enableBoostrapProcess(machinePod *corev1.Pod) error {
	log := r.Log.WithValues(namespaceLogName, machinePod.Namespace, podLogName, machinePod.Name)
	log.Info("Enabling bootstrap process")
	return r.kindContainerExec(machinePod, path.Join(cloudInitScriptsVolumeMountPath, cloudInitInstallScriptName))
}

func (r *KubernetesMachineReconciler) setNodeProviderID(cluster *clusterv1.Cluster, machinePod *corev1.Pod) error {
	log := r.Log.WithValues(namespaceLogName, cluster.Namespace, podLogName, machinePod.Name)

	// Find controller pod
	log.Info("Finding controller Pod")
	labels := map[string]string{
		clusterv1.MachineClusterLabelName:      cluster.Name,
		clusterv1.MachineControlPlaneLabelName: "true",
	}
	podList := &corev1.PodList{}
	if err := r.Client.List(context.TODO(), podList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
		return errors.Wrap(err, "failed to list controller Pods for Cluster")
	}
	if len(podList.Items) == 0 {
		return errors.New("Unable to find controller Pod for Cluster")
	}
	// TODO: consider other controller pods
	controllerPod := podList.Items[0]

	providerID, err := getProviderIDFromPod(machinePod)
	if err != nil {
		return err
	}
	log.Info("Setting Node provider ID")
	return r.kindContainerExec(&controllerPod, "kubectl",
		"--kubeconfig", "/etc/kubernetes/admin.conf",
		"patch",
		"node", machinePodNodeName(machinePod),
		"--patch", fmt.Sprintf(`{"spec": {"providerID": "%s"}}`, providerID))
}

// kubeadmReset will run `kubeadm reset` on the machine
func (r *KubernetesMachineReconciler) kubeadmReset(machinePod *corev1.Pod) error {
	log := r.Log.WithValues(namespaceLogName, machinePod.Namespace, podLogName, machinePod.Name)
	log.Info("Running kubeadm reset")
	return r.kindContainerExec(machinePod, "kubeadm", "reset", "--force")
}

func (r *KubernetesMachineReconciler) createMachinePod(cluster *clusterv1.Cluster, machine *clusterv1.Machine, kubernetesMachine *capkv1.KubernetesMachine) (ctrl.Result, error) {
	if util.IsControlPlaneMachine(machine) {
		return r.createControlPlaneMachinePod(cluster, machine, kubernetesMachine)
	}
	return r.createWorkerMachinePod(cluster, machine, kubernetesMachine)
}

func (r *KubernetesMachineReconciler) createControlPlaneMachinePod(cluster *clusterv1.Cluster, machine *clusterv1.Machine, kubernetesMachine *capkv1.KubernetesMachine) (ctrl.Result, error) {

	machinePod, err := r.getMachinePodBase(cluster, kubernetesMachine)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Set control plane label
	if machinePod.Labels == nil {
		machinePod.Labels = map[string]string{}
	}
	machinePod.Labels[clusterv1.MachineControlPlaneLabelName] = "true"

	// Set persistent volume claims
	err = r.updateStorage(kubernetesMachine, machinePod)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Set kind container
	kindContainer := setKindContainerBase(machine, machinePod)

	// Set readiness probe
	// The health check for an apiserver is a TCP connection check on its listening port
	// https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/high-availability/#create-load-balancer-for-kube-apiserver
	if kindContainer.ReadinessProbe == nil {
		kindContainer.ReadinessProbe = &corev1.Probe{
			PeriodSeconds: 3,
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(int(apiServerPort(cluster))),
				},
			},
		}
	}

	// Set apiserver container port
	apiServerContainerPortMissing := true
	for _, containerPort := range kindContainer.Ports {
		if containerPort.Name == apiServerPortName {
			apiServerContainerPortMissing = false
		}
	}
	if apiServerContainerPortMissing {
		apiServerContainerPort := corev1.ContainerPort{
			Name:          apiServerPortName,
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: apiServerPort(cluster),
		}
		kindContainer.Ports = append(kindContainer.Ports, apiServerContainerPort)
	}

	return ctrl.Result{}, r.Create(context.TODO(), machinePod)
}

func (r *KubernetesMachineReconciler) createWorkerMachinePod(cluster *clusterv1.Cluster, machine *clusterv1.Machine, kubernetesMachine *capkv1.KubernetesMachine) (ctrl.Result, error) {

	machinePod, err := r.getMachinePodBase(cluster, kubernetesMachine)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Set persistent volume claims
	err = r.updateStorage(kubernetesMachine, machinePod)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Set kind container
	setKindContainerBase(machine, machinePod)

	return ctrl.Result{}, r.Create(context.TODO(), machinePod)
}

func (r *KubernetesMachineReconciler) getMachinePodBase(cluster *clusterv1.Cluster, kubernetesMachine *capkv1.KubernetesMachine) (*corev1.Pod, error) {

	machinePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machinePodName(kubernetesMachine),
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				clusterv1.MachineClusterLabelName: cluster.Name,
			},
		},
		// TODO: work out why using `kubernetesMachine.Spec.PodSpec` causes
		// updates to the kubernetesMachine resource
		Spec: *kubernetesMachine.Spec.PodSpec.DeepCopy(),
	}

	// Never restart
	// TODO: revist this for extra containers e.g. sidecars
	machinePod.Spec.RestartPolicy = corev1.RestartPolicyNever

	// Set volumes
	// Inspired by kind's defaults
	// https://github.com/kubernetes-sigs/kind/blob/c8a82d8570b989988626c3f722f3a10c675f01f7/pkg/cluster/internal/providers/docker/provision.go#L167-L176
	tmpVolumeMissing := true
	runVolumeMissing := true
	libModulesVolumeMissing := true
	varLibVolumeMissing := true
	varLogVolumeMissing := true
	cloudInitScriptsVolumeMissing := true
	cloudInitSystemdUnitsVolumeMissing := true
	for _, volume := range machinePod.Spec.Volumes {
		if volume.Name == tmpVolumeName {
			tmpVolumeMissing = false
		}
		if volume.Name == runVolumeName {
			runVolumeMissing = false
		}
		if volume.Name == libModulesVolumeName {
			libModulesVolumeMissing = false
		}
		if volume.Name == varLibVolumeName {
			varLibVolumeMissing = false
		}
		if volume.Name == varLogVolumeName {
			varLogVolumeMissing = false
		}
		if volume.Name == cloudInitScriptsVolumeName {
			cloudInitScriptsVolumeMissing = false
		}
		if volume.Name == cloudInitSystemdUnitsVolume {
			cloudInitSystemdUnitsVolumeMissing = false
		}
	}
	if tmpVolumeMissing {
		tmpVolume := corev1.Volume{
			Name: tmpVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: "Memory",
				},
			},
		}
		machinePod.Spec.Volumes = append(machinePod.Spec.Volumes, tmpVolume)
	}
	if runVolumeMissing {
		runVolume := corev1.Volume{
			Name: runVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: "Memory",
				},
			},
		}
		machinePod.Spec.Volumes = append(machinePod.Spec.Volumes, runVolume)
	}
	if libModulesVolumeMissing {
		directory := corev1.HostPathDirectory
		libModulesVolume := corev1.Volume{
			Name: libModulesVolumeName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: libModulesVolumeMountPath,
					Type: &directory,
				},
			},
		}
		machinePod.Spec.Volumes = append(machinePod.Spec.Volumes, libModulesVolume)
	}
	if varLibVolumeMissing {
		varLibVolume := corev1.Volume{
			Name: varLibVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
		machinePod.Spec.Volumes = append(machinePod.Spec.Volumes, varLibVolume)
	}
	if varLogVolumeMissing {
		varLogVolume := corev1.Volume{
			Name: varLogVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
		machinePod.Spec.Volumes = append(machinePod.Spec.Volumes, varLogVolume)
	}
	if cloudInitScriptsVolumeMissing {
		cloudInitScriptsVolume := corev1.Volume{
			Name: cloudInitScriptsVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: machinePodName(kubernetesMachine) + "-cloud-init",
					Items: []corev1.KeyToPath{
						{
							Key:  cloudInitBootstrapScriptName,
							Path: cloudInitBootstrapScriptName,
						},
						{
							Key:  cloudInitInstallScriptName,
							Path: cloudInitInstallScriptName,
						},
					},
					DefaultMode: pointer.Int32Ptr(0500),
				},
			},
		}
		machinePod.Spec.Volumes = append(machinePod.Spec.Volumes, cloudInitScriptsVolume)
	}
	if cloudInitSystemdUnitsVolumeMissing {
		cloudInitSystemdUnitsVolume := corev1.Volume{
			Name: cloudInitSystemdUnitsVolume,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: machinePodName(kubernetesMachine) + "-cloud-init",
					Items: []corev1.KeyToPath{
						{
							Key:  cloudInitSystemdServiceUnitName,
							Path: cloudInitSystemdServiceUnitName,
						},
						{
							Key:  cloudInitSystemdPathUnitName,
							Path: cloudInitSystemdPathUnitName,
						},
					},
				},
			},
		}
		machinePod.Spec.Volumes = append(machinePod.Spec.Volumes, cloudInitSystemdUnitsVolume)
	}

	// Set controller reference
	if err := controllerutil.SetControllerReference(kubernetesMachine, machinePod, r.Scheme); err != nil {
		return nil, err
	}

	return machinePod, nil
}

func setKindContainerBase(machine *clusterv1.Machine, machinePod *corev1.Pod) *corev1.Container {

	// Find or create kind container
	var kindContainer *corev1.Container
	index := -1
	for index, container := range machinePod.Spec.Containers {
		if container.Name == kindContainerName {
			kindContainer = &machinePod.Spec.Containers[index]
		}
	}
	if kindContainer == nil {
		machinePod.Spec.Containers = append(machinePod.Spec.Containers, corev1.Container{})
		kindContainer = &machinePod.Spec.Containers[index+1]
	}

	// Set name
	kindContainer.Name = kindContainerName

	// Set image
	// TODO: allow custom image
	kindContainer.Image = machinePodImage(machine)

	// Ensure machine pod is not best effort
	// https://github.com/rancher/k3s/issues/1164#issuecomment-564301272
	// TODO: remove this when bug is fixed
	if utils.GetPodQOS(machinePod) == corev1.PodQOSBestEffort {
		kindContainer.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: *resource.NewMilliQuantity(1, resource.DecimalSI),
			},
		}
	}

	// Set privileged
	if kindContainer.SecurityContext == nil {
		kindContainer.SecurityContext = &corev1.SecurityContext{}
	}
	if kindContainer.SecurityContext.Privileged == nil {
		kindContainer.SecurityContext.Privileged = pointer.BoolPtr(true)
	}

	// Set volume mounts
	setVolumeMount(kindContainer, tmpVolumeName, tmpVolumeMountPath, "", false)
	setVolumeMount(kindContainer, runVolumeName, runVolumeMountPath, "", false)
	setVolumeMount(kindContainer, libModulesVolumeName, libModulesVolumeMountPath, "", true)
	setVolumeMount(kindContainer, varLibVolumeName, varLibVolumeMountPath, "", false)
	setVolumeMount(kindContainer, varLogVolumeName, varLogVolumeMountPath, "", false)
	setVolumeMount(kindContainer, cloudInitScriptsVolumeName, cloudInitScriptsVolumeMountPath, "", false)
	setVolumeMount(kindContainer, cloudInitSystemdUnitsVolume, path.Join(etcSystemdSystem, cloudInitSystemdServiceUnitName), cloudInitSystemdServiceUnitName, false)
	setVolumeMount(kindContainer, cloudInitSystemdUnitsVolume, path.Join(etcSystemdSystem, cloudInitSystemdPathUnitName), cloudInitSystemdPathUnitName, false)

	return kindContainer
}
