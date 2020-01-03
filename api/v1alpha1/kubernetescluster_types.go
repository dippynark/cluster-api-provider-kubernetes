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
	// ClusterFinalizer allows KubernetesClusterReconciler to clean up resources
	// associated with KubernetesCluster before removing it from the apiserver.
	KubernetesClusterFinalizer = "kubernetescluster.infrastructure.lukeaddison.co.uk"
)

// KubernetesClusterSpec defines the desired state of KubernetesCluster
type KubernetesClusterSpec struct {
	// +optional
	// +kubebuilder:default="ClusterIP"
	ControlPlaneServiceType corev1.ServiceType `json:"controlPlaneServiceType,omitempty"`
}

// KubernetesClusterStatus defines the observed state of KubernetesCluster
type KubernetesClusterStatus struct {
	// ErrorReason indicates that there is a problem reconciling the
	// state, and will be set to a token value suitable for
	// programmatic interpretation.
	// +optional
	ErrorReason *capierrors.ClusterStatusError `json:"errorReason,omitempty"`

	// ErrorMessage indicates that there is a problem reconciling the
	// state, and will be set to a descriptive error message.
	// +optional
	ErrorMessage *string `json:"errorMessage,omitempty"`

	// Phase represents the current phase of kubernetesMachine actuation.
	// e.g. Provisioning, Running, Failed, Terminated etc.
	// +optional
	Phase KubernetesClusterPhase `json:"phase,omitempty"`

	// Ready denotes that the kubernetes cluster (infrastructure) is ready.
	// +optional
	Ready bool `json:"ready"`

	// APIEndpoints represents the endpoints to communicate with the control
	// plane.
	// +optional
	APIEndpoints []APIEndpoint `json:"apiEndpoints,omitempty"`

	// ServiceName is the name of the Service corresponding to the KubernetesCluster
	ServiceName *string `json:"serviceName,omitempty"`
}

// APIEndpoint represents a reachable Kubernetes API endpoint.
type APIEndpoint struct {
	// Host is the hostname on which the API server is serving.
	Host string `json:"host"`

	// Port is the port on which the API server is serving.
	Port int32 `json:"port"`
}

// KubernetesClusterPhase describes the state of a KuberntesCluster
type KubernetesClusterPhase string

// These are the valid statuses of KubernetesClusters
const (
	// KubernetesClusterPhasePending is when KubernetesCluster is waiting for
	// bootstrap data
	KubernetesClusterPhasePending KubernetesClusterPhase = "Pending"

	// KubernetesClusterPhaseProvisioning is when the Machine Pod is being
	// provisioned
	KubernetesClusterPhaseProvisioning KubernetesClusterPhase = "Provisioning"

	// KubernetesClusterPhaseProvisioned is when the Machine Pod has been
	// provisioned
	KubernetesClusterPhaseProvisioned KubernetesClusterPhase = "Provisioned"

	// KubernetesClusterPhaseDeleting is when the machine pod is being deleted
	KubernetesClusterPhaseDeleting KubernetesClusterPhase = "Deleting"

	// KubernetesClusterPhaseFailed is when a failure has occurred
	KubernetesClusterPhaseFailed KubernetesClusterPhase = "Failed"
)

// SetErrorReason sets the KubernetesMachine failure reason
func (s *KubernetesMachineStatus) SetErrorReason(v capierrors.MachineStatusError) {
	s.ErrorReason = &v
}

// SetErrorMessage sets the KubernetesMachine failure message
func (s *KubernetesMachineStatus) SetErrorMessage(v error) {
	s.ErrorMessage = pointer.StringPtr(v.Error())
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=cluster-api
// +kubebuilder:printcolumn:name="phase",type="string",JSONPath=".status.phase",description="KubernetesCluster status such as Provisioning/Provisioned/Failed etc."
// +kubebuilder:printcolumn:name="host",type="string",JSONPath=".status.apiEndpoints[0].host",description="Endpoint host for reaching the control plane"
// +kubebuilder:printcolumn:name="port",type="integer",JSONPath=".status.apiEndpoints[0].port",description="Endpoint port for reaching the control plane"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"

// KubernetesCluster is the Schema for the kubernetesclusters API
type KubernetesCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesClusterSpec   `json:"spec,omitempty"`
	Status KubernetesClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KubernetesClusterList contains a list of KubernetesCluster
type KubernetesClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubernetesCluster{}, &KubernetesClusterList{})
}
