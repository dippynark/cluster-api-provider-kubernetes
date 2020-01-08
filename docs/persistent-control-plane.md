# Persistent Control Plane

By default KubernetesMachine Pods write to the corresponding kind container's
root filesystem and so changes are not persisted should the Pod be recreated.
Since a KubernetesMachine resource exposes the full Pod spec, PersistentVolumes
can be used in the same way as for other Kubernetes workloads to persist data.

Additionally, in the same way as [StatefulSets], KubernetesMachines support `volumeClaimTemplates`
to dynamically provision PersistentVolumeClaims which then bind to an available PersistentVolume or
trigger a PersistentVolume provisioner. In the same way as StatefulSets, deleting a
KubernetesMachine will not delete the volumes associated with it to ensure data safety.

The following manifest creates a controller KubernetesMachine that stores etcd data on a dynamically
provisioned PersistentVolume. It assumes we are running on a cluster that supports [dynamic volume
provisioning].

```yaml
kind: KubernetesMachine
apiVersion: infrastructure.lukeaddison.co.uk/v1alpha2
metadata:
  name: controller
spec:
  containers:
  - name: kind
    volumeMounts:
    - name: var-lib-etcd
      mountPath: /var/lib/etcd
  volumeClaimTemplates:
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
