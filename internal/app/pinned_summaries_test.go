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

// allDefaultEntries returns discovery entries that resolve every key in
// defaultPinnedSummaries.
func allDefaultEntries() []model.ResourceTypeEntry {
	return []model.ResourceTypeEntry{
		{Kind: "Job", APIGroup: "batch", APIVersion: "v1", Resource: "jobs"},
		{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments"},
		{Kind: "Application", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "applications"},
		{Kind: "Kustomization", APIGroup: "kustomize.toolkit.fluxcd.io", APIVersion: "v1", Resource: "kustomizations"},
		{Kind: "Certificate", APIGroup: "cert-manager.io", APIVersion: "v1", Resource: "certificates"},
	}
}

// seedSummaryTestModel builds a per-context resource-types model whose
// discovery set is extraDiscovered plus a "CronJobs" entry (batch/cronjobs) —
// the type the tests pin as the user's "first pin", never itself a default.
func seedSummaryTestModel(t *testing.T, extraDiscovered []model.ResourceTypeEntry) Model {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := baseModelWithFakeClient()
	m.nav.Context = "prod"
	m.nav.Level = model.LevelResourceTypes
	m.allGroupsExpanded = true
	m.hiddenState = newHiddenTypesState()
	m.pinnedSummariesState = newPinnedState()
	discovered := append([]model.ResourceTypeEntry{
		{Kind: "CronJob", APIGroup: "batch", APIVersion: "v1", Resource: "cronjobs"},
	}, extraDiscovered...)
	m.discoveredResources["prod"] = discovered
	m.setMiddleItems(model.BuildSidebarItems(discovered))
	return m
}

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

// TestTogglePinnedSummary_EnforcesCap_ConfigPinsCountTowardCap verifies the cap
// check counts config-level pins too: effectivePinnedSummaries (what the
// dashboard actually renders) merges config pins first, then state pins,
// truncated to maxPinnedSummaries. A state-only count would let a new pin
// through even though it can never render (issue: cap check ignored config
// pins).
func TestTogglePinnedSummary_EnforcesCap_ConfigPinsCountTowardCap(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	origConfig := ui.ConfigPinnedSummaries
	keys := make([]string, 0, maxPinnedSummaries)
	for i := range maxPinnedSummaries {
		keys = append(keys, fmt.Sprintf("config-group%d/things", i))
	}
	ui.ConfigPinnedSummaries = keys
	t.Cleanup(func() { ui.ConfigPinnedSummaries = origConfig })

	m := hiddenTestModel(t)
	m.setCursor(cursorIndexOfItem(&m, "Gadgets"))
	m.pinnedSummariesState = newPinnedState()

	mdl, _ := m.togglePinnedSummary()
	m = mdl.(Model)
	assert.Empty(t, m.pinnedSummariesState.Contexts[m.nav.Context], "state must stay empty: the 11th pin (10 config + 1 new) is rejected")
	assert.Contains(t, m.statusMessage, "limit reached", "rejection must be surfaced to the user")
	assert.True(t, m.statusMessageErr)
}

// TestDefaultPinsDisabled verifies the config-driven gate on defaultPinnedSummaries:
// the key being absent leaves defaults enabled, an explicit `pinned_summaries: []`
// disables them, and a non-empty explicit list also leaves them "enabled" (moot,
// since effectivePinnedSummaries is non-empty in that case anyway).
func TestDefaultPinsDisabled(t *testing.T) {
	origSet, origList := ui.ConfigPinnedSummariesSet, ui.ConfigPinnedSummaries
	t.Cleanup(func() {
		ui.ConfigPinnedSummariesSet = origSet
		ui.ConfigPinnedSummaries = origList
	})

	ui.ConfigPinnedSummariesSet = false
	ui.ConfigPinnedSummaries = nil
	assert.False(t, defaultPinsDisabled(), "key absent -> defaults stay enabled")

	ui.ConfigPinnedSummariesSet = true
	ui.ConfigPinnedSummaries = nil
	assert.True(t, defaultPinsDisabled(), "explicit [] -> defaults disabled")

	ui.ConfigPinnedSummaries = []string{"batch/jobs"}
	assert.False(t, defaultPinsDisabled(), "explicit non-empty list -> defaults not disabled by this gate")
}

// TestTogglePinnedSummary_EnforcesCap_MixedConfigAndState verifies the cap
// check sums config and state pins together (3 config + 7 state = 10),
// rejecting an 11th interactive pin.
func TestTogglePinnedSummary_EnforcesCap_MixedConfigAndState(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	origConfig := ui.ConfigPinnedSummaries
	ui.ConfigPinnedSummaries = []string{"config-a/things", "config-b/things", "config-c/things"}
	t.Cleanup(func() { ui.ConfigPinnedSummaries = origConfig })

	m := hiddenTestModel(t)
	m.setCursor(cursorIndexOfItem(&m, "Gadgets"))
	m.pinnedSummariesState = newPinnedState()
	stateKeys := make([]string, 0, 7)
	for i := range 7 {
		stateKeys = append(stateKeys, fmt.Sprintf("state-group%d/things", i))
	}
	m.pinnedSummariesState.Contexts[m.nav.Context] = stateKeys

	mdl, _ := m.togglePinnedSummary()
	m = mdl.(Model)
	assert.Len(t, m.pinnedSummariesState.Contexts[m.nav.Context], 7, "cap rejects the 11th pin (3 config + 7 state)")
	assert.Contains(t, m.statusMessage, "limit reached", "rejection must be surfaced to the user")
	assert.True(t, m.statusMessageErr)
}

// TestTogglePinnedSummary_SeedsResolvedDefaultsOnFirstPin verifies that
// pinning a type while the built-in defaults are showing (issue: pin
// replaced the defaults instead of adding to them) copies the resolved
// default subset into state, in default order, before appending the new key.
func TestTogglePinnedSummary_SeedsResolvedDefaultsOnFirstPin(t *testing.T) {
	m := seedSummaryTestModel(t, allDefaultEntries())
	m.setCursor(cursorIndexOfItem(&m, "CronJobs"))
	require.NotEqual(t, -1, cursorIndexOfItem(&m, "CronJobs"))

	mdl, _ := m.togglePinnedSummary()
	m = mdl.(Model)

	want := append(append([]string(nil), defaultPinnedSummaries...), "batch/cronjobs")
	assert.Equal(t, want, m.pinnedSummariesState.Contexts["prod"], "seed keeps default order, new key appended last")
	assert.Equal(t, want, m.effectivePinnedSummaries(), "dashboard's effective list must show defaults + the new pin")
}

// TestTogglePinnedSummary_SeedsOnlyResolvedDefaultsOnPartialDiscovery
// verifies that only the defaults the cluster actually has get seeded: a CRD
// default the cluster lacks must never become an explicit pin (an explicit
// pin renders a "(not installed)" placeholder the user never saw as a
// default).
func TestTogglePinnedSummary_SeedsOnlyResolvedDefaultsOnPartialDiscovery(t *testing.T) {
	partial := []model.ResourceTypeEntry{
		{Kind: "Job", APIGroup: "batch", APIVersion: "v1", Resource: "jobs"},
		{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments"},
	}
	m := seedSummaryTestModel(t, partial)
	m.setCursor(cursorIndexOfItem(&m, "CronJobs"))

	mdl, _ := m.togglePinnedSummary()
	m = mdl.(Model)

	assert.Equal(t, []string{"batch/jobs", "apps/deployments", "batch/cronjobs"}, m.pinnedSummariesState.Contexts["prod"])
}

// TestTogglePinnedSummary_SeedsAllDefaultsWhenNoDiscoveryData verifies the
// over-seed fallback: when the active scope's discovery map entry is empty
// or absent, seed all five defaults rather than risk losing rows the user is
// currently seeing (no discovery data means we cannot tell which resolve).
func TestTogglePinnedSummary_SeedsAllDefaultsWhenNoDiscoveryData(t *testing.T) {
	m := seedSummaryTestModel(t, nil)
	// Wipe discovery entirely for this context, simulating "no discovery data
	// yet" while still selecting a pinnable item (BuildSidebarItems ran
	// against the CronJob-only set captured before the wipe).
	m.discoveredResources["prod"] = nil
	m.setCursor(cursorIndexOfItem(&m, "CronJobs"))

	mdl, _ := m.togglePinnedSummary()
	m = mdl.(Model)

	want := append(append([]string(nil), defaultPinnedSummaries...), "batch/cronjobs")
	assert.Equal(t, want, m.pinnedSummariesState.Contexts["prod"])
}

// TestTogglePinnedSummary_UnpinActiveDefaultKeepsOthers verifies selecting
// "Unpin summary" on an active default (Jobs) seeds the resolved defaults
// then removes just that key — the other defaults stay visible.
func TestTogglePinnedSummary_UnpinActiveDefaultKeepsOthers(t *testing.T) {
	m := seedSummaryTestModel(t, allDefaultEntries())
	m.setCursor(cursorIndexOfItem(&m, "Jobs"))
	require.NotEqual(t, -1, cursorIndexOfItem(&m, "Jobs"))

	mdl, _ := m.togglePinnedSummary()
	m = mdl.(Model)

	assert.NotContains(t, m.pinnedSummariesState.Contexts["prod"], "batch/jobs")
	for _, k := range defaultPinnedSummaries {
		if k == "batch/jobs" {
			continue
		}
		assert.Contains(t, m.pinnedSummariesState.Contexts["prod"], k, "other defaults must stay pinned")
	}
}

// TestTogglePinnedSummary_DefaultsDisabledNoSeed verifies an explicit
// pinned_summaries: [] disables the seed entirely: the first pin is a plain
// single-key pin, matching defaultPinsDisabled's contract.
func TestTogglePinnedSummary_DefaultsDisabledNoSeed(t *testing.T) {
	origSet, origList := ui.ConfigPinnedSummariesSet, ui.ConfigPinnedSummaries
	ui.ConfigPinnedSummariesSet = true
	ui.ConfigPinnedSummaries = nil
	t.Cleanup(func() {
		ui.ConfigPinnedSummariesSet = origSet
		ui.ConfigPinnedSummaries = origList
	})

	m := seedSummaryTestModel(t, allDefaultEntries())
	m.setCursor(cursorIndexOfItem(&m, "CronJobs"))

	mdl, _ := m.togglePinnedSummary()
	m = mdl.(Model)

	assert.Equal(t, []string{"batch/cronjobs"}, m.pinnedSummariesState.Contexts["prod"], "defaults disabled: no seeding")
}

// TestTogglePinnedSummary_ConfigPinsPresentNoSeed verifies that when config
// already supplies pins, defaults were never active, so the first state pin
// does not seed anything.
func TestTogglePinnedSummary_ConfigPinsPresentNoSeed(t *testing.T) {
	origConfig := ui.ConfigPinnedSummaries
	ui.ConfigPinnedSummaries = []string{"batch/jobs"}
	t.Cleanup(func() { ui.ConfigPinnedSummaries = origConfig })

	m := seedSummaryTestModel(t, allDefaultEntries())
	m.setCursor(cursorIndexOfItem(&m, "CronJobs"))

	mdl, _ := m.togglePinnedSummary()
	m = mdl.(Model)

	assert.Equal(t, []string{"batch/cronjobs"}, m.pinnedSummariesState.Contexts["prod"], "config pins present: defaults weren't active, no seeding")
}

// TestTogglePinnedSummary_UnionSeedsAgainstMemberDiscoveries verifies seeding
// in a named union set resolves defaults against the union of all member
// contexts' discoveries: a default resolves if ANY member cluster has it.
func TestTogglePinnedSummary_UnionSeedsAgainstMemberDiscoveries(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := baseModelWithFakeClient()
	m.unionMode = true
	m.nav.Context = UnionContextSentinel
	m.unionContexts = []string{"prod", "staging"}
	m.unionSetName = "team-a"
	m.nav.Level = model.LevelResourceTypes
	m.allGroupsExpanded = true
	m.hiddenState = newHiddenTypesState()
	m.pinnedSummariesState = newPinnedState()

	// Split the resolvable defaults across the two member contexts so
	// neither one alone would resolve all five; the union must.
	m.discoveredResources["prod"] = []model.ResourceTypeEntry{
		{Kind: "Job", APIGroup: "batch", APIVersion: "v1", Resource: "jobs"},
		{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments"},
		{Kind: "CronJob", APIGroup: "batch", APIVersion: "v1", Resource: "cronjobs"},
	}
	m.discoveredResources["staging"] = []model.ResourceTypeEntry{
		{Kind: "Application", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "applications"},
		{Kind: "Kustomization", APIGroup: "kustomize.toolkit.fluxcd.io", APIVersion: "v1", Resource: "kustomizations"},
		{Kind: "Certificate", APIGroup: "cert-manager.io", APIVersion: "v1", Resource: "certificates"},
	}
	m.setMiddleItems(model.BuildSidebarItems(m.discoveredResources["prod"]))
	m.setCursor(cursorIndexOfItem(&m, "CronJobs"))

	mdl, _ := m.togglePinnedSummary()
	m = mdl.(Model)

	want := append(append([]string(nil), defaultPinnedSummaries...), "batch/cronjobs")
	assert.Equal(t, want, m.pinnedSummariesState.UnionSets["team-a"])
}

// TestTogglePinnedSummary_SaveFailureRollsBackSeedAndToggle verifies that when
// the disk write fails after a seed + toggle, the in-memory state is rolled
// back to its exact pre-seed value (empty), not left half-seeded or
// half-toggled.
func TestTogglePinnedSummary_SaveFailureRollsBackSeedAndToggle(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked-state")
	require.NoError(t, os.WriteFile(blocked, []byte("x"), 0o600))
	t.Setenv("LFK_STATE_DIR", blocked)

	m := seedSummaryTestModel(t, allDefaultEntries())
	m.setCursor(cursorIndexOfItem(&m, "CronJobs"))

	mdl, cmd := m.togglePinnedSummary()
	m = mdl.(Model)

	assert.Empty(t, m.pinnedSummariesState.Contexts["prod"], "save failure rolls back both the seed and the toggle")
	assert.Contains(t, m.statusMessage, "Failed to save")
	assert.NotNil(t, cmd)
}
