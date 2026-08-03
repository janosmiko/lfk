package app

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// handleMouseToggleKey intercepts the configured mouse-capture toggle key.
// Returns handled=false when the key doesn't match or a viewer text-input
// sub-mode owns the keystroke, so the keystroke falls through unchanged.
func (m Model) handleMouseToggleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	kb := ui.ActiveKeybindings
	if kb.MouseToggle == "" || msg.String() != kb.MouseToggle {
		return m, nil, false
	}
	// Never steal the key while a viewer search/filter input is focused.
	if m.yamlView.searchMode || m.logView.searchActive || m.helpSearchActive ||
		m.explainSearchActive || m.diffView.searchMode || m.describeView.searchActive ||
		m.helpFilterActive {
		return m, nil, false
	}
	mdl, cmd := m.toggleMouseCapture()
	return mdl, cmd, true
}

// toggleMouseCapture suspends or resumes mouse reporting at runtime. When
// suspended the terminal regains the mouse, so the user can select text,
// copy, and use native scrollback; resuming re-enables cell-motion capture
// (the same mode main.go installs at startup). It is a no-op with an
// explanatory message when mouse capture was never available.
func (m Model) toggleMouseCapture() (tea.Model, tea.Cmd) {
	if !m.mouseAvailable {
		m.setStatusMessage("Mouse capture is disabled (started with --no-mouse or config)", true)
		return m, scheduleStatusClear()
	}
	m.mouseCaptured = !m.mouseCaptured
	if m.mouseCaptured {
		m.setStatusMessage("Mouse capture ON", false)
		return m, scheduleStatusClear()
	}
	m.setStatusMessage("Mouse capture OFF — select text natively; press "+
		ui.ActiveKeybindings.MouseToggle+" to re-enable", false)
	return m, scheduleStatusClear()
}

// isMousePress reports whether msg is a button press. Bubble Tea v2 encodes
// the action in the message type rather than in a MouseMsg.Action field, so
// press/release is a type check now.
func isMousePress(msg tea.MouseMsg) bool {
	_, ok := msg.(tea.MouseClickMsg)
	return ok
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	mouse := msg.Mouse()
	// Trackpad momentum keeps emitting wheel ticks after the physical gesture
	// ends. Drop the tail once the burst is no longer productive (a boundary
	// was reached, the pointer moved to another pane, or a left/right
	// navigation happened) so the queued ticks don't "play out" on whatever
	// list is now under the pointer (#524).
	if mouse.Button == tea.MouseWheelUp || mouse.Button == tea.MouseWheelDown {
		next, drop := m.beginWheelTick(mouse.X)
		if drop {
			return next, nil
		}
		m = next
	}

	// Mouse wheel inside the embedded PTY pane scrolls the scrollback
	// ring (when present). One line per tick matches what most native
	// terminals do for their own scrollback. We only intercept the
	// wheel — clicks and other mouse input fall through so tab-bar
	// clicks and host-terminal selection (shift+drag) keep working.
	if m.mode == modeExec {
		switch mouse.Button {
		case tea.MouseWheelUp:
			return m.execScrollBy(-1), nil
		case tea.MouseWheelDown:
			return m.execScrollBy(1), nil
		}
		// Fall through for non-wheel mouse events (tab-bar clicks etc.)
	}

	// Handle mouse scroll in log viewer mode.
	if m.mode == modeLogs {
		return m.handleLogsMouse(msg)
	}

	// Object Explorer wheel: route by the pointer column. Over the right
	// (preview) pane the wheel scrolls the YAML preview; over the left and
	// middle panes it moves the tree cursor — mirroring the main explorer's
	// per-pane wheel routing (#379). Non-wheel mouse falls through so
	// tab-bar clicks keep working.
	if m.mode == modeObjectExplorer {
		switch mouse.Button {
		case tea.MouseWheelUp:
			return m.handleObjectExplorerWheel(mouse.X, -1)
		case tea.MouseWheelDown:
			return m.handleObjectExplorerWheel(mouse.X, 1)
		}
	}

	// Wheel scroll in the other full-screen viewer modes (YAML, Describe,
	// Diff, Help, Explain). Synthesize 3 j/k key presses per tick so the
	// existing per-mode scroll logic — cursor advance, ensure-visible,
	// clamps, page-X tracking, sub-mode dispatch — runs unchanged.
	// Other mouse buttons fall through so tab-bar clicks still work.
	if isViewerMode(m.mode) {
		switch mouse.Button {
		case tea.MouseWheelUp:
			return m.dispatchWheelKey("k")
		case tea.MouseWheelDown:
			return m.dispatchWheelKey("j")
		}
	}

	// Handle tab bar clicks in any mode — but only when no centered
	// overlay is open. A modal overlay covers the rest of the screen
	// and owns mouse input; without this guard a click on row 1 would
	// switch tabs underneath the modal.
	if m.overlay == overlayNone && len(m.tabs) > 1 && mouse.Button == tea.MouseLeft && isMousePress(msg) && mouse.Y == 1 {
		if tab := m.tabAtX(mouse.X); tab >= 0 && tab != m.activeTab {
			return m.switchToTab(tab)
		}
		return m, nil
	}

	// Don't handle mouse outside the explorer mode.
	if m.mode != modeExplorer {
		return m, nil
	}

	// Overlay-aware mouse: click outside a centered overlay dismisses it,
	// click inside (for supported overlays) activates the row under the
	// click. Wheel and other buttons fall through unchanged when no
	// overlay is active.
	if m.overlay != overlayNone {
		return m.handleOverlayMouse(msg)
	}

	switch mouse.Button {
	case tea.MouseWheelUp:
		return m.handleExplorerWheel(mouse.X, -3)
	case tea.MouseWheelDown:
		return m.handleExplorerWheel(mouse.X, 3)
	case tea.MouseLeft:
		if !isMousePress(msg) {
			return m, nil
		}
		return m.handleMouseClick(mouse.X, mouse.Y)
	case tea.MouseRight:
		if !isMousePress(msg) {
			return m, nil
		}
		return m.handleMouseRightClick(mouse.X, mouse.Y)
	}
	return m, nil
}

