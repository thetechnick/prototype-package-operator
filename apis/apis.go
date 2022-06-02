package apis

import (
	"github.com/thetechnick/package-operator/apis/coordination"
	"github.com/thetechnick/package-operator/apis/packages"
	"k8s.io/apimachinery/pkg/runtime"
)

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder = runtime.SchemeBuilder{
	coordination.AddToScheme,
	packages.AddToScheme,
}

// AddToScheme adds all addon Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
