package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// deleteConfirmModel opens the single-resource delete confirm on a Job.
func deleteConfirmModel(t *testing.T) Model {
	t.Helper()
	m := basePush80Model()
	m.actionCtx = actionContext{
		context:      "test-ctx",
		namespace:    "default",
		name:         "backup-nightly",
		kind:         "Job",
		resourceType: model.ResourceTypeEntry{APIGroup: "batch", APIVersion: "v1", Resource: "jobs", Kind: "Job", Namespaced: true},
	}
	ret, _ := m.executeActionDelete()
	got, ok := ret.(Model)
	require.True(t, ok)
	require.Equal(t, overlayConfirm, got.overlay)
	return got
}

func TestDeleteConfirm_SeedsPolicyFromConfig(t *testing.T) {
	original := ui.ConfigDeletePropagationPolicy
	t.Cleanup(func() { ui.ConfigDeletePropagationPolicy = original })

	ui.ConfigDeletePropagationPolicy = model.DeletePropagationForeground
	m := deleteConfirmModel(t)
	assert.Equal(t, model.DeletePropagationForeground, m.deletePropagation())
}

// A policy chosen for one delete must not carry into the next one.
func TestDeleteConfirm_DoesNotCarryPolicyBetweenDeletes(t *testing.T) {
	original := ui.ConfigDeletePropagationPolicy
	t.Cleanup(func() { ui.ConfigDeletePropagationPolicy = original })
	ui.ConfigDeletePropagationPolicy = model.DeletePropagationBackground

	m := deleteConfirmModel(t)
	m.confirmPropagation = model.DeletePropagationOrphan

	ret, _ := m.executeActionDelete()
	reopened, ok := ret.(Model)
	require.True(t, ok)
	assert.Equal(t, model.DeletePropagationBackground, reopened.deletePropagation(),
		"reopening the confirm must reseed from config, not keep the previous choice")
}

func TestDeleteConfirm_TabCyclesPolicy(t *testing.T) {
	m := deleteConfirmModel(t)
	m.confirmPropagation = model.DeletePropagationBackground

	want := []model.DeletePropagation{
		model.DeletePropagationForeground,
		model.DeletePropagationOrphan,
		model.DeletePropagationNone,
		model.DeletePropagationBackground,
	}
	for _, expected := range want {
		ret, _ := m.handleConfirmOverlayKey(keyMsg("tab"))
		next, ok := ret.(Model)
		require.True(t, ok)
		m = next
		assert.Equal(t, expected, m.deletePropagation())
	}
}

// Tab must not close the overlay or commit the delete.
func TestDeleteConfirm_TabKeepsOverlayOpen(t *testing.T) {
	m := deleteConfirmModel(t)
	ret, cmd := m.handleConfirmOverlayKey(keyMsg("tab"))
	after, ok := ret.(Model)
	require.True(t, ok)
	assert.Equal(t, overlayConfirm, after.overlay)
	assert.Nil(t, cmd, "tab must not dispatch the delete")
	assert.Equal(t, "Delete", after.pendingAction)
}

func TestDeleteConfirm_RendersCascadeRowAndHint(t *testing.T) {
	m := deleteConfirmModel(t)
	m.confirmPropagation = model.DeletePropagationOrphan

	body, _, _, _ := m.renderOverlayContent()
	plain := stripANSI(body)
	assert.Contains(t, plain, "Cascade")
	assert.Contains(t, plain, "Orphan")
	assert.Contains(t, stripANSI(m.overlayHintBarDialog()), "cascade")
}

// Orphan and None can both leave workloads running, so neither may look
// interchangeable with a cascading policy.
func TestDeleteConfirm_RiskyPoliciesAreCalledOut(t *testing.T) {
	m := deleteConfirmModel(t)

	risky := map[model.DeletePropagation]string{
		model.DeletePropagationOrphan: "dependents kept",
		model.DeletePropagationNone:   "server default",
	}
	for policy, note := range risky {
		m.confirmPropagation = policy
		body, _, _, _ := m.renderOverlayContent()
		assert.Contains(t, stripANSI(body), note, "%s must explain itself", policy)
	}

	for _, safe := range []model.DeletePropagation{
		model.DeletePropagationBackground,
		model.DeletePropagationForeground,
	} {
		m.confirmPropagation = safe
		body, _, _, _ := m.renderOverlayContent()
		plain := stripANSI(body)
		assert.NotContains(t, plain, "dependents kept", "%s must not be warned", safe)
		assert.NotContains(t, plain, "server default", "%s must not be warned", safe)
	}
}

