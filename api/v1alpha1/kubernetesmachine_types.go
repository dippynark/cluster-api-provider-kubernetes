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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

const (
	// KubernetesMachineFinalizer allows ReconcileKubernetesMachine to clean up
	// resources associated with KubernetesMachine before removing it from the
	// API Server.
	KubernetesMachineFinalizer = "kubernetesmachine.infrastructure.lukeaddison.co.uk"
)

// KubernetesMachineSpec defines the desired state of KubernetesMachine
type KubernetesMachineSpec struct {
	// ProviderID is in the form
	// `kubernetes://<podUID>`.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`
	// PodSpec forms the base of the Pod corresponding to the KubernetesMachine.
	corev1.PodSpec `json:",inline"`
	// VolumeClaimTemplates is a list of claims that PodSpec is allowed to
	// reference as volumeMounts. This has similar semantics to StatefulSet
	// volumeClaimTemplates.
	// +optional
	VolumeClaimTemplates []corev1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
}

// KubernetesMachineStatus defines the observed state of KubernetesMachine.
type KubernetesMachineStatus struct {
	// ErrorReason will be set in the event that there is a terminal problem
	// reconciling the KubernetesMachine and will contain a succinct value
	// suitable for machine interpretation.
	// +optional
	ErrorReason *capierrors.MachineStatusError `json:"errorReason,omitempty"`

	// ErrorMessage will be set in the event that there is a terminal problem
	// reconciling the KubernetesMachine and will contain a more verbose string
	// suitable for logging and human consumption.
	// +optional
	ErrorMessage *string `json:"errorMessage,omitempty"`

	// Phase represents the current phase of KubernetesMachine actuation.
	// +optional
	Phase KubernetesMachinePhase `json:"phase,omitempty"`

	// Ready denotes that the KubernetesMachine is ready.
	// +optional
	Ready bool `json:"ready"`
}

// SetErrorReason sets the KubernetesMachine error reason.
func (s *KubernetesMachineStatus) SetErrorReason(v capierrors.MachineStatusError) {
	s.ErrorReason = &v
}

// SetErrorMessage sets the KubernetesMachine error message.
func (s *KubernetesMachineStatus) SetErrorMessage(v error) {
	s.ErrorMessage = pointer.StringPtr(v.Error())
}

// KubernetesMachinePhase describes the state of a KubernetesMachine.
//
// This type is a high-level indicator of the status of the KuberntesMachine as it is provisioned,
// from the API user’s perspective. The value should not be interpreted by any software components
// as a reliable indication of the actual state of the KuberntesMachine.
type KubernetesMachinePhase string

// These are the valid statuses of KubernetesMachines.
const (
	// KubernetesMachinePhasePending is the first state a KubernetesMachine is
	// assigned after being created.
	KubernetesMachinePhasePending KubernetesMachinePhase = "Pending"

	// KubernetesMachinePhaseProvisioning is the state when the ProviderID has
	// been set.
	KubernetesMachinePhaseProvisioning KubernetesMachinePhase = "Provisioning"

	// KubernetesMachinePhaseRunning is the state when the ProviderID has been
	// set and the KubernetesMachine is ready.
	KubernetesMachinePhaseRunning KubernetesMachinePhase = "Running"

	// KubernetesMachinePhaseFailed is the state when the system might require
	// manual intervention.
	KubernetesMachinePhaseFailed KubernetesMachinePhase = "Failed"

	// KubernetesMachinePhaseDeleting is the state when a delete request has
	// been sent to the API Server.
	KubernetesMachinePhaseDeleting KubernetesMachinePhase = "Deleting"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=cluster-api
// +kubebuilder:printcolumn:name="provider-id",type="string",JSONPath=".spec.providerID",description="Provider ID"
// +kubebuilder:printcolumn:name="phase",type="string",JSONPath=".status.phase",description="KubernetesMachine status"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"

// KubernetesMachine is the Schema for the kubernetesmachines API.
type KubernetesMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesMachineSpec   `json:"spec,omitempty"`
	Status KubernetesMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KubernetesMachineList contains a list of KubernetesMachine.
type KubernetesMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubernetesMachine{}, &KubernetesMachineList{})
}
