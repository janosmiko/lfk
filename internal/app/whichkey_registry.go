package app

import (
	"slices"
	"strings"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// whichKeyGroup classifies a catalog entry. The panel renders one flat list
// with no section headers, the way neovim's which-key does; the category cue
// the headers used to be is the color each group tints its entries'
// DESCRIPTIONS with (whichKeyGroupStyles - the keys keep one shared accent).
// The group name reaches the screen only in the legend row at the foot of the
// panel (whichKeyLegendLine), drawn in that same color.
type whichKeyGroup string

const (
	wkActions   whichKeyGroup = "Actions"
	wkSelection whichKeyGroup = "Selection"
	wkViews     whichKeyGroup = "Views"
	wkSort      whichKeyGroup = "Sort"
	wkFilter    whichKeyGroup = "Filter"
	wkSettings  whichKeyGroup = "Settings"
)

// whichKeyGroupOrder is the declared set of catalog groups, listed
// most-reached-for first. It drives two things: the cluster order
// sortWhichKeyCells sorts by (each group's entries form one contiguous run in
// this order, so its color reads as a structural cue rather than noise), and
// the allowlist TestWhichKeyRegistry_EveryEntryHasADeclaredGroup checks every
// entry against. It is also the reading order for the catalog below.
func whichKeyGroupOrder() []whichKeyGroup {
	return []whichKeyGroup{wkActions, wkViews, wkFilter, wkSelection, wkSort, wkSettings}
}

// whichKeyAction is one row of the explorer's leader panel: a catalog entry
// whose predicates read explorer row state (wkCtx). Every other mode binds
// wkAction to its own context type — see whichkey_catalog.go.
type whichKeyAction = wkAction[*wkCtx]

// wkCtx carries the row state every predicate needs, resolved once per call.
// Predicates used to each call selectedMiddleItem() -> visibleMiddleItems(),
// which re-filters the whole list; at ~14 predicates that was 14 filter passes
// per render. unionSentinel and readOnly are included because multiple
// predicates (wkSingleCluster/wkUnionAllowed and wkWritable/wkWritableKindIn/
// PasteApply, respectively) each derive the same value from m.
type wkCtx struct {
	m             *Model
	sel           *model.Item
	kind          string
	level         model.Level
	unionSentinel bool
	readOnly      bool
}

// newWKCtx resolves the row, kind, level and the two shared context checks
// exactly once. Safe on a zero-value Model: selectedMiddleItem,
// selectedResourceKind, isUnionSentinel and readOnlyForContext all tolerate a
// zero Model without panicking (see TestWhichKeyPredicates_ZeroValueModelDoesNotPanic).
func newWKCtx(m *Model) *wkCtx {
	c := &wkCtx{m: m, level: m.nav.Level}
	c.sel = m.selectedMiddleItem()
	c.kind = m.selectedResourceKind()
	c.unionSentinel = m.isUnionSentinel()
	// Resolve read-only against the row's OWN source cluster. In union mode
	// nav.Context is the sentinel, and readOnlyForContext short-circuits to
	// writable whenever ctx == nav.Context — so passing nav.Context back in
	// offered Delete and Force delete on rows the handlers then refuse
	// (update_actions.go). Read c.sel.ClusterName directly rather than calling
	// effectiveContext(), which re-runs selectedMiddleItem() and would undo
	// the once-per-call row resolution this struct exists for.
	roCtx := m.nav.Context
	if c.sel != nil && c.sel.ClusterName != "" {
		roCtx = c.sel.ClusterName
	}
	c.readOnly = m.readOnlyForContext(roCtx)
	return c
}

// --- shared predicates ---

// wkRowSelected reports whether a row is highlighted, independent of
// navigation level. Handlers that only read the row's Name or visible
// columns (CopyName, the copy-as-YAML/JSON/table picker, CopyField) work
// identically at every level from LevelClusters through LevelContainers —
// see availableCopyFormats (copy_format.go), which explicitly documents
// "Clusters and ResourceTypes only support Table ... All other levels offer
// the full YAML / JSON / Table set" rather than refusing those levels.
func wkRowSelected(c *wkCtx) bool {
	return c.sel != nil
}

// wkLevelIn wraps a predicate with a navigation-level allowlist, so a
// handler's level scope is expressed once and read off its source rather
// than inherited by accident from a shared base like wkOnRow. Use this
// whenever a handler's level branches are narrower or wider than
// wkOnRow's default (LevelResources/LevelOwned/LevelContainers) — see the
// per-entry level audit in the Task 2 report.
func wkLevelIn(pred func(c *wkCtx) bool, levels ...model.Level) func(c *wkCtx) bool {
	allowed := make(map[model.Level]bool, len(levels))
	for _, lvl := range levels {
		allowed[lvl] = true
	}
	return func(c *wkCtx) bool {
		return pred(c) && allowed[c.level]
	}
}

// wkOnRow reports whether a resource row is highlighted at a level a direct
// kubectl-backed action (Describe/Edit/Logs/Force Delete/...) can act on.
// Most handlers in that family don't level-check explicitly; they rely on
// selectedResourceKind() returning "" (blocked by isVirtualResourceKind or
// an empty wkKindIn match) above LevelResources, so this stays a plain
// comparison rather than a wkLevelIn call for cheapness. Delete is the
// exception — directActionDelete (update_actions.go) opens with an explicit
// LevelContainers refusal — so its entry composes wkLevelIn on top.
func wkOnRow(c *wkCtx) bool {
	return c.level >= model.LevelResources && c.sel != nil
}

// wkActionMenuAvailable mirrors openActionMenu's dispatch (update_actions.go),
// which picks a different menu builder per level, each with its own row
// conditions, and silently returns an unchanged model when none apply. Spelled
// out rather than composed from the generic wrappers because the branches are
// alternatives, not stacked gates.
//
// One condition is deliberately approximated: openBulkSelectionMenu also
// requires selectedItemsList() to be non-empty, which is a full scan of the
// visible rows. hasSelection() only differs from it when every selected row is
// filtered out of view, so the cheap check is used instead.
func wkActionMenuAvailable(c *wkCtx) bool {
	sel := c.sel
	if sel == nil {
		return false
	}
	// Security rows get their own menu ahead of the level dispatch.
	if strings.HasPrefix(c.m.nav.ResourceType.Kind, "__security_") || strings.HasPrefix(sel.Kind, "__security_") {
		return true
	}
	if c.m.hasSelection() {
		return c.kind != ""
	}
	switch c.level {
	case model.LevelClusters:
		return !isUnionSetItem(sel)
	case model.LevelResourceTypes:
		return sel.Kind != "__collapsed_group__" && sel.Category != "Dashboards" && model.PinKeyFromRef(sel.Extra) != ""
	default:
		return c.kind != ""
	}
}

// wkWritable reports whether mutating actions apply: a row is highlighted and
// the active context is not read-only.
func wkWritable(c *wkCtx) bool {
	return wkOnRow(c) && !c.readOnly
}

// wkKindIn returns a predicate matching the highlighted row's kind.
func wkKindIn(kinds ...string) func(*wkCtx) bool {
	set := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		set[k] = true
	}
	return func(c *wkCtx) bool {
		if !wkOnRow(c) {
			return false
		}
		return set[c.kind]
	}
}

