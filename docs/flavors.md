# Flavors

Cluster API Provider Kubernetes supports a number of
[flavors](https://cluster-api.sigs.k8s.io/clusterctl/commands/config-cluster.html#flavors) for
creating clusters. Flavors are YAML templates that can by hydrated using
[clusterctl](https://cluster-api.sigs.k8s.io/clusterctl/commands/commands.html).

## Default

| Variable | Required | Default |
| - | - | - |
| CLUSTER_NAME | Yes | |
| KUBERNETES_CONTROL_PLANE_SERVICE_TYPE | Yes | |
| CONTROL_PLANE_MACHINE_COUNT | No | 1 |
| WORKER_MACHINE_COUNT | No | 1 |
| KUBERNETES_CONTROLLER_MACHINE_CPU_REQUEST | No | 0 |
| KUBERNETES_CONTROLLER_MACHINE_MEMORY_REQUEST | No | 0 |
| KUBERNETES_WORKER_MACHINE_CPU_REQUEST | No | 0 |
| KUBERNETES_WORKER_MACHINE_MEMORY_REQUEST | No | 0 |
| SERVICE_CIDR | No | ["10.128.0.0/12"] |
| POD_CIDR | No | ["192.168.0.0/16"] |
| SERVICE_DOMAIN | No | cluster.local |

## Persistent Control Plane

> `clusterctl config create --flavor persistent-control-plane`

| Variable | Required | Default |
| - | - | - |
| CLUSTER_NAME | Yes | |
| KUBERNETES_CONTROL_PLANE_SERVICE_TYPE | Yes | |
| ETCD_STORAGE_CLASS_NAME | Yes |  |
| ETCD_STORAGE_SIZE | Yes | |
| CONTROL_PLANE_MACHINE_COUNT | No | 1 |
| WORKER_MACHINE_COUNT | No | 1 |
| KUBERNETES_CONTROLLER_MACHINE_CPU_REQUEST | No | 0 |
| KUBERNETES_CONTROLLER_MACHINE_MEMORY_REQUEST | No | 0 |
| KUBERNETES_WORKER_MACHINE_CPU_REQUEST | No | 0 |
| KUBERNETES_WORKER_MACHINE_MEMORY_REQUEST | No | 0 |
| SERVICE_CIDR | No | ["10.128.0.0/12"] |
| POD_CIDR | No | ["192.168.0.0/16"] |
| SERVICE_DOMAIN | No | cluster.local |
