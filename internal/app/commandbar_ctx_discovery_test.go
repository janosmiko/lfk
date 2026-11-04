package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// ctxSwitchModel sits on the pod list of a cluster without CRDs, while the
// discovery cache already knows that "argo" serves Argo CD Applications.
func ctxSwitchModel() Model {
	m := basePush80Model()
	m.nav.Context = "plain"
	m.allGroupsExpanded = true // production default (app_init)
	m.leftItems = model.BuildSidebarItems(model.SeedResources())
	m.leftItemsHistory = [][]model.Item{{{Name: "plain"}, {Name: "argo"}}}
	m.discoveredResources["argo"] = append(model.SeedResources(), model.ResourceTypeEntry{
		Kind:       "Application",
		APIGroup:   "argoproj.io",
		APIVersion: "v1alpha1",
		Resource:   "applications",
		Namespaced: true,
	})
	return m
}

func hasKind(items []model.Item, kind string) bool {
	for _, it := range items {
		if it.Kind == kind {
			return true
		}
	}
	return false
}

func TestExecuteContextCommand_UsesDestinationResourceTypes(t *testing.T) {
	m := ctxSwitchModel()
	require.False(t, hasKind(m.resourceTypeItems(), "Application"))

	got, _ := m.executeContextCommand("argo")
	m = got.(Model)

	assert.True(t, hasKind(m.resourceTypeItems(), "Application"), "completion must offer the destination's CRDs")
	assert.True(t, hasKind(m.leftItems, "Application"), "left column must show the destination's resource types")
	assert.True(t, m.discoveringContexts["argo"], "discovery must start for the destination context")
}

func TestExecuteContextCommand_SkipsDiscoveryAlreadyRefreshed(t *testing.T) {
	m := ctxSwitchModel()
	m.discoveryRefreshedContexts = map[string]bool{"argo": true}

	got, _ := m.executeContextCommand("argo")
	m = got.(Model)

	assert.False(t, m.discoveringContexts["argo"])
}

func TestExecuteResourceJump_FindsCRDAfterContextSwitch(t *testing.T) {
	m := ctxSwitchModel()
	got, _ := m.executeContextCommand("argo")
	m = got.(Model)

	got, _ = m.executeResourceJump("applications")
	m = got.(Model)

	require.Equal(t, model.LevelResources, m.nav.Level)
	assert.Equal(t, "applications", m.nav.ResourceType.Resource)
	assert.Equal(t, "argoproj.io", m.nav.ResourceType.APIGroup)
}