// wkWritableKindIn is wkKindIn plus the read-only gate.
func wkWritableKindIn(kinds ...string) func(*wkCtx) bool {
	kindOK := wkKindIn(kinds...)
	return func(c *wkCtx) bool {
		return kindOK(c) && !c.readOnly
	}
}

// wkSingleCluster wraps a predicate with the "requires a single cluster" gate
// several handlers enforce in union mode. Secret/ConfigMap editor, label
// editor, and paste & apply check isUnionSentinel() directly and block for
// every kind. Edit has no such direct check, but ends up equivalent: it goes
// through executeAction's isUnionAllowedActionForKind backstop (readonly.go),
// which has no "Edit" case and so falls to its default of false for every
// kind — i.e. also unconditionally blocked. Not folded into wkWritable
// itself: wkWritable is also shared by Delete, whose union gate genuinely is
// kind-conditional (isUnionAllowedActionForKind allows Delete for
// kind == "Pod"), so a blanket gate there would hide an entry that actually
// works — see wkUnionAllowed for that case.
func wkSingleCluster(pred func(c *wkCtx) bool) func(c *wkCtx) bool {
	return func(c *wkCtx) bool {
		return pred(c) && !c.unionSentinel
	}
}

// wkRealKind wraps a predicate with the isVirtualResourceKind gate the
// direct-action dispatchers (Describe/Edit/Delete) use to silently no-op on
// synthetic rows (port forwards, captures, security findings). Not folded
// into wkOnRow: SaveResource (the other remaining wkOnRow user besides the
// kubectl-backed actions) explicitly supports LevelResources/Owned/Containers
// regardless of kind (update_keys_actions.go), so it doesn't share this
// restriction.
func wkRealKind(pred func(c *wkCtx) bool) func(c *wkCtx) bool {
	return func(c *wkCtx) bool {
		return pred(c) && !isVirtualResourceKind(c.kind)
	}
}

// wkExcludeKind is the inverse of wkKindIn: it blocks a specific handful of
// kinds a handler rejects inline (not via isVirtualResourceKind). Kept
// separate from wkRealKind because the blocked set differs — e.g. the label
// editor rejects only "__port_forwards__" and "__captures__", not the wider
// virtual-kind set (blank kind, port-forward entries, security findings).
func wkExcludeKind(pred func(c *wkCtx) bool, kinds ...string) func(c *wkCtx) bool {
	blocked := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		blocked[k] = true
	}
	return func(c *wkCtx) bool {
		return pred(c) && !blocked[c.kind]
	}
}

