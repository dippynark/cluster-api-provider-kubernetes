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
	"flag"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"

	// TODO: use upstream implementation -- see ./boostrap package README.md for details
	//"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"github.com/dippynark/cluster-api-provider-kubernetes/e2e/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

// Test suite flags
var (
	// configPath is the path to the e2e config file.
	configPath string

	// useExistingCluster instructs the test to use the current cluster instead of creating a new one (default discovery rules apply).
	useExistingCluster bool

	// artifactFolder is the folder to store e2e test artifacts.
	artifactFolder string

	// skipCleanup prevents cleanup of test resources e.g. for debug purposes.
	skipCleanup bool
)

// Test suite global vars
var (
	// e2eConfig to be used for this test, read from configPath.
	e2eConfig *clusterctl.E2EConfig

	// clusterctlConfigPath to be used for this test, created by generating a clusterctl local repository
	// with the providers specified in the configPath.
	clusterctlConfigPath string

	// bootstrapClusterProvider manages provisioning of the the bootstrap cluster to be used for the e2e tests.
	// Please note that provisioning will be skipped if e2e.use-existing-cluster is provided.
	bootstrapClusterProvider bootstrap.ClusterProvider

	// bootstrapClusterProxy allows to interact with the bootstrap cluster to be used for the e2e tests.
	bootstrapClusterProxy framework.ClusterProxy
)

func init() {
	flag.StringVar(&configPath, "e2e.config", "", "path to the e2e config file")
	flag.StringVar(&artifactFolder, "e2e.artifacts-folder", "", "folder where e2e test artifact should be stored")
	flag.BoolVar(&skipCleanup, "e2e.skip-resource-cleanup", false, "if true, the resource cleanup after tests will be skipped")
	flag.BoolVar(&useExistingCluster, "e2e.use-existing-cluster", false, "if true, the test uses the current cluster instead of creating a new one (default discovery rules apply)")
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t, "capk-e2e", []Reporter{})
}

var _ = BeforeSuite(func() {

	Expect(configPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config should be an existing file.")
	Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid test suite argument. Can't create e2e.artifacts-folder %q", artifactFolder)

	By("Initializing a runtime.Scheme with all the GVK relevant for this test")
	scheme := initScheme()

	e2e.Byf("Loading the e2e test configuration from %q", configPath)
	e2eConfig = loadE2EConfig(configPath)

	e2e.Byf("Creating a clusterctl local repository into %q", artifactFolder)
	clusterctlConfigPath = createClusterctlLocalRepository(e2eConfig, filepath.Join(artifactFolder, "repository"))

	By("Setting up the bootstrap cluster")
	bootstrapClusterProvider, bootstrapClusterProxy = setupBootstrapCluster(e2eConfig, scheme, useExistingCluster)

	By("Initializing the bootstrap cluster")
	initBootstrapCluster(bootstrapClusterProxy, e2eConfig, clusterctlConfigPath, artifactFolder)
})

func initScheme() *runtime.Scheme {
	sc := runtime.NewScheme()
	framework.TryAddDefaultSchemes(sc)
	return sc
}

func loadE2EConfig(configPath string) *clusterctl.E2EConfig {
	config := clusterctl.LoadE2EConfig(context.TODO(), clusterctl.LoadE2EConfigInput{ConfigPath: configPath})
	Expect(config).ToNot(BeNil(), "Failed to load E2E config from %s", configPath)

	return config
}

func createClusterctlLocalRepository(config *clusterctl.E2EConfig, repositoryFolder string) string {
	createRepositoryInput := clusterctl.CreateRepositoryInput{
		E2EConfig:        config,
		RepositoryFolder: repositoryFolder,
	}

	// TODO: deploy Calico CNI
	// Ensuring a CNI file is defined in the config and register a FileTransformation to inject the referenced file in place of the CNI_RESOURCES envSubst variable.
	// Expect(config.Variables).To(HaveKey(e2e.CNIPath), "Missing %s variable in the config", e2e.CNIPath)
	// cniPath := config.GetVariable(e2e.CNIPath)
	// Expect(cniPath).To(BeAnExistingFile(), "The %s variable should resolve to an existing file", e2e.CNIPath)

	// createRepositoryInput.RegisterClusterResourceSetConfigMapTransformation(cniPath, e2e.CNIResources)

	clusterctlConfig := clusterctl.CreateRepository(context.TODO(), createRepositoryInput)
	Expect(clusterctlConfig).To(BeAnExistingFile(), "The clusterctl config file does not exists in the local repository %s", repositoryFolder)
	return clusterctlConfig
}

var _ = AfterSuite(func() {
	By("Tearing down the management cluster")
	if !skipCleanup {
		tearDown(bootstrapClusterProvider, bootstrapClusterProxy)
	}
})

func setupBootstrapCluster(config *clusterctl.E2EConfig, scheme *runtime.Scheme, useExistingCluster bool) (bootstrap.ClusterProvider, framework.ClusterProxy) {
	var clusterProvider bootstrap.ClusterProvider
	kubeconfigPath := ""
	if !useExistingCluster {
		clusterProvider = bootstrap.CreateKindBootstrapClusterAndLoadImages(context.TODO(), bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
			Name:               config.ManagementClusterName,
			RequiresDockerSock: config.HasDockerProvider(),
			Images:             config.Images,
			// NodeImage:          "kindest/node:v1.19.1",
		})
		Expect(clusterProvider).ToNot(BeNil(), "Failed to create a bootstrap cluster")

		kubeconfigPath = clusterProvider.GetKubeconfigPath()
		Expect(kubeconfigPath).To(BeAnExistingFile(), "Failed to get the kubeconfig file for the bootstrap cluster")
	}

	clusterProxy := framework.NewClusterProxy("bootstrap", kubeconfigPath, scheme)
	Expect(clusterProxy).ToNot(BeNil(), "Failed to get a bootstrap cluster proxy")

	return clusterProvider, clusterProxy
}

func initBootstrapCluster(bootstrapClusterProxy framework.ClusterProxy, config *clusterctl.E2EConfig, clusterctlConfig, artifactFolder string) {
	clusterctl.InitManagementClusterAndWatchControllerLogs(context.TODO(), clusterctl.InitManagementClusterAndWatchControllerLogsInput{
		ClusterProxy:            bootstrapClusterProxy,
		ClusterctlConfigPath:    clusterctlConfig,
		InfrastructureProviders: config.InfrastructureProviders(),
		LogFolder:               filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
	}, config.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
}

func tearDown(bootstrapClusterProvider bootstrap.ClusterProvider, bootstrapClusterProxy framework.ClusterProxy) {
	if bootstrapClusterProxy != nil {
		bootstrapClusterProxy.Dispose(context.TODO())
	}
	if bootstrapClusterProvider != nil {
		bootstrapClusterProvider.Dispose(context.TODO())
	}
}
