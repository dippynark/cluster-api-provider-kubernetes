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
func (src *KubernetesMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*capkv1alpha3.KubernetesMachineTemplate)

	dst.ObjectMeta = src.ObjectMeta

	dst.Spec.Template.Spec.ProviderID = src.Spec.Template.Spec.ProviderID
	dst.Spec.Template.Spec.PodSpec = src.Spec.Template.Spec.PodSpec
	dst.Spec.Template.Spec.VolumeClaimTemplates = src.Spec.Template.Spec.VolumeClaimTemplates

	return nil
}

// ConvertFrom converts from the Hub version (capkv1alpha3) to this version.
func (dst *KubernetesMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*capkv1alpha3.KubernetesMachineTemplate)

	dst.ObjectMeta = src.ObjectMeta

	dst.Spec.Template.Spec.ProviderID = src.Spec.Template.Spec.ProviderID
	dst.Spec.Template.Spec.PodSpec = src.Spec.Template.Spec.PodSpec
	dst.Spec.Template.Spec.VolumeClaimTemplates = src.Spec.Template.Spec.VolumeClaimTemplates

	return nil
}
