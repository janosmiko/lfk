package app

import (
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
)

// TestOverlayHintBar_NoOverlay verifies that no hint bar is returned when no overlay is active.
func TestOverlayHintBar_NoOverlay(t *testing.T) {
	m := Model{overlay: overlayNone}
	got := m.overlayHintBar()
	if got != "" {
		t.Errorf("expected empty hint bar for overlayNone, got %q", got)
	}
}

// TestOverlayHintBar_ReturnsNonEmpty verifies every overlay kind produces a non-empty hint bar.
func TestOverlayHintBar_ReturnsNonEmpty(t *testing.T) {
	overlays := []struct {
		name    string
		kind    overlayKind
		setup   func(m *Model) // optional extra state
		wantKey string         // a key hint that must appear
	}{
		{"Namespace", overlayNamespace, nil, "esc"},
		{"Action", overlayAction, nil, "enter"},
		{"Confirm", overlayConfirm, nil, "Enter"},
		{"QuitConfirm", overlayQuitConfirm, nil, "Enter"},
		{"PasteConfirm", overlayPasteConfirm, nil, "paste"},
		{"ConfirmType", overlayConfirmType, nil, "DELETE"},
		{"ScaleInput", overlayScaleInput, nil, "Enter"},
		{"PortForward", overlayPortForward, nil, "enter"},
		{"ContainerSelect", overlayContainerSelect, nil, "enter"},
		{"PodSelect", overlayPodSelect, nil, "enter"},
		{"LogPodSelect", overlayLogPodSelect, nil, "enter"},
		{"LogContainerSelect", overlayLogContainerSelect, nil, "enter"},
		{"Bookmarks", overlayBookmarks, nil, "enter"},
		{"BookmarksFilter", overlayBookmarks, func(m *Model) { m.bookmarkSearchMode = bookmarkModeFilter }, "filter"},
		{"Templates", overlayTemplates, nil, "enter"},
		{"Colorscheme", overlayColorscheme, nil, "enter"},
		{"FilterPreset", overlayFilterPreset, nil, "enter"},
		{"RBAC", overlayRBAC, nil, "close"},
		{"BatchLabel", overlayBatchLabel, nil, "Enter"},
		{"PodStartup", overlayPodStartup, nil, "close"},
		{"QuotaDashboard", overlayQuotaDashboard, nil, "close"},
		{"EventTimeline", overlayEventTimeline, nil, "move"},
		{"Alerts", overlayAlerts, nil, "scroll"},
		{"NetworkPolicy", overlayNetworkPolicy, nil, "scroll"},
		{"SecretEditor", overlaySecretEditor, nil, "nav"},
		{"SecretEditorEditing", overlaySecretEditor, func(m *Model) { m.secretEditing = true }, "save"},
		{"ConfigMapEditor", overlayConfigMapEditor, nil, "nav"},
		{"ConfigMapEditorEditing", overlayConfigMapEditor, func(m *Model) { m.configMapEditing = true }, "save"},
		{"Rollback", overlayRollback, nil, "rollback"},
		{"HelmRollback", overlayHelmRollback, nil, "rollback"},
		{"HelmHistory", overlayHelmHistory, nil, "close"},
		{"LabelEditor", overlayLabelEditor, nil, "nav"},
		{"LabelEditorEditing", overlayLabelEditor, func(m *Model) { m.labelEditing = true }, "save"},
		{"CanI", overlayCanI, nil, "navigate"},
		{"CanISearch", overlayCanI, func(m *Model) { m.canISearchActive = true }, "search"},
		{"CanISubject", overlayCanISubject, nil, "select"},
		{"ExplainSearch", overlayExplainSearch, nil, "navigate"},
		{"Orphans", overlayOrphans, nil, "jump"},
		{"OrphansFilter", overlayOrphans, func(m *Model) { m.orphans.filterActive = true }, "filter"},
		{"OrphansStrictModeChip", overlayOrphans, func(m *Model) { m.orphans.strict = true }, "strict"},
		{"OrphansLenientModeChip", overlayOrphans, func(m *Model) { m.orphans.strict = false }, "lenient"},
	}

	for _, tt := range overlays {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{overlay: tt.kind, width: 120}
			if tt.setup != nil {
				tt.setup(&m)
			}
			got := m.overlayHintBar()
			if got == "" {
				t.Errorf("overlayHintBar() returned empty for %s", tt.name)
			}
			if !strings.Contains(got, tt.wantKey) {
				t.Errorf("overlayHintBar() for %s missing key %q in %q", tt.name, tt.wantKey, got)
			}
		})
	}
}

