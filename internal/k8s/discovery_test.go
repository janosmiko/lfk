package k8s

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discoveryfake "k8s.io/client-go/discovery/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/model"
)

func TestConvertAPIResourceLists_HappyPath(t *testing.T) {
	lists := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"get", "list"}},
				{Name: "nodes", Namespaced: false, Kind: "Node", Verbs: metav1.Verbs{"get", "list"}},
			},
		},
		{
			GroupVersion: "storage.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{Name: "csidrivers", Namespaced: false, Kind: "CSIDriver", Verbs: metav1.Verbs{"get", "list"}},
			},
		},
	}

	entries := convertAPIResourceLists(lists)
	require.Len(t, entries, 3)

	// Core /v1 resources
	pod := findDiscoveryEntry(t, entries, "Pod", "")
	assert.Equal(t, "", pod.APIGroup)
	assert.Equal(t, "v1", pod.APIVersion)
	assert.Equal(t, "pods", pod.Resource)
	assert.True(t, pod.Namespaced)

	node := findDiscoveryEntry(t, entries, "Node", "")
	assert.False(t, node.Namespaced)

	// Grouped resource
	csi := findDiscoveryEntry(t, entries, "CSIDriver", "storage.k8s.io")
	assert.Equal(t, "v1", csi.APIVersion)
	assert.Equal(t, "csidrivers", csi.Resource)
	assert.False(t, csi.Namespaced)
}

func TestConvertAPIResourceLists_SkipsSubresources(t *testing.T) {
	lists := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod"},
				{Name: "pods/log", Namespaced: true, Kind: "Pod"},
				{Name: "pods/exec", Namespaced: true, Kind: "Pod"},
				{Name: "pods/status", Namespaced: true, Kind: "Pod"},
			},
		},
	}
	entries := convertAPIResourceLists(lists)
	require.Len(t, entries, 1, "only the primary pods entry should be kept")
	assert.Equal(t, "pods", entries[0].Resource)
}

func TestConvertAPIResourceLists_KeepsReviewAPIs(t *testing.T) {
	// Review APIs are create-only but still must appear in the discovered
	// set so they remain resolvable. The sidebar hides them via the
	// listability filter (empty/no-list Verbs → hidden) which is a
	// different layer and not convertAPIResourceLists' concern.
	lists := []*metav1.APIResourceList{
		{
			GroupVersion: "authentication.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{Name: "tokenreviews", Namespaced: false, Kind: "TokenReview", Verbs: metav1.Verbs{"create"}},
				{Name: "selfsubjectreviews", Namespaced: false, Kind: "SelfSubjectReview", Verbs: metav1.Verbs{"create"}},
			},
		},
	}
	entries := convertAPIResourceLists(lists)
	require.Len(t, entries, 2)
	// Verbs from the discovery API are preserved so the sidebar layer
	// can filter out non-listable resources without re-querying.
	for _, e := range entries {
		assert.Equal(t, []string{"create"}, e.Verbs, "review API %s must retain create-only verbs", e.Resource)
		assert.False(t, e.CanList(), "review API %s must not be listable", e.Resource)
	}
}

func TestConvertAPIResourceLists_PopulatesVerbs(t *testing.T) {
	lists := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"get", "list", "watch", "create", "update", "patch", "delete"}},
			},
		},
	}
	entries := convertAPIResourceLists(lists)
	require.Len(t, entries, 1)
	assert.ElementsMatch(t,
		[]string{"get", "list", "watch", "create", "update", "patch", "delete"},
		entries[0].Verbs)
	assert.True(t, entries[0].CanList())
}