// wheelQuietGap is how long the wheel must be silent before a tick counts as a
// brand-new gesture. It must exceed the largest gap between two ticks of a
// decelerating trackpad-momentum stream (its sparse tail can be 150-300ms
// apart), so the whole tail of a voided gesture stays suppressed until the
// momentum actually stops — not just its dense head. Too small and the tail
// revives on the wrong list (the #524 "jumps by 3 after navigating" case); too
// large and a deliberate re-scroll right after navigating feels laggy.
const wheelQuietGap = 350 * time.Millisecond

// beginWheelTick advances the wheel-burst state machine for a wheel tick at
// pointer column x and reports whether the tick should be dropped. After
// wheelQuietGap of silence the tick starts a brand-new gesture and always
// scrolls; otherwise it is dropped once the gesture has been marked dead (a
// boundary was reached or a navigation happened) or when the pointer has moved
// to a different target than the one the gesture started on. Dropped ticks keep
// the gesture alive (lastAt advances), so a continuous momentum tail stays
// suppressed until it truly stops.
func (m Model) beginWheelTick(x int) (Model, bool) {
	now := time.Now()
	prev := m.wheel.lastAt
	m.wheel.lastAt = now
	target := m.wheelTargetID(x)

	if prev.IsZero() || now.Sub(prev) > wheelQuietGap {
		m.wheel.target = target
		m.wheel.dead = false
		return m, false
	}
	if m.wheel.dead || target != m.wheel.target {
		m.wheel.dead = true // keep suppressing the rest of the tail
		return m, true
	}
	return m, false
}

// wheelTargetID names what a wheel tick at pointer column x would currently
// scroll. Its only job is to detect when the pointer (or the screen) has moved
// off the target a momentum burst started on; the exact strings are internal.
// It mirrors the routing in handleMouse so a burst is bound to one scroll
// target.
func (m Model) wheelTargetID(x int) string {
	switch {
	case m.mode == modeExec:
		return "exec"
	case m.mode == modeLogs:
		if logW, previewW := splitLogPreviewWidth(m.width); m.logView.previewVisible && previewW > 0 && x >= logW {
			return "logs-preview"
		}
		return "logs"
	case m.mode == modeObjectExplorer:
		if x >= m.objectExplorerRightPaneStart() {
			return "oe-preview"
		}
		return "oe-tree"
	case isViewerMode(m.mode):
		return "viewer"
	case m.mode != modeExplorer:
		return "other"
	case m.overlay != overlayNone:
		return "overlay"
	case m.fullscreenDashboard:
		return "dash"
	}
	if _, middleEnd := m.columnBoundaries(); x >= middleEnd {
		if m.fullLogPreview {
			return "ex-logpreview"
		}
		return "ex-preview"
	}
	return "ex-cursor"
}

