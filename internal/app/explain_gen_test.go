package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// The API Explorer fetches answer asynchronously. A reply that was already in
// flight when the user switched tabs must not drop the new tab into the
// Explorer with the old tab's schema (TASK-876).

// explainGenTabsModel returns a two-tab model with the API Explorer open on
// tab 0 and a live explain session.
func explainGenTabsModel(t *testing.T) Model {
	t.Helper()
	m := basePush80Model()
	m.tabs = []TabState{{nav: m.nav}, {nav: m.nav}}
	m.mode = modeExplain
	m.explainResource = "deployments"
	m.explainAPIVersion = "apps/v1"
	m.explainPath = "spec"
	m.explainFields = []model.ExplainField{
		{Name: "replicas", Type: "<integer>", Path: "spec.replicas"},
	}
	m.beginExplainSession()
	require.NotZero(t, m.explainGen, "opening the Explorer must start a generation")
	return m
}

// switchToSecondTab leaves tab 0 for tab 1, which is a plain explorer view.
func switchToSecondTab(m *Model) {
	m.saveCurrentTab()
	m.tabs[1].mode = modeExplorer
	_ = m.loadTab(1)
}

func TestExplainLoaded_StaleGenerationDoesNotHijackNewTab(t *testing.T) {
	m := explainGenTabsModel(t)
	stale := m.explainGen

	switchToSecondTab(&m)
	require.Equal(t, modeExplorer, m.mode)

	result, _ := m.Update(explainLoadedMsg{
		gen:    stale,
		fields: []model.ExplainField{{Name: "template", Type: "<PodTemplateSpec>", Path: "spec.template"}},
		title:  "deployments (apps/v1)",
		path:   "spec",
	})
	rm := result.(Model)

	assert.Equal(t, modeExplorer, rm.mode, "a reply from the tab left behind must not open the Explorer here")
	assert.Empty(t, rm.explainFields, "the new tab's schema must stay untouched")
	assert.Empty(t, rm.explainTitle)
	assert.False(t, rm.loading, "leaving the tab must stop the spinner of the fetch it abandoned")
}

func TestExplainLoaded_StaleReplyLeavesTheNewSpinnerUp(t *testing.T) {
	m := explainGenTabsModel(t)
	stale := m.explainGen

	// The user leaves the Explorer and opens it again straight away. The
	// first fetch answers while the second is still running.
	m.exitExplainView()
	m.beginExplainSession()
	m.loading = true

	result, _ := m.Update(explainLoadedMsg{gen: stale, path: "spec"})
	rm := result.(Model)

	assert.True(t, rm.loading, "a stale reply must not take down the spinner of the fetch that replaced it")
}

func TestExitExplainView_StopsTheSpinner(t *testing.T) {
	m := explainGenTabsModel(t)
	m.loading = true

	m.exitExplainView()

	assert.False(t, m.loading, "the abandoned fetch will never answer, so nothing else clears it")
}

func TestExplainLoaded_CurrentGenerationApplies(t *testing.T) {
	m := explainGenTabsModel(t)

	result, _ := m.Update(explainLoadedMsg{
		gen:    m.explainGen,
		fields: []model.ExplainField{{Name: "template", Type: "<PodTemplateSpec>", Path: "spec.template"}},
		title:  "deployments (apps/v1)",
		path:   "spec",
	})
	rm := result.(Model)

	assert.Equal(t, modeExplain, rm.mode)
	assert.Len(t, rm.explainFields, 1)
	assert.Equal(t, "template", rm.explainFields[0].Name)
}

func TestExplainRecursive_StaleGenerationDropped(t *testing.T) {
	m := explainGenTabsModel(t)
	stale := m.explainGen

	switchToSecondTab(&m)

	result, _ := m.Update(explainRecursiveMsg{
		gen:     stale,
		matches: []model.ExplainField{{Name: "containers", Path: "spec.template.spec.containers"}},
		query:   "containers",
	})
	rm := result.(Model)

	assert.Equal(t, overlayNone, rm.overlay, "a stale recursive reply must not open the search overlay")
	assert.Empty(t, rm.explainRecursiveResults)
}

func TestExplainTreeLoaded_StaleGenerationDropped(t *testing.T) {
	m := explainGenTabsModel(t)
	m.explainTreeWanted = true
	stale := m.explainGen

	switchToSecondTab(&m)
	m.explainTreeWanted = true // the new tab wants a tree of its own

	result, _ := m.Update(explainTreeLoadedMsg{
		gen:    stale,
		fields: []model.ExplainField{{Name: "replicas", Type: "<integer>", Path: "spec.replicas"}},
		path:   m.explainPath,
	})
	rm := result.(Model)

	assert.False(t, rm.explainTree, "a stale tree reply must not switch the new tab into tree mode")
	assert.Empty(t, rm.explainTreeAll)
}

