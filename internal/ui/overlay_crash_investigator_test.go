package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderCrashInvestigatorOverlay_TabBar(t *testing.T) {
	entry := CrashInvestigatorEntry{
		PodName: "p", Namespace: "default", ActiveContainer: "app",
		Tab:           CrashTabSummary,
		AppContainers: []CrashContainerEntry{{Name: "app"}},
	}
	out := RenderCrashInvestigatorOverlay(entry, 0, 100, 30)
	assert.Contains(t, out, "Crash Investigator")
	assert.Contains(t, out, "Summary")
	assert.Contains(t, out, "Events")
	assert.Contains(t, out, "Logs")
	assert.Contains(t, out, "Describe")
	assert.Contains(t, out, "default/p")
}

func TestRenderCrashInvestigatorOverlay_SummaryTab(t *testing.T) {
	entry := CrashInvestigatorEntry{
		PodName: "p", Namespace: "default", ActiveContainer: "sidecar",
		Tab: CrashTabSummary,
		InitContainers: []CrashContainerEntry{
			{Name: "init-db", IsInit: true, State: "Waiting", StateReason: "CrashLoopBackOff", RestartCount: 4},
		},
		AppContainers: []CrashContainerEntry{
			{Name: "app", State: "Running", Ready: true, RestartCount: 0},
			{Name: "sidecar", State: "Waiting", StateReason: "CrashLoopBackOff", RestartCount: 3, HasLastTerm: true, LastReason: "Error", LastExitCode: 1},
		},
	}
	out := RenderCrashInvestigatorOverlay(entry, 0, 100, 30)
	// Header columns
	assert.Contains(t, out, "CONTAINER")
	assert.Contains(t, out, "STATE")
	assert.Contains(t, out, "RESTARTS")
	// Init sub-table label
	assert.Contains(t, stripANSI(out), "Init")
	// All container names
	assert.Contains(t, out, "init-db")
	assert.Contains(t, out, "app")
	assert.Contains(t, out, "sidecar")
	// Reason
	assert.Contains(t, out, "CrashLoopBackOff")
	// Active row marker
	assert.True(t, strings.Contains(out, "→ sidecar") || strings.Contains(out, "▶ sidecar"),
		"active container row must be visually marked, got:\n%s", out)
}

func TestRenderCrashInvestigatorOverlay_EventsTab(t *testing.T) {
	entry := CrashInvestigatorEntry{
		PodName: "p", Namespace: "default",
		Tab:           CrashTabEvents,
		AppContainers: []CrashContainerEntry{{Name: "app"}},
		Events: []CrashEventEntry{
			{Type: "Warning", Reason: "BackOff", Age: "5s", Message: "Back-off"},
			{Type: "Normal", Reason: "Pulled", Age: "1m", Message: "Image pulled"},
		},
	}
	out := RenderCrashInvestigatorOverlay(entry, 0, 100, 30)
	assert.Contains(t, out, "BackOff")
	assert.Contains(t, out, "Pulled")
	assert.Contains(t, out, "Back-off")
}

func TestRenderCrashInvestigatorOverlay_EventsTabEmpty(t *testing.T) {
	entry := CrashInvestigatorEntry{
		PodName: "p", Namespace: "default", Tab: CrashTabEvents,
		AppContainers: []CrashContainerEntry{{Name: "app"}},
	}
	out := RenderCrashInvestigatorOverlay(entry, 0, 100, 30)
	assert.Contains(t, out, "No events for this pod")
}

func TestRenderCrashInvestigatorOverlay_LogsTabPrevious(t *testing.T) {
	entry := CrashInvestigatorEntry{
		PodName: "p", Namespace: "default", ActiveContainer: "app",
		Tab: CrashTabLogs, ShowPrevious: true,
		AppContainers: []CrashContainerEntry{{
			Name: "app", PreviousLog: "panic: something\ngoroutine 1 [running]",
			CurrentLog: "starting up",
		}},
	}
	out := RenderCrashInvestigatorOverlay(entry, 0, 100, 30)
	assert.Contains(t, out, "previous")
	assert.Contains(t, out, "panic: something")
	assert.NotContains(t, out, "starting up", "current log must not bleed into previous mode")
}

