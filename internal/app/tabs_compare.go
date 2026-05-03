package app

import (
	"sort"
	"strconv"
	"strings"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// sortMiddleItems sorts middleItems based on the current sort column and direction.
// At LevelResourceTypes and LevelClusters, items keep their original ordering.
func (m *Model) sortMiddleItems() {
	if !m.sortApplies() {
		return
	}

	cols := ui.ActiveSortableColumns
	if len(cols) == 0 {
		return
	}

	colName := m.sortColumnName
	if colName == "" {
		// Production always seeds sortColumnName with sortColDefault in
		// NewModel; an empty value here means a test fixture that built
		// a bare Model{} literal. Skip sorting in that case — otherwise
		// the tiebreaker below would impose a deterministic order on
		// items the caller may want to keep in their original sequence.
		return
	}
	asc := m.sortAscending

	// Events default to LastSeen ordering (most recent first) when the
	// user hasn't explicitly chosen a different column. The override
	// uses a sentinel that comparePrimaryColumn recognizes, without
	// injecting "Last Seen" into the sortable-column cycle.
	if colName == sortColDefault && m.nav.ResourceType.Kind == "Event" {
		colName = sortColEventLastSeen
	}

	m.middleItemsRev++
	sort.SliceStable(m.middleItems, func(i, j int) bool {
		a, b := m.middleItems[i], m.middleItems[j]

		// Primary comparison on the selected column.
		primary := comparePrimaryColumn(a, b, colName)
		if primary != 0 {
			if asc {
				return primary < 0
			}
			return primary > 0
		}

		// Tiebreaker: items with identical primary keys fall through to a
		// stable chain that is always ascending, regardless of the
		// primary's asc/desc flag. The chain is primary-aware: the
		// identity triple (Name, Namespace, Age) forms the main fallback
		// in that order, with whichever of those three is already the
		// primary column skipped so the tiebreaker doesn't redo work
		// the primary already did. Kind and Extra are appended as
		// absolute final discriminators.
		//
		// Without this, watch-mode refreshes would reshuffle rows with
		// identical primary keys (e.g. a Helm release "traefik" deployed
		// to multiple namespaces), because k8s API list calls can return
		// items in different orders and sort.SliceStable would then
		// preserve that shifting order.
		return itemTiebreakerLess(a, b, colName)
	})
}

// itemTiebreakerLess defines a total order on model.Item used as a sort
// tiebreaker. Always ascending — independent of the primary sort's asc
// flag — so identical primary keys land in a deterministic order across
// refreshes whether the user is sorting ascending or descending.
//
// The chain is primary-aware: (Name, Namespace, Age) participates in
// that order, with whichever of those three is the current primary
// column excluded. Kind and Extra act as final fallbacks so rows with
// truly identical identity still have a stable order.
//
//	primary=Name      → (Namespace, Age,   Kind, Extra)
//	primary=Namespace → (Name,      Age,   Kind, Extra)
//	primary=Age       → (Name,      Namespace, Kind, Extra)
//	primary=anything  → (Name, Namespace, Age, Kind, Extra)
func itemTiebreakerLess(a, b model.Item, primaryCol string) bool {
	if primaryCol != "Name" {
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c < 0
		}
	}
	if primaryCol != "Namespace" {
		if c := strings.Compare(strings.ToLower(a.Namespace), strings.ToLower(b.Namespace)); c != 0 {
			return c < 0
		}
	}
	if primaryCol != "Age" {
		if c := compareAgeCmp(a, b); c != 0 {
			return c < 0
		}
	}
	if c := strings.Compare(a.Kind, b.Kind); c != 0 {
		return c < 0
	}
	return a.Extra < b.Extra
}

