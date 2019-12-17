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

// KubernetesMachineTemplateSpec defines the desired state of KubernetesMachineTemplate
type KubernetesMachineTemplateSpec struct {
	Template KubernetesMachineTemplateResource `json:"template"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=cluster-api

// KubernetesMachineTemplate is the Schema for the kubernetesmachinetemplates API
type KubernetesMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec KubernetesMachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// KubernetesMachineTemplateList contains a list of KubernetesMachineTemplate
type KubernetesMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesMachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubernetesMachineTemplate{}, &KubernetesMachineTemplateList{})
}

// KubernetesMachineTemplateResource describes the data needed to create a KubernetesMachine from a template
type KubernetesMachineTemplateResource struct {
	// Spec is the specification of the desired behavior of the machine.
	Spec KubernetesMachineSpec `json:"spec"`
}
