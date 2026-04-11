package app

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// newEvent returns a minimal Event-kind model.Item with the columns that
// groupEvents uses for its group key. The single timestamp is used for both
// first-seen and last-seen unless callers override LastSeen explicitly.
func newEvent(typ, reason, message, object, count string, t time.Time) model.Item {
	return model.Item{
		Name:   "evt-" + reason + "-" + object,
		Kind:   "Event",
		Status: typ,
		Age:    "5m",
		Columns: []model.KeyValue{
			{Key: "Object", Value: object},
			{Key: "Reason", Value: reason},
			{Key: "Message", Value: message},
			{Key: "Count", Value: count},
			{Key: "Source", Value: "kubelet"},
			{Key: "Last Seen", Value: "5m"},
		},
		CreatedAt: t,
		LastSeen:  t,
	}
}

// newEventWithRange constructs a Warning-kind Event-row for the same Pod
// with explicit first-seen (CreatedAt) and last-seen (LastSeen) timestamps,
// used to test the incident-window merge logic. The fixed object is
// intentional — these tests exercise duration semantics, not grouping keys.
func newEventWithRange(reason, message, count string, firstSeen, lastSeen time.Time) model.Item {
	it := newEvent("Warning", reason, message, "Pod/foo", count, firstSeen)
	it.LastSeen = lastSeen
	return it
}

func TestGroupEvents_EmptyInput(t *testing.T) {
	got := groupEvents(nil)
	assert.Empty(t, got)
	got = groupEvents([]model.Item{})
	assert.Empty(t, got)
}

func TestGroupEvents_SingleItemUnchanged(t *testing.T) {
	in := []model.Item{
		newEvent("Warning", "FailedScheduling", "0/3 nodes available", "Pod/foo", "1", time.Now()),
	}
	got := groupEvents(in)
	assert.Len(t, got, 1)
	assert.Equal(t, in[0].Name, got[0].Name)
	assert.Equal(t, "1", readCountColumn(got[0]))
}

func TestGroupEvents_MergesSameKey(t *testing.T) {
	now := time.Now()
	older := now.Add(-1 * time.Hour)
	in := []model.Item{
		// Caller passes newest-first. After grouping the merged row should
		// span the full incident window: CreatedAt = oldest observation
		// (1h ago), LastSeen = newest observation (now), Count = sum.
		newEventWithRange("FailedScheduling", "0/3 nodes available", "2", now, now),
		newEventWithRange("FailedScheduling", "0/3 nodes available", "3", older, older),
	}
	got := groupEvents(in)
	assert.Len(t, got, 1, "items with same (Type, Reason, Message, Object) should merge")
	assert.Equal(t, "5", readCountColumn(got[0]), "counts should be summed")
	assert.Equal(t, older, got[0].CreatedAt, "merged CreatedAt should track the earliest observation")
	assert.Equal(t, now, got[0].LastSeen, "merged LastSeen should track the latest observation")
}

func TestGroupEvents_DoesNotMergeDifferentObjects(t *testing.T) {
	now := time.Now()
	in := []model.Item{
		newEvent("Warning", "FailedScheduling", "same msg", "Pod/foo", "1", now),
		newEvent("Warning", "FailedScheduling", "same msg", "Pod/bar", "1", now),
	}
	got := groupEvents(in)
	assert.Len(t, got, 2)
}

func TestGroupEvents_DoesNotMergeDifferentMessages(t *testing.T) {
	now := time.Now()
	in := []model.Item{
		newEvent("Warning", "FailedScheduling", "msg A", "Pod/foo", "1", now),
		newEvent("Warning", "FailedScheduling", "msg B", "Pod/foo", "1", now),
	}
	got := groupEvents(in)
	assert.Len(t, got, 2)
}

func TestGroupEvents_DoesNotMergeDifferentReasons(t *testing.T) {
	now := time.Now()
	in := []model.Item{
		newEvent("Warning", "FailedScheduling", "msg", "Pod/foo", "1", now),
		newEvent("Warning", "BackOff", "msg", "Pod/foo", "1", now),
	}
	got := groupEvents(in)
	assert.Len(t, got, 2)
}

func TestGroupEvents_DoesNotMergeDifferentTypes(t *testing.T) {
	now := time.Now()
	in := []model.Item{
		newEvent("Warning", "Unhealthy", "msg", "Pod/foo", "1", now),
		newEvent("Normal", "Unhealthy", "msg", "Pod/foo", "1", now),
	}
	got := groupEvents(in)
	assert.Len(t, got, 2)
}

