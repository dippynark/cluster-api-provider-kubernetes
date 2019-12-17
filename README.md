# cluster-api-provider-kubernetes

This project implements the [cluster infrastructure
api](https://cluster-api.sigs.k8s.io/reference/providers.html#infrastructure)
for Kubernetes clusters.

Pods are created to serve as cluster Nodes which then form a cluster.

## Quickstart

The first thing we need is a cluster to provide the infrastructure. We are
assuming use of GKE here, but other clusters that support LoadBalancer Services
could be used similarly (support for clusters that do no support LoadBalancer
Services coming shortly).

> WARNING: the following will cost money - consider using the [GCP Free
> Tier](https://cloud.google.com/free/)

```sh
gcloud container clusters create management-cluster
gcloud container clusters get-credentials management-cluster
```

### Installation

```sh
# Apply cluster api
kubectl apply -f https://github.com/kubernetes-sigs/cluster-api/releases/download/v0.2.7/cluster-api-components.yaml
# Apply kubeadm bootstrap provider
kubectl apply -f https://github.com/kubernetes-sigs/cluster-api-bootstrap-provider-kubeadm/releases/download/v0.1.5/bootstrap-components.yaml
# Install kubernetes infrastructure crds
make install
# Run manager
make run
# In a separate window allow cluster api manager to interact with kubernetes infrastructure resources
kubectl apply -f <(cat <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: capi-kubernetes
rules:
- apiGroups: ["infrastructure.lukeaddison.co.uk"]
  resources: ["kubernetesclusters", "kubernetesmachines","kubernetesmachinetemplates"]
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
  clusterNetwork:
    services:
      cidrBlocks: ["172.16.0.0/12"]
    pods:
      cidrBlocks: ["192.168.0.0/16"]
    serviceDomain: "cluster.local"
  infrastructureRef:
    kind: KubernetesCluster
    apiVersion: infrastructure.lukeaddison.co.uk/v1alpha1
    name: example
EOF
)
# Wait for loadbalancer to be provisioned and retrieve IP address
LOADBALANCER_IP=$(kubectl get svc example-lb -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
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
        cgroups-per-qos: "false"
        enforce-node-allocatable: ""
  clusterConfiguration:
    controlPlaneEndpoint: "$LOADBALANCER_IP:443"
    controllerManager:
      extraArgs:
        enable-hostpath-provisioner: "true"
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
# Deploy worker machine deployment
kubectl apply -f <(cat <<EOF
kind: MachineDeployment
apiVersion: cluster.x-k8s.io/v1alpha2
metadata:
  name: worker
  labels:
    cluster.x-k8s.io/cluster-name: example
    nodepool: default
spec:
  replicas: 3
  selector:
    matchLabels:
      cluster.x-k8s.io/cluster-name: example
      nodepool: default
  template:
    metadata:
      labels:
        cluster.x-k8s.io/cluster-name: example
        nodepool: default
    spec:
      version: "v1.16.3"
      bootstrap:
        configRef:
          kind: KubeadmConfigTemplate
          apiVersion: bootstrap.cluster.x-k8s.io/v1alpha2
          name: worker
      infrastructureRef:
        kind: KubernetesMachineTemplate
        apiVersion: infrastructure.lukeaddison.co.uk/v1alpha1
        name: worker
---
apiVersion: bootstrap.cluster.x-k8s.io/v1alpha2
kind: KubeadmConfigTemplate
metadata:
  name: worker
spec:
  template:
    spec:
      joinConfiguration:
        nodeRegistration:
          kubeletExtraArgs:
            eviction-hard: nodefs.available<0%,nodefs.inodesFree<0%,imagefs.available<0%
            cgroups-per-qos: "false"
            enforce-node-allocatable: ""
---
kind: KubernetesMachineTemplate
apiVersion: infrastructure.lukeaddison.co.uk/v1alpha1
metadata:
  name: worker
spec:
  template:
    spec: {}
EOF
)
# Check that the node pods are being created
kubectl get pods
# Retrieve kubeconfig
kubectl get secret example-kubeconfig -o jsonpath='{.data.value}' | base64 --decode > example-kubeconfig
export KUBECONFIG=example-kubeconfig
# Check that the nodes are registering
kubectl get nodes
# Install an overlay and cni plugin
# Note that this may need to align with the configured pod cidr
# https://docs.projectcalico.org/v3.10/getting-started/kubernetes/installation/calico#installing-with-the-kubernetes-api-datastore50-nodes-or-less%23installing-with-the-kubernetes-api-datastore50-nodes-or-less
kubectl apply -f https://docs.projectcalico.org/v3.10/manifests/calico.yaml
# Check that the nodes become ready
kubectl get nodes
# Tear down cluster
unset KUBECONFIG
kubectl delete cluster example
```
