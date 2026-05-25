package app

import (
	"github.com/janosmiko/lfk/internal/logger"
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
	customCount := 0
	hits := 0
	missingRaw := 0
	for i := range items {
		raw := items[i].Raw
		if raw == nil {
			missingRaw++
			continue
		}
		for _, col := range view.Columns {
			if !col.IsCustom() || col.Compiled == nil {
				continue
			}
			customCount++
			val := ui.EvalCompiled(col.Compiled, raw)
			if val == "" {
				continue
			}
			hits++
			items[i].Columns = append(items[i].Columns, model.KeyValue{Key: col.Name, Value: val})
		}
	}
	if customCount > 0 {
		logger.Debug("applied view columns",
			"items", len(items),
			"custom_evals", customCount,
			"hits", hits,
			"missing_raw", missingRaw)
	}
}
