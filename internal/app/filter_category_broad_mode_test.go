package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// Filter (`f`) at LevelResourceTypes used to expand by category
// unconditionally — typing `f ing` would pull in every Networking item
// because the category name contains "ing". The new contract mirrors
// search: plain `f` filters by name only; Tab (filterBroadMode) opts
// in to category-expansion.

func TestVisibleMiddleItems_FilterCategoryGatedOnBroadMode(t *testing.T) {
	items := []model.Item{
		{Name: "Pods", Category: "Workloads"},
		{Name: "Deployments", Category: "Workloads"},
		{Name: "Services", Category: "Networking"},
		{Name: "Ingresses", Category: "Networking"},
		{Name: "NetworkPolicies", Category: "Networking"},
	}

	t.Run("default (no Tab): only name matches", func(t *testing.T) {
		m := Model{
			nav:               model.NavigationState{Level: model.LevelResourceTypes},
			middleItems:       items,
			filterText:        "ing",
			allGroupsExpanded: true,
		}
		visible := m.visibleMiddleItems()
		names := itemNames(visible)
		assert.Equal(t, []string{"Ingresses"}, names,
			"plain `f ing` must match only resource type names — not pull in "+
				"every Networking member just because the category contains 'ing'")
	})

	t.Run("broad mode at LevelResourceTypes: expands by matched category", func(t *testing.T) {
		// Tab + `f ing`: also pulls in every item under categories
		// whose name matches. Networking matches "ing" → all
		// Networking items appear, plus the Ingresses name match.
		m := Model{
			nav:               model.NavigationState{Level: model.LevelResourceTypes},
			middleItems:       items,
			filterText:        "ing",
			filterBroadMode:   true,
			allGroupsExpanded: true,
		}
		visible := m.visibleMiddleItems()
		names := itemNames(visible)
		assert.ElementsMatch(t,
			[]string{"Services", "Ingresses", "NetworkPolicies"},
			names,
			"with Tab on, all members of a category-matched group should appear")
	})

	t.Run("broad mode outside LevelResourceTypes: no category expansion", func(t *testing.T) {
		// At LevelResources items still carry a Category field, but
		// the bar isn't rendered. Tab there means "also match column
		// values" — not category expansion.
		levelItems := []model.Item{
			{Name: "alpha", Category: "Argo CD"},
			{Name: "beta", Category: "Argo CD"},
			{Name: "monitoring-pod", Category: ""},
		}
		m := Model{
			nav:             model.NavigationState{Level: model.LevelResources},
			middleItems:     levelItems,
			filterText:      "argo",
			filterBroadMode: true,
		}
		visible := m.visibleMiddleItems()
		names := itemNames(visible)
		assert.Empty(t, names,
			"Tab outside LevelResourceTypes must NOT expand by category")
	})
}

func itemNames(items []model.Item) []string {
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.Name)
	}
	return names
}
