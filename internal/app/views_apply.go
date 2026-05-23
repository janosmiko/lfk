package app

import (
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// applyViewColumns appends custom JSONPath-derived columns from view to each
// item, evaluating the compiled expression against item.Raw. Built-in columns
// in the view are NOT applied here — they're emitted by the populator. A nil
// view, an item with nil Raw, or an empty JSONPath result are all no-ops.
//
// Designed to run after the populator and before sort/filter so the custom
// columns are available as sort keys and filter targets.
func applyViewColumns(items []model.Item, view *ui.View) {
	if view == nil || len(view.Columns) == 0 {
		return
	}
	for i := range items {
		raw := items[i].Raw
		if raw == nil {
			continue
		}
		for _, col := range view.Columns {
			if !col.IsCustom() || col.Compiled == nil {
				continue
			}
			val := ui.EvalCompiled(col.Compiled, raw)
			if val == "" {
				continue
			}
			items[i].Columns = append(items[i].Columns, model.KeyValue{Key: col.Name, Value: val})
		}
	}
}
