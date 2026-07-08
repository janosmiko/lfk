package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- effectiveNamespace: negation ---

func TestEffectiveNamespaceNegated(t *testing.T) {
	tests := []struct {
		name               string
		namespace          string
		allNamespaces      bool
		selectedNamespaces map[string]bool
		nsSelectionNegated bool
		expected           string
	}{
		{
			name:               "negated with one excluded: returns empty (must list all)",
			namespace:          "default",
			selectedNamespaces: map[string]bool{"kube-system": true},
			nsSelectionNegated: true,
			expected:           "",
		},
		{
			name:               "negated with multiple excluded: returns empty",
			namespace:          "default",
			selectedNamespaces: map[string]bool{"kube-system": true, "monitoring": true},
			nsSelectionNegated: true,
			expected:           "",
		},
		{
			name:               "negated with empty set: returns empty (equivalent to all)",
			namespace:          "default",
			selectedNamespaces: nil,
			nsSelectionNegated: true,
			expected:           "",
		},
		{
			name:               "not negated, single selected: unchanged",
			namespace:          "default",
			selectedNamespaces: map[string]bool{"production": true},
			nsSelectionNegated: false,
			expected:           "production",
		},
		{
			name:               "not negated, allNamespaces: unchanged",
			namespace:          "default",
			allNamespaces:      true,
			nsSelectionNegated: false,
			expected:           "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				namespace:          tt.namespace,
				allNamespaces:      tt.allNamespaces,
				selectedNamespaces: tt.selectedNamespaces,
				nsSelectionNegated: tt.nsSelectionNegated,
			}
			assert.Equal(t, tt.expected, m.effectiveNamespace())
		})
	}
}

// --- filterLoadedItemsBySelectedNamespaces: negation ---

func TestFilterLoadedItemsBySelectedNamespacesNegated(t *testing.T) {
	items := []model.Item{
		{Name: "pod-a", Namespace: "default"},
		{Name: "pod-b", Namespace: "kube-system"},
		{Name: "pod-c", Namespace: "monitoring"},
		{Name: "node-1", Namespace: ""}, // cluster-scoped: always kept
	}

	t.Run("exclude one namespace: keeps others and cluster-scoped", func(t *testing.T) {
		m := Model{
			selectedNamespaces: map[string]bool{"kube-system": true},
			nsSelectionNegated: true,
		}
		result := m.filterLoadedItemsBySelectedNamespaces(items)
		names := negNsItemNames(result)
		assert.Contains(t, names, "pod-a")
		assert.NotContains(t, names, "pod-b")
		assert.Contains(t, names, "pod-c")
		assert.Contains(t, names, "node-1") // cluster-scoped always kept
	})

	t.Run("exclude multiple namespaces", func(t *testing.T) {
		m := Model{
			selectedNamespaces: map[string]bool{"kube-system": true, "monitoring": true},
			nsSelectionNegated: true,
		}
		result := m.filterLoadedItemsBySelectedNamespaces(items)
		names := negNsItemNames(result)
		assert.Contains(t, names, "pod-a")
		assert.NotContains(t, names, "pod-b")
		assert.NotContains(t, names, "pod-c")
		assert.Contains(t, names, "node-1")
	})

	t.Run("single excluded namespace is NOT short-circuited", func(t *testing.T) {
		// This is the critical regression: the old early-return
		// `if len(m.selectedNamespaces) <= 1 { return items }` must NOT fire
		// when negated with exactly one excluded namespace.
		m := Model{
			selectedNamespaces: map[string]bool{"kube-system": true},
			nsSelectionNegated: true,
		}
		result := m.filterLoadedItemsBySelectedNamespaces(items)
		// kube-system must be excluded
		for _, item := range result {
			assert.NotEqual(t, "kube-system", item.Namespace,
				"kube-system must be excluded when negated")
		}
	})

	t.Run("non-negated behavior unchanged with two namespaces", func(t *testing.T) {
		m := Model{
			selectedNamespaces: map[string]bool{"default": true, "monitoring": true},
			nsSelectionNegated: false,
		}
		result := m.filterLoadedItemsBySelectedNamespaces(items)
		names := negNsItemNames(result)
		assert.Contains(t, names, "pod-a")
		assert.NotContains(t, names, "pod-b")
		assert.Contains(t, names, "pod-c")
		assert.Contains(t, names, "node-1")
	})

	t.Run("non-negated behavior unchanged with one namespace (early-return)", func(t *testing.T) {
		m := Model{
			selectedNamespaces: map[string]bool{"default": true},
			nsSelectionNegated: false,
		}
		result := m.filterLoadedItemsBySelectedNamespaces(items)
		// Early-return: all items returned unchanged
		assert.Len(t, result, len(items))
	})
}

