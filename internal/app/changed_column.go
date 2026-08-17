package app

import (
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// ChangedColumnKey is the column that shows how long ago a row last changed.
// The name lives in ui because the sort-cycle builder needs it too.
const ChangedColumnKey = ui.ChangedColumnName

// changeAt returns when the object last changed. Comparing against an earlier
// poll instead would leave every row that changed before lfk started empty,
// which is nearly all of them. managedFields is the last resort: it says only
// that someone wrote, so it serves kinds with no conditions and no restarts.
func changeAt(it model.Item) (time.Time, bool) {
	last := it.LastRestartAt
	for _, c := range it.Conditions {
		if c.LastTransitionTime.After(last) {
			last = c.LastTransitionTime
		}
	}
	if !last.IsZero() {
		return last, true
	}
	return lastManagedFieldsWrite(it.Raw)
}

// lastManagedFieldsWrite returns the newest metadata.managedFields[].time.
func lastManagedFieldsWrite(raw map[string]any) (time.Time, bool) {
	meta, ok := raw["metadata"].(map[string]any)
	if !ok {
		return time.Time{}, false
	}
	entries, ok := meta["managedFields"].([]any)
	if !ok {
		return time.Time{}, false
	}
	var last time.Time
	for _, e := range entries {
		entry, ok := e.(map[string]any)
		if !ok {
			continue
		}
		stamp, ok := entry["time"].(string)
		if !ok {
			continue
		}
		t, err := time.Parse(time.RFC3339, stamp)
		if err == nil && t.After(last) {
			last = t
		}
	}
	return last, !last.IsZero()
}

// changeAge returns how long ago the item last changed.
func changeAge(it model.Item, now time.Time) (time.Duration, bool) {
	at, ok := changeAt(it)
	if !ok {
		return 0, false
	}
	return max(now.Sub(at), 0), true
}

// applyChangedColumn stamps the Change cell onto every item. The value is a
// duration string so the existing Uptime comparator orders it. A fresh slice,
// prepended: the detail pane renders slice order and tabs share the array.
func applyChangedColumn(items []model.Item, now time.Time) {
	for i := range items {
		value := ""
		if age, ok := changeAge(items[i], now); ok {
			value = k8s.FormatAge(age)
		}
		cols := make([]model.KeyValue, 0, len(items[i].Columns)+1)
		cols = append(cols, model.KeyValue{Key: ChangedColumnKey, Value: value})
		for _, kv := range items[i].Columns {
			if kv.Key != ChangedColumnKey {
				cols = append(cols, kv)
			}
		}
		items[i].Columns = cols
	}
}
