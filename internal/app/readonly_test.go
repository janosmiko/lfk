package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestIsMutatingAction(t *testing.T) {
	mutating := []string{
		"Delete", "Force Delete", "Force Finalize",
		"Edit", "Secret Editor", "ConfigMap Editor",
		"Scale", "Restart", "Rollback",
		"Exec", "Attach", "Shell",
		"Debug", "Debug Pod", "Debug Mount",
		"Port Forward",
		"Cordon", "Uncordon", "Drain",
		"Taint", "Untaint",
		"Trigger", "Stop", "Remove",
		"Labels / Annotations",
		"Permissions",
	}
	for _, label := range mutating {
		assert.True(t, isMutatingAction(label), "%q should be classified mutating", label)
	}

	readOnly := []string{
		"Logs", "Tail Logs", "Describe", "Events",
		"Resize", "Go to Pod", "Open in Browser",
		"Startup Analysis", "Alerts", "Visualize", "Vuln Scan",
		"", "unknown action", "Diff",
	}
	for _, label := range readOnly {
		assert.False(t, isMutatingAction(label), "%q must not be classified mutating", label)
	}
}

func TestReadOnlyBlockedMessage_Format(t *testing.T) {
	assert.Equal(t, "Read-only mode: Delete disabled", readOnlyBlockedMessage("Delete"))
	assert.Equal(t, "Read-only mode: Scale disabled", readOnlyBlockedMessage("Scale"))
}

func TestExecuteAction_ReadOnly_BlocksMutating(t *testing.T) {
	for label := range mutatingActions {
		t.Run(label, func(t *testing.T) {
			m := Model{
				readOnly: true,
				tabs:     []TabState{{}},
				width:    80, height: 40,
			}
			m.actionCtx = actionContext{kind: "Pod", name: "p", namespace: "default", context: "kind"}
			ret, _ := m.executeAction(label)
			result := ret.(Model)
			assert.Equal(t, readOnlyBlockedMessage(label), result.statusMessage)
			assert.True(t, result.statusMessageErr)
			// Sanity: the dispatcher must NOT have advanced into action-specific
			// state (e.g., overlayConfirm for Delete).
			assert.NotEqual(t, overlayConfirm, result.overlay)
			assert.NotEqual(t, overlayScaleInput, result.overlay)
		})
	}
}

func TestExecuteBulkAction_ReadOnly_BlocksMutating(t *testing.T) {
	for _, label := range []string{"Delete", "Force Delete", "Scale", "Restart", "Labels / Annotations"} {
		t.Run(label, func(t *testing.T) {
			m := Model{
				readOnly:  true,
				bulkMode:  true,
				bulkItems: []model.Item{{Name: "p1"}, {Name: "p2"}},
				tabs:      []TabState{{}},
				width:     80, height: 40,
			}
			ret, _ := m.executeBulkAction(label)
			result := ret.(Model)
			assert.Equal(t, readOnlyBlockedMessage(label), result.statusMessage)
			assert.True(t, result.statusMessageErr)
		})
	}
}

func TestHandleKeyReadOnlyToggle_InsideContext_FlipsActiveTab(t *testing.T) {
	// Inside a specific context, Ctrl+R locks/unlocks the active tab.
	m := Model{
		nav:  model.NavigationState{Level: model.LevelResources, Context: "prod"},
		tabs: []TabState{{}}, width: 80, height: 40,
	}
	require.False(t, m.readOnly)

	ret, _ := m.handleKeyReadOnlyToggle()
	result := ret.(Model)
	assert.True(t, result.readOnly)
	assert.Equal(t, "Read-only mode: ON", result.statusMessage)
	assert.False(t, result.statusMessageErr)

	ret2, _ := result.handleKeyReadOnlyToggle()
	result2 := ret2.(Model)
	assert.False(t, result2.readOnly)
	assert.Equal(t, "Read-only mode: OFF", result2.statusMessage)
}

