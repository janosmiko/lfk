package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// --- filteredBookmarks ---

func TestFilteredBookmarks(t *testing.T) {
	bookmarks := []model.Bookmark{
		{Name: "prod > Deployments", Slot: "a"},
		{Name: "staging > Pods", Slot: "b"},
		{Name: "prod > Services", Slot: "c"},
	}

	t.Run("empty filter returns all", func(t *testing.T) {
		m := Model{
			bookmarks:      bookmarks,
			bookmarkFilter: TextInput{},
		}
		result := m.filteredBookmarks()
		assert.Len(t, result, 3)
	})

	t.Run("filter by context", func(t *testing.T) {
		m := Model{
			bookmarks:      bookmarks,
			bookmarkFilter: TextInput{Value: "prod"},
		}
		result := m.filteredBookmarks()
		assert.Len(t, result, 2)
		assert.Equal(t, "a", result[0].Slot)
		assert.Equal(t, "c", result[1].Slot)
	})

	t.Run("filter by resource type", func(t *testing.T) {
		m := Model{
			bookmarks:      bookmarks,
			bookmarkFilter: TextInput{Value: "pods"},
		}
		result := m.filteredBookmarks()
		assert.Len(t, result, 1)
		assert.Equal(t, "b", result[0].Slot)
	})

	t.Run("case insensitive filter", func(t *testing.T) {
		m := Model{
			bookmarks:      bookmarks,
			bookmarkFilter: TextInput{Value: "DEPLOYMENTS"},
		}
		result := m.filteredBookmarks()
		assert.Len(t, result, 1)
		assert.Equal(t, "a", result[0].Slot)
	})

	t.Run("no match returns empty", func(t *testing.T) {
		m := Model{
			bookmarks:      bookmarks,
			bookmarkFilter: TextInput{Value: "nonexistent"},
		}
		result := m.filteredBookmarks()
		assert.Empty(t, result)
	})

	t.Run("nil bookmarks returns nil", func(t *testing.T) {
		m := Model{
			bookmarkFilter: TextInput{Value: "prod"},
		}
		result := m.filteredBookmarks()
		assert.Empty(t, result)
	})
}

// --- contextInList ---

func TestContextInList(t *testing.T) {
	items := []model.Item{
		{Name: "cluster-a"},
		{Name: "cluster-b"},
		{Name: "cluster-c"},
	}

	assert.True(t, contextInList("cluster-b", items))
	assert.False(t, contextInList("nonexistent", items))
	assert.False(t, contextInList("cluster-a", nil))
	assert.False(t, contextInList("", items))
}

// --- applySessionNamespaces ---

func TestApplySessionNamespaces(t *testing.T) {
	t.Run("all namespaces mode", func(t *testing.T) {
		m := Model{namespace: "old"}
		applySessionNamespaces(&m, true, "", nil)
		assert.True(t, m.allNamespaces)
		assert.Nil(t, m.selectedNamespaces)
	})

	t.Run("single namespace", func(t *testing.T) {
		m := Model{}
		applySessionNamespaces(&m, false, "production", nil)
		assert.Equal(t, "production", m.namespace)
		assert.False(t, m.allNamespaces)
	})

	t.Run("multiple namespaces", func(t *testing.T) {
		m := Model{}
		applySessionNamespaces(&m, false, "ns-1", []string{"ns-1", "ns-2", "ns-3"})
		assert.Equal(t, "ns-1", m.namespace)
		assert.Len(t, m.selectedNamespaces, 3)
		assert.True(t, m.selectedNamespaces["ns-1"])
		assert.True(t, m.selectedNamespaces["ns-2"])
		assert.True(t, m.selectedNamespaces["ns-3"])
	})
}

// --- bookmarkToSlot Global flag ---

// podResourceType returns a Pod ResourceTypeEntry for test fixtures.
func podResourceType() model.ResourceTypeEntry {
	return model.ResourceTypeEntry{
		Kind:        "Pod",
		DisplayName: "Pods",
		APIGroup:    "",
		APIVersion:  "v1",
		Resource:    "pods",
		Namespaced:  true,
	}
}