// forceDeleteConfirmModel opens the type-to-confirm force delete on a Pod.
func forceDeleteConfirmModel(t *testing.T) Model {
	t.Helper()
	m := basePush80Model()
	m.actionCtx = actionContext{
		context:      "test-ctx",
		namespace:    "default",
		name:         "web-0",
		kind:         "Pod",
		resourceType: model.ResourceTypeEntry{APIVersion: "v1", Resource: "pods", Kind: "Pod", Namespaced: true},
	}
	ret, _ := m.executeActionForceDelete()
	got, ok := ret.(Model)
	require.True(t, ok)
	require.Equal(t, overlayConfirmType, got.overlay)
	return got
}

func TestForceDeleteConfirm_ShowsCascadeRowAndHint(t *testing.T) {
	m := forceDeleteConfirmModel(t)

	body, _, _, _ := m.renderOverlayContent()
	plain := stripANSI(body)
	assert.Contains(t, plain, "Cascade")
	assert.Contains(t, plain, "Background")
	assert.Contains(t, plain, "DELETE", "the type-to-confirm row must survive")
	assert.Contains(t, stripANSI(m.overlayHintBarDialog()), "cascade")
}

// kubectl cannot express None, so the force-delete cycle must skip it.
func TestForceDeleteConfirm_TabCycleSkipsNone(t *testing.T) {
	m := forceDeleteConfirmModel(t)

	seen := map[model.DeletePropagation]bool{}
	for range 6 {
		ret, _ := m.handleConfirmTypeOverlayKey(keyMsg("tab"))
		next, ok := ret.(Model)
		require.True(t, ok)
		m = next
		assert.NotEqual(t, model.DeletePropagationNone, m.deletePropagation(),
			"force delete must never land on None")
		seen[m.deletePropagation()] = true
	}
	assert.Len(t, seen, 3, "cycle must visit exactly the three cascading policies")
}

// A configured default of none must not leak into the kubectl path.
func TestForceDeleteConfirm_ClampsConfiguredNone(t *testing.T) {
	original := ui.ConfigDeletePropagationPolicy
	t.Cleanup(func() { ui.ConfigDeletePropagationPolicy = original })
	ui.ConfigDeletePropagationPolicy = model.DeletePropagationNone

	m := forceDeleteConfirmModel(t)
	assert.Equal(t, model.DeletePropagationBackground, m.deletePropagation())
}

// Tab must not commit the delete or bypass the typed confirmation.
func TestForceDeleteConfirm_TabDoesNotCommit(t *testing.T) {
	m := forceDeleteConfirmModel(t)
	ret, cmd := m.handleConfirmTypeOverlayKey(keyMsg("tab"))
	after, ok := ret.(Model)
	require.True(t, ok)
	assert.Equal(t, overlayConfirmType, after.overlay)
	assert.Nil(t, cmd)
	assert.Empty(t, after.confirmTypeInput.Value, "tab must not be typed into the DELETE buffer")
}

// A Longhorn node force delete runs through client.ForceDeleteLonghornNode, not
// kubectl delete, so a chosen cascade would silently do nothing. The control
// must be absent rather than inert-but-visible.
func TestLonghornForceDeleteConfirm_HasNoCascadeRow(t *testing.T) {
	m := basePush80Model()
	m.actionCtx = actionContext{
		context:      "test-ctx",
		namespace:    "longhorn-system",
		name:         "lh-node-1",
		kind:         "Node",
		resourceType: model.ResourceTypeEntry{APIGroup: "longhorn.io", APIVersion: "v1beta2", Resource: "nodes", Kind: "Node", Namespaced: true},
	}
	ret, _ := m.executeActionForceDelete()
	m, ok := ret.(Model)
	require.True(t, ok)
	require.Equal(t, overlayConfirmType, m.overlay)
	require.False(t, m.forceDeleteConfirmShowsPolicy())

	body, _, _, _ := m.renderOverlayContent()
	assert.NotContains(t, stripANSI(body), "Cascade")
	assert.NotContains(t, stripANSI(m.overlayHintBarDialog()), "cascade")
}

