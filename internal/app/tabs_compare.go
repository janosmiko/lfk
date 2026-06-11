package app

import (
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

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
	m.middleItemsRev++
	sortItemsByColumn(m.middleItems, colName, m.sortAscending, m.nav.ResourceType.Kind)
}

// sortItemsByColumn sorts items in place by colName/asc with the shared
// stable tiebreaker chain. Used for the middle column (sortMiddleItems) and
// for the right-pane list preview so both render in the same order (issue
// #408). kind supplies the Event LastSeen default override.
func sortItemsByColumn(items []model.Item, colName string, asc bool, kind string) {
	if colName == "" {
		return
	}

	// Events default to LastSeen ordering (most recent first) when the
	// user hasn't explicitly chosen a different column. The override
	// uses a sentinel that comparePrimaryColumn recognizes, without
	// injecting "Last Seen" into the sortable-column cycle.
	if colName == sortColDefault && kind == "Event" {
		colName = sortColEventLastSeen
	}

	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]

		// Metrics-less rows ("n/a") always sort last, in BOTH directions.
		// This cannot live inside comparePrimaryColumn: the asc/desc sign
		// flip below would invert it and float missing data to the top when
		// sorting descending. Handle it here (like the direction-independent
		// tiebreaker) so flipping the sort never reorders the n/a block.
		if aMiss, bMiss := metricValueMissing(colName, a), metricValueMissing(colName, b); aMiss != bMiss {
			return bMiss // a sorts before b iff b is the missing one
		}

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
		// identity tuple (Name, Context, Namespace, Age) forms the main fallback
		// in that order, with whichever of those four is already the
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
// The chain is primary-aware: (Name, Context, Namespace, Age) participates in
// that order, with whichever of those four is the current primary
// column excluded. Kind and Extra act as final fallbacks so rows with
// truly identical identity still have a stable order.
//
//	primary=Name      → (Context, Namespace, Age, Kind, Extra)
//	primary=Context   → (Name, Namespace, Age, Kind, Extra)
//	primary=Namespace → (Name, Context, Age, Kind, Extra)
//	primary=Age       → (Name, Context, Namespace, Kind, Extra)
//	primary=anything  → (Name, Context, Namespace, Age, Kind, Extra)
func itemTiebreakerLess(a, b model.Item, primaryCol string) bool {
	if primaryCol != "Name" {
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c < 0
		}
	}
	if primaryCol != "Context" {
		if c := strings.Compare(strings.ToLower(a.ClusterName), strings.ToLower(b.ClusterName)); c != 0 {
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

// valueCmpFunc compares two extra-column string values. Used by the
// columnValueCmp registry below to type extra columns whose semantics
// can't be inferred from their value alone (e.g. "45%" could be a
// percent or a label, "80/TCP" could be a port or a path).
type valueCmpFunc func(a, b string) int

// columnValueCmp registers per-column comparators for extra-column
// values (those stored in Item.Columns rather than top-level Item fields).
// Adding a new typed column is a one-line registration here — keeping
// the format dispatch in one place avoids the bug class where new
// percent/resource columns silently fell through to lexicographic sort
// (e.g. "100%" < "9%").
var columnValueCmp = map[string]valueCmpFunc{
	"CPU":          func(a, b string) int { return compareResourceValuesCmp(a, b, "CPU") },
	"MEM":          func(a, b string) int { return compareResourceValuesCmp(a, b, "MEM") },
	"CPU%":         comparePercentCmp,
	"MEM%":         comparePercentCmp,
	"CPU/R":        comparePercentCmp,
	"CPU/L":        comparePercentCmp,
	"MEM/R":        comparePercentCmp,
	"MEM/L":        comparePercentCmp,
	"Ports":        comparePortsCmp,
	"Progress":     compareReadyCmp, // "N/M" fraction, same shape as Ready ratio
	"Duration":     compareDurationCmp,
	"REV":          compareREVCmp,
	"Cluster IP":   compareIPCmp,
	"Pod IP":       compareIPCmp,
	"External IPs": compareIPCmp,
}

// metricsMissingLastColumns are the metrics columns whose cells render as
// "n/a" when metrics-server has no data for the row. Rows missing such a value
// must sort to the bottom regardless of sort direction (see sortMiddleItems).
var metricsMissingLastColumns = map[string]bool{
	"CPU": true, "MEM": true,
	"CPU%": true, "MEM%": true,
	"CPU/R": true, "CPU/L": true, "MEM/R": true, "MEM/L": true,
}

// metricValueMissing reports whether item's value for colName is a metrics-less
// "n/a" placeholder that should always sort last. Returns false for non-metrics
// columns, so only the CPU/MEM family gets the always-last treatment.
func metricValueMissing(colName string, item model.Item) bool {
	if !metricsMissingLastColumns[colName] {
		return false
	}
	v := strings.TrimSpace(getColumnValue(item, colName))
	return v == "" || v == "n/a"
}

// comparePrimaryColumn returns -1, 0, or +1 for a < b, a == b, a > b
// according to the selected sort column. Returning 0 lets the caller run
// a tiebreaker chain instead of relying on sort.SliceStable's input-order
// preservation.
//
// Columns whose comparator needs the whole Item (top-level fields,
// timestamps, status priority) are handled inline; extra columns go
// through columnValueCmp, with auto-detection as the final fallback.
func comparePrimaryColumn(a, b model.Item, colName string) int {
	switch colName {
	case "Name":
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	case "Context":
		return strings.Compare(strings.ToLower(a.ClusterName), strings.ToLower(b.ClusterName))
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
	case "Ports":
		return comparePortsCmp(getColumnValue(a, "Ports"), getColumnValue(b, "Ports"))
	case "Progress":
		// Argo Workflow progress is an "N/M" fraction, identical in shape
		// to the Ready ratio — reuse the same comparator.
		return compareReadyCmp(getColumnValue(a, "Progress"), getColumnValue(b, "Progress"))
	case "Duration":
		return compareDurationCmp(getColumnValue(a, "Duration"), getColumnValue(b, "Duration"))
	case "Cluster IP", "Pod IP", "External IPs":
		return compareIPCmp(getColumnValue(a, colName), getColumnValue(b, colName))
	case "Severity":
		return cmpInt(severityRank(getColumnValue(a, "Severity")), severityRank(getColumnValue(b, "Severity")))
	case sortColEventLastSeen:
		return compareLastSeenCmp(a, b)
	}
	va, vb := getColumnValue(a, colName), getColumnValue(b, colName)
	if cmp, ok := columnValueCmp[colName]; ok {
		return cmp(va, vb)
	}
	return compareColumnValuesCmp(va, vb)
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

func cmpUint64(a, b uint64) int {
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

// compareResourceValuesCmp compares two CPU/MEM column values numerically.
// Values may carry a trend arrow ("↑ 1.3", "↓ 710m") — stripped during parse —
// or be "n/a" when metrics-server has no data for the row. "n/a" (and any other
// unparseable value) sorts after real values so metrics-less rows land at the
// bottom in ascending order, mirroring comparePercentCmp.
func compareResourceValuesCmp(a, b, col string) int {
	isCPU := strings.HasPrefix(col, "CPU")
	va, okA := ui.ParseResourceValueOK(a, isCPU)
	vb, okB := ui.ParseResourceValueOK(b, isCPU)
	switch {
	case okA && okB:
		return cmpInt64(va, vb)
	case okA:
		return -1
	case okB:
		return 1
	default:
		return strings.Compare(strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b)))
	}
}

// comparePercentCmp compares two percentage-formatted column values
// (e.g. "42%", "n/a") numerically by their leading integer percentage.
// "n/a" (and any other unparseable value) sorts after real values so
// metrics-less rows land at the bottom in ascending order.
func comparePercentCmp(a, b string) int {
	pa, okA := parsePercent(a)
	pb, okB := parsePercent(b)
	switch {
	case okA && okB:
		return cmpFloat(pa, pb)
	case okA:
		return -1
	case okB:
		return 1
	default:
		return strings.Compare(strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b)))
	}
}

