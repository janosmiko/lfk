package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// explainTreeDescModel builds a tree-mode model whose flat level carried
// per-field descriptions (as every flat kubectl explain load does).
func explainTreeDescModel(t *testing.T) Model {
	t.Helper()
	m := explainTreeBaseModel()
	m.explainFields[0].Description = "Number of desired pods."
	m.explainFields[1].Description = "Template describes the pods that will be created."
	mdl, _ := m.updateExplainTreeLoaded(sampleTreeMsg())
	rm, ok := mdl.(Model)
	require.True(t, ok)
	require.True(t, rm.explainTree)
	return rm
}

// TestExplainTreeDesc_SeedsFromFlatLevel: the depth-0 tree rows are the same
// fields the flat view just showed — their descriptions must carry over into
// tree mode without any extra fetch.
func TestExplainTreeDesc_SeedsFromFlatLevel(t *testing.T) {
	m := explainTreeDescModel(t)
	assert.Equal(t, "Number of desired pods.", m.explainFields[0].Description)
	assert.Equal(t, "Template describes the pods that will be created.", m.explainFields[1].Description)
	// The tree root's level is fully described — no fetch needed for it.
	_, fetched := m.explainTreeDescFetched["spec"]
	assert.True(t, fetched, "the flat level's path must be marked as described")
}

// TestExplainTreeDesc_NoPathAsDescription: tree rows must not carry their
// schema path as a fake description (the old stopgap the right pane showed).
func TestExplainTreeDesc_NoPathAsDescription(t *testing.T) {
	m := explainTreeDescModel(t)
	for _, f := range m.explainFields {
		assert.NotEqual(t, f.Path, f.Description,
			"field %q must not use its path as description", f.Name)
	}
}

// TestExplainTreeDesc_CursorMoveFetchesUnknownLevel: moving the cursor onto a
// row whose level has not been described yet dispatches exactly one fetch for
// that level and remembers it as in flight.
func TestExplainTreeDesc_CursorMoveFetchesUnknownLevel(t *testing.T) {
	m := explainTreeDescModel(t)
	require.Equal(t, 1, m.explainCursor) // on "spec.template" (depth 0, described)

	// j → "spec.template.spec": its level ("spec.template") is unknown.
	mdl, cmd := m.handleExplainKey(key("j"))
	m = mdl.(Model)
	require.Equal(t, "spec.template.spec", m.explainFields[m.explainCursor].Path)
	assert.NotNil(t, cmd, "moving onto an undescribed level must fetch its descriptions")
	_, inflight := m.explainTreeDescInflight["spec.template"]
	assert.True(t, inflight)

	// k back, then j again: the level is in flight — no duplicate fetch.
	mdl, _ = m.handleExplainKey(key("k"))
	m = mdl.(Model)
	mdl, cmd = m.handleExplainKey(key("j"))
	m = mdl.(Model)
	assert.Nil(t, cmd, "an in-flight level must not be fetched twice")
}

// TestExplainTreeDesc_CursorMoveOnDescribedLevelNoFetch: rows whose level is
// already described never trigger a fetch.
func TestExplainTreeDesc_CursorMoveOnDescribedLevelNoFetch(t *testing.T) {
	m := explainTreeDescModel(t)
	m.explainCursor = 1
	mdl, cmd := m.handleExplainKey(key("k")) // onto "spec.replicas" (depth 0)
	m = mdl.(Model)
	assert.Nil(t, cmd)
	assert.Empty(t, m.explainTreeDescInflight)
}

// TestExplainTreeDesc_LoadedMergesByPath: an arriving description batch fills
// the matching tree rows (visible list and full tree) and marks the level
// described.
func TestExplainTreeDesc_LoadedMergesByPath(t *testing.T) {
	m := explainTreeDescModel(t)
	mdl, _ := m.handleExplainKey(key("j")) // trigger fetch for "spec.template"
	m = mdl.(Model)

	mdl = m.updateExplainTreeDescLoaded(explainTreeDescMsg{
		resource:   "deployments",
		apiVersion: m.explainAPIVersion,
		kctx:       m.effectiveContext(),
		parent:     "spec.template",
		fields: []model.ExplainField{
			{Name: "spec", Path: "spec.template.spec", Description: "Specification of the desired behavior of the pod."},
		},
	})
	m = mdl.(Model)

	assert.Equal(t, "Specification of the desired behavior of the pod.",
		m.explainFields[2].Description)
	assert.Equal(t, "Specification of the desired behavior of the pod.",
		m.explainTreeAll[2].Description)
	_, fetched := m.explainTreeDescFetched["spec.template"]
	assert.True(t, fetched)
	assert.Empty(t, m.explainTreeDescInflight)

	// Revisiting the row fetches nothing.
	m.explainCursor = 1
	mdl, cmd := m.handleExplainKey(key("j"))
	m = mdl.(Model)
	assert.Nil(t, cmd)
}

// TestExplainTreeDesc_StaleIdentityIgnored: a batch whose request identity
// (resource, API version, or kube context) no longer matches the explorer
// session (the user left and reopened it elsewhere) must not merge.
func TestExplainTreeDesc_StaleIdentityIgnored(t *testing.T) {
	m := explainTreeDescModel(t)
	stale := []explainTreeDescMsg{
		{resource: "pods", apiVersion: m.explainAPIVersion, kctx: m.effectiveContext()},
		{resource: "deployments", apiVersion: "apps/v1beta1", kctx: m.effectiveContext()},
		{resource: "deployments", apiVersion: m.explainAPIVersion, kctx: "other-cluster"},
	}
	for _, msg := range stale {
		msg.parent = "spec.template"
		msg.fields = []model.ExplainField{
			{Name: "spec", Path: "spec.template.spec", Description: "stale batch"},
		}
		mdl := m.updateExplainTreeDescLoaded(msg)
		m = mdl.(Model)
		assert.Empty(t, m.explainFields[2].Description,
			"stale batch %+v must not merge", msg)
		_, fetched := m.explainTreeDescFetched["spec.template"]
		assert.False(t, fetched, "stale batch %+v must not mark the level described", msg)
	}
}

// TestExplainTreeDesc_ErrorMarksLevelDescribed: a failed fetch must not
// retry-storm on every cursor move; the level is treated as described (it
// resets with the next tree load).
func TestExplainTreeDesc_ErrorMarksLevelDescribed(t *testing.T) {
	m := explainTreeDescModel(t)
	mdl, _ := m.handleExplainKey(key("j"))
	m = mdl.(Model)

	mdl = m.updateExplainTreeDescLoaded(explainTreeDescMsg{
		resource:   "deployments",
		apiVersion: m.explainAPIVersion,
		kctx:       m.effectiveContext(),
		parent:     "spec.template",
		err:        assert.AnError,
	})
	m = mdl.(Model)
	assert.Empty(t, m.explainTreeDescInflight)

	m.explainCursor = 1
	mdl, cmd := m.handleExplainKey(key("j"))
	m = mdl.(Model)
	assert.Nil(t, cmd, "a failed level must not refetch on every cursor move")
}
