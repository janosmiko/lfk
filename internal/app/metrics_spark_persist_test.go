package app

import (
	"testing"
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The hotkey is the only writer of the mode, so it is the only place that can
// record it. Without this the choice dies with the process.
func TestHandleMetricsSparkCycle_PersistsTheChosenWindow(t *testing.T) {
	isolateMetricsSparkPref(t)
	ui.ConfigSparklineWindows = []time.Duration{5 * time.Minute, 15 * time.Minute}

	m := basePush80Model()
	m, _ = m.handleMetricsSparkCycle()
	require.Equal(t, ui.MetricsDisplaySpark, m.metricsSpark.Mode)
	chosen := m.metricsSpark.Window()

	ui.MetricsSparkStartupState = ui.MetricsSparkState{}
	ApplyViewerPrefs()

	assert.Equal(t, chosen, ui.MetricsSparkStartupState.Window(),
		"the window chosen by the keypress must be what the next start restores")
}

// Cycling back to numeric has to stick, or the next start resurrects the
// sparkline the user just turned off.
func TestHandleMetricsSparkCycle_PersistsTheReturnToNumeric(t *testing.T) {
	isolateMetricsSparkPref(t)
	ui.ConfigSparklineWindows = []time.Duration{5 * time.Minute}

	m := basePush80Model()
	m, _ = m.handleMetricsSparkCycle() // numeric -> 5m
	m, _ = m.handleMetricsSparkCycle() // 5m -> numeric
	require.Equal(t, ui.MetricsDisplayNumeric, m.metricsSpark.Mode)

	ui.MetricsSparkStartupState = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}
	ApplyViewerPrefs()

	assert.Equal(t, ui.MetricsDisplayNumeric, ui.MetricsSparkStartupState.Mode)
}

// NewModel must open with the restored mode, otherwise the value is persisted
// and read back but never reaches the screen.
func TestNewModel_SeedsTheDisplayModeFromTheStartupState(t *testing.T) {
	prev := ui.MetricsSparkStartupState
	prevWindows := ui.ConfigSparklineWindows
	t.Cleanup(func() {
		ui.MetricsSparkStartupState = prev
		ui.ConfigSparklineWindows = prevWindows
	})
	ui.ConfigSparklineWindows = []time.Duration{5 * time.Minute, 15 * time.Minute}
	ui.MetricsSparkStartupState = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark, WindowIdx: 1}

	m := NewModel(k8s.NewTestClient(nil, nil), StartupOptions{})

	assert.Equal(t, ui.MetricsDisplaySpark, m.metricsSpark.Mode)
	assert.Equal(t, 15*time.Minute, m.metricsSpark.Window())
}
