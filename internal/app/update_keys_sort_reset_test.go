package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestSortReset_NameHiddenFallsBackToVisibleColumn verifies that resetting the
// sort while the Name column is hidden does not leave the sort key on the
// (now invisible) "Name" column. Next/Prev already guard a hidden sort column
// (issue #339); reset must stay coherent too so the status bar and header
// indicator agree with the visible columns.
func TestSortReset_NameHiddenFallsBackToVisibleColumn(t *testing.T) {
	prevCols := ui.ActiveSortableColumns
	prevCount := ui.ActiveSortableColumnCount
	t.Cleanup(func() {
		ui.ActiveSortableColumns = prevCols
		ui.ActiveSortableColumnCount = prevCount
	})
	// Name hidden -> absent from the sortable set.
	ui.ActiveSortableColumns = []string{"Namespace", "Age"}
	ui.ActiveSortableColumnCount = 2

	m := &Model{}
	m.nav.Level = model.LevelResources
	m.sortColumnName = "Age"
	m.sortAscending = false

	res, _, _ := m.handleExplorerActionKeySortReset()
	updated := res.(Model)

	if _, ok := sortColumnIndex(updated.sortColumnName); !ok {
		t.Fatalf("reset left sort on hidden column %q (not in %v)",
			updated.sortColumnName, ui.ActiveSortableColumns)
	}
}

// TestSortReset_NameVisibleResetsToName verifies the unchanged default path:
// when Name is visible, reset restores the Name sort key.
func TestSortReset_NameVisibleResetsToName(t *testing.T) {
	prevCols := ui.ActiveSortableColumns
	prevCount := ui.ActiveSortableColumnCount
	t.Cleanup(func() {
		ui.ActiveSortableColumns = prevCols
		ui.ActiveSortableColumnCount = prevCount
	})
	ui.ActiveSortableColumns = []string{"Name", "Namespace", "Age"}
	ui.ActiveSortableColumnCount = 3

	m := &Model{}
	m.nav.Level = model.LevelResources
	m.sortColumnName = "Age"
	m.sortAscending = false

	res, _, _ := m.handleExplorerActionKeySortReset()
	updated := res.(Model)

	if updated.sortColumnName != sortColDefault {
		t.Fatalf("sortColumnName = %q, want %q", updated.sortColumnName, sortColDefault)
	}
}
