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

package v1alpha3

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

const (
	// KubernetesClusterFinalizer allows KubernetesClusterReconciler to clean up
	// resources associated with a KubernetesCluster before removing it from the
	// API Server.
	KubernetesClusterFinalizer = "kubernetescluster.infrastructure.lukeaddison.co.uk"
)

// KubernetesClusterSpec defines the desired state of KubernetesCluster
type KubernetesClusterSpec struct {
	// ControlPlaneServiceType is the type of Service used for the control plane
	// load balancer.
	// +optional
	// +kubebuilder:validation:Enum=ClusterIP;LoadBalancer
	ControlPlaneServiceType corev1.ServiceType `json:"controlPlaneServiceType,omitempty"`
	// TODO: default to ClusterIP

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint APIEndpoint `json:"controlPlaneEndpoint"`
}

// KubernetesClusterStatus defines the observed state of a KubernetesCluster.
type KubernetesClusterStatus struct {
	// ErrorReason will be set in the event that there is a terminal problem
	// reconciling the KubernetesCluster and will contain a succinct value
	// suitable for machine interpretation.
	// +optional
	FailureReason *capierrors.ClusterStatusError `json:"errorReason,omitempty"`

	// ErrorMessage will be set in the event that there is a terminal problem
	// reconciling the KubernetesCluster and will contain a more verbose string
	// suitable for logging and human consumption.
	// +optional
	FailureMessage *string `json:"errorMessage,omitempty"`

	// Phase represents the current phase of KubernetesCluster actuation.
	// +optional
	Phase KubernetesClusterPhase `json:"phase,omitempty"`

	// Ready denotes that the KubernetesCluster is ready.
	// +optional
	Ready bool `json:"ready"`
}

// SetErrorReason sets the KubernetesCluster error reason.
func (s *KubernetesClusterStatus) SetErrorReason(v capierrors.ClusterStatusError) {
	s.FailureReason = &v
}

// SetErrorMessage sets the KubernetesCluster error message.
func (s *KubernetesClusterStatus) SetErrorMessage(v error) {
	s.FailureMessage = pointer.StringPtr(v.Error())
}

// KubernetesClusterPhase describes the state of a KuberntesCluster.
//
// This type is a high-level indicator of the status of the KuberntesCluster as it is provisioned,
// from the API user’s perspective. The value should not be interpreted by any software components
// as a reliable indication of the actual state of the KuberntesCluster.
type KubernetesClusterPhase string

// These are the valid statuses of KubernetesClusters.
const (
	// KubernetesClusterPhaseProvisioning is the first state a KubernetesCluster is
	// assigned after being created.
	KubernetesClusterPhaseProvisioning KubernetesClusterPhase = "Provisioning"

	// KubernetesClusterPhaseProvisioned is the state when the KubernetesCluster
	// is ready.
	KubernetesClusterPhaseProvisioned KubernetesClusterPhase = "Provisioned"

	// KubernetesClusterPhaseFailed is the state when the system might require
	// manual intervention.
	KubernetesClusterPhaseFailed KubernetesClusterPhase = "Failed"

	// KubernetesClusterPhaseDeleting is the state when a delete request has
	// been sent to the API Server.
	KubernetesClusterPhaseDeleting KubernetesClusterPhase = "Deleting"
)

// APIEndpoint represents a reachable Kubernetes API endpoint.
type APIEndpoint struct {
	// Host is the hostname on which the API Server is serving.
	Host string `json:"host"`

	// Port is the port on which the API Server is serving.
	Port int32 `json:"port"`
}

// IsZero returns true if either the host or the port are zero values.
func (v APIEndpoint) IsZero() bool {
	return v.Host == "" || v.Port == 0
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="phase",type="string",JSONPath=".status.phase",description="KubernetesCluster status"
// +kubebuilder:printcolumn:name="host",type="string",JSONPath=".spec.controlPlaneEndpoint.host",description="Endpoint host for reaching the control plane"
// +kubebuilder:printcolumn:name="port",type="integer",JSONPath=".spec.controlPlaneEndpoint.port",description="Endpoint port for reaching the control plane"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"

// KubernetesCluster is the Schema for the kubernetesclusters API.
type KubernetesCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesClusterSpec   `json:"spec,omitempty"`
	Status KubernetesClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KubernetesClusterList contains a list of KubernetesCluster.
type KubernetesClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubernetesCluster{}, &KubernetesClusterList{})
}
