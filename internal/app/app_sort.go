package app

import (
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// sortColumnIndex returns the index of sortColumnName in ActiveSortableColumns,
// or 0 if not found.
func sortColumnIndex(name string) int {
	for i, col := range ui.ActiveSortableColumns {
		if col == name {
			return i
		}
	}
	return 0
}

// applyKindSortDefault sets m.sortColumnName and m.sortAscending from the
// configured view's SortColumn for the given resource type and cluster
// context. Falls back to the built-in default ("Name", ascending) when no
// view is configured or the view has no sort_column. Synthetic kinds
// (port-forwards, captures, union dashboards) are no-ops — pass an empty
// ResourceRef.Kind or skip the call entirely at those sites.
func (m *Model) applyKindSortDefault(rt ui.ResourceRef, context string) {
	if v, ok := ui.ResolveView(rt, context); ok && v.SortColumn != "" {
		m.sortColumnName = v.SortColumn
		m.sortAscending = v.SortAsc
		return
	}
	m.sortColumnName = sortColDefault
	m.sortAscending = true
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