// wkUnionAllowed wraps a predicate with the same kind-conditional union
// allowlist executeAction and the direct-action dispatchers consult as a
// backstop (isUnionAllowedActionForKind, readonly.go) — for actions like
// Delete and Force Delete that some kinds still permit at the union
// sentinel. Reuses the handler's own helper rather than restating its
// per-label kind rules here.
func wkUnionAllowed(pred func(c *wkCtx) bool, label string) func(c *wkCtx) bool {
	return func(c *wkCtx) bool {
		if !pred(c) {
			return false
		}
		if !c.unionSentinel {
			return true
		}
		return isUnionAllowedActionForKind(c.kind, label)
	}
}

// wkAtLevel returns a predicate matching an exact navigation level.
func wkAtLevel(lvl model.Level) func(*wkCtx) bool {
	return func(c *wkCtx) bool { return c.level == lvl }
}

// wkNotAtClusters excludes LevelClusters — the shared "requires a selected
// context" gate handleKeyNamespaceSelector, handleExplorerActionKeyAllNamespaces,
// and handleExplorerActionKeyCreateTemplate (update_keys.go,
// update_keys_actions.go) each open with.
func wkNotAtClusters(c *wkCtx) bool {
	return c.level != model.LevelClusters
}

// wkLevelResourcesUp reports whether the navigation is at LevelResources or
// deeper, with no row required. Shared by handleExplorerResourceMap (map
// view toggle), handleKeyColumnToggle (column visibility overlay), and
// handleExplorerActionKeyFilterPresets (update_keys_explorer.go,
// update_keys.go, update_keys_actions.go), none of which read the cursor row.
func wkLevelResourcesUp(c *wkCtx) bool {
	return c.level >= model.LevelResources
}

// wkNotFullscreenDashboard excludes the one state handleKeyFilter and
// handleKeySearch (update_keys_explorer.go) swallow the key for: the
// fullscreen dashboard scrolls a rendered preview, not a list, so there is
// nothing to filter or search.
func wkNotFullscreenDashboard(c *wkCtx) bool {
	return !c.m.fullscreenDashboard
}

// wkSortApplies delegates to Model.sortApplies (tabs.go), the exact gate
// handleExplorerActionKeySortNext/Prev/Flip/Reset (update_keys_sort.go) each
// check before touching the sort state. It already covers "no row needed"
// (sortMiddleItems on an empty list is a harmless no-op that still updates
// the remembered sort), so no further wrapping is needed.
func wkSortApplies(c *wkCtx) bool {
	return c.m.sortApplies()
}

// wkClusterRowSelected reports whether a real cluster context (not a union
// set) is highlighted at LevelClusters — the gate handleKeyClusterColorPicker
// (cluster_color_overlay.go) applies before opening the color overlay, and
// handleKeyReadOnlyToggle (readonly.go) applies identically for its
// LevelClusters branch.
func wkClusterRowSelected(c *wkCtx) bool {
	return c.level == model.LevelClusters && c.sel != nil && !isUnionSetItem(c.sel)
}

// wkPinnableResourceTypeRow mirrors the pin-eligibility check
// handleKeyPinGroup (update_keys_explorer.go) applies, which is also
// openResourceTypeActionMenu's own LevelResourceTypes condition
// (wkActionMenuAvailable): a collapsed-group header, the Dashboards
// pseudo-category, and any row PinKeyFromRef can't parse are all
// "cannot be pinned".
func wkPinnableResourceTypeRow(c *wkCtx) bool {
	if c.level != model.LevelResourceTypes || c.sel == nil {
		return false
	}
	return c.sel.Kind != "__collapsed_group__" && c.sel.Category != "Dashboards" && model.PinKeyFromRef(c.sel.Extra) != ""
}

// wkPinGroupAvailable mirrors handleKeyPinGroup (update_keys_explorer.go) in
// full: a pin-eligible row (wkPinnableResourceTypeRow) at LevelResourceTypes,
// plus its union-mode branch, which blocks pinning at the sentinel unless a
// named union set (m.unionSetName) is active.
func wkPinGroupAvailable(c *wkCtx) bool {
	if !wkPinnableResourceTypeRow(c) {
		return false
	}
	return !c.unionSentinel || c.m.unionSetName != ""
}

// wkCreateTemplateAvailable mirrors handleExplorerActionKeyCreateTemplate
// (update_keys_actions.go): blocked at LevelClusters (no selected context to
// create under), when read-only, or at the union sentinel — the same
// "requires a single cluster" shape as Edit/SecretEditor. Unlike those, no
// row is required: the template overlay lists choices independent of the
// cursor.
func wkCreateTemplateAvailable(c *wkCtx) bool {
	return wkNotAtClusters(c) && !c.readOnly && !c.unionSentinel
}

