# E2E Tests

This project uses the [Cluster API end-to-end testing
framework](https://github.com/kubernetes-sigs/cluster-api/tree/master/test/framework) to run
end-to-end tests on a local [kind](https://github.com/kubernetes-sigs/kind) cluster. The
configuration file used by the framework can be found in [e2e/e2e.conf](../e2e/e2e.conf).

Image dependencies are loaded into the kind cluster before the Cluster API managed cluster is
provisioned. A development `cluster-api-kubernetes-controller` image can be built using `make
docker-build` and the remaining images pulled using `make e2e_pull`.

The e2e tests can then be run using `make e2e`.
