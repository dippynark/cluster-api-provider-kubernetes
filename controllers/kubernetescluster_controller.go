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
	"time"

	capkv1alpha2 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha2"
	capkv1 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha3"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	clusterControllerName           = "KubernetesCluster-controller"
	apiServerPortName               = "kube-apiserver"
	loadBalancerIngressRequeueAfter = time.Second * 1
	defaultClusterLoadBalancerPort  = 443
)

// KubernetesClusterReconciler reconciles a KubernetesCluster object
type KubernetesClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.lukeaddison.co.uk,resources=kubernetesclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.lukeaddison.co.uk,resources=kubernetesclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create

func (r *KubernetesClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {
	ctx := context.Background()
	log := log.Log.WithValues(namespaceLogName, req.Namespace, kubernetesClusterLogName, req.Name)

	// Fetch the kubernetes cluster instance
	kubernetesCluster := &capkv1.KubernetesCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, kubernetesCluster); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(kubernetesCluster, r)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to patch the kubernetes cluster object and status after each reconciliation
	defer func() {
		r.reconcilePhase(kubernetesCluster)

		if err := patchHelper.Patch(ctx, kubernetesCluster); err != nil {
			log.Error(err, "failed to patch KubernetesCluster")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// TODO: move defaulting into webhook
	if kubernetesCluster.Spec.ControlPlaneServiceType == "" {
		kubernetesCluster.Spec.ControlPlaneServiceType = corev1.ServiceTypeClusterIP
	}

	// Fetch the cluster
	cluster, err := util.GetOwnerCluster(ctx, r.Client, kubernetesCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("Waiting for Cluster Controller to set OwnerRef on KubernetesCluster")
		return ctrl.Result{}, nil
	}

	log = log.WithValues(clusterLogName, cluster.Name)

	// Handle deleted clusters
	if !kubernetesCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(kubernetesCluster)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, cluster, kubernetesCluster)
}

func (r *KubernetesClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capkv1.KubernetesCluster{}).
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.ClusterToInfrastructureMapFunc(capkv1.GroupVersion.WithKind("KubernetesCluster")),
			},
		).
		// TODO: fix ClusterToInfrastructureMapFunc to allow watches on multiple versions
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.ClusterToInfrastructureMapFunc(capkv1alpha2.GroupVersion.WithKind("KubernetesCluster")),
			},
		).
		Watches(
			&source.Kind{Type: &corev1.Service{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(r.ServiceToKubernetesCluster),
			},
		).
		Complete(r)
}

// ServiceToKubernetesCluster is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of KubernetesClusters
func (r *KubernetesClusterReconciler) ServiceToKubernetesCluster(o handler.MapObject) []ctrl.Request {
	result := []ctrl.Request{}
	s, ok := o.Object.(*corev1.Service)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Service but got a %T", o.Object), "failed to get KubernetesCluster for Service")
		return nil
	}
	log := r.Log.WithValues(namespaceLogName, s.Namespace, serviceLogName, s.Name)

	// Only watch services owned by a kubernetes clusters
	ref := metav1.GetControllerOf(s)
	if ref == nil || (ref.Kind != "KubernetesCluster" || ref.APIVersion != capkv1.GroupVersion.String()) {
		return nil
	}
	log = log.WithValues(kubernetesClusterLogName, ref.Name)
	log.Info("Found Service owned by KubernetesCluster")
	name := client.ObjectKey{Namespace: s.Namespace, Name: ref.Name}
	result = append(result, ctrl.Request{NamespacedName: name})

	return result

}

