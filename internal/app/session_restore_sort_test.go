package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

// TestLoadTab_RestoredLazyTabAppliesRememberedSort covers issue #913: with one
// tab per cluster, a restart restored the remembered sort only on the active
// tab. The other tabs are built by buildSessionTabState with needsLoad set and
// no sort fields, so the first loadTab copied an empty sort onto the model.
func TestLoadTab_RestoredLazyTabAppliesRememberedSort(t *testing.T) {
	orig := ui.ConfigViews
	t.Cleanup(func() { ui.ConfigViews = orig })
	ui.ConfigViews = nil

	t.Setenv("LFK_STATE_DIR", t.TempDir())
	pods := ui.ResourceRef{Version: "v1", Resource: "pods", Kind: "Pod"}
	require.NoError(t, saveSortMemory(map[string]sortPref{
		sortMemoryKey(pods, "cluster-1"): {column: "Age", ascending: true},
		sortMemoryKey(pods, "cluster-2"): {column: "Age", ascending: true},
	}))

	m := baseModelWithFakeClient()
	m.sortMemory = loadSortMemory()
	tabs := make([]TabState, 0, 2)
	for i, ctx := range []string{"cluster-1", "cluster-2"} {
		tab := buildSessionTabState(&SessionTab{Context: ctx, AllNamespaces: true, ResourceType: "/v1/pods"}, nil)
		require.Equal(t, "pods", tab.nav.ResourceType.Resource, "the session ref must resolve to pods")
		tab.needsLoad = i != 0
		tabs = append(tabs, tab)
	}
	m.tabs = tabs
	m.activeTab = 0

	_ = m.loadTab(1)

	assert.Equal(t, "Age", m.sortColumnName, "the lazy tab must restore its cluster's remembered sort column")
	assert.True(t, m.sortAscending, "the lazy tab must restore its cluster's remembered sort direction")
}

// TestLoadTab_RestoredLazyTabWithoutMemoryUsesDefaultSort keeps a restored tab
// with nothing remembered on the built-in default instead of an empty column.
func TestLoadTab_RestoredLazyTabWithoutMemoryUsesDefaultSort(t *testing.T) {
	orig := ui.ConfigViews
	t.Cleanup(func() { ui.ConfigViews = orig })
	ui.ConfigViews = nil

	t.Setenv("LFK_STATE_DIR", t.TempDir())

	m := baseModelWithFakeClient()
	first := buildSessionTabState(&SessionTab{Context: "cluster-1", AllNamespaces: true, ResourceType: "/v1/pods"}, nil)
	second := buildSessionTabState(&SessionTab{Context: "cluster-2", AllNamespaces: true, ResourceType: "/v1/pods"}, nil)
	second.needsLoad = true
	m.tabs = []TabState{first, second}
	m.activeTab = 0

	_ = m.loadTab(1)

	assert.Equal(t, sortColDefault, m.sortColumnName)
	assert.True(t, m.sortAscending)
}