func TestNavigateChildCluster_AppliesPerContextReadOnly(t *testing.T) {
	// Save and restore the package var so the test is hermetic.
	prev := ui.ConfigClusterReadOnly
	t.Cleanup(func() { ui.ConfigClusterReadOnly = prev })
	ui.ConfigClusterReadOnly = map[string]bool{"prod": true}

	m := newTestModelForNav()
	sel := &model.Item{Name: "prod"}
	ret, _ := m.navigateChildCluster(sel)
	result := ret.(Model)
	assert.True(t, result.readOnly, "prod context should activate read-only via per-context config")

	// dev has no per-context config -> falls through to global (false here).
	m2 := newTestModelForNav()
	sel2 := &model.Item{Name: "dev"}
	ret2, _ := m2.navigateChildCluster(sel2)
	result2 := ret2.(Model)
	assert.False(t, result2.readOnly, "dev context should not activate read-only")
}

func TestNavigateChildCluster_PerContextOverrideWinsOverConfig(t *testing.T) {
	prev := ui.ConfigClusterReadOnly
	t.Cleanup(func() { ui.ConfigClusterReadOnly = prev })
	ui.ConfigClusterReadOnly = map[string]bool{"dev": false}

	m := newTestModelForNav()
	m.contextROOverrides = map[string]bool{"dev": true}
	sel := &model.Item{Name: "dev"}
	ret, _ := m.navigateChildCluster(sel)
	result := ret.(Model)
	assert.True(t, result.readOnly, "session override (Ctrl+R on row) must win over per-context config")
}

func TestNavigateChildCluster_CliReadOnly_Sticky(t *testing.T) {
	m := newTestModelForNav()
	m.cliReadOnly = true
	sel := &model.Item{Name: "any-context-without-config"}
	ret, _ := m.navigateChildCluster(sel)
	result := ret.(Model)
	assert.True(t, result.readOnly, "CLI flag must remain sticky across context switches")
}

func TestUpdateContextsLoaded_AnnotatesPerContextReadOnly(t *testing.T) {
	prevGlobal := ui.ConfigReadOnly
	prevCluster := ui.ConfigClusterReadOnly
	t.Cleanup(func() {
		ui.ConfigReadOnly = prevGlobal
		ui.ConfigClusterReadOnly = prevCluster
	})
	ui.ConfigReadOnly = false
	ui.ConfigClusterReadOnly = map[string]bool{"prod": true}

	m := Model{
		nav:               model.NavigationState{Level: model.LevelClusters},
		tabs:              []TabState{{}},
		cursorMemory:      map[string]int{},
		itemCache:         map[string][]model.Item{},
		cacheFingerprints: map[string]string{},
		width:             80, height: 40,
	}
	msg := contextsLoadedMsg{items: []model.Item{
		{Name: "prod"},
		{Name: "dev"},
		{Name: "audit", Status: "current"},
	}}
	ret, _ := m.updateContextsLoaded(msg)
	result := ret.(Model)

	require.Len(t, result.middleItems, 3)
	assert.True(t, result.middleItems[0].ReadOnly, "prod has per-context RO config")
	assert.False(t, result.middleItems[1].ReadOnly, "dev has no per-context config and global is off")
	assert.False(t, result.middleItems[2].ReadOnly, "audit has neither")
}

