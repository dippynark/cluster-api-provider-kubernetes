# Kubernetes Cluster API Provider Kubernetes

This project is a [Cluster API
Provider](https://cluster-api.sigs.k8s.io/reference/providers.html)
implementation using Kubernetes as the infrastructure provider. Pods running
[kind](https://github.com/kubernetes-sigs/kind) are created and configured to
serve as Nodes which form a cluster.

## Quickstart

We will create the infrastructure to run the Cluster API controllers, install
the controllers and then configure an example cluster using the Kubernetes
infrastructure provider.

### Infrastructure

Any recent Kubernetes cluster should be suitable (compatibility matrix to come).
The manifests below assume we are running on a cluster that supports
[LoadBalancer Service] types and [dynamic volume provisioning].

### Installation

```bash
# Install cluster api
kubectl apply -f https://github.com/kubernetes-sigs/cluster-api/releases/download/v0.2.7/cluster-api-components.yaml

# Install kubeadm bootstrap provider
kubectl apply -f https://github.com/kubernetes-sigs/cluster-api-bootstrap-provider-kubeadm/releases/download/v0.1.5/bootstrap-components.yaml

# Install kubernetes infrastructure provider
kubectl apply -f https://github.com/dippynark/cluster-api-provider-kubernetes/releases/download/v0.1.0/provider-components.yaml

# Allow cluster api controller to interact with kubernetes infrastructure resources
# If the kubernetes provider were SIG-sponsored this would not be necesarry ;)
# https://cluster-api.sigs.k8s.io/providers/v1alpha1-to-v1alpha2.html#the-new-api-groups
kubectl apply -f https://github.com/dippynark/cluster-api-provider-kubernetes/releases/download/v0.1.0/capi-kubernetes-rbac.yaml
```

### Configuration

```sh
# Apply cluster infrastructure
kubectl apply -f <(cat <<EOF
kind: KubernetesCluster
apiVersion: infrastructure.lukeaddison.co.uk/v1alpha1
metadata:
  name: example
spec:
  # Change or remove for clusters that do not support LoadBalancer Service types
  apiServerServiceType: LoadBalancer
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
  containers:
  - name: kind
    resources:
      requests:
        cpu: 200m
        memory: 200Mi
    volumeMounts:
    - name: etc-kubernetes
      mountPath: /etc/kubernetes
    - name: var-lib-kubelet
      mountPath: /var/lib/kubelet
    - name: var-lib-etcd
      mountPath: /var/lib/etcd
  volumeClaimTemplates:
  - metadata:
      name: etc-kubernetes
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 1Gi
  - metadata:
      name: var-lib-kubelet
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 1Gi
  - metadata:
      name: var-lib-etcd
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 10Gi
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
kind: KubernetesMachineTemplate
apiVersion: infrastructure.lukeaddison.co.uk/v1alpha1
metadata:
  name: worker
spec:
  template:
    spec:
      containers:
      - name: kind
        resources:
          requests:
            cpu: 100m
            memory: 100Mi
        volumeMounts:
        - name: etc-kubernetes
          mountPath: /etc/kubernetes
        - name: var-lib-kubelet
          mountPath: /var/lib/kubelet
      volumeClaimTemplates:
      - metadata:
          name: etc-kubernetes
        spec:
          accessModes:
          - ReadWriteOnce
          resources:
            requests:
              storage: 1Gi
      - metadata:
          name: var-lib-kubelet
        spec:
          accessModes:
          - ReadWriteOnce
          resources:
            requests:
              storage: 1Gi
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
EOF
)

# Retrieve kubeconfig
until [ -n "`kubectl get secret example-kubeconfig -o jsonpath='{.data.value}' 2>/dev/null`" ] ; do
  sleep 1
done
kubectl get secret example-kubeconfig -o jsonpath='{.data.value}' | base64 --decode > example-kubeconfig

# Switch to example cluster
export KUBECONFIG=example-kubeconfig

# Wait for the apiserver to come up
# If the example-lb Service is not reachable from your machine consider
# port-forwarding to it
until kubectl get nodes &>/dev/null; do
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
rm example-kubeconfig
kubectl delete cluster example
```

[dynamic volume provisioning]: https://kubernetes.io/docs/concepts/storage/dynamic-provisioning/
[LoadBalancer Service]: https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer
