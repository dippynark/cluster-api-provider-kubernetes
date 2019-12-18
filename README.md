# cluster-api-provider-kubernetes

This project is a [Cluster API
Provider](https://cluster-api.sigs.k8s.io/reference/providers.html)
implementation using Kubernetes as the infrastructure provider. Pods running
[kind](https://github.com/kubernetes-sigs/kind) are created and configured to
serve as Nodes which form a cluster.

## Quickstart

We will create the infrastructure to run the Cluster API controllers, install
the controllers and then configure an example cluster using the Kubernetes
infrastruture provider.

### Infrastructure

Any Kubernetes cluster should be suitable, but below are a few examples.

#### kind

```bash
# Install kind: https://github.com/kubernetes-sigs/kind#installation-and-usage
kind create cluster --name management-cluster
kind export kubeconfig --name management-cluster
```

#### GKE

> WARNING: using a GKE cluster will cost money - consider using the [GCP Free
> Tier](https://cloud.google.com/free/)

```bash
gcloud container clusters create management-cluster
gcloud container clusters get-credentials management-cluster
```

### Installation

```bash
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

# Wait for api endpoint to be provisioned
until [ -n "`kubectl get cluster example -o jsonpath='{.status.apiEndpoints[0]}'`" ] ; do
  sleep 1
done
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
until kubectl get secret example-kubeconfig 2>/dev/null; do
  sleep 1
done
kubectl get secret example-kubeconfig -o jsonpath='{.data.value}' | base64 --decode > example-kubeconfig

# Port-forward and override kubeconfig values
# Note: If the loadbalancer is reachable from your machine there is no need to do this
kubectl port-forward service/example-lb 6443:$LOADBALANCER_PORT 2>&1 >/dev/null &
kubectl --kubeconfig example-kubeconfig config set-cluster example \
  --server=https://127.0.0.1:6443 --insecure-skip-tls-verify=true

# Switch to example cluster
export KUBECONFIG=example-kubeconfig

# Wait for the apiserver to come up
until kubectl get nodes 2>&1 >/dev/null; do
  sleep 1
done

# Install a cni solution
# Note that this needs to align with the configured pod cidr
# https://docs.projectcalico.org/v3.10/getting-started/kubernetes/installation/calico#installing-with-the-kubernetes-api-datastore50-nodes-or-less%23installing-with-the-kubernetes-api-datastore50-nodes-or-less
kubectl apply -f https://docs.projectcalico.org/v3.10/manifests/calico.yaml

# Interact with your new cluster!
kubectl get nodes

# Clean up
unset KUBECONFIG
pkill kubectl port-forward service/example-lb 8080:$LOADBALANCER_PORT
kubectl delete cluster example
```
