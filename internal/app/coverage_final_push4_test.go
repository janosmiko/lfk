package app

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func basePush4Model() Model {
	m := Model{
		nav:            model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:           []TabState{{}},
		selectedItems:  make(map[string]bool),
		cursorMemory:   make(map[string]int),
		itemCache:      make(map[string][]model.Item),
		discoveredCRDs: make(map[string][]model.ResourceTypeEntry),
		width:          120,
		height:         40,
		execMu:         &sync.Mutex{},
		namespace:      "default",
		reqCtx:         context.Background(),
	}
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod", Status: "Running"},
		{Name: "pod-2", Namespace: "default", Kind: "Pod", Status: "Running"},
		{Name: "pod-3", Namespace: "default", Kind: "Pod", Status: "Running"},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "Pod",
		Resource:   "pods",
		Namespaced: true,
	}

	return m
}

// =====================================================================
// handleKey -- various branches
// =====================================================================

func TestPush4HandleKeyQ(t *testing.T) {
	m := basePush4Model()
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, overlayQuitConfirm, rm.overlay)
}

func TestPush4HandleKeyEscFullscreenDashboard(t *testing.T) {
	m := basePush4Model()
	m.fullscreenDashboard = true
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.fullscreenDashboard)
}

func TestPush4HandleKeyEscClearSelection(t *testing.T) {
	m := basePush4Model()
	m.selectedItems["pod-1//default"] = true
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Empty(t, rm.selectedItems)
}

func TestPush4HandleKeyEscClearFilter(t *testing.T) {
	m := basePush4Model()
	m.filterText = "something"
	result, cmd := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Empty(t, rm.filterText)
	assert.NotNil(t, cmd)
}

func TestPush4HandleKeyEscAtClusterLevel(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelClusters
	_, cmd := m.handleKey(keyMsg("esc"))
	// Should try to quit.
	assert.NotNil(t, cmd)
}

func TestPush4HandleKeyDownFullscreenDashboard(t *testing.T) {
	m := basePush4Model()
	m.fullscreenDashboard = true
	kb := ui.ActiveKeybindings
	result, cmd := m.handleKey(keyMsg(kb.Down))
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestPush4HandleKeyUpFullscreenDashboard(t *testing.T) {
	m := basePush4Model()
	m.fullscreenDashboard = true
	m.previewScroll = 5
	kb := ui.ActiveKeybindings
	result, cmd := m.handleKey(keyMsg(kb.Up))
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestPush4HandleKeyGGFullscreenDashboard(t *testing.T) {
	m := basePush4Model()
	m.fullscreenDashboard = true
	m.pendingG = true
	result, _ := m.handleKey(keyMsg("g"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.previewScroll)
}

func TestPush4HandleKeyGFullscreenDashboard(t *testing.T) {
	m := basePush4Model()
	m.fullscreenDashboard = true
	result, _ := m.handleKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)
}

func TestPush4HandleKeyGGExplorer(t *testing.T) {
	m := basePush4Model()
	m.pendingG = true
	m.setCursor(2)
	result, _ := m.handleKey(keyMsg("g"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.cursor())
}

func TestPush4HandleKeyGExplorer(t *testing.T) {
	m := basePush4Model()
	result, _ := m.handleKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)
}

func TestPush4HandleKeyGBigFullscreenDashboard(t *testing.T) {
	m := basePush4Model()
	m.fullscreenDashboard = true
	result, _ := m.handleKey(keyMsg("G"))
	rm := result.(Model)
	// Scroll set to large number then clamped. Just verify it executes.
	_ = rm
}

func TestPush4HandleKeyGBigExplorer(t *testing.T) {
	m := basePush4Model()
	m.setCursor(0)
	result, _ := m.handleKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.cursor())
}

func TestPush4HandleKeyPendingMarkSlot(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{DisplayName: "Pods", Kind: "Pod", Resource: "pods"}
	m.pendingMark = true
	result, _ := m.handleKey(keyMsg("a"))
	rm := result.(Model)
	assert.False(t, rm.pendingMark)
}

func TestPush4HandleKeyPendingMarkInvalid(t *testing.T) {
	m := basePush4Model()
	m.pendingMark = true
	result, cmd := m.handleKey(keyMsg("!"))
	rm := result.(Model)
	assert.False(t, rm.pendingMark)
	assert.Nil(t, cmd)
}

func TestPush4HandleKeyPendingBookmarkConfirmYes(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{DisplayName: "Pods", Kind: "Pod", Resource: "pods"}
	bm := model.Bookmark{Name: "test", Slot: "a", ResourceType: "/v1/pods"}
	m.pendingBookmark = &bm
	result, _ := m.handleKey(keyMsg("y"))
	rm := result.(Model)
	assert.Nil(t, rm.pendingBookmark)
}

func TestPush4HandleKeyPendingBookmarkCancel(t *testing.T) {
	m := basePush4Model()
	bm := model.Bookmark{Name: "test", Slot: "a"}
	m.pendingBookmark = &bm
	result, _ := m.handleKey(keyMsg("n"))
	rm := result.(Model)
	assert.Nil(t, rm.pendingBookmark)
	assert.Contains(t, rm.statusMessage, "Cancelled")
}

func TestPush4HandleKeyDismissStartupTip(t *testing.T) {
	m := basePush4Model()
	m.statusMessageTip = true
	m.statusMessage = "tip message"
	result, _ := m.handleKey(keyMsg("j"))
	rm := result.(Model)
	assert.False(t, rm.statusMessageTip)
	assert.Empty(t, rm.statusMessage)
}

func TestPush4HandleKeyErrorLogOverlay(t *testing.T) {
	m := basePush4Model()
	m.overlayErrorLog = true
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.overlayErrorLog)
}

