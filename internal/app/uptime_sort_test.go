package app

import (
	"testing"
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

func nodeWithUptime(name string, up time.Duration, now time.Time) model.Item {
	return model.Item{
		Name:     name,
		Kind:     "Node",
		BootedAt: now.Add(-up),
		Columns:  []model.KeyValue{{Key: "Uptime", Value: k8s.FormatAge(up)}},
	}
}

func TestUptimeSortUsesTheBootTimeNotTheRenderedCell(t *testing.T) {
	now := time.Now()
	// Both rows render "5d". Only the boot time separates them.
	items := []model.Item{
		nodeWithUptime("aaa-older", 5*24*time.Hour+3*time.Hour, now),
		nodeWithUptime("zzz-younger", 5*24*time.Hour, now),
	}
	if a, b := items[0].ColumnValue("Uptime"), items[1].ColumnValue("Uptime"); a != b {
		t.Fatalf("this test needs two equal cells, got %q and %q", a, b)
	}

	sortItemsByColumn(items, "Uptime", true, "Node")

	if items[0].Name != "zzz-younger" {
		t.Fatalf("the shorter uptime must sort first, got %q", items[0].Name)
	}
}

func TestUptimeSortFallsBackToTheCellWithoutABootTime(t *testing.T) {
	items := []model.Item{
		{Name: "aaa", Kind: "Widget", Columns: []model.KeyValue{{Key: "Uptime", Value: "10d"}}},
		{Name: "zzz", Kind: "Widget", Columns: []model.KeyValue{{Key: "Uptime", Value: "9h"}}},
	}

	sortItemsByColumn(items, "Uptime", true, "Widget")

	if items[0].Name != "zzz" {
		t.Fatalf("a CRD Uptime column must still sort by duration, got %q", items[0].Name)
	}
}

func TestUptimeEnrichmentStampsTheBootTime(t *testing.T) {
	m := baseModel()
	m.middleItems = []model.Item{{Name: "node-a"}}

	result, _ := m.Update(nodeUptimeEnrichedMsg{
		uptimes: k8s.NodeUptimes{ByName: map[string]time.Duration{"node-a": 3 * time.Hour}},
		gen:     0,
	})
	got := result.(Model).middleItems[0].BootedAt

	if got.IsZero() {
		t.Fatal("a matched node must carry a boot time")
	}
	if d := time.Since(got); d < 3*time.Hour-time.Minute || d > 3*time.Hour+time.Minute {
		t.Fatalf("want a boot time about 3h back, got %v", d)
	}
}

func TestUptimeCarryOverKeepsTheBootTime(t *testing.T) {
	now := time.Now()
	old := []model.Item{nodeWithUptime("node-a", 5*24*time.Hour, now)}
	fresh := []model.Item{{Name: "node-a", Kind: "Node"}}

	carryOverMetricsColumnsFrom(old, fresh)

	if !fresh[0].BootedAt.Equal(old[0].BootedAt) {
		t.Fatalf("a watch tick must keep the boot time, got %v", fresh[0].BootedAt)
	}
}