func TestBookmarkToSlot_GlobalFlag(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	rt := podResourceType()

	tests := []struct {
		slot        string
		wantGlobal  bool
		wantContext string // global saves context, local does not
		wantName    string // global includes context in name, local does not
	}{
		{slot: "a", wantGlobal: false, wantContext: "", wantName: "Pods"},
		{slot: "z", wantGlobal: false, wantContext: "", wantName: "Pods"},
		{slot: "0", wantGlobal: false, wantContext: "", wantName: "Pods"},
		{slot: "9", wantGlobal: false, wantContext: "", wantName: "Pods"},
		{slot: "A", wantGlobal: true, wantContext: "test", wantName: "test > Pods"},
		{slot: "Z", wantGlobal: true, wantContext: "test", wantName: "test > Pods"},
		{slot: "M", wantGlobal: true, wantContext: "test", wantName: "test > Pods"},
	}

	for _, tt := range tests {
		t.Run("slot_"+tt.slot, func(t *testing.T) {
			m := Model{
				nav: model.NavigationState{
					Level:        model.LevelResources,
					Context:      "test",
					ResourceType: rt,
				},
				namespace: "default",
				tabs:      []TabState{{}},
			}

			result, _ := m.bookmarkToSlot(tt.slot)
			resultModel := result.(Model)

			// The bookmark should be the last entry in the list.
			require.NotEmpty(t, resultModel.bookmarks, "bookmarks should not be empty for slot %q", tt.slot)
			bm := resultModel.bookmarks[len(resultModel.bookmarks)-1]
			assert.Equal(t, tt.slot, bm.Slot)
			assert.Equal(t, tt.wantGlobal, bm.Global, "slot %q: Global should be %v", tt.slot, tt.wantGlobal)
			assert.Equal(t, tt.wantContext, bm.Context,
				"slot %q: local bookmarks should not save context", tt.slot)
			assert.Equal(t, tt.wantName, bm.Name,
				"slot %q: local bookmarks should not include context in name", tt.slot)
			assert.Equal(t, rt.ResourceRef(), bm.ResourceType)
		})
	}
}

// --- navigateToBookmark context switching ---

// customCRDResourceType returns a CRD-based ResourceTypeEntry that won't match
// any built-in resource types. This ensures FindResourceTypeIn only succeeds
// when the correct cluster's discoveredCRDs contains it.
func customCRDResourceType() model.ResourceTypeEntry {
	return model.ResourceTypeEntry{
		Kind:        "Widget",
		DisplayName: "Widgets",
		APIGroup:    "test.example.com",
		APIVersion:  "v1alpha1",
		Resource:    "widgets",
		Namespaced:  true,
	}
}

func TestNavigateToBookmark_LocalKeepsContext(t *testing.T) {
	// The custom CRD exists only in cluster-A (the current context).
	// A local bookmark (Global=false) should look up resources in the current
	// context (cluster-A), not the bookmark's saved context (cluster-B).
	// Since the CRD is in cluster-A, the lookup succeeds, which proves the
	// function used the current context for lookup.
	crd := customCRDResourceType()

	m := Model{
		nav: model.NavigationState{
			Context: "cluster-A",
		},
		discoveredCRDs: map[string][]model.ResourceTypeEntry{
			"cluster-A": {crd},
			"cluster-B": {}, // cluster-B does NOT have the CRD
		},
	}

	bm := model.Bookmark{
		Context:      "cluster-B",
		Global:       false,
		ResourceType: crd.ResourceRef(),
		Namespace:    "default",
	}

	// If navigateToBookmark correctly uses the current context (cluster-A)
	// for a local bookmark, the CRD lookup will succeed and the function
	// will proceed past the "not found" check. It will then panic at
	// m.client.GetContexts() because client is nil, but the lookup
	// succeeding proves the correct context was used.
	assert.Panics(t, func() {
		m.navigateToBookmark(bm)
	}, "local bookmark should find CRD in current context and proceed to client call")
}

