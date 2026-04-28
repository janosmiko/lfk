package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// mutatingActions is the canonical set of action labels that change cluster
// state. When Model.readOnly is true, every label in this set is blocked at
// the dispatcher.
//
// Keep this list in sync with the action handlers in update_actions.go and
// the bulk handlers in update_actions.go (executeBulkAction). Adding a new
// mutating action without listing it here is a silent escape from read-only
// mode; the readonly_test.go suite asserts membership for every known label.
var mutatingActions = map[string]bool{
	"Delete":               true,
	"Force Delete":         true,
	"Force Finalize":       true,
	"Edit":                 true,
	"Secret Editor":        true,
	"ConfigMap Editor":     true,
	"Scale":                true,
	"Restart":              true,
	"Rollback":             true,
	"Exec":                 true,
	"Attach":               true,
	"Shell":                true,
	"Debug":                true,
	"Debug Pod":            true,
	"Debug Mount":          true,
	"Port Forward":         true,
	"Cordon":               true,
	"Uncordon":             true,
	"Drain":                true,
	"Taint":                true,
	"Untaint":              true,
	"Trigger":              true,
	"Stop":                 true,
	"Remove":               true,
	"Labels / Annotations": true,
	"Permissions":          true,
}

// isMutatingAction reports whether a given action label changes cluster state
// and should be blocked when read-only mode is active.
//
// Built-in action labels are checked against mutatingActions. Custom user
// actions (ui.CustomAction) bypass this set because their labels are
// arbitrary; isMutatingActionForKind handles them — call that variant
// when the resource kind is known so custom actions can be evaluated
// against their ReadOnlySafe flag.
func isMutatingAction(label string) bool {
	return mutatingActions[label]
}

// isMutatingActionForKind extends isMutatingAction with awareness of
// user-defined custom actions. A custom action is treated as mutating
// unless its CustomAction.ReadOnlySafe field is set to true. This is
// safer-by-default: a user who configures a destructive shell command
// without thinking about read-only mode does not silently bypass it.
//
// When the kind is not known (e.g. inside the dispatcher with only a
// label), fall back to isMutatingAction. Built-in labels still take
// effect because the mutatingActions map is checked first.
func isMutatingActionForKind(kind, label string) bool {
	if mutatingActions[label] {
		return true
	}
	if ca, ok := findCustomAction(kind, label); ok {
		return !ca.ReadOnlySafe
	}
	return false
}

// readOnlyBlockedMessage returns the toast message used when a mutating
// action is blocked. Centralised so tests can assert on the exact format.
func readOnlyBlockedMessage(actionLabel string) string {
	return "Read-only mode: " + actionLabel + " disabled"
}

// effectiveContextReadOnly returns the read-only state to display for the
// given context in the cluster picker, applying the same precedence as
// recomputeReadOnly. Used when annotating cluster picker rows so the
// [RO] marker matches what the user gets on entry.
func (m *Model) effectiveContextReadOnly(ctx string) bool {
	if m.cliReadOnly {
		return true
	}
	if v, ok := m.contextROOverrides[ctx]; ok {
		return v
	}
	return ui.ResolveReadOnly(ctx, false)
}

// refreshContextReadOnlyMarkers re-applies effectiveContextReadOnly to every
// row in middleItems when at the cluster picker. Call this after a state
// change that might affect the override map (tab switch into a picker view,
// :ctx command, etc.) so the [RO] markers don't go stale.
//
// No-op when the user is not at LevelClusters: middleItems then holds
// resource types or resources, none of which carry the ReadOnly flag.
func (m *Model) refreshContextReadOnlyMarkers() {
	if m.nav.Level != model.LevelClusters {
		return
	}
	for i := range m.middleItems {
		m.middleItems[i].ReadOnly = m.effectiveContextReadOnly(m.middleItems[i].Name)
	}
	// Keep the cache aligned so back-navigation re-shows the fresh markers.
	m.itemCache[m.navKey()] = m.middleItems
}

// recomputeReadOnly recalculates m.readOnly for the given context after a
// nav.Context change. Call it from every site that mutates nav.Context so
// CLI flag, per-context overrides, and config take effect on every
// navigation path (cluster picker, :ctx command, bookmark restore,
// session restore).
//
// Precedence (highest first):
//
//   - --read-only CLI flag (sticky for the process)
//   - per-context session override (Ctrl+R on a row in the picker)
//   - per-context config (clusters.<ctx>.read_only)
//   - global config (read_only)
//
// The in-context Ctrl+R toggle bypasses this function — it sets m.readOnly
// directly for the current tab. Switching contexts re-runs recompute and
// drops the in-context toggle.
func (m *Model) recomputeReadOnly(ctx string) {
	if m.cliReadOnly {
		m.readOnly = true
		return
	}
	if v, ok := m.contextROOverrides[ctx]; ok {
		m.readOnly = v
		return
	}
	m.readOnly = ui.ResolveReadOnly(ctx, false)
}

// handleKeyReadOnlyToggle handles Ctrl+R based on navigation level:
//
//   - At the cluster picker (LevelClusters), the toggle flips the
//     read-only state of the highlighted context row. The [RO] marker on
//     that row updates immediately, and the new state is stored in
//     contextROOverrides so it persists across re-navigations within the
//     session and is honored on context entry. Per-context config and
//     global config provide the *initial* state; the override wins until
//     toggled again.
//
//   - Inside a context, the toggle flips the active tab's read-only
//     state. Session-scoped, local to that context, and does not leak
//     across context switches.
//
// CLI flag stickiness is preserved at both levels: when --read-only was
// passed, the picker toggle is rejected with a hint (the flag forces RO
// on every context), and the in-context toggle's "off" state is
// re-asserted on the next context switch.
func (m Model) handleKeyReadOnlyToggle() (tea.Model, tea.Cmd) {
	if m.nav.Level == model.LevelClusters {
		if m.cliReadOnly {
			m.setStatusMessage("--read-only forces all contexts read-only", true)
			return m, scheduleStatusClear()
		}
		sel := m.selectedMiddleItem()
		if sel == nil {
			return m, nil
		}
		newState := !sel.ReadOnly
		if m.contextROOverrides == nil {
			m.contextROOverrides = make(map[string]bool)
		}
		m.contextROOverrides[sel.Name] = newState
		sel.ReadOnly = newState
		// Refresh the cached items so the marker survives back-navigation
		// to the picker without a context reload.
		m.itemCache[m.navKey()] = m.middleItems
		state := "OFF"
		if newState {
			state = "ON"
		}
		m.setStatusMessage(sel.Name+" read-only: "+state, false)
		return m, scheduleStatusClear()
	}
	m.readOnly = !m.readOnly
	state := "OFF"
	if m.readOnly {
		state = "ON"
	}
	m.setStatusMessage("Read-only mode: "+state, false)
	return m, scheduleStatusClear()
}
