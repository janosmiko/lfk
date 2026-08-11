package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// handleOverlayKeyTertiary is the last hop in the overlay key chain.
// handleOverlayKeySecondary sits at the gocyclo ceiling, so new overlays
// continue here rather than growing its switch past the limit.
func (m Model) handleOverlayKeyTertiary(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	if m.overlay == overlayUndeliverable {
		mdl, cmd := m.handleUndeliverableKey(msg)
		return mdl, cmd, true
	}
	return m, nil, false
}

// updateActionResultMsgTail is the last hop in the action/command message
// chain, for the same reason handleOverlayKeyTertiary exists.
func (m Model) updateActionResultMsgTail(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	if loaded, ok := msg.(undeliverableLoadedMsg); ok {
		mdl, cmd := m.handleUndeliverableLoaded(loaded)
		return mdl, cmd, true
	}
	return m, nil, false
}

// undeliverableOverlayW / H are the outer box dimensions. Centralised so the
// renderer and the move handlers cannot drift apart on viewport size.
func (m Model) undeliverableOverlayW() int {
	return ui.UndeliverableOverlayWidth(m.width)
}

func (m Model) undeliverableOverlayH() int {
	return min(30, m.height-6)
}

func (m Model) undeliverableVisibleLines() int {
	return ui.UndeliverableBodyHeight(m.undeliverableOverlayH())
}

// openUndeliverableOverlay shows the cluster-wide list, reusing the report
// already in state when one is loaded for this context so reopening is
// instant. Cursor, scroll, and filter query are preserved; only filterActive
// resets, so the next keypress navigates instead of being typed into the filter.
func (m Model) openUndeliverableOverlay() (Model, tea.Cmd) {
	if m.isUnionSentinel() {
		m.setStatusMessage("Undeliverable requires a single cluster", true)
		return m, scheduleStatusClear()
	}
	m.overlay = overlayUndeliverable
	m.undeliverable.filterActive = false

	if m.undeliverable.loadedFor == m.nav.Context && !m.undeliverable.inflight {
		m.undeliverable.loading = false
		return m.undeliverableClampCursor(), nil
	}
	m.undeliverable.loadedFor = m.nav.Context
	m.undeliverable.loading = true
	return m, (&m).cmdLoadUndeliverable(m.nav.Context)
}

func (m Model) closeUndeliverableOverlay() Model {
	m.overlay = overlayNone
	m.undeliverable.filterActive = false
	return m
}

// undeliverableMoveCursor moves the cursor by delta inside the visible list
// and scrolls only when the cursor would leave the viewport.
func (m Model) undeliverableMoveCursor(delta int) (Model, tea.Cmd) {
	return m.undeliverableJumpCursor(m.undeliverable.cursor + delta), nil
}

// undeliverableJumpCursor positions the cursor at an absolute index, clamped
// into the visible list. Powers g / G / Home / End and every relative move.
func (m Model) undeliverableJumpCursor(idx int) Model {
	rows := m.undeliverable.visibleRows()
	if len(rows) == 0 {
		m.undeliverable.cursor = 0
		m.undeliverable.scroll = 0
		return m
	}
	m.undeliverable.cursor = max(min(idx, len(rows)-1), 0)
	m.undeliverable.scroll = ui.UndeliverableScrollForCursor(
		m.undeliverable.scroll, m.undeliverable.cursor,
		m.undeliverableVisibleLines(), len(rows),
	)
	return m
}

// undeliverableResetCursor sends the cursor to the top of a list that just
// changed shape (filter typed or cleared), so a stale offset cannot render
// past the end.
func (m Model) undeliverableResetCursor() Model {
	m.undeliverable.cursor = 0
	m.undeliverable.scroll = ui.UndeliverableClampScroll(
		0, len(m.undeliverable.visibleRows()), m.undeliverableVisibleLines(),
	)
	return m
}

// undeliverableClampCursor keeps an existing cursor/scroll pair valid when a
// background refresh shrinks the report - unlike undeliverableResetCursor it
// preserves the user's position whenever that position still exists.
func (m Model) undeliverableClampCursor() Model {
	return m.undeliverableJumpCursor(m.undeliverable.cursor)
}