// wkAllNamespacesAvailable mirrors handleExplorerActionKeyAllNamespaces
// (update_keys_actions.go): blocked at LevelClusters and while union mode is
// active — checked via the raw m.unionMode flag rather than the
// unionSentinel context, because the toast fires for any navigated context
// while a union is active, not only at the sentinel.
func wkAllNamespacesAvailable(c *wkCtx) bool {
	return wkNotAtClusters(c) && !c.m.unionMode
}

// wkReadOnlyToggleAvailable mirrors handleKeyReadOnlyToggle (readonly.go):
// blocked outright when --read-only was set at startup (cliReadOnly, sticky
// for the process); at LevelClusters it additionally needs a selected
// context that isn't a union-set row (wkClusterRowSelected); every other
// level needs no row.
func wkReadOnlyToggleAvailable(c *wkCtx) bool {
	if c.m.cliReadOnly {
		return false
	}
	if c.level == model.LevelClusters {
		return c.sel != nil && !isUnionSetItem(c.sel)
	}
	return true
}

// wkTogglePreviewAvailable mirrors handleExplorerTogglePreview
// (update_keys_explorer.go): blocked only when hovering the Overview or
// Monitoring dashboard pseudo-item at LevelResourceTypes, a silent no-op in
// the handler; every other row and level toggles the preview.
func wkTogglePreviewAvailable(c *wkCtx) bool {
	if c.level != model.LevelResourceTypes || c.sel == nil {
		return true
	}
	return c.sel.Extra != "__overview__" && c.sel.Extra != "__monitoring__"
}

// wkTogglePreviewLogsAvailable mirrors handleExplorerToggleLogPreview via
// selectedPodForLogPreview (previewlog.go), plus the dispatch order that
// decides which handler actually sees the key: at LevelClusters, "L" is
// consumed unconditionally by ClusterColorPicker's case in
// handleExplorerNavKey (update_keys_explorer.go), which runs before
// handleExplorerUIKey (where TogglePreviewLogs is dispatched) — so the key
// never reaches handleExplorerToggleLogPreview there, regardless of
// fullLogPreview. Below LevelClusters: turning the preview OFF always
// succeeds, so once it is already on the key stays available regardless of
// kind; turning it ON only produces a real log stream for a Pod row, or a
// Container row with an owning pod name resolved (m.nav.OwnedName, set at
// LevelContainers).
func wkTogglePreviewLogsAvailable(c *wkCtx) bool {
	if c.level == model.LevelClusters {
		return false
	}
	if c.m.fullLogPreview {
		return true
	}
	return c.kind == "Pod" || (c.kind == "Container" && c.m.nav.OwnedName != "")
}

// wkAPIExplorerAvailable mirrors openExplainBrowser's level switch
// (update_explain.go): unavailable at LevelClusters; at LevelResourceTypes it
// additionally needs a selected row that isn't a collapsed-group header or
// one of the dashboard pseudo-items (kind or Extra "__overview__" /
// "__monitoring__"); LevelResources/Owned/Containers work off the navigated
// resource type and need no row.
func wkAPIExplorerAvailable(c *wkCtx) bool {
	switch c.level {
	case model.LevelResourceTypes:
		if c.sel == nil {
			return false
		}
		if c.sel.Kind == "__collapsed_group__" || c.sel.Kind == "__overview__" || c.sel.Kind == "__monitoring__" ||
			c.sel.Extra == "__overview__" || c.sel.Extra == "__monitoring__" {
			return false
		}
		return true
	case model.LevelResources, model.LevelOwned, model.LevelContainers:
		return true
	default:
		return false
	}
}

// wkOnSecurityView mirrors onSecurityView (update_actions_security_menu.go)
// exactly — same two checks, same source fields — but reads the already
// resolved c.sel instead of calling selectedMiddleItem() -> visibleMiddleItems()
// again, which is a full re-filter of the row list on every render (measured
// on BenchmarkAvailableWhichKeyActions/LevelResourcesFiltered at 200 rows:
// 211->420 allocs/op, 232KB->466KB, 36us->69us — exactly the cost wkCtx was
// introduced in Task 2.5 to eliminate). Reuses the same pattern already used
// inline in wkActionMenuAvailable above.
func wkOnSecurityView(c *wkCtx) bool {
	return strings.HasPrefix(c.m.nav.ResourceType.Kind, "__security_") ||
		(c.sel != nil && strings.HasPrefix(c.sel.Kind, "__security_"))
}

// wkNotOnSecurityView excludes a security view from a predicate. LabelEditor
// needs this: handleExplorerSecurityViewKeys (update_keys_actions_security.go)
// dispatches ahead of handleExplorerToolKeys (where LabelEditor is dispatched)
// and consumes the same default key ("i") whenever onSecurityView is true —
// SecurityIgnoreToggle deliberately reuses LabelEditor's binding, meaningless
// on a synthetic finding row. wkExcludeKind (LabelEditor's existing kind
// exclusion) can't express this: it blocks exact kinds via a map, not a
// "__security_" prefix across an open-ended set of source names.
func wkNotOnSecurityView(pred func(c *wkCtx) bool) func(c *wkCtx) bool {
	return func(c *wkCtx) bool {
		return pred(c) && !wkOnSecurityView(c)
	}
}

