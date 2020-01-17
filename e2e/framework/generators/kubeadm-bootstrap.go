/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package generators

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/dippynark/cluster-api-provider-kubernetes/e2e/framework/exec"
	"github.com/pkg/errors"
)

// KubeadmBootstrapGitHubManifestsFormat is a convenience string to get Cluster API manifests at an exact revision.
// Set KubeadmBootstrap.KustomizePath = fmt.Sprintf(KubeadmBootstrapGitHubManifestsFormat, <some git ref>).
var KubeadmBootstrapGitHubManifestsFormat = "https://github.com/kubernetes-sigs/cluster-api//bootstrap/kubeadm/config/default?ref=%s"

// KubeadmBootstrap generates provider components for the Kubeadm bootstrap provider.
type KubeadmBootstrap struct {
	// KustomizePath is a URL, relative or absolute filesystem path to a kustomize file that generates the Kubeadm Bootstrap manifests.
	// KustomizePath takes precedence over Version.
	KustomizePath string
	// Version defines the release version. If GitRef is not set Version must be set and will not use kustomize.
	Version string
}

// GetName returns the name of the components being generated.
func (g *KubeadmBootstrap) GetName() string {
	if g.KustomizePath != "" {
		return fmt.Sprintf("Using Kubeadm bootstrap provider manifests from: %q", g.KustomizePath)
	}
	return fmt.Sprintf("Kubeadm bootstrap provider manifests from Cluster API version release %s", g.Version)
}

func (g *KubeadmBootstrap) releaseYAMLPath() string {
	return fmt.Sprintf("https://github.com/kubernetes-sigs/cluster-api-bootstrap-provider-kubeadm/releases/download/%s/bootstrap-components.yaml", g.Version)
}

// Manifests return the generated components and any error if there is one.
func (g *KubeadmBootstrap) Manifests(ctx context.Context) ([]byte, error) {
	if g.KustomizePath != "" {
		kustomize := exec.NewCommand(
			exec.WithCommand("kustomize"),
			exec.WithArgs("build", g.KustomizePath),
		)
		stdout, stderr, err := kustomize.Run(ctx)
		if err != nil {
			fmt.Println(string(stderr))
			return nil, errors.WithStack(err)
		}
		stdout = bytes.Replace(stdout, []byte("imagePullPolicy: Always"), []byte("imagePullPolicy: IfNotPresent"), -1)
		return stdout, nil
	}
	resp, err := http.Get(g.releaseYAMLPath())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	out, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer resp.Body.Close()
	return out, nil
}
