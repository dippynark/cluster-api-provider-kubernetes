# HA Control Plane

This is managed by the [KubeadmControlPlane controller].

## Known Issues

Since the names of the Machines provisioned by the KubeadmControlPlane controller are randomly
generated rather than having a fixed identity, [persistence](persistent-control-plane) currently
does not work as expected and will result in a separate PersistentVolumeClaim being created for
each Machine rather being reused by replacement Machines.

A solution for this is required
[upstream](https://github.com/kubernetes-sigs/cluster-api/issues/1858).

[KubeadmControlPlane controller]: https://github.com/kubernetes-sigs/cluster-api/blob/master/docs/proposals/20191017-kubeadm-based-control-plane.md
