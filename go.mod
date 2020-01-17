module github.com/dippynark/cluster-api-provider-kubernetes

go 1.13

require (
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.10.3
	github.com/onsi/gomega v1.7.1
	github.com/pkg/errors v0.8.1
	gopkg.in/yaml.v3 v3.0.0-20191106092431-e228e37189d3
	k8s.io/api v0.16.4
	k8s.io/apimachinery v0.16.4
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/utils v0.0.0-20191030222137-2b95a09bc58d
	sigs.k8s.io/cluster-api v0.2.6-0.20191223162332-fd807a3d843b
	sigs.k8s.io/cluster-api/test/framework v0.0.0-20200117021457-961aa48ba85f
	sigs.k8s.io/controller-runtime v0.4.0
	sigs.k8s.io/kind v0.6.1
)

replace k8s.io/client-go => k8s.io/client-go v0.16.4
