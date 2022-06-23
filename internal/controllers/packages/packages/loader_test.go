package packages

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/require"
	packageapis "github.com/thetechnick/package-operator/apis"
	"github.com/thetechnick/package-operator/internal/testutil"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = packageapis.AddToScheme(scheme)
}

func TestLoader(t *testing.T) {
	l := newPackageLoaderBuilder(testutil.NewLogger(t), scheme)
	dep, err := l.Load("../../../../config/packages/nginx", map[string]interface{}{"metadata": map[string]string{"name": "test"}})
	require.NoError(t, err)

	y, _ := yaml.Marshal(dep)
	fmt.Println(string(y))
	// t.Fail()
}
