package app

import (
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// whichKeyGroup labels one section of the space-leader panel.
type whichKeyGroup string

const (
	wkActions   whichKeyGroup = "Actions"
	wkSelection whichKeyGroup = "Selection"
	wkViews     whichKeyGroup = "Views"
	wkSort      whichKeyGroup = "Sort"
	wkFilter    whichKeyGroup = "Filter"
	wkSettings  whichKeyGroup = "Settings"
)

// whichKeyGroupOrder is the fixed render order. Later groups are the first to be
// dropped when the panel does not fit.
func whichKeyGroupOrder() []whichKeyGroup { //nolint:unused // wired by the Task 4 renderer
	return []whichKeyGroup{wkActions, wkSelection, wkViews, wkFilter, wkSort, wkSettings}
}

// whichKeyAction is one row of the space-leader panel. Key is a function so a
// rebind is picked up at render time rather than baked in at package init.
// Avail is nil for entries that always apply in the explorer; it must be cheap
// and side-effect free because it runs on every render.
type whichKeyAction struct {
	Key   func(kb ui.Keybindings) string
	Label string
	Group whichKeyGroup
	Avail func(m *Model) bool
}

// --- shared predicates ---

// wkRowSelected reports whether a row is highlighted, independent of
// navigation level. Handlers that only read the row's Name or visible
// columns (CopyName, the copy-as-YAML/JSON/table picker, CopyField) work
// identically at every level from LevelClusters through LevelContainers —
// see availableCopyFormats (copy_format.go), which explicitly documents
// "Clusters and ResourceTypes only support Table ... All other levels offer
// the full YAML / JSON / Table set" rather than refusing those levels.
func wkRowSelected(m *Model) bool {
	return m.selectedMiddleItem() != nil
}

// wkLevelIn wraps a predicate with a navigation-level allowlist, so a
// handler's level scope is expressed once and read off its source rather
// than inherited by accident from a shared base like wkOnRow. Use this
// whenever a handler's level branches are narrower or wider than
// wkOnRow's default (LevelResources/LevelOwned/LevelContainers) — see the
// per-entry level audit in the Task 2 report.
func wkLevelIn(pred func(m *Model) bool, levels ...model.Level) func(m *Model) bool {
	allowed := make(map[model.Level]bool, len(levels))
	for _, lvl := range levels {
		allowed[lvl] = true
	}
	return func(m *Model) bool {
		return pred(m) && allowed[m.nav.Level]
	}
}

// wkOnRow reports whether a resource row is highlighted at a level a direct
// kubectl-backed action (Describe/Edit/Delete/Logs/Force Delete/...) can
// act on. Handlers in that family don't level-check explicitly; they rely
// on selectedResourceKind() returning "" (blocked by isVirtualResourceKind
// or an empty wkKindIn match) above LevelResources, so this stays a plain
// comparison rather than a wkLevelIn call for cheapness.
func wkOnRow(m *Model) bool {
	return m.nav.Level >= model.LevelResources && m.selectedMiddleItem() != nil
}

// wkWritable reports whether mutating actions apply: a row is highlighted and
// the active context is not read-only.
func wkWritable(m *Model) bool {
	return wkOnRow(m) && !m.readOnlyForContext(m.nav.Context)
}

// wkKindIn returns a predicate matching the highlighted row's kind.
func wkKindIn(kinds ...string) func(*Model) bool {
	set := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		set[k] = true
	}
	return func(m *Model) bool {
		if !wkOnRow(m) {
			return false
		}
		return set[m.selectedResourceKind()]
	}
}

