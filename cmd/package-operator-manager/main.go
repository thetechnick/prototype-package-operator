package main

import (
	"context"
	"flag"
	"net/http"
	"net/http/pprof"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	packageapis "github.com/thetechnick/package-operator/apis"
	"github.com/thetechnick/package-operator/internal/controllers/packages/objectdeployment"
	"github.com/thetechnick/package-operator/internal/controllers/packages/objectset"
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
	)
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&pprofAddr, "pprof-addr", "", "The address the pprof web endpoint binds to.")
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

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "unable to setup discovery client")
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

	// ObjectSet
	if err = (objectset.NewObjectSetController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("ObjectSet"),
		mgr.GetScheme(), dynamicClient, discoveryClient,
	).SetupWithManager(mgr)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ObjectSet")
		os.Exit(1)
	}
	if err = (objectset.NewClusterObjectSetController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("ClusterObjectSet"),
		mgr.GetScheme(), dynamicClient, discoveryClient,
	).SetupWithManager(mgr)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterObjectSet")
		os.Exit(1)
	}

	// ObjectDeployment
	if err = (objectdeployment.NewObjectDeploymentController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("ObjectDeployment"),
		mgr.GetScheme(),
	).SetupWithManager(mgr)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ObjectDeployment")
		os.Exit(1)
	}
	if err = (objectdeployment.NewClusterObjectDeploymentController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("ClusterObjectDeployment"),
		mgr.GetScheme(),
	).SetupWithManager(mgr)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterObjectDeployment")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
