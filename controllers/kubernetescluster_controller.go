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
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha1"
)

const (
	clusterControllerName = "KubernetesCluster-controller"
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
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;delete

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

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(kubernetesCluster, r)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to Patch the KubernetesCluster object and status after each reconciliation.
	defer func() {
		if err := patchHelper.Patch(ctx, kubernetesCluster); err != nil {
			log.Error(err, "failed to patch KubernetesCluster")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// Handle deleted clusters
	if !kubernetesCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(cluster, kubernetesCluster)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(cluster, kubernetesCluster)
}

func (r *KubernetesClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.KubernetesCluster{}).
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.ClusterToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("KubernetesCluster")),
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
	if ref == nil || (ref.Kind != "KubernetesCluster" || ref.APIVersion != infrav1.GroupVersion.String()) {
		return nil
	}
	log.Info(fmt.Sprintf("Found Service owned by KubernetesCluster %s/%s", s.Namespace, ref.Name))
	name := client.ObjectKey{Namespace: s.Namespace, Name: ref.Name}
	result = append(result, ctrl.Request{NamespacedName: name})

	return result

}

func (r *KubernetesClusterReconciler) reconcileNormal(cluster *clusterv1.Cluster, kubernetesCluster *capkv1.KubernetesCluster) (ctrl.Result, error) {
	// If the KubernetesCluster doesn't have finalizer, add it.
	if !util.Contains(kubernetesCluster.Finalizers, clusterv1.ClusterFinalizer) {
		kubernetesCluster.Finalizers = append(kubernetesCluster.Finalizers, clusterv1.ClusterFinalizer)
	}

	// Create load balancer service
	clusterService := &corev1.Service{}
	err := r.Get(context.TODO(), types.NamespacedName{
		Namespace: kubernetesCluster.Namespace,
		Name:      clusterServiceName(cluster),
	}, clusterService)
	if k8serrors.IsNotFound(err) {
		return r.createClusterService(cluster, kubernetesCluster)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	// Ensure load balancer is controlled by kubernetes cluster
	if ref := metav1.GetControllerOf(clusterService); ref == nil || ref.UID != kubernetesCluster.UID {
		return ctrl.Result{}, errors.Errorf("expected Service %s in Namespace %s to be controlled by KubernetesCluster %s", clusterService.Name, clusterService.Namespace, kubernetesCluster.Name)
	}

	// Update api endpoints
	host := clusterService.Spec.ClusterIP
	// TODO: currently this is certainly true as we create the service
	if clusterService.Spec.Type == corev1.ServiceTypeLoadBalancer {
		// TODO: consider other elements of ingress array
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
	kubernetesCluster.Status.APIEndpoints = []infrav1.APIEndpoint{
		{
			Host: host,
			Port: int(clusterService.Spec.Ports[0].Port),
		},
	}

	// Mark the kubernetesCluster ready
	kubernetesCluster.Status.Ready = true

	return ctrl.Result{}, nil
}

func (r *KubernetesClusterReconciler) reconcileDelete(cluster *clusterv1.Cluster, kubernetesCluster *capkv1.KubernetesCluster) (ctrl.Result, error) {
	// Delete load balancer service
	clusterService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterServiceName(cluster),
			Namespace: cluster.Namespace,
		},
	}
	err := r.Delete(context.TODO(), clusterService)
	if err != nil && !k8serrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// KubernetesCluster is deleted so remove the finalizer.
	kubernetesCluster.Finalizers = util.Filter(kubernetesCluster.Finalizers, clusterv1.ClusterFinalizer)

	return ctrl.Result{}, nil
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
					Port:       443,
					TargetPort: intstr.FromInt(6443),
				},
			},
			Type: corev1.ServiceTypeLoadBalancer,
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
