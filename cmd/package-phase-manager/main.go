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
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	packageapis "github.com/thetechnick/package-operator/apis"
	"github.com/thetechnick/package-operator/internal/controllers/packages/objectsetphases"
	"github.com/thetechnick/package-operator/internal/dynamicwatcher"
	"github.com/thetechnick/package-operator/internal/ownerhandling"
)

var (
	scheme       = runtime.NewScheme()
	targetScheme = runtime.NewScheme()
	setupLog     = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(targetScheme)
	_ = packageapis.AddToScheme(scheme)
}

func main() {
	var (
		metricsAddr                 string
		pprofAddr                   string
		targetClusterKubeconfigFile string
		enableLeaderElection        bool
		namespace                   string
		class                       string
	)
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&pprofAddr, "pprof-addr", "", "The address the pprof web endpoint binds to.")
	flag.StringVar(&namespace, "namespace", os.Getenv("PKO_NAMESPACE"), "Namespace to limit the operator to.")
	flag.StringVar(&class, "class", "", "class of the ObjectSetPhase to work on.")
	flag.StringVar(&targetClusterKubeconfigFile, "target-cluster-kubeconfig-file", "", "Filepath for a kubeconfig connecting to the deployment target cluster.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	if err := run(
		metricsAddr, pprofAddr,
		targetClusterKubeconfigFile,
		enableLeaderElection,
		namespace, class,
	); err != nil {
		setupLog.Error(err, "run manager")
		os.Exit(1)
	}
}

func run(
	metricsAddr string,
	pprofAddr string,
	targetClusterKubeconfigFile string,
	enableLeaderElection bool,
	namespace, class string,
) error {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                     scheme,
		MetricsBindAddress:         metricsAddr,
		Port:                       9443,
		LeaderElectionResourceLock: "leases",
		LeaderElection:             enableLeaderElection,
		LeaderElectionID:           "2b1op84x6s.package-operator-lock",
		Namespace:                  namespace,
	})
	if err != nil {
		return fmt.Errorf("unable to start manager: %w", err)
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

	// TargetCluster Setup
	targetCfg, err := clientcmd.BuildConfigFromFlags("", targetClusterKubeconfigFile)
	if err != nil {
		return fmt.Errorf("reading target cluster kubeconfig: %w", err)
	}
	targetMapper, err := apiutil.NewDiscoveryRESTMapper(targetCfg)
	if err != nil {
		return fmt.Errorf("creating target cluster rest mapper: %w", err)
	}
	targetClient, err := client.New(targetCfg, client.Options{
		Scheme: targetScheme,
		Mapper: targetMapper,
	})
	if err != nil {
		return fmt.Errorf("creating target cluster client: %w", err)
	}
	targetDynamicClient, err := dynamic.NewForConfig(targetCfg)
	if err != nil {
		setupLog.Error(err, "unable to setup dynamic client")
		os.Exit(1)
	}

	// Dynamic Watcher
	dw := dynamicwatcher.New(
		ctrl.Log.WithName("DynamicWatcher"),
		scheme, targetMapper, targetDynamicClient)

	// ObjectSetPhase
	if err = (objectsetphases.NewObjectSetPhaseController(
		class, ownerhandling.Annotation,
		mgr.GetClient(), targetClient,
		ctrl.Log.WithName("controllers").WithName("ObjectSetPhase"),
		mgr.GetScheme(),
		&clusterLevelEnforcingDynamicWatcher{dw},
	).SetupWithManager(mgr)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ObjectSetPhase")
		os.Exit(1)
	}
	// if err = (objectsetphase.NewClusterObjectSetPhaseController(
	// 	class,
	// 	mgr.GetClient(), targetClient,
	// 	ctrl.Log.WithName("controllers").WithName("ClusterObjectSetPhase"),
	// 	mgr.GetScheme(), dw,
	// ).SetupWithManager(mgr)); err != nil {
	// 	setupLog.Error(err, "unable to create controller", "controller", "ClusterObjectSetPhase")
	// 	os.Exit(1)
	// }
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("problem running manager: %w", err)
	}
	return nil
}

// dynamicWatcher enforcing cluster-level watches
// requires operator itself to run confined to a single namespace on the management cluster or cache free overlaps will occur.
type clusterLevelEnforcingDynamicWatcher struct {
	*dynamicwatcher.DynamicWatcher
}

func (dw *clusterLevelEnforcingDynamicWatcher) Watch(owner client.Object, obj runtime.Object) error {
	cowner := owner.DeepCopyObject().(client.Object)
	cowner.SetNamespace("")
	return dw.DynamicWatcher.Watch(cowner, obj)
}