func TestNavigateToBookmark_LocalKeepsContext_FailsInWrongCluster(t *testing.T) {
	// Complementary test: same bookmark but the CRD only exists in cluster-B.
	// A local bookmark should NOT look in cluster-B, so the lookup fails
	// and the function returns early with "not found" instead of panicking.
	crd := customCRDResourceType()

	m := Model{
		nav: model.NavigationState{
			Context: "cluster-A",
		},
		discoveredCRDs: map[string][]model.ResourceTypeEntry{
			"cluster-A": {}, // cluster-A does NOT have the CRD
			"cluster-B": {crd},
		},
	}

	bm := model.Bookmark{
		Context:      "cluster-B",
		Global:       false,
		ResourceType: crd.ResourceRef(),
		Namespace:    "default",
	}

	// Should return cleanly with an error (no panic), proving the function
	// looked in cluster-A (current), not cluster-B (bookmark).
	result, _ := m.navigateToBookmark(bm)
	resultModel := result.(Model)

	assert.Contains(t, resultModel.statusMessage, "Resource type not found in current cluster")
	assert.True(t, resultModel.statusMessageErr)
	assert.Equal(t, "cluster-A", resultModel.nav.Context,
		"local bookmark should not change context when resource is not found")
}

func TestNavigateToBookmark_GlobalSwitchesContext(t *testing.T) {
	// The custom CRD exists only in cluster-B (the bookmark's context).
	// A global bookmark (Global=true) should look up resources in the
	// bookmark's saved context (cluster-B), not the current context (cluster-A).
	// Since the CRD is in cluster-B, the lookup succeeds, proving the
	// function used the bookmark's context.
	crd := customCRDResourceType()

	m := Model{
		nav: model.NavigationState{
			Context: "cluster-A",
		},
		discoveredCRDs: map[string][]model.ResourceTypeEntry{
			"cluster-A": {}, // cluster-A does NOT have the CRD
			"cluster-B": {crd},
		},
	}

	bm := model.Bookmark{
		Context:      "cluster-B",
		Global:       true,
		ResourceType: crd.ResourceRef(),
		Namespace:    "default",
	}

	// If navigateToBookmark correctly uses the bookmark's context (cluster-B)
	// for a global bookmark, the CRD lookup will succeed. The function then
	// panics at m.client.GetContexts(), proving the correct context was used.
	assert.Panics(t, func() {
		m.navigateToBookmark(bm)
	}, "global bookmark should find CRD in bookmark context and proceed to client call")
}

func TestNavigateToBookmark_GlobalFailsInWrongCluster(t *testing.T) {
	// Complementary test: the CRD only exists in cluster-A.
	// A global bookmark should look in cluster-B (bookmark context), so the
	// lookup fails and the function returns with "not found".
	crd := customCRDResourceType()

	m := Model{
		nav: model.NavigationState{
			Context: "cluster-A",
		},
		discoveredCRDs: map[string][]model.ResourceTypeEntry{
			"cluster-A": {crd},
			"cluster-B": {}, // cluster-B does NOT have the CRD
		},
	}

	bm := model.Bookmark{
		Context:      "cluster-B",
		Global:       true,
		ResourceType: crd.ResourceRef(),
		Namespace:    "default",
	}

	// Should return cleanly with an error (no panic), proving the function
	// looked in cluster-B (bookmark), not cluster-A (current).
	result, _ := m.navigateToBookmark(bm)
	resultModel := result.(Model)

	assert.Contains(t, resultModel.statusMessage, "Resource type not found in current cluster")
	assert.True(t, resultModel.statusMessageErr)
}

func TestNavigateToBookmark_LocalResourceNotFound(t *testing.T) {
	// Use a custom CRD ref that doesn't exist anywhere.
	// With an empty discoveredCRDs for the current cluster, the function
	// should return the "not found" error.
	m := Model{
		nav: model.NavigationState{
			Context: "cluster-A",
		},
		discoveredCRDs: map[string][]model.ResourceTypeEntry{
			"cluster-A": {},
		},
	}

	bm := model.Bookmark{
		Context:      "cluster-B",
		Global:       false,
		ResourceType: "nonexistent.example.com/v1/fakes",
		Namespace:    "default",
	}

	// This should return early with an error message (no panic, no client call).
	result, _ := m.navigateToBookmark(bm)
	resultModel := result.(Model)

	assert.Contains(t, resultModel.statusMessage, "Resource type not found in current cluster")
	assert.True(t, resultModel.statusMessageErr)
	// Context should remain unchanged since navigation was aborted.
	assert.Equal(t, "cluster-A", resultModel.nav.Context)
}
