package app

import (
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
