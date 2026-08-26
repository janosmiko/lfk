package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetSparklineConfigForTest restores the sparkline globals to their
// defaults. Tests mutate package-level config, so each one cleans up.
func resetSparklineConfigForTest() {
	ConfigSparklineWindows = DefaultSparklineWindows
	ConfigSparklineWidth = DefaultSparklineWidth
	ConfigSparklineInterval = DefaultSparklineInterval
}

func TestMetricsSparkState_CyclesThroughWindowsAndBackToNumeric(t *testing.T) {
	t.Cleanup(resetSparklineConfigForTest)
	ConfigSparklineWindows = []time.Duration{5 * time.Minute, 15 * time.Minute, time.Hour}

	s := MetricsSparkState{}
	require.Equal(t, MetricsDisplayNumeric, s.Mode)

	s = s.Next()
	assert.Equal(t, MetricsDisplaySpark, s.Mode)
	assert.Equal(t, 5*time.Minute, s.Window())

	s = s.Next()
	assert.Equal(t, 15*time.Minute, s.Window())

	s = s.Next()
	assert.Equal(t, time.Hour, s.Window())

	s = s.Next()
	assert.Equal(t, MetricsDisplayNumeric, s.Mode, "the cycle returns to numeric")
	assert.Zero(t, s.Window())
}

// The cycle length follows the configured list, so two windows give a
// three-step cycle rather than always four steps.
func TestMetricsSparkState_CycleFollowsConfiguredWindowCount(t *testing.T) {
	t.Cleanup(resetSparklineConfigForTest)
	ConfigSparklineWindows = []time.Duration{time.Minute, 2 * time.Minute}

	s := MetricsSparkState{}.Next().Next()
	assert.Equal(t, 2*time.Minute, s.Window())
	assert.Equal(t, MetricsDisplayNumeric, s.Next().Mode)
}

// An empty window list must not produce a mode with no window to query.
func TestMetricsSparkState_NoWindowsStaysNumeric(t *testing.T) {
	t.Cleanup(resetSparklineConfigForTest)
	ConfigSparklineWindows = nil

	assert.Equal(t, MetricsDisplayNumeric, MetricsSparkState{}.Next().Mode)
}

func TestMetricsSparkState_Label(t *testing.T) {
	t.Cleanup(resetSparklineConfigForTest)
	ConfigSparklineWindows = []time.Duration{5 * time.Minute, time.Hour}

	assert.Equal(t, "numeric", MetricsSparkState{}.Label())
	assert.Equal(t, "sparkline 5m", MetricsSparkState{}.Next().Label())
	assert.Equal(t, "sparkline 1h", MetricsSparkState{}.Next().Next().Label())
}

// A stale index, for example from a session restored after the window list
// shrank, must not panic or read out of range.
func TestMetricsSparkState_OutOfRangeIndexIsNumeric(t *testing.T) {
	t.Cleanup(resetSparklineConfigForTest)
	ConfigSparklineWindows = []time.Duration{time.Minute}

	s := MetricsSparkState{Mode: MetricsDisplaySpark, WindowIdx: 7}
	assert.Zero(t, s.Window())
	assert.Equal(t, "numeric", s.Label())
}

func TestResolveMetricsSparkState_EmptyStringIsNumeric(t *testing.T) {
	t.Cleanup(resetSparklineConfigForTest)
	ConfigSparklineWindows = []time.Duration{5 * time.Minute}

	assert.Equal(t, MetricsSparkState{}, ResolveMetricsSparkState(""))
}

func TestResolveMetricsSparkState_MatchesConfiguredWindow(t *testing.T) {
	t.Cleanup(resetSparklineConfigForTest)
	ConfigSparklineWindows = []time.Duration{5 * time.Minute, 15 * time.Minute, time.Hour}

	got := ResolveMetricsSparkState((15 * time.Minute).String())
	assert.Equal(t, MetricsDisplaySpark, got.Mode)
	assert.Equal(t, 15*time.Minute, got.Window())
}

// A persisted window that is no longer in the configured list must not
// silently select a neighbour.
func TestResolveMetricsSparkState_UnmatchedWindowFallsBackToNumeric(t *testing.T) {
	t.Cleanup(resetSparklineConfigForTest)
	ConfigSparklineWindows = []time.Duration{5 * time.Minute}

	got := ResolveMetricsSparkState((15 * time.Minute).String())
	assert.Equal(t, MetricsDisplayNumeric, got.Mode)
}

func TestResolveMetricsSparkState_UnparseableFallsBackToNumeric(t *testing.T) {
	t.Cleanup(resetSparklineConfigForTest)
	ConfigSparklineWindows = []time.Duration{5 * time.Minute}

	got := ResolveMetricsSparkState("not-a-duration")
	assert.Equal(t, MetricsDisplayNumeric, got.Mode)
}

func TestClampSparklineWidth(t *testing.T) {
	assert.Equal(t, MinSparklineWidth, ClampSparklineWidth(0))
	assert.Equal(t, MinSparklineWidth, ClampSparklineWidth(-5))
	assert.Equal(t, MaxSparklineWidth, ClampSparklineWidth(999))
	assert.Equal(t, 12, ClampSparklineWidth(12))
}

func TestClampSparklineInterval(t *testing.T) {
	assert.Equal(t, MinSparklineInterval, ClampSparklineInterval(time.Second))
	assert.Equal(t, MaxWatchInterval, ClampSparklineInterval(24*time.Hour))
	assert.Equal(t, 30*time.Second, ClampSparklineInterval(30*time.Second))
	assert.Equal(t, time.Duration(0), ClampSparklineInterval(0), "non-positive disables the throttle")
}

func TestApplySparklineConfig_Defaults(t *testing.T) {
	t.Cleanup(resetSparklineConfigForTest)
	ConfigSparklineWindows = nil
	ConfigSparklineWidth = 0
	ConfigSparklineInterval = 0

	applySparklineConfig(nil, nil, "")

	assert.Equal(t, DefaultSparklineWindows, ConfigSparklineWindows)
	assert.Equal(t, DefaultSparklineWidth, ConfigSparklineWidth)
	assert.Equal(t, DefaultSparklineInterval, ConfigSparklineInterval)
}

func TestApplySparklineConfig_ParsesAndClamps(t *testing.T) {
	t.Cleanup(resetSparklineConfigForTest)
	width := 999

	applySparklineConfig([]string{"2m", "30m"}, &width, "1s")

	assert.Equal(t, []time.Duration{2 * time.Minute, 30 * time.Minute}, ConfigSparklineWindows)
	assert.Equal(t, MaxSparklineWidth, ConfigSparklineWidth)
	assert.Equal(t, MinSparklineInterval, ConfigSparklineInterval)
}

// An unparseable entry is dropped rather than failing the whole list, so one
// typo does not silently disable the feature.
func TestApplySparklineConfig_DropsUnparseableWindows(t *testing.T) {
	t.Cleanup(resetSparklineConfigForTest)

	applySparklineConfig([]string{"5m", "not-a-duration", "1h"}, nil, "")

	assert.Equal(t, []time.Duration{5 * time.Minute, time.Hour}, ConfigSparklineWindows)
}

// Every entry unparseable leaves the defaults in place rather than an empty
// list, which would make the hotkey do nothing.
func TestApplySparklineConfig_AllUnparseableFallsBackToDefaults(t *testing.T) {
	t.Cleanup(resetSparklineConfigForTest)

	applySparklineConfig([]string{"nope", "also-nope"}, nil, "")

	assert.Equal(t, DefaultSparklineWindows, ConfigSparklineWindows)
}