func (r *KubernetesClusterReconciler) reconcileNormal(ctx context.Context, cluster *clusterv1.Cluster, kubernetesCluster *capkv1.KubernetesCluster) (ctrl.Result, error) {
	log := r.Log.WithValues(namespaceLogName, cluster.Namespace, clusterLogName, cluster.Name, kubernetesClusterLogName, kubernetesCluster.Name)

	// If the kubernetes cluster does not have finalizer, add it.
	if !util.Contains(kubernetesCluster.Finalizers, capkv1.KubernetesClusterFinalizer) {
		kubernetesCluster.Finalizers = append(kubernetesCluster.Finalizers, capkv1.KubernetesClusterFinalizer)
	}

	// If the kubernetes cluster does not have the foregroundDeletion finalizer, add it
	if !util.Contains(kubernetesCluster.Finalizers, metav1.FinalizerDeleteDependents) {
		kubernetesCluster.Finalizers = append(kubernetesCluster.Finalizers, metav1.FinalizerDeleteDependents)
	}

	// Get or create load balancer service
	clusterService := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: kubernetesCluster.Namespace,
		Name:      clusterServiceName(cluster),
	}, clusterService)
	if k8serrors.IsNotFound(err) {
		return r.createClusterService(ctx, cluster, kubernetesCluster)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	// Ensure load balancer is controlled by kubernetes cluster
	if ref := metav1.GetControllerOf(clusterService); ref == nil || ref.UID != kubernetesCluster.UID {
		kubernetesCluster.Status.SetFailureReason(capierrors.UnsupportedChangeClusterError)
		err := errors.Errorf("expected Service %s to be controlled by KubernetesCluster", clusterService.Name)
		kubernetesCluster.Status.SetFailureMessage(err)
		return ctrl.Result{}, err
	}

	// TODO: use validating webhook to check service configuration is still valid

	// Retrieve service host
	host := clusterService.Spec.ClusterIP
	if clusterService.Spec.Type == corev1.ServiceTypeLoadBalancer {
		// TODO: consider all elements of ingress array
		if len(clusterService.Status.LoadBalancer.Ingress) == 0 {
			log.Info("Waiting for load balancer to be provisioned")
			return ctrl.Result{RequeueAfter: loadBalancerIngressRequeueAfter}, nil
		}
		loadBalancerHost := clusterService.Status.LoadBalancer.Ingress[0].Hostname
		if loadBalancerHost == "" {
			loadBalancerHost = clusterService.Status.LoadBalancer.Ingress[0].IP
		}
		if loadBalancerHost == "" {
			log.Info("Waiting for load balancer hostname or IP address")
			return ctrl.Result{RequeueAfter: loadBalancerIngressRequeueAfter}, nil
		}
		host = loadBalancerHost
	}

	// Retrieve service port
	var port int32
	for _, servicePort := range clusterService.Spec.Ports {
		if servicePort.TargetPort.String() == apiServerPortName {
			port = servicePort.Port
		}
	}

	// Set controlPlaneEndpoint host and port
	if kubernetesCluster.Spec.ControlPlaneEndpoint.IsZero() {
		kubernetesCluster.Spec.ControlPlaneEndpoint.Host = host
		kubernetesCluster.Spec.ControlPlaneEndpoint.Port = port
	}

	// Make sure endpoint has not changed
	if kubernetesCluster.Spec.ControlPlaneEndpoint.Host != host || kubernetesCluster.Spec.ControlPlaneEndpoint.Port != port {
		kubernetesCluster.Status.SetFailureReason(capierrors.UnsupportedChangeClusterError)
		kubernetesCluster.Status.SetFailureMessage(errors.Errorf("Service endpoint has changed"))
		return ctrl.Result{}, nil
	}

	// Mark the kubernetesCluster ready
	// TODO: should this ever go back to no-ready if the Service is deleted for example?
	kubernetesCluster.Status.Ready = true

	return ctrl.Result{}, nil
}

func (r *KubernetesClusterReconciler) reconcileDelete(kubernetesCluster *capkv1.KubernetesCluster) (ctrl.Result, error) {

	// KubernetesCluster is deleted so remove the finalizer
	// Rely on garbage collection to delete load balancer service
	kubernetesCluster.Finalizers = util.Filter(kubernetesCluster.Finalizers, capkv1.KubernetesClusterFinalizer)

	return ctrl.Result{}, nil
}

func (r *KubernetesClusterReconciler) reconcilePhase(k *capkv1.KubernetesCluster) {
	// Set phase to "Provisioning" if nil
	if k.Status.Phase == "" {
		k.Status.Phase = capkv1.KubernetesClusterPhaseProvisioning
	}

	// Set phase to "Provisioned" if the KubernetesCluster is ready
	if k.Status.Ready {
		k.Status.Phase = capkv1.KubernetesClusterPhaseProvisioned
	}

	// Set phase to "Failed" if any of Status.FailureReason or Status.FailureMessage
	// is not-nil
	if k.Status.FailureReason != nil || k.Status.FailureMessage != nil {
		k.Status.Phase = capkv1.KubernetesClusterPhaseFailed
	}

	// Set phase to "Deleting" if the deletion timestamp is set
	if !k.DeletionTimestamp.IsZero() {
		k.Status.Phase = capkv1.KubernetesClusterPhaseDeleting
	}
}

func (r *KubernetesClusterReconciler) createClusterService(ctx context.Context, cluster *clusterv1.Cluster, kubernetesCluster *capkv1.KubernetesCluster) (ctrl.Result, error) {

	// Attempt to acquire specified host
	desiredClusterIP := ""
	desiredLoadBalancerIP := ""
	switch kubernetesCluster.Spec.ControlPlaneServiceType {
	case corev1.ServiceTypeClusterIP:
		desiredClusterIP = kubernetesCluster.Spec.ControlPlaneEndpoint.Host
	case corev1.ServiceTypeLoadBalancer:
		desiredLoadBalancerIP = kubernetesCluster.Spec.ControlPlaneEndpoint.Host
	}

	// Set specified port
	desiredPort := int32(defaultClusterLoadBalancerPort)
	if kubernetesCluster.Spec.ControlPlaneEndpoint.Port != 0 {
		desiredPort = kubernetesCluster.Spec.ControlPlaneEndpoint.Port
	}

	clusterService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterServiceName(cluster),
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				clusterv1.ClusterLabelName: cluster.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				clusterv1.ClusterLabelName:             cluster.Name,
				clusterv1.MachineControlPlaneLabelName: "",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Protocol:   "TCP",
					Port:       desiredPort,
					TargetPort: intstr.FromString(apiServerPortName),
				},
			},
			Type:           kubernetesCluster.Spec.ControlPlaneServiceType,
			ClusterIP:      desiredClusterIP,
			LoadBalancerIP: desiredLoadBalancerIP,
		},
	}
	if err := controllerutil.SetControllerReference(kubernetesCluster, clusterService, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, r.Create(ctx, clusterService)
}

func clusterServiceName(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("%s-lb", cluster.Name)
}
