# cluster-api-provider-kubernetes

This project implements the [Cluster Infrastructure Provider
API](https://cluster-api.sigs.k8s.io/reference/providers.html#infrastructure)
for Kubernetes clusters.

Pods running [kind](https://github.com/kubernetes-sigs/kind) are created and
configured to serve as Nodes which form a cluster.

## Quickstart

We will create the infrastructure to run the Cluster API controllers, install
the controllers and then configure an example cluster using the Kubernetes
infrastrucutre provider.

### Infrastructure

Any Kubernetes cluster should be suitable, but below are a few examples.

#### kind

```sh
# install kind: https://github.com/kubernetes-sigs/kind#installation-and-usage
kind create cluster --name management-cluster
kind export kubeconfig --name management-cluster
```

#### GKE

> WARNING: the following will cost money - consider using the [GCP Free
> Tier](https://cloud.google.com/free/)

```sh
gcloud container clusters create management-cluster
gcloud container clusters get-credentials management-cluster
```

### Installation

```sh
# Install cluster api
kubectl apply -f https://github.com/kubernetes-sigs/cluster-api/releases/download/v0.2.7/cluster-api-components.yaml
# Install kubeadm bootstrap provider
kubectl apply -f https://github.com/kubernetes-sigs/cluster-api-bootstrap-provider-kubeadm/releases/download/v0.1.5/bootstrap-components.yaml
# Install kubernetes infrastructure crds
make install
# Deploy kubernetes infrastructure provider controller
make deploy
# Allow cluster api controller to interact with kubernetes infrastructure resources
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
```

### Configuration

```sh
# Apply cluster infrastructure
kubectl apply -f <(cat <<EOF
kind: KubernetesCluster
apiVersion: infrastructure.lukeaddison.co.uk/v1alpha1
metadata:
  name: example
spec: {}
#  serviceType: LoadBalancer # uncomment on clusters that support loadbalancers
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
LOADBALANCER_HOST=$(kubectl get cluster example -o jsonpath='{.status.apiEndpoints[0].host}')
LOADBALANCER_PORT=$(kubectl get cluster example -o jsonpath='{.status.apiEndpoints[0].port}')
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
  resources:
    requests:
      cpu: 200m
      memory: 200Mi
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
  replicas: 1
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
    spec:
      resources:
        requests:
          cpu: 100m
          memory: 100Mi
EOF
)
# Check that the node pods are being created
kubectl get pods
# Retrieve kubeconfig
kubectl get secret example-kubeconfig -o jsonpath='{.data.value}' | base64 --decode > example-kubeconfig
# If the loadbalancer is reachable there is no need to port-forward or set an alias
kubectl port-forward service/example-lb 8080:$LOADBALANCER_PORT
# In a separate terminal
alias kubectl='kubectl --kubeconfig example-kubeconfig --server https://127.0.0.1:8080 --insecure-skip-tls-verify=true'
# Install an overlay and cni plugin
# Note that this needs to align with the configured pod cidr
# https://docs.projectcalico.org/v3.10/getting-started/kubernetes/installation/calico#installing-with-the-kubernetes-api-datastore50-nodes-or-less%23installing-with-the-kubernetes-api-datastore50-nodes-or-less
kubectl apply -f https://docs.projectcalico.org/v3.10/manifests/calico.yaml
# Check that the nodes become ready
kubectl get nodes
# Tear down cluster
kubectl delete cluster example
```
