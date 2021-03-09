# Flavors

Cluster API Provider Kubernetes supports a number of
[flavors](https://cluster-api.sigs.k8s.io/clusterctl/commands/config-cluster.html#flavors) for
creating clusters. Flavors are YAML templates that can be hydrated using
[clusterctl](https://cluster-api.sigs.k8s.io/clusterctl/commands/commands.html).

## Default

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
