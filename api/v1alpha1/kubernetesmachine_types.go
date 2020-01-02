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
	// MachineFinalizer allows ReconcileKubernetesMachine to clean up resources
	// associated with KubernetesMachine before removing it from the apiserver.
	MachineFinalizer = "kubernetesmachine.infrastructure.lukeaddison.co.uk"
)

// KubernetesMachineSpec defines the desired state of KubernetesMachine
type KubernetesMachineSpec struct {
	// ProviderID is in the form `kubernetes://<namespace>/<clusterName>/<machineName>`
	// +optional
	ProviderID     *string `json:"providerID,omitempty"`
	corev1.PodSpec `json:",inline"`
	// +optional
	VolumeClaimTemplates []corev1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
}

// KubernetesMachineStatus defines the observed state of KubernetesMachine
type KubernetesMachineStatus struct {
	// Ready denotes that the machine (kubernetes pod) is ready
	// +optional
	Ready bool `json:"ready"`

	// Phase represents the current phase of kubernetesMachine actuation.
	// E.g. Provisioning, Running, Failed, Terminated etc.
	// +optional
	Phase *KubernetesMachinePhase `json:"phase,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	FailureReason *capierrors.MachineStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`
}

// KubernetesMachinePhase describes the state of a Machine Pod
type KubernetesMachinePhase string

// These are the valid statuses of KubernetesMachines
const (
	// KubernetesMachinePhaseProvisioning is when the Machine Pod has been
	// created
	KubernetesMachinePhaseProvisioning KubernetesMachinePhase = "Provisioning"

	// KubernetesMachinePhaseRunning is when the Machine Pod's kind container is
	// running and the bootstrap process has been enabled
	KubernetesMachinePhaseRunning KubernetesMachinePhase = "Running"

	// KubernetesMachinePhaseFailed is when a failure has occurred
	KubernetesMachinePhaseFailed KubernetesMachinePhase = "Failed"

	// KubernetesMachinePhaseTerminated is when the kind container has terminated
	KubernetesMachinePhaseTerminated KubernetesMachinePhase = "Terminated"
)

// GetPhase sets the KubernetesMachine phase
func (s *KubernetesMachineStatus) GetPhase() KubernetesMachinePhase {
	return *s.Phase
}

// SetPhase sets the KubernetesMachine phase
func (s *KubernetesMachineStatus) SetPhase(p KubernetesMachinePhase) {
	s.Phase = &p
}

// SetFailureReason sets the KubernetesMachine failure reason
func (s *KubernetesMachineStatus) SetFailureReason(v capierrors.MachineStatusError) {
	s.FailureReason = &v
}

// SetFailureMessage sets the KubernetesMachine failure message
func (s *KubernetesMachineStatus) SetFailureMessage(v error) {
	s.FailureMessage = pointer.StringPtr(v.Error())
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