func TestRenderCrashInvestigatorOverlay_LogsTabCurrent(t *testing.T) {
	entry := CrashInvestigatorEntry{
		PodName: "p", Namespace: "default", ActiveContainer: "app",
		Tab: CrashTabLogs, ShowPrevious: false,
		AppContainers: []CrashContainerEntry{{
			Name: "app", PreviousLog: "old", CurrentLog: "starting up",
		}},
	}
	out := RenderCrashInvestigatorOverlay(entry, 0, 100, 30)
	assert.Contains(t, out, "current")
	assert.Contains(t, out, "starting up")
	assert.NotContains(t, out, "old")
}

func TestRenderCrashInvestigatorOverlay_LogsTabPreviousEmpty(t *testing.T) {
	entry := CrashInvestigatorEntry{
		PodName: "p", Namespace: "default", ActiveContainer: "app",
		Tab: CrashTabLogs, ShowPrevious: true,
		AppContainers: []CrashContainerEntry{{Name: "app"}}, // no PreviousLog
	}
	out := RenderCrashInvestigatorOverlay(entry, 0, 100, 30)
	assert.Contains(t, out, "no previous container output")
}

func TestRenderCrashInvestigatorOverlay_LogsTabError(t *testing.T) {
	entry := CrashInvestigatorEntry{
		PodName: "p", Namespace: "default", ActiveContainer: "app",
		Tab: CrashTabLogs, ShowPrevious: true,
		AppContainers: []CrashContainerEntry{{Name: "app", LogError: "stream broken"}},
	}
	out := RenderCrashInvestigatorOverlay(entry, 0, 100, 30)
	assert.Contains(t, out, "failed to load logs")
	assert.Contains(t, out, "stream broken")
}

func TestRenderCrashInvestigatorOverlay_DescribeTab(t *testing.T) {
	entry := CrashInvestigatorEntry{
		PodName: "p", Namespace: "default", Tab: CrashTabDescribe,
		AppContainers: []CrashContainerEntry{{Name: "app"}},
		Describe:      "Name: p\nNamespace: default\nContainers:\n  app:\n    Image: nginx\n",
	}
	out := RenderCrashInvestigatorOverlay(entry, 0, 100, 30)
	assert.Contains(t, out, "Name: p")
	assert.Contains(t, out, "Image: nginx")
}

func TestRenderCrashInvestigatorOverlay_DescribeTabError(t *testing.T) {
	entry := CrashInvestigatorEntry{
		PodName: "p", Namespace: "default", Tab: CrashTabDescribe,
		AppContainers: []CrashContainerEntry{{Name: "app"}},
		DescribeError: "kubectl not found",
	}
	out := RenderCrashInvestigatorOverlay(entry, 0, 100, 30)
	assert.Contains(t, out, "kubectl not found")
}

func TestRenderCrashInvestigatorOverlay_ThemeBg(t *testing.T) {
	originalNoColor := ConfigNoColor
	originalTransparent := ConfigTransparentBg
	t.Cleanup(func() {
		ConfigNoColor = originalNoColor
		ConfigTransparentBg = originalTransparent
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = false
	ConfigTransparentBg = false
	ApplyTheme(DefaultTheme())
	// ApplyTheme restores originalColorProfile (theme.go:109-110), so
	// re-force TrueColor here for the SGR-counting assertion to be
	// observable at all.

	entry := CrashInvestigatorEntry{
		PodName: "p", Namespace: "default", ActiveContainer: "app",
		Tab: CrashTabSummary,
		AppContainers: []CrashContainerEntry{
			{Name: "app", State: "Running", Ready: true, RestartCount: 0},
		},
	}
	out := RenderCrashInvestigatorOverlay(entry, 0, 100, 30)
	// Guard against the "fg-only spans punching through to terminal default
	// bg" regression. lipgloss in truecolor mode merges fg+bg into a single
	// SGR like `\x1b[38;2;R;G;B;48;2;R;G;B;m`, so a theme-aware span emits
	// both `38;2;` and `48;2;` markers in the same sequence; a buggy
	// fg-only span emits only `38;2;`. Requiring at least 4 bg markers AND
	// bg >= fg ensures every styled span carries a bg.
	bgCount := strings.Count(out, "48;2;")
	fgCount := strings.Count(out, "38;2;")
	assert.GreaterOrEqual(t, bgCount, 4,
		"renderer must emit at least 4 background-set escapes so the overlay reads as one uniform surface; got %d.\n%s", bgCount, out)
	assert.GreaterOrEqual(t, bgCount, fgCount,
		"every foreground-set SGR must also set a background to avoid fg-only spans punching through to terminal default bg; got fg=%d, bg=%d.\n%s", fgCount, bgCount, out)
}
