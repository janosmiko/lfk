package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/ui"
)

// handleOverlayMouse routes mouse events while a centered overlay is open.
//
// Behavior:
//   - Click outside the overlay's bounding box dismisses it by synthesizing
//     Esc through handleOverlayKey, so each overlay's per-type Esc handler
//     (and any cleanup it does) runs unchanged.
//   - Left-click on a row inside the action menu activates that row, just
//     like Enter on the keyboard.
//   - Wheel events and clicks on overlays without a known interactive
//     layout are ignored — they used to be ignored unconditionally, so
//     this is no regression.
//
// Fullscreen overlays (secret/configmap editors, rollback, helm history,
// label editor, auto-sync) and custom-rendered overlays (Can-I,
// CanISubject, NetworkPolicy) are skipped here: they cover the whole
// screen so "outside" has no meaning, and their internal layouts are too
// varied to map to clicks without per-overlay code.
func (m Model) handleOverlayMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	box, ok := m.centeredOverlayBox()
	if !ok {
		// Fullscreen / custom-rendered overlay: swallow every mouse
		// event (clicks AND wheel) so we don't accidentally drive the
		// explorer underneath, and so wheel doesn't start scrolling
		// list state for an overlay we documented as keyboard-only.
		return m, nil
	}

	// Wheel events scroll the overlay's list cursor by synthesizing
	// arrow-key presses. Each overlay's normal-mode key handler reacts
	// to "up"/"down" by moving its cursor (overlayCursor / schemeCursor /
	// templateCursor / etc.), so we don't need per-overlay knowledge
	// here. In filter / text-input modes, arrow keys are no-ops in the
	// shared handleFilterKey, so wheel ticks won't pollute the filter
	// text.
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		return m.dispatchOverlayWheel(msg.Button)
	}

	// Only react to button presses for clicks.
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if msg.Button != tea.MouseButtonLeft && msg.Button != tea.MouseButtonRight {
		return m, nil
	}

	// Click outside the overlay box: dismiss it. We dispatch through the
	// keyboard path so each overlay's own Esc handler runs (and so
	// behaviors like clearing pendingAction or restoring previousOverlay
	// stay in one place). Esc is the safe universal cancel for confirm
	// dialogs, text-input prompts, list selectors, and read-only info
	// panels alike.
	if msg.X < box.x || msg.X >= box.x+box.w ||
		msg.Y < box.y || msg.Y >= box.y+box.h {
		return m.handleOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
	}

	// Click inside the box. Per-overlay activation:
	if msg.Button == tea.MouseButtonLeft {
		innerY := msg.Y - (box.y + 2) // -1 border, -1 top padding
		if mdl, cmd, handled := m.activateOverlayItemAt(innerY); handled {
			return mdl, cmd
		}
	}

	// Click landed inside the box but on a non-interactive area (title,
	// padding, blank rows below the items, or an overlay we haven't
	// wired up yet). Swallow it so it doesn't dismiss the overlay.
	return m, nil
}

// dispatchOverlayWheel translates a wheel tick into 3 down/up arrow key
// presses and routes them through handleOverlayKey. The 3-step factor
// matches the explorer-mode wheel speed elsewhere in this file. We use
// arrow keys rather than j/k because they are a no-op in the shared
// handleFilterKey path, so wheel ticks while a list overlay is in
// /-filter mode don't accidentally type letters into the filter input.
func (m Model) dispatchOverlayWheel(button tea.MouseButton) (tea.Model, tea.Cmd) {
	const wheelStep = 3
	keyType := tea.KeyDown
	if button == tea.MouseButtonWheelUp {
		keyType = tea.KeyUp
	}
	var lastCmd tea.Cmd
	for range wheelStep {
		mdl, cmd := m.handleOverlayKey(tea.KeyMsg{Type: keyType})
		m = mdl.(Model)
		if cmd != nil {
			lastCmd = cmd
		}
	}
	return m, lastCmd
}

