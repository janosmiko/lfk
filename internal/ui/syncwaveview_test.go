package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncWaveGlyph(t *testing.T) {
	tests := []struct {
		opStatus, syncStatus, healthStatus string
		want                               string
	}{
		{"Succeeded", "", "", "✓"},
		{"Synced", "", "", "✓"},
		{"", "Synced", "Healthy", "✓"},
		{"Running", "", "", "⟳"},
		{"", "", "Progressing", "⟳"},
		{"Failed", "", "", "✗"},
		{"Error", "", "", "✗"},
		{"", "", "Degraded", "✗"},
		{"", "Missing", "", "✗"},
		{"", "Synced", "", "○"}, // synced w/o health → idle dot
		{"", "", "", "○"},
	}
	for _, tt := range tests {
		got := SyncWaveGlyph(tt.opStatus, tt.syncStatus, tt.healthStatus)
		assert.Equal(t, tt.want, got, "op=%q sync=%q health=%q", tt.opStatus, tt.syncStatus, tt.healthStatus)
	}
}

func TestRenderSyncWave_HeaderShowsAppName(t *testing.T) {
	entry := SyncWaveTimelineEntry{AppName: "my-app", AppNamespace: "argocd"}
	got := RenderSyncWaveTimeline(entry, 100, 30)
	assert.Contains(t, got, "Sync Wave Timeline: my-app")
}

func TestRenderSyncWave_HeaderShowsLastOperation(t *testing.T) {
	entry := SyncWaveTimelineEntry{
		AppName: "my-app",
		LastOperation: &SyncWaveLastOperation{
			Phase:      "Succeeded",
			FinishedAt: time.Now().Add(-12 * time.Minute),
			Revision:   "8a3c4d1",
		},
	}
	got := RenderSyncWaveTimeline(entry, 100, 30)
	assert.Contains(t, got, "Last Sync: Succeeded")
	assert.Contains(t, got, "8a3c4d1")
	assert.Contains(t, got, "12m ago")
}

func TestRenderSyncWave_HeaderHidesLastOperationWhenNil(t *testing.T) {
	entry := SyncWaveTimelineEntry{AppName: "my-app"}
	got := RenderSyncWaveTimeline(entry, 100, 30)
	assert.NotContains(t, got, "Last Sync:")
}

func TestRenderSyncWave_HeaderShowsLivePhaseOnlyIfRunning(t *testing.T) {
	entry := SyncWaveTimelineEntry{AppName: "x", LivePhase: "Running"}
	got := RenderSyncWaveTimeline(entry, 100, 30)
	assert.Contains(t, got, "Live phase: Running")

	entry.LivePhase = "Succeeded"
	got = RenderSyncWaveTimeline(entry, 100, 30)
	assert.NotContains(t, got, "Live phase")
}

// Two-phase load: the skeleton paints the phase structure but the wave
// numbers haven't been fetched yet. The header surfaces this transient
// state with a spinner glyph + "Loading wave map…" line so the operator
// knows wave numbers are still arriving and the overlay isn't frozen.
func TestRenderSyncWave_LoadingShowsIndicator(t *testing.T) {
	entry := SyncWaveTimelineEntry{AppName: "x", Loading: true}
	got := RenderSyncWaveTimeline(entry, 100, 30)
	assert.Contains(t, got, "Loading wave map…",
		"Loading: true must surface a wave-map indicator in the header")

	// And the indicator must NOT be present once the full fetch lands.
	entry.Loading = false
	got = RenderSyncWaveTimeline(entry, 100, 30)
	assert.NotContains(t, got, "Loading wave map",
		"Loading: false must drop the wave-map indicator")
}

