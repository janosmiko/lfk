package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
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

// --- bookmarkToSlot context-aware flag ---

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

func TestBookmarkToSlot_ContextAwareFlag(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	rt := podResourceType()

	tests := []struct {
		slot             string
		wantContextAware bool
		wantContext      string // context-aware saves context, context-free does not
		wantName         string // context-aware includes context in name, context-free does not
	}{
		{slot: "a", wantContextAware: true, wantContext: "test", wantName: "test > Pods"},
		{slot: "z", wantContextAware: true, wantContext: "test", wantName: "test > Pods"},
		{slot: "0", wantContextAware: true, wantContext: "test", wantName: "test > Pods"},
		{slot: "9", wantContextAware: true, wantContext: "test", wantName: "test > Pods"},
		{slot: "A", wantContextAware: false, wantContext: "", wantName: "Pods"},
		{slot: "Z", wantContextAware: false, wantContext: "", wantName: "Pods"},
		{slot: "M", wantContextAware: false, wantContext: "", wantName: "Pods"},
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

			require.NotEmpty(t, resultModel.bookmarks,
				"bookmarks should not be empty for slot %q", tt.slot)
			bm := resultModel.bookmarks[len(resultModel.bookmarks)-1]
			assert.Equal(t, tt.slot, bm.Slot)
			assert.Equal(t, tt.wantContextAware, bm.IsContextAware(),
				"slot %q: IsContextAware should be %v", tt.slot, tt.wantContextAware)
			assert.Equal(t, tt.wantContext, bm.Context,
				"slot %q: Context should be %q", tt.slot, tt.wantContext)
			assert.Equal(t, tt.wantName, bm.Name,
				"slot %q: Name should be %q", tt.slot, tt.wantName)
			assert.Equal(t, rt.ResourceRef(), bm.ResourceType,
				"slot %q: ResourceType should match the current nav resource type", tt.slot)
		})
	}
}

// --- bookmarkToSlot display name resolution ---

// TestBookmarkToSlot_CRDNameIncludesResourceType covers the user-reported
// regression where bookmarking on a Custom Resource (like External Secrets)
// produced a bookmark with an empty name. The root cause was that
// DiscoverAPIResources never populates ResourceTypeEntry.DisplayName, so the
// bookmark label resolver always fell through to just the context.
func TestBookmarkToSlot_CRDNameIncludesResourceType(t *testing.T) {
	tests := []struct {
		name     string
		rt       model.ResourceTypeEntry
		wantName string
	}{
		{
			// External Secrets: the exact CRD from the bug report. The
			// group/resource key lives in BuiltInMetadata with a curated
			// plural DisplayName, so that wins over the raw Kind.
			name: "CRD with BuiltInMetadata entry (External Secrets)",
			rt: model.ResourceTypeEntry{
				Kind:       "ExternalSecret",
				APIGroup:   "external-secrets.io",
				APIVersion: "v1beta1",
				Resource:   "externalsecrets",
				Namespaced: true,
			},
			wantName: "prod > ExternalSecrets",
		},
		{
			// CRD unknown to BuiltInMetadata: Kind is the nicest fallback
			// because the plural resource name (widgets) is awkward.
			name: "CRD without BuiltInMetadata entry",
			rt: model.ResourceTypeEntry{
				Kind:       "Widget",
				APIGroup:   "example.com",
				APIVersion: "v1alpha1",
				Resource:   "widgets",
				Namespaced: true,
			},
			wantName: "prod > Widget",
		},
		{
			// Built-in core resource: ResourceTypeEntry.DisplayName is empty
			// after the discovery refactor, so the resolver must look up
			// BuiltInMetadata ("pods" → "Pods").
			name: "Core built-in (Pods) without DisplayName",
			rt: model.ResourceTypeEntry{
				Kind:       "Pod",
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "pods",
				Namespaced: true,
			},
			wantName: "prod > Pods",
		},
		{
			// Pseudo-resource with a pre-set DisplayName — the resolver
			// must honor it instead of reaching into BuiltInMetadata.
			name: "Pseudo-resource with DisplayName (Releases)",
			rt: model.ResourceTypeEntry{
				DisplayName: "Releases",
				Kind:        "HelmRelease",
				APIGroup:    "_helm",
				APIVersion:  "v1",
				Resource:    "releases",
				Namespaced:  true,
			},
			wantName: "prod > Releases",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			m := Model{
				nav: model.NavigationState{
					Level:        model.LevelResources,
					Context:      "prod",
					ResourceType: tt.rt,
				},
				namespace: "default",
				tabs:      []TabState{{}},
			}

			result, _ := m.bookmarkToSlot("a")
			rm := result.(Model)
			require.NotEmpty(t, rm.bookmarks)
			bm := rm.bookmarks[0]
			assert.Equal(t, tt.wantName, bm.Name)
			assert.Equal(t, tt.rt.ResourceRef(), bm.ResourceType)
		})
	}
}

// TestBookmarkToSlot_AtResourceTypesLevel covers bookmarking while still on
// the resource types list (middle column), before drilling into a specific
// type. Before the fix, nav.ResourceType was zero at this level so the
// bookmark had nothing but the context.
func TestBookmarkToSlot_AtResourceTypesLevel(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	crd := model.ResourceTypeEntry{
		Kind:       "ExternalSecret",
		APIGroup:   "external-secrets.io",
		APIVersion: "v1beta1",
		Resource:   "externalsecrets",
		Namespaced: true,
	}

	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResourceTypes,
			Context: "prod",
		},
		discoveredResources: map[string][]model.ResourceTypeEntry{
			"prod": {crd},
		},
		// allGroupsExpanded keeps the sidebar expanded so the CRD item is
		// directly visible instead of being hidden behind a collapsed group
		// placeholder — matching the user's view when they press m<key>.
		allGroupsExpanded: true,
		middleItems: []model.Item{
			{
				Name:     "ExternalSecrets",
				Kind:     "ExternalSecret",
				Extra:    crd.ResourceRef(),
				Category: "external-secrets.io",
			},
		},
		namespace: "default",
		tabs:      []TabState{{}},
	}
	m.setCursor(0)

	result, _ := m.bookmarkToSlot("a")
	rm := result.(Model)
	require.NotEmpty(t, rm.bookmarks)
	bm := rm.bookmarks[0]
	assert.Equal(t, "prod > ExternalSecrets", bm.Name)
	assert.Equal(t, crd.ResourceRef(), bm.ResourceType,
		"bookmark must capture the ResourceRef of the item under the cursor")
}

