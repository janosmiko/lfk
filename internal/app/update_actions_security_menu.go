// Package app — update_actions_security_menu.go
// Tailored action menu for security finding groups and affected resources.
// Replaces the kind-based k8s action menu when the user is on a security
// view (m.nav.ResourceType.Kind starts with __security_) or has a security
// item selected. Offers Ignore (Group), Ignore (This Resource), Un-ignore,
// and Refresh. Group / resource scope is per-cluster-context (see
// SecurityIgnoreState.Contexts), not cross-cluster.
package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// openSecurityActionMenuIfApplicable swaps in the security-tailored
// action menu when the user is inside a security view, returning ok=true
// to short-circuit the kind-based menu in openActionMenu.
func (m Model) openSecurityActionMenuIfApplicable() (Model, bool) {
	if strings.HasPrefix(m.nav.ResourceType.Kind, "__security_") {
		return m.openSecurityActionMenu(), true
	}
	if sel := m.selectedMiddleItem(); sel != nil && strings.HasPrefix(sel.Kind, "__security_") {
		return m.openSecurityActionMenu(), true
	}
	return m, false
}

// openSecurityActionMenu builds the security-specific action menu. The
// available entries depend on the selected row's Kind (group vs. affected
// resource) and the current ignore state for that row in the active context.
func (m Model) openSecurityActionMenu() Model {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m
	}
	var items []model.Item
	kctx := m.nav.Context
	sourceName := m.securitySourceForAction(sel)

	switch sel.Kind {
	case "__security_finding_group__":
		groupKey := sel.Extra
		ignored := isGroupIgnored(m.securityIgnores, kctx, sourceName, groupKey)
		if !ignored {
			items = append(items, model.Item{
				Name:   "Ignore (Group)",
				Extra:  "Hide all instances of this finding (scoped to this cluster)",
				Status: "i",
			})
		} else {
			items = append(items, model.Item{
				Name:   "Un-ignore",
				Extra:  "Stop ignoring this finding group",
				Status: "u",
			})
		}

	case "__security_affected_resource__":
		groupKey := sel.Extra
		resourceKey := sel.ColumnValue("__resource_key__")
		namespace := sel.Namespace
		groupIgnored := isGroupIgnored(m.securityIgnores, kctx, sourceName, groupKey)
		nsIgnored := isNamespaceIgnored(m.securityIgnores, kctx, sourceName, groupKey, namespace)
		// resourceIgnored is true when ANY scope (group / namespace / resource)
		// already hides this row, so the "Ignore (...)" entries only offer
		// strictly-broader scopes the user hasn't applied yet.
		resourceIgnored := isResourceIgnored(m.securityIgnores, kctx, sourceName, groupKey, resourceKey)
		if !groupIgnored {
			items = append(items, model.Item{
				Name:   "Ignore (Group)",
				Extra:  "Hide all instances of this finding (scoped to this cluster)",
				Status: "i",
			})
		}
		if namespace != "" && !groupIgnored && !nsIgnored {
			items = append(items, model.Item{
				Name:   "Ignore (Namespace)",
				Extra:  "Hide this finding for all resources in " + namespace,
				Status: "n",
			})
		}
		if !resourceIgnored && resourceKey != "" {
			items = append(items, model.Item{
				Name:   "Ignore (This Resource)",
				Extra:  "Hide this specific resource from the group",
				Status: "r",
			})
		}
		if resourceIgnored || groupIgnored {
			items = append(items, model.Item{
				Name:   "Un-ignore",
				Extra:  "Stop ignoring this entry",
				Status: "u",
			})
		}
	}

	items = append(items, model.Item{
		Name:   "Refresh",
		Extra:  "Reload security findings",
		Status: "R",
	})

	sortActionMenuItems(items)
	m.overlay = overlayAction
	m.overlayItems = items
	m.overlayCursor = 0
	return m
}

