package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
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

type opts struct {
	metricsAddr          string
	pprofAddr            string
	enableLeaderElection bool
	namespace            string
	probeAddr            string
}

func main() {
	var opts opts
	flag.StringVar(&opts.metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&opts.pprofAddr, "pprof-addr", "", "The address the pprof web endpoint binds to.")
	flag.StringVar(&opts.namespace, "namespace", os.Getenv("PKO_NAMESPACE"), "xx")
	flag.BoolVar(&opts.enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&opts.probeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	if err := run(opts); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
}

func run(opts opts) error {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                     scheme,
		MetricsBindAddress:         opts.metricsAddr,
		HealthProbeBindAddress:     opts.probeAddr,
		Port:                       9443,
		LeaderElectionResourceLock: "leases",
		LeaderElection:             opts.enableLeaderElection,
		LeaderElectionID:           "8a4hp84a6s.package-operator-lock",
	})
	if err != nil {
		return fmt.Errorf("creating manager: %w", err)
	}

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	// Dynamic Watcher
	dynamicClient, err := dynamic.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("unable to setup dynamic client: %w", err)
	}

	// -----
	// PPROF
	// -----
	if len(opts.pprofAddr) > 0 {
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		s := &http.Server{Addr: opts.pprofAddr, Handler: mux}
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
			return fmt.Errorf("unable to create pprof server: %w", err)
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
		return fmt.Errorf("unable to create controller for ObjectSet: %w", err)

	}
	if err = (objectsets.NewClusterObjectSetController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("ClusterObjectSet"),
		mgr.GetScheme(), dw,
	).SetupWithManager(mgr)); err != nil {
		return fmt.Errorf("unable to create controller for ClusterObjectSet: %w", err)

	}

	// ObjectSetPhase
	if err = (objectsetphases.NewObjectSetPhaseController(
		"default", ownerhandling.Native,
		mgr.GetClient(), mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("ObjectSetPhase"),
		mgr.GetScheme(), dw,
	).SetupWithManager(mgr)); err != nil {
		return fmt.Errorf("unable to create controller for ObjectSetPhase: %w", err)

	}
	if err = (objectsetphases.NewClusterObjectSetPhaseController(
		"default", ownerhandling.Native,
		mgr.GetClient(), mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("ClusterObjectSetPhase"),
		mgr.GetScheme(), dw,
	).SetupWithManager(mgr)); err != nil {
		return fmt.Errorf("unable to create controller for ClusterObjectSetPhase: %w", err)

	}

	// ObjectDeployment
	if err = (objectdeployments.NewObjectDeploymentController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("ObjectDeployment"),
		mgr.GetScheme(),
	).SetupWithManager(mgr)); err != nil {
		return fmt.Errorf("unable to create controller for ObjectDeployment: %w", err)

	}
	if err = (objectdeployments.NewClusterObjectDeploymentController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("ClusterObjectDeployment"),
		mgr.GetScheme(),
	).SetupWithManager(mgr)); err != nil {
		return fmt.Errorf("unable to create controller for ClusterObjectDeployment: %w", err)

	}

	// Package
	if err = (packages.NewPackageController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("Package"),
		mgr.GetScheme(), opts.namespace,
	).SetupWithManager(mgr)); err != nil {
		return fmt.Errorf("unable to create controller for Package: %w", err)
	}
	if err = (packages.NewClusterPackageController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("ClusterPackage"),
		mgr.GetScheme(), opts.namespace,
	).SetupWithManager(mgr)); err != nil {
		return fmt.Errorf("unable to create controller for ClusterPackage: %w", err)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("problem running manager: %w", err)
	}
	return nil
}