// The loading line must include one of the spinner glyphs so the
// operator gets visual motion confirming the overlay isn't frozen
// during the wave-annotation fan-out.
func TestRenderSyncWave_LoadingShowsSpinnerGlyph(t *testing.T) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for i, frame := range frames {
		entry := SyncWaveTimelineEntry{AppName: "x", Loading: true, LoadingFrame: i}
		got := RenderSyncWaveTimeline(entry, 100, 30)
		assert.Contains(t, got, frame+" Loading wave map…",
			"frame %d must render glyph %q before the loading text", i, frame)
	}
	// Modulo wrap so unbounded frame indices keep cycling.
	entry := SyncWaveTimelineEntry{AppName: "x", Loading: true, LoadingFrame: len(frames) + 3}
	got := RenderSyncWaveTimeline(entry, 100, 30)
	assert.Contains(t, got, frames[3]+" Loading wave map…",
		"frame index past the table length must wrap with modulo")
}

func TestRenderSyncWave_PhaseRowsAndWaveBuckets(t *testing.T) {
	entry := SyncWaveTimelineEntry{
		AppName: "x",
		Phases: []SyncWavePhaseEntry{
			{
				Name: "Sync",
				Waves: []SyncWaveBucketEntry{
					{Wave: 0, Resources: []SyncWaveResourceEntry{
						{Kind: "ConfigMap", Name: "config", SyncStatus: "Synced", HealthStatus: "Healthy"},
					}},
					{Wave: 1, Resources: []SyncWaveResourceEntry{
						{Kind: "Deployment", Name: "api", SyncStatus: "OutOfSync", HealthStatus: "Progressing"},
					}},
					{Wave: SyncWaveUnknownWave, Resources: []SyncWaveResourceEntry{
						{Kind: "Ingress", Name: "ing", SyncStatus: "OutOfSync", HealthStatus: "Healthy"},
					}},
				},
			},
		},
	}
	got := RenderSyncWaveTimeline(entry, 100, 30)
	assert.Contains(t, got, "Sync")
	assert.Contains(t, got, "wave 0")
	assert.Contains(t, got, "ConfigMap/config")
	assert.Contains(t, got, "wave 1")
	assert.Contains(t, got, "Deployment/api")
	assert.Contains(t, got, "wave ?")
	assert.Contains(t, got, "Ingress/ing")
}

func TestRenderSyncWave_CollapsedPhaseShowsOnlyHeader(t *testing.T) {
	// In the two-pane layout, collapse is driven by the Collapsed map
	// (key = phase name). The selected phase's body shows the
	// "<phase> collapsed — Enter to expand" placeholder instead of the
	// resource rows.
	entry := SyncWaveTimelineEntry{
		AppName: "x",
		Phases: []SyncWavePhaseEntry{
			{
				Name:      "PostSync",
				Collapsed: true,
				Waves: []SyncWaveBucketEntry{
					{Wave: 0, Resources: []SyncWaveResourceEntry{
						{Kind: "Job", Name: "smoke", IsHook: true, OpStatus: "Succeeded"},
					}},
				},
			},
		},
		SidebarCursor: 0,
		Collapsed:     map[string]bool{"PostSync": true},
	}
	got := RenderSyncWaveTimeline(entry, 100, 30)
	assert.Contains(t, got, "PostSync")
	assert.NotContains(t, got, "smoke", "collapsed phase must not render rows")
}

func TestRenderSyncWave_HookRowShowsHookPhase(t *testing.T) {
	entry := SyncWaveTimelineEntry{
		AppName: "x",
		Phases: []SyncWavePhaseEntry{
			{Name: "PreSync", Waves: []SyncWaveBucketEntry{{Wave: SyncWaveUnknownWave, Resources: []SyncWaveResourceEntry{
				{Kind: "Job", Name: "migrate", IsHook: true, OpStatus: "Succeeded", HookPhase: "Succeeded"},
			}}}},
		},
	}
	got := RenderSyncWaveTimeline(entry, 100, 30)
	assert.Contains(t, got, "Job/migrate")
	assert.Contains(t, got, "Succeeded")
}

