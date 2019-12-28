# Persistent Control Plane

By default KubernetesMachine Pods write to the corresponding kind container's
root filesystem and so changes are not persisted should the Pod be recreated.
Since a KubernetesMachine resource exposes the full Pod spec, PersistentVolumes
can be used in the same way as for other Kubernetes workloads to persist data.

Additionally, in the same way as [StatefulSets], KubernetesMachines support
`volumeClaimTemplates` to dynamically provision PersistentVolumeClaims. The main
difference here compared with StatefulSets is that the resulting
PersistentVolumeClaims are [owned] by the KubernetesMachine; this means that
they will be deleted should the KubernetesMachine be deleted. If you do not want
this behaviour make sure to provision and mount the necessary PersistentVolumes
manually.

The following manifests assume we are running on a cluster that supports
[dynamic volume provisioning].

```sh
# Wait for api endpoint to be provisioned
until [ -n "`kubectl get cluster example -o jsonpath='{.status.apiEndpoints[0]}'`" ] ; do
  sleep 1
done
LOADBALANCER_HOST=$(kubectl get cluster example -o jsonpath='{.status.apiEndpoints[0].host}')
LOADBALANCER_PORT=$(kubectl get cluster example -o jsonpath='{.status.apiEndpoints[0].port}')

# Deploy persistent controller machine
kubectl apply -f <(cat <<EOF
kind: KubeadmConfig
apiVersion: bootstrap.cluster.x-k8s.io/v1alpha2
metadata:
  name: controller
spec:
  initConfiguration:
    nodeRegistration:
      kubeletExtraArgs:
        eviction-hard: nodefs.available<0%,nodefs.inodesFree<0%,imagefs.available<0%
        cgroups-per-qos: "false"
        enforce-node-allocatable: ""
  clusterConfiguration:
    controlPlaneEndpoint: "${LOADBALANCER_HOST}:${LOADBALANCER_PORT}"
    controllerManager:
      extraArgs:
        enable-hostpath-provisioner: "true"
---
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
---
kind: Machine
apiVersion: cluster.x-k8s.io/v1alpha2
metadata:
  name: controller
  labels:
    cluster.x-k8s.io/cluster-name: example
    cluster.x-k8s.io/control-plane: "true"
spec:
  version: "v1.16.3"
  bootstrap:
    configRef:
      kind: KubeadmConfig
      apiVersion: bootstrap.cluster.x-k8s.io/v1alpha2
      name: controller
  infrastructureRef:
    kind: KubernetesMachine
    apiVersion: infrastructure.lukeaddison.co.uk/v1alpha1
    name: controller
EOF
)
```

[StatefulSets]: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/
[owned]: https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection
[dynamic volume provisioning]: https://kubernetes.io/docs/concepts/storage/dynamic-provisioning/
