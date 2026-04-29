package ui

import (
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// LiveAge returns a freshly-computed age string from item.CreatedAt so
// the value moves forward between watch ticks and after explicit
// refreshes — without it, the precomputed Item.Age captured at load
// time would stay frozen until a real refetch (which the cache shortcut
// in loadResources can suppress for long stretches).
//
// Falls back to the precomputed Item.Age when CreatedAt isn't set
// (synthetic rows, helm-merged rows, etc.) so we don't regress those to
// "0s" or an empty string.
func LiveAge(item model.Item) string {
	if !item.CreatedAt.IsZero() {
		return k8s.FormatAge(time.Since(item.CreatedAt))
	}
	return item.Age
}
