# Kubernetes Cluster API Provider Kubernetes

The [Cluster API](https://github.com/kubernetes-sigs/cluster-api) brings declarative,
Kubernetes-style APIs to cluster creation, configuration and management.

This project is a [Cluster API Infrastructure
Provider](https://cluster-api.sigs.k8s.io/reference/providers.html#infrastructure) implementation
using Kubernetes itself to provide the infrastructure. Pods running
[kind](https://github.com/kubernetes-sigs/kind) are created and configured to serve as Nodes which
form a cluster.

The primary use cases for this project are testing and experimentation.

## Quickstart

We will deploy a Kubernetes cluster to provide the infrastructure, install the Cluster API
controllers and configure an example Kubernetes cluster using the Cluster API and the Kubernetes
infrastructure provider. We will refer to the infrastructure cluster as the outer cluster and the
Cluster API cluster as the inner cluster.

### Infrastructure

Any recent Kubernetes cluster (1.16+) should be suitable for the outer cluster.

We are going to use [Calico](https://docs.projectcalico.org/v3.11/getting-started/kubernetes/) as an
overlay implementation for the inner cluster with [IP-in-IP
encapsulation](https://docs.projectcalico.org/v3.11/getting-started/kubernetes/installation/config-options#configuring-ip-in-ip)
enabled so that our outer cluster does not need to know about the inner cluster's Pod IP range. To
make this work we need to ensure that the `ipip` kernel module is loadable and that IPv4
encapsulated packets are forwarded by the kernel.

On GKE this can be accomplished as follows:

```sh
# The GKE Ubuntu image includes the ipip kernel module
# Calico handles loading the module if necessary
# https://github.com/projectcalico/felix/blob/9469e77e0fa530523be915dfaa69cc42d30b8317/dataplane/linux/ipip_mgr.go#L107-L110
MANAGEMENT_CLUSTER_NAME="management"
gcloud container clusters create $MANAGEMENT_CLUSTER_NAME \
  --image-type=UBUNTU \
  --machine-type=n1-standard-4

# Allow IP-in-IP traffic between outer cluster Nodes from inner cluster Pods
CLUSTER_CIDR=`gcloud container clusters describe $MANAGEMENT_CLUSTER_NAME --format="value(clusterIpv4Cidr)"`
gcloud compute firewall-rules create allow-$MANAGEMENT_CLUSTER_NAME-cluster-pods-ipip \
  --source-ranges=$CLUSTER_CIDR \
  --allow=ipip

# Forward IPv4 encapsulated packets
kubectl apply -f hack/forward-ipencap.yaml
```

### Installation

```sh
# Install clusterctl
# https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl
CLUSTER_API_VERSION=v0.3.14
curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/$CLUSTER_API_VERSION/clusterctl-`uname -s  | tr '[:upper:]' '[:lower:]'`-amd64 -o clusterctl
chmod +x ./clusterctl
sudo mv ./clusterctl /usr/local/bin/clusterctl

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
```

### Configuration

```sh
CLUSTER_NAME="example"

# Use ClusterIP for clusters that do not support Services of type LoadBalancer
export KUBERNETES_CONTROL_PLANE_SERVICE_TYPE="LoadBalancer"
export KUBERNETES_CONTROLLER_MACHINE_CPU_REQUEST="1"
export KUBERNETES_CONTROLLER_MACHINE_MEMORY_REQUEST="2Gi"
export KUBERNETES_WORKER_MACHINE_CPU_REQUEST="1"
export KUBERNETES_WORKER_MACHINE_MEMORY_REQUEST="1Gi"
clusterctl config cluster $CLUSTER_NAME \
  --infrastructure=kubernetes \
  --kubernetes-version=v1.20.2 \
  --control-plane-machine-count=1 \
  --worker-machine-count=1 \
  | kubectl apply -f -

# Retrieve kubeconfig
until [ -n "`kubectl get secret $CLUSTER_NAME-kubeconfig -o jsonpath='{.data.value}' 2>/dev/null`" ] ; do
  sleep 1
done
kubectl get secret $CLUSTER_NAME-kubeconfig -o jsonpath='{.data.value}' | base64 --decode > $CLUSTER_NAME-kubeconfig

# Switch to new kind cluster
# If the cluster api endpoint is not reachable from your machine you can exec into a
# controller Node (Pod) and run `export KUBECONFIG=/etc/kubernetes/admin.conf` instead
export KUBECONFIG=$CLUSTER_NAME-kubeconfig

# Wait for the apiserver to come up
until kubectl get nodes &>/dev/null; do
  sleep 1
done

# Install Calico
kubectl apply -f https://docs.projectcalico.org/manifests/calico.yaml

# Interact with your new cluster!
kubectl get nodes
```

### Clean up

```sh
unset KUBECONFIG
rm -f example-kubeconfig
kubectl delete cluster example
# If using the GKE example above
yes | gcloud compute firewall-rules delete allow-$MANAGEMENT_CLUSTER_NAME-cluster-pods-ipip
yes | gcloud container clusters delete $MANAGEMENT_CLUSTER_NAME --async
```

## TODO

- Implement finalizer for control plane Pods to prevent deletion that'd lose quorum (i.e. PDB)
- Work out why KCP replicas 3 has 0 failure tolerance
  - https://github.com/kubernetes-sigs/cluster-api/blob/master/controlplane/kubeadm/controllers/remediation.go#L158-L159
- Document how to configure persistence
- Fix persistent control plane with 3 nodes
- Default cluster service type to ClusterIP
  - https://book.kubebuilder.io/cronjob-tutorial/webhook-implementation.html
