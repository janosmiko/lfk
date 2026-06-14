package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// TestGetNodeClaimNodeName covers the happy path: a NodeClaim with
// status.nodeName populated. The TUI relies on this value to forward
// Cordon / Uncordon / Drain Node from the NodeClaim row to the
// kubectl helpers, so the helper must return the literal string from
// the API, not a derived form.
func TestGetNodeClaimNodeName(t *testing.T) {
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		karpenterNodeClaimGVR: "NodeClaimList",
	}
	claim := &unstructured.Unstructured{}
	claim.SetUnstructuredContent(map[string]any{
		"apiVersion": "karpenter.sh/v1",
		"kind":       "NodeClaim",
		"metadata":   map[string]any{"name": "claim-1"},
		"status":     map[string]any{"nodeName": "ip-10-0-0-1.ec2.internal"},
	})
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs, claim)
	c := newFakeClient(nil, dyn)

	got, err := c.GetNodeClaimNodeName("test-ctx", "claim-1")
	require.NoError(t, err)
	assert.Equal(t, "ip-10-0-0-1.ec2.internal", got)
}

// TestGetNodeClaimNodeName_NotYetBound covers the steady-state of a
// freshly-created NodeClaim before Karpenter has bound it to a node:
// status.nodeName is unset, but the resource exists. The TUI should
// see ("", nil) so it can render "node not yet bound" cleanly without
// surfacing a "field missing" error.
func TestGetNodeClaimNodeName_NotYetBound(t *testing.T) {
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		karpenterNodeClaimGVR: "NodeClaimList",
	}
	claim := &unstructured.Unstructured{}
	claim.SetUnstructuredContent(map[string]any{
		"apiVersion": "karpenter.sh/v1",
		"kind":       "NodeClaim",
		"metadata":   map[string]any{"name": "claim-pending"},
		// no status.nodeName yet
	})
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs, claim)
	c := newFakeClient(nil, dyn)

	got, err := c.GetNodeClaimNodeName("test-ctx", "claim-pending")
	require.NoError(t, err)
	assert.Empty(t, got, "unbound NodeClaim returns empty nodeName, not an error")
}

// TestGetNodeClaimNodeName_NotFound exercises the lookup-failure path:
// the NodeClaim doesn't exist (deleted, never created, or RBAC denies
// the Get). The helper must surface a wrapped error so callers can
// distinguish "missing" from the legitimate ("", nil) "not bound yet"
// state covered by TestGetNodeClaimNodeName_NotYetBound.
func TestGetNodeClaimNodeName_NotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		karpenterNodeClaimGVR: "NodeClaimList",
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs)
	c := newFakeClient(nil, dyn)

	_, err := c.GetNodeClaimNodeName("test-ctx", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting NodeClaim")
}

// TestDisruptNodeClaim asserts the happy path of the Karpenter
// disrupt action: the NodeClaim is deleted from the API. Karpenter's
// real-world cascade (terminating the underlying cloud instance and
// the matching core Node) is not exercised here — the dynamic fake
// only models the NodeClaim object — but the delete is the trigger
// Karpenter watches for, so verifying the delete is sufficient.
func TestDisruptNodeClaim(t *testing.T) {
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		karpenterNodeClaimGVR: "NodeClaimList",
	}
	claim := &unstructured.Unstructured{}
	claim.SetUnstructuredContent(map[string]any{
		"apiVersion": "karpenter.sh/v1",
		"kind":       "NodeClaim",
		"metadata":   map[string]any{"name": "doomed"},
	})
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs, claim)
	c := newFakeClient(nil, dyn)

	require.NoError(t, c.DisruptNodeClaim("test-ctx", "doomed"))

	// Verify the NodeClaim is gone from the fake API.
	_, err := dyn.Resource(karpenterNodeClaimGVR).Get(t.Context(), "doomed", metav1.GetOptions{})
	require.Error(t, err, "DisruptNodeClaim must delete the NodeClaim from the API")
}

// TestDisruptNodeClaim_NotFound asserts that disrupting a NodeClaim
// that no longer exists returns a wrapped error rather than silently
// succeeding. The TUI relies on this so the type-to-confirm overlay's
// status message accurately reports the failure instead of claiming
// success on a no-op.
func TestDisruptNodeClaim_NotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		karpenterNodeClaimGVR: "NodeClaimList",
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs)
	c := newFakeClient(nil, dyn)

	err := c.DisruptNodeClaim("test-ctx", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disrupting NodeClaim")
}