func negNsItemNames(items []model.Item) []string {
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.Name
	}
	return names
}

// --- fetchFingerprint: negation changes fingerprint ---

func TestFetchFingerprintNegationDifferent(t *testing.T) {
	sel := map[string]bool{"kube-system": true}

	mInclude := Model{
		namespace:          "default",
		selectedNamespaces: sel,
		nsSelectionNegated: false,
	}
	mExclude := Model{
		namespace:          "default",
		selectedNamespaces: sel,
		nsSelectionNegated: true,
	}

	fpInclude := mInclude.fetchFingerprint()
	fpExclude := mExclude.fetchFingerprint()

	assert.NotEqual(t, fpInclude, fpExclude,
		"include-scope and exclude-scope over the same set must produce different fingerprints")
}

func TestFetchFingerprintNegationContainsBang(t *testing.T) {
	m := Model{
		namespace:          "default",
		selectedNamespaces: map[string]bool{"kube-system": true},
		nsSelectionNegated: true,
	}
	fp := m.fetchFingerprint()
	assert.True(t, strings.Contains(fp, "!"),
		"negated fingerprint must contain '!' prefix: got %q", fp)
}

// --- namespace selector "tab" key (toggles exclude mode) ---

func TestNamespaceOverlayTabTogglesNegation(t *testing.T) {
	m := newNamespaceOverlayModel()
	m.selectedNamespaces = map[string]bool{"kube-system": true}
	m.nsSelectionNegated = false

	ret, _ := m.handleNamespaceOverlayKey(keyMsg("tab"))
	result := ret.(Model)

	assert.True(t, result.nsSelectionNegated, "tab must set nsSelectionNegated to true")
	assert.True(t, result.nsSelectionModified, "tab must set nsSelectionModified")
}

func TestNamespaceOverlayTabTogglesOff(t *testing.T) {
	m := newNamespaceOverlayModel()
	m.selectedNamespaces = map[string]bool{"kube-system": true}
	m.nsSelectionNegated = true

	ret, _ := m.handleNamespaceOverlayKey(keyMsg("tab"))
	result := ret.(Model)

	assert.False(t, result.nsSelectionNegated, "tab must toggle nsSelectionNegated off")
	assert.True(t, result.nsSelectionModified, "tab must set nsSelectionModified")
}

func TestNamespaceOverlayTabWithEmptySetIsHarmless(t *testing.T) {
	m := newNamespaceOverlayModel()
	m.selectedNamespaces = nil
	m.nsSelectionNegated = false

	ret, _ := m.handleNamespaceOverlayKey(keyMsg("tab"))
	result := ret.(Model)

	assert.True(t, result.nsSelectionNegated, "tab on empty set must still flip the flag")
}

func TestNamespaceOverlayAllNamespacesClearsNegation(t *testing.T) {
	m := newNamespaceOverlayModel()
	m.selectedNamespaces = map[string]bool{"kube-system": true}
	m.nsSelectionNegated = true

	ret, _ := m.handleNamespaceOverlayKey(keyMsg(ui.ActiveKeybindings.AllNamespaces))
	result := ret.(Model)

	assert.False(t, result.nsSelectionNegated, "'A' must clear nsSelectionNegated")
	assert.True(t, result.allNamespaces, "'A' must set allNamespaces")
}

func TestNamespaceOverlayApplySingleNamespaceClearsNegation(t *testing.T) {
	items := []model.Item{
		{Name: "All Namespaces", Status: "all"},
		{Name: "default"},
		{Name: "kube-system"},
	}
	m := newNamespaceOverlayModel()
	m.overlayItems = items
	m.overlayCursor = 1 // cursor on "default"
	m.nsSelectionNegated = true
	m.nsSelectionModified = false // no Space pressed

	ret, _ := m.handleNamespaceOverlayKey(specialKey(tea.KeyEnter))
	result := ret.(Model)

	assert.False(t, result.nsSelectionNegated,
		"applying single namespace via cursor must clear nsSelectionNegated")
}

