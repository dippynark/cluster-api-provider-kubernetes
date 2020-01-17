/*

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

package e2e

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	capkv1 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/generators"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	mgmt    *CAPKCluster
	ctx     = context.Background()
	logPath string
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{envtest.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	// Create the logs directory
	artifactPath := os.Getenv("ARTIFACTS")
	logPath = path.Join(artifactPath, "logs")
	Expect(os.MkdirAll(filepath.Dir(logPath), 0755)).To(Succeed())

	// Figure out the names of the images to load into kind
	managerImage := os.Getenv("MANAGER_IMAGE")
	if managerImage == "" {
		managerImage = "dippynark/cluster-api-kubernetes-controller:dev"
	}
	capiImage := os.Getenv("CAPI_IMAGE")
	if capiImage == "" {
		capiImage = "gcr.io/k8s-staging-cluster-api/cluster-api-controller:master"
	}
	capiKubeadmBootstrapImage := os.Getenv("CAPI_KUBEADM_BOOTSTRAP_IMAGE")
	if capiKubeadmBootstrapImage == "" {
		capiKubeadmBootstrapImage = "gcr.io/k8s-staging-cluster-api/kubeadm-bootstrap-controller:master"
	}
	capiKubeadmControlPlaneImage := os.Getenv("CAPI_KUBEADM_CONTROL_PLANE_IMAGE")
	if capiKubeadmControlPlaneImage == "" {
		capiKubeadmControlPlaneImage = "gcr.io/k8s-staging-cluster-api/kubeadm-control-plane-controller:master"
	}
	By("Setting up test environment")
	var err error

	// Set up the provider component generators
	core := &generators.ClusterAPI{KustomizePath: "../../../../sigs.k8s.io/cluster-api/config/default"}
	bootstrap := &generators.KubeadmBootstrap{KustomizePath: "../../../../sigs.k8s.io/cluster-api/bootstrap/kubeadm/config/default"}
	controlPlane := &generators.KubeadmControlPlane{KustomizePath: "../../../../sigs.k8s.io/cluster-api/controlplane/kubeadm/config/default"}
	// Set up capk components based on current files
	capk := &provider{}

	// Set up cert manager
	cm := &generators.CertManager{ReleaseVersion: "v0.11.1"}

	scheme := runtime.NewScheme()
	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	Expect(appsv1.AddToScheme(scheme)).To(Succeed())
	Expect(capiv1.AddToScheme(scheme)).To(Succeed())
	Expect(cabpkv1.AddToScheme(scheme)).To(Succeed())
	Expect(capkv1.AddToScheme(scheme)).To(Succeed())
	Expect(controlplanev1.AddToScheme(scheme)).To(Succeed())

	// Create the management cluster
	kindClusterName := os.Getenv("CAPI_MGMT_CLUSTER_NAME")
	if kindClusterName == "" {
		kindClusterName = "capk-e2e-" + util.RandomString(6)
	}
	mgmt, err = NewClusterForCAPK(ctx, kindClusterName, scheme, managerImage, capiImage, capiKubeadmBootstrapImage, capiKubeadmControlPlaneImage)
	Expect(err).NotTo(HaveOccurred())
	Expect(mgmt).NotTo(BeNil())

	// Install the cert-manager components first as some CRDs there will be part of the other providers
	framework.InstallComponents(ctx, mgmt, cm)

	// Wait for cert manager service
	// TODO: consider finding a way to make this service name dynamic.
	framework.WaitForAPIServiceAvailable(ctx, mgmt, "v1beta1.webhook.cert-manager.io")

	// Install all components
	framework.InstallComponents(ctx, mgmt, core, bootstrap, controlPlane, capk)
	framework.WaitForPodsReadyInNamespace(ctx, mgmt, "capi-system")
	framework.WaitForPodsReadyInNamespace(ctx, mgmt, "capi-kubeadm-bootstrap-system")
	framework.WaitForPodsReadyInNamespace(ctx, mgmt, "capi-kubeadm-control-plane-system")
	framework.WaitForPodsReadyInNamespace(ctx, mgmt, "capk-system")
	framework.WaitForPodsReadyInNamespace(ctx, mgmt, "cert-manager")

	close(done)
}, 300)

var _ = AfterSuite(func() {
	By("Tearing down test environment")
	Expect(mgmt.Teardown(ctx)).To(Succeed())
})