func TestUpdateContextsLoaded_CliFlagMarksAllRowsReadOnly(t *testing.T) {
	prevGlobal := ui.ConfigReadOnly
	prevCluster := ui.ConfigClusterReadOnly
	t.Cleanup(func() {
		ui.ConfigReadOnly = prevGlobal
		ui.ConfigClusterReadOnly = prevCluster
	})
	ui.ConfigReadOnly = false
	ui.ConfigClusterReadOnly = map[string]bool{}

	m := Model{
		nav:               model.NavigationState{Level: model.LevelClusters},
		tabs:              []TabState{{}},
		cliReadOnly:       true,
		cursorMemory:      map[string]int{},
		itemCache:         map[string][]model.Item{},
		cacheFingerprints: map[string]string{},
		width:             80, height: 40,
	}
	msg := contextsLoadedMsg{items: []model.Item{{Name: "a"}, {Name: "b"}}}
	ret, _ := m.updateContextsLoaded(msg)
	result := ret.(Model)
	for _, it := range result.middleItems {
		assert.True(t, it.ReadOnly, "--read-only must mark every context row")
	}
}

func TestRecomputeReadOnly_Precedence(t *testing.T) {
	prevGlobal := ui.ConfigReadOnly
	prevCluster := ui.ConfigClusterReadOnly
	t.Cleanup(func() {
		ui.ConfigReadOnly = prevGlobal
		ui.ConfigClusterReadOnly = prevCluster
	})

	t.Run("cli flag wins over override and config", func(t *testing.T) {
		ui.ConfigReadOnly = false
		ui.ConfigClusterReadOnly = map[string]bool{}
		m := Model{cliReadOnly: true, contextROOverrides: map[string]bool{"dev": false}}
		m.recomputeReadOnly("dev")
		assert.True(t, m.readOnly, "--read-only must win even when an override says false")
	})

	t.Run("session override beats per-context config", func(t *testing.T) {
		ui.ConfigReadOnly = false
		ui.ConfigClusterReadOnly = map[string]bool{"dev": true}
		m := Model{contextROOverrides: map[string]bool{"dev": false}}
		m.recomputeReadOnly("dev")
		assert.False(t, m.readOnly, "Ctrl+R toggle off must override per-context config saying true")
	})

	t.Run("per-context config overrides global", func(t *testing.T) {
		ui.ConfigReadOnly = true
		ui.ConfigClusterReadOnly = map[string]bool{"dev": false}
		m := Model{}
		m.recomputeReadOnly("dev")
		assert.False(t, m.readOnly)
	})

	t.Run("global applies when no per-context entry", func(t *testing.T) {
		ui.ConfigReadOnly = true
		ui.ConfigClusterReadOnly = map[string]bool{}
		m := Model{}
		m.recomputeReadOnly("anything")
		assert.True(t, m.readOnly)
	})

	t.Run("downgrades when leaving a RO context for a non-RO one", func(t *testing.T) {
		// The recompute must reset to the destination context's effective
		// state instead of letting the previous tab's RO state leak.
		ui.ConfigReadOnly = false
		ui.ConfigClusterReadOnly = map[string]bool{"prod": true}
		m := Model{readOnly: true} // came from prod
		m.recomputeReadOnly("dev")
		assert.False(t, m.readOnly, "switching from RO context to non-RO context must clear RO")
	})
}

func TestHandleKeyReadOnlyToggle_AtClusterPicker_TogglesRowMarker(t *testing.T) {
	// At the cluster picker, Ctrl+R toggles the highlighted row's [RO]
	// state. The active tab's m.readOnly is unaffected (no current
	// context to lock); the change is recorded in contextROOverrides so
	// it persists and is honored on entry.
	m := Model{
		nav: model.NavigationState{Level: model.LevelClusters},
		middleItems: []model.Item{
			{Name: "prod"},
			{Name: "dev"},
		},
		cursors:           [5]int{1, 0, 0, 0, 0}, // highlight "dev" (index 1)
		tabs:              []TabState{{}},
		itemCache:         map[string][]model.Item{},
		cacheFingerprints: map[string]string{},
		width:             80, height: 40,
	}
	ret, _ := m.handleKeyReadOnlyToggle()
	result := ret.(Model)
	assert.False(t, result.readOnly, "picker toggle must not flip the active tab's RO state")
	assert.True(t, result.middleItems[1].ReadOnly, "highlighted row's [RO] marker must turn on")
	assert.False(t, result.middleItems[0].ReadOnly, "non-highlighted rows must be untouched")
	assert.True(t, result.contextROOverrides["dev"], "override map must record the toggle for the highlighted context")
	assert.Contains(t, result.statusMessage, "dev read-only: ON")

	ret2, _ := result.handleKeyReadOnlyToggle()
	result2 := ret2.(Model)
	assert.False(t, result2.middleItems[1].ReadOnly, "second press clears the row marker")
	assert.False(t, result2.contextROOverrides["dev"])
	assert.Contains(t, result2.statusMessage, "dev read-only: OFF")
}

