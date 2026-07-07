package app

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
)

// taintEditorFocus tracks which part of the editor receives keys: the
// taint list, or one of the add-row input fields.
type taintEditorFocus int

const (
	taintFocusList taintEditorFocus = iota
	taintFocusKey
	taintFocusValue
	taintFocusEffect
)

// taintEditorRow is one taint in the editor: an existing taint (with an
// optional removal mark) or a staged addition not yet applied.
type taintEditorRow struct {
	taint  model.Taint
	staged bool // [+] addition, not yet on the node
	remove bool // [x] existing taint marked for removal
}

// taintEditorState is the node taint editor overlay (action menu →
// Taints). Existing taints are fetched fresh on open; removals and
// additions are staged locally and applied in one confirmed update.
type taintEditorState struct {
	active  bool
	node    string
	kctx    string
	cursor  int
	scroll  int
	rows    []taintEditorRow
	focus   taintEditorFocus
	addKey  string
	addVal  string
	addEff  int // index into model.ValidTaintEffects
	loading bool
	seq     int // fetch sequence — stale taintsLoadedMsg are dropped
}

// taintsLoadedMsg carries the fetched node taints for the editor.
type taintsLoadedMsg struct {
	taints []model.Taint
	seq    int
	err    error
}

// taintsAppliedMsg reports the result of the confirmed taint update.
type taintsAppliedMsg struct {
	node    string
	added   int
	removed int
	err     error
}

// openTaintEditor opens the editor on the action-menu target node and
// dispatches the fresh spec.taints fetch (the parsed Taints display
// column is not trusted — it is display-formatted and may be stale).
func (m Model) openTaintEditor() (tea.Model, tea.Cmd) {
	name := m.actionCtx.name
	kctx := m.actionCtx.context
	seq := m.taintEditor.seq + 1
	m.taintEditor = taintEditorState{
		active:  true,
		node:    name,
		kctx:    kctx,
		loading: true,
		seq:     seq,
	}
	m.previousOverlay = m.overlay
	m.overlay = overlayTaintEditor
	cmd := m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindYAMLFetch,
		"Taints: "+name, bgtaskTarget(kctx, ""),
		func(ctx context.Context) tea.Msg {
			taints, err := m.client.GetNodeTaints(ctx, kctx, name)
			return taintsLoadedMsg{taints: taints, seq: seq, err: err}
		})
	return m, cmd
}

// closeTaintEditor clears editor state and restores the prior overlay.
// The fetch sequence counter survives the reset so an in-flight fetch
// from this editor can never be mistaken for the next one's.
func (m *Model) closeTaintEditor() {
	m.taintEditor = taintEditorState{seq: m.taintEditor.seq}
	if m.overlay == overlayTaintEditor {
		m.overlay = m.previousOverlay
	}
	// Cleared unconditionally: on the confirmed-apply path the confirm
	// handler has already reset m.overlay, and leaving previousOverlay
	// pointing at a dead overlay is a latent footgun.
	m.previousOverlay = overlayNone
}

// updateTaintsLoaded fills the editor from a completed fetch. Stale
// fetches (editor closed and reopened meanwhile) are dropped; a failed
// fetch closes the editor with the error — there is nothing to edit.
func (m Model) updateTaintsLoaded(msg taintsLoadedMsg) (tea.Model, tea.Cmd) {
	p := &m.taintEditor
	if !p.active || msg.seq != p.seq {
		return m, nil
	}
	if msg.err != nil {
		m.closeTaintEditor()
		m.setStatusMessage("Taints: "+msg.err.Error(), true)
		return m, scheduleStatusClear()
	}
	p.loading = false
	p.rows = make([]taintEditorRow, 0, len(msg.taints))
	for _, t := range msg.taints {
		p.rows = append(p.rows, taintEditorRow{taint: t})
	}
	return m, nil
}

// taintEditorChanges returns the staged removals and additions.
func (m Model) taintEditorChanges() (removals, additions []model.Taint) {
	for _, r := range m.taintEditor.rows {
		switch {
		case r.staged:
			additions = append(additions, r.taint)
		case r.remove:
			removals = append(removals, r.taint)
		}
	}
	return removals, additions
}

// confirmTaintEditorApply routes the staged changes through the
// standard confirm overlay (always — a mistyped taint can reshape
// scheduling for the whole node). The editor state stays alive so a
// cancelled confirm returns to it.
func (m Model) confirmTaintEditorApply() (tea.Model, tea.Cmd) {
	removals, additions := m.taintEditorChanges()
	if len(removals) == 0 && len(additions) == 0 {
		m.closeTaintEditor()
		m.setStatusMessage("No changes", false)
		return m, scheduleStatusClear()
	}
	m.confirmTitle = "Apply taints"
	m.confirmQuestion = taintApplySummary(m.taintEditor.node, removals, additions)
	m.confirmAction = m.taintEditor.node + " (taints)"
	m.pendingAction = "Apply Taints"
	m.overlay = overlayConfirm
	return m, nil
}

// taintApplySummary is the confirm-overlay question: counts plus an
// explicit eviction warning when an addition has effect NoExecute.
func taintApplySummary(node string, removals, additions []model.Taint) string {
	parts := make([]string, 0, 2)
	if n := len(additions); n > 0 {
		parts = append(parts, fmt.Sprintf("%d added", n))
	}
	if n := len(removals); n > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", n))
	}
	q := fmt.Sprintf("Apply taint changes to %s: %s?", node, strings.Join(parts, ", "))
	if slices.ContainsFunc(additions, func(a model.Taint) bool { return a.Effect == "NoExecute" }) {
		q += " NoExecute evicts non-tolerating pods from the node."
	}
	return q
}

// routeTaintMsg dispatches the taint editor's async results (kept out
// of updateActionResultMsg's type switch for its gocyclo budget).
func (m Model) routeTaintMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch t := msg.(type) {
	case taintsLoadedMsg:
		return m.updateTaintsLoaded(t)
	case taintsAppliedMsg:
		return m.updateTaintsApplied(t)
	}
	return m, nil
}

// applyTaintEditor dispatches the confirmed taint update and closes the
// editor. Called from the confirm overlay's yes branch.
func (m *Model) applyTaintEditor() tea.Cmd {
	removals, additions := m.taintEditorChanges()
	node := m.taintEditor.node
	kctx := m.taintEditor.kctx
	m.closeTaintEditor()
	m.overlay = overlayNone

	return m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindYAMLFetch,
		"Apply taints: "+node, bgtaskTarget(kctx, ""),
		func(ctx context.Context) tea.Msg {
			err := m.client.UpdateNodeTaints(ctx, kctx, node, removals, additions)
			return taintsAppliedMsg{node: node, added: len(additions), removed: len(removals), err: err}
		})
}

// updateTaintsApplied surfaces the apply result and refreshes the node
// list so the Taints column reflects the change.
func (m Model) updateTaintsApplied(msg taintsAppliedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage("Taints: "+msg.err.Error(), true)
		return m, scheduleStatusClear()
	}
	parts := make([]string, 0, 2)
	if msg.added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", msg.added))
	}
	if msg.removed > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", msg.removed))
	}
	m.setStatusMessage(fmt.Sprintf("Taints updated on %s (%s)", msg.node, strings.Join(parts, ", ")), false)
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}