// Force Finalize shares overlayConfirmType but is not a cascading delete.
func TestForceFinalizeConfirm_HasNoCascadeRow(t *testing.T) {
	m := basePush80Model()
	m.actionCtx = actionContext{
		context:      "test-ctx",
		namespace:    "default",
		name:         "api",
		resourceType: model.ResourceTypeEntry{APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Kind: "Deployment", Namespaced: true},
	}
	ret, _ := m.executeActionForceFinalize()
	m, ok := ret.(Model)
	require.True(t, ok)

	body, _, _, _ := m.renderOverlayContent()
	assert.NotContains(t, stripANSI(body), "Cascade")
	assert.NotContains(t, stripANSI(m.overlayHintBarDialog()), "cascade")
}

func TestFitChoiceRow(t *testing.T) {
	tests := []struct {
		name      string
		inner     int
		wantLabel string
		wantValue string
	}{
		{"everything fits", 40, "Cascade", "Orphan (dependents kept)"},
		{"note dropped first", 20, "Cascade", "Orphan"},
		{"label dropped next", 12, "", "Orphan"},
		{"value truncated last", 4, "", "Orp~"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label, value := fitChoiceRow("Cascade", "Orphan", " (dependents kept)", tt.inner)
			assert.Equal(t, tt.wantLabel, label)
			assert.Equal(t, tt.wantValue, value)

			rendered := value
			if label != "" {
				rendered = label + ": " + value
			}
			assert.LessOrEqual(t, lipgloss.Width(rendered), tt.inner)
		})
	}
}

// A zero or negative width must not panic or produce a negative-length slice.
func TestFitChoiceRow_DegenerateWidth(t *testing.T) {
	for _, inner := range []int{0, -5} {
		label, value := fitChoiceRow("Cascade", "Orphan", "", inner)
		assert.Empty(t, label)
		assert.LessOrEqual(t, lipgloss.Width(value), max(inner, 0))
	}
}

// The cascade row is not wrapped, so the note must be dropped rather
// than overflow the box on a narrow terminal.
func TestDeleteConfirm_PolicyNoteFitsNarrowWidth(t *testing.T) {
	for _, policy := range []model.DeletePropagation{
		model.DeletePropagationBackground,
		model.DeletePropagationForeground,
		model.DeletePropagationOrphan,
		model.DeletePropagationNone,
	} {
		m := deleteConfirmModel(t)
		m.confirmPropagation = policy

		for _, width := range []int{30, 40, 50, 120} {
			m.width = width
			body, w, _, _ := m.renderOverlayContent()
			for line := range strings.SplitSeq(stripANSI(body), "\n") {
				assert.LessOrEqual(t, lipgloss.Width(line), w-4,
					"%s at width %d: line %q exceeds the box inner width", policy, width, line)
			}
			assert.Contains(t, stripANSI(body), policy.Label(),
				"%s: the policy itself must always show", policy)
		}
	}
}

// Non-delete confirms share overlayConfirm; they must not advertise or accept
// a cascade policy they never send.
func TestNonDeleteConfirm_HasNoCascadeRow(t *testing.T) {
	m := basePush80Model()
	m.actionCtx = actionContext{
		context:      "test-ctx",
		name:         "node-1",
		resourceType: model.ResourceTypeEntry{APIGroup: "longhorn.io", APIVersion: "v1beta2", Resource: "nodes", Kind: "Node"},
	}
	ret, _ := m.executeActionEvictReplicas()
	m, ok := ret.(Model)
	require.True(t, ok)
	require.Equal(t, overlayConfirm, m.overlay)

	body, _, _, _ := m.renderOverlayContent()
	assert.NotContains(t, stripANSI(body), "Cascade")
	assert.NotContains(t, strings.ToLower(stripANSI(m.overlayHintBarDialog())), "cascade")

	before := m.confirmPropagation
	ret, _ = m.handleConfirmOverlayKey(keyMsg("tab"))
	after, ok := ret.(Model)
	require.True(t, ok)
	assert.Equal(t, before, after.confirmPropagation, "tab must be inert on a non-delete confirm")
	assert.Equal(t, overlayConfirm, after.overlay)
}

// Helm uninstall does not go through DeleteResource, so the policy is moot.
func TestHelmDeleteConfirm_HasNoCascadeRow(t *testing.T) {
	m := basePush80Model()
	m.actionCtx = actionContext{
		context:      "test-ctx",
		namespace:    "default",
		name:         "my-release",
		resourceType: model.ResourceTypeEntry{APIGroup: "_helm", Resource: "releases", Kind: "Release", Namespaced: true},
	}
	ret, _ := m.executeActionDelete()
	m, ok := ret.(Model)
	require.True(t, ok)

	body, _, _, _ := m.renderOverlayContent()
	assert.NotContains(t, stripANSI(body), "Cascade")
}