// TestBookmarkToSlot_AtResourceTypesLevel_CollapsedGroup verifies that
// attempting to bookmark while the cursor sits on a collapsed group header
// produces a clear error rather than an empty bookmark.
func TestBookmarkToSlot_AtResourceTypesLevel_CollapsedGroup(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResourceTypes,
			Context: "prod",
		},
		allGroupsExpanded: true,
		middleItems: []model.Item{
			{
				Name:     "external-secrets.io",
				Kind:     "__collapsed_group__",
				Category: "external-secrets.io",
			},
		},
		namespace: "default",
		tabs:      []TabState{{}},
	}
	m.setCursor(0)

	result, _ := m.bookmarkToSlot("a")
	rm := result.(Model)
	assert.Empty(t, rm.bookmarks, "should not create a bookmark for a collapsed group header")
	assert.Contains(t, rm.statusMessage, "Select a resource type")
}

// --- navigateToBookmark context switching ---

// customCRDResourceType returns a CRD-based ResourceTypeEntry that won't match
// any built-in resource types. This ensures FindResourceTypeIn only succeeds
// when the correct cluster's discoveredResources contains it.
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
	// A context-free bookmark should look up resources in the current
	// context (cluster-A). Since the CRD is in cluster-A, the lookup
	// succeeds, which proves the function used the current context for
	// lookup.
	crd := customCRDResourceType()

	m := Model{
		nav: model.NavigationState{
			Context: "cluster-A",
		},
		discoveredResources: map[string][]model.ResourceTypeEntry{
			"cluster-A": {crd},
			"cluster-B": {}, // cluster-B does NOT have the CRD
		},
	}

	bm := model.Bookmark{
		ResourceType: crd.ResourceRef(),
		Namespace:    "default",
	}

	// If navigateToBookmark correctly uses the current context (cluster-A)
	// for a context-free bookmark, the CRD lookup will succeed and the
	// function will proceed past the "not found" check. It will then panic
	// at m.client.GetContexts() because client is nil, but the lookup
	// succeeding proves the correct context was used.
	assert.Panics(t, func() {
		m.navigateToBookmark(bm)
	}, "context-free bookmark should find CRD in current context and proceed to client call")
}

func TestNavigateToBookmark_LocalKeepsContext_FailsInWrongCluster(t *testing.T) {
	// Complementary test: the CRD only exists in cluster-B but the bookmark
	// is context-free. A context-free bookmark should NOT look in cluster-B,
	// so the lookup fails and the function returns early with "not found"
	// instead of panicking.
	crd := customCRDResourceType()

	m := Model{
		nav: model.NavigationState{
			Context: "cluster-A",
		},
		discoveredResources: map[string][]model.ResourceTypeEntry{
			"cluster-A": {}, // cluster-A does NOT have the CRD
			"cluster-B": {crd},
		},
	}

	bm := model.Bookmark{
		ResourceType: crd.ResourceRef(),
		Namespace:    "default",
	}

	// Should return cleanly with an error (no panic), proving the function
	// looked in cluster-A (current), not cluster-B.
	result, _ := m.navigateToBookmark(bm)
	resultModel := result.(Model)

	assert.Contains(t, resultModel.statusMessage, "Resource type not found in current cluster")
	assert.True(t, resultModel.statusMessageErr)
	assert.Equal(t, "cluster-A", resultModel.nav.Context,
		"context-free bookmark should not change context when resource is not found")
}

func TestNavigateToBookmark_GlobalSwitchesContext(t *testing.T) {
	// The custom CRD exists only in cluster-B (the bookmark's context).
	// A context-aware bookmark should look up resources in the bookmark's
	// saved context (cluster-B), not the current context (cluster-A).
	// Since the CRD is in cluster-B, the lookup succeeds, proving the
	// function used the bookmark's context.
	crd := customCRDResourceType()

	m := Model{
		nav: model.NavigationState{
			Context: "cluster-A",
		},
		discoveredResources: map[string][]model.ResourceTypeEntry{
			"cluster-A": {}, // cluster-A does NOT have the CRD
			"cluster-B": {crd},
		},
	}

	bm := model.Bookmark{
		Context:      "cluster-B",
		ResourceType: crd.ResourceRef(),
		Namespace:    "default",
	}

	// If navigateToBookmark correctly uses the bookmark's context (cluster-B)
	// for a context-aware bookmark, the CRD lookup will succeed. The function
	// then panics at m.client.GetContexts(), proving the correct context was
	// used.
	assert.Panics(t, func() {
		m.navigateToBookmark(bm)
	}, "context-aware bookmark should find CRD in bookmark context and proceed to client call")
}