// Confirm-style dialogs advertise both halves of the key pair the
// handlers accept — Enter/y for confirm and Esc/n for cancel. Users
// who reach for y or n shouldn't have to read source to find them.
// PRs #80 and #97 surfaced this gap by trying to add inline `[y] yes
// [n] no` text inside the overlay; the hint bar is now the place
// these letters live.
func TestOverlayHintBar_ConfirmDialogsAdvertiseYAndN(t *testing.T) {
	cases := []struct {
		name  string
		setup func() Model
	}{
		{"Confirm", func() Model {
			return Model{overlay: overlayConfirm, width: 120}
		}},
		{"QuitConfirm", func() Model {
			return Model{overlay: overlayQuitConfirm, width: 120}
		}},
		{"PasteConfirm", func() Model {
			return Model{overlay: overlayPasteConfirm, width: 120}
		}},
		{"BookmarkConfirmDelete", func() Model {
			return Model{overlay: overlayBookmarks, bookmarkSearchMode: bookmarkModeConfirmDelete, width: 120}
		}},
		{"BookmarkConfirmDeleteAll", func() Model {
			return Model{overlay: overlayBookmarks, bookmarkSearchMode: bookmarkModeConfirmDeleteAll, width: 120}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.setup().overlayHintBar()
			if got == "" {
				t.Fatalf("overlayHintBar() returned empty for %s", tc.name)
			}
			// Both halves of the pair must appear, slash-grouped. Plain
			// "n" / "y" substring matches would be satisfied by "Enter"
			// or "cancel" alone, so we lock in the slash form (or its
			// reverse) instead.
			confirm := strings.Contains(got, "Enter/y") || strings.Contains(got, "y/Enter")
			cancel := strings.Contains(got, "Esc/n") || strings.Contains(got, "n/Esc")
			if !confirm {
				t.Errorf("overlayHintBar() for %s missing Enter/y pair in %q", tc.name, got)
			}
			if !cancel {
				t.Errorf("overlayHintBar() for %s missing Esc/n pair in %q", tc.name, got)
			}
		})
	}
}

// TestStatusBar_ShowsOverlayHints verifies the status bar uses overlay hints when an overlay is active.
func TestStatusBar_ShowsOverlayHints(t *testing.T) {
	m := Model{
		overlay: overlayNamespace,
		width:   120,
		height:  40,
	}
	bar := m.statusBar()
	// The bar should contain namespace overlay hints, not explorer hints.
	if !strings.Contains(bar, "esc") {
		t.Error("status bar with overlay active should show overlay hints")
	}
	// Should NOT contain explorer-only hints like "navigate".
	if strings.Contains(bar, "h/l") {
		t.Error("status bar with overlay active should not show explorer hints")
	}
}

func TestCovRenderHints(t *testing.T) {
	m := baseModelCov()
	hints := []ui.HintEntry{
		{Key: "j/k", Desc: "navigate"},
		{Key: "q", Desc: "quit"},
	}
	result := m.renderHints(hints)
	assert.NotEmpty(t, result)
}

// --- Right-sizing hint bar (strategy + headroom pickers) ---

// rightsizingHintBarModel returns a Model wired into a sensible
// loaded state for the right-sizing hint bar tests.
func rightsizingHintBarModel(available []string) Model {
	avail := make([]model.RightsizingStrategy, 0, len(available))
	for _, s := range available {
		avail = append(avail, model.RightsizingStrategy(s))
	}
	m := Model{
		overlay: overlayRightsizing,
		width:   120,
		height:  40,
	}
	m.rightsizing.available = avail
	m.rightsizing.headroom = model.DefaultRightsizingHeadroom
	m.rightsizing.data = &model.Rightsizing{
		Strategy:            avail[0],
		AvailableStrategies: avail,
		Headroom:            model.DefaultRightsizingHeadroom,
		Containers:          []model.ContainerRec{{Name: "app"}},
	}
	return m
}

func TestOverlayHintBarOverlayRightsizing_HasHeadroomCycle(t *testing.T) {
	// </> headroom cycle is always advertised — RightsizingHeadrooms
	// is a fixed 6-entry list, so the cycle never disappears.
	m := rightsizingHintBarModel([]string{"vpa", "snapshot"})
	got := m.overlayHintBar()
	assert.Contains(t, got, "</>", "headroom cycle key should appear")
	assert.Contains(t, got, "headroom", "headroom hint should label the chord")
}

func TestOverlayHintBarOverlayRightsizing_HasStrategyCycleWhenMultiAvailable(t *testing.T) {
	// [/] strategy cycle appears only when more than one strategy is
	// available (otherwise the cycle is a no-op and the hint is a lie).
	m := rightsizingHintBarModel([]string{"vpa", "snapshot"})
	got := m.overlayHintBar()
	assert.Contains(t, got, "[/]", "strategy cycle key should appear when multiple strategies are available")
	assert.Contains(t, got, "strategy", "strategy hint should label the chord")
}

func TestOverlayHintBarOverlayRightsizing_HidesStrategyCycleWhenSingleAvailable(t *testing.T) {
	// With only one strategy, [/] would be a no-op so the hint is
	// suppressed. Headroom hint stays — the value cycle still works.
	m := rightsizingHintBarModel([]string{"snapshot"})
	got := m.overlayHintBar()
	assert.NotContains(t, got, "[/]: strategy", "strategy cycle hint should NOT appear when only one strategy is available")
	assert.Contains(t, got, "</>", "headroom cycle should still appear")
}