func TestRenderSyncWave_FocusedPhaseMarker(t *testing.T) {
	entry := SyncWaveTimelineEntry{
		Phases: []SyncWavePhaseEntry{
			{Name: "PreSync"},
			{Name: "Sync", Focused: true},
		},
	}
	got := RenderSyncWaveTimeline(entry, 100, 30)
	// The focus marker is "▸" or "▾" (focused row uses bold/▸ chevron).
	assert.Contains(t, got, "Sync")
}

// buildLongPhase produces a single-phase entry with `n` resource rows in
// a single wave. The scroll argument feeds the global
// SyncWaveTimelineEntry.BodyScroll — there is no per-phase scroll any
// more. focused is preserved purely so existing tests can keep asserting
// against a focused row marker.
func buildLongPhase(n int, focused bool, scroll int) SyncWaveTimelineEntry {
	resources := make([]SyncWaveResourceEntry, n)
	for i := range n {
		resources[i] = SyncWaveResourceEntry{
			Kind:         "ConfigMap",
			Name:         fmt.Sprintf("r%03d", i),
			SyncStatus:   "Synced",
			HealthStatus: "Healthy",
		}
	}
	return SyncWaveTimelineEntry{
		AppName:    "x",
		BodyScroll: scroll,
		Phases: []SyncWavePhaseEntry{
			{
				Name:    "Sync",
				Focused: focused,
				Waves:   []SyncWaveBucketEntry{{Wave: 0, Resources: resources}},
			},
		},
	}
}

func TestRenderSyncWave_ScrollClipsBody(t *testing.T) {
	const total = 30
	// Global scroll counts every body line — including the phase
	// header, which is body line 0. So scroll=6 skips the phase header
	// + r000..r004, leaving r005 as the first content row.
	entry := buildLongPhase(total, true, 6)
	// Use a tall enough viewport that height never clips the body —
	// only the scroll offset should advance the body window.
	got := RenderSyncWaveTimeline(entry, 100, 200)

	assert.Contains(t, got, "ConfigMap/r005", "row 5 should be visible after scrolling 6")
	// The first 5 rows (r000..r004) must have been scrolled past.
	for i := range 5 {
		assert.NotContains(t, got, fmt.Sprintf("ConfigMap/r%03d", i),
			"row %d should be scrolled out of view", i)
	}
}

func TestRenderSyncWave_HeightClipsToViewport(t *testing.T) {
	const total = 60
	const height = 10
	entry := buildLongPhase(total, false, 0)
	got := RenderSyncWaveTimeline(entry, 100, height)
	lines := strings.Split(got, "\n")
	assert.LessOrEqual(t, len(lines), height,
		"rendered output must fit within %d lines, got %d", height, len(lines))
}

func TestRenderSyncWave_HeaderPreservedWhenClipping(t *testing.T) {
	// Even when the viewport is small, the header rows (title, last-op,
	// live-phase, divider) must remain visible — clipping happens on the
	// body rows only.
	entry := buildLongPhase(40, true, 0)
	entry.LivePhase = "Running"
	entry.LastOperation = &SyncWaveLastOperation{
		Phase:      "Running",
		StartedAt:  time.Now().Add(-30 * time.Second),
		FinishedAt: time.Time{},
		Revision:   "deadbee",
	}
	got := RenderSyncWaveTimeline(entry, 100, 6)
	assert.Contains(t, got, "Sync Wave Timeline: x")
	assert.Contains(t, got, "Last Sync: Running")
	assert.Contains(t, got, "Live phase: Running")
}