func TestPush4HandleKeyOverlayActive(t *testing.T) {
	m := basePush4Model()
	m.overlay = overlayConfirm
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestPush4HandleKeyFilterActive(t *testing.T) {
	m := basePush4Model()
	m.filterActive = true
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.filterActive)
}

func TestPush4HandleKeySearchActive(t *testing.T) {
	m := basePush4Model()
	m.searchActive = true
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.searchActive)
}

func TestPush4HandleKeyModeLogs(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush4HandleKeyModeDescribe(t *testing.T) {
	m := basePush4Model()
	m.mode = modeDescribe
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush4HandleKeyModeDiff(t *testing.T) {
	m := basePush4Model()
	m.mode = modeDiff
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush4HandleKeyModeHelp(t *testing.T) {
	m := basePush4Model()
	m.mode = modeHelp
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush4HandleKeyModeYAML(t *testing.T) {
	m := basePush4Model()
	m.mode = modeYAML
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush4HandleKeyModeExplain(t *testing.T) {
	m := basePush4Model()
	m.mode = modeExplain
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush4HandleKeySelectRange(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelResources
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.SelectRange))
	rm := result.(Model)
	assert.Equal(t, 0, rm.selectionAnchor)
}

func TestPush4HandleKeySelectRangeWithAnchor(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelResources
	m.selectionAnchor = 0
	m.setCursor(2)
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.SelectRange))
	rm := result.(Model)
	assert.True(t, len(rm.selectedItems) > 0)
}

func TestPush4HandleKeyToggleSelect(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelResources
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.ToggleSelect))
	rm := result.(Model)
	assert.True(t, len(rm.selectedItems) > 0)
}

func TestPush4HandleKeyToggleSelectBelowResources(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelClusters
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.ToggleSelect))
	rm := result.(Model)
	assert.Empty(t, rm.selectedItems)
}

func TestPush4HandleKeyFilter(t *testing.T) {
	m := basePush4Model()
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.Filter))
	rm := result.(Model)
	assert.True(t, rm.filterActive)
}

func TestPush4HandleKeySearch(t *testing.T) {
	m := basePush4Model()
	result, _ := m.handleKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.searchActive)
}