// activateOverlayItemAt translates a click at inner-content row innerY
// into a per-overlay activation. Each list-style overlay maintains its
// own header layout (title rows, filter input, scroll indicator) and
// scroll offset, so the math is encapsulated per case and not factored
// into a shared helper — that way a renderer change only forces an
// update in one place.
//
// handled=false means the click was inside the box but not on a
// clickable row; the caller should swallow it (no dismiss) so a stray
// click on the title or padding doesn't close the overlay.
func (m Model) activateOverlayItemAt(innerY int) (tea.Model, tea.Cmd, bool) {
	switch m.overlay {
	case overlayAction:
		// Inner rows: 0=title, 1=title-padding, 2..N+1=items.
		idx := innerY - 2
		if idx < 0 || idx >= len(m.overlayItems) {
			return m, nil, false
		}
		m.overlayCursor = idx
		mdl, cmd := m.executeAction(m.overlayItems[idx].Name)
		return mdl, cmd, true

	case overlayNamespace:
		// Inner rows: 0=title, 1=title-padding, 2=filter, 3=blank
		// (from "\n\n"), 4=scroll-above row, 5..5+visible-1=items.
		const itemsStartRow = 5
		items := m.filteredOverlayItems()
		visibleCount := min(15, len(items))
		rowInItems := innerY - itemsStartRow
		if rowInItems < 0 || rowInItems >= visibleCount {
			return m, nil, false
		}
		idx := ui.GetOverlayNsScroll() + rowInItems
		if idx < 0 || idx >= len(items) {
			return m, nil, false
		}
		m.overlayCursor = idx
		// Hand off to the overlay's normal Enter handler so the same
		// "apply selection and close" path runs as keyboard activation.
		mdl, cmd := m.handleOverlayKey(tea.KeyMsg{Type: tea.KeyEnter})
		return mdl, cmd, true
	}

	return m, nil, false
}

// overlayBoundingBox holds the screen-space rectangle for a centered
// overlay. The rectangle is inclusive of the rounded border drawn by
// OverlayStyle.
type overlayBoundingBox struct {
	x, y, w, h int
}

// centeredOverlayBox returns the screen rectangle of the current overlay
// when it is rendered through the standard centered path
// (renderOverlayContent). ok=false means the overlay is fullscreen,
// custom-rendered, or absent — in any of those cases the caller has no
// reliable "outside the overlay" coordinate to test.
//
// The math mirrors view_overlays.go renderOverlay. OverlayStyle has
// Padding(1, 2) and a rounded border, so a clean Width(W).Height(H)
// produces a visual W+2 / H+2 box. The catch: when the rendered content
// is taller (or wider) than the requested W/H, lipgloss expands the box
// to fit instead of truncating — so we have to take the natural content
// size into account, otherwise our predicted box.y is off by however
// many rows the content overflows the requested H by, and clicks land on
// the wrong row. This bit us at narrow heights where the namespace
// overlay's full layout (title + filter + items + scroll indicators) is
// taller than min(20, m.height-6).
func (m Model) centeredOverlayBox() (overlayBoundingBox, bool) {
	if m.overlay == overlayNone {
		return overlayBoundingBox{}, false
	}
	content, ow, oh, ok := m.renderOverlayContent()
	if !ok {
		return overlayBoundingBox{}, false
	}
	if ow < 10 {
		ow = 10
	}
	if oh < 3 {
		oh = 3
	}

	// Border (1 each side) + Padding(1, 2). When content fits, visual =
	// requested + 2 (border). When content overflows, visual = content
	// natural size + 2 padding + 2 border.
	contentLines := strings.Count(content, "\n") + 1
	visualH := oh + 2
	if naturalH := contentLines + 4; naturalH > visualH {
		visualH = naturalH
	}
	visualW := ow + 2
	for line := range strings.SplitSeq(content, "\n") {
		if natural := lipgloss.Width(line) + 6; natural > visualW {
			visualW = natural
		}
	}

	x := (m.width - visualW) / 2
	y := (m.height - visualH) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return overlayBoundingBox{x: x, y: y, w: visualW, h: visualH}, true
}