func TestRenderSyncWave_EmptyPhaseShowsNoneAnnotation(t *testing.T) {
	// In the two-pane layout, empty phases are annotated in the sidebar
	// with "(none)" so operators can see the full pipeline at a glance
	// even when a phase contributed no resources this run. Selecting the
	// non-empty phase via SidebarCursor renders its resources in the
	// body; selecting the empty phase shows the placeholder.
	entry := SyncWaveTimelineEntry{
		AppName: "x",
		Phases: []SyncWavePhaseEntry{
			{Name: "PostSync"}, // no waves
			{Name: "Sync", Waves: []SyncWaveBucketEntry{
				{Wave: 0, Resources: []SyncWaveResourceEntry{
					{Kind: "ConfigMap", Name: "cfg", SyncStatus: "Synced", HealthStatus: "Healthy"},
				}},
			}},
		},
		SidebarCursor: 1, // select the non-empty phase so the body renders ConfigMap/cfg
	}
	got := RenderSyncWaveTimeline(entry, 100, 30)
	assert.Contains(t, got, "PostSync",
		"empty phases must still appear in the sidebar so operators see the full pipeline")
	assert.Contains(t, got, "(none)",
		"empty phases must be annotated with (none) in the sidebar")
	// The non-empty phase still renders its body row.
	assert.Contains(t, got, "ConfigMap/cfg")
}

func TestRenderSyncWave_BodyPaddedToHeight(t *testing.T) {
	// When the rendered content is shorter than the viewport, the
	// renderer pads with empty lines so the surrounding overlay frame
	// stays a fixed size as the user scrolls.
	entry := SyncWaveTimelineEntry{
		AppName: "x",
		Phases: []SyncWavePhaseEntry{
			{Name: "Sync", Waves: []SyncWaveBucketEntry{
				{Wave: 0, Resources: []SyncWaveResourceEntry{
					{Kind: "ConfigMap", Name: "cfg", SyncStatus: "Synced", HealthStatus: "Healthy"},
				}},
			}},
		},
	}
	const height = 25
	got := RenderSyncWaveTimeline(entry, 100, height)
	lines := strings.Split(got, "\n")
	assert.Equal(t, height, len(lines),
		"renderer must pad to exactly %d lines so overlay box stays fixed", height)
}

func TestRenderSyncWave_LineClampedToWidth(t *testing.T) {
	// Long resource labels must be truncated to the inner width so they
	// can't soft-wrap and shift the next overlay row's `│` border into
	// the middle of the line.
	long := strings.Repeat("a", 120)
	entry := SyncWaveTimelineEntry{
		AppName: "x",
		Phases: []SyncWavePhaseEntry{
			{Name: "Sync", Waves: []SyncWaveBucketEntry{
				{Wave: 0, Resources: []SyncWaveResourceEntry{
					{Kind: "Deployment", Name: long, SyncStatus: "Synced", HealthStatus: "Healthy"},
				}},
			}},
		},
	}
	const width = 60
	got := RenderSyncWaveTimeline(entry, width, 30)
	for line := range strings.SplitSeq(got, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), width,
			"line wider than %d cells will soft-wrap and break the border alignment: %q",
			width, line)
	}
}

func TestFlattenBodyRows_ExpandedPhase(t *testing.T) {
	phase := SyncWavePhaseEntry{
		Name: "Sync",
		Waves: []SyncWaveBucketEntry{
			{Wave: 0, Resources: []SyncWaveResourceEntry{
				{Kind: "ConfigMap", Name: "config"},
				{Kind: "Secret", Name: "creds"},
			}},
			{Wave: 1, Resources: []SyncWaveResourceEntry{
				{Kind: "Deployment", Name: "api"},
			}},
		},
	}
	rows := flattenBodyRows(phase, nil)
	require.Len(t, rows, 5)
	assert.Equal(t, bodyRowKindWaveHeader, rows[0].kind)
	assert.Equal(t, 0, rows[0].waveIdx)
	assert.Equal(t, bodyRowKindResource, rows[1].kind)
	assert.Equal(t, 0, rows[1].waveIdx)
	assert.Equal(t, 0, rows[1].resourceIdx)
	assert.Equal(t, bodyRowKindResource, rows[2].kind)
	assert.Equal(t, 1, rows[2].resourceIdx)
	assert.Equal(t, bodyRowKindWaveHeader, rows[3].kind)
	assert.Equal(t, 1, rows[3].waveIdx)
	assert.Equal(t, bodyRowKindResource, rows[4].kind)
}

