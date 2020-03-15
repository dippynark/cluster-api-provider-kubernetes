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

package main

import (
	"flag"
	"os"

	capkv1alpha2 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha2"
	capkv1 "github.com/dippynark/cluster-api-provider-kubernetes/api/v1alpha3"
	"github.com/dippynark/cluster-api-provider-kubernetes/controllers"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	coreV1Client "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)

	_ = capkv1.AddToScheme(scheme)
	_ = capkv1alpha2.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var watchNamespace string
	var enableLeaderElection bool
	var enableWebhook bool
	var debug bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&watchNamespace, "namespace", "",
		"Namespace that the controller watches to reconcile cluster-api objects. If unspecified, the controller watches for cluster-api objects across all namespaces.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&debug, "debug", false, "Enable debug logging.")
	flag.BoolVar(&enableWebhook, "enable-webhook", false,
		"Disabled by default. When enabled, the manager will only work as webhook server, no reconcilers are installed.")
	flag.Parse()

	if watchNamespace != "" {
		setupLog.Info("Watching cluster-api objects only in namespace for reconciliation", "namespace", watchNamespace)
	}

	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = debug
	}))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		Port:               9443,
		Namespace:          watchNamespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if !enableWebhook {

		if err = (&controllers.KubernetesClusterReconciler{
			Client: mgr.GetClient(),
			Log:    ctrl.Log.WithName("controller").WithName("KubernetesCluster"),
			Scheme: mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "KubernetesCluster")
			os.Exit(1)
		}

		// Create a Kubernetes core/v1 client.
		config := mgr.GetConfig()
		coreV1Client, err := coreV1Client.NewForConfig(config)
		if err != nil {
			setupLog.Error(err, "unable to initialise core client")
			os.Exit(1)
		}

		if err = (&controllers.KubernetesMachineReconciler{
			Client:       mgr.GetClient(),
			CoreV1Client: coreV1Client,
			Config:       config,
			Log:          ctrl.Log.WithName("controller").WithName("KubernetesMachine"),
			Scheme:       mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "KubernetesMachine")
			os.Exit(1)
		}
	} else {

		// Setup webhooks
		if err = (&capkv1.KubernetesCluster{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "KubernetesCluster")
			os.Exit(1)
		}
		if err = (&capkv1.KubernetesMachine{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "KubernetesMachine")
			os.Exit(1)
		}
		if err = (&capkv1.KubernetesMachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "KubernetesMachineTemplate")
			os.Exit(1)
		}
		if err = (&capkv1alpha2.KubernetesMachine{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "KubernetesMachine")
			os.Exit(1)
		}
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
