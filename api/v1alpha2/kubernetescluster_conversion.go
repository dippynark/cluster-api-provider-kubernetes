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
	capkv1alpha3 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this CronJob to the Hub version (capkv1alpha3).
func (src *KubernetesCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*capkv1alpha3.KubernetesCluster)

	dst.ObjectMeta = src.ObjectMeta

	dst.Spec.ControlPlaneServiceType = src.Spec.ControlPlaneServiceType
	dst.Spec.ControlPlaneEndpoint = capkv1alpha3.APIEndpoint(src.Spec.ControlPlaneEndpoint)

	dst.Status.FailureReason = src.Status.ErrorReason
	dst.Status.FailureMessage = src.Status.ErrorMessage
	dst.Status.Phase = capkv1alpha3.KubernetesClusterPhase(src.Status.Phase)
	dst.Status.Ready = src.Status.Ready

	return nil
}

// ConvertFrom converts from the Hub version (capkv1alpha3) to this version.
func (dst *KubernetesCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*capkv1alpha3.KubernetesCluster)

	dst.ObjectMeta = src.ObjectMeta

	dst.Spec.ControlPlaneServiceType = src.Spec.ControlPlaneServiceType
	dst.Spec.ControlPlaneEndpoint = APIEndpoint(src.Spec.ControlPlaneEndpoint)

	dst.Status.ErrorReason = src.Status.FailureReason
	dst.Status.ErrorMessage = src.Status.FailureMessage
	dst.Status.Phase = KubernetesClusterPhase(src.Status.Phase)
	dst.Status.Ready = src.Status.Ready

	if !src.Spec.ControlPlaneEndpoint.IsZero() {
		dst.Status.APIEndpoints = []APIEndpoint{
			{
				Host: src.Spec.ControlPlaneEndpoint.Host,
				Port: int32(src.Spec.ControlPlaneEndpoint.Port),
			},
		}
	}

	return nil
}
