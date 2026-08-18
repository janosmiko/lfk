package app

import (
	"time"

	"github.com/janosmiko/lfk/internal/model"
)

// compareUptimeItemCmp orders the Uptime column by the node boot time rather
// than by the rendered cell. FormatAge buckets a whole range of durations
// into one string ("5d" covers a full day), so two nodes inside one bucket
// fell through to name order and swapped position the moment one crossed the
// boundary. Rows with no boot time keep the string comparison: a CRD
// additionalPrinterColumn may also be named Uptime, and Prometheus does not
// match every node.
func compareUptimeItemCmp(a, b model.Item) int {
	if a.BootedAt.IsZero() || b.BootedAt.IsZero() {
		return compareUptimeCmp(getColumnValue(a, "Uptime"), getColumnValue(b, "Uptime"))
	}
	// A later boot is a shorter uptime, which sorts first ascending — the
	// same direction the string comparator gives.
	switch {
	case a.BootedAt.After(b.BootedAt):
		return -1
	case a.BootedAt.Before(b.BootedAt):
		return 1
	default:
		return 0
	}
}

// carryOverBootedAt copies the node boot time onto freshly loaded items, keyed
// exactly as carryOverMetricsColumnsFrom keys the Uptime cell it carries. A
// watch tick replaces every Item, and without this the sort drops back to the
// bucketed cell until the next Prometheus fetch lands.
func carryOverBootedAt(oldItems, newItems []model.Item) {
	type itemKey struct{ cluster, ns, name string }
	booted := make(map[itemKey]time.Time, len(oldItems))
	for _, item := range oldItems {
		if !item.BootedAt.IsZero() {
			booted[itemKey{item.ClusterName, item.Namespace, item.Name}] = item.BootedAt
		}
	}
	if len(booted) == 0 {
		return
	}
	for i := range newItems {
		if !newItems[i].BootedAt.IsZero() {
			continue
		}
		key := itemKey{newItems[i].ClusterName, newItems[i].Namespace, newItems[i].Name}
		if at, ok := booted[key]; ok {
			newItems[i].BootedAt = at
		}
	}
}