func TestNavigateToBookmark_GlobalFailsInWrongCluster(t *testing.T) {
	// Complementary test: the CRD only exists in cluster-A.
	// A context-aware bookmark should look in cluster-B (bookmark context),
	// so the lookup fails and the function returns with "not found".
	crd := customCRDResourceType()

	m := Model{
		nav: model.NavigationState{
			Context: "cluster-A",
		},
		discoveredResources: map[string][]model.ResourceTypeEntry{
			"cluster-A": {crd},
			"cluster-B": {}, // cluster-B does NOT have the CRD
		},
	}

	bm := model.Bookmark{
		Context:      "cluster-B",
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

// --- saveBookmark / removeBookmark immutability ---

func TestRemoveBookmark_DoesNotMutateOriginal(t *testing.T) {
	original := []model.Bookmark{
		{Slot: "a", Name: "bm-a"},
		{Slot: "b", Name: "bm-b"},
		{Slot: "c", Name: "bm-c"},
	}

	result := removeBookmark(original, 0)

	// Result should contain [b, c].
	require.Len(t, result, 2)
	assert.Equal(t, "b", result[0].Slot)
	assert.Equal(t, "c", result[1].Slot)

	// Original must be unchanged — no backing-array mutation.
	require.Len(t, original, 3)
	assert.Equal(t, "a", original[0].Slot, "original[0] should still be 'a'")
	assert.Equal(t, "bm-a", original[0].Name)
	assert.Equal(t, "b", original[1].Slot, "original[1] should still be 'b'")
	assert.Equal(t, "c", original[2].Slot, "original[2] should still be 'c'")
}

func TestRemoveBookmark_MiddleIndex(t *testing.T) {
	original := []model.Bookmark{
		{Slot: "a", Name: "bm-a"},
		{Slot: "b", Name: "bm-b"},
		{Slot: "c", Name: "bm-c"},
	}

	result := removeBookmark(original, 1)

	require.Len(t, result, 2)
	assert.Equal(t, "a", result[0].Slot)
	assert.Equal(t, "c", result[1].Slot)

	// Original must be unchanged.
	assert.Equal(t, "a", original[0].Slot)
	assert.Equal(t, "b", original[1].Slot, "original[1] should still be 'b'")
	assert.Equal(t, "c", original[2].Slot)
}

func TestSaveBookmark_OverwriteDoesNotCorruptOriginal(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	rt := podResourceType()

	// Assign the bookmarks slice to a variable so we can check it after the call.
	bookmarks := []model.Bookmark{
		{Slot: "a", Name: "bm-a", ResourceType: rt.ResourceRef()},
		{Slot: "b", Name: "bm-b", ResourceType: rt.ResourceRef()},
		{Slot: "c", Name: "bm-c", ResourceType: rt.ResourceRef()},
	}

	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			Context:      "test",
			ResourceType: rt,
		},
		namespace: "default",
		tabs:      []TabState{{}},
		bookmarks: bookmarks,
	}

	// Overwrite slot "a" — triggers in-place removal + append.
	result, _ := m.saveBookmark(model.Bookmark{
		Slot:         "a",
		Name:         "bm-a-updated",
		ResourceType: rt.ResourceRef(),
	})
	resultModel := result.(Model)

	// Result should have [b, c, a-updated].
	require.Len(t, resultModel.bookmarks, 3)
	assert.Equal(t, "b", resultModel.bookmarks[0].Slot)
	assert.Equal(t, "c", resultModel.bookmarks[1].Slot)
	assert.Equal(t, "a", resultModel.bookmarks[2].Slot)
	assert.Equal(t, "bm-a-updated", resultModel.bookmarks[2].Name)

	// The original bookmarks slice must be untouched — no backing-array corruption.
	require.Len(t, bookmarks, 3)
	assert.Equal(t, "a", bookmarks[0].Slot, "original bookmarks[0] should be 'a'")
	assert.Equal(t, "bm-a", bookmarks[0].Name, "original bookmarks[0].Name should be 'bm-a'")
	assert.Equal(t, "b", bookmarks[1].Slot, "original bookmarks[1] should be 'b'")
	assert.Equal(t, "c", bookmarks[2].Slot, "original bookmarks[2] should be 'c'")
	assert.Equal(t, "bm-c", bookmarks[2].Name, "original bookmarks[2].Name should be 'bm-c'")
}

func TestNavigateToBookmark_LocalResourceNotFound(t *testing.T) {
	// Use a custom CRD ref that doesn't exist anywhere.
	// With an empty discoveredResources for the current cluster, the function
	// should return the "not found" error.
	m := Model{
		nav: model.NavigationState{
			Context: "cluster-A",
		},
		discoveredResources: map[string][]model.ResourceTypeEntry{
			"cluster-A": {},
		},
	}

	bm := model.Bookmark{
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

func TestCovHandleBookmarkOverlayKeyDispatch(t *testing.T) {
	m := baseModelCov()
	m.bookmarkSearchMode = bookmarkModeFilter
	r, _ := m.handleBookmarkOverlayKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)

	m.bookmarkSearchMode = bookmarkModeConfirmDelete
	r, _ = m.handleBookmarkOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)

	m.bookmarkSearchMode = bookmarkModeConfirmDeleteAll
	r, _ = m.handleBookmarkOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)
}

func TestCovHandleBookmarkNormalMode(t *testing.T) {
	filtered := []model.Bookmark{{Name: "a", Slot: "1"}, {Name: "b", Slot: "2"}, {Name: "c", Slot: "3"}}
	m := baseModelCov()
	m.overlay = overlayBookmarks

	r, _ := m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyEscape}, nil)
	assert.Equal(t, overlayNone, r.(Model).overlay)

	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, filtered)
	assert.Equal(t, 1, r.(Model).overlayCursor)

	m.overlayCursor = 1
	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, filtered)
	assert.Equal(t, 0, r.(Model).overlayCursor)

	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}, filtered)
	assert.Equal(t, 2, r.(Model).overlayCursor)

	m.pendingG = true
	m.overlayCursor = 2
	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}, filtered)
	assert.Equal(t, 0, r.(Model).overlayCursor)

	// "D" is no longer a delete hotkey — it must fall through to slot jump
	// (which is a no-op here because no bookmark is stored in slot D).
	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}}, filtered)
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)

	// ctrl+x is now the single-delete hotkey.
	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyCtrlX}, filtered)
	assert.Equal(t, bookmarkModeConfirmDelete, r.(Model).bookmarkSearchMode)

	// alt+x is now the delete-all hotkey.
	m.bookmarkSearchMode = bookmarkModeNormal
	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}, Alt: true}, filtered)
	assert.Equal(t, bookmarkModeConfirmDeleteAll, r.(Model).bookmarkSearchMode)

	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}, filtered)
	assert.Equal(t, bookmarkModeFilter, r.(Model).bookmarkSearchMode)

	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyCtrlD}, filtered)
	assert.GreaterOrEqual(t, r.(Model).overlayCursor, 0)

	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyCtrlU}, filtered)
	assert.GreaterOrEqual(t, r.(Model).overlayCursor, 0)
}