// --- scope label rendering ---

func TestNsLabelNegatedRendersExclPrefix(t *testing.T) {
	m := Model{
		width:              80,
		namespace:          "default",
		selectedNamespaces: map[string]bool{"kube-system": true},
		nsSelectionNegated: true,
	}
	// Build the label the same way view.go does — exercise the logic
	// via the actual view function rather than re-implementing it here.
	label := m.buildNsLabelText()
	assert.True(t, strings.Contains(label, "!kube-system"),
		"negated single ns label must contain '!kube-system', got: %q", label)
}

func TestNsLabelNegatedMultipleNs(t *testing.T) {
	m := Model{
		width:     80,
		namespace: "default",
		selectedNamespaces: map[string]bool{
			"kube-system": true,
			"monitoring":  true,
		},
		nsSelectionNegated: true,
	}
	label := m.buildNsLabelText()
	assert.True(t, strings.Contains(label, "!"),
		"negated multi-ns label must contain '!' prefixes, got: %q", label)
}

func TestNsLabelNegatedOverflowHasExcludeHint(t *testing.T) {
	m := Model{
		width:     80,
		namespace: "default",
		selectedNamespaces: map[string]bool{
			"ns-a": true, "ns-b": true, "ns-c": true, "ns-d": true, "ns-e": true,
		},
		nsSelectionNegated: true,
	}
	label := m.buildNsLabelText()
	assert.Contains(t, label, "+2 more excl",
		"negated overflow label must signal the count refers to excluded namespaces, got: %q", label)
}

func TestNsLabelNonNegatedOverflowNoExcludeHint(t *testing.T) {
	m := Model{
		width:     80,
		namespace: "default",
		selectedNamespaces: map[string]bool{
			"ns-a": true, "ns-b": true, "ns-c": true, "ns-d": true, "ns-e": true,
		},
		nsSelectionNegated: false,
	}
	label := m.buildNsLabelText()
	assert.Contains(t, label, "+2 more")
	assert.NotContains(t, label, "excl")
}

func TestNsLabelNotNegated(t *testing.T) {
	m := Model{
		width:              80,
		namespace:          "kube-system",
		selectedNamespaces: map[string]bool{"kube-system": true},
		nsSelectionNegated: false,
	}
	label := m.buildNsLabelText()
	assert.False(t, strings.Contains(label, "!"),
		"non-negated label must not contain '!', got: %q", label)
}

// --- session round-trip ---

func TestSessionRoundTripPreservesNegation(t *testing.T) {
	st := &SessionTab{
		Context:            "prod",
		Namespace:          "default",
		AllNamespaces:      false,
		SelectedNamespaces: []string{"kube-system"},
		NsSelectionNegated: true,
	}
	tab := buildSessionTabState(st, nil)
	assert.True(t, tab.nsSelectionNegated,
		"buildSessionTabState must preserve NsSelectionNegated")
}

func TestSessionRoundTripNegationFalseDefault(t *testing.T) {
	st := &SessionTab{
		Context:       "prod",
		Namespace:     "default",
		AllNamespaces: false,
	}
	tab := buildSessionTabState(st, nil)
	assert.False(t, tab.nsSelectionNegated,
		"NsSelectionNegated must default to false when not set")
}

// --- bookmark round-trip ---