func TestFlattenBodyRows_CollapsedWaveHidesResources(t *testing.T) {
	phase := SyncWavePhaseEntry{
		Name: "Sync",
		Waves: []SyncWaveBucketEntry{
			{Wave: 0, Resources: []SyncWaveResourceEntry{
				{Kind: "ConfigMap", Name: "config"},
			}},
			{Wave: 1, Resources: []SyncWaveResourceEntry{
				{Kind: "Deployment", Name: "api"},
			}},
		},
	}
	collapsed := map[string]bool{"Sync/wave 0": true}
	rows := flattenBodyRows(phase, collapsed)
	require.Len(t, rows, 3)
	assert.Equal(t, bodyRowKindWaveHeader, rows[0].kind)
	assert.Equal(t, 0, rows[0].waveIdx)
	assert.Equal(t, bodyRowKindWaveHeader, rows[1].kind)
	assert.Equal(t, 1, rows[1].waveIdx)
	assert.Equal(t, bodyRowKindResource, rows[2].kind)
}

func TestFlattenBodyRows_PhaseCollapsedReturnsPlaceholder(t *testing.T) {
	phase := SyncWavePhaseEntry{
		Name: "Sync",
		Waves: []SyncWaveBucketEntry{
			{Wave: 0, Resources: []SyncWaveResourceEntry{
				{Kind: "ConfigMap", Name: "config"},
			}},
		},
	}
	collapsed := map[string]bool{"Sync": true}
	rows := flattenBodyRows(phase, collapsed)
	require.Len(t, rows, 1)
	assert.Equal(t, bodyRowKindPlaceholder, rows[0].kind)
}

func TestFlattenBodyRows_EmptyPhaseReturnsPlaceholder(t *testing.T) {
	phase := SyncWavePhaseEntry{Name: "PostSync"}
	rows := flattenBodyRows(phase, nil)
	require.Len(t, rows, 1)
	assert.Equal(t, bodyRowKindPlaceholder, rows[0].kind)
}

func TestFlattenBodyRows_WaveLabelMatchesUnknown(t *testing.T) {
	phase := SyncWavePhaseEntry{
		Name: "Sync",
		Waves: []SyncWaveBucketEntry{
			{Wave: SyncWaveUnknownWave, Resources: []SyncWaveResourceEntry{
				{Kind: "Pod", Name: "p"},
			}},
		},
	}
	collapsed := map[string]bool{"Sync/wave ?": true}
	rows := flattenBodyRows(phase, collapsed)
	require.Len(t, rows, 1)
	assert.Equal(t, bodyRowKindWaveHeader, rows[0].kind)
}

// buildSidebar produces the left-pane phase index. Each line is exactly
// sidebarWidth cells wide; viewportRows pads or clamps to a fixed line
// count so the sidebar stays a stable column inside the overlay frame.
// Highlight tier is derived from ActivePane + SidebarCursor — exercised
// in the renderer-level tests; here we only assert the row content +
// width contract so the helper is testable in isolation.
func TestBuildSidebar_PhasesWithCounts(t *testing.T) {
	entry := SyncWaveTimelineEntry{
		Phases: []SyncWavePhaseEntry{
			{Name: "PreSync", Waves: []SyncWaveBucketEntry{
				{Wave: 0, Resources: []SyncWaveResourceEntry{{Kind: "Job", Name: "x"}}},
			}},
			{Name: "Sync", Waves: []SyncWaveBucketEntry{
				{Wave: 0, Resources: []SyncWaveResourceEntry{{Kind: "Pod", Name: "a"}, {Kind: "Pod", Name: "b"}}},
			}},
			{Name: "PostSync"}, // empty
		},
	}
	lines := buildSidebar(entry, 3)
	require.Len(t, lines, 3)
	assert.Contains(t, lines[0], "▾ PreSync")
	assert.Contains(t, lines[0], "(1)")
	assert.Contains(t, lines[1], "▾ Sync")
	assert.Contains(t, lines[1], "(2)")
	assert.Contains(t, lines[2], "▸ PostSync")
	assert.Contains(t, lines[2], "(none)")
}