// handleUndeliverableKey routes one key press inside the overlay.
func (m Model) handleUndeliverableKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.undeliverable.filterActive {
		return m.handleUndeliverableFilterInput(msg)
	}

	half := max(m.undeliverableVisibleLines()/2, 1)
	page := max(m.undeliverableVisibleLines(), 1)
	switch msg.String() {
	case "j", "down":
		return m.undeliverableMoveCursor(+1)
	case "k", "up":
		return m.undeliverableMoveCursor(-1)
	case "g", "home":
		return m.undeliverableJumpCursor(0), nil
	case "G", "end":
		return m.undeliverableJumpCursor(len(m.undeliverable.visibleRows()) - 1), nil
	case "ctrl+d", "shift+down":
		return m.undeliverableMoveCursor(+half)
	case "ctrl+u", "shift+up":
		return m.undeliverableMoveCursor(-half)
	case "ctrl+f", "pgdown":
		return m.undeliverableMoveCursor(+page)
	case "ctrl+b", "pgup":
		return m.undeliverableMoveCursor(-page)
	case "/":
		m.undeliverable.filterActive = true
		return m, nil
	case "R":
		m.undeliverable.loadedFor = m.nav.Context
		m.undeliverable.loading = true
		return m, (&m).cmdLoadUndeliverable(m.nav.Context)
	case "enter":
		return m.jumpToUndeliverable()
	case "esc", "q":
		return m.closeUndeliverableOverlay(), nil
	}
	return m, nil
}

// handleUndeliverableFilterInput handles keys while the / filter is capturing
// input. Esc clears the query, Enter keeps it and leaves input mode.
func (m Model) handleUndeliverableFilterInput(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.undeliverable.filter.Clear()
		m.undeliverable.filterActive = false
		return m.undeliverableResetCursor(), nil
	case "enter":
		m.undeliverable.filterActive = false
		return m, nil
	case "backspace":
		m.undeliverable.filter.Backspace()
		return m.undeliverableResetCursor(), nil
	default:
		// Insert the character produced, not the key pressed: shift+a
		// reports Code 'a' but Text "A".
		if len([]rune(msg.Text)) == 1 {
			m.undeliverable.filter.Insert(msg.Text)
			return m.undeliverableResetCursor(), nil
		}
		return m, nil
	}
}

// jumpToUndeliverable navigates to the highlighted resource: switches to its
// namespace, opens its resource type with the cursor on it, and closes the
// overlay. Mirrors jumpToOrphan, including the discoveredResources lookup -
// scanning middleItems by name silently no-ops when that list is still cold.
func (m Model) jumpToUndeliverable() (Model, tea.Cmd) {
	rows := m.undeliverable.visibleRows()
	if m.undeliverable.cursor < 0 || m.undeliverable.cursor >= len(rows) {
		return m, nil
	}
	target := rows[m.undeliverable.cursor]

	rt, ok := model.FindResourceTypeByKind(target.Kind, m.discoveredResources[m.nav.Context])
	if !ok {
		m.setStatusMessage(
			fmt.Sprintf("Cannot jump to %s/%s — resource type not yet discovered, retry in a moment",
				target.Kind, target.Name), true)
		return m, scheduleStatusClear()
	}

	m.pushJumpHistory()
	m = m.closeUndeliverableOverlay()

	// Cluster-wide list to a single-namespace resource list: all-namespaces
	// must be off and any prior multi-namespace selection replaced, or the
	// load can drop the very namespace the target is in. A cluster-scoped
	// row (a terminating Namespace) has no namespace to switch to.
	if target.Namespace != "" {
		m.allNamespaces = false
		m.namespace = target.Namespace
		m.selectedNamespaces = map[string]bool{target.Namespace: true}
	}

	for m.nav.Level > model.LevelResourceTypes {
		ret, _ := m.navigateParent()
		m = ret.(Model)
	}
	if m.nav.Level < model.LevelResourceTypes {
		m.setStatusMessage("Cannot jump: enter a context first", true)
		return m, scheduleStatusClear()
	}

	for i, item := range m.middleItems {
		if item.Extra == rt.ResourceRef() {
			m.setCursor(i)
			m.pendingTarget = target.Name
			ret, cmd := m.navigateChild()
			next, ok := ret.(Model)
			if !ok {
				return m, cmd
			}
			return next, cmd
		}
	}

	m.setStatusMessage(
		fmt.Sprintf("Cannot jump: %s not in sidebar (toggle rare resources with H?)", target.Kind),
		true)
	return m, scheduleStatusClear()
}
