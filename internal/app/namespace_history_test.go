package app

import (
	"testing"

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

func TestJumpToPreviousNamespace_BlockedAtClusters(t *testing.T) {
	m := newTestModel()
	m.nav.Level = model.LevelClusters
	m.previousNsScope = &nsScope{namespace: "default"}

	out, _ := m.jumpToPreviousNamespace()
	got := out.(Model)
	assert.Equal(t, "", got.namespace, "namespace unchanged at cluster level")
	assert.NotNil(t, got.previousNsScope, "previous scope preserved")
}
