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

Any recent Kubernetes cluster (1.16+) should be suitable for the outer cluster.

We are going to use [Calico] as an overlay implementation for the inner cluster with [IP-in-IP
encapsulation] enabled so that our outer cluster does not need to know about the inner cluster's Pod
IP range. To make this work we need to ensure that the `ipip` kernel module is loadable and that
IPv4 encapsulated packets are forwarded by the kernel.

On GKE this can be accomplished as follows:

```sh
# The GKE Ubuntu image includes the ipip kernel module
# Calico handles loading the module if necessary
# https://github.com/projectcalico/felix/blob/9469e77e0fa530523be915dfaa69cc42d30b8317/dataplane/linux/ipip_mgr.go#L107-L110
gcloud beta container clusters create management-cluster \
  --image-type=UBUNTU \
  --release-channel=rapid \
  --machine-type=n1-standard-4 \
  --enable-autorepair

# Allow IP-in-IP traffic between outer cluster Nodes from inner cluster Pods
CLUSTER_CIDR=$(gcloud container clusters describe management-cluster --format="value(clusterIpv4Cidr)")
gcloud compute firewall-rules create allow-management-cluster-pods-ipip \
  --source-ranges=$CLUSTER_CIDR \
  --allow=ipip

# Forward IPv4 encapsulated packets
kubectl apply -f hack/forward-ipencap.yaml
```

### Installation

```sh
# Install clusterctl
# TODO: use proper release when available
# https://github.com/kubernetes-sigs/cluster-api/pull/2684
git clone --single-branch --branch add-gcp-auth-provider-support https://github.com/dippynark/cluster-api
cd cluster-api
go build -ldflags "$(hack/version.sh)" -o bin/clusterctl ./cmd/clusterctl
export PATH=$PWD/bin:$PATH

# Add the Kubernetes infrastructure provider
mkdir -p $HOME/.cluster-api
cat > $HOME/.cluster-api/clusterctl.yaml <<EOF
providers:
- name: kubernetes
  url: https://github.com/dippynark/cluster-api-provider-kubernetes/releases/latest/infrastructure-components.yaml
  type: InfrastructureProvider
EOF

# Initialise
clusterctl init --infrastructure kubernetes
# Apply kubadm control plane RBAC
# TODO: use aggregation label when available
# https://github.com/kubernetes-sigs/cluster-api/pull/2685
kubectl apply -f https://github.com/dippynark/cluster-api-provider-kubernetes/releases/download/v0.3.0/kubeadm-control-plane-rbac.yaml
```

### Configuration

```sh
# Use ClusterIP for clusters that do not support Services of type LoadBalancer
export KUBERNETES_CONTROL_PLANE_SERVICE_TYPE="LoadBalancer"
export KUBERNETES_CONTROL_PLANE_MACHINE_CPU_REQUESTS="1"
export KUBERNETES_CONTROL_PLANE_MACHINE_MEMORY_REQUESTS="1Gi"
export KUBERNETES_NODE_MACHINE_CPU_REQUESTS="1"
export KUBERNETES_NODE_MACHINE_MEMORY_REQUESTS="1Gi"
clusterctl config cluster example \
  --kubernetes-version=v1.17.0 \
  --control-plane-machine-count=1 \
  --worker-machine-count=1 \
  | kubectl apply -f -

# Retrieve kubeconfig
until [ -n "`kubectl get secret example-kubeconfig -o jsonpath='{.data.value}' 2>/dev/null`" ] ; do
  sleep 1
done
kubectl get secret example-kubeconfig -o jsonpath='{.data.value}' | base64 --decode > example-kubeconfig

# Switch to example cluster
# If the cluster api endpoint is not reachable from your machine you can exec into a
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
