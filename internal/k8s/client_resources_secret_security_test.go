package k8s

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
)

// secretObj builds a minimal *unstructured.Unstructured Secret with name+namespace.
func secretObj(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": "2026-04-01T00:00:00Z",
			},
		},
	}
}

// listSecretActionCount counts "list secrets" actions the fake dynamic
// client has observed, mirroring listActionCount's pod-counting sibling.
func listSecretActionCount(actions []clienttesting.Action) int {
	n := 0
	for _, a := range actions {
		if a.GetVerb() == "list" && a.GetResource().Resource == "secrets" {
			n++
		}
	}
	return n
}

// TestGetResources_SecretsNeverInformerCached_AlwaysMode is the red-first
// regression guard: InformerCacheAlways must never route Secrets through
// the informer cache, even under PreferCache (every watch-tick refresh
// passes it) -- a persistent watch would hold every Secret's data in
// memory and stream cluster-wide Secret changes indefinitely.
func TestGetResources_SecretsNeverInformerCached_AlwaysMode(t *testing.T) {
	dc := newFakeDynClient(secretObj("s1", "default"))
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAlways)
	t.Cleanup(c.Shutdown)

	for range 3 {
		items, err := c.GetResources(t.Context(), "", "default", secretRT, PreferCache())
		require.NoError(t, err)
		require.Len(t, items, 1)
	}

	assert.Equal(t, 3, listSecretActionCount(dc.Actions()),
		"every call must hit a direct list, never the cache")
	_, ok := c.informers.entries[""][secretGVR]
	assert.False(t, ok, "an informer must never be started for secrets")
}

// TestGetResources_SecretsNeverAutoPromoted is the red-first regression
// guard: a Secret list larger than the auto-promote threshold must not
// flip auto-mode's per-GVR state to cache-backed.
func TestGetResources_SecretsNeverAutoPromoted(t *testing.T) {
	objs := make([]apiruntime.Object, 0, autoPromoteAt+200)
	for i := range autoPromoteAt + 200 {
		objs = append(objs, secretObj(fmt.Sprintf("s-%d", i), "default"))
	}
	dc := newFakeDynClient(objs...)
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAuto)
	t.Cleanup(c.Shutdown)

	items, err := c.GetResources(t.Context(), "", "default", secretRT)
	require.NoError(t, err)
	require.Len(t, items, autoPromoteAt+200)
	assert.False(t, c.informers.isPromoted("", secretGVR), "a large secret list must never auto-promote")

	items2, err := c.GetResources(t.Context(), "", "default", secretRT)
	require.NoError(t, err)
	require.Len(t, items2, autoPromoteAt+200)
	assert.Equal(t, 2, listSecretActionCount(dc.Actions()), "second call must still hit a direct list")
	_, ok := c.informers.entries[""][secretGVR]
	assert.False(t, ok, "an informer must never be started for auto-promoted secrets")
}
