package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

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
	_, err := dyn.Resource(karpenterNodeClaimGVR).Get(context.Background(), "doomed", metav1.GetOptions{})
	require.Error(t, err, "DisruptNodeClaim must delete the NodeClaim from the API")
}

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
