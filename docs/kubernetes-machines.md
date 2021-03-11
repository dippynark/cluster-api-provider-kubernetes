# KubernetesMachines

KubernetesMachine resources represent a Kubernetes Pod that will bootstrap to become a Kubernetes
Node. KubernetesMachines expose the full spec of the Pod with the name of the container running the
Kubernetes components hardcoded to `kind`.

For example, to set the resource requests of a controller KubernetesMachine, the following resource
can be used:

```yaml
apiVersion: infrastructure.dippynark.co.uk/v1alpha3
kind: KubernetesMachine
metadata:
  name: controller
spec:
  containers:
  - name: kind
    resources:
      requests:
        cpu: 200m
        memory: 200Mi
```

Similarly, to distribute a set of workers across Nodes of the management cluster, the following
KubernetesMachineTemplate could be used in a MachineDeployment:

```yaml
apiVersion: infrastructure.dippynark.co.uk/v1alpha3
kind: KubernetesMachineTemplate
metadata:
  name: worker
spec:
  template:
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: cluster.x-k8s.io/cluster-name
                  operator: Exists
              topologyKey: kubernetes.io/hostname
```

Currently some fields can not be changed, for example the `image` of the `kind` container is always
set to `kindest/node` tagged with the Kubernetes version of the owning Machine, but this may change
in future releases to support custom images.