// handleExplorerWheel routes a wheel tick to the pane under the pointer
// at x. The right pane scrolls its preview content; the left and middle
// panes move the row cursor — the whole-window behaviour the wheel had
// before per-pane routing existed (issue #319 b). delta is the signed
// line count (negative scrolls up, positive scrolls down).
//
// In fullscreen, columnBoundaries collapses to (0, m.width) so x can
// never reach the right pane and the wheel keeps moving the cursor,
// preserving the prior fullscreen behaviour.
func (m Model) handleExplorerWheel(x, delta int) (tea.Model, tea.Cmd) {
	// The fullscreen dashboard has no right pane and reuses previewScroll for
	// its own content, so route the wheel straight to the dashboard scroll
	// (mirroring the j/k keys). Without this the collapsed columnBoundaries
	// (0, m.width) make x never reach a right pane, so the wheel would move the
	// hidden middle-list cursor instead of scrolling the dashboard (#524).
	if m.fullscreenDashboard {
		before := m.previewScroll
		m.previewScroll += delta
		m.clampDashboardScroll()
		m.markWheelBurstDeadIfClamped(before, m.previewScroll)
		return m, nil
	}
	_, middleEnd := m.columnBoundaries()
	if x >= middleEnd {
		// Live-log preview uses a bottom-anchored offset (fromBottom): wheel up
		// (delta<0) scrolls into older lines, wheel down (delta>0) toward the
		// newest. Subtracting delta maps the shared scroll direction onto it.
		if m.fullLogPreview {
			before := m.previewLog.fromBottom
			m.previewLog.fromBottom -= delta
			if m.previewLog.fromBottom < 0 {
				m.previewLog.fromBottom = 0
			}
			m.clampPreviewScroll()
			m.markWheelBurstDeadIfClamped(before, m.previewLog.fromBottom)
			// Trigger lazy history loading when scrolling up (delta<0) and at top.
			if delta < 0 {
				m, cmd := m.maybeLoadMorePreviewHistory()
				return m, cmd
			}
			return m, nil
		}
		// Right pane: scroll the preview, mirroring the PreviewUp/PreviewDown
		// keys (J/K). Scroll-up is a plain decrement with a zero floor;
		// scroll-down increments and clamps to the rendered content height.
		before := m.previewScroll
		m.previewScroll += delta
		if delta < 0 {
			if m.previewScroll < 0 {
				m.previewScroll = 0
			}
		} else {
			m.clampPreviewScroll()
		}
		m.markWheelBurstDeadIfClamped(before, m.previewScroll)
		return m, nil
	}
	// Left and middle panes: move the selection cursor. Reaching the top or
	// bottom of the list empties the momentum queue so the tail can't keep
	// firing once there's nothing left to scroll (#524).
	mdl, cmd := m.moveCursor(delta)
	m = mdl.(Model)
	if visible := len(m.visibleMiddleItems()); visible == 0 ||
		(delta < 0 && m.cursor() == 0) || (delta > 0 && m.cursor() == visible-1) {
		m.wheel.dead = true
	}
	return m, cmd
}

// markWheelBurstDeadIfClamped ends the current wheel burst when a scroll tick
// produced no movement — the content is already at the top or bottom, so the
// rest of a momentum burst has nothing left to do and must not leak onto
// another target (#524).
func (m *Model) markWheelBurstDeadIfClamped(before, after int) {
	if before == after {
		m.wheel.dead = true
	}
}

// handleObjectExplorerWheel routes a wheel tick in the Object Explorer to
// the pane under the pointer at x. Over the right (preview) pane it scrolls
// the YAML preview, mirroring the J/K keys; over the left and middle panes
// it moves the tree cursor, mirroring j/k. dir is -1 (up) or +1 (down);
// each tick moves wheelStep lines to match the other viewers' wheel feel.
func (m Model) handleObjectExplorerWheel(x, dir int) (tea.Model, tea.Cmd) {
	const wheelStep = 3
	rt := &m.objectExplorerView
	if x >= m.objectExplorerRightPaneStart() {
		// Scroll-up is a plain decrement with a zero floor; scroll-down
		// increments and clamps to the preview content height (which
		// re-marshals the node YAML, so only do it when scrolling down).
		before := rt.previewScroll
		rt.previewScroll += dir * wheelStep
		if dir < 0 {
			if rt.previewScroll < 0 {
				rt.previewScroll = 0
			}
		} else {
			m.clampObjectExplorerPreviewScroll()
		}
		m.markWheelBurstDeadIfClamped(before, rt.previewScroll)
		return m, nil
	}
	before := rt.cursor
	m.moveObjectExplorerCursor(dir * wheelStep)
	m.clampObjectExplorerScroll()
	m.markWheelBurstDeadIfClamped(before, rt.cursor)
	return m, nil
}

