# Kubernetes Cluster API Provider Kubernetes

The [Cluster API] brings declarative, Kubernetes-style APIs to cluster creation,
configuration and management.

This project is a [Cluster API Infrastructure Provider] implementation using
Kubernetes itself to provide the infrastructure. Pods running [kind] are
created and configured to serve as Nodes which form a cluster.

The primary use cases for this project are testing and experimentation.

## Quickstart

We will deploy a Kubernetes cluster to provide the infrastructure, install the Cluster API
controllers and configure an example Kubernetes cluster using the Cluster API and the Kubernetes
infrastructure provider. We will refer to the infrastructure cluster as the outer cluster and the
Cluster API cluster as the inner cluster.

### Infrastructure

Any recent Kubernetes cluster should be suitable for the outer cluster (compatibility matrix to
come). The following manifests assume we are using an outer cluster that supports [LoadBalancer
Service] types, although they can be adapted for clusters that do not.

We are going to use [Calico] as an overlay implementation for the inner cluster with [IP-in-IP
encapsulation] enabled so that our outer cluster does not need to know about the inner cluster's Pod
IP range. To make this work we need to ensure that the `ipip` kernel module is loadable and that
IPv4 encapsulated packets are forwarded by the kernel.

On GKE this can be accomplished as follows:

```sh
# The GKE Ubuntu image includes the ipip kernel module
# Calico handles loading the module if necessary
# https://github.com/projectcalico/felix/blob/9469e77e0fa530523be915dfaa69cc42d30b8317/dataplane/linux/ipip_mgr.go#L107-L110
gcloud beta container clusters create management-cluster --image-type=UBUNTU --release-channel=rapid --enable-autorepair

# Allow IP-in-IP traffic between outer cluster Nodes from inner cluster Pods
CLUSTER_CIDR=$(gcloud container clusters describe management-cluster --format="value(clusterIpv4Cidr)")
gcloud compute firewall-rules create allow-management-cluster-pods-ipip --source-ranges=$CLUSTER_CIDR --allow=ipip

# Forward IPv4 encapsulated packets
kubectl apply -f hack/forward-ipencap.yaml
```

### Installation

```sh
# Install cert-manager
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v0.13.1/cert-manager.yaml

# Install cluster api manager
kubectl apply -f https://github.com/kubernetes-sigs/cluster-api/releases/download/v0.3.0/cluster-api-components.yaml

# Install kubernetes infrastructure provider
kubectl apply -f https://github.com/dippynark/cluster-api-provider-kubernetes/releases/download/v0.3.0/infrastructure-components.yaml

# Allow cluster api controller to interact with kubernetes infrastructure resources
# If the kubernetes provider were SIG-sponsored this would not be necesarry ;)
kubectl apply -f https://github.com/dippynark/cluster-api-provider-kubernetes/releases/download/v0.3.0/capi-kubernetes-rbac.yaml
```

### Configuration

