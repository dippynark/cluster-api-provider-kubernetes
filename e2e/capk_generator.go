package e2e

import (
	"context"
	"fmt"

	"github.com/dippynark/cluster-api-provider-kubernetes/e2e/framework/exec"
	"github.com/pkg/errors"
)

type provider struct{}

// GetName returns the name of the components being generated.
func (g *provider) GetName() string {
	return "Cluster API Provider Kubernetes version: Local files"
}

func (g *provider) kustomizePath(path string) string {
	return "../config/" + path
}

// Manifests return the generated components and any error if there is one.
func (g *provider) Manifests(ctx context.Context) ([]byte, error) {
	kustomize := exec.NewCommand(
		exec.WithCommand("kustomize"),
		exec.WithArgs("build", g.kustomizePath("default")),
	)
	stdout, stderr, err := kustomize.Run(ctx)
	if err != nil {
		fmt.Println(string(stderr))
		return nil, errors.WithStack(err)
	}

	manifests := stdout

	cat := exec.NewCommand(
		exec.WithCommand("cat"),
		exec.WithArgs("../config/samples/capi-kubernetes-rbac.yaml"),
	)
	stdout, stderr, err = cat.Run(ctx)
	if err != nil {
		fmt.Println(string(stderr))
		return nil, errors.WithStack(err)
	}

	manifests = append(manifests, []byte("\n---\n")...)
	manifests = append(manifests, stdout...)

	return manifests, nil
}
