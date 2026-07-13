package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// The state file round-trips through the same PinnedState shape the sidebar
// pins use, just at a different path.
func TestPinnedSummariesState_RoundTrip(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())

	s := loadPinnedSummariesState()
	require.NotNil(t, s)
	assert.Empty(t, s.Contexts)

	added := togglePinnedType(s, "ctx-a", "batch/jobs")
	assert.True(t, added)
	require.NoError(t, savePinnedSummariesState(s))

	reloaded := loadPinnedSummariesState()
	assert.Equal(t, []string{"batch/jobs"}, reloaded.Contexts["ctx-a"])

	removed := togglePinnedType(reloaded, "ctx-a", "batch/jobs")
	assert.False(t, removed)
	require.NoError(t, savePinnedSummariesState(reloaded))
	assert.Empty(t, loadPinnedSummariesState().Contexts["ctx-a"])
}

func TestPinnedSummariesState_CorruptFileStartsFresh(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LFK_STATE_DIR", dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pinned_summaries.yaml"), []byte("{not yaml"), 0o600))
	s := loadPinnedSummariesState()
	require.NotNil(t, s)
	assert.Empty(t, s.Contexts)
}

func TestEffectivePinnedSummaries_MergesConfigAndState(t *testing.T) {
	orig := ui.ConfigPinnedSummaries
	ui.ConfigPinnedSummaries = []string{"batch/jobs", "/pods"}
	t.Cleanup(func() { ui.ConfigPinnedSummaries = orig })

	m := Model{pinnedSummariesState: newPinnedState()}
	m.nav.Context = "ctx-a"
	m.pinnedSummariesState.Contexts["ctx-a"] = []string{"argoproj.io/applications", "batch/jobs"}

	got := m.effectivePinnedSummaries()
	assert.Equal(t, []string{"batch/jobs", "/pods", "argoproj.io/applications"}, got)
}

func TestEffectivePinnedSummaries_CapsAtMax(t *testing.T) {
	m := Model{pinnedSummariesState: newPinnedState()}
	m.nav.Context = "ctx-a"
	keys := make([]string, 0, maxPinnedSummaries+3)
	for i := range maxPinnedSummaries + 3 {
		keys = append(keys, fmt.Sprintf("group%d/things", i))
	}
	m.pinnedSummariesState.Contexts["ctx-a"] = keys
	assert.Len(t, m.effectivePinnedSummaries(), maxPinnedSummaries)
}

func TestIsSummaryPinned(t *testing.T) {
	m := Model{pinnedSummariesState: newPinnedState()}
	m.nav.Context = "ctx-a"
	m.pinnedSummariesState.Contexts["ctx-a"] = []string{"batch/jobs"}
	assert.True(t, m.isSummaryPinned("batch/jobs"))
	assert.False(t, m.isSummaryPinned("/pods"))
	assert.False(t, Model{}.isSummaryPinned("batch/jobs"), "nil state must not panic")
}

// TestTogglePinnedSummary_PinsPersistsAndReloadsDashboard verifies the
// action-menu toggle pins the selected type's summary, persists it per
// context, drops the stale dashboard cache so a refresh reflects the change,
// and un-pins on a second toggle.
func TestTogglePinnedSummary_PinsPersistsAndReloadsDashboard(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	orig := ui.ConfigDashboard
	ui.ConfigDashboard = true
	t.Cleanup(func() { ui.ConfigDashboard = orig })

	m := hiddenTestModel(t)
	m.setCursor(cursorIndexOfItem(&m, "Gadgets"))
	m.pinnedSummariesState = newPinnedState()

	mdl, cmd := m.togglePinnedSummary()
	m = mdl.(Model)
	key := model.PinKeyFromRef(m.selectedMiddleItem().Extra)
	assert.True(t, m.isSummaryPinned(key))
	require.NotNil(t, cmd)
	// Per-context mode with the dashboard enabled batches the eager reload
	// with the status clear. Executing a tea.Batch cmd only unwraps the
	// BatchMsg; its sub-cmds are not run.
	batch, ok := cmd().(tea.BatchMsg)
	require.True(t, ok, "expected a batch of reload + status clear")
	assert.Len(t, batch, 2)
	// Persisted to disk.
	assert.Contains(t, loadPinnedSummariesState().Contexts[m.nav.Context], key)

	// Toggle again unpins.
	mdl, _ = m.togglePinnedSummary()
	m = mdl.(Model)
	assert.False(t, m.isSummaryPinned(key))
}