```sh
# Apply cluster infrastructure
kubectl apply -f <(cat <<EOF
apiVersion: infrastructure.lukeaddison.co.uk/v1alpha3
kind: KubernetesCluster
metadata:
  name: example
spec:
  # Change for clusters that do not support LoadBalancer Service types
  controlPlaneServiceType: LoadBalancer
---
apiVersion: cluster.x-k8s.io/v1alpha3
kind: Cluster
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
    apiVersion: infrastructure.lukeaddison.co.uk/v1alpha3
    kind: KubernetesCluster
    name: example
  controlPlaneRef:
    apiVersion: controlplane.cluster.x-k8s.io/v1alpha3
    kind: KubeadmControlPlane
    name: example
EOF
)

# Apply machine template
kubectl apply -f <(cat <<EOF
apiVersion: infrastructure.lukeaddison.co.uk/v1alpha3
kind: KubernetesMachineTemplate
metadata:
  name: example
spec:
  template:
    spec: {}
EOF
)

# Apply control plane
kubectl apply -f <(cat <<EOF
apiVersion: controlplane.cluster.x-k8s.io/v1alpha3
kind: KubeadmControlPlane
metadata:
  name: example
spec:
  replicas: 1
  version: v1.17.0
  infrastructureTemplate:
    kind: KubernetesMachineTemplate
    apiVersion: infrastructure.lukeaddison.co.uk/v1alpha3
    name: example
  kubeadmConfigSpec:
    initConfiguration:
      nodeRegistration:
        kubeletExtraArgs:
          eviction-hard: nodefs.available<0%,nodefs.inodesFree<0%,imagefs.available<0%
          cgroups-per-qos: "false"
          enforce-node-allocatable: ""
    clusterConfiguration:
      controllerManager:
        extraArgs:
          enable-hostpath-provisioner: "true"
    joinConfiguration:
      nodeRegistration:
        kubeletExtraArgs:
          eviction-hard: nodefs.available<0%,nodefs.inodesFree<0%,imagefs.available<0%
          cgroups-per-qos: "false"
          enforce-node-allocatable: ""
EOF
)

# Apply workers
kubectl apply -f <(cat <<EOF
apiVersion: bootstrap.cluster.x-k8s.io/v1alpha3
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
apiVersion: cluster.x-k8s.io/v1alpha3
kind: MachineDeployment
metadata:
  name: workers
  labels:
    cluster.x-k8s.io/cluster-name: example
    nodepool: default
spec:
  clusterName: example
  replicas: 3
  selector:
    matchLabels:
      nodepool: default
  template:
    metadata:
      labels:
        nodepool: default
    spec:
      clusterName: example
      version: "v1.17.0"
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1alpha3
          kind: KubeadmConfigTemplate
          name: worker
      infrastructureRef:
        apiVersion: infrastructure.lukeaddison.co.uk/v1alpha3
        kind: KubernetesMachineTemplate
        name: example
EOF
)

# Retrieve kubeconfig
until [ -n "`kubectl get secret example-kubeconfig -o jsonpath='{.data.value}' 2>/dev/null`" ] ; do
  sleep 1
done
kubectl get secret example-kubeconfig -o jsonpath='{.data.value}' | base64 --decode > example-kubeconfig

# Switch to example cluster
# If the cluster api endpoint is not reachable from your machine you can exec into the
# controller Node (Pod) and run `export KUBECONFIG=/etc/kubernetes/admin.conf` instead
export KUBECONFIG=example-kubeconfig

# Wait for the apiserver to come up
until kubectl get nodes &>/dev/null; do
  sleep 1
done

# Install Calico overlay
# Note that this needs to align with the configured pod cidr
# https://docs.projectcalico.org/v3.12/getting-started/kubernetes/installation/calico#installing-with-the-kubernetes-api-datastore50-nodes-or-less%23installing-with-the-kubernetes-api-datastore50-nodes-or-less
kubectl apply -f https://docs.projectcalico.org/v3.12/manifests/calico.yaml

# Interact with your new cluster!
kubectl get nodes

# Clean up
unset KUBECONFIG
rm example-kubeconfig
kubectl delete cluster example
# If using the GKE example above
yes | gcloud compute firewall-rules delete allow-management-cluster-pods-ipip
yes | gcloud container clusters delete management-cluster --async
```

[Cluster API]: https://github.com/kubernetes-sigs/cluster-api
[Cluster API Infrastructure Provider]: https://cluster-api.sigs.k8s.io/reference/providers.html#infrastructure
[kind]: https://github.com/kubernetes-sigs/kind
[LoadBalancer Service]: https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer
[Calico]: https://docs.projectcalico.org/v3.11/getting-started/kubernetes/
[IP-in-IP encapsulation]: https://docs.projectcalico.org/v3.11/getting-started/kubernetes/installation/config-options#configuring-ip-in-ip
