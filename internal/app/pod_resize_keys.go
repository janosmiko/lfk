package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// podResizeCharset is the set of characters a quantity field accepts:
// digits, the decimal point, and the Kubernetes quantity suffix letters
// this overlay supports (m, k, M, G, i).
const podResizeCharset = "0123456789.mkMGi"

// podResizeFieldCount is the number of editable rows per container (cpu
// req, cpu lim, mem req, mem lim), matching podResizeRow's field order.
const podResizeFieldCount = 4

// totalFields returns the number of editable rows across every container.
func (s *podResizeState) totalFields() int {
	return len(s.containers) * podResizeFieldCount
}

// active returns a pointer to the currently focused input.
func (s *podResizeState) active() *TextInput {
	return s.containers[s.field/podResizeFieldCount].fieldAt(s.field % podResizeFieldCount)
}

// fieldAt returns the row's cpuReq/cpuLim/memReq/memLim input by index,
// matching podResizeFieldCount's field order.
func (r *podResizeRow) fieldAt(i int) *TextInput {
	switch i {
	case 0:
		return &r.cpuReq
	case 1:
		return &r.cpuLim
	case 2:
		return &r.memReq
	default:
		return &r.memLim
	}
}

// clampPodResizeScroll keeps the cursor within the visible window via the
// shared vim-style scrolloff viewport.
func (m *Model) clampPodResizeScroll() {
	overlayListScroll(&m.podResize.scroll, m.podResize.field, m.podResize.totalFields(), m.podResizeMaxVisible())
}

// podResizeMaxVisible caps the field rows shown at once, leaving room for
// the pod-level read-only rows and the restart-warning note.
func (m Model) podResizeMaxVisible() int {
	return min(12, max(m.height-14, 3))
}

// handlePodResizeOverlayKey routes a keypress in the Resize Pod overlay:
// j/k and tab move between fields, arrows move the cursor, digits and unit
// letters type, enter applies, esc cancels.
func (m Model) handlePodResizeOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.podResize = podResizeState{}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	case "j", "down", "tab":
		return m.movePodResizeField(1), nil
	case "k", "up":
		return m.movePodResizeField(-1), nil
	case "left":
		m.podResize.active().Left()
		return m, nil
	case "right":
		m.podResize.active().Right()
		return m, nil
	case "enter":
		return m.applyPodResizeOverlay()
	case "backspace":
		m.podResize.active().Backspace()
		return m, nil
	case "ctrl+w":
		m.podResize.active().DeleteWord()
		return m, nil
	default:
		key := msg.String()
		if len(key) == 1 && strings.ContainsRune(podResizeCharset, rune(key[0])) {
			m.podResize.active().Insert(key)
		}
		return m, nil
	}
}

// movePodResizeField steps the focused field by delta rows, wrapping, and
// keeps the scroll window in sync.
func (m Model) movePodResizeField(delta int) Model {
	total := m.podResize.totalFields()
	if total == 0 {
		return m
	}
	m.podResize.field = (m.podResize.field + delta + total) % total
	m.clampPodResizeScroll()
	return m
}

// applyPodResizeOverlay validates the form and dispatches the resize patch
// for whichever containers changed.
func (m Model) applyPodResizeOverlay() (tea.Model, tea.Cmd) {
	specs, err := parsePodResizeForm(m.podResize)
	if err != nil {
		m.setStatusMessage("Resize: "+err.Error(), true)
		return m, scheduleStatusClear()
	}
	if len(specs) == 0 {
		m.overlay = overlayNone
		m.podResize = podResizeState{}
		m.setStatusMessage("No changes", false)
		return m, scheduleStatusClear()
	}

	// This is the only read-only gate for the resize action: "Resize" is
	// not in mutatingActions, so the dispatcher lets the form open and the
	// check has to happen at submit, as the PVC path does.
	if m.actionTargetBlockedByReadOnly() {
		m.overlay = overlayNone
		m.podResize = podResizeState{}
		m.setStatusMessage(readOnlyBlockedMessage("Resize"), true)
		return m, scheduleStatusClear()
	}

	m.overlay = overlayNone
	m.loading = true
	m.podResize = podResizeState{}
	return m, m.resizePodResources(specs)
}
