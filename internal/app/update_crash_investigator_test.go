package app

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
)

func sampleCrashInfo() *k8s.CrashInvestigation {
	return &k8s.CrashInvestigation{
		Pod: k8s.PodSummary{Name: "p", Namespace: "default", Phase: "Running"},
		AppContainers: []k8s.ContainerCrash{
			{Name: "app", State: "Running", Ready: true},
			{
				Name: "sidecar", State: "Waiting", StateReason: "CrashLoopBackOff", RestartCount: 3,
				LastTermination: &k8s.ContainerTermination{Reason: "Error", ExitCode: 1},
			},
		},
	}
}

func TestUpdateCrashInvestigation_Success(t *testing.T) {
	m := baseModel()
	m.loading = true
	mdl, _ := m.updateCrashInvestigation(crashInvestigationMsg{info: sampleCrashInfo()})
	got := mdl.(Model)
	assert.False(t, got.loading)
	assert.Equal(t, overlayCrashInvestigator, got.overlay)
	require.NotNil(t, got.crashInv.data)
	assert.Equal(t, "sidecar", got.crashInv.activeContainer, "must default to first failing container")
	assert.Equal(t, crashInvTabSummary, got.crashInv.activeTab)
	assert.True(t, got.crashInv.showPrevious)
}

func TestUpdateCrashInvestigation_Error(t *testing.T) {
	m := baseModel()
	mdl, _ := m.updateCrashInvestigation(crashInvestigationMsg{err: errors.New("boom")})
	got := mdl.(Model)
	assert.NotEqual(t, overlayCrashInvestigator, got.overlay)
}

func TestUpdateCrashInvestigation_NilInfo(t *testing.T) {
	m := baseModel()
	mdl, _ := m.updateCrashInvestigation(crashInvestigationMsg{})
	got := mdl.(Model)
	assert.NotEqual(t, overlayCrashInvestigator, got.overlay)
}

func TestCrashInvestigator_TabCycle(t *testing.T) {
	m := baseModel()
	m.overlay = overlayCrashInvestigator
	m.crashInv = crashInvState{data: sampleCrashInfo(), activeTab: crashInvTabSummary, activeContainer: "app"}

	for _, want := range []crashInvTab{crashInvTabEvents, crashInvTabLogs, crashInvTabDescribe, crashInvTabSummary} {
		mdl, _ := m.handleCrashInvestigatorOverlayKey(tea.KeyPressMsg{Code: tea.KeyTab})
		m = mdl.(Model)
		assert.Equal(t, want, m.crashInv.activeTab)
	}
}

func TestCrashInvestigator_DirectJumpKeys(t *testing.T) {
	cases := map[string]crashInvTab{
		"1": crashInvTabSummary, "2": crashInvTabEvents, "3": crashInvTabLogs, "4": crashInvTabDescribe,
	}
	for key, want := range cases {
		t.Run(key, func(t *testing.T) {
			m := baseModel()
			m.overlay = overlayCrashInvestigator
			m.crashInv = crashInvState{data: sampleCrashInfo(), activeTab: crashInvTabSummary, activeContainer: "app"}
			mdl, _ := m.handleCrashInvestigatorOverlayKey(keyPressText(key))
			m = mdl.(Model)
			assert.Equal(t, want, m.crashInv.activeTab)
		})
	}
}

