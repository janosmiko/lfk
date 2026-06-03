package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestApplyKindSortDefault_FromMemory verifies a remembered sort preference is
// restored when re-entering a kind, overriding the built-in default.
func TestApplyKindSortDefault_FromMemory(t *testing.T) {
	orig := ui.ConfigViews
	t.Cleanup(func() { ui.ConfigViews = orig })
	ui.ConfigViews = nil

	ref := ui.ResourceRef{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"}
	m := &Model{sortMemory: map[string]sortPref{
		sortMemoryKey(ref, "ctx"): {column: "CPU", ascending: false},
	}}
	m.applyKindSortDefault(ref, "ctx")
	if m.sortColumnName != "CPU" {
		t.Fatalf("sortColumnName = %q, want CPU", m.sortColumnName)
	}
	if m.sortAscending {
		t.Fatalf("sortAscending = true, want false")
	}
}

// TestApplyKindSortDefault_MemoryWinsOverView verifies a runtime sort choice
// takes precedence over a configured view's default sort.
func TestApplyKindSortDefault_MemoryWinsOverView(t *testing.T) {
	orig := ui.ConfigViews
	t.Cleanup(func() { ui.ConfigViews = orig })
	v, err := ui.BuildView(&ui.ConfigView{SortColumn: "REV:desc"})
	if err != nil {
		t.Fatalf("BuildView: %v", err)
	}
	ui.ConfigViews = map[string]*ui.View{"apps/v1/deployments": v}

	ref := ui.ResourceRef{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"}
	m := &Model{sortMemory: map[string]sortPref{
		sortMemoryKey(ref, ""): {column: "Age", ascending: true},
	}}
	m.applyKindSortDefault(ref, "")
	if m.sortColumnName != "Age" || !m.sortAscending {
		t.Fatalf("sort = %q asc=%v, want Age asc (memory wins over view)", m.sortColumnName, m.sortAscending)
	}
}

// TestApplyKindSortDefault_ContextScoped verifies remembered sort is scoped per
// cluster context: a pref for one context does not leak to another.
func TestApplyKindSortDefault_ContextScoped(t *testing.T) {
	orig := ui.ConfigViews
	t.Cleanup(func() { ui.ConfigViews = orig })
	ui.ConfigViews = nil

	ref := ui.ResourceRef{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"}
	m := &Model{sortMemory: map[string]sortPref{
		sortMemoryKey(ref, "prod"): {column: "CPU", ascending: false},
	}}
	m.applyKindSortDefault(ref, "dev")
	if m.sortColumnName != sortColDefault || !m.sortAscending {
		t.Fatalf("sort = %q asc=%v, want default for unseen context", m.sortColumnName, m.sortAscending)
	}
}

// TestRememberSort_RoundTrip verifies a user sort change is stored and restored
// across a navigate-away/return cycle.
func TestRememberSort_RoundTrip(t *testing.T) {
	orig := ui.ConfigViews
	t.Cleanup(func() { ui.ConfigViews = orig })
	ui.ConfigViews = nil

	rt := model.ResourceTypeEntry{APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Kind: "Deployment"}
	m := &Model{sortMemory: make(map[string]sortPref)}
	m.nav = model.NavigationState{Level: model.LevelResources, Context: "ctx", ResourceType: rt}
	m.sortColumnName = "CPU"
	m.sortAscending = false
	m.rememberSort()

	// Simulate navigating away and back: default reset then re-resolve.
	m.sortColumnName = sortColDefault
	m.sortAscending = true
	m.applyResourceTypeSortDefault(rt, "ctx")
	if m.sortColumnName != "CPU" || m.sortAscending {
		t.Fatalf("sort = %q asc=%v, want CPU desc after round-trip", m.sortColumnName, m.sortAscending)
	}
}

// TestRememberSort_SkipsSyntheticResource verifies synthetic kinds (empty
// Resource) are not stored, avoiding degenerate keys.
func TestRememberSort_SkipsSyntheticResource(t *testing.T) {
	m := &Model{sortMemory: make(map[string]sortPref)}
	m.nav = model.NavigationState{Level: model.LevelResources, Context: "ctx"}
	m.sortColumnName = "CPU"
	m.rememberSort()
	if len(m.sortMemory) != 0 {
		t.Fatalf("sortMemory has %d entries, want 0 for synthetic resource", len(m.sortMemory))
	}
}

// TestForgetSort_RestoresViewDefault verifies the sort-reset action drops the
// remembered pref so the configured view default applies again on re-entry,
// rather than pinning Name ascending forever.
func TestForgetSort_RestoresViewDefault(t *testing.T) {
	orig := ui.ConfigViews
	t.Cleanup(func() { ui.ConfigViews = orig })
	v, err := ui.BuildView(&ui.ConfigView{SortColumn: "REV:desc"})
	if err != nil {
		t.Fatalf("BuildView: %v", err)
	}
	ui.ConfigViews = map[string]*ui.View{"apps/v1/deployments": v}

	rt := model.ResourceTypeEntry{APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Kind: "Deployment"}
	m := &Model{sortMemory: make(map[string]sortPref)}
	m.nav = model.NavigationState{Level: model.LevelResources, Context: "ctx", ResourceType: rt}

	// User customizes, then resets.
	m.sortColumnName = "CPU"
	m.sortAscending = false
	m.rememberSort()
	m.forgetSort()

	// Re-entry should resolve to the configured view default, not the
	// customized sort and not the built-in default.
	m.applyResourceTypeSortDefault(rt, "ctx")
	if m.sortColumnName != "REV" || m.sortAscending {
		t.Fatalf("sort = %q asc=%v, want REV desc (view default after reset)", m.sortColumnName, m.sortAscending)
	}
}

// TestRememberSort_SkipsWhenSortNotApplicable verifies no pref is stored at
// navigation levels where sort has no effect.
func TestRememberSort_SkipsWhenSortNotApplicable(t *testing.T) {
	rt := model.ResourceTypeEntry{APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Kind: "Deployment"}
	m := &Model{sortMemory: make(map[string]sortPref)}
	m.nav = model.NavigationState{Level: model.LevelResourceTypes, Context: "ctx", ResourceType: rt}
	m.sortColumnName = "CPU"
	m.rememberSort()
	if len(m.sortMemory) != 0 {
		t.Fatalf("sortMemory has %d entries, want 0 at non-sortable level", len(m.sortMemory))
	}
}

// TestRememberSort_PersistsAcrossRestart verifies a user sort change is written
// to the state file so it survives an app restart (issue #353): rememberSort
// persists, and a fresh loadSortMemory recovers the same preference.
func TestRememberSort_PersistsAcrossRestart(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())

	rt := model.ResourceTypeEntry{APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Kind: "Deployment"}
	m := &Model{sortMemory: make(map[string]sortPref)}
	m.nav = model.NavigationState{Level: model.LevelResources, Context: "ctx", ResourceType: rt}
	m.sortColumnName = "CPU"
	m.sortAscending = false
	m.rememberSort()

	// Simulate a restart: load fresh from disk.
	reloaded := loadSortMemory()
	ref := ui.ResourceRef{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"}
	got, ok := reloaded[sortMemoryKey(ref, "ctx")]
	if !ok {
		t.Fatalf("remembered sort not persisted; reloaded map = %v", reloaded)
	}
	if got.column != "CPU" || got.ascending {
		t.Fatalf("reloaded sort = %q asc=%v, want CPU desc", got.column, got.ascending)
	}
}

// TestForgetSort_PersistsRemoval verifies the sort-reset action persists the
// removal, so a reset survives a restart instead of resurrecting the old sort.
func TestForgetSort_PersistsRemoval(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())

	rt := model.ResourceTypeEntry{APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Kind: "Deployment"}
	m := &Model{sortMemory: make(map[string]sortPref)}
	m.nav = model.NavigationState{Level: model.LevelResources, Context: "ctx", ResourceType: rt}
	m.sortColumnName = "CPU"
	m.sortAscending = false
	m.rememberSort()
	m.forgetSort()

	if reloaded := loadSortMemory(); len(reloaded) != 0 {
		t.Fatalf("reloaded map has %d entries after reset, want 0", len(reloaded))
	}
}
