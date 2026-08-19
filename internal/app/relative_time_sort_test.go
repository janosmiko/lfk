package app

import (
	"testing"
	"time"

	"github.com/janosmiko/lfk/internal/model"
)

func crdWithTransition(name, cell string) model.Item {
	return model.Item{
		Name:    name,
		Kind:    "CustomResourceDefinition",
		Columns: []model.KeyValue{{Key: "Last Transition", Value: cell}},
	}
}

func TestLastTransitionSortsByDurationNotLexically(t *testing.T) {
	items := []model.Item{
		crdWithTransition("aaa-ancient", "1138d ago"),
		crdWithTransition("zzz-recent", "14d ago"),
	}

	sortItemsByColumn(items, "Last Transition", true, "CustomResourceDefinition")

	if items[0].Name != "zzz-recent" {
		t.Fatalf("the most recent transition must sort first, got %q", items[0].Name)
	}
}

func TestLastTransitionOrdersEveryUnit(t *testing.T) {
	items := []model.Item{
		crdWithTransition("d", "3d ago"),
		crdWithTransition("s", "45s ago"),
		crdWithTransition("h", "2h ago"),
		crdWithTransition("m", "30m ago"),
	}

	sortItemsByColumn(items, "Last Transition", true, "CustomResourceDefinition")

	for i, want := range []string{"s", "m", "h", "d"} {
		if items[i].Name != want {
			t.Fatalf("row %d: want %q, got %q", i, want, items[i].Name)
		}
	}
}

func TestSyncedAtSortsThroughItsStatusPrefix(t *testing.T) {
	items := []model.Item{
		{Name: "aaa", Kind: "Application", Columns: []model.KeyValue{{Key: "Synced At", Value: "1138d ago"}}},
		{Name: "zzz", Kind: "Application", Columns: []model.KeyValue{{Key: "Synced At", Value: "syncing 14d ago"}}},
	}

	sortItemsByColumn(items, "Synced At", true, "Application")

	if items[0].Name != "zzz" {
		t.Fatalf("the prefix must not defeat the duration, got %q", items[0].Name)
	}
}

func itemWithColumn(name, key, value string) model.Item {
	return model.Item{Name: name, Kind: "Widget", Columns: []model.KeyValue{{Key: key, Value: value}}}
}

// The cells hold formatAge output, where "10d" is older than "9h" but sorts
// before it as text.
func TestAgeShapedColumnsSortByDuration(t *testing.T) {
	for _, col := range []string{"Last Scale Time", "Next", "Last Deployed", "Last Seen"} {
		items := []model.Item{
			itemWithColumn("aaa-older", col, "10d"),
			itemWithColumn("zzz-newer", col, "9h"),
		}
		sortItemsByColumn(items, col, true, "Widget")

		if items[0].Name != "zzz-newer" {
			t.Fatalf("%s: the shorter duration must sort first, got %q", col, items[0].Name)
		}
	}
}

func TestTimeShapedColumnsKeepUnreadableCellsLast(t *testing.T) {
	cases := []struct{ col, good string }{
		{"Last Transition", "14d ago"},
		{"Synced At", "14d ago"},
		{"Last Scale Time", "9h"},
		{"Next", "9h"},
		{"Last Deployed", "9h"},
		{"Last Seen", "9h"},
		{"Interval", "5m0s"},
	}
	// A CRD printer column may reuse any of these names and hold any text.
	for _, c := range cases {
		for _, bad := range []string{"", "n/a", "unknown"} {
			for _, asc := range []bool{true, false} {
				items := []model.Item{
					itemWithColumn("aaa-bad", c.col, bad),
					itemWithColumn("zzz-real", c.col, c.good),
				}
				sortItemsByColumn(items, c.col, asc, "Widget")

				if items[len(items)-1].Name != "aaa-bad" {
					t.Fatalf("%s cell %q asc=%v: an unreadable cell must sort last, got %q",
						c.col, bad, asc, items[len(items)-1].Name)
				}
			}
		}
	}
}

func TestLastSeenColumnPrefersTheEventTimestamp(t *testing.T) {
	now := time.Now()
	older := itemWithColumn("aaa-older", "Last Seen", "1m")
	older.LastSeen = now.Add(-61 * time.Second)
	newer := itemWithColumn("zzz-newer", "Last Seen", "1m")
	newer.LastSeen = now.Add(-60 * time.Second)

	items := []model.Item{older, newer}
	sortItemsByColumn(items, "Last Seen", true, "Event")

	if items[0].Name != "zzz-newer" {
		t.Fatalf("the real timestamp must break the tie, got %q", items[0].Name)
	}
}

func TestIntervalSortsByDuration(t *testing.T) {
	items := []model.Item{
		itemWithColumn("aaa-long", "Interval", "10m0s"),
		itemWithColumn("zzz-short", "Interval", "5m0s"),
	}

	sortItemsByColumn(items, "Interval", true, "GitRepository")

	if items[0].Name != "zzz-short" {
		t.Fatalf("the shorter interval must sort first, got %q", items[0].Name)
	}
}