func TestCovHandleBookmarkFilterMode(t *testing.T) {
	m := baseModelCov()
	m.bookmarkSearchMode = bookmarkModeFilter

	r, _ := m.handleBookmarkFilterMode(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)

	m.bookmarkSearchMode = bookmarkModeFilter
	m.bookmarkFilter = TextInput{Value: "pr", Cursor: 2}
	r, _ = m.handleBookmarkFilterMode(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "p", r.(Model).bookmarkFilter.Value)

	m.bookmarkFilter = TextInput{}
	r, _ = m.handleBookmarkFilterMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	assert.Equal(t, "p", r.(Model).bookmarkFilter.Value)

	m.bookmarkFilter = TextInput{Value: "hello", Cursor: 5}
	r, _ = m.handleBookmarkFilterMode(tea.KeyMsg{Type: tea.KeyCtrlW})
	assert.Empty(t, r.(Model).bookmarkFilter.Value)

	r, _ = m.handleBookmarkFilterMode(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)
}

func TestCovHandleBookmarkConfirmDelete(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := baseModelCov()
	m.bookmarkSearchMode = bookmarkModeConfirmDelete
	m.bookmarks = []model.Bookmark{{Name: "test", Slot: "a"}}
	r, cmd := m.handleBookmarkConfirmDelete(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)
	assert.NotNil(t, cmd)

	r, cmd = m.handleBookmarkConfirmDelete(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)
	assert.NotNil(t, cmd)
}

func TestCovHandleBookmarkConfirmDeleteAll(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := baseModelCov()
	m.bookmarks = []model.Bookmark{{Name: "test", Slot: "a"}}
	r, cmd := m.handleBookmarkConfirmDeleteAll(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)
	assert.NotNil(t, cmd)

	m.bookmarks = []model.Bookmark{{Name: "test", Slot: "a"}}
	r, cmd = m.handleBookmarkConfirmDeleteAll(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)
	assert.NotNil(t, cmd)
}

func TestCovBuildSessionTabState(t *testing.T) {
	st := &SessionTab{
		Context:       "my-ctx",
		Namespace:     "my-ns",
		AllNamespaces: false,
		ResourceType:  "",
	}
	tab := buildSessionTabState(st, nil)
	assert.Equal(t, "my-ctx", tab.nav.Context)
	assert.Equal(t, "my-ns", tab.namespace)
	assert.Equal(t, model.LevelResourceTypes, tab.nav.Level)
}

func TestCovBuildSessionTabStateAllNS(t *testing.T) {
	st := &SessionTab{
		Context:       "my-ctx",
		AllNamespaces: true,
	}
	tab := buildSessionTabState(st, nil)
	assert.True(t, tab.allNamespaces)
}

func TestCovBuildSessionTabStateNoContext(t *testing.T) {
	st := &SessionTab{}
	tab := buildSessionTabState(st, nil)
	assert.Equal(t, model.LevelClusters, tab.nav.Level)
}

func TestCovBuildSessionTabStateWithSelectedNS(t *testing.T) {
	st := &SessionTab{
		Context:            "ctx",
		Namespace:          "ns1",
		SelectedNamespaces: []string{"ns1", "ns2"},
	}
	tab := buildSessionTabState(st, nil)
	assert.True(t, tab.selectedNamespaces["ns1"])
	assert.True(t, tab.selectedNamespaces["ns2"])
}

func TestFinal2RestoreSessionSingleTab(t *testing.T) {
	m := baseFinalModel()
	m.pendingSession = &SessionState{
		Context:   "test-ctx",
		Namespace: "default",
	}
	m.sessionRestored = false
	contexts := []model.Item{{Name: "test-ctx"}, {Name: "other-ctx"}}
	result, cmd := m.restoreSession(contexts)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.sessionRestored)
}

func TestFinal2RestoreSingleTabSessionContextNotFound(t *testing.T) {
	m := baseFinalModel()
	sess := &SessionState{Context: "nonexistent"}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, _ := m.restoreSingleTabSession(sess, contexts)
	_ = result.(Model)
}

func TestFinal2RestoreSingleTabSessionWithResourceType(t *testing.T) {
	m := baseFinalModel()
	// Pre-populate discoveredResources so restoreSingleTabSession can resolve the
	// saved ResourceType ref against the parameter-only Find* lookup.
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}
	sess := &SessionState{
		Context:      "test-ctx",
		Namespace:    "default",
		ResourceType: "/v1/pods",
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, cmd := m.restoreSingleTabSession(sess, contexts)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, model.LevelResources, rm.nav.Level)
}

// TestFinal2RestoreSingleTabSessionSeedFallback verifies that session
// restore resolves core K8s resource types from the seed list even when
// runtime API discovery has not yet populated discoveredResources. This
// regression guard prevents users from being dropped back at the resource
// types list instead of their saved view.
func TestFinal2RestoreSingleTabSessionSeedFallback(t *testing.T) {
	m := baseFinalModel()
	// Intentionally do NOT populate discoveredResources — simulates the
	// real startup path where discovery is still in flight.
	sess := &SessionState{
		Context:      "test-ctx",
		Namespace:    "default",
		ResourceType: "/v1/pods",
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, cmd := m.restoreSingleTabSession(sess, contexts)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, model.LevelResources, rm.nav.Level, "must navigate into resources level via seed fallback")
	assert.Equal(t, "Pod", rm.nav.ResourceType.Kind)
	assert.Equal(t, "pods", rm.nav.ResourceType.Resource)
}

