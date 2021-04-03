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
resource. The controller Nodes write etcd state to the container file system and the corresponding
KubernetesMachines will fail if the underlying Pods fails, relying on the
KubeadmControlPlane for remediation.

```sh
CLUSTER_NAME="example"
export KUBERNETES_CONTROL_PLANE_SERVICE_TYPE="LoadBalancer"
clusterctl config cluster $CLUSTER_NAME \
  --infrastructure kubernetes \
  --kubernetes-version v1.20.2 \
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
resource. PersistentVolumes are dynamically provisioned for the controller Nodes to write etcd state
and the corresponding KubernetesMachines are configured to recreate the underlying Pod if it is
deleted as described in [persistence.md](persistence.md).

```sh
CLUSTER_NAME="example"
export KUBERNETES_CONTROL_PLANE_SERVICE_TYPE="LoadBalancer"
export ETCD_STORAGE_CLASS_NAME="ssd"
export ETCD_STORAGE_SIZE="1Gi"
clusterctl config cluster $CLUSTER_NAME \
  --infrastructure kubernetes \
  --kubernetes-version v1.20.2 \
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
