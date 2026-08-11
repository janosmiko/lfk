package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"

	"github.com/janosmiko/lfk/internal/tainted"
)

// themeA and themeB differ only in their Base background, which is enough for
// the recompose assertions: a stale (non-recomposed) string keeps themeA's
// "48;2;..." background code, a recomposed one carries themeB's.
func themeA() ui.Theme { t := ui.DefaultTheme(); t.Base = "#101010"; return t }
func themeB() ui.Theme { t := ui.DefaultTheme(); t.Base = "#fa00fa"; return t }

func withTrueColor(t *testing.T) {
	origTheme := ui.ActiveTheme
	origSchemeName := ui.ActiveSchemeName
	t.Cleanup(func() {
		ui.ApplyTheme(origTheme)
		ui.ActiveSchemeName = origSchemeName
	})
}

// TestRecomposeThemedContentRerendersCachedPreviews guards the theme-switch
// staleness fix: dashboard, monitoring, metrics, and events previews bake the
// active theme's colors into cached strings, so a theme switch must recompose
// them from retained raw data rather than wait for the next data tick.
func TestRecomposeThemedContentRerendersCachedPreviews(t *testing.T) {
	withTrueColor(t)
	ui.ApplyTheme(themeA())

	m := Model{
		width:  120,
		height: 40,
		metricsData: &metricsInputs{
			cpuUsed: 500, cpuReq: 1000, cpuLim: 2000,
			memUsed: 256, memReq: 512, memLim: 1024,
		},
		previewEventsData: []ui.EventTimelineEntry{
			{Timestamp: time.Time{}, Type: tainted.Wrap("Warning"), Reason: tainted.Wrap("BackOff"), Message: tainted.Wrap("boom"), InvolvedName: tainted.Wrap("p"), InvolvedKind: tainted.Wrap("Pod")},
		},
		monitoringData: map[string]monitoringData{
			"ctx": {alerts: []k8s.AlertInfo{{Name: "HighCPU", State: "firing", Severity: "critical"}}},
		},
		dashboardData: map[string]dashboardData{
			"ctx": {nodeCount: 3, readyNodes: 3, pods: podStats{total: 10, running: 10}},
		},
		nav: model.NavigationState{Context: "ctx"},
	}

	// Render everything under theme A.
	m = m.recomposeThemedContent()
	metricsA, eventsA, monA, dashA := m.metricsContent, m.previewEventsContent, m.monitoringPreview, m.dashboardPreview
	require.NotEmpty(t, metricsA)
	require.NotEmpty(t, eventsA)
	require.NotEmpty(t, monA)
	require.NotEmpty(t, dashA)

	// Switch theme and recompose.
	ui.ApplyTheme(themeB())
	m = m.recomposeThemedContent()

	assert.NotEqual(t, metricsA, m.metricsContent, "metrics bar must re-render under the new theme")
	assert.NotEqual(t, eventsA, m.previewEventsContent, "events footer must re-render under the new theme")
	assert.NotEqual(t, monA, m.monitoringPreview, "monitoring dashboard must re-render under the new theme")
	assert.NotEqual(t, dashA, m.dashboardPreview, "cluster dashboard must re-render under the new theme")
}

// TestColorschemeCommitRecomposesFooters guards the handler wiring: committing
// a scheme in the overlay must re-render the cached footers so the new theme
// applies immediately instead of on the next data tick.
func TestColorschemeCommitRecomposesFooters(t *testing.T) {
	withTrueColor(t)
	ui.ApplyTheme(ui.DefaultTheme())

	m := Model{
		overlay:            overlayColorscheme,
		schemeEntries:      []ui.SchemeEntry{{Name: "Dark", IsHeader: true}, {Name: "dracula"}},
		schemeCursor:       0,
		schemeOriginalName: "tokyonight",
		width:              120,
		height:             40,
		tabs:               []TabState{{}},
		metricsData: &metricsInputs{
			cpuUsed: 500, cpuReq: 1000, cpuLim: 2000,
			memUsed: 256, memReq: 512, memLim: 1024,
		},
		nav: model.NavigationState{Context: "ctx"},
	}
	m = m.recomposeMetrics()
	before := m.metricsContent
	require.NotEmpty(t, before)

	ret, _ := m.handleColorschemeNormalMode(specialKey(tea.KeyEnter))
	result := ret.(Model)

	assert.Equal(t, overlayNone, result.overlay, "overlay must close on commit")
	assert.NotEqual(t, before, result.metricsContent,
		"committing a new scheme must recompose the metrics footer with the new theme")
}

