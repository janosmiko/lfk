package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConstraintsView_EnterJumpsToRowObject(t *testing.T) {
	m := basePush80Model()
	m.mode = modeConstraints
	m.constraints.report = k8s.ConstraintReport{
		Rows: []k8s.ConstraintRow{{Kind: "Pod", Namespace: "kube-system", Name: "naked", Source: "PriorityClass"}},
	}
	m.constraints.cursor = 0
	m.nav.Context = "test"
	m.nav.Level = model.LevelResourceTypes

	podRT := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1"}
	m.discoveredResources = map[string][]model.ResourceTypeEntry{"test": {podRT}}
	m.middleItems = []model.Item{{Name: "Pods", Extra: podRT.ResourceRef()}}

	require.Empty(t, m.jumpBackStack)

	result, _ := m.handleConstraintsKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated, ok := result.(Model)
	require.True(t, ok)

	assert.Equal(t, modeExplorer, updated.mode, "closing the view returns to the pre-open mode")
	assert.Equal(t, "kube-system", updated.namespace)
	assert.Equal(t, "naked", updated.pendingTarget)
	assert.Len(t, updated.jumpBackStack, 1, "jump must record where it came from")
}

func TestConstraintsView_RendersHeadroomAndOmitsDeniedSource(t *testing.T) {
	m := basePush80Model()
	m.mode = modeConstraints
	m.constraints.title = "web-1"
	m.constraints.report = k8s.ConstraintReport{
		Rows: []k8s.ConstraintRow{{
			Source: "Quota", Kind: "ResourceQuota", Namespace: "default", Name: "compute-quota",
			Detail: "requests 500m cpu", Headroom: "1",
		}},
		Skipped: []string{"poddisruptionbudgets"},
	}

	out := stripANSI(m.View().Content)

	assert.Contains(t, out, "poddisruptionbudgets", "Skipped sources must show in the banner")
	assert.Contains(t, out, "compute-quota", "a populated source's row must still render")
	assert.NotContains(t, out, "web-pdb", "no row should exist for a source that was skipped")
}