func TestHandleKeyReadOnlyToggle_AtClusterPicker_CliFlagBlocks(t *testing.T) {
	// When --read-only is active, the row toggle is rejected — the CLI
	// flag forces every context RO and the toggle would be misleading.
	m := Model{
		nav:               model.NavigationState{Level: model.LevelClusters},
		middleItems:       []model.Item{{Name: "prod", ReadOnly: true}},
		cursors:           [5]int{0, 0, 0, 0, 0},
		cliReadOnly:       true,
		tabs:              []TabState{{}},
		itemCache:         map[string][]model.Item{},
		cacheFingerprints: map[string]string{},
		width:             80, height: 40,
	}
	ret, _ := m.handleKeyReadOnlyToggle()
	result := ret.(Model)
	assert.True(t, result.middleItems[0].ReadOnly, "CLI flag keeps the marker on")
	assert.True(t, result.statusMessageErr)
	assert.Contains(t, result.statusMessage, "--read-only")
}

func TestIsMutatingActionForKind_CustomActionsBlockedByDefault(t *testing.T) {
	// Custom actions are arbitrary user-defined shell commands. They must
	// be treated as mutating in read-only mode unless explicitly opted out
	// via ReadOnlySafe. This is the safer-by-default behavior so a user
	// who configures e.g. "Reboot via SSH" without thinking about RO mode
	// doesn't silently bypass the gate.
	prev := ui.ConfigCustomActions
	t.Cleanup(func() { ui.ConfigCustomActions = prev })
	ui.ConfigCustomActions = map[string][]ui.CustomAction{
		"Pod": {
			{Label: "Reboot via SSH", Command: "ssh ..."}, // unsafe (default)
			{Label: "Show owner refs", Command: "kubectl get -o ...", ReadOnlySafe: true},
		},
	}

	assert.True(t, isMutatingActionForKind("Pod", "Reboot via SSH"),
		"custom action without ReadOnlySafe must be treated as mutating")
	assert.False(t, isMutatingActionForKind("Pod", "Show owner refs"),
		"custom action with ReadOnlySafe=true must be allowed")

	// Built-in mutating labels still take effect regardless of kind.
	assert.True(t, isMutatingActionForKind("Pod", "Delete"))
	assert.True(t, isMutatingActionForKind("AnyKind", "Edit"))

	// Unknown labels for unknown kinds are not blocked (no false positives
	// for the in-context Ctrl+R toggle hint or other code paths that route
	// non-action labels through the predicate).
	assert.False(t, isMutatingActionForKind("Pod", "Logs"))
	assert.False(t, isMutatingActionForKind("UnknownKind", "Some Custom Label"))
}

func TestExecuteActionDefault_BlocksUnsafeCustomActionInReadOnly(t *testing.T) {
	prev := ui.ConfigCustomActions
	t.Cleanup(func() { ui.ConfigCustomActions = prev })
	ui.ConfigCustomActions = map[string][]ui.CustomAction{
		"Pod": {{Label: "Reboot via SSH", Command: "ssh root@$NODE shutdown -r now"}},
	}

	m := Model{
		readOnly:  true,
		actionCtx: actionContext{kind: "Pod", name: "p", namespace: "default", context: "kind"},
		tabs:      []TabState{{}},
		width:     80, height: 40,
	}
	ret, _ := m.executeActionDefault("Reboot via SSH")
	result := ret.(Model)
	assert.Equal(t, readOnlyBlockedMessage("Reboot via SSH"), result.statusMessage)
	assert.True(t, result.statusMessageErr)
}