// handleLogsMouse routes a mouse event in the log viewer. The wheel scrolls
// the pane under the pointer: over the preview side panel it scrolls the
// structured preview (mirroring the J/K keys); over the log stream it scrolls
// the log — the same per-pane routing as the explorers (#379). Non-wheel
// mouse is a no-op.
func (m Model) handleLogsMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	mouse := msg.Mouse()
	if logW, previewW := splitLogPreviewWidth(m.width); m.logView.previewVisible && previewW > 0 && mouse.X >= logW {
		switch mouse.Button {
		case tea.MouseWheelUp:
			return m.scrollLogPreviewWheel(-1), nil
		case tea.MouseWheelDown:
			return m.scrollLogPreviewWheel(1), nil
		}
	}
	switch mouse.Button {
	case tea.MouseWheelUp:
		m.logView.follow = false
		if m.logView.scroll > 0 {
			m.logView.scroll -= 3
			if m.logView.scroll < 0 {
				m.logView.scroll = 0
			}
		}
		return m, m.maybeLoadMoreHistory()
	case tea.MouseWheelDown:
		m.logView.follow = false
		m.logView.scroll += 3
		m.clampLogScroll()
	}
	return m, nil
}

// scrollLogPreviewWheel scrolls the log preview side panel by wheelStep rows
// in dir (-1 up, +1 down), reusing the J/K key handlers so the clamping math
// lives in one place. Callers gate on previewVisible and a non-zero panel
// width before calling.
func (m Model) scrollLogPreviewWheel(dir int) Model {
	const wheelStep = 3
	for range wheelStep {
		if dir < 0 {
			m = m.handleLogKeyK2()
		} else {
			m = m.handleLogKeyJ2()
		}
	}
	return m
}

// objectExplorerRightPaneStart returns the screen x at which the Object
// Explorer's right (preview) pane begins. It mirrors the column math in
// ui.RenderObjectExplorerView: an outer frame border (1 cell) precedes the
// left and middle columns, each of which adds 2 border cells around its
// content width (padding is folded into the width).
func (m Model) objectExplorerRightPaneStart() int {
	usable := m.width - 8
	leftW := max(10, usable*12/100)
	middleW := max(10, usable*51/100)
	return 1 + (leftW + 2) + (middleW + 2)
}

// columnBoundaries returns the x boundaries between left/middle and
// middle/right columns. Must match viewExplorer's layout math.
//
// viewExplorer subtracts only the 2-char border for each of the three
// columns from m.width to compute `usable`, then splits `usable` into
// leftW / middleW / rightW. ActiveColumnStyle has Padding(0, 1) which
// lipgloss folds INTO the .Width(W) value, so leftW already includes
// padding and the only extra cells on screen are the 2 border chars
// per column. Adding 4 here would make each boundary 2 cells too wide
// and misclassify clicks landing on the inner padding next to the
// pane separators.
func (m Model) columnBoundaries() (leftEnd, middleEnd int) {
	if m.fullscreenMiddle || m.fullscreenDashboard {
		// Fullscreen: only middle column exists.
		return 0, m.width
	}
	// Single source of truth for widths; hideLeftPane returns leftW=0
	// so the left band collapses and the middle band starts at x=0.
	leftW, middleW, _ := m.explorerColumnWidths()
	leftEnd = leftW + 2
	if leftW == 0 {
		leftEnd = 0
	}
	middleEnd = leftEnd + middleW + 2
	return leftEnd, middleEnd
}

// isMiddleTableLevel reports whether the current navigation level renders
// the middle column as a table (with a header row above the items) versus
// as a plain column with a single label header.
func (m Model) isMiddleTableLevel() bool {
	switch m.nav.Level {
	case model.LevelResources, model.LevelOwned, model.LevelContainers:
		return true
	}
	return false
}