// wkFullscreenAvailable mirrors handleExplorerFullscreen (update_keys_explorer.go):
// unavailable only in the one branch that shows a toast instead of doing
// anything — hovering the Overview/Monitoring dashboard pseudo-item at
// LevelResourceTypes while in union mode ("Open dashboard members with
// right-arrow"). Every other row/level either toggles the dashboard
// fullscreen or cycles the three-pane layout unconditionally.
func wkFullscreenAvailable(c *wkCtx) bool {
	if c.level != model.LevelResourceTypes || c.sel == nil {
		return true
	}
	onDashboard := c.sel.Extra == "__overview__" || c.sel.Extra == "__monitoring__"
	return !onDashboard || !c.unionSentinel
}

// wkDiffAvailable mirrors handleExplorerActionKeyDiff (update_keys_actions.go):
// available from LevelResources down, and only when exactly two rows are
// selected — one or three-plus still "handles" the key but only to show a
// "select exactly 2" toast, so those counts are excluded the same way
// Delete/Edit exclude union-blocked kinds. len(m.selectedItems) is used
// instead of the handler's selectedItemsList() to avoid an extra
// visibleMiddleItems filter pass per render; the two differ only when a
// selected row has since scrolled out of the current filter, the same
// accepted approximation wkActionMenuAvailable documents for hasSelection().
func wkDiffAvailable(c *wkCtx) bool {
	return c.level >= model.LevelResources && len(c.m.selectedItems) == 2
}