// TestResolveSessionResourceTypeSeedFallback is a unit test for the helper
// that performs the discovered-then-seed lookup.
func TestResolveSessionResourceTypeSeedFallback(t *testing.T) {
	// With empty discovered set, /v1/pods should resolve from the seed list.
	rt, ok := resolveSessionResourceType("/v1/pods", nil)
	require.True(t, ok)
	assert.Equal(t, "Pod", rt.Kind)

	// With a populated discovered set that doesn't include Pod, seed fallback still applies.
	discovered := []model.ResourceTypeEntry{
		{Kind: "Custom", APIGroup: "example.com", APIVersion: "v1", Resource: "customs"},
	}
	rt, ok = resolveSessionResourceType("/v1/pods", discovered)
	require.True(t, ok)
	assert.Equal(t, "Pod", rt.Kind)

	// Unknown ref that is in neither discovered nor seed returns !ok.
	_, ok = resolveSessionResourceType("unknown.example.com/v1/widgets", nil)
	assert.False(t, ok)
}

func TestFinal2RestoreSingleTabSessionWithResourceName(t *testing.T) {
	m := baseFinalModel()
	// Pre-populate discoveredResources so restoreSingleTabSession can resolve the
	// saved ResourceType ref against the parameter-only Find* lookup.
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}
	sess := &SessionState{
		Context:      "test-ctx",
		Namespace:    "default",
		ResourceType: "/v1/pods",
		ResourceName: "my-pod",
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, cmd := m.restoreSingleTabSession(sess, contexts)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, "my-pod", rm.pendingTarget)
}

func TestFinal2RestoreSingleTabSessionNoResourceType(t *testing.T) {
	m := baseFinalModel()
	sess := &SessionState{
		Context:   "test-ctx",
		Namespace: "default",
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, cmd := m.restoreSingleTabSession(sess, contexts)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, model.LevelResourceTypes, rm.nav.Level)
}

func TestFinal2RestoreSingleTabSessionAllNamespaces(t *testing.T) {
	m := baseFinalModel()
	sess := &SessionState{
		Context:       "test-ctx",
		AllNamespaces: true,
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, _ := m.restoreSingleTabSession(sess, contexts)
	rm := result.(Model)
	assert.True(t, rm.allNamespaces)
}

func TestFinal2RestoreSingleTabSessionSelectedNS(t *testing.T) {
	m := baseFinalModel()
	sess := &SessionState{
		Context:            "test-ctx",
		Namespace:          "ns1",
		SelectedNamespaces: []string{"ns1", "ns2"},
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, _ := m.restoreSingleTabSession(sess, contexts)
	rm := result.(Model)
	assert.True(t, rm.selectedNamespaces["ns1"])
	assert.True(t, rm.selectedNamespaces["ns2"])
}

func TestFinal2RestoreMultiTabSession(t *testing.T) {
	m := baseFinalModel()
	sess := &SessionState{
		ActiveTab: 0,
		Tabs: []SessionTab{
			{Context: "test-ctx", Namespace: "default", ResourceType: "/v1/pods"},
			{Context: "test-ctx", Namespace: "kube-system"},
		},
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, cmd := m.restoreMultiTabSession(sess, contexts)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, 2, len(rm.tabs))
	assert.Equal(t, 0, rm.activeTab)
}

func TestFinal2RestoreMultiTabSessionInvalidActiveTab(t *testing.T) {
	m := baseFinalModel()
	sess := &SessionState{
		ActiveTab: 5,
		Tabs: []SessionTab{
			{Context: "test-ctx", Namespace: "default"},
		},
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, _ := m.restoreMultiTabSession(sess, contexts)
	rm := result.(Model)
	assert.Equal(t, 0, rm.activeTab)
}

func TestFinal2RestoreMultiTabSessionContextNotFound(t *testing.T) {
	m := baseFinalModel()
	sess := &SessionState{
		ActiveTab: 0,
		Tabs: []SessionTab{
			{Context: "nonexistent"},
		},
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, _ := m.restoreMultiTabSession(sess, contexts)
	_ = result.(Model)
}

func TestFinal2RestoreSessionMultiTab(t *testing.T) {
	m := baseFinalModel()
	m.pendingSession = &SessionState{
		ActiveTab: 0,
		Tabs: []SessionTab{
			{Context: "test-ctx", Namespace: "default"},
		},
	}
	m.sessionRestored = false
	contexts := []model.Item{{Name: "test-ctx"}}
	result, _ := m.restoreSession(contexts)
	rm := result.(Model)
	assert.True(t, rm.sessionRestored)
}

func TestFinalBookmarkToSlotTooLow(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelClusters
	result, _ := m.bookmarkToSlot("a")
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Navigate to a resource type")
}

func TestFinalBookmarkToSlotLocal(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := baseFinalModel()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{DisplayName: "Pods", Kind: "Pod", Resource: "pods"}
	result, cmd := m.bookmarkToSlot("a")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Mark 'a' set")
}

func TestFinalBookmarkToSlotGlobal(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := baseFinalModel()
	m.nav.Level = model.LevelResources
	m.nav.Context = "prod-cluster"
	m.nav.ResourceType = model.ResourceTypeEntry{DisplayName: "Pods", Kind: "Pod", Resource: "pods"}
	result, cmd := m.bookmarkToSlot("A")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Mark 'A' set")
}

func TestFinalBookmarkToSlotOverwrite(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{DisplayName: "Pods", Kind: "Pod", Resource: "pods"}
	m.bookmarks = []model.Bookmark{{Slot: "a", Name: "existing"}}
	result, cmd := m.bookmarkToSlot("a")
	assert.Nil(t, cmd) // Should prompt for confirmation
	rm := result.(Model)
	assert.NotNil(t, rm.pendingBookmark)
}

func TestFinalBookmarkToSlotAllNamespaces(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := baseFinalModel()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{DisplayName: "Pods", Kind: "Pod", Resource: "pods"}
	m.allNamespaces = true
	result, cmd := m.bookmarkToSlot("b")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalBookmarkToSlotMultiNamespaces(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := baseFinalModel()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{DisplayName: "Pods", Kind: "Pod", Resource: "pods"}
	m.selectedNamespaces = map[string]bool{"ns1": true, "ns2": true}
	result, cmd := m.bookmarkToSlot("c")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalFilteredBookmarksEmpty(t *testing.T) {
	m := baseFinalModel()
	m.bookmarks = nil
	result := m.filteredBookmarks()
	assert.Empty(t, result)
}

func TestFinalFilteredBookmarksNoFilter(t *testing.T) {
	m := baseFinalModel()
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}, {Name: "bm2", Slot: "b"}}
	result := m.filteredBookmarks()
	assert.Len(t, result, 2)
}

func TestFinalBookmarkDeleteCurrentEmpty(t *testing.T) {
	m := baseFinalModel()
	m.bookmarks = nil
	cmd := m.bookmarkDeleteCurrent()
	assert.Nil(t, cmd)
}

func TestFinalBookmarkDeleteCurrentValid(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := baseFinalModel()
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}}
	m.overlayCursor = 0
	cmd := m.bookmarkDeleteCurrent()
	assert.NotNil(t, cmd)
	assert.Empty(t, m.bookmarks)
}

func TestFinalBookmarkDeleteAll(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := baseFinalModel()
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}, {Name: "bm2", Slot: "b"}}
	cmd := m.bookmarkDeleteAll()
	assert.NotNil(t, cmd)
	assert.Nil(t, m.bookmarks)
}

