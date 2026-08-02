package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// Issue #398: wheel over the right (preview) pane runs clampPreviewScroll in
// the Update path, which renders right-pane content (cursor=-1). Those renders
// must not touch the persistent pane-scroll globals the View path maintains —
// RenderTable/RenderColumn write ActiveMiddleScroll/ActiveLeftScroll
// unconditionally, and VimScrollOff with cursor=-1 returns 0, so an unguarded
// render zeroes the left/middle viewport and the pane visibly jumps.

// saveScrollGlobals pins the scroll globals to known values and restores the
// originals on cleanup.
func saveScrollGlobals(t *testing.T, middle, left int) {
	t.Helper()
	prevMiddle, prevLeft := ui.ActiveMiddleScroll, ui.ActiveLeftScroll
	prevLineMap := ui.ActiveMiddleLineMap
	t.Cleanup(func() {
		ui.ActiveMiddleScroll, ui.ActiveLeftScroll = prevMiddle, prevLeft
		ui.ActiveMiddleLineMap = prevLineMap
	})
	ui.ActiveMiddleScroll, ui.ActiveLeftScroll = middle, left
}

// Wheel over the right pane at LevelResourceTypes: the (memoized) measure
// render of the preview table must not clobber the middle/left scroll state.
func TestWheelOverRightPaneKeepsScrollGlobalsMeasurePath(t *testing.T) {
	saveScrollGlobals(t, 7, 4)

	m := baseExplorerModel()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{
		{Name: "Pods", Kind: "ResourceType", Extra: "/v1/pods"},
	}
	m.setCursor(0)
	m.rightItems = []model.Item{
		{Name: "pod-a", Kind: "Pod", Namespace: "default"},
		{Name: "pod-b", Kind: "Pod", Namespace: "default"},
	}

	// X=100 is the right pane (>= middleEnd=77 at width 120).
	_, _ = m.handleMouse(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 100})

	assert.Equal(t, 7, ui.ActiveMiddleScroll, "measure render must not clobber the middle pane scroll")
	assert.Equal(t, 4, ui.ActiveLeftScroll, "measure render must not clobber the left pane scroll")
}

// Wheel over the right pane at LevelResources with a split preview: the
// pinned-header table render inside clampPreviewScroll runs on every tick
// (not memoized) and must not clobber the scroll state or the click line map.
func TestWheelOverRightPaneKeepsScrollGlobalsSplitPreview(t *testing.T) {
	saveScrollGlobals(t, 7, 4)
	ui.ActiveMiddleLineMap = []int{0, 1, 2}

	m := baseExplorerModel() // LevelResources, Kind "Pod" -> hasSplitPreview
	m.rightItems = []model.Item{
		{Name: "container-a", Kind: "Container"},
	}

	_, _ = m.handleMouse(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 100})

	assert.Equal(t, 7, ui.ActiveMiddleScroll, "split-preview render must not clobber the middle pane scroll")
	assert.Equal(t, 4, ui.ActiveLeftScroll, "split-preview render must not clobber the left pane scroll")
	assert.Equal(t, []int{0, 1, 2}, ui.ActiveMiddleLineMap, "split-preview render must not rebuild the middle click map from right-pane items")
}

// The preview J/K keys clamp through the same path as the wheel; guard them too.
func TestClampPreviewScrollKeepsScrollGlobals(t *testing.T) {
	saveScrollGlobals(t, 9, 2)

	m := baseExplorerModel()
	m.rightItems = []model.Item{
		{Name: "container-a", Kind: "Container"},
	}
	m.previewScroll = 50
	m.clampPreviewScroll()

	assert.Equal(t, 9, ui.ActiveMiddleScroll, "clamp must not clobber the middle pane scroll")
	assert.Equal(t, 2, ui.ActiveLeftScroll, "clamp must not clobber the left pane scroll")
}
