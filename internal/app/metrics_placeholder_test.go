package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Changing focus to a different pod must not leave the previous pod's resource
// bar on screen while the new metrics load. moveCursor swaps the stale bar for
// a loading placeholder so the footer reflects the new selection immediately.
func TestMoveCursorShowsMetricsPlaceholderForPod(t *testing.T) {
	m := basePush80Model() // Pod kind at LevelResources
	m.setCursor(0)
	m.metricsContent = "OLD POD BAR"
	m.metricsData = &metricsInputs{cpuUsed: 100, cpuLim: 200, memUsed: 100, memLim: 200}

	result, _ := m.moveCursor(1)
	rm := result.(Model)

	assert.True(t, rm.metricsLoading, "moveCursor must arm the metrics loading state for a Pod")
	assert.Nil(t, rm.metricsData, "moveCursor must drop the previous pod's metrics numbers")
	assert.NotEqual(t, "OLD POD BAR", rm.metricsContent, "moveCursor must not keep the previous pod's bar")
	assert.Contains(t, rm.metricsContent, "RESOURCE USAGE", "moveCursor must render the metrics placeholder")
}

// A non-metrics kind (ConfigMap) has no resource-usage footer, so changing
// focus must clear the bar entirely rather than show a placeholder.
func TestMoveCursorClearsMetricsForNonEligibleKind(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "ConfigMap", Resource: "configmaps", APIVersion: "v1", Namespaced: true}
	m.middleItems = []model.Item{
		{Name: "cm-1", Namespace: "default", Kind: "ConfigMap"},
		{Name: "cm-2", Namespace: "default", Kind: "ConfigMap"},
	}
	m.setCursor(0)
	m.metricsContent = "OLD POD BAR"
	m.metricsData = &metricsInputs{cpuUsed: 100, cpuLim: 200}

	result, _ := m.moveCursor(1)
	rm := result.(Model)

	assert.False(t, rm.metricsLoading, "ConfigMap must not arm the metrics loading state")
	assert.Nil(t, rm.metricsData)
	assert.Empty(t, rm.metricsContent, "ConfigMap must clear the metrics bar, not show a placeholder")
}

// When the real metrics arrive, the loading state is cleared and the bar shows
// actual numbers.
func TestMetricsLoadedClearsLoadingState(t *testing.T) {
	m := basePush80Model()
	m.metricsLoading = true
	m.metricsContent = "placeholder"

	out := m.updateMetricsLoaded(metricsLoadedMsg{
		cpuUsed: 50, cpuReq: 100, cpuLim: 200,
		memUsed: 50, memReq: 100, memLim: 200,
		gen: m.requestGen,
	})

	assert.False(t, out.metricsLoading, "real metrics must clear the loading state")
	require.NotNil(t, out.metricsData)
	assert.Contains(t, out.metricsContent, "RESOURCE USAGE")
}

// A no-metrics result (no metrics-server / no usage) clears both the bar and
// the loading state so the footer collapses instead of spinning forever.
func TestMetricsLoadedZeroClearsLoadingState(t *testing.T) {
	m := basePush80Model()
	m.metricsLoading = true
	m.metricsContent = "placeholder"

	out := m.updateMetricsLoaded(metricsLoadedMsg{gen: m.requestGen})

	assert.False(t, out.metricsLoading)
	assert.Nil(t, out.metricsData)
	assert.Empty(t, out.metricsContent)
}
