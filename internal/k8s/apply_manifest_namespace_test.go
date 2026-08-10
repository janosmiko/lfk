package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/janosmiko/lfk/internal/k8s/demo"
)

// manifestWithNamespace builds a minimal Pod manifest for the given
// namespace, reused across the namespace-validation tests below.
func manifestWithNamespace(namespace string) string {
	return "apiVersion: v1\n" +
		"kind: Pod\n" +
		"metadata:\n" +
		"  name: web\n" +
		"  namespace: \"" + namespace + "\"\n" +
		"spec:\n" +
		"  containers:\n" +
		"  - name: app\n" +
		"    image: nginx\n"
}

// TestApplyManifest_RejectsBidiNamespace guards TASK-865 finding 1: the
// reviewer's exact case, a namespace carrying a RIGHT-TO-LEFT OVERRIDE
// character. Without namespace validation this reaches the fake tracker
// and later surfaces unescaped in render paths that format "namespace/name"
// (resourceTitleLabel in internal/app/view_status.go).
func TestApplyManifest_RejectsBidiNamespace(t *testing.T) {
	cs := demo.NewClientset()
	dyn := demo.NewDynamicClient()
	c := newFakeClient(cs, dyn)

	const evilChar = "\u202e" // RIGHT-TO-LEFT OVERRIDE
	const evilNamespace = "demo" + evilChar + "HACKED"

	err := c.ApplyManifest(t.Context(), "test-ctx", evilNamespace, manifestWithNamespace(evilNamespace))
	require.Error(t, err, "expected ApplyManifest to reject a namespace carrying a bidi override character")

	list, listErr := dyn.Resource(schema.GroupVersionResource{Version: "v1", Resource: "pods"}).
		Namespace(evilNamespace).List(t.Context(), metav1.ListOptions{})
	require.NoError(t, listErr)
	for _, item := range list.Items {
		assert.NotContains(t, item.GetNamespace(), evilChar,
			"a manifest with an invalid namespace must never reach the fake tracker")
	}
}

// TestApplyManifest_RejectsNamespaceBreakingRFC1123Label guards the "not
// the same rule as the name" half of finding 1: a namespace is an RFC1123
// label, not a subdomain, so a dotted value that would pass
// IsDNS1123Subdomain (like a valid DNS name) must still be rejected.
func TestApplyManifest_RejectsNamespaceBreakingRFC1123Label(t *testing.T) {
	cs := demo.NewClientset()
	dyn := demo.NewDynamicClient()
	c := newFakeClient(cs, dyn)

	const dottedNamespace = "demo.example" // valid subdomain, invalid label

	err := c.ApplyManifest(t.Context(), "test-ctx", dottedNamespace, manifestWithNamespace(dottedNamespace))
	require.Error(t, err, "expected ApplyManifest to reject a namespace that is a valid subdomain but not a valid RFC1123 label")
}

// TestApplyManifest_AcceptsValidNamespace is the control case: a
// well-formed RFC1123 label namespace must still apply cleanly.
func TestApplyManifest_AcceptsValidNamespace(t *testing.T) {
	cs := demo.NewClientset()
	dyn := demo.NewDynamicClient()
	c := newFakeClient(cs, dyn)

	const validNamespace = "demo-valid-1"

	err := c.ApplyManifest(t.Context(), "test-ctx", validNamespace, manifestWithNamespace(validNamespace))
	require.NoError(t, err)

	_, getErr := dyn.Resource(schema.GroupVersionResource{Version: "v1", Resource: "pods"}).
		Namespace(validNamespace).Get(t.Context(), "web", metav1.GetOptions{})
	require.NoError(t, getErr, "expected the valid pod to have been created")
}