// comparePrimaryColumn returns -1, 0, or +1 for a < b, a == b, a > b
// according to the selected sort column. Returning 0 lets the caller run
// a tiebreaker chain instead of relying on sort.SliceStable's input-order
// preservation.
func comparePrimaryColumn(a, b model.Item, colName string) int {
	switch colName {
	case "Name":
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	case "Namespace":
		return strings.Compare(strings.ToLower(a.Namespace), strings.ToLower(b.Namespace))
	case "Ready":
		return compareReadyCmp(a.Ready, b.Ready)
	case "Restarts":
		return compareNumericCmp(a.Restarts, b.Restarts)
	case "Status":
		if c := cmpInt(statusPriority(a.Status), statusPriority(b.Status)); c != 0 {
			return c
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	case "Age":
		return compareAgeCmp(a, b)
	case sortColEventLastSeen:
		return compareLastSeenCmp(a, b)
	default:
		return compareColumnValuesCmp(getColumnValue(a, colName), getColumnValue(b, colName), colName)
	}
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func cmpInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func compareReady(a, b string) bool {
	return compareReadyCmp(a, b) < 0
}

func compareReadyCmp(a, b string) int {
	return cmpFloat(parseReadyRatio(a), parseReadyRatio(b))
}

func parseReadyRatio(s string) float64 {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return 0
	}
	num, _ := strconv.ParseFloat(parts[0], 64)
	den, _ := strconv.ParseFloat(parts[1], 64)
	if den == 0 {
		return 0
	}
	return num / den
}

func compareNumeric(a, b string) bool {
	return compareNumericCmp(a, b) < 0
}

func compareNumericCmp(a, b string) int {
	na, _ := strconv.Atoi(strings.TrimSpace(a))
	nb, _ := strconv.Atoi(strings.TrimSpace(b))
	return cmpInt(na, nb)
}

func compareResourceValues(a, b, col string) bool {
	return compareResourceValuesCmp(a, b, col) < 0
}

func compareResourceValuesCmp(a, b, col string) int {
	isCPU := strings.HasPrefix(col, "CPU")
	return cmpInt64(ui.ParseResourceValue(a, isCPU), ui.ParseResourceValue(b, isCPU))
}

// compareAgeCmp returns the three-way age comparison with zero-time
// values sorted last and newer timestamps sorted first ("ascending" age
// means newest-first in the UI).
func compareAgeCmp(a, b model.Item) int {
	aZero := a.CreatedAt.IsZero()
	bZero := b.CreatedAt.IsZero()
	switch {
	case aZero && bZero:
		return strings.Compare(a.Name, b.Name)
	case aZero:
		return 1 // zero sorts after any real time
	case bZero:
		return -1
	}
	// Newer timestamps are "less" (render higher in ascending view).
	switch {
	case a.CreatedAt.After(b.CreatedAt):
		return -1
	case a.CreatedAt.Before(b.CreatedAt):
		return 1
	default:
		return 0
	}
}

// compareLastSeenCmp compares by the LastSeen timestamp (Events only).
// Most recent observation sorts first in ascending mode, matching the
// natural expectation of "what happened most recently" at the top.
func compareLastSeenCmp(a, b model.Item) int {
	aZero := a.LastSeen.IsZero()
	bZero := b.LastSeen.IsZero()
	switch {
	case aZero && bZero:
		return strings.Compare(a.Name, b.Name)
	case aZero:
		return 1
	case bZero:
		return -1
	}
	switch {
	case a.LastSeen.After(b.LastSeen):
		return -1
	case a.LastSeen.Before(b.LastSeen):
		return 1
	default:
		return 0
	}
}

// compareColumnValuesCmp compares two column values with automatic detection
// of resource quantities (10Gi, 500Mi, 100m), plain numbers, and strings.
// Returns -1, 0, or +1 so sort.SliceStable callers can detect equality
// and fall through to the row-identity tiebreaker chain.
func compareColumnValuesCmp(a, b, colName string) int {
	// Known CPU/MEM columns: use resource value parser directly.
	if colName == "CPU" || colName == "MEM" || colName == "CPU/R" || colName == "CPU/L" || colName == "MEM/R" || colName == "MEM/L" {
		return compareResourceValuesCmp(a, b, colName)
	}

	// Try parsing as resource quantities (Gi, Mi, Ki, B suffixes or millicores).
	if looksLikeResourceQuantity(a) || looksLikeResourceQuantity(b) {
		va := ui.ParseResourceValue(a, false)
		vb := ui.ParseResourceValue(b, false)
		if va != 0 || vb != 0 {
			return cmpInt64(va, vb)
		}
	}

	// Try parsing as plain numbers.
	na, errA := strconv.ParseFloat(strings.TrimSpace(a), 64)
	nb, errB := strconv.ParseFloat(strings.TrimSpace(b), 64)
	if errA == nil && errB == nil {
		return cmpFloat(na, nb)
	}

	// Fall back to lexicographic comparison.
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// looksLikeResourceQuantity returns true if the value has a Kubernetes resource
// quantity suffix (Gi, Mi, Ki, B, m for millicores).
func looksLikeResourceQuantity(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasSuffix(s, "Gi") ||
		strings.HasSuffix(s, "Mi") ||
		strings.HasSuffix(s, "Ki") ||
		strings.HasSuffix(s, "Ti") ||
		(strings.HasSuffix(s, "m") && len(s) > 1 && s[len(s)-2] >= '0' && s[len(s)-2] <= '9')
}

func getColumnValue(item model.Item, key string) string {
	for _, kv := range item.Columns {
		if kv.Key == key {
			return kv.Value
		}
	}
	return ""
}

// statusPriority returns a sort priority for a status string.
func statusPriority(status string) int {
	switch status {
	case "Running", "Active", "Bound", "Available", "Ready", "Healthy", "Healthy/Synced", "Deployed":
		return 0
	case "Pending", "ContainerCreating", "Waiting", "Init", "Progressing", "Progressing/Synced", "Suspended",
		"Pending-install", "Pending-upgrade", "Pending-rollback", "Uninstalling":
		return 1
	case "Failed", "CrashLoopBackOff", "Error", "ImagePullBackOff", "Degraded", "Degraded/OutOfSync":
		return 2
	default:
		return 3
	}
}
