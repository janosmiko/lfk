package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCovBulkRemoveFinalizer(t *testing.T) {
	m := baseModelWithFakeClient()
	m.finalizerSearch.results = []k8s.FinalizerMatch{
		{Namespace: "default", Kind: "Pod", Name: "pod-1", Matched: "test/finalizer"},
		{Namespace: "default", Kind: "Pod", Name: "pod-2", Matched: "test/finalizer"},
	}
	m.finalizerSearch.selected = map[string]bool{
		"default/Pod/pod-1": true,
	}
	cmd := m.bulkRemoveFinalizer()
	msg := execCmd(t, cmd)
	result, ok := msg.(finalizerRemoveResultMsg)
	require.True(t, ok)
	// Fake client doesn't have these resources, so removal fails.
	assert.Equal(t, 1, result.failed)
	assert.Equal(t, 0, result.succeeded)
}

func TestCovBulkRemoveFinalizerNoneSelected(t *testing.T) {
	m := baseModelWithFakeClient()
	m.finalizerSearch.results = []k8s.FinalizerMatch{
		{Namespace: "default", Kind: "Pod", Name: "pod-1", Matched: "test/finalizer"},
	}
	m.finalizerSearch.selected = map[string]bool{}
	cmd := m.bulkRemoveFinalizer()
	msg := execCmd(t, cmd)
	result, ok := msg.(finalizerRemoveResultMsg)
	require.True(t, ok)
	assert.Equal(t, 0, result.failed)
	assert.Equal(t, 0, result.succeeded)
}

func TestCovSearchFinalizers(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true}
	cmd := m.searchFinalizers("test/finalizer")
	msg := execCmd(t, cmd)
	result, ok := msg.(finalizerSearchResultMsg)
	require.True(t, ok)
	// No resources with finalizers in fake client.
	assert.NoError(t, result.err)
}

func TestFinalizerMatcher_HonorsSearchMode(t *testing.T) {
	orig := ui.ConfigDefaultSearchMode
	t.Cleanup(func() { ui.ConfigDefaultSearchMode = orig })

	tests := []struct {
		name    string
		mode    string
		pattern string
		want    bool
	}{
		{"default plain substring", ui.DefaultSearchModeDefault, "clean", true},
		{"default plain no match", ui.DefaultSearchModeDefault, "xyz", false},
		{"default regex auto-detect", ui.DefaultSearchModeDefault, `clean.*`, true},
		{"fuzzy default typo tolerant", ui.DefaultSearchModeFuzzy, "cleanp", true},
		{"regex default anchors", ui.DefaultSearchModeRegex, `^my\.finalizer\.io/cleanup$`, true},
		{"literal prefix escapes regex meta", ui.DefaultSearchModeRegex, `\my.finalizer`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ui.ConfigDefaultSearchMode = tt.mode
			got := finalizerMatcher(tt.pattern)("my.finalizer.io/cleanup")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCovSearchFinalizersAllTypes(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelResourceTypes
	cmd := m.searchFinalizers("test/*")
	// Just verify cmd is returned; executing it would panic because the
	// fake dynamic client does not have all resource list kinds registered.
	assert.NotNil(t, cmd)
}
