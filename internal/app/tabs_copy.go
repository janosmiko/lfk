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

// copyMapStringSortPref deep copies the per-kind sort memory.
func copyMapStringSortPref(m map[string]sortPref) map[string]sortPref {
	if m == nil {
		return make(map[string]sortPref)
	}
	c := make(map[string]sortPref, len(m))
	maps.Copy(c, m)
	return c
}

// copyItemCache deep copies the item cache.
func copyItemCache(m map[string][]model.Item) map[string][]model.Item {
	if m == nil {
		return make(map[string][]model.Item)
	}
	c := make(map[string][]model.Item, len(m))
	for k, v := range m {
		c[k] = append([]model.Item(nil), v...)
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
