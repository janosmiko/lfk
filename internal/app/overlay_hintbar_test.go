package app

import (
	"strings"
	"testing"

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
