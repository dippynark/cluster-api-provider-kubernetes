# cluster-api-provider-kubernetes

This project implements the [cluster infrastructure
api](https://cluster-api.sigs.k8s.io/reference/providers.html#infrastructure)
for Kubernetes clusters.

## Quickstart

We assume here that the latest version of [kind is
installed](https://github.com/kubernetes-sigs/kind#installation-and-usage).
Alternatively, any other Kubernetes cluster can be used as the management
cluster.

```sh
# Create management cluster
kind create cluster --name management-cluster
kind export kubeconfig --name management-cluster
# Apply cluster api
kubectl apply -f https://github.com/kubernetes-sigs/cluster-api/releases/download/v0.2.7/cluster-api-components.yaml
# Apply kubeadm bootstrap provider
kubectl apply -f https://github.com/kubernetes-sigs/cluster-api-bootstrap-provider-kubeadm/releases/download/v0.1.5/bootstrap-components.yaml
# Install kubernetes infrastructure crds
make install
# Run manager
make run
# In a separate window allow cluster api manager to interact with kubernetes infrastructure
kubectl apply -f <(cat <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: capi-kubernetes
rules:
- apiGroups: ["infrastructure.lukeaddison.co.uk"]
  resources: ["kubernetesclusters", "kubernetesmachines"]
  verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: capi-kubernetes
subjects:
- kind: ServiceAccount
  name: default
  namespace: capi-system
roleRef:
  kind: ClusterRole
  name: capi-kubernetes
  apiGroup: rbac.authorization.k8s.io
EOF
)
# Apply cluster infrastructure
kubectl apply -f <(cat <<EOF
kind: KubernetesCluster
apiVersion: infrastructure.lukeaddison.co.uk/v1alpha1
metadata:
  name: example
---
kind: Cluster
apiVersion: cluster.x-k8s.io/v1alpha2
metadata:
  name: example
spec:
  infrastructureRef:
    kind: KubernetesCluster
    apiVersion: infrastructure.lukeaddison.co.uk/v1alpha1
    name: example
EOF
)
# Wait for loadbalancer and retrieve IP
LOADBALANCER_IP=$(kubectl get svc example-lb -o jsonpath='{.spec.clusterIP}')
# Deploy controller machine
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
  clusterConfiguration:
    controllerManager:
      extraArgs:
        enable-hostpath-provisioner: "true"
    apiServer:
      extraArgs:
        advertise-address: "$LOADBALANCER_IP"
---
kind: KubernetesMachine
apiVersion: infrastructure.lukeaddison.co.uk/v1alpha1
metadata:
  name: controller
---
kind: Machine
apiVersion: cluster.x-k8s.io/v1alpha2
metadata:
  name: controller
  labels:
    cluster.x-k8s.io/cluster-name: example
    cluster.x-k8s.io/control-plane: "true"
    set: controlplane
spec:
  bootstrap:
    configRef:
      kind: KubeadmConfig
      apiVersion: bootstrap.cluster.x-k8s.io/v1alpha2
      name: controller
  infrastructureRef:
    kind: KubernetesMachine
    apiVersion: infrastructure.lukeaddison.co.uk/v1alpha1
    name: controller
  version: "v1.14.2"
EOF
)
# Exec into controller node and interact!
kubectl exec -it example-controller -- kubectl get nodes --kubeconfig /etc/kubernetes/admin.conf
```
