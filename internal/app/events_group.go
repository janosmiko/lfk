package app

import (
	"strconv"
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// Column keys used by the grouping logic. We alias the canonical exports from
// the k8s package (which owns populateEvent) so renaming the column keys
// breaks grouping at compile time instead of silently mis-aggregating.
const (
	eventColObject   = k8s.EventColObject
	eventColReason   = k8s.EventColReason
	eventColMessage  = k8s.EventColMessage
	eventColCount    = k8s.EventColCount
	eventColLastSeen = k8s.EventColLastSeen
)

// eventGroupKey uniquely identifies a group of events that should collapse
// into a single row. Two events sharing the same key are considered duplicate
// reports of the same incident (e.g. the same pod failing to schedule 20
// times produces 20 Event objects with identical Type/Reason/Message/Object).
type eventGroupKey struct {
	Type    string
	Reason  string
	Message string
	Object  string
}

// groupEvents collapses events sharing the same (Type, Reason, Message,
// Object) tuple into a single model.Item whose Count column holds the sum of
// the originals. The merged row exposes the full incident window:
//
//   - CreatedAt / Age column = the OLDEST observation in the group (when the
//     incident first started). Callers pass events sorted newest-first, so the
//     first occurrence of each key initially wins, then later occurrences pull
//     CreatedAt backwards as we encounter them.
//   - LastSeen / Last Seen column = the NEWEST observation in the group.
//
// Items whose Kind is not "Event" are returned unchanged and in their
// original relative order, so the function is safe to call even on mixed
// slices. The input slice and its items are never mutated.
func groupEvents(items []model.Item) []model.Item {
	if len(items) == 0 {
		return items
	}
	groups := make(map[eventGroupKey]int, len(items))
	result := make([]model.Item, 0, len(items))
	for i := range items {
		it := items[i]
		if it.Kind != "Event" {
			result = append(result, it)
			continue
		}
		key := extractEventGroupKey(it)
		if idx, ok := groups[key]; ok {
			mergeEventInto(&result[idx], it)
			continue
		}
		clone := cloneEventItem(it)
		// Seed GroupedRefs with the first item in the group so bulk
		// operations can expand the grouped row back into individual
		// delete calls covering every underlying Event object.
		clone.GroupedRefs = []model.GroupedRef{{Name: it.Name, Namespace: it.Namespace}}
		// Refresh the relative-time columns so the cloned row always
		// reflects its CreatedAt/LastSeen fields, even if the source item
		// arrived with stale strings.
		refreshEventTimeColumns(&clone)
		groups[key] = len(result)
		result = append(result, clone)
	}
	return result
}

// refreshEventTimeColumns recomputes the Age string and Last Seen column from
// the item's CreatedAt and LastSeen fields, so visible time labels always
// match the underlying timestamps after a clone or merge.
func refreshEventTimeColumns(it *model.Item) {
	if !it.CreatedAt.IsZero() {
		it.Age = k8s.FormatAge(time.Since(it.CreatedAt))
	}
	if !it.LastSeen.IsZero() {
		setEventColumn(it, eventColLastSeen, k8s.FormatAge(time.Since(it.LastSeen)))
	}
}

// mergeEventInto folds src into dst, summing counts and widening the
// first/last-seen window to cover both items. dst must already be a clone
// owned by the result slice (cloneEventItem ensures this).
func mergeEventInto(dst *model.Item, src model.Item) {
	addEventCount(dst, readEventCount(src))
	mergeFirstSeen(dst, src)
	mergeLastSeen(dst, src)
	dst.GroupedRefs = append(dst.GroupedRefs, model.GroupedRef{Name: src.Name, Namespace: src.Namespace})
}

// mergeFirstSeen pulls dst.CreatedAt backwards if src has an earlier
// observation, and refreshes the Age string accordingly.
func mergeFirstSeen(dst *model.Item, src model.Item) {
	if src.CreatedAt.IsZero() {
		return
	}
	if dst.CreatedAt.IsZero() || src.CreatedAt.Before(dst.CreatedAt) {
		dst.CreatedAt = src.CreatedAt
		dst.Age = k8s.FormatAge(time.Since(dst.CreatedAt))
	}
}

// mergeLastSeen pushes dst.LastSeen forward if src has a later observation,
// and refreshes the Last Seen column accordingly.
func mergeLastSeen(dst *model.Item, src model.Item) {
	if src.LastSeen.IsZero() {
		return
	}
	if dst.LastSeen.IsZero() || src.LastSeen.After(dst.LastSeen) {
		dst.LastSeen = src.LastSeen
		setEventColumn(dst, eventColLastSeen, k8s.FormatAge(time.Since(dst.LastSeen)))
	}
}

// setEventColumn updates the value of an existing column or appends a new one
// if absent. dst must already be a clone owned by the result slice.
func setEventColumn(dst *model.Item, key, value string) {
	for i, col := range dst.Columns {
		if col.Key == key {
			dst.Columns[i].Value = value
			return
		}
	}
	dst.Columns = append(dst.Columns, model.KeyValue{Key: key, Value: value})
}

// extractEventGroupKey reads the subset of columns that defines the group.
// Missing columns fall back to empty strings, which still produces a stable
// key for merging two equally-incomplete items.
func extractEventGroupKey(it model.Item) eventGroupKey {
	k := eventGroupKey{Type: it.Status}
	for _, col := range it.Columns {
		switch col.Key {
		case eventColReason:
			k.Reason = col.Value
		case eventColMessage:
			k.Message = col.Value
		case eventColObject:
			k.Object = col.Value
		}
	}
	return k
}

// cloneEventItem returns a shallow copy of it with freshly allocated slices
// so subsequent in-place updates (count summation) don't leak into the caller's
// data. Fields that are value types copy automatically via the struct literal.
func cloneEventItem(it model.Item) model.Item {
	clone := it
	if len(it.Columns) > 0 {
		clone.Columns = append([]model.KeyValue(nil), it.Columns...)
	}
	if len(it.Conditions) > 0 {
		clone.Conditions = append([]model.ConditionEntry(nil), it.Conditions...)
	}
	return clone
}

// addEventCount adds delta to the Count column of dst, creating the column
// if missing. dst must already be a clone owned by the result slice.
func addEventCount(dst *model.Item, delta int64) {
	total := readEventCount(*dst) + delta
	str := strconv.FormatInt(total, 10)
	for i, col := range dst.Columns {
		if col.Key == eventColCount {
			dst.Columns[i].Value = str
			return
		}
	}
	dst.Columns = append(dst.Columns, model.KeyValue{Key: eventColCount, Value: str})
}

// readEventCount returns the integer value of the Count column. The fallback
// rules are deliberate:
//
//   - Missing Count column or unparseable value -> 1 (each row represents at
//     least one observed occurrence, so treating it as 1 keeps grouped totals
//     honest).
//   - Explicit zero or negative values -> 1 (should never happen with data
//     from the Kubernetes API, which uses int32 >= 1 once an event fires;
//     clamping keeps pathological input from producing misleading subtractions).
func readEventCount(it model.Item) int64 {
	for _, col := range it.Columns {
		if col.Key != eventColCount {
			continue
		}
		n, err := strconv.ParseInt(col.Value, 10, 64)
		if err != nil || n <= 0 {
			return 1
		}
		return n
	}
	return 1
}