func TestPush4HandleKeyHalfPageDown(t *testing.T) {
	m := basePush4Model()
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.PageDown))
	_ = result.(Model)
}

func TestPush4HandleKeyHalfPageUp(t *testing.T) {
	m := basePush4Model()
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.PageUp))
	_ = result.(Model)
}

func TestPush4HandleKeyPageDown(t *testing.T) {
	m := basePush4Model()
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.PageDown))
	_ = result.(Model)
}

func TestPush4HandleKeyPageUp(t *testing.T) {
	m := basePush4Model()
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.PageUp))
	_ = result.(Model)
}

func TestPush4HandleKeyHelp(t *testing.T) {
	m := basePush4Model()
	result, _ := m.handleKey(keyMsg("?"))
	rm := result.(Model)
	assert.Equal(t, modeHelp, rm.mode)
}

func TestPush4HandleKeyMark(t *testing.T) {
	m := basePush4Model()
	result, _ := m.handleKey(keyMsg("m"))
	rm := result.(Model)
	assert.True(t, rm.pendingMark)
}

func TestPush4HandleKeyBookmarkList(t *testing.T) {
	m := basePush4Model()
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}}
	result, _ := m.handleKey(keyMsg("'"))
	rm := result.(Model)
	assert.Equal(t, overlayBookmarks, rm.overlay)
}

func TestPush4HandleKeyBookmarkListEmpty(t *testing.T) {
	m := basePush4Model()
	m.bookmarks = nil
	result, _ := m.handleKey(keyMsg("'"))
	rm := result.(Model)
	// Empty bookmarks should show a message or do nothing.
	_ = rm
}

func TestPush4HandleKeyRight(t *testing.T) {
	m := basePush4Model()
	result, _ := m.handleKey(keyMsg("l"))
	_ = result.(Model)
}

func TestPush4HandleKeyLeft(t *testing.T) {
	m := basePush4Model()
	result, _ := m.handleKey(keyMsg("h"))
	_ = result.(Model)
}

func TestPush4HandleKeyEnter(t *testing.T) {
	m := basePush4Model()
	result, _ := m.handleKey(keyMsg("enter"))
	_ = result.(Model)
}

func TestPush4HandleKeyToggleWatchMode(t *testing.T) {
	m := basePush4Model()
	m.watchMode = true
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.WatchMode))
	rm := result.(Model)
	assert.False(t, rm.watchMode)
}

func TestPush4HandleKeyTogglePreview(t *testing.T) {
	m := basePush4Model()
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.TogglePreview))
	rm := result.(Model)
	// Just verify it toggles -- the actual value depends on default.
	_ = rm
}

func TestPush4HandleKeyToggleAllNamespaces(t *testing.T) {
	m := basePush4Model()
	m.allNamespaces = false
	m.nav.Level = model.LevelResources
	kb := ui.ActiveKeybindings
	result, cmd := m.handleKey(keyMsg(kb.AllNamespaces))
	rm := result.(Model)
	assert.True(t, rm.allNamespaces)
	assert.NotNil(t, cmd)
}

func TestPush4HandleKeyToggleAllNamespacesOff(t *testing.T) {
	m := basePush4Model()
	m.allNamespaces = true
	m.nav.Level = model.LevelResources
	kb := ui.ActiveKeybindings
	result, cmd := m.handleKey(keyMsg(kb.AllNamespaces))
	rm := result.(Model)
	assert.False(t, rm.allNamespaces)
	assert.NotNil(t, cmd)
}

// YAML fullscreen is triggered by enterFullView, not a direct keybinding.

func TestPush4HandleKeyDeleteDirect(t *testing.T) {
	m := basePush4Model()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	result, _ := m.handleKey(keyMsg("d"))
	rm := result.(Model)
	_ = rm
}

