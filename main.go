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
	"os"
	"runtime"
	"strconv"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"com.ramos/es-provisioner/pkg/es"
	"github.com/joho/godotenv"

	k8Runtime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	esprovisionerv1 "com.ramos/es-provisioner/api/v1"
	"com.ramos/es-provisioner/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = k8Runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(esprovisionerv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func setEnvironment() {
	env := os.Getenv("ENV")
	setupLog.Info("env %s", env)
	e := godotenv.Load("/etc/" + env + ".env")
	if e != nil {
		e = godotenv.Load()
	}
	handleError(e, true)

	// concurrency level, use 2x number of vCPU
	runtime.GOMAXPROCS(runtime.NumCPU() * 2)
	setupLog.Info("GOMAXPROCS %v", runtime.GOMAXPROCS(-1))
}

func handleError(err error, fatal ...bool) {
	if err != nil {
		setupLog.Error(err, "ERROR: %v", err)
		if len(fatal) > 0 && fatal[0] {
			panic(err)
		}
	}
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	setEnvironment()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "983ceab2.com.ramos",
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
	retries, _ := strconv.Atoi(os.Getenv("RETRIES"))
	esOps := es.EsOptions{
		Connection: os.Getenv("ES_URL"),
		Username:   os.Getenv("ES_USERNAME"),
		Password:   os.Getenv("ES_PASSWORD"),
		Retries:    retries,
	}
	esService, err := es.NewEsService(&esOps)
	if err != nil {
		setupLog.Error(err, "unable to initialize ElasticSearch")
		os.Exit(1)
	}

	// creates the clientset
	k8sClient, err := kubernetes.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		setupLog.Error(err, "Error Creating K8s Client")
		os.Exit(1)
	}

	if err = (&controllers.IndexReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		EsService: &esService,
		K8sClient: k8sClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Index")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
