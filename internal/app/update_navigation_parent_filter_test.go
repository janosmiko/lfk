package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// Regression: a filter committed at a child level (e.g. a typed-then-Enter'd
// query at LevelResourceTypes) must not survive into the parent level. The
// previous bug left m.filterText populated, so visibleMiddleItems silently
// filtered every cluster-picker entry whose name didn't match — making the
// context list look empty after backing out of a filtered view.
//
// The table covers every branch in navigateParent so a future early-return
// added above the filter-clear block can't reintroduce the leak.
func TestNavigateParent_ClearsFilter(t *testing.T) {
	cases := []struct {
		name      string
		setup     func() Model
		wantLevel model.Level
	}{
		{
			name: "LevelClusters is a no-op but still wipes any stale filter",
			setup: func() Model {
				m := basePush80Model()
				m.nav.Level = model.LevelClusters
				m.middleItems = []model.Item{{Name: "ctx-prod"}, {Name: "ctx-staging"}}
				return m
			},
			wantLevel: model.LevelClusters,
		},
		{
			name: "LevelResourceTypes → LevelClusters (the bug's original repro)",
			setup: func() Model {
				m := basePush80Model()
				m.nav.Level = model.LevelResourceTypes
				m.nav.Context = "test-ctx"
				m.leftItems = []model.Item{{Name: "ctx-prod"}, {Name: "ctx-staging"}}
				m.leftItemsHistory = [][]model.Item{nil}
				return m
			},
			wantLevel: model.LevelClusters,
		},
		{
			name: "LevelResources → LevelResourceTypes (discovery cached)",
			setup: func() Model {
				m := basePush80Model()
				m.nav.Level = model.LevelResources
				m.nav.Context = "test-ctx"
				m.leftItems = []model.Item{{Name: "Pods"}, {Name: "Deployments"}}
				m.leftItemsHistory = [][]model.Item{{{Name: "test-ctx"}}}
				m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
					{DisplayName: "Pods", Kind: "Pod", APIVersion: "v1", Resource: "pods"},
				}
				return m
			},
			wantLevel: model.LevelResourceTypes,
		},
		{
			name: "LevelOwned → LevelResources (no parent stack)",
			setup: func() Model {
				m := basePush80Model()
				m.nav.Level = model.LevelOwned
				m.nav.ResourceName = "deploy-1"
				m.leftItems = []model.Item{{Name: "deploy-1"}}
				m.leftItemsHistory = [][]model.Item{{{Name: "cluster"}}, {{Name: "Deployments"}}}
				return m
			},
			wantLevel: model.LevelResources,
		},
		{
			name: "LevelOwned → LevelOwned (nested via ownedParentStack)",
			setup: func() Model {
				m := basePush80Model()
				m.nav.Level = model.LevelOwned
				m.nav.ResourceName = "deploy-1"
				m.ownedParentStack = []ownedParentState{{
					resourceType: model.ResourceTypeEntry{Kind: "Application"},
					resourceName: "my-app",
					namespace:    "argocd",
				}}
				m.leftItems = []model.Item{{Name: "deploy-1"}}
				m.leftItemsHistory = [][]model.Item{{{Name: "cluster"}}, {{Name: "Apps"}}, {{Name: "app-1"}}}
				return m
			},
			wantLevel: model.LevelOwned,
		},
		{
			name: "LevelContainers (Pod parent) → LevelResources",
			setup: func() Model {
				m := basePush80Model()
				m.nav.Level = model.LevelContainers
				m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
				m.leftItems = []model.Item{{Name: "container-1"}}
				m.leftItemsHistory = [][]model.Item{{{Name: "cluster"}}, {{Name: "Pods"}}, {{Name: "pod-1"}}}
				return m
			},
			wantLevel: model.LevelResources,
		},
		{
			name: "LevelContainers (non-Pod parent) → LevelOwned",
			setup: func() Model {
				m := basePush80Model()
				m.nav.Level = model.LevelContainers
				m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment"}
				m.leftItems = []model.Item{{Name: "container-1"}}
				m.leftItemsHistory = [][]model.Item{{{Name: "cluster"}}, {{Name: "Deploys"}}, {{Name: "dep-1"}}}
				return m
			},
			wantLevel: model.LevelOwned,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.setup()
			// Simulate a filter the user typed at the child level. The text
			// is intentionally a non-matcher so any residual application
			// would empty out the parent's visible list.
			m.filterText = "zzz"
			m.filterInput.Set("zzz")
			m.filterActive = false // Enter committed the filter

			result, _ := m.navigateParent()
			rm := result.(Model)

			require.Equal(t, tc.wantLevel, rm.nav.Level)
			assert.Empty(t, rm.filterText, "filterText must clear when navigating to parent")
			assert.Empty(t, rm.filterInput.Value, "filterInput must clear when navigating to parent")
			assert.False(t, rm.filterActive, "filterActive must reset when navigating to parent")
		})
	}
}

// Companion to the table above: the cluster-picker repro path also asserts
// that visibleMiddleItems re-shows the context rows once the filter clears.
// Kept as a separate test because the other branches don't all populate
// middleItems with the parent-level rows on return.
func TestNavigateParentFromResourceTypes_FilterClearRevealsContexts(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.leftItems = []model.Item{{Name: "ctx-prod"}, {Name: "ctx-staging"}}
	m.leftItemsHistory = [][]model.Item{nil}
	m.filterText = "zzz"
	m.filterInput.Set("zzz")

	result, _ := m.navigateParent()
	rm := result.(Model)

	require.Equal(t, model.LevelClusters, rm.nav.Level)
	assert.Len(t, rm.visibleMiddleItems(), 2,
		"contexts must reappear in the cluster picker after navigating back")
}
