module github.com/dippynark/cluster-api-provider-kubernetes

go 1.13

require (
	github.com/ghodss/yaml v0.0.0-20180820084758-c7ce16629ff4
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/pengsrc/go-shared v0.2.0
	github.com/pkg/errors v0.8.1
	go.uber.org/zap v1.9.1
	k8s.io/api v0.16.4
	k8s.io/apimachinery v0.16.4
	k8s.io/client-go v0.16.4
	k8s.io/utils v0.0.0-20190809000727-6c36bc71fc4a
	sigs.k8s.io/cluster-api v0.2.8
	sigs.k8s.io/controller-runtime v0.4.0
	sigs.k8s.io/kind v0.6.1
)
