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
	// Core kubectl mutations.
	"Delete":               true,
	"Force Delete":         true,
	"Force Finalize":       true,
	"Finalizer Remove":     true,
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
	"Port Forward & Open":  true,
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

	// ArgoCD application / sync state mutations.
	"Configure AutoSync": true,
	"Auto Sync":          true,
	"Sync":               true,
	"Sync (Apply Only)":  true,
	"Terminate Sync":     true,

	// Argo Workflows lifecycle mutations.
	"Suspend Workflow":     true,
	"Resume Workflow":      true,
	"Stop Workflow":        true,
	"Terminate Workflow":   true,
	"Resubmit Workflow":    true,
	"Submit Workflow":      true,
	"Suspend CronWorkflow": true,
	"Resume CronWorkflow":  true,

	// cert-manager / ExternalSecrets / KEDA / Flux mutations.
	"Force Renew":   true,
	"Force Refresh": true,
	"Pause":         true,
	"Unpause":       true,
	"Reconcile":     true,
	"Suspend":       true,
	"Resume":        true,

	// Longhorn node mutations. Force Delete (above) disables scheduling then
	// deletes; Evict Replicas / Cancel Eviction toggle spec.evictionRequested.
	"Evict Replicas":  true,
	"Cancel Eviction": true,

	// Karpenter NodeClaim mutations. Disrupt removes the underlying
	// node; Cordon / Uncordon / Drain Node operate on the node bound
	// to the NodeClaim.
	"Disrupt":       true,
	"Cordon Node":   true,
	"Uncordon Node": true,
	"Drain Node":    true,

	// Knative Serving mutations. Activate patches the parent Service's
	// spec.traffic to send 100% to a Revision.
	"Activate": true,

	// Helm release mutations.
	"Edit Values": true,
	"Upgrade":     true,
}