// wkWritableKindIn is wkKindIn plus the read-only gate.
func wkWritableKindIn(kinds ...string) func(*Model) bool {
	kindOK := wkKindIn(kinds...)
	return func(m *Model) bool {
		return kindOK(m) && !m.readOnlyForContext(m.nav.Context)
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
func wkSingleCluster(pred func(m *Model) bool) func(m *Model) bool {
	return func(m *Model) bool {
		return pred(m) && !m.isUnionSentinel()
	}
}

// wkRealKind wraps a predicate with the isVirtualResourceKind gate the
// direct-action dispatchers (Describe/Edit/Delete) use to silently no-op on
// synthetic rows (port forwards, captures, security findings). Not folded
// into wkOnRow: SaveResource (the other remaining wkOnRow user besides the
// kubectl-backed actions) explicitly supports LevelResources/Owned/Containers
// regardless of kind (update_keys_actions.go), so it doesn't share this
// restriction.
func wkRealKind(pred func(m *Model) bool) func(m *Model) bool {
	return func(m *Model) bool {
		return pred(m) && !isVirtualResourceKind(m.selectedResourceKind())
	}
}

// wkExcludeKind is the inverse of wkKindIn: it blocks a specific handful of
// kinds a handler rejects inline (not via isVirtualResourceKind). Kept
// separate from wkRealKind because the blocked set differs — e.g. the label
// editor rejects only "__port_forwards__" and "__captures__", not the wider
// virtual-kind set (blank kind, port-forward entries, security findings).
func wkExcludeKind(pred func(m *Model) bool, kinds ...string) func(m *Model) bool {
	blocked := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		blocked[k] = true
	}
	return func(m *Model) bool {
		return pred(m) && !blocked[m.selectedResourceKind()]
	}
}

// wkUnionAllowed wraps a predicate with the same kind-conditional union
// allowlist executeAction and the direct-action dispatchers consult as a
// backstop (isUnionAllowedActionForKind, readonly.go) — for actions like
// Delete and Force Delete that some kinds still permit at the union
// sentinel. Reuses the handler's own helper rather than restating its
// per-label kind rules here.
func wkUnionAllowed(pred func(m *Model) bool, label string) func(m *Model) bool {
	return func(m *Model) bool {
		if !pred(m) {
			return false
		}
		if !m.isUnionSentinel() {
			return true
		}
		return isUnionAllowedActionForKind(m.selectedResourceKind(), label)
	}
}

// whichKeyExplorerActions is the full catalog for explorer mode. Navigation
// bindings are absent by construction — the panel is for actions the user is
// unlikely to remember, not for h/j/k/l.
func whichKeyExplorerActions() []whichKeyAction {
	return []whichKeyAction{
		{Key: func(kb ui.Keybindings) string { return kb.ActionMenu }, Label: "Action menu", Group: wkActions, Avail: wkOnRow},
		{Key: func(kb ui.Keybindings) string { return kb.Logs }, Label: "Logs (fullscreen)", Group: wkActions, Avail: wkKindIn("Pod", "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob")},
		{Key: func(kb ui.Keybindings) string { return kb.Describe }, Label: "Describe", Group: wkActions, Avail: wkRealKind(wkOnRow)},
		{Key: func(kb ui.Keybindings) string { return kb.Edit }, Label: "Edit in $EDITOR", Group: wkActions, Avail: wkSingleCluster(wkRealKind(wkWritable))},
		{Key: func(kb ui.Keybindings) string { return kb.Delete }, Label: "Delete", Group: wkActions, Avail: wkUnionAllowed(wkRealKind(wkWritable), "Delete")},
		{Key: func(kb ui.Keybindings) string { return kb.Delete }, Label: "Remove port forward", Group: wkActions, Avail: wkLevelIn(wkKindIn("__port_forwards__"), model.LevelResources)},
		{Key: func(kb ui.Keybindings) string { return kb.ForceDelete }, Label: "Force delete", Group: wkActions, Avail: wkUnionAllowed(wkWritableKindIn("Pod", "Job"), "Force Delete")},
		{Key: func(kb ui.Keybindings) string { return kb.SecretEditor }, Label: "Secret/ConfigMap editor", Group: wkActions, Avail: wkLevelIn(wkSingleCluster(wkWritableKindIn("Secret", "ConfigMap")), model.LevelResources)},
		{Key: func(kb ui.Keybindings) string { return kb.LabelEditor }, Label: "Label/annotation editor", Group: wkActions, Avail: wkLevelIn(wkSingleCluster(wkExcludeKind(wkWritable, "__port_forwards__", "__captures__")), model.LevelResources, model.LevelOwned)},
		{Key: func(kb ui.Keybindings) string { return kb.CopyName }, Label: "Copy name", Group: wkActions, Avail: wkRowSelected},
		{Key: func(kb ui.Keybindings) string { return kb.CopyYAML }, Label: "Copy as (YAML/JSON/table)", Group: wkActions, Avail: wkRowSelected},
		{Key: func(kb ui.Keybindings) string { return kb.CopyField }, Label: "Copy a field", Group: wkActions, Avail: wkRowSelected},
		{Key: func(kb ui.Keybindings) string { return kb.PasteApply }, Label: "Apply from clipboard", Group: wkActions, Avail: wkSingleCluster(func(m *Model) bool { return !m.readOnlyForContext(m.nav.Context) })},
		{Key: func(kb ui.Keybindings) string { return kb.OpenBrowser }, Label: "Open in browser", Group: wkActions, Avail: wkKindIn("Ingress", "__port_forwards__", "__port_forward_entry__")},
		{Key: func(kb ui.Keybindings) string { return kb.OpenBrowser }, Label: "Port forward & open", Group: wkActions, Avail: wkWritableKindIn("Service")},
		{Key: func(kb ui.Keybindings) string { return kb.SaveResource }, Label: "Save to file", Group: wkActions, Avail: wkOnRow},
		{Key: func(kb ui.Keybindings) string { return kb.Refresh }, Label: "Refresh view", Group: wkActions},
	}
}

// availableWhichKeyActions filters the explorer catalog to what applies right
// now, dropping entries whose binding the user cleared.
func (m Model) availableWhichKeyActions() []whichKeyAction {
	kb := ui.ActiveKeybindings
	all := whichKeyExplorerActions()
	out := make([]whichKeyAction, 0, len(all))
	for _, a := range all {
		if a.Key(kb) == "" {
			continue
		}
		if a.Avail != nil && !a.Avail(&m) {
			continue
		}
		out = append(out, a)
	}
	return out
}