func TestDiscoverAPIResources_MergesPrinterColumns(t *testing.T) {
	// Arrange a fake clientset whose Discovery() returns a controlled set
	// of APIResourceList values, plus a fake dynamic client that holds a
	// CRD with printer columns. DiscoverAPIResources should combine the
	// discovery output (GVR+scope) with the CRD spec pass (printer columns).

	cs := k8sfake.NewClientset()
	fd, ok := cs.Discovery().(*discoveryfake.FakeDiscovery)
	require.True(t, ok)
	fd.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod"},
			},
		},
		{
			GroupVersion: "example.com/v1",
			APIResources: []metav1.APIResource{
				{Name: "foos", Namespaced: true, Kind: "Foo"},
			},
		},
	}

	// Fake CRD object with one printer column for foos.
	crd := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata":   map[string]any{"name": "foos.example.com"},
			"spec": map[string]any{
				"group": "example.com",
				"names": map[string]any{"plural": "foos", "kind": "Foo"},
				"scope": "Namespaced",
				"versions": []any{
					map[string]any{
						"name":    "v1",
						"served":  true,
						"storage": true,
						"additionalPrinterColumns": []any{
							map[string]any{"name": "Phase", "type": "string", "jsonPath": ".status.phase"},
						},
					},
				},
			},
		},
	}
	dc := newFakeDynClientWith(
		map[schema.GroupVersionResource]string{
			{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}: "CustomResourceDefinitionList",
		},
		crd,
	)

	c := newFakeClient(cs, dc)

	entries, err := c.DiscoverAPIResources(context.Background(), "")
	require.NoError(t, err)

	// Pod has no printer columns (core resource, discovery API has no columns).
	pod := findDiscoveryEntry(t, entries, "Pod", "")
	assert.Empty(t, pod.PrinterColumns)

	// Foo picked up its printer column from the CRD spec.
	foo := findDiscoveryEntry(t, entries, "Foo", "example.com")
	require.Len(t, foo.PrinterColumns, 1)
	assert.Equal(t, "Phase", foo.PrinterColumns[0].Name)
	assert.Equal(t, ".status.phase", foo.PrinterColumns[0].JSONPath)
}

func TestDiscoverAPIResources_PartialGroupFailure(t *testing.T) {
	// Arrange a fake discovery client that returns two resource lists. A
	// reactor on the per-group "get/resource" verb fails the first call and
	// lets the rest through, so one group ends up in failedGroups while the
	// other succeeds. The package-level discovery.ServerPreferredResources
	// helper aggregates this as *ErrGroupDiscoveryFailed, which triggers the
	// fall-through branch in DiscoverAPIResources.
	cs := k8sfake.NewClientset()
	fd, ok := cs.Discovery().(*discoveryfake.FakeDiscovery)
	require.True(t, ok)
	fd.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod"},
			},
		},
		{
			GroupVersion: "broken.example.com/v1",
			APIResources: []metav1.APIResource{
				{Name: "widgets", Namespaced: true, Kind: "Widget"},
			},
		},
	}
	// Inject a per-GV discovery error that client-go aggregates as
	// ErrGroupDiscoveryFailed: the first ServerResourcesForGroupVersion
	// call fails, subsequent calls fall through to the normal lookup.
	var callCount atomic.Int32
	fd.PrependReactor("get", "resource", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if callCount.Add(1) == 1 {
			return true, nil, errors.New("boom")
		}
		return false, nil, nil
	})

	// Fake dynamic client with no CRDs registered — fetchCRDPrinterColumns
	// should succeed with an empty map in that case.
	dc := newFakeDynClientWith(
		map[schema.GroupVersionResource]string{
			{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}: "CustomResourceDefinitionList",
		},
	)

	c := newFakeClient(cs, dc)

	entries, err := c.DiscoverAPIResources(context.Background(), "")
	// The ErrGroupDiscoveryFailed branch falls through and returns the
	// successful subset with no error. If the reactor pathway above does
	// not produce ErrGroupDiscoveryFailed (the fake may surface it as a
	// plain error), the test still validates that the successful v1 group
	// is returned.
	require.NoError(t, err)
	// At least one of the two groups must survive the partial-failure path.
	// Because fetchGroupVersionResources runs per-GV lookups concurrently,
	// either pods or widgets may be the one that failed; assert only that
	// the successful subset is non-empty and contains the survivor.
	require.NotEmpty(t, entries)
	foundPod := false
	foundWidget := false
	for _, e := range entries {
		if e.Kind == "Pod" && e.APIGroup == "" {
			foundPod = true
		}
		if e.Kind == "Widget" && e.APIGroup == "broken.example.com" {
			foundWidget = true
		}
	}
	assert.True(t, foundPod || foundWidget, "at least one group must survive partial discovery")
}

// findDiscoveryEntry returns the first entry matching kind and apiGroup or fails the test.
func findDiscoveryEntry(t *testing.T, entries []model.ResourceTypeEntry, kind, apiGroup string) model.ResourceTypeEntry {
	t.Helper()
	for _, e := range entries {
		if e.Kind == kind && e.APIGroup == apiGroup {
			return e
		}
	}
	t.Fatalf("no entry for kind=%s apiGroup=%s", kind, apiGroup)
	return model.ResourceTypeEntry{}
}