func parsePercent(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return n, true
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

// comparePortsCmp compares Service "Ports" column values numerically by
// their leading port number, so "99/TCP" sorts before "10000/TCP" instead
// of lexicographically. Values are comma-joined lists (e.g. "80/TCP, 443/TCP");
// only the first entry's port drives the comparison. Falls back to a
// case-insensitive lexicographic comparison when either value lacks a
// leading number.
func comparePortsCmp(a, b string) int {
	na, okA := leadingPortNumber(a)
	nb, okB := leadingPortNumber(b)
	if okA && okB {
		if c := cmpInt(na, nb); c != 0 {
			return c
		}
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// leadingPortNumber extracts the leading run of digits from a port string
// (e.g. "8080:30000/TCP" → 8080). Returns false when there is no leading digit.
func leadingPortNumber(s string) (int, bool) {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(s[:i])
	if err != nil {
		return 0, false
	}
	return n, true
}

// compareDurationCmp compares two Go duration strings (e.g. "5m30s") by
// their actual length, so "10m" sorts after "5m" instead of before it.
// Falls back to case-insensitive lexicographic comparison when either
// value is not a parseable duration.
func compareDurationCmp(a, b string) int {
	da, errA := time.ParseDuration(strings.TrimSpace(a))
	db, errB := time.ParseDuration(strings.TrimSpace(b))
	if errA == nil && errB == nil {
		return cmpInt64(int64(da), int64(db))
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// compareREVCmp compares REV column values numerically (decimal). Falls back
// to case-insensitive lexicographic comparison when either value is not
// parseable as a uint64.
func compareREVCmp(a, b string) int {
	na, errA := strconv.ParseUint(strings.TrimSpace(a), 10, 64)
	nb, errB := strconv.ParseUint(strings.TrimSpace(b), 10, 64)
	if errA == nil && errB == nil {
		return cmpUint64(na, nb)
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// compareIPCmp compares IP-address column values numerically, so
// "10.0.0.9" sorts before "10.0.0.10". Values may be comma-joined lists
// (e.g. External IPs); only the first address drives the comparison.
// Falls back to case-insensitive lexicographic comparison when either
// value lacks a parseable leading address (e.g. "None", "<none>").
func compareIPCmp(a, b string) int {
	ia, okA := leadingIP(a)
	ib, okB := leadingIP(b)
	if okA && okB {
		if c := ia.Compare(ib); c != 0 {
			return c
		}
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// leadingIP parses the first comma-separated token of s as an IP address.
func leadingIP(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, ','); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr, true
}

// compareColumnValuesCmp compares two column values with automatic detection
// of resource quantities (10Gi, 500Mi, 100m), plain numbers, and strings.
// Returns -1, 0, or +1 so sort.SliceStable callers can detect equality
// and fall through to the row-identity tiebreaker chain.
func compareColumnValuesCmp(a, b string) int {
	// Auto-detect for unknown columns: typed columns are dispatched via
	// columnValueCmp before reaching here.

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

// severityRank maps a Severity column value to a sort priority. Lower
// rank sorts first in ascending order, so CRIT lands at the top and
// unknown severity ("?") at the bottom. Mirrors severityLabel in the
// k8s package but lives here so app's sort stays self-contained.
func severityRank(sev string) int {
	switch sev {
	case "CRIT":
		return 0
	case "HIGH":
		return 1
	case "MED":
		return 2
	case "LOW":
		return 3
	}
	return 4
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
