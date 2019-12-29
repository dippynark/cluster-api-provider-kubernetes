# Persistent Control Plane

By default KubernetesMachine Pods write to the corresponding kind container's
root filesystem and so changes are not persisted should the Pod be recreated.
Since a KubernetesMachine resource exposes the full Pod spec, PersistentVolumes
can be used in the same way as for other Kubernetes workloads to persist data.

Additionally, in the same way as [StatefulSets], KubernetesMachines support
`volumeClaimTemplates` to dynamically provision PersistentVolumeClaims which
then bind to an available PersistentVolume or trigger a PersistentVolume
provisioner. The main difference here compared with StatefulSets is that the
resulting PersistentVolumeClaims are [owned] by the KubernetesMachine; this
means that they will be deleted should the KubernetesMachine be deleted. If you
do not want this behaviour make sure to manually specify the list of volumes to
mount.

The following manifest creates a KubernetesMachine controller that stores etcd
data as well as data created during the kubeadm bootstrap phase on dynamically
provisioned PersistentVolumes. It assumes we are running on a cluster that
supports [dynamic volume provisioning].

```yaml
kind: KubernetesMachine
apiVersion: infrastructure.lukeaddison.co.uk/v1alpha1
metadata:
  name: controller
spec:
  containers:
  - name: kind
    volumeMounts:
    - name: etc-kubernetes
      mountPath: /etc/kubernetes
    - name: var-lib-kubelet
      mountPath: /var/lib/kubelet
    - name: var-lib-etcd
      mountPath: /var/lib/etcd
  volumeClaimTemplates:
  - metadata:
      name: etc-kubernetes
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 1Gi
  - metadata:
      name: var-lib-kubelet
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 1Gi
  - metadata:
      name: var-lib-etcd
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 10Gi
```

[StatefulSets]: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/
[owned]: https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection
[dynamic volume provisioning]: https://kubernetes.io/docs/concepts/storage/dynamic-provisioning/
