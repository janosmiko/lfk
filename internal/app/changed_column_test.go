package app

import (
	"testing"
	"time"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func podWithConditions(name string, transitions ...time.Time) model.Item {
	it := model.Item{Name: name, Namespace: "prod", Kind: "Pod"}
	for i, at := range transitions {
		it.Conditions = append(it.Conditions, model.ConditionEntry{
			Type:               []string{"Ready", "Initialized", "PodScheduled"}[i%3],
			Status:             "True",
			LastTransitionTime: at,
		})
	}
	return it
}

func rawWithManagedFields(stamps ...string) map[string]any {
	entries := make([]any, 0, len(stamps))
	for _, s := range stamps {
		entries = append(entries, map[string]any{"manager": "kubelet", "time": s})
	}
	return map[string]any{"metadata": map[string]any{"managedFields": entries}}
}

func TestChangedAgeUsesTheNewestCondition(t *testing.T) {
	now := time.Now()
	it := podWithConditions("web",
		now.Add(-time.Hour),
		now.Add(-5*time.Minute),
		now.Add(-2*time.Hour),
	)

	age, ok := changeAge(it, now)
	if !ok {
		t.Fatal("a pod with conditions must report a change time")
	}
	if age != 5*time.Minute {
		t.Fatalf("want the newest transition, got %v", age)
	}
}

func TestChangedAgePrefersTheRestartOverAnOlderCondition(t *testing.T) {
	now := time.Now()
	it := podWithConditions("web", now.Add(-time.Hour))
	it.LastRestartAt = now.Add(-30 * time.Second)

	age, _ := changeAge(it, now)
	if age != 30*time.Second {
		t.Fatalf("a newer restart must win over an older condition, got %v", age)
	}
}

func TestChangedAgeKeepsTheConditionWhenTheRestartIsOlder(t *testing.T) {
	now := time.Now()
	it := podWithConditions("web", now.Add(-30*time.Second))
	it.LastRestartAt = now.Add(-time.Hour)

	age, _ := changeAge(it, now)
	if age != 30*time.Second {
		t.Fatalf("the newest of the two must win, got %v", age)
	}
}

func TestChangedAgeReportsWithoutAnyPollHistory(t *testing.T) {
	now := time.Now()
	// The whole point of the rewrite: one observation is enough.
	it := podWithConditions("web", now.Add(-3*24*time.Hour))

	if _, ok := changeAge(it, now); !ok {
		t.Fatal("a single observation must be enough to report a change time")
	}
}

func TestChangedAgeFallsBackToManagedFields(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	// A ConfigMap: no conditions, no restarts.
	it := model.Item{
		Name: "settings", Namespace: "prod", Kind: "ConfigMap",
		Raw: rawWithManagedFields(
			now.Add(-time.Hour).Format(time.RFC3339),
			now.Add(-90*time.Second).Format(time.RFC3339),
		),
	}

	age, ok := changeAge(it, now)
	if !ok {
		t.Fatal("a kind with no conditions must fall back to managedFields")
	}
	if age != 90*time.Second {
		t.Fatalf("want the newest write, got %v", age)
	}
}

func TestChangedAgeIgnoresManagedFieldsWhenAConditionExists(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	it := podWithConditions("web", now.Add(-time.Hour))
	it.Raw = rawWithManagedFields(now.Format(time.RFC3339))

	age, _ := changeAge(it, now)
	if age != time.Hour {
		t.Fatalf("a real state change must outrank a bare write, got %v", age)
	}
}

func TestChangedAgeReportsNothingWithoutAnySource(t *testing.T) {
	it := model.Item{Name: "synthetic", Kind: "PortForward"}

	if _, ok := changeAge(it, time.Now()); ok {
		t.Fatal("an item with no timestamps must report no change time")
	}
}

func TestChangedAgeSurvivesMalformedRaw(t *testing.T) {
	now := time.Now()
	cases := []map[string]any{
		nil,
		{"metadata": "not-a-map"},
		{"metadata": map[string]any{"managedFields": "not-a-list"}},
		{"metadata": map[string]any{"managedFields": []any{"not-a-map", map[string]any{}}}},
		{"metadata": map[string]any{"managedFields": []any{map[string]any{"time": "nonsense"}}}},
	}
	for i, raw := range cases {
		it := model.Item{Name: "x", Kind: "ConfigMap", Raw: raw}
		if _, ok := changeAge(it, now); ok {
			t.Fatalf("case %d: malformed raw must report no change time", i)
		}
	}
}

func TestChangedAgeNeverGoesNegative(t *testing.T) {
	now := time.Now()
	// Clock skew between lfk and the apiserver can stamp a future time.
	it := podWithConditions("web", now.Add(time.Minute))

	age, ok := changeAge(it, now)
	if !ok || age != 0 {
		t.Fatalf("a future timestamp must clamp to 0, got %v ok=%v", age, ok)
	}
}

func TestChangedColumnStampsEveryRowOnTheFirstRender(t *testing.T) {
	now := time.Now()
	items := []model.Item{
		podWithConditions("a", now.Add(-45*time.Second)),
		podWithConditions("b", now.Add(-2*time.Hour)),
	}

	applyChangedColumn(items, now)

	if got := items[0].ColumnValue(ChangedColumnKey); got != "45s" {
		t.Fatalf("row a: want 45s, got %q", got)
	}
	if got := items[1].ColumnValue(ChangedColumnKey); got != "2h" {
		t.Fatalf("row b: want 2h, got %q", got)
	}
}

func TestChangedColumnIsEmptyWithoutASource(t *testing.T) {
	items := []model.Item{{Name: "synthetic", Kind: "PortForward"}}
	applyChangedColumn(items, time.Now())

	if got := items[0].ColumnValue(ChangedColumnKey); got != "" {
		t.Fatalf("a row with no source must render empty, got %q", got)
	}
}

func TestChangedColumnReplacesItsOwnCell(t *testing.T) {
	now := time.Now()
	items := []model.Item{podWithConditions("a", now)}

	applyChangedColumn(items, now)
	applyChangedColumn(items, now)

	count := 0
	for _, kv := range items[0].Columns {
		if kv.Key == ChangedColumnKey {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("restamping must not duplicate the cell, have %d", count)
	}
}

func TestChangedColumnComesFirstAndKeepsPeers(t *testing.T) {
	it := podWithConditions("web", time.Now())
	it.Columns = []model.KeyValue{{Key: "Node", Value: "n1"}}
	items := []model.Item{it}

	applyChangedColumn(items, time.Now())

	if items[0].Columns[0].Key != ChangedColumnKey {
		t.Fatalf("the cell must be prepended, got %q first", items[0].Columns[0].Key)
	}
	if items[0].ColumnValue("Node") != "n1" {
		t.Fatal("existing columns must survive the stamp")
	}
}

func TestChangedSortPutsTheMostRecentChangeFirst(t *testing.T) {
	now := time.Now()
	items := []model.Item{
		podWithConditions("aaa-stale", now.Add(-3*time.Hour)),
		podWithConditions("zzz-fresh", now.Add(-10*time.Second)),
	}

	applyChangedColumn(items, now)
	sortItemsByColumn(items, ChangedColumnKey, true, "Pod")

	if items[0].Name != "zzz-fresh" {
		t.Fatalf("the most recent change must sort first, got %q", items[0].Name)
	}
}

func TestChangedSortKeepsSourcelessRowsLastInBothDirections(t *testing.T) {
	now := time.Now()
	for _, asc := range []bool{true, false} {
		items := []model.Item{
			{Name: "aaa-none", Kind: "PortForward"},
			podWithConditions("zzz-real", now.Add(-time.Minute)),
		}
		applyChangedColumn(items, now)
		sortItemsByColumn(items, ChangedColumnKey, asc, "Pod")

		if items[len(items)-1].Name != "aaa-none" {
			t.Fatalf("asc=%v: an empty cell must sort last, got %q", asc, items[len(items)-1].Name)
		}
	}
}

func TestChangedSortFallsBackToNameForEqualTimes(t *testing.T) {
	now := time.Now()
	at := now.Add(-time.Minute)
	items := []model.Item{
		podWithConditions("charlie", at),
		podWithConditions("alpha", at),
		podWithConditions("bravo", at),
	}

	applyChangedColumn(items, now)
	sortItemsByColumn(items, ChangedColumnKey, true, "Pod")

	for i, want := range []string{"alpha", "bravo", "charlie"} {
		if items[i].Name != want {
			t.Fatalf("row %d: want %q, got %q", i, want, items[i].Name)
		}
	}
}

func TestChangedSortSeparatesUnionClusters(t *testing.T) {
	now := time.Now()
	east := podWithConditions("web", now.Add(-2*time.Hour))
	east.ClusterName = "east"
	west := podWithConditions("web", now.Add(-5*time.Second))
	west.ClusterName = "west"

	items := []model.Item{east, west}
	applyChangedColumn(items, now)
	sortItemsByColumn(items, ChangedColumnKey, true, "Pod")

	if items[0].ClusterName != "west" {
		t.Fatalf("the row that changed most recently must rise, got %q", items[0].ClusterName)
	}
}

func TestChangedJoinsTheSortCycle(t *testing.T) {
	prev := ui.ActiveSortableColumns
	prevCount := ui.ActiveSortableColumnCount
	t.Cleanup(func() {
		ui.ActiveSortableColumns = prev
		ui.ActiveSortableColumnCount = prevCount
	})

	ui.ActiveSortableColumns = []string{"Name", "Age", ui.ChangedColumnName}
	ui.ActiveSortableColumnCount = len(ui.ActiveSortableColumns)

	idx, ok := sortColumnIndex(ChangedColumnKey)
	if !ok || idx != 2 {
		t.Fatalf("the Changed key must be reachable in the cycle, got idx=%d ok=%v", idx, ok)
	}
}
