package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

const constraintsScrollOff = 3

// openConstraintsView opens the "what constrains this object" fullscreen
// view for the currently selected middle-column row.
func (m Model) openConstraintsView() (tea.Model, tea.Cmd) {
	if m.nav.Level < model.LevelResources {
		m.setStatusMessage("Select a resource first", true)
		return m, scheduleStatusClear()
	}
	sel := m.selectedMiddleItem()
	if sel == nil || sel.Raw == nil {
		m.setStatusMessage("No resource data available", true)
		return m, scheduleStatusClear()
	}

	target := k8s.TargetFromRaw(sel.Raw)
	target.Namespace = sel.Namespace
	target.Kind = sel.Kind
	target.GVR = schema.GroupVersionResource{
		Group:    m.nav.ResourceType.APIGroup,
		Version:  m.nav.ResourceType.APIVersion,
		Resource: m.nav.ResourceType.Resource,
	}

	m.constraints = constraintsViewState{
		title:      resourceTitleLabel(sel.Kind, sel.Namespace, sel.Name),
		name:       sel.Name,
		target:     target,
		loading:    true,
		returnMode: m.mode,
		gen:        m.constraints.gen + 1,
	}
	m.mode = modeConstraints
	return m, m.loadConstraints(sel.Name)
}

// handleConstraintsKey handles input for the fullscreen constraints view.
func (m Model) handleConstraintsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	kb := ui.ActiveKeybindings
	rows := m.constraints.visibleRows()
	maxIdx := len(rows) - 1
	half := max(m.constraintsViewportHeight()/2, 1)

	// Cleared up front so no exit path leaves a half-typed gg armed for the
	// explorer to complete after the view closes.
	pendingG := m.pendingG
	m.pendingG = false

	switch msg.String() {
	case kb.Down, "j", "down":
		m.constraints.cursor = clampOverlayCursor(m.constraints.cursor, 1, maxIdx)
	case kb.Up, "k", "up":
		m.constraints.cursor = clampOverlayCursor(m.constraints.cursor, -1, maxIdx)
	case kb.JumpTop, "g":
		if !pendingG {
			m.pendingG = true
			return m, nil
		}
		m.constraints.cursor = 0
	case kb.JumpBottom, "G":
		m.constraints.cursor = maxIdx
	case kb.PageDown, "ctrl+d":
		m.constraints.cursor = clampOverlayCursor(m.constraints.cursor, half, maxIdx)
	case kb.PageUp, "ctrl+u":
		m.constraints.cursor = clampOverlayCursor(m.constraints.cursor, -half, maxIdx)
	case kb.Enter, "enter":
		return m.jumpFromConstraintsRow()
	case kb.Refresh:
		m.constraints.loading = true
		m.constraints.gen++
		return m, m.loadConstraints(m.constraints.name)
	case "q", "esc":
		m.mode = m.constraints.returnMode
		return m, nil
	default:
		return m, nil
	}
	m.constraints.scroll = ui.VimScrollOff(
		m.constraints.scroll, m.constraints.cursor, len(rows),
		m.constraintsViewportHeight(), constraintsScrollOff,
		func(from, to int) int { return to - from },
	)
	return m, nil
}

// jumpFromConstraintsRow reuses jumpToOrphan's recipe: resolve the row's
// Kind via discovery, record jump history, switch namespace, climb to
// LevelResourceTypes, then drill into the row's object by name.
func (m Model) jumpFromConstraintsRow() (tea.Model, tea.Cmd) {
	rows := m.constraints.visibleRows()
	if m.constraints.cursor < 0 || m.constraints.cursor >= len(rows) {
		return m, nil
	}
	row := rows[m.constraints.cursor]
	if row.Kind == "" || row.Name == "" {
		m.setStatusMessage("This row names no single object to jump to", true)
		return m, scheduleStatusClear()
	}

	rt, ok := model.FindResourceTypeByKind(row.Kind, m.discoveredResources[m.nav.Context])
	if !ok {
		m.setStatusMessage(
			fmt.Sprintf("Cannot jump to %s/%s — resource type not yet discovered, retry in a moment",
				row.Kind, row.Name), true)
		return m, scheduleStatusClear()
	}

	// Every navigation change lands on a candidate copy, so a jump that
	// cannot resolve its target leaves the view, namespace and jump
	// history as the user left them.
	cand := m
	cand.pushJumpHistory()
	cand.mode = m.constraints.returnMode

	if row.Namespace != "" {
		cand.allNamespaces = false
		cand.namespace = row.Namespace
		cand.selectedNamespaces = map[string]bool{row.Namespace: true}
	}

	for cand.nav.Level > model.LevelResourceTypes {
		ret, _ := cand.navigateParent()
		cand = ret.(Model)
	}
	if cand.nav.Level < model.LevelResourceTypes {
		m.setStatusMessage("Cannot jump: enter a context first", true)
		return m, scheduleStatusClear()
	}

	for i, item := range cand.middleItems {
		if item.Extra == rt.ResourceRef() {
			cand.setCursor(i)
			cand.pendingTarget = row.Name
			ret, cmd := cand.navigateChild()
			next, ok := ret.(Model)
			if !ok {
				return m, cmd
			}
			return next, cmd
		}
	}

	m.setStatusMessage(
		fmt.Sprintf("Cannot jump: %s not in sidebar (toggle rare resources with H?)", row.Kind), true)
	return m, scheduleStatusClear()
}
