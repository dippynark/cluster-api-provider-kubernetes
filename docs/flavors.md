# Flavors

Cluster API Provider Kubernetes supports a number of
[flavors](https://cluster-api.sigs.k8s.io/clusterctl/commands/config-cluster.html#flavors) for
creating clusters. Flavors are YAML templates that can be hydrated using
[clusterctl](https://cluster-api.sigs.k8s.io/clusterctl/commands/commands.html).

## Default

The default flavor creates a Kubernetes cluster with the controller Nodes managed by a
[KubeadmControlPlane](https://github.com/kubernetes-sigs/cluster-api/blob/master/docs/proposals/20191017-kubeadm-based-control-plane.md)
resource and the worker Nodes managed by a
[MachineDeployment](https://cluster-api.sigs.k8s.io/developer/architecture/controllers/machine-deployment.html)
resource.

The controller Nodes write etcd state to an
[emptyDir](https://kubernetes.io/docs/concepts/storage/volumes/#emptydir) volume and the
corresponding KubernetesMachines will fail if the underlying Pods fail, relying on the
KubeadmControlPlane for remediation.

The worker Nodes will fail if their underlying Pods fail, relying on the MachineDeployment and a
[MachineHealthCheck](https://cluster-api.sigs.k8s.io/developer/architecture/controllers/machine-health-check.html)
for remediation.

```sh
CLUSTER_NAME="example"
export KUBERNETES_CONTROL_PLANE_SERVICE_TYPE="LoadBalancer"
clusterctl config cluster $CLUSTER_NAME \
  --infrastructure kubernetes \
  --kubernetes-version v1.17.0 \
  --control-plane-machine-count 1 \
  --worker-machine-count 1 \
  | kubectl apply -f -
```

| Variable | Required | Default |
| - | - | - |
| CLUSTER_NAME | Yes | |
| CONTROL_PLANE_MACHINE_COUNT | Yes | |
| KUBERNETES_CONTROLLER_MACHINE_CPU_REQUEST | No | 0 |
| KUBERNETES_CONTROLLER_MACHINE_MEMORY_REQUEST | No | 0 |
| KUBERNETES_CONTROL_PLANE_SERVICE_TYPE | Yes | |
| KUBERNETES_VERSION | Yes | |
| KUBERNETES_WORKER_MACHINE_CPU_REQUEST | No | 0 |
| KUBERNETES_WORKER_MACHINE_MEMORY_REQUEST | No | 0 |
| POD_CIDR | No | ["192.168.0.0/16"] |
| SERVICE_CIDR | No | ["10.128.0.0/12"] |
| SERVICE_DOMAIN | No | cluster.local |
| WORKER_MACHINE_COUNT | Yes | |

## Persistent Control Plane

The persistent control plane flavor creates a Kubernetes cluster with the controller Nodes managed
by a
[KubeadmControlPlane](https://github.com/kubernetes-sigs/cluster-api/blob/master/docs/proposals/20191017-kubeadm-based-control-plane.md)
resource and the worker Nodes managed by a
[MachineDeployment](https://cluster-api.sigs.k8s.io/developer/architecture/controllers/machine-deployment.html)
resource.

PersistentVolumes are dynamically provisioned for the controller Nodes to write etcd state and the
corresponding KubernetesMachines are configured to recreate their underlying Pods if they fail as
described in [persistence.md](persistence.md).

The worker Nodes will fail if their underlying Pods fail, relying on the MachineDeployment and a
[MachineHealthCheck](https://cluster-api.sigs.k8s.io/developer/architecture/controllers/machine-health-check.html)
for remediation.

```sh
CLUSTER_NAME="example"
export KUBERNETES_CONTROL_PLANE_SERVICE_TYPE="LoadBalancer"
export ETCD_STORAGE_CLASS_NAME="ssd"
export ETCD_STORAGE_SIZE="10Gi"
clusterctl config cluster $CLUSTER_NAME \
  --infrastructure kubernetes \
  --kubernetes-version v1.17.0 \
  --control-plane-machine-count 1 \
  --worker-machine-count 1 \
  --flavor persistent-control-plane \
  | kubectl apply -f -
```

| Variable | Required | Default |
| - | - | - |
| CLUSTER_NAME | Yes | |
| CONTROL_PLANE_MACHINE_COUNT | Yes | |
| ETCD_STORAGE_CLASS_NAME | Yes |  |
| ETCD_STORAGE_SIZE | Yes | |
| KUBERNETES_CONTROLLER_MACHINE_CPU_REQUEST | No | 0 |
| KUBERNETES_CONTROLLER_MACHINE_MEMORY_REQUEST | No | 0 |
| KUBERNETES_CONTROL_PLANE_SERVICE_TYPE | Yes | |
| KUBERNETES_VERSION | Yes | |
| KUBERNETES_WORKER_MACHINE_CPU_REQUEST | No | 0 |
| KUBERNETES_WORKER_MACHINE_MEMORY_REQUEST | No | 0 |
| POD_CIDR | No | ["192.168.0.0/16"] |
| SERVICE_CIDR | No | ["10.128.0.0/12"] |
| SERVICE_DOMAIN | No | cluster.local |
| WORKER_MACHINE_COUNT | Yes | |
