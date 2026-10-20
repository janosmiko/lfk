package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func pinnedOrderDiscovery() []model.ResourceTypeEntry {
	return []model.ResourceTypeEntry{
		{Kind: "Job", APIGroup: "batch", APIVersion: "v1", Resource: "jobs"},
		{Kind: "CronJob", APIGroup: "batch", APIVersion: "v1", Resource: "cronjobs"},
		{Kind: "Gadget", APIGroup: "example.com", APIVersion: "v1", Resource: "gadgets"},
	}
}

// pinnedOrderModel builds a dashboard-selected model with the given state pins.
func pinnedOrderModel(t *testing.T, pins ...string) Model {
	t.Helper()
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	m := baseModelWithFakeClient()
	m.nav.Context = "prod"
	m.nav.Level = model.LevelResourceTypes
	m.discoveredResources["prod"] = pinnedOrderDiscovery()
	m.pinnedSummariesState = newPinnedState()
	if len(pins) > 0 {
		m.pinnedSummariesState.Contexts["prod"] = pins
	}
	return withMiddleItem(m, model.Item{Name: "Cluster", Kind: "__overview__", Extra: "__overview__", Category: "Dashboards"})
}

func pinOrderKey(t *testing.T, m Model, k string) (Model, tea.Cmd) {
	t.Helper()
	mdl, cmd := m.Update(keyMsg(k))
	return mdl.(Model), cmd
}

func openedPinnedOrder(t *testing.T, m Model) Model {
	t.Helper()
	m, _ = pinOrderKey(t, m, ",")
	require.Equal(t, overlayPinnedOrder, m.overlay)
	return m
}

func pinnedOrderKeys(m Model) []string {
	keys := make([]string, len(m.pinnedOrder.items))
	for i, e := range m.pinnedOrder.items {
		keys[i] = e.key
	}
	return keys
}

func TestPinnedOrder_OpensOnDashboardOnly(t *testing.T) {
	m := pinnedOrderModel(t, "batch/jobs", "batch/cronjobs")
	openedPinnedOrder(t, m)

	plain := withMiddleItem(m, model.Item{Name: "Jobs", Kind: "Job", Extra: "batch/v1/jobs"})
	plain, _ = pinOrderKey(t, plain, ",")
	assert.NotEqual(t, overlayPinnedOrder, plain.overlay)

	mon := withMiddleItem(m, model.Item{Name: "Monitoring", Kind: "__monitoring__", Extra: "__monitoring__"})
	mon, _ = pinOrderKey(t, mon, ",")
	assert.NotEqual(t, overlayPinnedOrder, mon.overlay)
}

func TestPinnedOrder_ColumnToggleStillOpensAtResources(t *testing.T) {
	m := pinnedOrderModel(t)
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Job", APIGroup: "batch", APIVersion: "v1", Resource: "jobs"}
	m, _ = pinOrderKey(t, m, ",")
	assert.Equal(t, overlayColumnToggle, m.overlay)
}

func TestPinnedOrder_ConfigPinsAreLockedAndShown(t *testing.T) {
	orig := ui.ConfigPinnedSummaries
	ui.ConfigPinnedSummaries = []string{"example.com/gadgets"}
	t.Cleanup(func() { ui.ConfigPinnedSummaries = orig })

	m := openedPinnedOrder(t, pinnedOrderModel(t, "batch/jobs", "batch/cronjobs"))
	assert.Equal(t, []string{"example.com/gadgets", "batch/jobs", "batch/cronjobs"}, pinnedOrderKeys(m))
	assert.Equal(t, 1, m.pinnedOrder.cursor, "cursor starts on first unlocked row")

	m, _ = pinOrderKey(t, m, "K")
	assert.Equal(t, []string{"example.com/gadgets", "batch/jobs", "batch/cronjobs"}, pinnedOrderKeys(m), "cannot swap into a locked row")

	view := stripANSI(func() string { s, _, _ := m.renderOverlayPinnedOrder(); return s }())
	assert.Contains(t, view, "config")
}