func TestRefreshContextReadOnlyMarkers_SyncsAcrossTabs(t *testing.T) {
	// Multi-tab desync regression: contextROOverrides is on Model (shared)
	// but middleItems is per-tab. After a Ctrl+R toggle in one tab, other
	// tabs at LevelClusters carry stale markers until the next context
	// reload. refreshContextReadOnlyMarkers (called from loadTab) brings
	// them in sync immediately on tab switch.
	prev := ui.ConfigClusterReadOnly
	t.Cleanup(func() { ui.ConfigClusterReadOnly = prev })
	ui.ConfigClusterReadOnly = map[string]bool{}

	m := Model{
		nav: model.NavigationState{Level: model.LevelClusters},
		middleItems: []model.Item{
			{Name: "prod", ReadOnly: false}, // stale: override says true
			{Name: "dev", ReadOnly: true},   // stale: no override, no config -> false
		},
		contextROOverrides: map[string]bool{"prod": true},
		itemCache:          map[string][]model.Item{},
	}
	m.refreshContextReadOnlyMarkers()

	assert.True(t, m.middleItems[0].ReadOnly, "prod must reflect the override")
	assert.False(t, m.middleItems[1].ReadOnly, "dev must reflect no-override / no-config")
	// itemCache must be in sync so back-navigation re-shows the fresh markers.
	require.Contains(t, m.itemCache, "")
	assert.True(t, m.itemCache[""][0].ReadOnly)
	assert.False(t, m.itemCache[""][1].ReadOnly)
}

func TestRefreshContextReadOnlyMarkers_NoOpOutsideClusterPicker(t *testing.T) {
	// At deeper levels middleItems holds resource types / resources that
	// don't carry the ReadOnly flag. The function must not touch them.
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources, Context: "prod"},
		middleItems: []model.Item{
			{Name: "pod-a"},
			{Name: "pod-b"},
		},
		contextROOverrides: map[string]bool{"prod": true},
		itemCache:          map[string][]model.Item{},
	}
	m.refreshContextReadOnlyMarkers()
	assert.False(t, m.middleItems[0].ReadOnly, "non-context items must not pick up the flag")
	assert.False(t, m.middleItems[1].ReadOnly)
}

func TestHandleKeyReadOnlyToggle_InsideContext(t *testing.T) {
	// Inside a specific context, the toggle flips the active tab's RO
	// state directly. The override map is not touched.
	m := Model{
		nav:  model.NavigationState{Level: model.LevelResources, Context: "prod"},
		tabs: []TabState{{}}, width: 80, height: 40,
	}
	ret, _ := m.handleKeyReadOnlyToggle()
	result := ret.(Model)
	assert.True(t, result.readOnly)
	assert.Empty(t, result.contextROOverrides, "in-context toggle must not write to the override map")
}

// newTestModelForNav builds a minimal Model adequate for navigateChildCluster.
// The function is local to the readonly tests so other tests in this package
// aren't coupled to its shape.
func newTestModelForNav() Model {
	return Model{
		nav: model.NavigationState{Level: model.LevelClusters},
		tabs: []TabState{{
			nav:               model.NavigationState{Level: model.LevelClusters},
			cursorMemory:      map[string]int{},
			itemCache:         map[string][]model.Item{},
			cacheFingerprints: map[string]string{},
		}},
		discoveredResources: map[string][]model.ResourceTypeEntry{},
		discoveringContexts: map[string]bool{},
		cursorMemory:        map[string]int{},
		itemCache:           map[string][]model.Item{},
		cacheFingerprints:   map[string]string{},
		width:               80,
		height:              40,
	}
}
