package app

import (
	"testing"

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

func TestLastTransitionKeepsUnparseableCellsLast(t *testing.T) {
	for _, asc := range []bool{true, false} {
		items := []model.Item{
			crdWithTransition("aaa-empty", ""),
			crdWithTransition("zzz-real", "14d ago"),
		}
		sortItemsByColumn(items, "Last Transition", asc, "CustomResourceDefinition")

		if items[len(items)-1].Name != "aaa-empty" {
			t.Fatalf("asc=%v: an empty cell must sort last, got %q", asc, items[len(items)-1].Name)
		}
	}
}
