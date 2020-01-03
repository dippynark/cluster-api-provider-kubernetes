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

	capkv1 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
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
	clusterControllerName   = "KubernetesCluster-controller"
	apiServerPortName       = "kube-apiserver"
	clusterLoadBalancerPort = 443
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
	log := log.Log.WithName(clusterControllerName).WithValues("kubernetes-cluster", req.NamespacedName)

	// Fetch the KubernetesCluster instance
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
	// Always attempt to Patch the KubernetesCluster object and status after each reconciliation.
	defer func() {
		r.reconcilePhase(kubernetesCluster)

		if err := patchHelper.Patch(ctx, kubernetesCluster); err != nil {
			log.Error(err, "failed to patch KubernetesCluster")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, kubernetesCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("Waiting for Cluster Controller to set OwnerRef on KubernetesCluster")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Handle deleted clusters
	if !kubernetesCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(kubernetesCluster)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(cluster, kubernetesCluster)
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
	log := r.Log.WithValues("Service", s.Name, "Namespace", s.Namespace)

	// Only watch services owned by a kubernetescluster
	ref := metav1.GetControllerOf(s)
	if ref == nil || (ref.Kind != "KubernetesCluster" || ref.APIVersion != capkv1.GroupVersion.String()) {
		return nil
	}
	log.Info(fmt.Sprintf("Found Service owned by KubernetesCluster %s/%s", s.Namespace, ref.Name))
	name := client.ObjectKey{Namespace: s.Namespace, Name: ref.Name}
	result = append(result, ctrl.Request{NamespacedName: name})

	return result

}

func (r *KubernetesClusterReconciler) reconcileNormal(cluster *clusterv1.Cluster, kubernetesCluster *capkv1.KubernetesCluster) (ctrl.Result, error) {
	// If the KubernetesCluster doesn't have finalizer, add it.
	if !util.Contains(kubernetesCluster.Finalizers, capkv1.KubernetesClusterFinalizer) {
		kubernetesCluster.Finalizers = append(kubernetesCluster.Finalizers, capkv1.KubernetesClusterFinalizer)
	}

	// If the KubernetesCluster doesn't have foregroundDeletion, add it.
	if !util.Contains(kubernetesCluster.Finalizers, metav1.FinalizerDeleteDependents) {
		kubernetesCluster.Finalizers = append(kubernetesCluster.Finalizers, "foregroundDeletion")
	}

	// Get or create load balancer service
	clusterService := &corev1.Service{}
	err := r.Get(context.TODO(), types.NamespacedName{
		Namespace: kubernetesCluster.Namespace,
		Name:      clusterServiceName(cluster),
	}, clusterService)
	if k8serrors.IsNotFound(err) {
		// TODO: if service is being recreated, request same ip as was originally aquired
		return r.createClusterService(cluster, kubernetesCluster)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	// Ensure load balancer is controlled by kubernetes cluster
	if ref := metav1.GetControllerOf(clusterService); ref == nil || ref.UID != kubernetesCluster.UID {
		kubernetesCluster.Status.SetErrorReason(capierrors.UnsupportedChangeClusterError)
		kubernetesCluster.Status.SetErrorMessage(errors.Errorf("Cluster Service is not controlled by KubernetesCluster"))
		return ctrl.Result{}, errors.Errorf("expected Service %s in Namespace %s to be controlled by KubernetesCluster %s", clusterService.Name, clusterService.Namespace, kubernetesCluster.Name)
	}

	// Service has been created so update KubernetesCluster service name
	if kubernetesCluster.Status.ServiceName == nil {
		kubernetesCluster.Status.ServiceName = &clusterService.Name
		return ctrl.Result{}, nil
	}

	// Check service name matches status
	// This should never not happen
	if clusterService.Name != *kubernetesCluster.Status.ServiceName {
		kubernetesCluster.Status.SetErrorReason(capierrors.UnsupportedChangeClusterError)
		kubernetesCluster.Status.SetErrorMessage(errors.Errorf("Machine Pod name has changed"))
		return ctrl.Result{}, nil
	}

	// Update load balancer type
	// TODO: Check labels, ports and selector, update if necessary
	if clusterService.Spec.Type != kubernetesCluster.Spec.ControlPlaneServiceType {
		clusterService.Spec.Type = kubernetesCluster.Spec.ControlPlaneServiceType
		return ctrl.Result{}, r.Update(context.TODO(), clusterService)
	}

	// Set api endpoint with load balancer details so the Cluster API Cluster Controller can pull it
	host := clusterService.Spec.ClusterIP
	if clusterService.Spec.Type == corev1.ServiceTypeLoadBalancer {
		// TODO: consider all elements of ingress array
		if len(clusterService.Status.LoadBalancer.Ingress) == 0 {
			r.Log.Info("Waiting for loadbalancer to be provisioned")
			return ctrl.Result{}, nil
		}
		loadBalancerHost := clusterService.Status.LoadBalancer.Ingress[0].Hostname
		if loadBalancerHost == "" {
			loadBalancerHost = clusterService.Status.LoadBalancer.Ingress[0].IP
		}
		if loadBalancerHost == "" {
			r.Log.Info("Waiting for loadbalancer hostname or IP")
			return ctrl.Result{}, nil
		}
		host = loadBalancerHost
	}
	kubernetesCluster.Status.APIEndpoints = []capkv1.APIEndpoint{
		{
			Host: host,
			Port: clusterLoadBalancerPort,
		},
	}

	// Mark the kubernetesCluster ready
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
	// Set phase to "Pending" if nil
	if k.Status.Phase == "" {
		k.Status.Phase = capkv1.KubernetesClusterPhasePending
	}

	// Set phase to "Provisioning" if the corresponding Service has been created
	if k.Status.ServiceName != nil {
		k.Status.Phase = capkv1.KubernetesClusterPhaseProvisioning
	}

	// Set phase to "Provisioned" if the KubernetesCluster is ready
	if k.Status.Ready {
		k.Status.Phase = capkv1.KubernetesClusterPhaseProvisioned
	}

	// Set phase to "Failed" if any of Status.ErrorReason or Status.ErrorMessage
	// is not-nil
	if k.Status.ErrorReason != nil || k.Status.ErrorMessage != nil {
		k.Status.Phase = capkv1.KubernetesClusterPhaseFailed
	}

	// Set phase to "Deleting" if the deletion timestamp is set
	if !k.DeletionTimestamp.IsZero() {
		k.Status.Phase = capkv1.KubernetesClusterPhaseDeleting
	}
}

func (r *KubernetesClusterReconciler) createClusterService(cluster *clusterv1.Cluster, kubernetesCluster *capkv1.KubernetesCluster) (ctrl.Result, error) {

	clusterService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterServiceName(cluster),
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				clusterv1.MachineClusterLabelName: cluster.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				clusterv1.MachineClusterLabelName:      cluster.Name,
				clusterv1.MachineControlPlaneLabelName: "true",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Protocol:   "TCP",
					Port:       clusterLoadBalancerPort,
					TargetPort: intstr.FromString(apiServerPortName),
				},
			},
			Type: kubernetesCluster.Spec.ControlPlaneServiceType,
		},
	}
	if err := controllerutil.SetControllerReference(kubernetesCluster, clusterService, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, r.Create(context.TODO(), clusterService)
}

func clusterServiceName(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("%s-lb", cluster.Name)
}
