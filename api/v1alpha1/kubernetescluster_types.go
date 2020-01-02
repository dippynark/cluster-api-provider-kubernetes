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
)

const (
	// ClusterFinalizer allows KubernetesClusterReconciler to clean up resources
	// associated with KubernetesCluster before removing it from the apiserver.
	ClusterFinalizer = "kubernetescluster.infrastructure.lukeaddison.co.uk"
)

// KubernetesClusterSpec defines the desired state of KubernetesCluster
type KubernetesClusterSpec struct {
	// +optional
	// +kubebuilder:default="ClusterIP"
	ControlPlaneServiceType corev1.ServiceType `json:"controlPlaneServiceType,omitempty"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint APIEndpoint `json:"controlPlaneEndpoint"`
}

// KubernetesClusterStatus defines the observed state of KubernetesCluster
type KubernetesClusterStatus struct {
	// Ready denotes that the kubernetes cluster (infrastructure) is ready.
	// +optional
	Ready bool `json:"ready"`

	// APIEndpoints represents the endpoints to communicate with the control
	// plane.
	// +optional
	APIEndpoints []APIEndpoint `json:"apiEndpoints,omitempty"`
}

// APIEndpoint represents a reachable Kubernetes API endpoint.
type APIEndpoint struct {
	// Host is the hostname on which the API server is serving.
	Host string `json:"host"`

	// Port is the port on which the API server is serving.
	Port int32 `json:"port"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=cluster-api
// +kubebuilder:printcolumn:name="control-plane-service-type",type="string",JSONPath=".spec.controlPlaneServiceType",description="Service type used for the control plane load balancer"
// +kubebuilder:printcolumn:name="host",type="string",JSONPath=".spec.controlPlaneEndpoint.host",description="Endpoint host for reaching the control plane"
// +kubebuilder:printcolumn:name="port",type="integer",JSONPath=".spec.controlPlaneEndpoint.port",description="Endpoint port for reaching the control plane"
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