// TestTogglePinnedSummary_DashboardDisabledSkipsReload verifies the eager
// reload respects the dashboard config gate: the pin still persists and the
// cached frames are invalidated, but no dashboard fetch is fired.
func TestTogglePinnedSummary_DashboardDisabledSkipsReload(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	orig := ui.ConfigDashboard
	ui.ConfigDashboard = false
	t.Cleanup(func() { ui.ConfigDashboard = orig })

	m := hiddenTestModel(t)
	m.setCursor(cursorIndexOfItem(&m, "Gadgets"))
	m.pinnedSummariesState = newPinnedState()
	m.dashboardPreview = "stale frame"
	m.dashboardEventsPreview = "stale events"
	m.dashboardData = map[string]dashboardData{m.nav.Context: {}}

	assert.Nil(t, m.summaryDashboardReloadCmd(), "config gate off must suppress the reload")

	mdl, cmd := m.togglePinnedSummary()
	m = mdl.(Model)
	key := model.PinKeyFromRef(m.selectedMiddleItem().Extra)
	assert.True(t, m.isSummaryPinned(key))
	assert.Contains(t, loadPinnedSummariesState().Contexts[m.nav.Context], key)
	// Frames invalidated so a later view recomposes fresh.
	assert.Empty(t, m.dashboardPreview)
	assert.Empty(t, m.dashboardEventsPreview)
	assert.NotContains(t, m.dashboardData, m.nav.Context)
	assert.NotNil(t, cmd, "status clear still scheduled")
}

// TestTogglePinnedSummary_UnionNamedSetSkipsReload verifies a named-union-set
// toggle persists to the union scope and invalidates the cache, but does not
// fire the eager reload: loadDashboard would fetch for unionContexts[0] while
// handleDashboardPartial filters on the sentinel, discarding every section.
// The union dashboard path reloads lazily via its own gated loader on view.
func TestTogglePinnedSummary_UnionNamedSetSkipsReload(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	orig := ui.ConfigDashboard
	ui.ConfigDashboard = true
	t.Cleanup(func() { ui.ConfigDashboard = orig })

	m := hiddenTestModel(t)
	m.setCursor(cursorIndexOfItem(&m, "Gadgets"))
	m.pinnedSummariesState = newPinnedState()
	m.unionMode = true
	m.nav.Context = UnionContextSentinel
	m.unionContexts = []string{"prod", "staging"}
	m.unionSetName = "team-a"
	m.dashboardPreview = "stale frame"
	m.dashboardData = map[string]dashboardData{UnionContextSentinel: {}}

	assert.Nil(t, m.summaryDashboardReloadCmd(), "union sentinel must suppress the reload even with the dashboard enabled")

	mdl, cmd := m.togglePinnedSummary()
	m = mdl.(Model)
	key := model.PinKeyFromRef(m.selectedMiddleItem().Extra)
	assert.True(t, m.isSummaryPinned(key))
	assert.Contains(t, loadPinnedSummariesState().UnionSets["team-a"], key)
	assert.Empty(t, m.dashboardPreview)
	assert.NotContains(t, m.dashboardData, UnionContextSentinel)
	assert.NotNil(t, cmd, "status clear still scheduled")
}

// TestSummaryDashboardReloadCmd_PerContextEnabled pins down the positive case
// of the reload gate: per-context mode with the dashboard enabled reloads.
func TestSummaryDashboardReloadCmd_PerContextEnabled(t *testing.T) {
	orig := ui.ConfigDashboard
	ui.ConfigDashboard = true
	t.Cleanup(func() { ui.ConfigDashboard = orig })

	m := hiddenTestModel(t)
	assert.NotNil(t, m.summaryDashboardReloadCmd())
}

// TestTogglePinnedSummary_EnforcesCap verifies the cap is checked before the
// toggle mutates state: pinning an 11th summary is rejected outright rather
// than added and then trimmed, and the user is told why.
func TestTogglePinnedSummary_EnforcesCap(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	m := hiddenTestModel(t)
	m.setCursor(cursorIndexOfItem(&m, "Gadgets"))
	m.pinnedSummariesState = newPinnedState()
	keys := make([]string, 0, maxPinnedSummaries)
	for i := range maxPinnedSummaries {
		keys = append(keys, fmt.Sprintf("group%d/things", i))
	}
	m.pinnedSummariesState.Contexts[m.nav.Context] = keys

	mdl, _ := m.togglePinnedSummary()
	m = mdl.(Model)
	assert.Len(t, m.pinnedSummariesState.Contexts[m.nav.Context], maxPinnedSummaries, "cap rejects the 11th pin")
	assert.Contains(t, m.statusMessage, "limit reached", "rejection must be surfaced to the user")
	assert.True(t, m.statusMessageErr)
}