func TestCrashInvestigator_ContainerSwitch(t *testing.T) {
	m := baseModel()
	m.overlay = overlayCrashInvestigator
	m.crashInv = crashInvState{data: sampleCrashInfo(), activeTab: crashInvTabLogs, activeContainer: "app"}

	mdl, _ := m.handleCrashInvestigatorOverlayKey(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = mdl.(Model)
	assert.Equal(t, "sidecar", m.crashInv.activeContainer)
	assert.Equal(t, crashInvTabLogs, m.crashInv.activeTab, "tab must be preserved across container switch")
}

func TestCrashInvestigator_PreviousToggleOnLogsTabOnly(t *testing.T) {
	m := baseModel()
	m.overlay = overlayCrashInvestigator
	m.crashInv = crashInvState{data: sampleCrashInfo(), activeTab: crashInvTabSummary, showPrevious: true}

	// On Summary tab — p must NOT toggle.
	mdl, _ := m.handleCrashInvestigatorOverlayKey(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = mdl.(Model)
	assert.True(t, m.crashInv.showPrevious)

	// Switch to Logs — p toggles.
	m.crashInv.activeTab = crashInvTabLogs
	mdl, _ = m.handleCrashInvestigatorOverlayKey(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = mdl.(Model)
	assert.False(t, m.crashInv.showPrevious)
}

func TestCrashInvestigator_EscClosesAndClearsState(t *testing.T) {
	m := baseModel()
	m.overlay = overlayCrashInvestigator
	m.crashInv = crashInvState{data: sampleCrashInfo(), activeContainer: "app"}

	mdl, _ := m.handleCrashInvestigatorOverlayKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = mdl.(Model)
	assert.Equal(t, overlayNone, m.overlay)
	assert.Nil(t, m.crashInv.data)
}

func TestUpdateCrashInvestigation_RefreshPreservesState(t *testing.T) {
	m := baseModel()
	m.overlay = overlayCrashInvestigator
	m.crashInv = crashInvState{
		data:            sampleCrashInfo(),
		activeContainer: "app",
		activeTab:       crashInvTabLogs,
		showPrevious:    false,
	}

	// Re-fetch returns the same shape; activeContainer "app" still exists.
	mdl, _ := m.updateCrashInvestigation(crashInvestigationMsg{info: sampleCrashInfo()})
	got := mdl.(Model)
	assert.Equal(t, "app", got.crashInv.activeContainer, "active container preserved across refresh")
	assert.Equal(t, crashInvTabLogs, got.crashInv.activeTab)
	assert.False(t, got.crashInv.showPrevious)
}

func TestUpdateCrashInvestigation_RefreshFallsBackWhenContainerGone(t *testing.T) {
	m := baseModel()
	m.overlay = overlayCrashInvestigator
	m.crashInv = crashInvState{
		data:            sampleCrashInfo(),
		activeContainer: "removed-container",
	}

	mdl, _ := m.updateCrashInvestigation(crashInvestigationMsg{info: sampleCrashInfo()})
	got := mdl.(Model)
	// "removed-container" is not in sampleCrashInfo() — must fall back.
	assert.NotEqual(t, "removed-container", got.crashInv.activeContainer)
	assert.NotEmpty(t, got.crashInv.activeContainer)
}

func TestCrashInvestigator_ScrollDownUp(t *testing.T) {
	m := baseModel()
	m.overlay = overlayCrashInvestigator
	m.crashInv = crashInvState{
		data:            sampleCrashInfo(),
		activeTab:       crashInvTabLogs,
		activeContainer: "app",
		scroll:          map[crashInvScrollKey]int{},
	}

	mdl, _ := m.handleCrashInvestigatorOverlayKey(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = mdl.(Model)
	key := crashInvScrollKey{tab: crashInvTabLogs, container: "app"}
	assert.Equal(t, 1, m.crashInv.scroll[key])

	mdl, _ = m.handleCrashInvestigatorOverlayKey(tea.KeyPressMsg{Code: 'k', Text: "k"})
	m = mdl.(Model)
	assert.Equal(t, 0, m.crashInv.scroll[key], "k must clamp at 0, not go negative")
}

func TestCrashInvestigator_ScrollGTopBottom(t *testing.T) {
	m := baseModel()
	m.overlay = overlayCrashInvestigator
	key := crashInvScrollKey{tab: crashInvTabDescribe, container: "app"}
	m.crashInv = crashInvState{
		data:            sampleCrashInfo(),
		activeTab:       crashInvTabDescribe,
		activeContainer: "app",
		scroll:          map[crashInvScrollKey]int{key: 5},
	}

	// g — top
	mdl, _ := m.handleCrashInvestigatorOverlayKey(tea.KeyPressMsg{Code: 'g', Text: "g"})
	m = mdl.(Model)
	assert.Equal(t, 0, m.crashInv.scroll[key])

	// G — bottom (sentinel; renderer clamps)
	mdl, _ = m.handleCrashInvestigatorOverlayKey(tea.KeyPressMsg{Code: 'G', Text: "G"})
	m = mdl.(Model)
	assert.GreaterOrEqual(t, m.crashInv.scroll[key], 100,
		"G must set a large sentinel so the renderer's clamp lands on the last page")
}

func TestCrashInvestigator_ScrollKeyByTab(t *testing.T) {
	// Summary and Events share a pod-scoped key (no container), Logs
	// and Describe are container-scoped. Verify scrollKey reflects this.
	m := baseModel()
	m.crashInv = crashInvState{activeTab: crashInvTabSummary, activeContainer: "app"}
	assert.Equal(t, crashInvScrollKey{tab: crashInvTabSummary}, m.scrollKey())

	m.crashInv.activeTab = crashInvTabEvents
	assert.Equal(t, crashInvScrollKey{tab: crashInvTabEvents}, m.scrollKey())

	m.crashInv.activeTab = crashInvTabLogs
	assert.Equal(t, crashInvScrollKey{tab: crashInvTabLogs, container: "app"}, m.scrollKey())

	m.crashInv.activeTab = crashInvTabDescribe
	assert.Equal(t, crashInvScrollKey{tab: crashInvTabDescribe, container: "app"}, m.scrollKey())
}

func TestCrashInvestigator_ScrollHalfPage(t *testing.T) {
	m := baseModel()
	m.overlay = overlayCrashInvestigator
	m.crashInv = crashInvState{
		data:            sampleCrashInfo(),
		activeTab:       crashInvTabLogs,
		activeContainer: "app",
		scroll:          map[crashInvScrollKey]int{},
	}
	key := crashInvScrollKey{tab: crashInvTabLogs, container: "app"}

	mdl, _ := m.handleCrashInvestigatorOverlayKey(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	m = mdl.(Model)
	assert.Equal(t, 10, m.crashInv.scroll[key])

	mdl, _ = m.handleCrashInvestigatorOverlayKey(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	m = mdl.(Model)
	assert.Equal(t, 0, m.crashInv.scroll[key])
}