func TestBuildSidebar_CollapsedPhaseShowsArrow(t *testing.T) {
	entry := SyncWaveTimelineEntry{
		Phases: []SyncWavePhaseEntry{
			{Name: "Sync", Waves: []SyncWaveBucketEntry{
				{Wave: 0, Resources: []SyncWaveResourceEntry{{Kind: "Pod", Name: "a"}}},
			}},
		},
	}
	collapsedEntry := entry
	collapsedEntry.Phases = []SyncWavePhaseEntry{{Name: "Sync", Collapsed: true, Waves: entry.Phases[0].Waves}}
	lines := buildSidebar(collapsedEntry, 1)
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "▸ Sync")
}

func TestBuildSidebar_PadsToViewportRows(t *testing.T) {
	entry := SyncWaveTimelineEntry{
		Phases: []SyncWavePhaseEntry{
			{Name: "Sync", Waves: []SyncWaveBucketEntry{
				{Wave: 0, Resources: []SyncWaveResourceEntry{{Kind: "Pod", Name: "a"}}},
			}},
		},
	}
	lines := buildSidebar(entry, 5) // 1 phase, 5 viewport rows
	require.Len(t, lines, 5)
	// Empty padding rows are width-equal blanks.
	assert.Equal(t, lipgloss.Width(lines[0]), lipgloss.Width(lines[3]))
}

func TestBuildSidebar_EveryLineIsExactSidebarWidth(t *testing.T) {
	entry := SyncWaveTimelineEntry{
		Phases: []SyncWavePhaseEntry{{Name: "Sync"}},
	}
	lines := buildSidebar(entry, 1)
	require.Len(t, lines, 1)
	assert.Equal(t, sidebarWidth, lipgloss.Width(lines[0]))
}

func TestBuildBody_RendersFlatRows(t *testing.T) {
	entry := SyncWaveTimelineEntry{
		Phases: []SyncWavePhaseEntry{
			{Name: "Sync", Waves: []SyncWaveBucketEntry{
				{Wave: 0, Resources: []SyncWaveResourceEntry{
					{Kind: "ConfigMap", Name: "config", SyncStatus: "Synced", HealthStatus: "Healthy"},
				}},
				{Wave: 1, Resources: []SyncWaveResourceEntry{
					{Kind: "Deployment", Name: "api", SyncStatus: "OutOfSync"},
				}},
			}},
		},
		SidebarCursor: 0,
	}
	lines := buildBody(entry, 60, 10)
	require.Len(t, lines, 10) // padded to viewportRows
	body := strings.Join(lines, "\n")
	assert.Contains(t, body, "wave 0")
	assert.Contains(t, body, "ConfigMap/config")
	assert.Contains(t, body, "wave 1")
	assert.Contains(t, body, "Deployment/api")
}

func TestBuildBody_CollapsedPhaseShowsPlaceholder(t *testing.T) {
	entry := SyncWaveTimelineEntry{
		Phases: []SyncWavePhaseEntry{
			{Name: "PostSync", Collapsed: true, Waves: []SyncWaveBucketEntry{
				{Wave: 0, Resources: []SyncWaveResourceEntry{{Kind: "Job", Name: "j"}}},
			}},
		},
		SidebarCursor: 0,
		Collapsed:     map[string]bool{"PostSync": true},
	}
	lines := buildBody(entry, 60, 10)
	body := strings.Join(lines, "\n")
	assert.Contains(t, body, "PostSync collapsed — Enter to expand")
	assert.NotContains(t, body, "Job/j")
}

