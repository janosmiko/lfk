package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// Regression for the bug where switching from a Pods tab (with metrics
// rendered at the bottom of the right pane) to a Services tab left
// the Pods metrics rendered. Root cause: m.metricsContent and
// m.previewEventsContent were Model-level fields, so saveCurrentTab /
// loadTab didn't preserve them per-tab — the next tab inherited
// whatever the previous tab had set, and Service has no metrics
// loader to overwrite it.
//
// Fix: both fields now live on TabState. This test pins the per-tab
// round-trip so a future refactor can't silently drop them again.
func TestSaveAndLoadTab_PreservesMetricsAndEventsFootersPerTab(t *testing.T) {
	m := Model{
		tabs: []TabState{
			{nav: model.NavigationState{Context: "kctx"}}, // tab 0: pods
			{nav: model.NavigationState{Context: "kctx"}}, // tab 1: services
		},
		activeTab: 0,
		// Tab 0 (Pods) has rendered metrics + an events footer.
		metricsContent:       "POD METRICS BAR",
		previewEventsContent: "warning: ImagePullBackoff",
	}

	// User switches from tab 0 to tab 1.
	m.saveCurrentTab()
	require.Equal(t, "POD METRICS BAR", m.tabs[0].metricsContent,
		"metricsContent must persist into TabState so it survives tab switches")
	require.Equal(t, "warning: ImagePullBackoff", m.tabs[0].previewEventsContent,
		"previewEventsContent likewise")

	cmd := m.loadTab(1)
	_ = cmd // load may fire a refresh; we only care about state here.

	assert.Empty(t, m.metricsContent,
		"loadTab must not leak the previous tab's metricsContent into the new tab — Services has no metrics loader to clear it")
	assert.Empty(t, m.previewEventsContent,
		"same per-tab guarantee for the events footer")

	// Switch back to tab 0 → Pods footers must come back from the cache
	// so the user sees the values they had before the round-trip,
	// pending the next watch tick refresh.
	m.saveCurrentTab()
	m.loadTab(0)
	assert.Equal(t, "POD METRICS BAR", m.metricsContent,
		"returning to a tab restores its previously-rendered metricsContent")
	assert.Equal(t, "warning: ImagePullBackoff", m.previewEventsContent)
}
