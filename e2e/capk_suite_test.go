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
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	capkv1 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha3"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
)

var (
	mgmt       *CAPKCluster
	ctx        = context.Background()
	config     *framework.Config
	configPath string
)

func init() {
	flag.StringVar(&configPath, "e2e.config", "e2e.conf", "Path to the e2e config file")
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{})
}

var _ = BeforeSuite(func() {
	By("loading e2e config")
	configData, err := ioutil.ReadFile(configPath)
	Expect(err).ShouldNot(HaveOccurred())
	config, err = framework.LoadConfig(configData)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(config).ShouldNot(BeNil())

	By("initializing the scheme")
	scheme := runtime.NewScheme()
	framework.TryAddDefaultSchemes(scheme)
	Expect(capkv1.AddToScheme(scheme)).To(Succeed())

	By("initialzing the management cluster name")
	config.ManagementClusterName = os.Getenv("CAPI_MGMT_CLUSTER_NAME")
	if config.ManagementClusterName == "" {
		config.ManagementClusterName = "capk-e2e-" + util.RandomString(6)
	}

	managementCluster := framework.InitManagementCluster(
		ctx, &framework.InitManagementClusterInput{
			Config: *config,
			Scheme: scheme,
			NewManagementClusterFn: func() (framework.ManagementCluster, error) {
				return NewClusterForCAPK(ctx, config.ManagementClusterName, scheme)
			},
		})
	Expect(managementCluster).ToNot(BeNil())
	Expect(managementCluster).To(BeAssignableToTypeOf(&CAPKCluster{}))
	mgmt = managementCluster.(*CAPKCluster)

	fmt.Printf("export KUBECONFIG=%q\n", mgmt.KubeconfigPath)
})

var _ = AfterSuite(func() {
	By("deleting the management cluster")
	// If any part of teardown fails it will print what must be manually cleaned up
	mgmt.Teardown(ctx)
})