// executeSecurityIgnoreAction handles Ignore / Un-ignore actions emitted by
// openSecurityActionMenu. It replaces m.securityIgnores with a new
// SecurityIgnoreState (the add/remove helpers never mutate in place),
// dispatches the disk persistence asynchronously (so an fsync stall on slow disks does
// not freeze the UI), re-installs the IgnoreChecker on the client, and
// refreshes the current level so the user sees the result immediately — a
// cache-hit re-filter, not a re-scan (see the refresh return below).
// Persistence failures arrive via
// securityIgnoresSaveErrMsg and replace the optimistic success status with
// a clear error.
func (m Model) executeSecurityIgnoreAction(actionLabel string) (tea.Model, tea.Cmd) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	kctx := m.nav.Context
	groupKey := sel.Extra
	sourceName := m.securitySourceForAction(sel)
	// Never write an ignore rule under an empty source — it would be
	// unmatchable and silently hide nothing (or, worse, collide across
	// sources). Refuse rather than persist a misattributed rule.
	if sourceName == "" {
		m.setStatusMessage("Cannot determine security source", true)
		return m, scheduleStatusClear()
	}

	switch actionLabel {
	case "Ignore (Group)":
		m.securityIgnores = addSecurityIgnore(m.securityIgnores, kctx, SecurityIgnoreRule{
			Source:   sourceName,
			GroupKey: groupKey,
		})
		m.setStatusMessage("Ignored: "+groupKey, false)

	case "Ignore (Namespace)":
		namespace := sel.Namespace
		if namespace == "" {
			m.setStatusMessage("Cannot determine namespace", true)
			return m, scheduleStatusClear()
		}
		m.securityIgnores = addSecurityIgnore(m.securityIgnores, kctx, SecurityIgnoreRule{
			Source:    sourceName,
			GroupKey:  groupKey,
			Namespace: namespace,
		})
		m.setStatusMessage("Ignored in namespace "+namespace+": "+groupKey, false)

	case "Ignore (This Resource)":
		resourceKey := sel.ColumnValue("__resource_key__")
		if resourceKey == "" {
			m.setStatusMessage("Cannot determine resource key", true)
			return m, scheduleStatusClear()
		}
		m.securityIgnores = addSecurityIgnore(m.securityIgnores, kctx, SecurityIgnoreRule{
			Source:   sourceName,
			GroupKey: groupKey,
			Resource: resourceKey,
		})
		m.setStatusMessage("Ignored resource: "+resourceKey, false)

	case "Un-ignore":
		// Peel back the most specific matching rule first: resource, then
		// namespace, then the cluster-wide group rule. Config-file glob
		// ignores are read-only and intentionally not removable here.
		namespace, resourceKey, rk := "", "", ""
		if sel.Kind == "__security_affected_resource__" {
			rk = sel.ColumnValue("__resource_key__")
			switch {
			case isResourceSpecificIgnored(m.securityIgnores, kctx, sourceName, groupKey, rk):
				resourceKey = rk
			case isNamespaceIgnored(m.securityIgnores, kctx, sourceName, groupKey, sel.Namespace):
				namespace = sel.Namespace
			}
		}
		m.securityIgnores = removeSecurityIgnore(m.securityIgnores, kctx, sourceName, groupKey, namespace, resourceKey)
		msg := "Un-ignored: " + groupKey
		// If a broader rule (or a read-only config pattern) still hides this
		// row, say so — a bare "Un-ignored" would be misleading.
		if rk != "" && isResourceIgnored(m.securityIgnores, kctx, sourceName, groupKey, rk) {
			msg += " (still hidden by a broader rule)"
		}
		m.setStatusMessage(msg, false)
	}

	if m.client != nil {
		m.client.SetIgnoreChecker(newModelIgnoreChecker(m.securityIgnores, kctx))
	}
	// No manager-cache invalidation: ignoring changes only the checker, which
	// groupFindings applies AFTER FetchAll's (filter-independent) cache. The
	// refresh re-filters from cache instantly; invalidating would force a slow
	// full re-scan. Explicit "Refresh" still invalidates (see dispatch).
	return m, tea.Batch(saveSecurityIgnoresCmd(m.securityIgnores), m.refreshCurrentLevel(), scheduleStatusClear())
}

