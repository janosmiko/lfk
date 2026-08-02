package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestNsScopeEqual(t *testing.T) {
	base := nsScope{namespace: "a", selectedNamespaces: map[string]bool{"a": true}}
	tests := []struct {
		name string
		a, b nsScope
		want bool
	}{
		{"identical single", base, nsScope{namespace: "a", selectedNamespaces: map[string]bool{"a": true}}, true},
		{"different namespace", base, nsScope{namespace: "b", selectedNamespaces: map[string]bool{"a": true}}, false},
		{"different set size", base, nsScope{namespace: "a", selectedNamespaces: map[string]bool{"a": true, "b": true}}, false},
		{"different set member", base, nsScope{namespace: "a", selectedNamespaces: map[string]bool{"b": true}}, false},
		{"all-ns differs", nsScope{allNamespaces: true}, nsScope{allNamespaces: false}, false},
		{"negated differs", nsScope{nsSelectionNegated: true}, nsScope{nsSelectionNegated: false}, false},
		{"both empty", nsScope{}, nsScope{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.a.equal(tt.b))
		})
	}
}

func TestNsScopeCloneIsDeep(t *testing.T) {
	orig := &nsScope{namespace: "a", selectedNamespaces: map[string]bool{"a": true}}
	c := orig.clone()
	c.selectedNamespaces["b"] = true
	assert.NotContains(t, orig.selectedNamespaces, "b", "clone must not alias the source map")
	assert.Nil(t, (*nsScope)(nil).clone(), "nil clone stays nil")
}

func TestRecordPreviousNamespace_OnlyOnChange(t *testing.T) {
	m := newTestModel()
	m.namespace = "default"
	m.selectedNamespaces = map[string]bool{"default": true}

	// No change: previous stays nil.
	before := m.captureNamespaceScope()
	m.recordPreviousNamespace(before)
	assert.Nil(t, m.previousNsScope, "unchanged scope must not record a previous")

	// Change: previous captures the old scope.
	before = m.captureNamespaceScope()
	m.namespace = "kube-system"
	m.selectedNamespaces = map[string]bool{"kube-system": true}
	m.recordPreviousNamespace(before)
	if assert.NotNil(t, m.previousNsScope) {
		assert.Equal(t, "default", m.previousNsScope.namespace)
	}
}

func TestJumpToPreviousNamespace_Swaps(t *testing.T) {
	m := newTestModel()
	m.nav.Level = model.LevelResources
	m.namespace = "kube-system"
	m.selectedNamespaces = map[string]bool{"kube-system": true}
	m.previousNsScope = &nsScope{namespace: "default", selectedNamespaces: map[string]bool{"default": true}}

	out, _ := m.jumpToPreviousNamespace()
	got := out.(Model)
	assert.Equal(t, "default", got.namespace, "jump lands on the previous namespace")
	assert.Equal(t, map[string]bool{"default": true}, got.selectedNamespaces)
	// The scope we left becomes the new previous, so a second jump returns.
	if assert.NotNil(t, got.previousNsScope) {
		assert.Equal(t, "kube-system", got.previousNsScope.namespace)
	}

	out2, _ := got.jumpToPreviousNamespace()
	got2 := out2.(Model)
	assert.Equal(t, "kube-system", got2.namespace, "second jump swaps back")
}

func TestJumpToPreviousNamespace_NoPrevious(t *testing.T) {
	m := newTestModel()
	m.nav.Level = model.LevelResources
	m.namespace = "default"
	m.previousNsScope = nil

	out, _ := m.jumpToPreviousNamespace()
	got := out.(Model)
	assert.Equal(t, "default", got.namespace, "no-op when nothing recorded")
	assert.True(t, got.statusMessageErr, "reports an error message")
}

// TestNamespaceOverlayEntryScope_FallsBackToCurrent verifies the helper
// returns the live scope when no overlay snapshot has been taken.
func TestNamespaceOverlayEntryScope_FallsBackToCurrent(t *testing.T) {
	m := newTestModel()
	m.namespace = "default"
	m.nsOverlayEntryScope = nil
	assert.Equal(t, "default", m.namespaceOverlayEntryScope().namespace)

	m.nsOverlayEntryScope = &nsScope{namespace: "kube-system"}
	assert.Equal(t, "kube-system", m.namespaceOverlayEntryScope().namespace)
}

// TestOverlayCommit_RecordsPreOverlayScopeAfterSpaceEdit is the regression
// guard for the CodeRabbit finding: Space mutates the live selection during
// the overlay session, so the "previous" scope must come from the snapshot
// taken when the overlay opened — not a capture at Enter time (which would
// see the already-edited state and record nothing).
func TestOverlayCommit_RecordsPreOverlayScopeAfterSpaceEdit(t *testing.T) {
	m := newTestModel()
	m.overlay = overlayNamespace
	m.namespace = "default"
	m.selectedNamespaces = map[string]bool{"default": true}
	// Snapshot taken on open (pre-edit).
	m.nsOverlayEntryScope = &nsScope{namespace: "default", selectedNamespaces: map[string]bool{"default": true}}
	// Simulate a Space toggle having already mutated the live selection.
	m.nsSelectionModified = true
	m.selectedNamespaces = map[string]bool{"kube-system": true}

	out, _ := m.handleNamespaceOverlayKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	rm := out.(Model)

	if assert.NotNil(t, rm.previousNsScope, "commit after a Space edit must record the pre-overlay scope") {
		assert.Equal(t, "default", rm.previousNsScope.namespace)
		assert.Equal(t, map[string]bool{"default": true}, rm.previousNsScope.selectedNamespaces)
	}
	assert.Equal(t, "kube-system", rm.namespace, "commit still applies the edited selection")
}

func TestJumpToPreviousNamespace_BlockedAtClusters(t *testing.T) {
	m := newTestModel()
	m.nav.Level = model.LevelClusters
	m.previousNsScope = &nsScope{namespace: "default"}

	out, _ := m.jumpToPreviousNamespace()
	got := out.(Model)
	assert.Equal(t, "", got.namespace, "namespace unchanged at cluster level")
	assert.NotNil(t, got.previousNsScope, "previous scope preserved")
}
