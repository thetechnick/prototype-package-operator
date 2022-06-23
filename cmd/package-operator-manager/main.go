package main

import (
	"context"
	"flag"
	"net/http"
	"net/http/pprof"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	packageapis "github.com/thetechnick/package-operator/apis"
	"github.com/thetechnick/package-operator/internal/controllers/packages/objectdeployments"
	"github.com/thetechnick/package-operator/internal/controllers/packages/objectsetphases"
	"github.com/thetechnick/package-operator/internal/controllers/packages/objectsets"
	"github.com/thetechnick/package-operator/internal/controllers/packages/packages"
	"github.com/thetechnick/package-operator/internal/dynamicwatcher"
	"github.com/thetechnick/package-operator/internal/ownerhandling"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = packageapis.AddToScheme(scheme)
}

func main() {
	var (
		metricsAddr          string
		pprofAddr            string
		enableLeaderElection bool
		namespace            string
	)
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&pprofAddr, "pprof-addr", "", "The address the pprof web endpoint binds to.")
	flag.StringVar(&namespace, "namespace", os.Getenv("PKO_NAMESPACE"), "xx")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                     scheme,
		MetricsBindAddress:         metricsAddr,
		Port:                       9443,
		LeaderElectionResourceLock: "leases",
		LeaderElection:             enableLeaderElection,
		LeaderElectionID:           "8a4hp84a6s.package-operator-lock",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Dynamic Watcher
	dynamicClient, err := dynamic.NewForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "unable to setup dynamic client")
		os.Exit(1)
	}

	// -----
	// PPROF
	// -----
	if len(pprofAddr) > 0 {
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		s := &http.Server{Addr: pprofAddr, Handler: mux}
		err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
			errCh := make(chan error)
			defer func() {
				for range errCh {
				} // drain errCh for GC
			}()
			go func() {
				defer close(errCh)
				errCh <- s.ListenAndServe()
			}()

			select {
			case err := <-errCh:
				return err
			case <-ctx.Done():
				s.Close()
				return nil
			}
		}))
		if err != nil {
			setupLog.Error(err, "unable to create pprof server")
			os.Exit(1)
		}
	}

	// Dynamic Watcher
	dw := dynamicwatcher.New(
		ctrl.Log.WithName("DynamicWatcher"),
		mgr.GetScheme(), mgr.GetClient().RESTMapper(),
		dynamicClient)

	// ObjectSet
	if err = (objectsets.NewObjectSetController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("ObjectSet"),
		mgr.GetScheme(), dw,
	).SetupWithManager(mgr)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ObjectSet")
		os.Exit(1)
	}
	if err = (objectsets.NewClusterObjectSetController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("ClusterObjectSet"),
		mgr.GetScheme(), dw,
	).SetupWithManager(mgr)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterObjectSet")
		os.Exit(1)
	}

	// ObjectSetPhase
	if err = (objectsetphases.NewObjectSetPhaseController(
		"default", ownerhandling.Native,
		mgr.GetClient(), mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("ObjectSetPhase"),
		mgr.GetScheme(), dw,
	).SetupWithManager(mgr)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ObjectSetPhase")
		os.Exit(1)
	}
	if err = (objectsetphases.NewClusterObjectSetPhaseController(
		"default", ownerhandling.Native,
		mgr.GetClient(), mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("ClusterObjectSetPhase"),
		mgr.GetScheme(), dw,
	).SetupWithManager(mgr)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterObjectSetPhase")
		os.Exit(1)
	}

	// ObjectDeployment
	if err = (objectdeployments.NewObjectDeploymentController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("ObjectDeployment"),
		mgr.GetScheme(),
	).SetupWithManager(mgr)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ObjectDeployment")
		os.Exit(1)
	}
	if err = (objectdeployments.NewClusterObjectDeploymentController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("ClusterObjectDeployment"),
		mgr.GetScheme(),
	).SetupWithManager(mgr)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterObjectDeployment")
		os.Exit(1)
	}

	// Package
	if err = (packages.NewPackageController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("Package"),
		mgr.GetScheme(), namespace,
	).SetupWithManager(mgr)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Package")
		os.Exit(1)
	}
	if err = (packages.NewClusterPackageController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("ClusterPackage"),
		mgr.GetScheme(), namespace,
	).SetupWithManager(mgr)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterPackage")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
