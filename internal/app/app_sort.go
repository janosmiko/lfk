package app

import (
	"time"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// sortColumnIndex returns the index of name in ActiveSortableColumns and
// whether it was found. Callers use the found flag to handle a sort column
// that is hidden in the current layout (e.g. a wide-only column shown only in
// fullscreen) rather than silently treating it as index 0 (issue #339).
func sortColumnIndex(name string) (int, bool) {
	for i, col := range ui.ActiveSortableColumns {
		if col == name {
			return i, true
		}
	}
	return 0, false
}

// sortPref records a user's chosen sort column and direction for a resource
// kind so it survives leaving and re-entering the list during a session.
type sortPref struct {
	column    string
	ascending bool
}

// sortMemoryKey builds the per-kind sort-memory key from a resource ref and
// cluster context, mirroring ResolveView's GVR-based scoping so a remembered
// sort lines up with the same kind across navigation.
func sortMemoryKey(rt ui.ResourceRef, context string) string {
	return context + "\x00" + rt.GVRKey()
}

// currentSortKey returns the sort-memory key for the resource kind currently
// in view, or false at navigation levels where sort has no effect or for
// synthetic and empty resource types (no GVR to key on).
func (m *Model) currentSortKey() (string, bool) {
	if !m.sortApplies() {
		return "", false
	}
	rt := m.nav.ResourceType
	if rt.Resource == "" {
		return "", false
	}
	ref := ui.ResourceRef{Group: rt.APIGroup, Version: rt.APIVersion, Resource: rt.Resource, Kind: rt.Kind}
	return sortMemoryKey(ref, m.nav.Context), true
}

// rememberSort stores the current sort column/direction for the resource kind
// currently in view, so re-entering that kind restores the user's choice.
// Several callers are value-receiver update handlers: this mutates the shared
// backing map in place, so it must never reassign m.sortMemory (a replacement
// would not propagate back to the caller's copy).
func (m *Model) rememberSort() {
	key, ok := m.currentSortKey()
	if !ok {
		return
	}
	if m.sortMemory == nil {
		m.sortMemory = make(map[string]sortPref)
	}
	pref := sortPref{column: m.sortColumnName, ascending: m.sortAscending}
	m.sortMemory[key] = pref
	persistRememberedSort(key, pref)
}

// forgetSort drops any remembered sort for the resource kind currently in
// view, so re-entering it falls back to the configured view default (or the
// built-in default). Used by the explicit sort-reset action — reset means
// "forget my customization", not "pin Name ascending forever".
func (m *Model) forgetSort() {
	if key, ok := m.currentSortKey(); ok {
		delete(m.sortMemory, key)
		persistForgottenSort(key)
	}
}

// applyKindSortDefault sets m.sortColumnName and m.sortAscending for the given
// resource type and cluster context. A sort the user chose earlier this
// session (recorded by rememberSort) takes precedence, then the configured
// view's SortColumn, then the built-in default ("Name", ascending). Synthetic
// kinds (port-forwards, captures, union dashboards) are no-ops — pass an empty
// ResourceRef.Kind or skip the call entirely at those sites.
func (m *Model) applyKindSortDefault(rt ui.ResourceRef, context string) {
	m.sortColumnName, m.sortAscending = m.kindSortPref(rt, context)
}

// kindSortPref resolves the sort column/direction a list of the given resource
// type renders with, without mutating the model: session sort memory first,
// then the configured view's SortColumn, then the built-in default ("Name",
// ascending). Shared by applyKindSortDefault and the right-pane list preview
// so the preview order matches the drilled-in list (issue #408).
func (m *Model) kindSortPref(rt ui.ResourceRef, context string) (column string, ascending bool) {
	if rt.Resource != "" {
		if p, ok := m.sortMemory[sortMemoryKey(rt, context)]; ok {
			return p.column, p.ascending
		}
	}
	if v, ok := ui.ResolveView(rt, context); ok && v.SortColumn != "" {
		return v.SortColumn, v.SortAsc
	}
	return sortColDefault, true
}

// sortPreviewItems sorts a right-pane preview list in place exactly as the
// drilled-in list of rt renders it — session sort memory, then view config,
// then Name ascending (issue #408). Synthetic previews (port-forwards,
// captures, union dashboards) carry a zero-valued rt and keep their fetch
// order.
func (m *Model) sortPreviewItems(items []model.Item, rt model.ResourceTypeEntry) {
	if rt.Resource == "" {
		return
	}
	ref := ui.ResourceRef{Group: rt.APIGroup, Version: rt.APIVersion, Resource: rt.Resource, Kind: rt.Kind}
	col, asc := m.kindSortPref(ref, m.nav.Context)
	applyChangedColumn(items, time.Now())
	sortItemsByColumn(items, col, asc, rt.Kind)
}

// applyResourceTypeSortDefault is a convenience wrapper around applyKindSortDefault
// for call sites that already hold a model.ResourceTypeEntry.
func (m *Model) applyResourceTypeSortDefault(rt model.ResourceTypeEntry, context string) {
	m.applyKindSortDefault(ui.ResourceRef{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
		Kind:     rt.Kind,
	}, context)
}
