package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestCollectBuiltinToggleEntries_UsesViewDerivedHidden verifies the overlay's
// initial entry list reflects view-derived hidden state when no session
// override exists. Without this, the first live-apply silently commits
// "all builtins visible" as session state, wiping the view's effect.
func TestCollectBuiltinToggleEntries_UsesViewDerivedHidden(t *testing.T) {
	origV := ui.ConfigViews
	t.Cleanup(func() { ui.ConfigViews = origV })

	v, err := ui.BuildView(&ui.ConfigView{
		Columns: []string{"Name", "NodeName:.spec.nodeName", "Age"},
	})
	if err != nil {
		t.Fatal(err)
	}
	ui.ConfigViews = map[string]*ui.View{"pod": v}

	items := []model.Item{
		{Name: "a", Namespace: "default", Ready: "1/1", Restarts: "0", Status: "Running", Age: "5m"},
	}
	m := &Model{}
	entries := m.collectBuiltinToggleEntries(items, "pod")

	got := map[string]bool{}
	for _, e := range entries {
		got[e.key] = e.visible
	}
	assert.False(t, got["Namespace"], "Namespace omitted from view should be hidden in overlay")
	assert.False(t, got["Ready"], "Ready omitted from view should be hidden in overlay")
	assert.False(t, got["Restarts"], "Restarts omitted from view should be hidden in overlay")
	assert.False(t, got["Status"], "Status omitted from view should be hidden in overlay")
	assert.True(t, got["Age"], "Age listed in view should be visible in overlay")
}

func TestCollectBuiltinToggleEntries_SessionWinsOverView(t *testing.T) {
	origV := ui.ConfigViews
	t.Cleanup(func() { ui.ConfigViews = origV })

	v, _ := ui.BuildView(&ui.ConfigView{Columns: []string{"Name", "Age"}})
	ui.ConfigViews = map[string]*ui.View{"pod": v}

	items := []model.Item{
		{Name: "a", Namespace: "default", Ready: "1/1", Restarts: "0", Status: "Running", Age: "5m"},
	}
	m := &Model{
		hiddenBuiltinColumns: map[string][]string{
			"pod": {"Age"}, // session: hide Age (view would have kept it)
		},
	}
	entries := m.collectBuiltinToggleEntries(items, "pod")
	got := map[string]bool{}
	for _, e := range entries {
		got[e.key] = e.visible
	}
	assert.False(t, got["Age"], "session-hidden Age wins over view's Age inclusion")
	assert.True(t, got["Namespace"], "session sets only Age; other builtins are not view-hidden because session presence disables view fallback")
}
