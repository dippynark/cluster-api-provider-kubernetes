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

package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var kubernetesmachinelog = logf.Log.WithName("kubernetesmachine-resource")

func (r *KubernetesMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-infrastructure-dippynark-co-uk-v1alpha2-kubernetesmachine,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.dippynark.co.uk,resources=kubernetesmachines,verbs=create;update,versions=v1alpha2,name=mkubernetesmachine.kb.io,sideEffects=None,admissionReviewVersions=v1beta1

var _ webhook.Defaulter = &KubernetesMachine{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *KubernetesMachine) Default() {
	kubernetesmachinelog.Info("default", "name", r.Name)

	// Work around for containers being required in the PodSpec struct
	if r.Spec.Containers == nil {
		r.Spec.Containers = []corev1.Container{}
	}
}
