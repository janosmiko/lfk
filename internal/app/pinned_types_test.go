package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// resetPinGlobals restores the package-level pin state mutated by applyPinnedTypes
// so tests stay isolated.
func resetPinGlobals(t *testing.T) {
	t.Helper()
	origTypes := model.PinnedTypes
	origCfgGroups := ui.ConfigPinnedGroups
	origCfgTypes := ui.ConfigPinnedTypes
	t.Cleanup(func() {
		model.PinnedTypes = origTypes
		ui.ConfigPinnedGroups = origCfgGroups
		ui.ConfigPinnedTypes = origCfgTypes
	})
	model.PinnedTypes = nil
	ui.ConfigPinnedGroups = nil
	ui.ConfigPinnedTypes = nil
}

func TestApplyPinnedTypes_ContextScope(t *testing.T) {
	resetPinGlobals(t)
	m := Model{
		nav:         model.NavigationState{Context: "prod"},
		pinnedState: &PinnedState{Contexts: map[string][]string{"prod": {"apps/deployments"}}},
	}
	m.applyPinnedTypes()
	assert.Equal(t, []string{"apps/deployments"}, model.PinnedTypes)
}

func TestApplyPinnedTypes_NilStateAndNoConfig(t *testing.T) {
	resetPinGlobals(t)
	m := Model{nav: model.NavigationState{Context: "prod"}}
	m.applyPinnedTypes()
	assert.Empty(t, model.PinnedTypes)
}

func TestApplyPinnedTypes_ContextIsolation(t *testing.T) {
	resetPinGlobals(t)
	m := Model{
		nav: model.NavigationState{Context: "dev"},
		pinnedState: &PinnedState{Contexts: map[string][]string{
			"prod": {"apps/deployments"},
			"dev":  {"/pods"},
		}},
	}
	m.applyPinnedTypes()
	assert.Equal(t, []string{"/pods"}, model.PinnedTypes, "only the active context's pins apply")
}

func TestApplyPinnedTypes_ConfigGroupExpandsViaDiscovery(t *testing.T) {
	resetPinGlobals(t)
	ui.ConfigPinnedGroups = []string{"example.com"}
	m := Model{
		nav: model.NavigationState{Context: "prod"},
		discoveredResources: map[string][]model.ResourceTypeEntry{
			"prod": {
				{Kind: "Widget", APIGroup: "example.com", APIVersion: "v1", Resource: "widgets"},
				{Kind: "Gadget", APIGroup: "example.com", APIVersion: "v1", Resource: "gadgets"},
			},
		},
	}
	m.applyPinnedTypes()
	assert.ElementsMatch(t, []string{"example.com/widgets", "example.com/gadgets"}, model.PinnedTypes)
}

func TestApplyPinnedTypes_ConfigType(t *testing.T) {
	resetPinGlobals(t)
	ui.ConfigPinnedTypes = []string{"apps/deployments"}
	m := Model{nav: model.NavigationState{Context: "prod"}}
	m.applyPinnedTypes()
	assert.Equal(t, []string{"apps/deployments"}, model.PinnedTypes)
}

func TestApplyPinnedTypes_LegacyGroupMigratedInPlace(t *testing.T) {
	// Migration persists to disk; isolate to a temp state dir.
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	resetPinGlobals(t)
	m := Model{
		nav: model.NavigationState{Context: "prod"},
		pinnedState: &PinnedState{Contexts: map[string][]string{
			"prod": {"example.com"}, // legacy whole-group pin (no "/")
		}},
		discoveredResources: map[string][]model.ResourceTypeEntry{
			"prod": {
				{Kind: "Widget", APIGroup: "example.com", APIVersion: "v1", Resource: "widgets"},
				{Kind: "Gadget", APIGroup: "example.com", APIVersion: "v1", Resource: "gadgets"},
			},
		},
	}
	m.applyPinnedTypes()

	// Resolved pin keys surface in model.PinnedTypes.
	assert.ElementsMatch(t, []string{"example.com/widgets", "example.com/gadgets"}, model.PinnedTypes)
	// The legacy entry is rewritten in place to member type keys.
	assert.ElementsMatch(t,
		[]string{"example.com/widgets", "example.com/gadgets"},
		m.pinnedState.Contexts["prod"],
	)
}

func TestApplyPinnedTypes_LegacyGroupKeptWhenNoMembers(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	resetPinGlobals(t)
	m := Model{
		nav: model.NavigationState{Context: "prod"},
		pinnedState: &PinnedState{Contexts: map[string][]string{
			"prod": {"notinstalled.io"}, // legacy group, no discovered members
		}},
		discoveredResources: map[string][]model.ResourceTypeEntry{"prod": {}},
	}
	m.applyPinnedTypes()
	// Kept as-is so a pin for a not-yet-installed CRD group is not lost.
	require.Contains(t, m.pinnedState.Contexts["prod"], "notinstalled.io")
}

func TestApplyPinnedTypes_UnionSetScope(t *testing.T) {
	resetPinGlobals(t)
	m := Model{
		unionMode:     true,
		nav:           model.NavigationState{Context: UnionContextSentinel},
		unionSetName:  "staging",
		unionContexts: []string{"c1"},
		pinnedState: &PinnedState{UnionSets: map[string][]string{
			"staging": {"apps/deployments"},
		}},
		discoveredResources: map[string][]model.ResourceTypeEntry{
			"c1": {{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments"}},
		},
	}
	m.applyPinnedTypes()
	assert.Equal(t, []string{"apps/deployments"}, model.PinnedTypes)
}