func TestExplainTreeDesc_StaleGenerationDropped(t *testing.T) {
	m := explainGenTabsModel(t)
	stale := m.explainGen

	// Tab 1 is in tree mode for the same resource, so only the generation
	// tells the two requests apart.
	switchToSecondTab(&m)
	m.mode = modeExplain
	m.explainTree = true
	m.explainResource = "deployments"
	m.explainAPIVersion = "apps/v1"
	m.explainTreeAll = []model.ExplainField{{Name: "replicas", Type: "<integer>", Path: "spec.replicas"}}
	m.explainFields = m.explainTreeAll
	// The stale batch's own marker, still waiting to be released.
	m.explainTreeDescInflight = map[string]uint64{"spec": stale}

	result, _ := m.Update(explainTreeDescMsg{
		gen:        stale,
		resource:   "deployments",
		apiVersion: "apps/v1",
		kctx:       m.effectiveContext(),
		parent:     "spec",
		fields:     []model.ExplainField{{Name: "replicas", Path: "spec.replicas", Description: "stale text"}},
	})
	rm := result.(Model)

	assert.Empty(t, rm.explainTreeAll[0].Description, "a stale description batch must not be merged")
	assert.NotContains(t, rm.explainTreeDescFetched, "spec", "and must not mark the level described")
	assert.NotContains(t, rm.explainTreeDescInflight, "spec",
		"but the level must be released, or it can never be described again")
}

func TestExplainTreeDesc_StaleBatchKeepsALiveMarkerForTheSameLevel(t *testing.T) {
	m := explainGenTabsModel(t)
	stale := m.explainGen

	// The new session asked for the same level, so its marker sits under the
	// same key. Releasing it here would let a second subprocess start.
	m.beginExplainSession()
	m.explainTree = true
	m.explainTreeAll = []model.ExplainField{{Name: "replicas", Type: "<integer>", Path: "spec.replicas"}}
	m.explainFields = m.explainTreeAll
	m.explainTreeDescInflight = map[string]uint64{"spec": m.explainGen}

	result, _ := m.Update(explainTreeDescMsg{
		gen:        stale,
		resource:   "deployments",
		apiVersion: "apps/v1",
		kctx:       m.effectiveContext(),
		parent:     "spec",
		fields:     []model.ExplainField{{Name: "replicas", Path: "spec.replicas", Description: "stale text"}},
	})
	rm := result.(Model)

	assert.Contains(t, rm.explainTreeDescInflight, "spec", "the live fetch's marker must survive")
	assert.Empty(t, rm.explainTreeAll[0].Description)
}

// A tab left with the API Explorer still loading comes back to an empty view:
// the switch cancels the fetch and the generation guard drops its reply, so
// nothing fills the level (TASK-878).

// explainLoadingTabsModel returns a two-tab model whose tab 0 has the API
// Explorer open with a fetch in flight and no fields yet.
func explainLoadingTabsModel() Model {
	m := basePush80Model()
	m.tabs = []TabState{{nav: m.nav}, {nav: m.nav, mode: modeExplorer}}
	m.mode = modeExplain
	m.explainResource = "deployments"
	m.explainAPIVersion = "apps/v1"
	m.explainPath = "spec"
	m.explainFields = nil
	m.loading = true
	m.beginExplainSession()
	return m
}

func TestLoadTab_ResumesAnExplainLeftUnfinished(t *testing.T) {
	m := explainLoadingTabsModel()

	m.saveCurrentTab()
	_ = m.loadTab(1)
	require.False(t, m.loading, "the abandoned fetch takes its spinner with it")

	m.saveCurrentTab()
	cmd := m.loadTab(0)

	require.Equal(t, modeExplain, m.mode)
	assert.NotNil(t, cmd, "returning to the tab must re-issue the schema fetch")
	assert.True(t, m.loading, "and show that it is loading again")
}

func TestLoadTab_DoesNotRefetchAnExplainThatAlreadyLoaded(t *testing.T) {
	m := explainLoadingTabsModel()
	m.explainFields = []model.ExplainField{{Name: "replicas", Path: "spec.replicas"}}
	m.explainTitle = "deployments (apps/v1) > spec"
	m.loading = false

	m.saveCurrentTab()
	_ = m.loadTab(1)
	m.saveCurrentTab()
	cmd := m.loadTab(0)

	assert.Nil(t, cmd, "the level is already on screen, so nothing needs fetching")
	assert.False(t, m.loading)
}

func TestLoadTab_LeavesANonExplainTabAlone(t *testing.T) {
	m := explainLoadingTabsModel()
	m.mode = modeExplorer

	m.saveCurrentTab()
	_ = m.loadTab(1)
	m.saveCurrentTab()
	cmd := m.loadTab(0)

	assert.Nil(t, cmd)
	assert.False(t, m.loading)
}

// kubectl explain of a primitive field answers with a description and no
// FIELDS section, so a level that loaded holds zero fields. Deciding from the
// field list would refetch it on every visit and flash a spinner over a level
// that is already right.
func TestLoadTab_DoesNotRefetchALevelThatLoadedWithNoFields(t *testing.T) {
	m := explainLoadingTabsModel()

	// The reply for spec.replicas: a description, no fields.
	mdl, _ := m.updateExplainLoaded(explainLoadedMsg{
		gen:         m.explainGen,
		description: "Number of desired pods.",
		title:       "deployments (apps/v1) > spec > replicas",
		path:        "spec.replicas",
	})
	m = mdl.(Model)
	require.Empty(t, m.explainFields, "a primitive field has no FIELDS section")
	require.NotEmpty(t, m.explainTitle, "but the reply still names the level")

	m.saveCurrentTab()
	_ = m.loadTab(1)
	m.saveCurrentTab()
	cmd := m.loadTab(0)

	assert.Nil(t, cmd, "the level arrived, so revisiting the tab must not refetch it")
	assert.False(t, m.loading, "and must not flash a spinner over it")
	assert.Equal(t, "Number of desired pods.", m.explainDesc, "the description stays on screen")
}