func TestPinnedOrder_CursorAndMoveKeys(t *testing.T) {
	m := openedPinnedOrder(t, pinnedOrderModel(t, "batch/jobs", "batch/cronjobs", "example.com/gadgets"))

	m, _ = pinOrderKey(t, m, "j")
	assert.Equal(t, 1, m.pinnedOrder.cursor)
	assert.Equal(t, []string{"batch/jobs", "batch/cronjobs", "example.com/gadgets"}, pinnedOrderKeys(m), "j moves only the cursor")

	m, _ = pinOrderKey(t, m, "J")
	assert.Equal(t, []string{"batch/jobs", "example.com/gadgets", "batch/cronjobs"}, pinnedOrderKeys(m))
	assert.Equal(t, 2, m.pinnedOrder.cursor)

	m, _ = pinOrderKey(t, m, "J")
	assert.Equal(t, 2, m.pinnedOrder.cursor, "J at the bottom is a no-op")

	m, _ = pinOrderKey(t, m, "K")
	m, _ = pinOrderKey(t, m, "K")
	assert.Equal(t, []string{"batch/cronjobs", "batch/jobs", "example.com/gadgets"}, pinnedOrderKeys(m))
	assert.Equal(t, 0, m.pinnedOrder.cursor)

	m, _ = pinOrderKey(t, m, "k")
	assert.Equal(t, 0, m.pinnedOrder.cursor)
}

func TestPinnedOrder_EnterPersists(t *testing.T) {
	m := pinnedOrderModel(t, "batch/jobs", "batch/cronjobs")
	m.dashboardData = map[string]dashboardData{m.dashboardPreviewTargetContext(): {}}
	m.dashboardPreview = "stale"
	m = openedPinnedOrder(t, m)
	m, _ = pinOrderKey(t, m, "J")

	m, cmd := pinOrderKey(t, m, "enter")
	want := []string{"batch/cronjobs", "batch/jobs"}
	assert.Equal(t, want, m.pinnedSummariesState.Contexts["prod"])
	assert.Equal(t, want, loadPinnedSummariesState().Contexts["prod"])
	assert.NotContains(t, m.dashboardData, m.dashboardPreviewTargetContext())
	assert.Empty(t, m.dashboardPreview)
	assert.Equal(t, overlayNone, m.overlay)
	assert.Contains(t, m.statusMessage, "reordered")
	assert.NotNil(t, cmd)
}

func TestPinnedOrder_EscLeavesStateUnchanged(t *testing.T) {
	m := openedPinnedOrder(t, pinnedOrderModel(t, "batch/jobs", "batch/cronjobs"))
	m, _ = pinOrderKey(t, m, "J")
	m, _ = pinOrderKey(t, m, "esc")
	assert.Equal(t, overlayNone, m.overlay)
	assert.Equal(t, []string{"batch/jobs", "batch/cronjobs"}, m.pinnedSummariesState.Contexts["prod"])
	_, err := os.Stat(filepath.Join(os.Getenv("LFK_STATE_DIR"), pinnedSummariesFileName))
	assert.True(t, os.IsNotExist(err), "esc must not write the file")
}

func TestPinnedOrder_UnchangedEnterDoesNotWrite(t *testing.T) {
	m := openedPinnedOrder(t, pinnedOrderModel(t, "batch/jobs", "batch/cronjobs"))
	m, _ = pinOrderKey(t, m, "enter")
	assert.Equal(t, overlayNone, m.overlay)
	_, err := os.Stat(filepath.Join(os.Getenv("LFK_STATE_DIR"), pinnedSummariesFileName))
	assert.True(t, os.IsNotExist(err))
}

func TestPinnedOrder_DefaultsShowingStoresReorderedDefaults(t *testing.T) {
	m := pinnedOrderModel(t)
	m.discoveredResources["prod"] = allDefaultEntries()
	m = openedPinnedOrder(t, m)
	assert.Equal(t, defaultPinnedSummaries, pinnedOrderKeys(m))

	m, _ = pinOrderKey(t, m, "J")
	m, _ = pinOrderKey(t, m, "enter")
	want := append([]string{defaultPinnedSummaries[1], defaultPinnedSummaries[0]}, defaultPinnedSummaries[2:]...)
	assert.Equal(t, want, m.pinnedSummariesState.Contexts["prod"])
	assert.Equal(t, want, loadPinnedSummariesState().Contexts["prod"])
}

