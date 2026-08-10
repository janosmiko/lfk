package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

func dependentsConfirmModel() Model {
	m := Model{width: 80, height: 24, pendingAction: "Delete", confirmAction: "web"}
	m.confirmPropagation = model.DeletePropagationBackground
	m.deps.count = &k8s.DependentCount{
		Total:  5,
		ByKind: map[string]int{"Pod": 3, "ReplicaSet": 2},
	}
	return m
}

func TestRenderOverlayConfirm_ShowsTheDependentCount(t *testing.T) {
	m := dependentsConfirmModel()

	out, _, _, _ := m.renderOverlayConfirm()

	assert.Contains(t, stripANSI(out), "3 pods, 2 replicasets also removed")
}

func TestRenderOverlayConfirm_TabUpdatesTheCountInPlace(t *testing.T) {
	m := dependentsConfirmModel()

	// Background: the dependents go with the target.
	assert.Contains(t, stripANSI(mustRenderConfirm(m)), "also removed")

	m.cycleDeletePropagation() // Foreground
	assert.Contains(t, stripANSI(mustRenderConfirm(m)), "also removed")

	m.cycleDeletePropagation() // Orphan
	assert.Contains(t, stripANSI(mustRenderConfirm(m)), "stay in the cluster")

	m.cycleDeletePropagation() // None
	assert.Contains(t, stripANSI(mustRenderConfirm(m)), "may stay (server decides)")

	// The dialog is still the delete confirm throughout; Tab never closed it.
	assert.Contains(t, stripANSI(mustRenderConfirm(m)), "web")
}

func TestRenderOverlayConfirm_ShowsAPlaceholderWhileCounting(t *testing.T) {
	m := Model{width: 80, height: 24, pendingAction: "Delete", confirmAction: "web"}
	m.deps.loading = true

	out, _, _, _ := m.renderOverlayConfirm()

	assert.Contains(t, stripANSI(out), "counting...")
}

func TestRenderOverlayConfirm_UnknownKindLeavesTheRowOut(t *testing.T) {
	m := Model{width: 80, height: 24, pendingAction: "Delete", confirmAction: "web"}

	out, _, _, _ := m.renderOverlayConfirm()

	assert.NotContains(t, stripANSI(out), "Dependents")
}

func TestRenderOverlayConfirmType_ShowsTheDependentCount(t *testing.T) {
	m := Model{width: 80, height: 24, pendingAction: "Force Delete", confirmTitle: "Confirm Force Delete"}
	m.confirmPropagation = model.DeletePropagationBackground
	m.deps.count = &k8s.DependentCount{Total: 1, ByKind: map[string]int{"Pod": 1}}

	out, _, h, _ := m.renderOverlayConfirmType()

	assert.Contains(t, stripANSI(out), "1 pod also removed")
	assert.Positive(t, h)
}

func mustRenderConfirm(m Model) string {
	out, _, _, _ := m.renderOverlayConfirm()
	return out
}