func TestBookmarkRoundTripPreservesNegation(t *testing.T) {
	tests := []struct {
		name           string
		bookmark       model.Bookmark
		initialNegated bool
		expectNegated  bool
		expectSelected []string
		expectAllNs    bool
	}{
		{
			name: "negated single namespace restores flag and set",
			bookmark: model.Bookmark{
				Namespace:          "kube-system",
				Namespaces:         []string{"kube-system"},
				NsSelectionNegated: true,
			},
			initialNegated: false,
			expectNegated:  true,
			expectSelected: []string{"kube-system"},
		},
		{
			name: "negated multiple namespaces restores flag and set",
			bookmark: model.Bookmark{
				Namespace:          "kube-system",
				Namespaces:         []string{"kube-system", "monitoring"},
				NsSelectionNegated: true,
			},
			initialNegated: false,
			expectNegated:  true,
			expectSelected: []string{"kube-system", "monitoring"},
		},
		{
			name: "non-negated bookmark resets stale true flag to false",
			bookmark: model.Bookmark{
				Namespace:          "production",
				Namespaces:         []string{"production"},
				NsSelectionNegated: false,
			},
			initialNegated: true,
			expectNegated:  false,
			expectSelected: []string{"production"},
		},
		{
			name: "all-namespaces bookmark resets stale true flag to false",
			bookmark: model.Bookmark{
				Namespace:          "",
				Namespaces:         nil,
				NsSelectionNegated: false,
			},
			initialNegated: true,
			expectNegated:  false,
			expectAllNs:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				nsSelectionNegated:    tt.initialNegated,
				bookmarkLoadNamespace: true,
			}
			target := bookmarkTarget{kind: bookmarkTargetContext, context: "ctx", lookupContext: "ctx"}
			m.applyBookmarkNamespace(tt.bookmark, target, "ctx")

			assert.Equal(t, tt.expectNegated, m.nsSelectionNegated)
			if tt.expectAllNs {
				assert.True(t, m.allNamespaces)
				assert.Nil(t, m.selectedNamespaces)
				return
			}
			assert.False(t, m.allNamespaces)
			for _, ns := range tt.expectSelected {
				assert.True(t, m.selectedNamespaces[ns], "expected %q selected", ns)
			}
			assert.Len(t, m.selectedNamespaces, len(tt.expectSelected))
		})
	}
}

// --- Space-deselect-to-empty clears negation ---

func TestNamespaceOverlaySpaceDeselectToEmptyClearsNegation(t *testing.T) {
	m := newNamespaceOverlayModel()
	m.selectedNamespaces = map[string]bool{"kube-system": true}
	m.allNamespaces = false
	m.nsSelectionNegated = true
	m.overlayCursor = 2 // cursor on "kube-system"

	ret, _ := m.handleNamespaceOverlayKey(keyMsg(" "))
	result := ret.(Model)

	assert.True(t, result.allNamespaces,
		"deselecting last namespace must set allNamespaces")
	assert.False(t, result.nsSelectionNegated,
		"deselecting to empty must clear nsSelectionNegated")
}

// --- Enter commits a built exclude set (negation preserved) ---

func TestNamespaceOverlayEnterCommitsExcludeSet(t *testing.T) {
	m := newNamespaceOverlayModel()
	m.overlayCursor = 2 // cursor on "kube-system"

	// Space selects, Tab negates, Enter commits.
	ret, _ := m.handleNamespaceOverlayKey(keyMsg(" "))
	m = ret.(Model)
	ret, _ = m.handleNamespaceOverlayKey(keyMsg("tab"))
	m = ret.(Model)
	ret, _ = m.handleNamespaceOverlayKey(specialKey(tea.KeyEnter))
	result := ret.(Model)

	assert.True(t, result.nsSelectionNegated,
		"Enter on a built exclude set must preserve negation")
	assert.True(t, result.selectedNamespaces["kube-system"],
		"Enter must commit the selected namespace")
	assert.False(t, result.allNamespaces)
}

// --- committing a single filtered namespace clears a restored exclude scope ---

func TestNamespaceOverlayFilterAcceptSingleResultClearsNegation(t *testing.T) {
	m := newNamespaceOverlayModel()
	// Exclude scope restored from a session/bookmark: negation is on, but
	// the user has not toggled anything in this overlay session yet.
	m.nsSelectionNegated = true
	m.nsSelectionModified = false
	m.nsFilterMode = true
	m.overlayFilter.Value = "kube-system"

	// First Enter accepts the filter and keeps the overlay open; the second
	// Enter (normal mode) commits the single cursored namespace.
	ret, _ := m.handleNamespaceOverlayKey(specialKey(tea.KeyEnter))
	m = ret.(Model)
	ret, _ = m.handleNamespaceOverlayKey(specialKey(tea.KeyEnter))
	result := ret.(Model)

	assert.False(t, result.nsSelectionNegated,
		"committing a single filtered namespace must clear the restored negation")
	assert.True(t, result.selectedNamespaces["kube-system"],
		"the filtered namespace must be committed as an include selection")
	assert.False(t, result.allNamespaces)
}

// --- helpers ---

func newNamespaceOverlayModel() Model {
	return Model{
		overlay: overlayNamespace,
		overlayItems: []model.Item{
			{Name: "All Namespaces", Status: "all"},
			{Name: "default"},
			{Name: "kube-system"},
		},
		overlayCursor: 0,
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
}