func TestGroupEvents_NonEventPassthrough(t *testing.T) {
	now := time.Now()
	podItem := model.Item{Name: "pod-a", Kind: "Pod", Status: "Running"}
	in := []model.Item{
		podItem,
		newEvent("Warning", "FailedScheduling", "msg", "Pod/foo", "1", now),
		newEvent("Warning", "FailedScheduling", "msg", "Pod/foo", "1", now),
		podItem,
	}
	got := groupEvents(in)
	// 2 pod items preserved + 1 merged event.
	assert.Len(t, got, 3)
	assert.Equal(t, "Pod", got[0].Kind)
	assert.Equal(t, "Event", got[1].Kind)
	assert.Equal(t, "2", readCountColumn(got[1]))
	assert.Equal(t, "Pod", got[2].Kind)
}

func TestGroupEvents_DoesNotMutateInput(t *testing.T) {
	now := time.Now()
	in := []model.Item{
		newEvent("Warning", "FailedScheduling", "msg", "Pod/foo", "2", now),
		newEvent("Warning", "FailedScheduling", "msg", "Pod/foo", "3", now.Add(-time.Minute)),
	}
	// Deep snapshot of the input so we can compare after the call.
	original := make([]model.Item, len(in))
	for i := range in {
		original[i] = in[i]
		original[i].Columns = append([]model.KeyValue(nil), in[i].Columns...)
	}

	_ = groupEvents(in)

	for i := range in {
		if !reflect.DeepEqual(in[i], original[i]) {
			t.Fatalf("groupEvents mutated input at index %d: got %+v want %+v", i, in[i], original[i])
		}
	}
}

func TestGroupEvents_TracksFirstAndLastSeen(t *testing.T) {
	now := time.Now()
	hourAgo := now.Add(-1 * time.Hour)
	dayAgo := now.Add(-24 * time.Hour)

	// Three observations of the same event spread over 24 hours. Input is
	// caller-sorted newest-first (matching the resource list query).
	in := []model.Item{
		newEventWithRange("FailedScheduling", "msg", "1",
			now, now), // most recent: just now
		newEventWithRange("FailedScheduling", "msg", "1",
			hourAgo, hourAgo), // 1 hour ago
		newEventWithRange("FailedScheduling", "msg", "1",
			dayAgo, dayAgo), // 1 day ago
	}
	got := groupEvents(in)
	require.Len(t, got, 1)

	// CreatedAt should be the OLDEST timestamp (start of the incident).
	assert.Equal(t, dayAgo, got[0].CreatedAt,
		"merged CreatedAt should be the earliest occurrence (incident start)")
	// LastSeen should be the NEWEST timestamp (most recent occurrence).
	assert.Equal(t, now, got[0].LastSeen,
		"merged LastSeen should be the latest occurrence (most recent fire)")
}

func TestGroupEvents_LastSeenColumnReflectsNewestObservation(t *testing.T) {
	now := time.Now()
	older := now.Add(-30 * time.Minute)

	in := []model.Item{
		newEventWithRange("BackOff", "msg", "1", now, now),
		newEventWithRange("BackOff", "msg", "1", older, older),
	}
	got := groupEvents(in)
	require.Len(t, got, 1)

	// "Last Seen" column reflects time since the most recent occurrence
	// (~0s, formatted as "Ns").
	lastSeen := readEventColumn(got[0], "Last Seen")
	assert.NotEmpty(t, lastSeen)
	assert.Regexp(t, `^\d+s$`, lastSeen, "Last Seen should be seconds-old for a just-now event")
}

func TestGroupEvents_AgeColumnReflectsFirstObservation(t *testing.T) {
	now := time.Now()
	hourAgo := now.Add(-1 * time.Hour)

	in := []model.Item{
		newEventWithRange("BackOff", "msg-age", "1", now, now),
		newEventWithRange("BackOff", "msg-age", "1", hourAgo, hourAgo),
	}
	got := groupEvents(in)
	require.Len(t, got, 1)

	// Age reflects ~1 hour, the time since the first occurrence (the start
	// of the incident).
	assert.Regexp(t, `^(59m|1h)$`, got[0].Age,
		"merged Age should reflect time since first observation, not last")
}

