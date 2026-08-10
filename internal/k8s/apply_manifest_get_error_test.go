package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/k8s/demo"
)

// TestApplyManifest_SurfacesNonNotFoundGetError guards against ApplyManifest
// treating a Forbidden (or any non-NotFound) Get error as "object absent,
// go ahead and Create" -- that would run a blocked read as a write instead
// of surfacing the real failure to the caller.
func TestApplyManifest_SurfacesNonNotFoundGetError(t *testing.T) {
	cs := demo.NewClientset()
	dyn := demo.NewDynamicClient()
	dyn.PrependReactor("get", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "web", nil)
	})
	c := newFakeClient(cs, dyn)

	manifest := "apiVersion: v1\n" +
		"kind: Pod\n" +
		"metadata:\n" +
		"  name: web\n" +
		"  namespace: demo\n" +
		"spec:\n" +
		"  containers:\n" +
		"  - name: app\n" +
		"    image: nginx\n"

	err := c.ApplyManifest(t.Context(), "test-ctx", "demo", manifest)
	require.Error(t, err, "expected ApplyManifest to surface a non-NotFound Get error instead of treating it as absence")
	assert.Contains(t, err.Error(), "forbidden")
}
