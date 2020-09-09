//###############################################################################
//# Copyright (c) 2020 Red Hat, Inc.
//###############################################################################

package main

import (
	"flag"
	"fmt"
	"os"
	goruntime "runtime"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/open-cluster-management/klusterlet-addon-lease-controller/controllers"
	"github.com/open-cluster-management/klusterlet-addon-lease-controller/pkg/bindata"

	corev1 "k8s.io/api/core/v1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

var (
	operatorMetricsPort int = 8687
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(corev1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme

	//The flag must be set here because the main_test.go used in the `go test` set also some parameters
	//and so these paramters are not registered if set in the main as the `go test` make a parse before
	//they are created.
	flag.StringVar(&metricsHost, "metrics-host", "0.0.0.0", "The host the metric endpoint binds to.")
	flag.StringVar(&metricsPort, "metrics-port", "8384", "The address the metric endpoint binds to.")
	flag.StringVar(&leaseName, "lease-name", "", "The lease name")
	flag.StringVar(&leaseNamespace, "lease-namespace", "", "The lease namespace")
	flag.StringVar(&hubConfigSecretName, "hub-kubeconfig-secret", "", "The lease namespace")
	flag.IntVar(&leaseDurationSeconds, "lease-duration", 60, "The lease duration in seconds, default 60 sec.")
	flag.IntVar(&startupDelay, "startup-delay", 10, "The startup delay in seconds, default 10 sec.")
}

func printVersion() {
	setupLog.Info(fmt.Sprintf("Go Version: %s", goruntime.Version()))
	setupLog.Info(fmt.Sprintf("Go OS/Arch: %s/%s", goruntime.GOOS, goruntime.GOARCH))
	n, err := bindata.Asset("COMPONENT_NAME/COMPONENT_NAME")
	if err != nil {
		setupLog.Error(err, "./COMPONENT_NAME file not available")
	}
	v, err := bindata.Asset("COMPONENT_VERSION/COMPONENT_VERSION")
	if err != nil {
		setupLog.Error(err, "./COMPONENT_VERSION file not available")
	}
	setupLog.Info(fmt.Sprintf("Component name/version: %s@%s", string(n), string(v)))
}

var metricsHost string
var metricsPort string
var leaseName string
var leaseNamespace string
var hubConfigSecretName string
var leaseDurationSeconds int
var startupDelay int

func main() {
	//The parse is set here in case we don't use the `go test`
	flag.Parse()
	var enableLeaderElection bool
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	if leaseName == "" || leaseNamespace == "" {
		flag.Usage()
		setupLog.Error(fmt.Errorf("Missing parameters:"), "")
		os.Exit(1)
	}

	enableLeaderElection = false
	if _, err := rest.InClusterConfig(); err == nil {
		setupLog.Info("LeaderElection enabled as running in a cluster")
		enableLeaderElection = true
	} else {
		setupLog.Info("LeaderElection disabled as not running in a cluster")
	}

	printVersion()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		Namespace:          os.Getenv("WATCH_NAMESPACE"),
		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
		Port:               operatorMetricsPort,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   leaseName + "addon-lease.agent.open-cluster-management.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.LeaseReconciler{
		Client:                        mgr.GetClient(),
		Log:                           ctrl.Log.WithName("controllers").WithName("Lease"),
		Scheme:                        mgr.GetScheme(),
		LeaseName:                     leaseName,
		LeaseNamespace:                leaseNamespace,
		LeaseDurationSeconds:          int32(leaseDurationSeconds),
		HubConfigSecretName:           hubConfigSecretName,
		BuildKubeClientWithSecretFunc: controllers.BuildKubeClientWithSecret,
		PodName:                       os.Getenv("POD_NAME"),
		PodNamespace:                  os.Getenv("POD_NAMESPACE"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Lease")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder
	setupLog.Info(fmt.Sprintf("Waiting to startup... %d seconds", startupDelay))
	time.Sleep(time.Duration(startupDelay) * time.Second)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