func TestBuildBody_EmptyPhaseShowsPlaceholder(t *testing.T) {
	entry := SyncWaveTimelineEntry{
		Phases: []SyncWavePhaseEntry{
			{Name: "PostSync"},
		},
		SidebarCursor: 0,
	}
	lines := buildBody(entry, 60, 10)
	body := strings.Join(lines, "\n")
	assert.Contains(t, body, "PostSync collapsed — Enter to expand")
}

func TestBuildBody_CollapsedWaveShowsItemCount(t *testing.T) {
	entry := SyncWaveTimelineEntry{
		Phases: []SyncWavePhaseEntry{
			{Name: "Sync", Waves: []SyncWaveBucketEntry{
				{Wave: 0, Resources: []SyncWaveResourceEntry{
					{Kind: "Pod", Name: "a"}, {Kind: "Pod", Name: "b"}, {Kind: "Pod", Name: "c"},
				}},
			}},
		},
		SidebarCursor: 0,
		Collapsed:     map[string]bool{"Sync/wave 0": true},
	}
	lines := buildBody(entry, 60, 10)
	body := strings.Join(lines, "\n")
	assert.Contains(t, body, "▸ wave 0")
	assert.Contains(t, body, "(3 items)")
	assert.NotContains(t, body, "Pod/a")
}

