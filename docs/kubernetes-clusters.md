# KubernetesClusters

KubernetesCluster resources represent a Kubernetes Service that can be used to reach the API
Server(s) running on the controller Pod(s).

The `controlPlaneServiceType` field can be set to specify the type of the Service. Currently only
ClusterIP and LoadBalancer are supported. ClusterIP is the default if the field is omitted:

```yaml
apiVersion: infrastructure.lukeaddison.co.uk/v1alpha3
kind: KubernetesCluster
metadata:
  name: example
spec:
  controlPlaneServiceType: ClusterIP
```
