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
	KubernetesMachineFinalizer = "kubernetesmachine.infrastructure.lukeaddison.co.uk"
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
	// ErrorReason indicates that there is a problem reconciling the
	// state, and will be set to a token value suitable for
	// programmatic interpretation.
	// +optional
	ErrorReason *capierrors.MachineStatusError `json:"errorReason,omitempty"`

	// ErrorMessage indicates that there is a problem reconciling the
	// state, and will be set to a descriptive error message.
	// +optional
	ErrorMessage *string `json:"errorMessage,omitempty"`

	// Phase represents the current phase of kubernetesMachine actuation.
	// e.g. Provisioning, Running, Failed, Terminated etc.
	// +optional
	Phase KubernetesMachinePhase `json:"phase,omitempty"`

	// Ready denotes that the machine (kubernetes pod) is ready.
	// +optional
	Ready bool `json:"ready"`

	// PodName is the name of the Pod corresponding to the KubernetesMachine.
	PodName *string `json:"podName,omitempty"`
}

// KubernetesMachinePhase describes the state of a Machine Pod
type KubernetesMachinePhase string

// These are the valid statuses of KubernetesMachines
const (
	// KubernetesMachinePhasePending is when Machine Pod hasn't been created yet
	KubernetesMachinePhasePending KubernetesMachinePhase = "Pending"

	// KubernetesMachinePhaseProvisioning is when the Machine Pod is being
	// provisioned
	KubernetesMachinePhaseProvisioning KubernetesMachinePhase = "Provisioning"

	// KubernetesMachinePhaseProvisioned is when the Machine Pod has been
	// provisioned
	KubernetesMachinePhaseProvisioned KubernetesMachinePhase = "Provisioned"

	// KubernetesMachinePhaseRunning is when the Machine Pod's kind container is
	// running and the bootstrap process has been enabled
	KubernetesMachinePhaseRunning KubernetesMachinePhase = "Running"

	// KubernetesMachinePhaseDeleting is when the machine pod is being deleted
	KubernetesMachinePhaseDeleting KubernetesMachinePhase = "Deleting"

	// KubernetesMachinePhaseFailed is when a failure has occurred
	KubernetesMachinePhaseFailed KubernetesMachinePhase = "Failed"
)

// SetErrorReason sets the KubernetesCluster failure reason
func (s *KubernetesMachineStatus) SetErrorReason(v capierrors.MachineStatusError) {
	s.ErrorReason = &v
}

// SetErrorMessage sets the KubernetesCluster failure message
func (s *KubernetesMachineStatus) SetErrorMessage(v error) {
	s.ErrorMessage = pointer.StringPtr(v.Error())
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=cluster-api
// +kubebuilder:printcolumn:name="provider-id",type="string",JSONPath=".spec.providerID",description="Provider ID"
// +kubebuilder:printcolumn:name="phase",type="string",JSONPath=".status.phase",description="KubernetesMachine status such as Provisioning/Running/Failed etc."
// +kubebuilder:printcolumn:name="pod-name",type="string",JSONPath=".status.podName",description="The name of the Pod corresponding to the KubernetesMachine"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"

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