func TestFinalHandleBookmarkNormalModeEsc(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	result, _ := m.handleBookmarkOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleBookmarkNormalModeJ(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}, {Name: "bm2", Slot: "b"}}
	m.overlayCursor = 0
	result, _ := m.handleBookmarkOverlayKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.overlayCursor)
}

func TestFinalHandleBookmarkNormalModeK(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}, {Name: "bm2", Slot: "b"}}
	m.overlayCursor = 1
	result, _ := m.handleBookmarkOverlayKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.overlayCursor)
}

func TestFinalHandleBookmarkNormalModeGG(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}, {Name: "bm2", Slot: "b"}}
	m.overlayCursor = 1
	m.pendingG = true
	result, _ := m.handleBookmarkOverlayKey(keyMsg("g"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.overlayCursor)
}

func TestFinalHandleBookmarkNormalModeG(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	result, _ := m.handleBookmarkOverlayKey(keyMsg("G"))
	rm := result.(Model)
	_ = rm
}

func TestFinalHandleBookmarkNormalModeSlash(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	result, _ := m.handleBookmarkOverlayKey(keyMsg("/"))
	rm := result.(Model)
	assert.Equal(t, bookmarkModeFilter, rm.bookmarkSearchMode)
}

func TestFinalHandleBookmarkNormalModeDJumpsToSlot(t *testing.T) {
	// Pressing "D" in the bookmark overlay must NOT trigger delete. It should
	// be passed through to the slot-jump default branch so context-free
	// bookmarks stored in slot D can be reached from the overlay. This guards
	// against regressing the delete hotkey back onto the uppercase letter.
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}}
	m.overlayCursor = 0
	result, _ := m.handleBookmarkOverlayKey(keyMsg("D"))
	rm := result.(Model)
	assert.Equal(t, bookmarkModeNormal, rm.bookmarkSearchMode)
}

func TestFinalHandleBookmarkNormalModeCtrlXSingleDelete(t *testing.T) {
	// ctrl+x is the single-bookmark delete hotkey (moved off of "D" to free
	// the uppercase letter for context-free slot jumps).
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}}
	m.overlayCursor = 0
	result, _ := m.handleBookmarkOverlayKey(keyMsg("ctrl+x"))
	rm := result.(Model)
	assert.Equal(t, bookmarkModeConfirmDelete, rm.bookmarkSearchMode)
}

func TestFinalHandleBookmarkNormalModeAltXDeleteAll(t *testing.T) {
	// alt+x is the delete-all hotkey (moved off of ctrl+x which now handles
	// single delete). Uses the "cut one" / "cut all" mental model.
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}}
	result, _ := m.handleBookmarkOverlayKey(keyMsg("alt+x"))
	rm := result.(Model)
	assert.Equal(t, bookmarkModeConfirmDeleteAll, rm.bookmarkSearchMode)
}

func TestFinalHandleBookmarkNormalModeCtrlD(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = make([]model.Bookmark, 20)
	m.overlayCursor = 0
	result, _ := m.handleBookmarkOverlayKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.Greater(t, rm.overlayCursor, 0)
}

func TestFinalHandleBookmarkNormalModeCtrlU(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = make([]model.Bookmark, 20)
	m.overlayCursor = 15
	result, _ := m.handleBookmarkOverlayKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.Less(t, rm.overlayCursor, 15)
}

func TestFinalHandleBookmarkNormalModeCtrlF(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = make([]model.Bookmark, 30)
	m.overlayCursor = 0
	result, _ := m.handleBookmarkOverlayKey(keyMsg("ctrl+f"))
	rm := result.(Model)
	assert.Greater(t, rm.overlayCursor, 0)
}

func TestFinalHandleBookmarkNormalModeCtrlB(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = make([]model.Bookmark, 30)
	m.overlayCursor = 25
	result, _ := m.handleBookmarkOverlayKey(keyMsg("ctrl+b"))
	rm := result.(Model)
	assert.Less(t, rm.overlayCursor, 25)
}

func TestFinalHandleBookmarkFilterModeEsc(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeFilter
	result, _ := m.handleBookmarkOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, bookmarkModeNormal, rm.bookmarkSearchMode)
}

func TestFinalHandleBookmarkFilterModeEnter(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeFilter
	result, _ := m.handleBookmarkOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, bookmarkModeNormal, rm.bookmarkSearchMode)
}

func TestFinalHandleBookmarkFilterModeTyping(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeFilter
	result, _ := m.handleBookmarkOverlayKey(keyMsg("a"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.overlayCursor)
}

func TestFinalHandleBookmarkConfirmDeleteYes(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeConfirmDelete
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}}
	m.overlayCursor = 0
	result, cmd := m.handleBookmarkOverlayKey(keyMsg("y"))
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, bookmarkModeNormal, rm.bookmarkSearchMode)
}