// isUnionAllowedActionForKind reports whether an action is allowed at the
// union sentinel. Fetch-only actions pass through, including custom actions
// explicitly marked read_only_safe. Mutating actions must opt in by both kind
// and label so the merged view cannot delete arbitrary resource kinds just
// because they expose a "Delete" action.
func isUnionAllowedActionForKind(kind, label string) bool {
	if !isMutatingActionForKind(kind, label) {
		return true
	}
	switch label {
	case "Delete", "Force Delete", "Force Finalize":
		return kind == "Pod"
	case "Port Forward":
		switch kind {
		case "Pod", "Service", "Deployment", "StatefulSet", "DaemonSet":
			return true
		}
		return false
	case "Port Forward & Open":
		return kind == "Service"
	case "Restart":
		return model.IsRestartableKind(kind)
	default:
		return false
	}
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

// readOnlyForContext resolves read-only state for a specific target context.
// This is distinct from Model.readOnly in union mode: the active navigation
// context is the internal union sentinel, but each row action targets a real
// source cluster whose own read-only policy still has to be honored.
func (m Model) readOnlyForContext(ctx string) bool {
	if m.cliReadOnly || m.readOnly {
		return true
	}
	if !m.unionMode || ctx == "" || ctx == m.nav.Context || ctx == UnionContextSentinel {
		return false
	}
	if v, ok := m.contextROOverrides[ctx]; ok {
		return v
	}
	return ui.ResolveReadOnly(ctx, false)
}

// bulkReadOnlyContext returns the first target context that would make a
// mutating bulk action illegal. Bulk union actions are all-or-nothing at the
// dispatcher; individual handlers should not partially mutate a mixed
// read-only/read-write selection.
func (m Model) bulkReadOnlyContext() (string, bool) {
	if len(m.bulkItems) == 0 {
		ctx := m.actionCtx.context
		if m.unionMode && ctx == UnionContextSentinel {
			return ctx, true
		}
		return ctx, m.readOnlyForContext(ctx)
	}
	for _, item := range m.bulkItems {
		contexts, unknown := m.bulkItemTargetContexts(item)
		if unknown {
			return UnionContextSentinel, true
		}
		for _, ctx := range contexts {
			if m.readOnlyForContext(ctx) {
				return ctx, true
			}
		}
	}
	return "", false
}

func (m Model) bulkItemTargetContexts(item model.Item) ([]string, bool) {
	if item.ClusterName != "" {
		return []string{item.ClusterName}, false
	}
	if len(item.GroupedRefs) > 0 {
		contexts := make([]string, 0, len(item.GroupedRefs))
		seen := make(map[string]struct{}, len(item.GroupedRefs))
		for _, ref := range item.GroupedRefs {
			if ref.ClusterName == "" {
				return nil, true
			}
			if _, ok := seen[ref.ClusterName]; ok {
				continue
			}
			seen[ref.ClusterName] = struct{}{}
			contexts = append(contexts, ref.ClusterName)
		}
		if len(contexts) > 0 {
			return contexts, false
		}
	}
	ctx := m.actionCtx.context
	if ctx != "" && ctx != UnionContextSentinel {
		return []string{ctx}, false
	}
	if m.unionMode || ctx == UnionContextSentinel {
		return nil, true
	}
	return []string{ctx}, false
}

func (m Model) actionTargetBlockedByReadOnly() bool {
	if m.bulkMode && len(m.bulkItems) > 0 {
		_, blocked := m.bulkReadOnlyContext()
		return blocked
	}
	return m.readOnlyForContext(m.actionCtx.context)
}

func (m Model) pendingActionBlockedByReadOnly() bool {
	if !isMutatingAction(m.pendingAction) {
		return false
	}
	return m.actionTargetBlockedByReadOnly()
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
// The in-context Ctrl+R toggle also records its result in
// contextROOverrides (keyed by the active context), so re-entering that
// context re-reads the override here and preserves the user's last choice
// rather than reverting to config. Switching to a *different* context reads
// that context's own override/config, so an unlock never leaks across
// contexts.
func (m *Model) recomputeReadOnly(ctx string) {
	// Recompute every per-context view default on context switch. Done here
	// because recomputeReadOnly is already called at every switch point;
	// keep it before the read-only early-returns so it always runs.
	m.recomputeHideSecurityBadges(ctx)
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

// recomputeHideSecurityBadges sets m.hideSecurityBadges for the given context.
// Precedence: per-context session override (kb.SecurityBadgeToggle) > per-context
// config > global config. A toggle made in one context never leaks into another.
func (m *Model) recomputeHideSecurityBadges(ctx string) {
	if v, ok := m.contextBadgeOverrides[ctx]; ok {
		m.hideSecurityBadges = v
		return
	}
	m.hideSecurityBadges = ui.ResolveSecurityHideBadges(ctx)
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
// CLI flag stickiness is absolute: when --read-only was passed, both the
// picker toggle and the in-context toggle are rejected with a status
// hint. The flag is the strongest level of the precedence chain and
// cannot be defeated within the running process.
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
		if isUnionSetItem(sel) {
			m.setStatusMessage("Read-only toggle applies to contexts", true)
			return m, scheduleStatusClear()
		}
		newState := !sel.ReadOnly
		if m.contextROOverrides == nil {
			m.contextROOverrides = make(map[string]bool)
		}
		m.contextROOverrides[sel.Name] = newState
		// Update m.middleItems by index rather than via the sel pointer.
		// selectedMiddleItem may return a pointer into a transient filtered
		// slice on its fallback path; writing through that pointer would
		// not reach the cached middleItems below, leaving the [RO] marker
		// stale until the next refresh.
		for i := range m.middleItems {
			if m.middleItems[i].Name == sel.Name {
				m.middleItems[i].ReadOnly = newState
				break
			}
		}
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
	if m.cliReadOnly {
		m.setStatusMessage("--read-only forces all contexts read-only", true)
		return m, scheduleStatusClear()
	}
	m.readOnly = !m.readOnly
	// Persist the toggle to the per-context override so it survives re-entry
	// (recomputeReadOnly reads the override on context entry) and keeps the
	// cluster-picker [RO] marker in sync. Keyed by context, so unlocking one
	// context never leaks read-write state into another. The union sentinel
	// is skipped: it is not a real context and each member keeps its own
	// policy via readOnlyForContext.
	if !m.isUnionSentinel() && m.nav.Context != "" {
		if m.contextROOverrides == nil {
			m.contextROOverrides = make(map[string]bool)
		}
		m.contextROOverrides[m.nav.Context] = m.readOnly
	}
	state := "OFF"
	if m.readOnly {
		state = "ON"
	}
	m.setStatusMessage("Read-only mode: "+state, false)
	return m, scheduleStatusClear()
}
