package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

func TestLoadSecurityAvailabilityNilManager(t *testing.T) {
	m := Model{}
	assert.Nil(t, m.loadSecurityAvailability())
}

func TestLoadSecurityAvailabilityDispatches(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "fake", Available: true})

	m := Model{securityManager: mgr}
	m.nav.Context = "kctx"

	cmd := m.loadSecurityAvailability()
	require.NotNil(t, cmd)
	msg := cmd()

	loaded, ok := msg.(securityAvailabilityLoadedMsg)
	require.True(t, ok)
	assert.Equal(t, "kctx", loaded.context)
	assert.True(t, loaded.availability["fake"], "fake source should be available")
}

func TestSecurityAvailabilityLoadedMsgUpdatesModel(t *testing.T) {
	m := Model{securityAvailabilityByName: make(map[string]bool)}
	m.nav.Context = "kctx"
	msg := securityAvailabilityLoadedMsg{
		context: "kctx",
		availability: map[string]bool{
			"trivy-operator": true,
			"heuristic":      true,
		},
	}
	updated := m.handleSecurityAvailabilityLoaded(msg)
	assert.True(t, updated.securityAvailabilityByName["trivy-operator"])
	assert.True(t, updated.securityAvailabilityByName["heuristic"])
}

func TestSecurityAvailabilityLoadedStaleContextDiscarded(t *testing.T) {
	m := Model{securityAvailabilityByName: make(map[string]bool)}
	m.nav.Context = "current"
	msg := securityAvailabilityLoadedMsg{
		context:      "stale",
		availability: map[string]bool{"trivy-operator": true},
	}
	updated := m.handleSecurityAvailabilityLoaded(msg)
	assert.False(t, updated.securityAvailabilityByName["trivy-operator"])
}

// installTestSecurityHook wires SecuritySourcesFn so BuildSidebarItems sees
// the provided sources during the test. Returns a cleanup that restores the
// prior hook. Tests that rely on the handler rebuilding sidebar items via
// BuildSidebarItems must install this hook; otherwise the default nil hook
// makes injectSecuritySourceItems a no-op and the tests can't observe the
// Security category.
func installTestSecurityHook(t *testing.T, sources ...model.SecuritySourceEntry) {
	t.Helper()
	prev := model.SecuritySourcesFn
	t.Cleanup(func() { model.SecuritySourcesFn = prev })
	model.SecuritySourcesFn = func() []model.SecuritySourceEntry {
		return sources
	}
}

// TestSecurityAvailabilityLoadedRebuildsLeftItemsAtLevelResources verifies
// the fix for the cold-start bug: when the user restores a session and lands
// at LevelResources, the parent column (leftItems) holds the resource types
// list — which was built with an empty availability map and therefore has
// no Security entries. When the probe result arrives, the handler must
// rebuild leftItems (and clear any stale itemCache entry for the resource
// types level) so that navigating back with `h` immediately shows the
// Security category.
func TestSecurityAvailabilityLoadedRebuildsLeftItemsAtLevelResources(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "trivy-operator", Available: true})

	m := Model{
		securityManager:            mgr,
		securityAvailabilityByName: make(map[string]bool),
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		itemCache:                  make(map[string][]model.Item),
		cursorMemory:               make(map[string]int),
	}
	m.nav.Level = model.LevelResources
	m.nav.Context = "kctx"
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true,
	}
	m.discoveredResources["kctx"] = []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}
	// leftItems starts with no Security entries (hook not installed yet).
	m.leftItems = model.BuildSidebarItems(m.discoveredResources["kctx"])
	for _, it := range m.leftItems {
		require.NotEqual(t, "Security", it.Category,
			"precondition: leftItems should have no Security entries yet")
	}
	// Install the hook AFTER the precondition so the rebuild picks up the
	// Trivy entry but the precondition check above still holds.
	installTestSecurityHook(t, model.SecuritySourceEntry{
		DisplayName: "Trivy",
		SourceName:  "trivy-operator",
		Icon:        "◈",
		Count:       -1,
	})

	// Availability probe completes and reports Trivy is available.
	updated := m.handleSecurityAvailabilityLoaded(securityAvailabilityLoadedMsg{
		context:      "kctx",
		availability: map[string]bool{"trivy-operator": true},
	})

	// leftItems must now contain the Trivy Security entry.
	var gotTrivy bool
	for _, it := range updated.leftItems {
		if it.Category == "Security" && it.Kind == "__security_trivy-operator__" {
			gotTrivy = true
			break
		}
	}
	assert.True(t, gotTrivy,
		"handler must rebuild leftItems so Security category appears at LevelResources")
}

// TestSecurityAvailabilityLoadedStillRebuildsMiddleAtLevelResourceTypes
// guards the existing behavior: at LevelResourceTypes, middleItems (not
// leftItems) holds the sidebar, so the original rebuild path must still fire.
func TestSecurityAvailabilityLoadedStillRebuildsMiddleAtLevelResourceTypes(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "heuristic", Available: true})

	m := Model{
		securityManager:            mgr,
		securityAvailabilityByName: make(map[string]bool),
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		itemCache:                  make(map[string][]model.Item),
		cursorMemory:               make(map[string]int),
	}
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "kctx"
	m.discoveredResources["kctx"] = []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}
	m.middleItems = model.BuildSidebarItems(m.discoveredResources["kctx"])
	installTestSecurityHook(t, model.SecuritySourceEntry{
		DisplayName: "Heuristic",
		SourceName:  "heuristic",
		Icon:        "◉",
		Count:       -1,
	})

	updated := m.handleSecurityAvailabilityLoaded(securityAvailabilityLoadedMsg{
		context:      "kctx",
		availability: map[string]bool{"heuristic": true},
	})

	var gotHeuristic bool
	for _, it := range updated.middleItems {
		if it.Category == "Security" && it.Kind == "__security_heuristic__" {
			gotHeuristic = true
			break
		}
	}
	assert.True(t, gotHeuristic,
		"middleItems must be rebuilt at LevelResourceTypes")
}

func TestRenderRightResourcesShowsFindingDetails(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{
		{
			Name: "CVE-2024-1234",
			Kind: "__security_finding__",
			Columns: []model.KeyValue{
				{Key: "Severity", Value: "CRIT"},
				{Key: "Title", Value: "CVE-2024-1234"},
				{Key: "Resource", Value: "deploy/api"},
			},
		},
	}
	out := m.renderRightResources(80, 20)
	assert.Contains(t, out, "CRIT")
	assert.Contains(t, out, "CVE-2024-1234")
	assert.Contains(t, out, "deploy/api")
}