func TestGroupEvents_HandlesMissingCount(t *testing.T) {
	now := time.Now()
	// Item without a Count column (defensive fallback: treat as 1).
	noCount := model.Item{
		Kind:   "Event",
		Status: "Warning",
		Columns: []model.KeyValue{
			{Key: "Object", Value: "Pod/foo"},
			{Key: "Reason", Value: "BackOff"},
			{Key: "Message", Value: "restarting"},
		},
		CreatedAt: now,
	}
	in := []model.Item{noCount, noCount}
	got := groupEvents(in)
	assert.Len(t, got, 1)
	assert.Equal(t, "2", readCountColumn(got[0]), "missing Count columns should count as 1 each")
}

// TestGroupEvents_MatchesPopulateEventKeys pins the grouping column-key
// constants to the canonical k8s package exports. If populateEvent renames
// any of these strings without updating the constants, grouping would
// silently fall back to empty-string keys and merge unrelated rows — this
// test fails loudly instead.
func TestGroupEvents_MatchesPopulateEventKeys(t *testing.T) {
	assert.Equal(t, k8s.EventColObject, eventColObject)
	assert.Equal(t, k8s.EventColReason, eventColReason)
	assert.Equal(t, k8s.EventColMessage, eventColMessage)
	assert.Equal(t, k8s.EventColCount, eventColCount)
}

// TestRebuildEventsFromCache_PreservesFilterPreset verifies that toggling
// grouping or the warnings filter while a filter preset is active keeps the
// preset applied. Regression guard for the issue where the toggle paths
// rebuilt from cache and accidentally dropped the preset.
func TestRebuildEventsFromCache_PreservesFilterPreset(t *testing.T) {
	now := time.Now()
	raw := []model.Item{
		newEvent("Warning", "FailedScheduling", "msg", "Pod/foo", "1", now),
		newEvent("Warning", "BackOff", "msg", "Pod/bar", "1", now),
		newEvent("Normal", "Pulled", "msg", "Pod/baz", "1", now),
	}
	navKey := "test-nav-key"
	onlyWarnings := &FilterPreset{
		Name:    "warnings",
		MatchFn: func(it model.Item) bool { return it.Status == "Warning" },
	}
	m := &Model{
		middleItems:        nil,
		itemCache:          map[string][]model.Item{navKey: raw},
		activeFilterPreset: onlyWarnings,
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Event"},
		},
		warningEventsOnly: false,
		eventGrouping:     false,
		tabs:              []TabState{{nav: model.NavigationState{Level: model.LevelResources, ResourceType: model.ResourceTypeEntry{Kind: "Event"}}}},
	}
	// Override navKey by planting the cache under whatever m.navKey() returns.
	// The real navKey() derives a string from m.nav; plant the cache there too.
	m.itemCache[m.navKey()] = raw

	m.rebuildEventsFromCache()

	// Expect: 2 warnings passing preset, no grouping.
	assert.Len(t, m.middleItems, 2, "preset should still filter after rebuild")
	for _, it := range m.middleItems {
		assert.Equal(t, "Warning", it.Status)
	}

	// Now turn grouping back on and ensure the preset still applies on top.
	m.eventGrouping = true
	m.rebuildEventsFromCache()
	assert.Len(t, m.middleItems, 2, "grouping+preset should still yield 2 (different objects)")
	for _, it := range m.middleItems {
		assert.Equal(t, "Warning", it.Status)
	}
}

// TestRebuildEventsFromCache_Miss documents that toggling an Events view
// option (warnings/grouping) with no cached raw list leaves middleItems
// untouched — the next resource load will rebuild the view.
func TestRebuildEventsFromCache_Miss(t *testing.T) {
	existing := []model.Item{
		newEvent("Warning", "Foo", "msg", "Pod/x", "1", time.Now()),
	}
	m := &Model{
		middleItems: existing,
		itemCache:   map[string][]model.Item{}, // empty: cache miss
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Event"},
		},
		eventGrouping: false,
	}
	m.rebuildEventsFromCache()
	assert.Equal(t, existing, m.middleItems,
		"cache miss should be a no-op so the user doesn't see an empty list")
}

// readCountColumn returns the value of the Count column on an item, or empty
// string if absent.
func readCountColumn(it model.Item) string {
	return readEventColumn(it, "Count")
}

// readEventColumn returns the value of the named column or empty string.
func readEventColumn(it model.Item, key string) string {
	for _, col := range it.Columns {
		if col.Key == key {
			return col.Value
		}
	}
	return ""
}
