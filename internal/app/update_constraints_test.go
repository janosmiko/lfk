package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenConstraintsView_RefusesKindWithoutPodTemplate(t *testing.T) {
	m := basePush80Model()
	m.setMiddleItems([]model.Item{{
		Name: "cfg", Namespace: "default", Kind: "ConfigMap", Raw: map[string]any{"kind": "ConfigMap"},
	}})
	m.setCursor(0)

	result, cmd := m.openConstraintsView()
	updated, ok := result.(Model)
	require.True(t, ok)

	assert.Equal(t, modeExplorer, updated.mode, "must not open the constraints view for a kind with no pod template")
	assert.True(t, updated.hasStatusMessage())
	assert.Contains(t, updated.statusMessage, "Constraints apply to workloads only")
	require.NotNil(t, cmd)
}

func TestOpenConstraintsView_OpensForEveryWorkloadKind(t *testing.T) {
	for _, kind := range constraintsWorkloadKinds {
		t.Run(kind, func(t *testing.T) {
			m := basePush80Model()
			m.setMiddleItems([]model.Item{{
				Name: "obj", Namespace: "default", Kind: kind, Raw: map[string]any{"kind": kind},
			}})
			m.setCursor(0)

			result, _ := m.openConstraintsView()
			updated, ok := result.(Model)
			require.True(t, ok)
			assert.Equal(t, modeConstraints, updated.mode, "must open the constraints view for %s", kind)
		})
	}
}

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

func TestConstraintsView_HiddenTargetLeavesNavigationUntouched(t *testing.T) {
	m := basePush80Model()
	m.mode = modeConstraints
	m.constraints.returnMode = modeExplorer
	m.constraints.report = k8s.ConstraintReport{
		Rows: []k8s.ConstraintRow{{Kind: "Pod", Namespace: "kube-system", Name: "naked", Source: "PriorityClass"}},
	}
	m.nav.Context = "test"
	m.nav.Level = model.LevelResourceTypes
	m.namespace = "default"
	m.discoveredResources = map[string][]model.ResourceTypeEntry{
		"test": {{Kind: "Pod", Resource: "pods", APIVersion: "v1"}},
	}
	m.middleItems = nil // the Pods row is filtered out of the sidebar

	result, _ := m.handleConstraintsKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated, ok := result.(Model)
	require.True(t, ok)

	assert.Equal(t, modeConstraints, updated.mode, "a failed jump must keep the view open")
	assert.Equal(t, "default", updated.namespace, "a failed jump must not switch namespace")
	assert.Empty(t, updated.jumpBackStack, "a failed jump must not record history")
}

func TestConstraintsView_QuitClearsPendingG(t *testing.T) {
	m := basePush80Model()
	m.mode = modeConstraints
	m.constraints.returnMode = modeExplorer

	armed, _ := m.handleConstraintsKey(tea.KeyPressMsg{Code: 'g', Text: "g"})
	m, ok := armed.(Model)
	require.True(t, ok)
	require.True(t, m.pendingG, "the first g arms the gg prefix")

	quit, _ := m.handleConstraintsKey(tea.KeyPressMsg{Code: 'q', Text: "q"})
	closed, ok := quit.(Model)
	require.True(t, ok)

	assert.Equal(t, modeExplorer, closed.mode)
	assert.False(t, closed.pendingG, "closing the view must not leave gg half-typed for the explorer")
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
