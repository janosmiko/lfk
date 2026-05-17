package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/janosmiko/lfk/internal/model"
)

// The fake dynamic client is context-agnostic: it returns the same objects
// for every contextName that GetResources is called with. That is exactly
// what we want here — the test verifies that GetResourcesUnion fans out to
// every context, stamps each returned item with its source ClusterName,
// leaves the Context column to the UI renderer, and merges/sorts results. Per-context
// divergence is covered by the existing GetResources tests.
func TestGetResourcesUnion_FanOutStampingAndSort(t *testing.T) {
	pod := func(name string) *unstructured.Unstructured {
		return &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      name,
					"namespace": "cloud-cd",
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "c", "image": "nginx"},
					},
				},
				"status": map[string]any{"phase": "Running"},
			},
		}
	}
	// Seed two distinct pods so the merged result has 2 × N rows.
	dyn := newFakeDynClient(pod("alpha"), pod("beta"))
	c := newFakeClient(nil, dyn)

	contexts := []string{"green", "blue"} // unsorted to prove secondary sort
	items, err := c.GetResourcesUnion(
		context.Background(),
		contexts,
		"cloud-cd",
		model.ResourceTypeEntry{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true},
	)
	require.NoError(t, err)
	require.Len(t, items, 4, "two pods × two contexts = four merged rows")

	// Sort order: primary by Name, secondary by ClusterName (lexicographic).
	// Expected sequence: alpha/blue, alpha/green, beta/blue, beta/green.
	expected := []struct {
		name    string
		cluster string
	}{
		{"alpha", "blue"},
		{"alpha", "green"},
		{"beta", "blue"},
		{"beta", "green"},
	}
	for i, want := range expected {
		assert.Equal(t, want.name, items[i].Name, "row %d name", i)
		assert.Equal(t, want.cluster, items[i].ClusterName, "row %d cluster", i)

		for _, kv := range items[i].Columns {
			assert.NotEqual(t, "Context", kv.Key,
				"row %d: Context is rendered from ClusterName, not synthetic Item.Columns metadata", i)
		}
	}
}

func TestGetResourcesUnion_SortsSameNameClusterByNamespace(t *testing.T) {
	pod := func(namespace string) *unstructured.Unstructured {
		return &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "shared",
					"namespace": namespace,
				},
				"spec":   map[string]any{"containers": []any{map[string]any{"name": "c", "image": "nginx"}}},
				"status": map[string]any{"phase": "Running"},
			},
		}
	}
	dyn := newFakeDynClient(pod("zeta"), pod("alpha"))
	c := newFakeClient(nil, dyn)

	items, err := c.GetResourcesUnion(
		context.Background(),
		[]string{"blue"},
		"",
		model.ResourceTypeEntry{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true},
	)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "alpha", items[0].Namespace)
	assert.Equal(t, "zeta", items[1].Namespace)
}

func TestGetResourcesUnion_EmptyContextsList(t *testing.T) {
	// Degenerate case: no contexts. Should not hang, not error, not panic.
	dyn := newFakeDynClient()
	c := newFakeClient(nil, dyn)

	items, err := c.GetResourcesUnion(
		context.Background(),
		nil,
		"cloud-cd",
		model.ResourceTypeEntry{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true},
	)
	require.NoError(t, err)
	assert.Empty(t, items)
}

// Partial-failure error propagation (first error is surfaced alongside the
// items that other contexts did succeed to return) is not covered here
// because the fake dynamic client is context-agnostic — every context sees
// the same backing store, so per-context divergence cannot be simulated
// without refactoring dynamicForContext to dispatch by context name. The
// logic is four lines of straightforward aggregation and is better verified
// by code review than by tests that pretend to diverge.

// Compile-time sanity: the constructor used by the test fake is still in
// sync with GetResources' expectations for Pod listing.
func TestGetResourcesUnion_UsesStandardGetResources(t *testing.T) {
	dyn := newFakeDynClient(&unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   map[string]any{"name": "solo", "namespace": "cloud-cd"},
			"spec":       map[string]any{"containers": []any{map[string]any{"name": "c", "image": "x"}}},
			"status":     map[string]any{"phase": "Running"},
		},
	})
	c := newFakeClient(nil, dyn)

	// Single-context union: one cluster, one pod → one row with ClusterName stamped.
	items, err := c.GetResourcesUnion(
		context.Background(),
		[]string{"solo-cluster"},
		"cloud-cd",
		model.ResourceTypeEntry{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true},
	)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "solo-cluster", items[0].ClusterName)

	// And the non-union GetResources must NOT stamp ClusterName — that field
	// is the union feature's contract. Verify the two code paths differ here.
	plain, err := c.GetResources(
		context.Background(),
		"solo-cluster",
		"cloud-cd",
		model.ResourceTypeEntry{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true},
	)
	require.NoError(t, err)
	require.Len(t, plain, 1)
	assert.Empty(t, plain[0].ClusterName, "GetResources must leave ClusterName empty; only GetResourcesUnion stamps it")
}
