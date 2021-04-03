# Persistence

By default when the Pod backing a KubernetesMachine fails or is deleted, the KubernetesMachine (and
therefore the managing Machine) will be set to failed. However, if the
`kubernetesMachine.spec.allowRecreation` field is set to `true`, the Pod will instead be recreated
with the same name. For controller Machines, by mounting a PersistentVolume at the etcd data
directory, the Pod can recover without data loss and without the managing KubernetesMachine failing:

```yaml
apiVersion: infrastructure.dippynark.co.uk/v1alpha3
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