func TestFinalHandleBookmarkConfirmDeleteNo(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeConfirmDelete
	result, _ := m.handleBookmarkOverlayKey(keyMsg("n"))
	rm := result.(Model)
	assert.Equal(t, bookmarkModeNormal, rm.bookmarkSearchMode)
	assert.Contains(t, rm.statusMessage, "Cancelled")
}

func TestFinalHandleBookmarkConfirmDeleteAllYes(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeConfirmDeleteAll
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}}
	result, cmd := m.handleBookmarkOverlayKey(keyMsg("Y"))
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, bookmarkModeNormal, rm.bookmarkSearchMode)
}

func TestFinalHandleBookmarkConfirmDeleteAllNo(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeConfirmDeleteAll
	result, _ := m.handleBookmarkOverlayKey(keyMsg("n"))
	rm := result.(Model)
	assert.Equal(t, bookmarkModeNormal, rm.bookmarkSearchMode)
}

func TestFinalContextInList(t *testing.T) {
	items := []model.Item{{Name: "ctx-1"}, {Name: "ctx-2"}}
	assert.True(t, contextInList("ctx-1", items))
	assert.True(t, contextInList("ctx-2", items))
	assert.False(t, contextInList("ctx-3", items))
}

func TestFinalContextInListEmpty(t *testing.T) {
	assert.False(t, contextInList("any", nil))
}

func TestFinalApplySessionNamespaces(t *testing.T) {
	m := baseFinalModel()
	applySessionNamespaces(&m, true, "", nil)
	assert.True(t, m.allNamespaces)

	m2 := baseFinalModel()
	applySessionNamespaces(&m2, false, "custom-ns", nil)
	assert.Equal(t, "custom-ns", m2.namespace)
	assert.False(t, m2.allNamespaces)

	m3 := baseFinalModel()
	applySessionNamespaces(&m3, false, "ns1", []string{"ns1", "ns2"})
	assert.Equal(t, "ns1", m3.namespace)
	assert.True(t, m3.selectedNamespaces["ns1"])
	assert.True(t, m3.selectedNamespaces["ns2"])
}

func TestFinalBuildSessionTabState(t *testing.T) {
	st := SessionTab{
		Context:      "ctx-1",
		Namespace:    "ns-1",
		ResourceType: "/v1/pods",
	}
	// Provide the discovered resource type so the saved ResourceType ref
	// can be resolved; without this, tab.nav.Level falls back to
	// LevelResourceTypes because FindResourceTypeIn iterates the parameter only.
	discovered := []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}
	tab := buildSessionTabState(&st, discovered)
	assert.Equal(t, "ctx-1", tab.nav.Context)
	assert.Equal(t, model.LevelResources, tab.nav.Level)
}

func TestFinalBuildSessionTabStateNoResourceType(t *testing.T) {
	st := SessionTab{
		Context: "ctx-1",
	}
	tab := buildSessionTabState(&st, nil)
	assert.Equal(t, model.LevelResourceTypes, tab.nav.Level)
}

func TestFinalBuildSessionTabStateNoContext(t *testing.T) {
	st := SessionTab{}
	tab := buildSessionTabState(&st, nil)
	assert.Equal(t, model.LevelClusters, tab.nav.Level)
}

func TestFinalBuildSessionTabStateAllNamespaces(t *testing.T) {
	st := SessionTab{
		Context:       "ctx-1",
		AllNamespaces: true,
	}
	tab := buildSessionTabState(&st, nil)
	assert.True(t, tab.allNamespaces)
}

func TestFinalBuildSessionTabStateSelectedNS(t *testing.T) {
	st := SessionTab{
		Context:            "ctx-1",
		Namespace:          "ns1",
		SelectedNamespaces: []string{"ns1", "ns2"},
	}
	tab := buildSessionTabState(&st, nil)
	assert.True(t, tab.selectedNamespaces["ns1"])
	assert.True(t, tab.selectedNamespaces["ns2"])
}

func TestFinalBuildSessionTabStateNSOnly(t *testing.T) {
	st := SessionTab{
		Context:   "ctx-1",
		Namespace: "ns1",
	}
	tab := buildSessionTabState(&st, nil)
	assert.Equal(t, "ns1", tab.namespace)
	assert.True(t, tab.selectedNamespaces["ns1"])
}

func TestFinalNavigateToBookmarkResourceNotFound(t *testing.T) {
	m := baseFinalModel()
	bm := model.Bookmark{
		ResourceType: "nonexistent",
	}
	result, cmd := m.navigateToBookmark(bm)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "not found")
}

func TestFinalNavigateToBookmarkAllNamespaces(t *testing.T) {
	m := baseFinalModel()
	podRT := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{podRT}
	bm := model.Bookmark{
		ResourceType: podRT.ResourceRef(),
		Namespace:    "",
	}
	result, cmd := m.navigateToBookmark(bm)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.allNamespaces)
}

func TestFinalNavigateToBookmarkMultiNS(t *testing.T) {
	m := baseFinalModel()
	podRT := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{podRT}
	bm := model.Bookmark{
		ResourceType: podRT.ResourceRef(),
		Namespaces:   []string{"ns1", "ns2"},
	}
	result, cmd := m.navigateToBookmark(bm)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.selectedNamespaces["ns1"])
	assert.True(t, rm.selectedNamespaces["ns2"])
}

func TestFinalNavigateToBookmarkSingleNS(t *testing.T) {
	m := baseFinalModel()
	podRT := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{podRT}
	bm := model.Bookmark{
		ResourceType: podRT.ResourceRef(),
		Namespace:    "production",
	}
	result, cmd := m.navigateToBookmark(bm)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, "production", rm.namespace)
}