func TestPinnedOrder_UnionWithoutNamedSetRefuses(t *testing.T) {
	m := pinnedOrderModel(t, "batch/jobs", "batch/cronjobs")
	m.unionMode = true
	m.nav.Context = UnionContextSentinel
	m, cmd := pinOrderKey(t, m, ",")
	assert.NotEqual(t, overlayPinnedOrder, m.overlay)
	assert.Contains(t, m.statusMessage, "named union set")
	assert.True(t, m.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPinnedOrder_SaveFailureRollsBack(t *testing.T) {
	m := openedPinnedOrder(t, pinnedOrderModel(t, "batch/jobs", "batch/cronjobs"))
	blocked := filepath.Join(t.TempDir(), "blocked-state")
	require.NoError(t, os.WriteFile(blocked, []byte("x"), 0o600))
	t.Setenv("LFK_STATE_DIR", blocked)

	m, _ = pinOrderKey(t, m, "J")
	m, _ = pinOrderKey(t, m, "enter")
	assert.Equal(t, []string{"batch/jobs", "batch/cronjobs"}, m.pinnedSummariesState.Contexts["prod"])
	assert.Contains(t, m.statusMessage, "Failed to save pinned summaries")
	assert.True(t, m.statusMessageErr)
}

func TestPinnedOrder_CapTruncatedTailIsPreserved(t *testing.T) {
	orig := ui.ConfigMaxPinnedSummaries
	ui.ConfigMaxPinnedSummaries = 2
	t.Cleanup(func() { ui.ConfigMaxPinnedSummaries = orig })

	m := openedPinnedOrder(t, pinnedOrderModel(t, "batch/jobs", "batch/cronjobs", "example.com/gadgets"))
	assert.Equal(t, []string{"batch/jobs", "batch/cronjobs"}, pinnedOrderKeys(m))
	m, _ = pinOrderKey(t, m, "J")
	m, _ = pinOrderKey(t, m, "enter")
	assert.Equal(t, []string{"batch/cronjobs", "batch/jobs", "example.com/gadgets"}, m.pinnedSummariesState.Contexts["prod"])
}

func TestPinnedOrder_NothingToReorder(t *testing.T) {
	m := pinnedOrderModel(t, "batch/jobs")
	m, cmd := pinOrderKey(t, m, ",")
	assert.NotEqual(t, overlayPinnedOrder, m.overlay)
	assert.Contains(t, m.statusMessage, "Nothing to reorder")
	assert.NotNil(t, cmd)
}

func TestPinnedOrder_ViewShowsTitleAndDisplayNames(t *testing.T) {
	m := openedPinnedOrder(t, pinnedOrderModel(t, "batch/jobs", "example.com/gadgets"))
	m.width, m.height = 120, 40
	view := stripANSI(func() string { s, _, _ := m.renderOverlayPinnedOrder(); return s }())
	assert.Contains(t, view, "Pinned Summaries Order")
	assert.Contains(t, view, "Job")
	assert.Contains(t, view, "Gadget")
}

func TestPinnedOrder_HintBar(t *testing.T) {
	m := openedPinnedOrder(t, pinnedOrderModel(t, "batch/jobs", "batch/cronjobs"))
	assert.Contains(t, stripANSI(m.overlayHintBar()), "J/K")
}

func TestPinnedOrder_ConfigDuplicateInStateSurvivesSave(t *testing.T) {
	orig := ui.ConfigPinnedSummaries
	ui.ConfigPinnedSummaries = []string{"example.com/gadgets"}
	t.Cleanup(func() { ui.ConfigPinnedSummaries = orig })

	m := openedPinnedOrder(t, pinnedOrderModel(t, "batch/jobs", "example.com/gadgets", "batch/cronjobs"))
	m, _ = pinOrderKey(t, m, "J")
	m, _ = pinOrderKey(t, m, "enter")
	assert.Equal(t, []string{"batch/cronjobs", "batch/jobs", "example.com/gadgets"}, m.pinnedSummariesState.Contexts["prod"])
}

func TestPinnedOrder_ConfigDuplicateUnchangedEnterDoesNotWrite(t *testing.T) {
	orig := ui.ConfigPinnedSummaries
	ui.ConfigPinnedSummaries = []string{"example.com/gadgets"}
	t.Cleanup(func() { ui.ConfigPinnedSummaries = orig })

	m := openedPinnedOrder(t, pinnedOrderModel(t, "batch/jobs", "example.com/gadgets", "batch/cronjobs"))
	pinOrderKey(t, m, "enter")
	_, err := os.Stat(filepath.Join(os.Getenv("LFK_STATE_DIR"), pinnedSummariesFileName))
	assert.True(t, os.IsNotExist(err))
}

func TestPinnedOrder_CtrlCClearsState(t *testing.T) {
	m := openedPinnedOrder(t, pinnedOrderModel(t, "batch/jobs", "batch/cronjobs"))
	m, _ = pinOrderKey(t, m, "ctrl+c")
	assert.Empty(t, m.pinnedOrder.items)
}