// updateSecurityIgnoresSaveErr handles the async failure path from
// saveSecurityIgnoresCmd. The optimistic status message set in
// executeSecurityIgnoreAction is overwritten with an error so the user
// learns the rule did not persist to disk.
func (m Model) updateSecurityIgnoresSaveErr(msg securityIgnoresSaveErrMsg) (tea.Model, tea.Cmd) {
	logger.Info("Failed to save security ignores", "error", msg.err)
	m.setStatusMessage("Failed to save ignore rule", true)
	return m, scheduleStatusClear()
}

// dispatchSecurityActionIfApplicable routes the action labels emitted by
// openSecurityActionMenu (Ignore variants, Un-ignore, Refresh) when the
// active navigation is inside a security view. Returning ok=false leaves
// the rest of the dispatch chain (k8s actions, custom actions) untouched
// so labels that happen to be named "Refresh" elsewhere keep their normal
// semantics.
func (m Model) dispatchSecurityActionIfApplicable(actionLabel string) (tea.Model, tea.Cmd, bool) {
	if !onSecurityView(&m) {
		return m, nil, false
	}
	switch actionLabel {
	case "Ignore (Group)", "Ignore (Namespace)", "Ignore (This Resource)", "Un-ignore":
		mdl, cmd := m.executeSecurityIgnoreAction(actionLabel)
		return mdl, cmd, true
	case "Refresh":
		if m.securityManager != nil {
			m.securityManager.Invalidate()
		}
		return m, m.refreshCurrentLevel(), true
	}
	return m, nil, false
}

// onSecurityView reports whether the current navigation is inside a
// security source (sidebar item or finding row). Gates the ignore-action
// dispatch so labels never collide with k8s-resource actions of the same
// name elsewhere in the explorer.
func onSecurityView(m *Model) bool {
	if strings.HasPrefix(m.nav.ResourceType.Kind, "__security_") {
		return true
	}
	if sel := m.selectedMiddleItem(); sel != nil && strings.HasPrefix(sel.Kind, "__security_") {
		return true
	}
	return false
}

// securitySourceForAction resolves the security source id for an ignore/un-ignore
// action. It prefers the navigated resource type's kind
// ("__security_<source>__"), and when that is not a security source — e.g. the
// menu was opened via the selected-item fallback while the nav kind is a normal
// resource — falls back to the selected row: its hidden __source__ column
// (finding-group and affected-resource rows carry it) or, for a sidebar source
// entry, its "__security_<source>__" kind. Returns "" only when no source can
// be determined, so callers refuse to write a misattributed (empty-source)
// rule. Note securitySourceFromKind matches the sentinel kinds
// ("__security_finding_group__" etc.) too, so those are handled via __source__
// and excluded from the kind-based fallback.
func (m Model) securitySourceForAction(sel *model.Item) string {
	if s := securitySourceFromKind(m.nav.ResourceType.Kind); s != "" {
		return s
	}
	if sel == nil {
		return ""
	}
	if s := sel.ColumnValue("__source__"); s != "" {
		return s
	}
	switch sel.Kind {
	case "__security_finding_group__", "__security_affected_resource__":
		return ""
	default:
		return securitySourceFromKind(sel.Kind)
	}
}

// securitySourceFromKind extracts the source name from a security RT kind
// like "__security_heuristic__" -> "heuristic". Returns "" for malformed
// inputs so callers degrade safely (the action menu just falls back to a
// no-op Refresh entry rather than mis-identifying the source).
func securitySourceFromKind(kind string) string {
	const prefix = "__security_"
	const suffix = "__"
	if !strings.HasPrefix(kind, prefix) || !strings.HasSuffix(kind, suffix) {
		return ""
	}
	return kind[len(prefix) : len(kind)-len(suffix)]
}
