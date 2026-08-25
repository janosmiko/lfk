package app

import (
	"reflect"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestHandleMetricsSparkCycle_AdvancesAndReportsMode(t *testing.T) {
	prev := ui.ConfigSparklineWindows
	t.Cleanup(func() { ui.ConfigSparklineWindows = prev })
	ui.ConfigSparklineWindows = []time.Duration{5 * time.Minute, time.Hour}

	m := basePush80Model()
	require.Equal(t, ui.MetricsDisplayNumeric, m.metricsSpark.Mode)

	m, _ = m.handleMetricsSparkCycle()
	assert.Equal(t, ui.MetricsDisplaySpark, m.metricsSpark.Mode)
	assert.Equal(t, 5*time.Minute, m.metricsSpark.Window())
	assert.Contains(t, m.statusMessage, "sparkline 5m")

	m, _ = m.handleMetricsSparkCycle()
	assert.Equal(t, time.Hour, m.metricsSpark.Window())

	m, _ = m.handleMetricsSparkCycle()
	assert.Equal(t, ui.MetricsDisplayNumeric, m.metricsSpark.Mode)
	assert.Contains(t, m.statusMessage, "numeric")
}

// Leaving spark mode must repaint the cells right away rather than leave the
// old glyphs on screen until the next throttled tick, so the numeric branch
// must also fire the instant list loader (bypassing the fetch throttle).
func TestHandleMetricsSparkCycle_LeavingSparkRefetchesInstantly(t *testing.T) {
	prev := ui.ConfigSparklineWindows
	t.Cleanup(func() { ui.ConfigSparklineWindows = prev })
	ui.ConfigSparklineWindows = []time.Duration{5 * time.Minute}

	m := basePush80Model()
	m, _ = m.handleMetricsSparkCycle() // Numeric -> Spark
	require.Equal(t, ui.MetricsDisplaySpark, m.metricsSpark.Mode)

	m, cmd := m.handleMetricsSparkCycle() // Spark -> Numeric
	require.Equal(t, ui.MetricsDisplayNumeric, m.metricsSpark.Mode)
	require.NotNil(t, cmd)

	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	require.True(t, ok, "leaving spark mode must batch more than the status-clear cmd")
	assert.Len(t, batch, 2, "must include both the status-clear and the instant metrics repaint")
}

// The mode is per tab. A mode set in one tab must not follow the user into
// another, the way every other view-shaped piece of tab state behaves.
func TestMetricsSparkMode_DoesNotLeakBetweenTabs(t *testing.T) {
	prev := ui.ConfigSparklineWindows
	t.Cleanup(func() { ui.ConfigSparklineWindows = prev })
	ui.ConfigSparklineWindows = []time.Duration{5 * time.Minute}

	m := basePush80Model()
	m, _ = m.handleMetricsSparkCycle()
	require.Equal(t, ui.MetricsDisplaySpark, m.metricsSpark.Mode)

	m.saveCurrentTab()
	saved := m.tabs[m.activeTab].metricsSpark
	assert.Equal(t, ui.MetricsDisplaySpark, saved.Mode, "saveCurrentTab must persist the mode")

	clone := m.cloneCurrentTab()
	assert.Equal(t, ui.MetricsDisplaySpark, clone.metricsSpark.Mode, "a cloned tab inherits the mode")

	// A tab whose saved state is numeric must restore numeric, not inherit
	// the live model's sparkline mode.
	m.tabs[m.activeTab].metricsSpark = ui.MetricsSparkState{}
	m.loadTab(m.activeTab)
	assert.Equal(t, ui.MetricsDisplayNumeric, m.metricsSpark.Mode, "loadTab must restore the tab's own mode")
}

// allDefaultKeybindingValues mirrors the reflection walk in
// TestDefaultKeybindingsNoUnreachableControlAliases (internal/ui), which is
// unexported and lives in a different package.
func allDefaultKeybindingValues(kb ui.Keybindings) []string {
	v := reflect.ValueOf(kb)
	values := make([]string, 0, v.NumField())
	for _, f := range v.Fields() {
		if f.Kind() != reflect.String {
			continue
		}
		values = append(values, f.String())
	}
	return values
}

// A union dashboard member does not clear ResourceType like the plain
// dashboard does - it sets Kind to unionClusterDashboardKind instead - so the
// tilde key must recognise that shape too, or it goes silently inert there.
func TestDashboardMetricsKind_UnionClusterMember(t *testing.T) {
	m := basePush80Model()
	m.unionMode = true
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: unionClusterDashboardKind}

	assert.Equal(t, "Cluster", m.dashboardMetricsKind())
}

// The union monitoring dashboard shares the member-list shape but has no
// CPU/Mem section, so it must stay excluded the same way "__monitoring__" is
// excluded for the non-union dashboard.
func TestDashboardMetricsKind_UnionMonitoringMemberStaysExcluded(t *testing.T) {
	m := basePush80Model()
	m.unionMode = true
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: unionMonitoringDashboardKind}

	assert.NotEqual(t, "Cluster", m.dashboardMetricsKind())
}

// Navigating into containers only changes nav.Level, never nav.ResourceType,
// so dashboardMetricsKind must check the level itself or it stays "Pod".
func TestDashboardMetricsKind_ContainerLevel(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}

	assert.Equal(t, "Container", m.dashboardMetricsKind())
}

func TestMetricsSparkAvailable_ContainerLevel(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}

	assert.True(t, m.metricsSparkAvailable(), "the container level must advertise the key once it works there")
}

func TestMetricsSparkAvailable_KindWithNoColumnsStaysUnavailable(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment"}

	assert.False(t, m.metricsSparkAvailable())
}

// The tilde must stay free of every other default binding.
func TestMetricsSparkCycle_DefaultBindingIsUnique(t *testing.T) {
	kb := ui.DefaultKeybindings()
	assert.Equal(t, "~", kb.MetricsSparkCycle)

	seen := 0
	for _, key := range allDefaultKeybindingValues(kb) {
		if key == "~" {
			seen++
		}
	}
	assert.Equal(t, 1, seen, "the tilde is bound more than once")
}
