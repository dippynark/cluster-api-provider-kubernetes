package e2e

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api/test/framework/exec"
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

	// Since this is not a SIG-sponsored provider we need to give the Cluster API manager extra
	// RBAC permissions for resources defined in this project
	// https://cluster-api.sigs.k8s.io/providers/v1alpha1-to-v1alpha2.html#the-new-api-groups
	rbacManifests, err := ioutil.ReadFile("../config/samples/capi-kubernetes-rbac.yaml")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	manifests = append(manifests, []byte("\n---\n")...)
	manifests = append(manifests, rbacManifests...)

	return manifests, nil
}
