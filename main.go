/*
Copyright 2022.

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
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	karmadaClusterv1alpha1 "github.com/karmada-io/api/cluster/v1alpha1"
	karmadaPolicyv1alpha1 "github.com/karmada-io/api/policy/v1alpha1"
	karmadaWorkv1alpha2 "github.com/karmada-io/api/work/v1alpha2"
	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/fedstate/fedstate/controllers"
	"github.com/fedstate/fedstate/pkg/config"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"github.com/go-logr/zapr"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	// "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/fedstate/fedstate/api/v1alpha1"
	middlewarev1alpha1 "github.com/fedstate/fedstate/api/v1alpha1"
	//+kubebuilder:scaffold:imports

	c "github.com/fedstate/fedstate/pkg/config"
	"github.com/fedstate/fedstate/pkg/logi"
	"github.com/fedstate/fedstate/pkg/metrics"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(middlewarev1alpha1.AddToScheme(scheme))
	utilruntime.Must(karmadaPolicyv1alpha1.AddToScheme(scheme))
	utilruntime.Must(karmadaClusterv1alpha1.AddToScheme(scheme))
	utilruntime.Must(karmadaWorkv1alpha2.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {

	configFlags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	config := config.SetupFlag(configFlags)

	flagset := pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	flagset.AddGoFlagSet(configFlags)

	var metricsAddr string
	var enableLeaderElection, enableCertRotation bool
	var probeAddr string
	var kubeConfig string
	flagset.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flagset.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flagset.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flagset.StringVar(&kubeConfig, "kubeconfig", "/etc/kubeconfig", "karmada api config")
	flagset.BoolVar(&enableCertRotation, "enablecertrotation", true, "start webhook ca get")
	err := flagset.Parse(os.Args[1:])
	if err != nil {
		fmt.Printf("flagset err: %v", err)
	}

	// ctrl.SetLogger(zap.New(zap.UseFlagOptions(&logi.ZapOptions)))
	ctrl.SetLogger(zapr.NewLogger(logi.Log))

	var restConfig *rest.Config
	if config.EnableMultiCloudMongoDBController {
		loader := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfig}
		loadConfig, err := loader.Load()
		if err != nil {
			os.Exit(1)
		}
		karmadaApiRestConfig, err := clientcmd.NewNonInteractiveClientConfig(*loadConfig, c.Vip.GetString("KarmadaCxt"), &clientcmd.ConfigOverrides{}, loader).ClientConfig()
		if err != nil {
			os.Exit(1)
		}
		restConfig = karmadaApiRestConfig
	} else {
		restConfig = ctrl.GetConfigOrDie()
	}

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "d6aa819a.fedstate.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	setupFinished := make(chan struct{})
	if config.EnableMultiCloudMongoDBController {
		err := doWebhook(*middlewarev1alpha1.MultiCloudMongoWebhook, GetOperatorNamespace(), enableCertRotation, mgr, setupFinished)
		if err != nil {
			os.Exit(1)
		}
	}
	if config.EnableMongoDBController {
		err := doWebhook(*middlewarev1alpha1.MongoWebhook, GetOperatorNamespace(), enableCertRotation, mgr, setupFinished)
		if err != nil {
			os.Exit(1)
		}
	}

	go func() {
		<-setupFinished
		if config.EnableMultiCloudMongoDBController {
			if err = (&controllers.MultiCloudMongoDBReconciler{
				Client: mgr.GetClient(),
				Scheme: mgr.GetScheme(),
				Log:    logi.Log.With(zap.String("controller", "MultiCloudMongoDB")).Sugar(),
			}).SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "MultiCloudMongoDB")
				os.Exit(1)
			}

			if err = (&middlewarev1alpha1.MultiCloudMongoDB{}).SetupWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create webhook", "webhook", "MultiCloudMongoDB")
				os.Exit(1)
			}
		}
		if config.EnableMongoDBController {
			if err = (&controllers.MongoDBReconciler{
				Client: mgr.GetClient(),
				Scheme: mgr.GetScheme(),
				Log:    logi.Log.With(zap.String("controller", "MongoDB")).Sugar(),
			}).SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "MongoDB")
				os.Exit(1)
			}

			if err = (&middlewarev1alpha1.MongoDB{}).SetupWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create webhook", "webhook", "MongoDB")
				os.Exit(1)
			}
		}
	}()

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	metrics.ServeCustomMetrics()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func doWebhook(webhook v1alpha1.Webhook, namespace string, enableCertRotation bool, mgr manager.Manager, ch chan struct{}) error {
	var (
		dnsName  = fmt.Sprintf("%s.%s.svc", webhook.ServiceName, namespace)
		webhooks = []rotator.WebhookInfo{
			{
				Name: namespace + "-validating-webhook-configuration",
				Type: rotator.Validating,
			},
			{
				Name: namespace + "-mutating-webhook-configuration",
				Type: rotator.Mutating,
			},
		}
	)

	if enableCertRotation {
		setupLog.Info("setting up cert rotation")
		err := rotator.AddRotator(mgr, &rotator.CertRotator{
			SecretKey: types.NamespacedName{
				Namespace: namespace,
				Name:      webhook.DefaultSecretName,
			},
			CertDir:                webhook.CertDir,
			CAName:                 webhook.CaName,
			CAOrganization:         webhook.CaOrganization,
			DNSName:                dnsName,
			IsReady:                ch,
			Webhooks:               webhooks,
			RestartOnSecretRefresh: true,
		})
		if err != nil {
			setupLog.Error(err, "unable to set up cert rotation")
			return err
		}
	} else {
		close(ch)
	}
	return nil

}

func GetOperatorNamespace() string {
	nsBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		setupLog.Error(err, "unable to read file")
		if os.IsNotExist(err) {
			return "operators"
		}
	}
	ns := strings.TrimSpace(string(nsBytes))
	return ns
}