// TestColorschemeLivePreviewRecomposesFooters guards the live-preview path: the
// picker keeps the explorer un-dimmed for side-by-side comparison, so moving the
// cursor (j/k) must recompose the cached previews with the previewed theme, not
// wait for commit or the next tick.
func TestColorschemeLivePreviewRecomposesFooters(t *testing.T) {
	withTrueColor(t)
	ui.ApplyTheme(ui.DefaultTheme())

	m := Model{
		overlay: overlayColorscheme,
		// Cursor starts on the first scheme; "j" moves to the second (dracula),
		// which differs from the initial tokyonight default.
		schemeEntries:      []ui.SchemeEntry{{Name: "tokyonight"}, {Name: "dracula"}},
		schemeCursor:       0,
		schemeOriginalName: "tokyonight",
		width:              120,
		height:             40,
		tabs:               []TabState{{}},
		metricsData: &metricsInputs{
			cpuUsed: 500, cpuReq: 1000, cpuLim: 2000,
			memUsed: 256, memReq: 512, memLim: 1024,
		},
		nav: model.NavigationState{Context: "ctx"},
	}
	m = m.recomposeMetrics()
	before := m.metricsContent
	require.NotEmpty(t, before)

	ret, _ := m.handleColorschemeNormalMode(runeKey('j'))
	result := ret.(Model)

	assert.Equal(t, overlayColorscheme, result.overlay, "overlay stays open during preview")
	assert.NotEqual(t, before, result.metricsContent,
		"moving the cursor must recompose the metrics footer with the previewed theme")
}

// TestColorModeReportRecomposesFooters guards the auto dark/light path: when
// the terminal reports a color-mode flip, the configured scheme is applied
// outside the model, so the cached footers must be recomposed too — otherwise
// they keep the previous theme's baked colors until the next data tick.
func TestColorModeReportRecomposesFooters(t *testing.T) {
	withTrueColor(t)
	origDark := ui.ConfigDarkColorscheme
	origLight := ui.ConfigLightColorscheme
	t.Cleanup(func() {
		ui.ConfigDarkColorscheme = origDark
		ui.ConfigLightColorscheme = origLight
	})
	ui.ConfigDarkColorscheme = "dracula"
	ui.ConfigLightColorscheme = "rose-pine-dawn"
	ui.SetColorMode(true)

	m := Model{
		width:  120,
		height: 40,
		tabs:   []TabState{{}},
		metricsData: &metricsInputs{
			cpuUsed: 500, cpuReq: 1000, cpuLim: 2000,
			memUsed: 256, memReq: 512, memLim: 1024,
		},
		nav: model.NavigationState{Context: "ctx"},
	}
	m = m.recomposeMetrics()
	before := m.metricsContent
	require.NotEmpty(t, before)

	ret, _ := m.updateImpl(uv.LightColorSchemeEvent{})
	result := ret.(Model)

	assert.Equal(t, "rose-pine-dawn", ui.ActiveSchemeName, "light report must apply the light scheme")
	assert.NotEqual(t, before, result.metricsContent,
		"a color-mode flip must recompose the metrics footer with the new theme")
}

// TestRecomposeIsNoOpWithoutRetainedData ensures recompose leaves the footers
// empty (no panic, no spurious content) when nothing is loaded.
func TestRecomposeIsNoOpWithoutRetainedData(t *testing.T) {
	withTrueColor(t)
	ui.ApplyTheme(themeA())
	m := Model{width: 120, height: 40, nav: model.NavigationState{Context: "ctx"}}
	m = m.recomposeThemedContent()
	assert.Empty(t, m.metricsContent)
	assert.Empty(t, m.previewEventsContent)
	assert.Empty(t, m.monitoringPreview)
	assert.Empty(t, m.dashboardPreview)
}