// resolveMiddleColumnClick maps a click at (x, y) within the middle-column
// band to a target item index. Pass leftEnd as returned by columnBoundaries.
//
// Returns:
//   - idx >= 0: clickable item index inside m.visibleMiddleItems().
//   - isHeader == true: the click landed on the table header row (only
//     possible at table levels). relX is set to the X offset relative to
//     the middle-column content origin so handleHeaderClick can dispatch
//     to the right sort column.
//   - all zero / idx == -1: separator, beyond items, or column-view header
//     row — caller should treat as no-op.
func (m Model) resolveMiddleColumnClick(x, y, leftEnd int) (idx int, isHeader bool, relX int) {
	baseOffset := 2 // title bar (1) + top border (1)
	if len(m.tabs) > 1 {
		baseOffset = 3 // title bar (1) + tab bar (1) + top border (1)
	}
	itemY := y - baseOffset

	if m.isMiddleTableLevel() {
		itemY-- // subtract table header row
		if itemY < 0 {
			// Header row click — caller should sort.
			r := x - 2 // border + padding
			if !m.fullscreenMiddle && !m.fullscreenDashboard {
				r = x - leftEnd - 2
			}
			return -1, true, r
		}
	} else {
		// Column view has a single header line above the items.
		itemY-- // subtract column header
	}

	if itemY < 0 || itemY >= len(ui.ActiveMiddleLineMap) {
		return -1, false, 0
	}
	targetIdx := ui.ActiveMiddleLineMap[itemY]
	if targetIdx < 0 || targetIdx >= len(m.visibleMiddleItems()) {
		return -1, false, 0
	}
	return targetIdx, false, 0
}

func (m Model) handleMouseClick(x, y int) (tea.Model, tea.Cmd) {
	// Title bar (y=0) has its own clickable regions (namespace badge,
	// future: read-only toggle, watch indicator). Handle it first so a
	// click on the badge doesn't accidentally fall through to the
	// table-header sort path.
	if y == 0 {
		if mdl, cmd, ok := m.handleTitleBarClick(x); ok {
			return mdl, cmd
		}
		return m, nil
	}

	leftEnd, middleEnd := m.columnBoundaries()

	switch {
	case x < leftEnd:
		// Left column click: navigate parent.
		return m.navigateParent()
	case x < middleEnd:
		targetIdx, isHeader, relX := m.resolveMiddleColumnClick(x, y, leftEnd)
		if isHeader {
			return m.handleHeaderClick(relX)
		}
		if targetIdx < 0 {
			return m, nil
		}
		// Click on the row already under the cursor drills into it,
		// matching Enter / right-arrow. First click on a different row
		// just selects + previews so the user can scan items in the
		// right pane without committing.
		if targetIdx == m.cursor() {
			return m.navigateChild()
		}
		m.setCursor(targetIdx)
		if !m.isMiddleTableLevel() {
			m.syncExpandedGroup()
		}
		return m, m.loadPreview()
	default:
		// Right column click: navigate child.
		return m.navigateChild()
	}
}

// handleMouseRightClick dispatches a right-button press to the action menu.
// Right-click on the middle column moves the cursor to the clicked row
// before opening the menu so the action targets the row that was clicked,
// matching standard GUI context-menu behavior. Right-click on the right
// pane opens the menu for the currently-selected item (no cursor change).
// Right-click on the left pane is a no-op (no resource context).
func (m Model) handleMouseRightClick(x, y int) (tea.Model, tea.Cmd) {
	// Title bar right-click currently has no action — suppress it so a
	// right-click on the namespace badge doesn't accidentally open the
	// action menu via the right-pane fallback below.
	if y == 0 {
		return m, nil
	}

	leftEnd, middleEnd := m.columnBoundaries()

	switch {
	case x < leftEnd:
		return m, nil
	case x < middleEnd:
		targetIdx, isHeader, _ := m.resolveMiddleColumnClick(x, y, leftEnd)
		if isHeader || targetIdx < 0 {
			return m, nil
		}
		cursorMoved := targetIdx != m.cursor()
		if cursorMoved {
			m.setCursor(targetIdx)
			if !m.isMiddleTableLevel() {
				m.syncExpandedGroup()
			}
		}
		mdl := m.openActionMenu()
		if cursorMoved {
			// Refresh preview so the right pane matches the new cursor
			// once the menu is dismissed.
			return mdl, mdl.loadPreview()
		}
		return mdl, nil
	default:
		return m.openActionMenu(), nil
	}
}

