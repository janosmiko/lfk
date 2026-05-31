// Package app — tabs_copy_helpers.go
// Deep-copy helpers used by tab persistence and cloning. Extracted from
// tabs.go to keep that file under the revive file-length-limit.
package app

import (
	"maps"

	"github.com/janosmiko/lfk/internal/model"
)

// copyMapStringInt deep copies a map[string]int.
func copyMapStringInt(m map[string]int) map[string]int {
	if m == nil {
		return make(map[string]int)
	}
	c := make(map[string]int, len(m))
	maps.Copy(c, m)
	return c
}

// copyMapStringBool deep copies a map[string]bool.
func copyMapStringBool(m map[string]bool) map[string]bool {
	if m == nil {
		return make(map[string]bool)
	}
	c := make(map[string]bool, len(m))
	maps.Copy(c, m)
	return c
}

// copyItemCache copies the item cache, isolating each tab's snapshot from
// later in-place mutation. model.Item's mutable reference-typed slices
// (Columns, Conditions, GroupedRefs) are cloned because the metrics and
// event-grouping paths append to them in place (e.g. update_metrics_msgs.go,
// events_group.go), which would otherwise write through a shared backing
// array into another tab's cache. Item.Raw is deliberately shared: it is the
// immutable source object, only ever re-read for JSONPath evaluation, so
// cloning it would waste memory for no isolation benefit.
func copyItemCache(m map[string][]model.Item) map[string][]model.Item {
	if m == nil {
		return make(map[string][]model.Item)
	}
	c := make(map[string][]model.Item, len(m))
	for k, v := range m {
		items := make([]model.Item, len(v))
		for i, it := range v {
			if it.Columns != nil {
				it.Columns = append([]model.KeyValue(nil), it.Columns...)
			}
			if it.Conditions != nil {
				it.Conditions = append([]model.ConditionEntry(nil), it.Conditions...)
			}
			if it.GroupedRefs != nil {
				it.GroupedRefs = append([]model.GroupedRef(nil), it.GroupedRefs...)
			}
			items[i] = it
		}
		c[k] = items
	}
	return c
}

// copyMapStringString returns a shallow copy of a string-to-string map.
// A nil input yields a non-nil empty map so callers can write into it
// without a second nil check.
func copyMapStringString(m map[string]string) map[string]string {
	if m == nil {
		return make(map[string]string)
	}
	c := make(map[string]string, len(m))
	maps.Copy(c, m)
	return c
}
