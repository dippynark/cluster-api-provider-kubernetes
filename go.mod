module github.com/dippynark/cluster-api-provider-kubernetes

go 1.13

require (
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/pengsrc/go-shared v0.2.0
	github.com/pkg/errors v0.8.1
	go.uber.org/zap v1.10.0
	k8s.io/api v0.17.1
	k8s.io/apiextensions-apiserver v0.17.1 // indirect
	k8s.io/apimachinery v0.17.1
	k8s.io/client-go v0.17.1
	k8s.io/utils v0.0.0-20191114184206-e782cd3c129f
	sigs.k8s.io/cluster-api v0.2.8
	sigs.k8s.io/controller-runtime v0.4.0
	sigs.k8s.io/controller-tools v0.2.4 // indirect
	sigs.k8s.io/kind v0.6.1
)
