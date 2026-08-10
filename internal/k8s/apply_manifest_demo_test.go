package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/janosmiko/lfk/internal/k8s/demo"
)

// TestApplyManifest_RejectsInvalidRFC1123Name guards TASK-865 finding 4:
// the demo apply path decodes arbitrary YAML with no name validation the
// way a real apiserver would perform. A crafted metadata.name carrying a
// bidi-override character (a raw ASCII escape byte like \x1b is already
// rejected by the YAML decoder itself — "control characters are not
// allowed" — so it can't reach the name-validation path this test targets)
// must never reach the fake dynamic client's tracker, since
// democli/logs.go later formats pod names into generated log output.
func TestApplyManifest_RejectsInvalidRFC1123Name(t *testing.T) {
	cs := demo.NewClientset()
	dyn := demo.NewDynamicClient()
	c := newFakeClient(cs, dyn)

	const evilChar = "\u202e" // RIGHT-TO-LEFT OVERRIDE
	const evilName = "web" + evilChar + "HACKED"
	manifest := "apiVersion: v1\n" +
		"kind: Pod\n" +
		"metadata:\n" +
		"  name: \"" + evilName + "\"\n" +
		"  namespace: demo\n" +
		"spec:\n" +
		"  containers:\n" +
		"  - name: app\n" +
		"    image: nginx\n"

	err := c.ApplyManifest(t.Context(), "test-ctx", demo.NamespaceDemo, manifest)
	require.Error(t, err, "expected ApplyManifest to reject an invalid RFC1123 name")

	list, listErr := dyn.Resource(schema.GroupVersionResource{Version: "v1", Resource: "pods"}).
		Namespace(demo.NamespaceDemo).List(t.Context(), metav1.ListOptions{})
	require.NoError(t, listErr)
	for _, item := range list.Items {
		assert.NotContains(t, item.GetName(), evilChar,
			"a manifest with an invalid name must never reach the fake tracker")
	}
}

// TestApplyManifest_AcceptsValidRFC1123Name is the control case: a
// well-formed name must still apply cleanly.
func TestApplyManifest_AcceptsValidRFC1123Name(t *testing.T) {
	cs := demo.NewClientset()
	dyn := demo.NewDynamicClient()
	c := newFakeClient(cs, dyn)

	manifest := "apiVersion: v1\n" +
		"kind: Pod\n" +
		"metadata:\n" +
		"  name: web-valid-1\n" +
		"  namespace: demo\n" +
		"spec:\n" +
		"  containers:\n" +
		"  - name: app\n" +
		"    image: nginx\n"

	err := c.ApplyManifest(t.Context(), "test-ctx", demo.NamespaceDemo, manifest)
	require.NoError(t, err)

	_, getErr := dyn.Resource(schema.GroupVersionResource{Version: "v1", Resource: "pods"}).
		Namespace(demo.NamespaceDemo).Get(t.Context(), "web-valid-1", metav1.GetOptions{})
	require.NoError(t, getErr, "expected the valid pod to have been created")
}
