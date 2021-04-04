module github.com/dippynark/cluster-api-provider-kubernetes

go 1.13

require (
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.10.5
	github.com/pkg/errors v0.9.1
	k8s.io/api v0.17.9
	k8s.io/apimachinery v0.17.9
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/utils v0.0.0-20200619165400-6e3d28b6ed19
	sigs.k8s.io/cluster-api v0.3.15
	sigs.k8s.io/controller-runtime v0.5.14
	sigs.k8s.io/kind v0.7.1-0.20200303021537-981bd80d3802
)

replace k8s.io/client-go => k8s.io/client-go v0.17.9
