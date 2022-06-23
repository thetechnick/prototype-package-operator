package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	packageapis "github.com/thetechnick/package-operator/apis"
	"github.com/thetechnick/package-operator/internal/controllers/packages/packages"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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
		packagePath      string
		packageName      string
		packageNamespace string
	)
	flag.StringVar(&packagePath, "package-path", "", "The directory to search for package files in.")
	flag.StringVar(&packageName, "package-name", "", "Name of the (Cluster)Package object")
	flag.StringVar(&packageNamespace, "package-namespace", "", "Namespace of the Package object")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	if err := run(
		packagePath,
		packageName,
		packageNamespace,
	); err != nil {
		setupLog.Error(err, "run manager")
		os.Exit(1)
	}
}

func run(
	packagePath string,
	packageName string,
	packageNamespace string,
) error {
	c, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("creating ctrl client: %w", err)
	}

	var unpackController *packages.UnpackController
	if len(packageNamespace) == 0 {
		// cluster scoped
		unpackController = packages.NewClusterUnpackController(
			ctrl.Log.WithName("controllers").WithName("Unpack"),
			scheme, c, packagePath,
		)
	} else {
		// namespace scoped
		unpackController = packages.NewUnpackController(
			ctrl.Log.WithName("controllers").WithName("Unpack"),
			scheme, c, packagePath,
		)
	}

	ctx := context.Background()
	_, err = unpackController.Reconcile(ctx, ctrl.Request{
		NamespacedName: client.ObjectKey{Name: packageName, Namespace: packageNamespace},
	})
	if err != nil {
		return fmt.Errorf("reconciling : %w", err)
	}

	return nil
}
