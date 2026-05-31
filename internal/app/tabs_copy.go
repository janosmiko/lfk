package app

import (
	"maps"
)

// copyMapStringSortPref deep copies the per-kind sort memory.
func copyMapStringSortPref(m map[string]sortPref) map[string]sortPref {
	if m == nil {
		return make(map[string]sortPref)
	}
	c := make(map[string]sortPref, len(m))
	maps.Copy(c, m)
	return c
}