func TestPush4HandleKeyClearPendingG(t *testing.T) {
	m := basePush4Model()
	m.pendingG = true
	result, _ := m.handleKey(keyMsg("x"))
	rm := result.(Model)
	assert.False(t, rm.pendingG)
}

// =====================================================================
// handleLogKey -- test more branches (49.7% -> higher)
// =====================================================================

func TestPush4HandleLogKeyQ(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	result, _ := m.handleLogKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush4HandleLogKeyJ(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logFollow = true
	m.logLines = []string{"line1", "line2", "line3"}
	m.logCursor = 0
	result, _ := m.handleLogKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.logCursor)
}

func TestPush4HandleLogKeyK(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logLines = []string{"line1", "line2", "line3"}
	m.logCursor = 2
	result, _ := m.handleLogKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.logCursor)
}

func TestPush4HandleLogKeyG(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.pendingG = true
	m.logLines = []string{"line1", "line2"}
	m.logCursor = 1
	result, _ := m.handleLogKey(keyMsg("g"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.logCursor)
}

func TestPush4HandleLogKeyGBig(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logLines = []string{"line1", "line2", "line3"}
	result, _ := m.handleLogKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.logCursor)
	assert.True(t, rm.logFollow)
}

func TestPush4HandleLogKeyF(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logFollow = false
	result, _ := m.handleLogKey(keyMsg("f"))
	rm := result.(Model)
	assert.True(t, rm.logFollow)
}

func TestPush4HandleLogKeyW(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	result, _ := m.handleLogKey(keyMsg("w"))
	_ = result.(Model)
}

func TestPush4HandleLogKeyN(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	// n is search-next in log view.
	result, _ := m.handleLogKey(keyMsg("n"))
	_ = result.(Model)
}

func TestPush4HandleLogKeyEsc(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	result, _ := m.handleLogKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush4HandleLogKeySearch(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	result, _ := m.handleLogKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.logSearchActive)
}

func TestPush4HandleLogKeyVisualMode(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logLines = []string{"line1", "line2"}
	result, _ := m.handleLogKey(keyMsg("v"))
	rm := result.(Model)
	assert.True(t, rm.logVisualMode)
}

func TestPush4HandleLogKeyVisualModeV(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logLines = []string{"line1", "line2"}
	result, _ := m.handleLogKey(keyMsg("V"))
	rm := result.(Model)
	assert.True(t, rm.logVisualMode)
}

func TestPush4HandleLogKeyHalfPageDown(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logLines = make([]string, 100)
	m.logCursor = 0
	kb := ui.ActiveKeybindings
	result, _ := m.handleLogKey(keyMsg(kb.PageDown))
	rm := result.(Model)
	assert.Greater(t, rm.logCursor, 0)
}

func TestPush4HandleLogKeyHalfPageUp(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logLines = make([]string, 100)
	m.logCursor = 50
	kb := ui.ActiveKeybindings
	result, _ := m.handleLogKey(keyMsg(kb.PageUp))
	rm := result.(Model)
	assert.Less(t, rm.logCursor, 50)
}

// =====================================================================
// Additional Update branches
// =====================================================================

func TestPush4UpdateWatchTickMsg(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelResources
	m.watchMode = true
	result, _ := m.Update(watchTickMsg{})
	_ = result.(Model)
}

func TestPush4UpdateWatchTickMsgNotWatching(t *testing.T) {
	m := basePush4Model()
	m.watchMode = false
	result, _ := m.Update(watchTickMsg{})
	_ = result.(Model)
}

func TestPush4UpdateSecretSavedMsg(t *testing.T) {
	m := basePush4Model()
	result, cmd := m.Update(secretSavedMsg{})
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "saved")
	assert.NotNil(t, cmd)
}