func TestBuildBody_CursorHighlightActivePane(t *testing.T) {
	// Tests run without a TTY, so termenv defaults to a stripped color
	// profile and lipgloss drops the foreground/background codes that
	// make the highlight visible. Force the renderer to ANSI mode so
	// activeRowStyle().Render(...) emits real escape sequences.
	original := lipgloss.DefaultRenderer().ColorProfile()
	originalNoColor := ConfigNoColor
	t.Cleanup(func() {
		lipgloss.DefaultRenderer().SetColorProfile(original)
		ConfigNoColor = originalNoColor
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = false
	lipgloss.DefaultRenderer().SetColorProfile(termenv.ANSI)
	ApplyTheme(DefaultTheme())
	lipgloss.DefaultRenderer().SetColorProfile(termenv.ANSI)

	entry := SyncWaveTimelineEntry{
		Phases: []SyncWavePhaseEntry{
			{Name: "Sync", Waves: []SyncWaveBucketEntry{
				{Wave: 0, Resources: []SyncWaveResourceEntry{
					{Kind: "Pod", Name: "a"},
				}},
			}},
		},
		SidebarCursor: 0,
		BodyCursor:    SyncWaveBodyCursor{WaveIdx: 0, ResourceIdx: -1}, // cursor on wave header
		ActivePane:    SyncWavePaneBody,
	}
	lines := buildBody(entry, 60, 5)
	// The wave header line should contain ANSI escape sequences for the
	// bright highlight (escape codes start with \x1b).
	assert.Contains(t, lines[0], "\x1b[")
}

func TestBuildBody_ScrollOffsetSlicesBody(t *testing.T) {
	resources := make([]SyncWaveResourceEntry, 20)
	for i := range resources {
		resources[i] = SyncWaveResourceEntry{Kind: "Pod", Name: fmt.Sprintf("p%02d", i)}
	}
	entry := SyncWaveTimelineEntry{
		Phases: []SyncWavePhaseEntry{
			{Name: "Sync", Waves: []SyncWaveBucketEntry{{Wave: 0, Resources: resources}}},
		},
		SidebarCursor: 0,
		BodyScroll:    5, // skip first 5 rows (wave header + first 4 resources)
	}
	lines := buildBody(entry, 60, 10)
	body := strings.Join(lines, "\n")
	// Should NOT contain p00..p03 (5 rows skipped: wave header + p00..p03).
	assert.NotContains(t, body, "Pod/p00")
	assert.Contains(t, body, "Pod/p04")
}

// TestRenderSyncWaveTimeline_TwoPaneLayout exercises the renderer
// integration: the sidebar must list all phases, the body must show the
// selected phase's content (Sync, picked via SidebarCursor=1), and the
// header must contain the title. This is the marker test for the
// two-pane layout — fails until the renderer is rewritten.
func TestRenderSyncWaveTimeline_TwoPaneLayout(t *testing.T) {
	entry := SyncWaveTimelineEntry{
		AppName: "my-app",
		Phases: []SyncWavePhaseEntry{
			{Name: "PreSync", Waves: []SyncWaveBucketEntry{
				{Wave: 0, Resources: []SyncWaveResourceEntry{{Kind: "Job", Name: "j"}}},
			}},
			{Name: "Sync", Waves: []SyncWaveBucketEntry{
				{Wave: 0, Resources: []SyncWaveResourceEntry{{Kind: "Pod", Name: "a"}}},
			}},
		},
		SidebarCursor: 1, // selected phase = Sync
		BodyCursor:    SyncWaveBodyCursor{WaveIdx: 0, ResourceIdx: -1},
		ActivePane:    SyncWavePaneSidebar,
	}
	out := RenderSyncWaveTimeline(entry, 80, 20)
	// Sidebar visible: should contain phase names.
	assert.Contains(t, out, "PreSync")
	assert.Contains(t, out, "Sync")
	// Body should contain the selected phase's content (Sync's resources).
	assert.Contains(t, out, "Pod/a")
	// Title in header.
	assert.Contains(t, out, "Sync Wave Timeline: my-app")
}

// TestRenderSyncWaveTimeline_SinglePaneFallbackBelow50Cols verifies the
// narrow-viewport fallback: with width=40 (or SinglePane=true) the
// renderer drops the sidebar and renders the body across the full width.
func TestRenderSyncWaveTimeline_SinglePaneFallbackBelow50Cols(t *testing.T) {
	entry := SyncWaveTimelineEntry{
		AppName:    "x",
		SinglePane: true,
		Phases: []SyncWavePhaseEntry{
			{Name: "Sync", Waves: []SyncWaveBucketEntry{
				{Wave: 0, Resources: []SyncWaveResourceEntry{{Kind: "Pod", Name: "a"}}},
			}},
		},
		SidebarCursor: 0,
	}
	out := RenderSyncWaveTimeline(entry, 40, 10)
	// In single-pane mode, the body should still render.
	assert.Contains(t, out, "Pod/a")
}

// TestRenderSyncWaveTimeline_FixedHeightPreserved guarantees the
// renderer pads to exactly `height` lines even with no phases — keeps
// the surrounding overlay frame stable.
func TestRenderSyncWaveTimeline_FixedHeightPreserved(t *testing.T) {
	entry := SyncWaveTimelineEntry{AppName: "x"}
	out := RenderSyncWaveTimeline(entry, 80, 10)
	lines := strings.Split(out, "\n")
	assert.Len(t, lines, 10)
}

// TestRenderSyncWaveTimeline_HeaderStillWorks verifies the header
// pipeline (title, last-op, live-phase, loading) is preserved through
// the layout rewrite — these lines are full-width and live above the
// two-pane split.
func TestRenderSyncWaveTimeline_HeaderStillWorks(t *testing.T) {
	entry := SyncWaveTimelineEntry{
		AppName: "my-app",
		LastOperation: &SyncWaveLastOperation{
			Phase:      "Succeeded",
			FinishedAt: time.Now().Add(-12 * time.Minute),
			Revision:   "abc1234",
		},
		LivePhase: "Running",
		Loading:   true,
	}
	out := RenderSyncWaveTimeline(entry, 80, 30)
	assert.Contains(t, out, "Sync Wave Timeline: my-app")
	assert.Contains(t, out, "Last Sync: Succeeded")
	assert.Contains(t, out, "abc1234")
	assert.Contains(t, out, "Live phase: Running")
	assert.Contains(t, out, "Loading wave map")
}
