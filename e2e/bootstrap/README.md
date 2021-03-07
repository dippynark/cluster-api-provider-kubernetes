# bootstrap

We copy the upstream implementation of the bootstrap test framework package and make changes to
allow the image used for the kind cluster to be specified.

```sh
cp /Users/luke/go/pkg/mod/sigs.k8s.io/cluster-api@v0.3.14/test/framework/bootstrap/* e2e/bootstrap
```

Make the following changes to kind_provider.go

const (
	// DefaultNodeImage is the default node image to be used for for testing.
	DefaultNodeImage = "kindest/node:v1.19.1"
)

// WithNodeImage implements a New Option that instruct the kindClusterProvider to use a specific node image / Kubernetes version
func WithNodeImage(image string) KindClusterOption {
	return kindClusterOptionAdapter(func(k *kindClusterProvider) {
		k.nodeImage = image
	})
}

nodeImage      string

nodeImage := DefaultNodeImage
if k.nodeImage != "" {
  nodeImage = k.nodeImage
}
kindCreateOptions = append(kindCreateOptions, kind.CreateWithNodeImage(nodeImage))


func Logf(format string, a ...interface{}) {
	fmt.Fprintf(GinkgoWriter, "INFO: "+format+"\n", a...)
}

Make the following changes to kind_urtil.go

// Image to use for kind node
NodeImage  string

if input.NodeImage {
  options = append(options, WithNodeImage(input.NodeImage))
}