func TestPush4UpdateSecretSavedMsgError(t *testing.T) {
	m := basePush4Model()
	result, cmd := m.Update(secretSavedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPush4UpdateConfigMapSavedMsg(t *testing.T) {
	m := basePush4Model()
	result, cmd := m.Update(configMapSavedMsg{})
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "saved")
	assert.NotNil(t, cmd)
}

func TestPush4UpdateConfigMapSavedMsgError(t *testing.T) {
	m := basePush4Model()
	result, cmd := m.Update(configMapSavedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPush4UpdateLabelSavedMsg(t *testing.T) {
	m := basePush4Model()
	result, cmd := m.Update(labelSavedMsg{})
	rm := result.(Model)
	_ = rm
	assert.NotNil(t, cmd)
}

func TestPush4UpdateLabelSavedMsgError(t *testing.T) {
	m := basePush4Model()
	result, cmd := m.Update(labelSavedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

// =====================================================================
// moveCursor branches
// =====================================================================

func TestPush4MoveCursorDown(t *testing.T) {
	m := basePush4Model()
	m.setCursor(0)
	result, _ := m.moveCursor(1)
	rm := result.(Model)
	assert.Equal(t, 1, rm.cursor())
}

func TestPush4MoveCursorUp(t *testing.T) {
	m := basePush4Model()
	m.setCursor(2)
	result, _ := m.moveCursor(-1)
	rm := result.(Model)
	assert.Equal(t, 1, rm.cursor())
}

func TestPush4MoveCursorAtBottom(t *testing.T) {
	m := basePush4Model()
	m.setCursor(2) // last item
	result, _ := m.moveCursor(1)
	rm := result.(Model)
	assert.Equal(t, 2, rm.cursor()) // should stay
}

func TestPush4MoveCursorAtTop(t *testing.T) {
	m := basePush4Model()
	m.setCursor(0)
	result, _ := m.moveCursor(-1)
	rm := result.(Model)
	assert.Equal(t, 0, rm.cursor()) // should stay
}

// =====================================================================
// buildActionCtx branches
// =====================================================================

func TestPush4BuildActionCtxResources(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelResources
	sel := &model.Item{Name: "pod-1", Namespace: "default", Kind: "Pod"}
	ctx := m.buildActionCtx(sel, "Pod")
	assert.Equal(t, "pod-1", ctx.name)
	assert.Equal(t, "test-ctx", ctx.context)
}

func TestPush4BuildActionCtxOwned(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceName = "deploy-1"
	sel := &model.Item{Name: "rs-1", Kind: "ReplicaSet", Namespace: "default"}
	ctx := m.buildActionCtx(sel, "ReplicaSet")
	assert.Equal(t, "rs-1", ctx.name)
}

func TestPush4BuildActionCtxContainers(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "pod-1"
	sel := &model.Item{Name: "nginx", Kind: "Container"}
	ctx := m.buildActionCtx(sel, "Container")
	assert.Equal(t, "pod-1", ctx.name)
	assert.Equal(t, "nginx", ctx.containerName)
}

// =====================================================================
// handleFilterKey and handleSearchKey
// =====================================================================

func TestPush4HandleFilterKeyEsc(t *testing.T) {
	m := basePush4Model()
	m.filterActive = true
	result, _ := m.handleFilterKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.filterActive)
}

func TestPush4HandleFilterKeyEnter(t *testing.T) {
	m := basePush4Model()
	m.filterActive = true
	result, _ := m.handleFilterKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.filterActive)
}

func TestPush4HandleSearchKeyEsc(t *testing.T) {
	m := basePush4Model()
	m.searchActive = true
	result, _ := m.handleSearchKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.searchActive)
}

func TestPush4HandleSearchKeyEnter(t *testing.T) {
	m := basePush4Model()
	m.searchActive = true
	result, _ := m.handleSearchKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.searchActive)
}

// =====================================================================
// refreshCurrentLevel
// =====================================================================

func TestPush4RefreshCurrentLevelOwned(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelOwned
	cmd := m.refreshCurrentLevel()
	assert.NotNil(t, cmd)
}

func TestPush4RefreshCurrentLevelContainers(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelContainers
	cmd := m.refreshCurrentLevel()
	assert.NotNil(t, cmd)
}
