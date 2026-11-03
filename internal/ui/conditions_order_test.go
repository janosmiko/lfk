package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func TestConditionsByRecency(t *testing.T) {
	now := time.Now()
	in := []model.ConditionEntry{
		{Type: "ConsistentStateFound", LastTransitionTime: now.Add(-3 * time.Hour)},
		{Type: "NoTimeA"},
		{Type: "Drifted", LastTransitionTime: now.Add(-time.Minute)},
		{Type: "NoTimeB"},
		{Type: "Ready", LastTransitionTime: now.Add(-24 * time.Hour)},
	}

	got := conditionsByRecency(in)

	types := make([]string, len(got))
	for i, c := range got {
		types[i] = c.Type
	}
	assert.Equal(t, []string{"Drifted", "ConsistentStateFound", "Ready", "NoTimeA", "NoTimeB"}, types)
	assert.Equal(t, "ConsistentStateFound", in[0].Type, "input slice must not be reordered")
}

func TestRenderResourceSummary_ConditionsNewestFirst(t *testing.T) {
	now := time.Now()
	item := &model.Item{Name: "nc", Kind: "NodeClaim", Columns: []model.KeyValue{{Key: "Node", Value: "n1"}}, Conditions: []model.ConditionEntry{
		{Type: "Consolidatable", Status: "True", LastTransitionTime: now.Add(-2 * time.Hour)},
		{Type: "Drifted", Status: "True", LastTransitionTime: now.Add(-time.Minute)},
	}}

	view := stripANSI(RenderResourceSummary(item, "", 100, 60))

	drifted, consolidatable := strings.Index(view, "Drifted"), strings.Index(view, "Consolidatable")
	require.GreaterOrEqual(t, drifted, 0)
	require.GreaterOrEqual(t, consolidatable, 0)
	assert.Less(t, drifted, consolidatable)
}