// whichKeyExplorerActionList is the full catalog for explorer mode. Navigation
// bindings are absent by construction — the panel is for actions the user is
// unlikely to remember, not for h/j/k/l.
//
// The section comments below match the render order now: the panel is one
// flat list, no section headers, but sortWhichKeyCells clusters entries by
// Group (whichKeyGroupOrder) before its within-group key sort, so each
// section below lands as one contiguous, one-colored run on screen.
//
// Built once at package init rather than per call: the panel re-filters on
// every render, and rebuilding the catalog would re-run every wkKindIn /
// wkLevelIn / wkExcludeKind constructor and so reallocate their lookup sets
// each frame. Only the Key indirection stays dynamic, so a runtime rebind is
// still picked up.
var whichKeyExplorerActionList = []whichKeyAction{
	{Key: func(kb ui.Keybindings) string { return kb.ActionMenu }, Label: "Action menu", Group: wkActions, Avail: wkActionMenuAvailable},
	{Key: func(kb ui.Keybindings) string { return kb.Logs }, Label: "Logs (fullscreen)", Group: wkActions, Avail: wkKindIn("Pod", "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob")},
	{Key: func(kb ui.Keybindings) string { return kb.Describe }, Label: "Describe", Group: wkActions, Avail: wkRealKind(wkOnRow)},
	{Key: func(kb ui.Keybindings) string { return kb.Edit }, Label: "Edit in $EDITOR", Group: wkActions, Avail: wkSingleCluster(wkRealKind(wkWritable))},
	// LevelContainers is excluded: directActionDelete refuses it outright with
	// a toast, ahead of every other check (update_actions.go).
	{Key: func(kb ui.Keybindings) string { return kb.Delete }, Label: "Delete", Group: wkActions, Avail: wkLevelIn(wkUnionAllowed(wkRealKind(wkWritable), "Delete"), model.LevelResources, model.LevelOwned)},
	{Key: func(kb ui.Keybindings) string { return kb.Delete }, Label: "Remove port forward", Group: wkActions, Avail: wkLevelIn(wkKindIn("__port_forwards__"), model.LevelResources)},
	{Key: func(kb ui.Keybindings) string { return kb.ForceDelete }, Label: "Force delete", Group: wkActions, Avail: wkUnionAllowed(wkWritableKindIn("Pod", "Job"), "Force Delete")},
	{Key: func(kb ui.Keybindings) string { return kb.SecretEditor }, Label: "Secret/ConfigMap editor", Group: wkActions, Avail: wkLevelIn(wkSingleCluster(wkWritableKindIn("Secret", "ConfigMap")), model.LevelResources)},
	{Key: func(kb ui.Keybindings) string { return kb.LabelEditor }, Label: "Label/annotation editor", Group: wkActions, Avail: wkLevelIn(wkNotOnSecurityView(wkSingleCluster(wkExcludeKind(wkWritable, "__port_forwards__", "__captures__"))), model.LevelResources, model.LevelOwned)},
	{Key: func(kb ui.Keybindings) string { return kb.CopyName }, Label: "Copy name", Group: wkActions, Avail: wkRowSelected},
	{Key: func(kb ui.Keybindings) string { return kb.CopyYAML }, Label: "Copy as...", Group: wkActions, Avail: wkRowSelected},
	{Key: func(kb ui.Keybindings) string { return kb.CopyField }, Label: "Copy a field", Group: wkActions, Avail: wkRowSelected},
	{Key: func(kb ui.Keybindings) string { return kb.PasteApply }, Label: "Paste & apply", Group: wkActions, Avail: wkSingleCluster(func(c *wkCtx) bool { return !c.readOnly })},
	{Key: func(kb ui.Keybindings) string { return kb.OpenBrowser }, Label: "Open in browser", Group: wkActions, Avail: wkKindIn("Ingress", "__port_forwards__", "__port_forward_entry__")},
	{Key: func(kb ui.Keybindings) string { return kb.OpenBrowser }, Label: "Port forward & open", Group: wkActions, Avail: wkWritableKindIn("Service")},
	{Key: func(kb ui.Keybindings) string { return kb.SaveResource }, Label: "Save to file", Group: wkActions, Avail: wkOnRow},
	{Key: func(kb ui.Keybindings) string { return kb.Refresh }, Label: "Refresh view", Group: wkActions},
	{Key: func(kb ui.Keybindings) string { return kb.CreateTemplate }, Label: "Create from template", Group: wkActions, Avail: wkCreateTemplateAvailable},
	{Key: func(kb ui.Keybindings) string { return kb.Scale }, Label: "Scale", Group: wkActions, Avail: wkSingleCluster(wkWritableKindIn("Deployment", "StatefulSet", "ReplicaSet", "HorizontalPodAutoscaler"))},

	// Selection
	{Key: func(kb ui.Keybindings) string { return kb.ToggleSelect }, Label: "Toggle selection", Group: wkSelection, Avail: wkOnRow},
	// SelectAll's gate is level-only (handleKeySelectAll, update_keys.go):
	// unlike SelectRange, it never reads the cursor row, only
	// m.selectedItems (survives filtering) and visibleMiddleItems() directly,
	// so wkOnRow's "sel != nil" would wrongly hide it when a selection exists
	// but the filter has narrowed the visible list to zero rows.
	{Key: func(kb ui.Keybindings) string { return kb.SelectAll }, Label: "Select/deselect all", Group: wkSelection, Avail: wkLevelResourcesUp},
	{Key: func(kb ui.Keybindings) string { return kb.SelectRange }, Label: "Select range", Group: wkSelection, Avail: wkOnRow},
	{Key: func(kb ui.Keybindings) string { return kb.Diff }, Label: "Diff two selected", Group: wkSelection, Avail: wkDiffAvailable},

	// Views
	{Key: func(kb ui.Keybindings) string { return kb.TogglePreview }, Label: "Details / YAML preview", Group: wkViews, Avail: wkTogglePreviewAvailable},
	{Key: func(kb ui.Keybindings) string { return kb.TogglePreviewLogs }, Label: "Live log preview", Group: wkViews, Avail: wkTogglePreviewLogsAvailable},
	{Key: func(kb ui.Keybindings) string { return kb.ResourceMap }, Label: "Resource map", Group: wkViews, Avail: wkLevelResourcesUp},
	{Key: func(kb ui.Keybindings) string { return kb.ObjectExplorer }, Label: "Object Explorer", Group: wkViews, Avail: func(c *wkCtx) bool { return wkOnRow(c) && c.sel.Raw != nil }},
	{Key: func(kb ui.Keybindings) string { return kb.APIExplorer }, Label: "API Explorer", Group: wkViews, Avail: wkAPIExplorerAvailable},
	{Key: func(kb ui.Keybindings) string { return kb.RBACBrowser }, Label: "RBAC browser", Group: wkViews},
	{Key: func(kb ui.Keybindings) string { return kb.OrphanOverlay }, Label: "Orphan overview", Group: wkViews, Avail: func(c *wkCtx) bool { return !c.unionSentinel }},
	{Key: func(kb ui.Keybindings) string { return kb.SessionManager }, Label: "Session manager", Group: wkViews},
	{Key: func(kb ui.Keybindings) string { return kb.ColumnToggle }, Label: "Column visibility", Group: wkViews, Avail: wkLevelResourcesUp},
	{Key: func(kb ui.Keybindings) string { return kb.Monitoring }, Label: "Monitoring dashboard", Group: wkViews, Avail: func(c *wkCtx) bool { return c.level >= model.LevelResourceTypes }},
	{Key: func(kb ui.Keybindings) string { return kb.QuotaDashboard }, Label: "Quota dashboard", Group: wkViews},
	{Key: func(kb ui.Keybindings) string { return kb.TasksOverlay }, Label: "Task queue", Group: wkViews},
	{Key: func(kb ui.Keybindings) string { return kb.ErrorLog }, Label: "Error log", Group: wkViews},
	{Key: func(kb ui.Keybindings) string { return kb.FinalizerSearch }, Label: "Finalizer search", Group: wkViews},
	{Key: func(kb ui.Keybindings) string { return kb.Fullscreen }, Label: "Cycle layout", Group: wkViews, Avail: wkFullscreenAvailable},
	{Key: func(kb ui.Keybindings) string { return kb.PinGroup }, Label: "Pin/unpin type", Group: wkViews, Avail: wkPinGroupAvailable},
	{Key: func(kb ui.Keybindings) string { return kb.ToggleRare }, Label: "Show rare/hidden types", Group: wkViews},
	{Key: func(kb ui.Keybindings) string { return kb.LocalClusterManager }, Label: "Local cluster manager", Group: wkViews, Avail: wkAtLevel(model.LevelClusters)},
	{Key: func(kb ui.Keybindings) string { return kb.OpenMarks }, Label: "Bookmarks", Group: wkViews},

	// Filter
	{Key: func(kb ui.Keybindings) string { return kb.Filter }, Label: "Filter list", Group: wkFilter, Avail: wkNotFullscreenDashboard},
	{Key: func(kb ui.Keybindings) string { return kb.Search }, Label: "Search and jump", Group: wkFilter, Avail: wkNotFullscreenDashboard},
	{Key: func(kb ui.Keybindings) string { return kb.FilterPresets }, Label: "Filter presets", Group: wkFilter, Avail: wkLevelResourcesUp},
	{Key: func(kb ui.Keybindings) string { return kb.NamespaceSelector }, Label: "Namespace selector", Group: wkFilter, Avail: wkNotAtClusters},
	{Key: func(kb ui.Keybindings) string { return kb.AllNamespaces }, Label: "All namespaces", Group: wkFilter, Avail: wkAllNamespacesAvailable},
	{Key: func(kb ui.Keybindings) string { return kb.CommandBar }, Label: "Command bar", Group: wkFilter},

	// Sort. SortPrev/SortNext/SortFlip carry an explicit Order (USER DECISION:
	// "<" then ">" then "="): they're punctuation that ties on every other
	// comparator, so a plain ASCII compare split the "<"/">" pair apart with
	// "=" in between. SortReset ("-") is left unordered on purpose — it isn't
	// part of that pair, and the unset default (wkOrderRank) sorts it after
	// all three regardless.
	{Key: func(kb ui.Keybindings) string { return kb.SortPrev }, Label: "Sort previous column", Group: wkSort, Avail: wkSortApplies, Order: 1},
	{Key: func(kb ui.Keybindings) string { return kb.SortNext }, Label: "Sort next column", Group: wkSort, Avail: wkSortApplies, Order: 2},
	{Key: func(kb ui.Keybindings) string { return kb.SortFlip }, Label: "Flip sort direction", Group: wkSort, Avail: wkSortApplies, Order: 3},
	{Key: func(kb ui.Keybindings) string { return kb.SortReset }, Label: "Reset sort", Group: wkSort, Avail: wkSortApplies},

	// Settings
	{Key: func(kb ui.Keybindings) string { return kb.WatchMode }, Label: "Watch mode", Group: wkSettings},
	{Key: func(kb ui.Keybindings) string { return kb.ReadOnlyToggle }, Label: "Read-only mode", Group: wkSettings, Avail: wkReadOnlyToggleAvailable},
	{Key: func(kb ui.Keybindings) string { return kb.ThemeSelector }, Label: "Color scheme", Group: wkSettings},
	{Key: func(kb ui.Keybindings) string { return kb.TerminalToggle }, Label: "Terminal mode", Group: wkSettings},
	{Key: func(kb ui.Keybindings) string { return kb.MouseToggle }, Label: "Mouse capture", Group: wkSettings, Avail: func(c *wkCtx) bool { return c.m.mouseAvailable }},
	{Key: func(kb ui.Keybindings) string { return kb.SecretToggle }, Label: "Reveal secret values", Group: wkSettings},
	{Key: func(kb ui.Keybindings) string { return kb.SecurityBadgeToggle }, Label: "Security badge", Group: wkSettings},
	{Key: func(kb ui.Keybindings) string { return kb.SecurityIgnoreToggle }, Label: "Show ignored findings", Group: wkSettings, Avail: wkOnSecurityView},
	{Key: func(kb ui.Keybindings) string { return kb.ClusterColorPicker }, Label: "Cluster color", Group: wkSettings, Avail: wkClusterRowSelected},
	// Views, not Settings: this opens a screen, like every other entry in the
	// group. It sat in Settings until the help catalog's own row stopped
	// hardcoding f1 and the alignment guard could finally see the disagreement.
	{Key: whichKeyHelpKey, Label: "Full help", Group: wkViews},
}