// findSortableCol returns the index of name in ActiveSortableColumns, or -1.
func findSortableCol(name string) int {
	for i, c := range ui.ActiveSortableColumns {
		if c == name {
			return i
		}
	}
	return -1
}

// handleHeaderClick sorts the table by the column that was clicked in the header row.
// relX is the click position relative to the start of the middle column content area.
// It consumes the ActiveMiddleColumnLayout populated by RenderTable so the mapping
// from click X to column key always matches the actual rendered order, even when
// the user has reordered columns via the column-toggle overlay.
func (m Model) handleHeaderClick(relX int) (tea.Model, tea.Cmd) {
	if !m.sortApplies() {
		return m, nil
	}
	items := m.visibleMiddleItems()
	if len(items) == 0 || len(ui.ActiveSortableColumns) == 0 || len(ui.ActiveMiddleColumnLayout) == 0 {
		return m, nil
	}

	// Find which column region the click falls into.
	clickedKey := ""
	for _, region := range ui.ActiveMiddleColumnLayout {
		if relX >= region.StartX && relX < region.EndX {
			clickedKey = region.Key
			break
		}
	}
	// Clicks past the last column fall through to the rightmost column so
	// the behavior matches the previous implementation.
	if clickedKey == "" {
		last := ui.ActiveMiddleColumnLayout[len(ui.ActiveMiddleColumnLayout)-1]
		if relX >= last.EndX {
			clickedKey = last.Key
		}
	}
	if clickedKey == "" {
		return m, nil
	}

	// Only react if the clicked column is actually sortable.
	if findSortableCol(clickedKey) < 0 {
		return m, nil
	}

	if m.sortColumnName == clickedKey {
		m.sortAscending = !m.sortAscending
	} else {
		m.sortColumnName = clickedKey
		m.sortAscending = true
	}
	m.rememberSort()
	m.sortMiddleItems()
	m.clampCursor()
	m.setStatusMessage("Sort: "+m.sortModeName(), false)
	return m, tea.Batch(m.loadPreview(), scheduleStatusClear())
}

// tabAtX returns the tab index at the given X coordinate in the tab bar,
// or -1 if the click is not on any tab.
func (m *Model) tabAtX(x int) int {
	labels := m.tabLabels()
	// Tab bar: each tab label is padded with 1 char on each side (Padding(0,1)),
	// separated by " | " (3 chars). Tab bar starts at x=1 (bar left padding).
	pos := 1
	for i, label := range labels {
		tabW := len(label) + 2 // label + padding(0,1) on each side
		if x >= pos && x < pos+tabW {
			return i
		}
		pos += tabW + 3 // separator " | "
	}
	return -1
}

// isViewerMode returns true for full-screen content viewers that don't
// have native wheel-scroll handling. modeLogs, modeExplorer, and
// modeObjectExplorer have their own per-pane wheel paths and are handled
// separately.
func isViewerMode(mode viewMode) bool {
	switch mode {
	case modeYAML, modeDescribe, modeDiff, modeHelp, modeExplain:
		return true
	}
	return false
}

// dispatchWheelKey synthesizes 3 presses of key (typically "j" or "k")
// through handleKey so each viewer mode's existing scroll logic runs
// unchanged. The model is threaded between iterations; the last cmd is
// returned (per-mode scroll handlers are pure state mutations and
// typically return nil, so dropping intermediate cmds is safe).
func (m Model) dispatchWheelKey(key string) (tea.Model, tea.Cmd) {
	const wheelStep = 3
	var lastCmd tea.Cmd
	runes := []rune(key)
	for range wheelStep {
		mdl, cmd := m.handleKey(keyPressRunes(runes))
		m = mdl.(Model)
		if cmd != nil {
			lastCmd = cmd
		}
	}
	return m, lastCmd
}

// switchToTab saves the current tab and loads the target tab.
func (m Model) switchToTab(tab int) (tea.Model, tea.Cmd) {
	m.saveCurrentTab()
	if cmd := m.loadTab(tab); cmd != nil {
		return m, cmd
	}
	if m.mode == modeExplorer {
		return m, m.loadPreview()
	}
	if m.mode == modeLogs && m.logView.ch != nil {
		return m, m.waitForLogLineIfIdle()
	}
	if m.mode == modeExec && m.execPTY != nil {
		return m, m.scheduleExecTick()
	}
	return m, nil
}
