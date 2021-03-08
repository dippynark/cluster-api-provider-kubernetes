# Persistence

By default when the Pod backing a KubernetesMachine is deleted, the KubernetesMachine (and therefore
the managing Machine) will be set to failed. However, if the
`kubernetesMachine.spec.allowRecreation` field is set to `true`, the Pod will instead be recreated
with the same name. For controller Machines, by mounting a PersistentVolume at the etcd data
directory, the Pod can recover without data loss and without the managing KubernetesMachine failing:

```yaml
apiVersion: infrastructure.lukeaddison.co.uk/v1alpha3
kind: KubernetesMachine
metadata:
  name: controller
spec:
  allowRecreation: true
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

The
[release/cluster-template-persistent-control-plane.yaml](../release/cluster-template-persistent-control-plane.yaml)
template configures persistent controller Machines using the default StorageClass. Note that it is
[recommended to back etcd's storage with an
SSD](https://etcd.io/docs/v3.3.12/op-guide/hardware/#disks).
