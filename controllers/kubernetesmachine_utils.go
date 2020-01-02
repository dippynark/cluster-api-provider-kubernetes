package controllers

import (
	"encoding/base64"
	"fmt"

	capkv1 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha1"
	infrav1 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha1"
	"github.com/dippynark/cluster-api-provider-kubernetes/pkg/cloudinit"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *KubernetesMachineReconciler) getPersistentVolumeClaims(kubernetesMachine *capkv1.KubernetesMachine) (map[string]corev1.PersistentVolumeClaim, error) {
	templates := kubernetesMachine.Spec.VolumeClaimTemplates
	claims := make(map[string]corev1.PersistentVolumeClaim, len(templates))
	for i := range templates {
		claim := templates[i]
		claim.Name = getPersistentVolumeClaimName(kubernetesMachine, &claim)
		claim.Namespace = kubernetesMachine.Namespace
		// TODO: make pvs be owned by the cluster? or the kubeadmControlPlane?
		if err := controllerutil.SetControllerReference(kubernetesMachine, &claim, r.Scheme); err != nil {
			return claims, err
		}
		claims[templates[i].Name] = claim
	}
	return claims, nil
}

func getPersistentVolumeClaimName(kubernetesMachine *capkv1.KubernetesMachine, claim *corev1.PersistentVolumeClaim) string {
	// TODO: should we conform to the stateful set heuristics?
	// https://github.com/kubernetes/kubernetes/blob/2cb17cc67745aa39f700e1d0c3d22c70074bee46/pkg/volume/util.go#L301-L303
	return fmt.Sprintf("%s-%s", kubernetesMachine.Name, claim.Name)
}

func (r *KubernetesMachineReconciler) createPersistentVolumeClaims(kubernetesMachine *capkv1.KubernetesMachine) error {
	var errs []error
	// TODO: we watch for pvcs and create multiple in one reconcile loop
	// this potentially triggers many nop loops - do something about this
	// ReplicaSet controller seems to do this too so maybe this is okay
	// https://github.com/kubernetes/kubernetes/blob/ff975e865df4ff941688c98a0bb02db7fae28dfe/pkg/controller/replicaset/replica_set.go#L741
	claims, err := r.getPersistentVolumeClaims(kubernetesMachine)
	if err != nil {
		return err
	}
	for _, claim := range claims {
		_, err = r.CoreV1Client.PersistentVolumeClaims(claim.Namespace).Get(claim.Name, metav1.GetOptions{})
		switch {
		case k8serrors.IsNotFound(err):
			_, err := r.CoreV1Client.PersistentVolumeClaims(claim.Namespace).Create(&claim)
			// IsAlreadyExists should never happen
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				errs = append(errs, fmt.Errorf("failed to create PVC %s: %s", claim.Name, err))
			}
		case err != nil:
			errs = append(errs, fmt.Errorf("failed to retrieve PVC %s: %s", claim.Name, err))
		}
		// TODO: Check resource requirements and accessmodes, update if necessary
		// TODO: check pvc is owned by kubernetesMachine
	}
	return errorutils.NewAggregate(errs)
}

// updateStorage updates pod's Volumes to conform with the PersistentVolumeClaim
// of kubernetesMachine's templates. If pod has conflicting local Volumes these
// are replaced with Volumes that conform to the kubernetesMachine's templates.
func (r *KubernetesMachineReconciler) updateStorage(kubernetesMachine *infrav1.KubernetesMachine, machinePod *corev1.Pod) error {
	currentVolumes := machinePod.Spec.Volumes
	claims, err := r.getPersistentVolumeClaims(kubernetesMachine)
	if err != nil {
		return err
	}
	newVolumes := make([]corev1.Volume, 0, len(claims))
	for name, claim := range claims {
		newVolumes = append(newVolumes, corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: claim.Name,
					// TODO: Use source definition to set this value when we have one
					ReadOnly: false,
				},
			},
		})
	}
	for i := range currentVolumes {
		if _, ok := claims[currentVolumes[i].Name]; !ok {
			newVolumes = append(newVolumes, currentVolumes[i])
		}
	}
	machinePod.Spec.Volumes = newVolumes
	return nil
}

func (r *KubernetesMachineReconciler) generatateCloudInitSecret(cluster *clusterv1.Cluster, machine *clusterv1.Machine, kubernetesMachine *capkv1.KubernetesMachine, data *string) (*corev1.Secret, error) {

	cloudConfig, err := base64.StdEncoding.DecodeString(*data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode machine's bootstrap data")
	}
	cloudInitScript, err := cloudinit.GenerateScript(cloudConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate machine's cloudinit bootstrap script")
	}

	// Create cloud-init secret
	cloudInitScriptSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machinePodName(cluster, machine) + "-cloud-init",
			Namespace: machine.Namespace,
			Labels: map[string]string{
				clusterv1.MachineClusterLabelName: cluster.Name,
			},
		},
		StringData: map[string]string{
			cloudInitBootstrapScriptName:    cloudInitScript,
			cloudInitInstallScriptName:      cloudInitInstallScript,
			cloudInitSystemdServiceUnitName: cloudInitSystemdServiceUnit,
			cloudInitSystemdPathUnitName:    cloudInitSystemdPathUnit,
		},
	}

	// TODO: make secret be owned by machine pod?
	if err := controllerutil.SetControllerReference(kubernetesMachine, cloudInitScriptSecret, r.Scheme); err != nil {
		return nil, err
	}

	return cloudInitScriptSecret, nil
}

func setVolumeMount(kindContainer *corev1.Container, name, mountPath, subPath string, readOnly bool) {
	volumeMountPathMissing := true
	for _, volumeMount := range kindContainer.VolumeMounts {
		// Only create mount point if unused elsewhere
		if volumeMount.MountPath == mountPath {
			volumeMountPathMissing = false
			break
		}
	}
	if volumeMountPathMissing {
		volumeMount := corev1.VolumeMount{
			Name:      name,
			MountPath: mountPath,
			ReadOnly:  readOnly,
			SubPath:   subPath,
		}
		kindContainer.VolumeMounts = append(kindContainer.VolumeMounts, volumeMount)
	}

}

func apiServerPort(cluster *clusterv1.Cluster) int32 {
	// TODO: kubeadm bootstrap provider currently does not take APIServerPort
	// into account - should we ignore this too?
	if cluster.Spec.ClusterNetwork != nil && cluster.Spec.ClusterNetwork.APIServerPort != nil {
		return *cluster.Spec.ClusterNetwork.APIServerPort
	}
	return defaultAPIServerPort
}

func machinePodImage(machine *clusterv1.Machine) string {
	if machine.Spec.Version == nil {
		return fmt.Sprintf("%s:%s", defaultImageName, defaultImageTag)
	}
	return fmt.Sprintf("%s:%s", defaultImageName, *machine.Spec.Version)
}

func machinePodName(cluster *clusterv1.Cluster, machine *clusterv1.Machine) string {
	return fmt.Sprintf("%s-%s", cluster.Name, machine.Name)
}

func providerID(cluster *clusterv1.Cluster, machine *clusterv1.Machine) string {
	// TODO: is there some more standard format for this?
	return fmt.Sprintf("kubernetes://%s/%s/%s", cluster.Namespace, cluster.Name, machine.Name)
}

func kubeconfigSecretName(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("%s-kubeconfig", cluster.Name)
}
