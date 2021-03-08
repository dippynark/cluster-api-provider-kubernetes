# Remediation

Remediation refers to the recovery mechanism for failed Machines. By default Machines (implemented
as Pods for KubernetesMachines) will never restart and will be marked as failed if they exit after
being started.

Worker Machine remediation is achieved using a
[MachineHealthCheck](https://cluster-api.sigs.k8s.io/tasks/healthcheck.html). Failed Machines will
be deleted if they are managed by
[MachineSet](https://cluster-api.sigs.k8s.io/developer/architecture/controllers/machine-set.html)
causing them to be recreated.

Failed controller Machines will only be deleted if they are managed by a [Control
Plane](https://cluster-api.sigs.k8s.io/developer/architecture/controllers/control-plane.html)
controller (for example a
[KubeadmControlPlane](https://github.com/kubernetes-sigs/cluster-api/blob/master/docs/proposals/20191017-kubeadm-based-control-plane.md))
and even then [only when a set of conditions are
satisfied](https://github.com/kubernetes-sigs/cluster-api/blob/master/docs/proposals/20191017-kubeadm-based-control-plane.md#remediation-using-delete-and-recreate).
In particular, a KubeadmControlPlane [must have at least 3
Machines](https://github.com/kubernetes-sigs/cluster-api/blob/df91a0e8db5ae2e8cc7b84e440850d614abe26e0/controlplane/kubeadm/controllers/remediation.go#L153-L165)
for remediation to work (although from my testing 5 are required to give a failure threshold of 1,
but I believe this to be a bug). By using a KubeadmControlPlane with enough replicas, controller
Machines can fail even if the underlying KubernetesMachines and Pods are stateless.
