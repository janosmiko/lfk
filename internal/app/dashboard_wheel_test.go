package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// Issue #524 follow-up: the fullscreen (pinned) cluster dashboard reuses
// previewScroll for its own content and has no right pane, so the mouse wheel
// must scroll the dashboard, mirroring the j/k keys. Before the fix the
// collapsed column boundaries routed the wheel to moveCursor, which moved the
// hidden middle-list cursor instead of scrolling and left the dashboard frozen.

func dashboardModel(preview string) Model {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{
		{Name: "Cluster Dashboard", Kind: "__dashboard__", Extra: "__overview__"},
		{Name: "Pods", Kind: "ResourceType"},
		{Name: "Services", Kind: "ResourceType"},
	}
	m.setCursor(0)
	m.fullscreenDashboard = true
	m.dashboardPreview = preview
	return m
}

func TestDashboardWheelScrollsContentNotCursor(t *testing.T) {
	m := dashboardModel(strings.Repeat("dashboard line\n", 200))
	startCursor := m.cursor()

	mdl, _ := m.handleMouse(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 60})
	m = mdl.(Model)

	assert.Equal(t, startCursor, m.cursor(), "wheel in the fullscreen dashboard must not move the middle cursor")
	assert.Positive(t, m.previewScroll, "wheel down in the fullscreen dashboard must scroll the content")
}

// The reported symptom is scrolling up past the top. Wheel-up at the top must
// clamp at 0 and never move the underlying cursor.
func TestDashboardWheelUpClampsAtTop(t *testing.T) {
	m := dashboardModel(strings.Repeat("dashboard line\n", 200))
	startCursor := m.cursor()

	for range 5 {
		mdl, _ := m.handleMouse(tea.MouseWheelMsg{Button: tea.MouseWheelUp, X: 60})
		m = mdl.(Model)
	}

	assert.Equal(t, 0, m.previewScroll, "wheel up at the top of the dashboard must clamp at 0")
	assert.Equal(t, startCursor, m.cursor(), "wheel up at the top must not move the middle cursor")
}

// The monitoring dashboard shares the fullscreen path; the wheel scrolls its
// preview the same way.
func TestMonitoringDashboardWheelScrolls(t *testing.T) {
	m := dashboardModel("")
	m.middleItems[0] = model.Item{Name: "Monitoring", Kind: "__dashboard__", Extra: "__monitoring__"}
	m.monitoringPreview = strings.Repeat("metric line\n", 200)

	mdl, _ := m.handleMouse(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 60})
	m = mdl.(Model)

	assert.Positive(t, m.previewScroll, "wheel down in the monitoring dashboard must scroll the content")
}

// Scrolling a Service's details (right) pane must not move the parent
// resource-kinds list on the left (the render-clobber class of #398/#524).
func TestServiceDetailsScrollNoParentJump(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{DisplayName: "Services", Kind: "Service", Resource: "services"}
	left := make([]model.Item, 30)
	for i := range left {
		left[i] = model.Item{Name: "kind-" + string(rune('a'+i%26)), Kind: "ResourceType"}
	}
	left[25] = model.Item{Name: "Services", Kind: "ResourceType"}
	m.leftItems = left
	m.middleItems = []model.Item{{Name: "svc-a", Kind: "Service"}, {Name: "svc-b", Kind: "Service"}}
	m.setCursor(0)
	m.rightItems = []model.Item{{Name: "pod-1", Kind: "Pod"}, {Name: "pod-2", Kind: "Pod"}}
	m.previewYAML = strings.Repeat("field: value\n", 200)

	_ = m.View()
	leftBefore := ui.ActiveLeftScroll

	for range 5 {
		mdl, _ := m.handleMouse(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 110})
		m = mdl.(Model)
		_ = m.View()
	}

	assert.Equal(t, leftBefore, ui.ActiveLeftScroll, "scrolling the service details pane must not move the parent list")
}
