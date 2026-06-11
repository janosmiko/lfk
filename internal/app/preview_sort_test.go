package app

import (
	"testing"
	"time"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// Issue #408: hovering a resource type renders its list in the right pane;
// the preview must use the same sort the drilled-in list will (sort memory,
// then view config, then Name ascending) instead of API fetch order.

func previewPodsRT() model.ResourceTypeEntry {
	return model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
}

func TestPreviewListAppliesRememberedSort(t *testing.T) {
	origViews := ui.ConfigViews
	t.Cleanup(func() { ui.ConfigViews = origViews })
	ui.ConfigViews = nil

	m := basePush80v2Model()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx" // pin explicitly: the sortMemory key below depends on it
	ref := ui.ResourceRef{Version: "v1", Resource: "pods", Kind: "Pod"}
	m.sortMemory = map[string]sortPref{
		sortMemoryKey(ref, "test-ctx"): {column: "Status", ascending: false},
	}

	items := []model.Item{
		{Name: "a", Kind: "Pod", Status: "Running"}, // priority 0
		{Name: "b", Kind: "Pod", Status: "Failed"},  // priority 2
		{Name: "c", Kind: "Pod", Status: "Pending"}, // priority 1
	}
	res, _ := m.Update(resourcesLoadedMsg{items: items, forPreview: true, rt: previewPodsRT()})
	rm := res.(Model)

	want := []string{"b", "c", "a"} // Status descending: Failed, Pending, Running
	if len(rm.rightItems) != 3 {
		t.Fatalf("rightItems = %d items, want 3", len(rm.rightItems))
	}
	for i, name := range want {
		if rm.rightItems[i].Name != name {
			t.Fatalf("rightItems[%d] = %q, want %q (remembered Status desc sort not applied)", i, rm.rightItems[i].Name, name)
		}
	}
}

func TestPreviewListAppliesViewConfigSort(t *testing.T) {
	origViews := ui.ConfigViews
	t.Cleanup(func() { ui.ConfigViews = origViews })
	v, err := ui.BuildView(&ui.ConfigView{SortColumn: "Restarts:desc"})
	if err != nil {
		t.Fatalf("BuildView: %v", err)
	}
	ui.ConfigViews = map[string]*ui.View{"/v1/pods": v}

	m := basePush80v2Model()
	m.nav.Level = model.LevelResourceTypes

	items := []model.Item{
		{Name: "a", Kind: "Pod", Restarts: "1"},
		{Name: "b", Kind: "Pod", Restarts: "7"},
		{Name: "c", Kind: "Pod", Restarts: "3"},
	}
	res, _ := m.Update(resourcesLoadedMsg{items: items, forPreview: true, rt: previewPodsRT()})
	rm := res.(Model)

	want := []string{"b", "c", "a"}
	for i, name := range want {
		if rm.rightItems[i].Name != name {
			t.Fatalf("rightItems[%d] = %q, want %q (view Restarts:desc sort not applied)", i, rm.rightItems[i].Name, name)
		}
	}
}

func TestPreviewListDefaultsToNameAscending(t *testing.T) {
	origViews := ui.ConfigViews
	t.Cleanup(func() { ui.ConfigViews = origViews })
	ui.ConfigViews = nil

	m := basePush80v2Model()
	m.nav.Level = model.LevelResourceTypes

	items := []model.Item{
		{Name: "charlie", Kind: "Pod"},
		{Name: "alpha", Kind: "Pod"},
		{Name: "bravo", Kind: "Pod"},
	}
	res, _ := m.Update(resourcesLoadedMsg{items: items, forPreview: true, rt: previewPodsRT()})
	rm := res.(Model)

	want := []string{"alpha", "bravo", "charlie"}
	for i, name := range want {
		if rm.rightItems[i].Name != name {
			t.Fatalf("rightItems[%d] = %q, want %q (default Name asc sort not applied)", i, rm.rightItems[i].Name, name)
		}
	}
}

// Synthetic previews (no resource type) must keep their fetch order.
func TestPreviewListWithoutResourceTypeKeepsOrder(t *testing.T) {
	m := basePush80v2Model()
	m.nav.Level = model.LevelResourceTypes

	items := []model.Item{
		{Name: "zeta"},
		{Name: "alpha"},
	}
	res, _ := m.Update(resourcesLoadedMsg{items: items, forPreview: true})
	rm := res.(Model)

	if rm.rightItems[0].Name != "zeta" || rm.rightItems[1].Name != "alpha" {
		t.Fatalf("synthetic preview reordered: %q, %q", rm.rightItems[0].Name, rm.rightItems[1].Name)
	}
}

// sortItemsByColumn carries the Event LastSeen default override so a default-
// sorted Event preview matches the drilled-in Event list (most recent first).
func TestSortItemsByColumnEventDefaultLastSeen(t *testing.T) {
	now := time.Now()
	items := []model.Item{
		{Name: "old", Kind: "Event", LastSeen: now.Add(-time.Hour)},
		{Name: "new", Kind: "Event", LastSeen: now},
		{Name: "mid", Kind: "Event", LastSeen: now.Add(-time.Minute)},
	}
	sortItemsByColumn(items, sortColDefault, true, "Event")
	want := []string{"new", "mid", "old"}
	for i, name := range want {
		if items[i].Name != name {
			t.Fatalf("items[%d] = %q, want %q", i, items[i].Name, name)
		}
	}
}