func TestFinalNavigateToBookmarkGlobal(t *testing.T) {
	m := baseFinalModel()
	// Context-aware bookmarks switch context. CRD must have matching ResourceRef.
	podRT := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.discoveredResources["prod-ctx"] = []model.ResourceTypeEntry{podRT}
	bm := model.Bookmark{
		ResourceType: podRT.ResourceRef(),
		Context:      "prod-ctx",
		Namespace:    "default",
	}
	result, cmd := m.navigateToBookmark(bm)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Jumped to")
}

func TestFinalNavigateToBookmarkSingleNamespaceInList(t *testing.T) {
	m := baseFinalModel()
	podRT := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{podRT}
	bm := model.Bookmark{
		ResourceType: podRT.ResourceRef(),
		Namespaces:   []string{"only-ns"},
	}
	result, cmd := m.navigateToBookmark(bm)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Jumped to")
}

func TestFinalBookmarkDeleteAllWithFilter(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := baseFinalModel()
	m.bookmarks = []model.Bookmark{
		{Name: "alpha-bm", Slot: "a"},
		{Name: "beta-bm", Slot: "b"},
		{Name: "gamma-bm", Slot: "c"},
	}
	m.bookmarkFilter.Value = "alpha"
	cmd := m.bookmarkDeleteAll()
	assert.NotNil(t, cmd)
	// Only bookmarks not matching the filter should remain.
	assert.Equal(t, 2, len(m.bookmarks))
}

func TestFinalBookmarkDeleteAllEmptyFiltered(t *testing.T) {
	m := baseFinalModel()
	m.bookmarks = nil
	cmd := m.bookmarkDeleteAll()
	assert.Nil(t, cmd)
}

func TestFinalHandleBookmarkEnterNoItems(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = nil
	result, cmd := m.handleBookmarkOverlayKey(keyMsg("enter"))
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestFinalHandleBookmarkSlotJump(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = []model.Bookmark{{Name: "bm", Slot: "x"}}
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true},
	}
	// Pressing a slot key that doesn't exist should show error.
	result, _ := m.handleBookmarkOverlayKey(keyMsg("z"))
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "not set")
}

// Test the removeBookmark helper used by bookmark delete.
func TestFinalRemoveBookmark(t *testing.T) {
	bms := []model.Bookmark{
		{Slot: "a", Name: "first"},
		{Slot: "b", Name: "second"},
		{Slot: "c", Name: "third"},
	}
	result := removeBookmark(bms, 1)
	assert.Len(t, result, 2)
	assert.Equal(t, "a", result[0].Slot)
	assert.Equal(t, "c", result[1].Slot)
}

// TestSaveBookmarkStatusMessageIncludesKind verifies the status message
// after setting a bookmark explicitly states whether it is context-aware
// or context-free. This makes the new slot-case convention visible to
// users on every save.
func TestSaveBookmarkStatusMessageIncludesKind(t *testing.T) {
	t.Run("context-aware bookmark", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("XDG_STATE_HOME", tmpDir)
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelResources,
				Context:      "test",
				ResourceType: podResourceType(),
			},
			namespace: "default",
			tabs:      []TabState{{}},
		}
		result, _ := m.bookmarkToSlot("a")
		rm := result.(Model)
		assert.Contains(t, rm.statusMessage, "(context-aware)",
			"status message must call out context-aware kind")
	})

	t.Run("context-free bookmark", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("XDG_STATE_HOME", tmpDir)
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelResources,
				Context:      "test",
				ResourceType: podResourceType(),
			},
			namespace: "default",
			tabs:      []TabState{{}},
		}
		result, _ := m.bookmarkToSlot("A")
		rm := result.(Model)
		assert.Contains(t, rm.statusMessage, "(context-free)",
			"status message must call out context-free kind")
	})
}

// --- session restore wires security availability ---

// TestRestoreSingleTabSessionDispatchesSecurityAvailability is the
// regression guard for the cold-start bug: if session restore does not
// dispatch loadSecurityAvailability, the probe never runs and the Security
// category stays empty forever — because the only other call site is
// navigateChildCluster, which is NOT hit when the app restores a session
// past the cluster picker.
//
// We can't fully invoke the restore's command batch in isolation — some of
// the inner cmds panic against the fake dynamic client (missing list
// kinds). Instead the test asserts on two observable side effects:
//
//  1. refreshSecuritySources ran, which replaces m.securityManager with a
//     fresh Manager wired to the newly-selected context. This is detectable
//     because the manager pointer changes.
//  2. The returned batch contains one more command than the pre-fix batch
//     would have. Before the fix the restore dispatched two commands
//     (discoverAPIResources + loadPreview); after the fix it dispatches
//     three, with loadSecurityAvailability in between.
func TestRestoreSingleTabSessionDispatchesSecurityAvailability(t *testing.T) {
	m := baseFinalModel()
	// Install an initial manager with a fake source — when restore runs
	// refreshSecuritySources, this pointer must be replaced by a new one.
	oldMgr := security.NewManager()
	oldMgr.Register(&security.FakeSource{NameStr: "sentinel", Available: true})
	m.securityManager = oldMgr
	m.securityAvailabilityByName = make(map[string]bool)

	sess := &SessionState{Context: "test-ctx", Namespace: "default"}
	contexts := []model.Item{{Name: "test-ctx"}}

	result, cmd := m.restoreSingleTabSession(sess, contexts)
	rm, ok := result.(Model)
	require.True(t, ok)
	require.NotNil(t, cmd, "restore must return a command batch")

	// refreshSecuritySources must have replaced the manager.
	assert.NotSame(t, oldMgr, rm.securityManager,
		"restore must call refreshSecuritySources, which replaces the manager")

	// The returned cmd must be a tea.Batch. Walk its inner cmds — the fix
	// adds loadSecurityAvailability between discoverAPIResources and
	// loadPreview, so the batch length must be 3.
	batch, ok := cmd().(tea.BatchMsg)
	require.True(t, ok, "restore must return a tea.Batch")
	assert.GreaterOrEqual(t, len(batch), 3,
		"restore must dispatch discoverAPIResources + loadSecurityAvailability + loadPreview")
}
