module github.com/dippynark/cluster-api-provider-kubernetes

go 1.13

require (
	github.com/coreos/bbolt v1.3.1-coreos.6 // indirect
	github.com/coreos/etcd v3.3.15+incompatible // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/natefinch/lumberjack v2.0.0+incompatible // indirect
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/pkg/errors v0.9.0
	gopkg.in/yaml.v1 v1.0.0-20140924161607-9f9df34309c0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20191120175047-4206685974f2
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/utils v0.0.0-20191114184206-e782cd3c129f
	sigs.k8s.io/cluster-api v0.3.0-rc.2.0.20200302175844-3011d8c2580c
	sigs.k8s.io/cluster-api/test/framework v0.0.0-20200304170348-97097699f713
	sigs.k8s.io/controller-runtime v0.5.0
	sigs.k8s.io/kind v0.7.0
	sigs.k8s.io/testing_frameworks v0.1.2 // indirect
)

replace k8s.io/client-go => k8s.io/client-go v0.16.4
