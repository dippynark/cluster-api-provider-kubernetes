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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// MachineFinalizer allows ReconcileKubernetesMachine to clean up resources
	// associated with KubernetesMachine before removing it from the apiserver.
	MachineFinalizer = "kubernetesmachine.infrastructure.lukeaddison.co.uk"
)

// KubernetesMachineSpec defines the desired state of KubernetesMachine
type KubernetesMachineSpec struct {
	// ProviderID will be the pod uid in ProviderID format
	// (kubernetes:////<poduid>)
	// +optional
	ProviderID *string `json:"providerID,omitempty"`
}

// KubernetesMachineStatus defines the observed state of KubernetesMachine
type KubernetesMachineStatus struct {
	// Ready denotes that the machine (kubernetes pod) is ready
	Ready bool `json:"ready"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=cluster-api

// KubernetesMachine is the Schema for the kubernetesmachines API
type KubernetesMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesMachineSpec   `json:"spec,omitempty"`
	Status KubernetesMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KubernetesMachineList contains a list of KubernetesMachine
type KubernetesMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubernetesMachine{}, &KubernetesMachineList{})
}
