package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// collapsedSidebarModel sits on the resource-type list in accordion mode with
// only the Workloads group open, so the Argo CD Applications row is hidden.
func collapsedSidebarModel() Model {
	m := basePush80Model()
	entries := append(model.SeedResources(), model.ResourceTypeEntry{
		Kind:       "Application",
		APIGroup:   "argoproj.io",
		APIVersion: "v1alpha1",
		Resource:   "applications",
		Namespaced: true,
	})
	m.discoveredResources["test-ctx"] = entries
	m.nav.Level = model.LevelResourceTypes
	m.nav.ResourceType = model.ResourceTypeEntry{}
	m.leftItemsHistory = [][]model.Item{{{Name: "test-ctx"}}}
	m.leftItems = []model.Item{{Name: "test-ctx"}}
	m.setMiddleItems(model.BuildSidebarItems(entries))
	m.allGroupsExpanded = false
	m.expandedGroup = "Workloads"
	return m
}

func TestExecuteResourceJump_CollapsedGroupOpensTarget(t *testing.T) {
	got, _ := collapsedSidebarModel().executeResourceJump("applications")
	m := got.(Model)

	require.Equal(t, model.LevelResources, m.nav.Level)
	assert.Equal(t, "applications", m.nav.ResourceType.Resource)
}

func TestExecuteResourceJump_GroupSelectorCollapsedOpensTarget(t *testing.T) {
	got, _ := collapsedSidebarModel().executeResourceJump("applications.argoproj.io")
	m := got.(Model)

	require.Equal(t, model.LevelResources, m.nav.Level)
	assert.Equal(t, "argoproj.io", m.nav.ResourceType.APIGroup)
}

func TestExecuteResourceJump_FilterHidingTargetOpensTarget(t *testing.T) {
	m := collapsedSidebarModel()
	m.allGroupsExpanded = true
	m.filterText = "pods"

	got, _ := m.executeResourceJump("applications")
	m = got.(Model)

	require.Equal(t, model.LevelResources, m.nav.Level)
	assert.Equal(t, "applications", m.nav.ResourceType.Resource)
}

func TestExecuteMonitoringCommand_FilterHidingDashboardOpensMonitoring(t *testing.T) {
	m := collapsedSidebarModel()
	m.filterText = "pods"

	got, _ := m.executeMonitoringCommand()
	m = got.(Model)

	sel := m.selectedMiddleItem()
	require.NotNil(t, sel)
	assert.Equal(t, "__monitoring__", sel.Extra)
	assert.True(t, m.fullscreenDashboard)
}