// whichKeyExplorerCatalog is the explorer's registry entry. The explorer's own
// text-input modes (filter, search, command bar) are claimed by handleKey
// before the leader dispatch ever runs, so it declares no input hook.
var whichKeyExplorerCatalog = wkCatalog[*wkCtx]{
	resolve: newWKCtx,
	actions: whichKeyExplorerActionList,
}

// whichKeyHelpKey reports the key that actually opens the help screen. The
// leader is dispatched ahead of kb.Help in every mode that has a catalog, so
// when the two collide (both default to "?") only the hardcoded f1 alias still
// reaches help, and advertising "?" there would be a lie.
func whichKeyHelpKey(kb ui.Keybindings) string {
	return kb.HelpScreenKey()
}

// whichKeyExplorerActions returns a copy of the shared explorer catalog, so a
// caller cannot reorder or overwrite the package slice through it. Only tests
// call this; the render path reads whichKeyExplorerActionList directly, so the
// clone costs nothing per frame.
func whichKeyExplorerActions() []whichKeyAction {
	return slices.Clone(whichKeyExplorerActionList)
}

// whichKeyExcludedBindings lists the ui.Keybindings fields that deliberately do
// not appear in ANY mode's leader panel, keyed by Go field name. Navigation is
// excluded because the panel is a discovery aid for actions, not a full keymap;
// the remaining viewer-local keys belong to viewers that have no catalog yet.
// TestWhichKeyRegistry_CoversEveryBinding fails when a new binding is neither
// registered in some catalog nor listed here.
func whichKeyExcludedBindings() map[string]string {
	return map[string]string{
		// Navigation — excluded by design.
		"Left": "navigation", "Right": "navigation", "Down": "navigation", "Up": "navigation",
		"Enter": "navigation", "JumpTop": "navigation", "JumpBottom": "navigation",
		"PageDown": "navigation", "PageUp": "navigation", "PageForward": "navigation", "PageBack": "navigation",
		"LevelCluster": "navigation", "LevelTypes": "navigation", "LevelResources": "navigation",
		"PreviewDown": "navigation", "PreviewUp": "navigation",
		"JumpOwner": "navigation", "JumpBack": "navigation", "ExpandCollapse": "navigation",
		"NextMatch": "navigation within search", "PrevMatch": "navigation within search",

		// The leader itself: pressing it opens the panel rather than running a
		// listed action, so listing it would advertise the panel from inside
		// the panel.
		"WhichKeyLeader": "the leader key itself",
		// SetMark ("m") arms m.pendingMark and waits for the bookmark-slot
		// key (update_keys_explorer.go:26-33,330-332); unlike the g-prefix
		// (armWhichKey/renderWhichKey), nothing renders while pendingMark is
		// true — there is no popup, just a silent wait for the next key.
		"SetMark": "chord prefix, no rendered continuation",

		// Tabs: muscle-memory keys that would crowd out the actions.
		"NewTab": "tab management", "NextTab": "tab management", "PrevTab": "tab management",
		"MoveTabLeft": "tab management", "MoveTabRight": "tab management",

		// Goto chords (whichkey.go): each is a full "g<x>" chord dispatched by
		// handleGotoChord while the g prefix is armed, and already has its own
		// which-key-style popup (renderWhichKey) distinct from the leader
		// panel this registry drives.
		"GotoPods": "goto chord, has its own popup", "GotoDeployments": "goto chord, has its own popup",
		"GotoServices": "goto chord, has its own popup", "GotoNodes": "goto chord, has its own popup",
		"GotoNamespaces": "goto chord, has its own popup", "GotoIngresses": "goto chord, has its own popup",
		"GotoJobs": "goto chord, has its own popup", "GotoCronJobs": "goto chord, has its own popup",
		"GotoReplicaSets": "goto chord, has its own popup", "GotoDaemonSets": "goto chord, has its own popup",
		"GotoStatefulSets": "goto chord, has its own popup", "GotoConfigMaps": "goto chord, has its own popup",
		"GotoSecrets": "goto chord, has its own popup", "GotoHPAs": "goto chord, has its own popup",
		"GotoPVCs": "goto chord, has its own popup", "GotoPVs": "goto chord, has its own popup",
		"GotoPDBs": "goto chord, has its own popup",
		// PreviousNamespace ("g\\") is dispatched by the same handleGotoChord
		// and listed in the same goto popup (whichKeyCells), even though it
		// swaps namespace scope rather than switching resource type.
		"PreviousNamespace": "goto chord, has its own popup",

		// Restart and Exec are defined in DefaultKeybindings and read by
		// TestDefaultKeybindings, but no explorer dispatcher ever compares a
		// keypress against kb.Restart or kb.Exec: the "Restart"/"Exec" actions
		// (update_actions.go, update_actions_exec_pod.go) are reachable only
		// through the Action menu, whose per-kind items (model.ActionsForKind)
		// carry their own hardcoded quick-key hints independent of these
		// fields. Registering either here would advertise a keystroke that the
		// explorer never dispatches.
		"Restart": "not dispatched outside the action menu (dead binding)",
		"Exec":    "not dispatched outside the action menu (dead binding)",
	}
}
