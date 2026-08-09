package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func hpaActionCtxModel() Model {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "web", "default", "HorizontalPodAutoscaler", model.ResourceTypeEntry{
		APIGroup: "autoscaling", APIVersion: "v2", Resource: "horizontalpodautoscalers", Namespaced: true,
	})
	m.actionCtx.columns = []model.KeyValue{
		{Key: "Target", Value: "Deployment/web"},
		{Key: "Min Replicas", Value: "2"},
		{Key: "Max Replicas", Value: "5"},
		{Key: "Current Replicas", Value: "3"},
		{Key: "Desired Replicas", Value: "4"},
	}
	return m
}

func TestExecuteActionScale_HPAOpensHPAOverlay(t *testing.T) {
	m := hpaActionCtxModel()
	m, _ = m.executeActionScale()

	assert.Equal(t, overlayHPAScale, m.overlay)
	assert.Equal(t, "2", m.hpaScale.min.Value)
	assert.Equal(t, "5", m.hpaScale.max.Value)
	assert.Equal(t, "4", m.hpaScale.target.Value) // desired preferred over current
	assert.Equal(t, "Deployment", m.hpaScale.targetKind)
	assert.Equal(t, "web", m.hpaScale.targetName)
	assert.Equal(t, "2", m.hpaScale.origMin)
	assert.Equal(t, "5", m.hpaScale.origMax)
	assert.Equal(t, "4", m.hpaScale.origTarget)
}

func TestExecuteActionScale_DeploymentUsesPlainOverlay(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "web", "default", "Deployment", model.ResourceTypeEntry{})
	m, _ = m.executeActionScale()
	assert.Equal(t, overlayScaleInput, m.overlay)
}

func TestHPAScaleOverlay_FieldNavigation(t *testing.T) {
	m := hpaActionCtxModel().openHPAScaleOverlay()

	mdl, _ := m.handleHPAScaleOverlayKey(keyMsg("down")) // j/down = next field
	m = mdl.(Model)
	assert.Equal(t, 1, m.hpaScale.field)

	mdl, _ = m.handleHPAScaleOverlayKey(keyMsg("j"))
	m = mdl.(Model)
	assert.Equal(t, 2, m.hpaScale.field)

	mdl, _ = m.handleHPAScaleOverlayKey(keyMsg("down"))
	m = mdl.(Model)
	assert.Equal(t, 0, m.hpaScale.field) // wraps

	mdl, _ = m.handleHPAScaleOverlayKey(keyMsg("k")) // k/up = prev field
	m = mdl.(Model)
	assert.Equal(t, 2, m.hpaScale.field) // wraps backwards
}

func TestHPAScaleOverlay_DigitsEditActiveField(t *testing.T) {
	m := hpaActionCtxModel().openHPAScaleOverlay()
	// Move to max field (j = next), clear it, type 9.
	mdl, _ := m.handleHPAScaleOverlayKey(keyMsg("j"))
	m = mdl.(Model)
	mdl, _ = m.handleHPAScaleOverlayKey(keyMsg("backspace"))
	m = mdl.(Model)
	mdl, _ = m.handleHPAScaleOverlayKey(keyMsg("9"))
	m = mdl.(Model)
	assert.Equal(t, "9", m.hpaScale.max.Value)
	assert.Equal(t, "2", m.hpaScale.min.Value) // min untouched
}

func TestHPAScaleOverlay_StepKeys(t *testing.T) {
	m := hpaActionCtxModel().openHPAScaleOverlay() // min=2, max=5, target=4

	// l/right/+ increment the active (min) field.
	mdl, _ := m.handleHPAScaleOverlayKey(keyMsg("l"))
	m = mdl.(Model)
	assert.Equal(t, "3", m.hpaScale.min.Value)

	// h/left/- decrement.
	mdl, _ = m.handleHPAScaleOverlayKey(keyMsg("h"))
	m = mdl.(Model)
	assert.Equal(t, "2", m.hpaScale.min.Value)

	// min floors at 1.
	mdl, _ = m.handleHPAScaleOverlayKey(keyMsg("-"))
	m = mdl.(Model)
	mdl, _ = m.handleHPAScaleOverlayKey(keyMsg("-"))
	m = mdl.(Model)
	assert.Equal(t, "1", m.hpaScale.min.Value)
}

func TestHPAScaleOverlay_TargetFloorsAtZero(t *testing.T) {
	m := hpaActionCtxModel().openHPAScaleOverlay()
	m.hpaScale.field = 2 // target
	m.hpaScale.target.Set("1")
	mdl, _ := m.handleHPAScaleOverlayKey(keyMsg("-"))
	m = mdl.(Model)
	mdl, _ = m.handleHPAScaleOverlayKey(keyMsg("-"))
	m = mdl.(Model)
	assert.Equal(t, "0", m.hpaScale.target.Value)
}

func TestOpenHPAScaleOverlay_PrefillsFromRaw(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "api", "default", "HorizontalPodAutoscaler", model.ResourceTypeEntry{
		APIGroup: "autoscaling", APIVersion: "v2", Resource: "horizontalpodautoscalers", Namespaced: true,
	})
	m.actionCtx.raw = map[string]any{
		"spec": map[string]any{
			"minReplicas":    int64(3),
			"maxReplicas":    int64(9),
			"scaleTargetRef": map[string]any{"kind": "Deployment", "name": "api"},
		},
		"status": map[string]any{"desiredReplicas": int64(6)},
	}
	m = m.openHPAScaleOverlay()
	assert.Equal(t, "3", m.hpaScale.min.Value)
	assert.Equal(t, "9", m.hpaScale.max.Value)
	assert.Equal(t, "6", m.hpaScale.target.Value)
	assert.Equal(t, "Deployment", m.hpaScale.targetKind)
	assert.Equal(t, "api", m.hpaScale.targetName)
}

func TestExecuteActionScale_WorkloadPrefillsReplicas(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "web", "default", "Deployment", model.ResourceTypeEntry{})
	m.actionCtx.raw = map[string]any{"spec": map[string]any{"replicas": int64(4)}}
	m, _ = m.executeActionScale()
	assert.Equal(t, overlayScaleInput, m.overlay)
	assert.Equal(t, "4", m.scaleInput.Value)
}

func TestHPAScaleOverlay_EscCancels(t *testing.T) {
	m := hpaActionCtxModel().openHPAScaleOverlay()
	mdl, _ := m.handleHPAScaleOverlayKey(keyMsg("esc"))
	m = mdl.(Model)
	assert.Equal(t, overlayNone, m.overlay)
	assert.Empty(t, m.hpaScale.min.Value)
}

func TestHPAScaleOverlay_NoChangeClosesWithoutCmd(t *testing.T) {
	m := hpaActionCtxModel().openHPAScaleOverlay()
	mdl, cmd := m.handleHPAScaleOverlayKey(keyMsg("enter"))
	m = mdl.(Model)
	assert.Equal(t, overlayNone, m.overlay)
	require.NotNil(t, cmd) // scheduleStatusClear
	assert.False(t, m.loading)
}

func TestHPAScaleOverlay_MaxLessThanMinRejected(t *testing.T) {
	m := hpaActionCtxModel().openHPAScaleOverlay()
	m.hpaScale.min.Set("5")
	m.hpaScale.max.Set("2")
	mdl, _ := m.handleHPAScaleOverlayKey(keyMsg("enter"))
	m = mdl.(Model)
	assert.Equal(t, overlayNone, m.overlay)
	assert.False(t, m.loading) // not applied
}

func TestHPAScaleOverlay_ApplyMinMaxDispatches(t *testing.T) {
	m := hpaActionCtxModel().openHPAScaleOverlay()
	m.hpaScale.min.Set("3")
	m.hpaScale.max.Set("10")
	mdl, cmd := m.handleHPAScaleOverlayKey(keyMsg("enter"))
	m = mdl.(Model)
	assert.Equal(t, overlayNone, m.overlay)
	assert.True(t, m.loading)
	require.NotNil(t, cmd)
}

func TestParseHPAScaleInputs(t *testing.T) {
	tests := []struct {
		name        string
		min, max    string
		target      string
		scaleMinMax bool
		scaleTarget bool
		wantErr     bool
		wantMin     int32
		wantMax     int32
		wantTarget  int32
	}{
		{name: "valid min/max only", min: "2", max: "5", scaleMinMax: true, wantMin: 2, wantMax: 5},
		{name: "valid with target", min: "1", max: "8", target: "4", scaleMinMax: true, scaleTarget: true, wantMin: 1, wantMax: 8, wantTarget: 4},
		{name: "target only ignores unset min/max", min: "", max: "", target: "3", scaleTarget: true, wantTarget: 3},
		{name: "min zero rejected", min: "0", max: "5", scaleMinMax: true, wantErr: true},
		{name: "max below min rejected", min: "5", max: "2", scaleMinMax: true, wantErr: true},
		{name: "non-numeric rejected", min: "x", max: "5", scaleMinMax: true, wantErr: true},
		{name: "negative target rejected", min: "1", max: "5", target: "-1", scaleMinMax: true, scaleTarget: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := hpaScaleState{}
			st.min.Set(tt.min)
			st.max.Set(tt.max)
			st.target.Set(tt.target)
			minR, maxR, targetR, err := parseHPAScaleInputs(st, tt.scaleMinMax, tt.scaleTarget)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantMin, minR)
			assert.Equal(t, tt.wantMax, maxR)
			assert.Equal(t, tt.wantTarget, targetR)
		})
	}
}

func TestHPAScaleOverlay_Renders(t *testing.T) {
	m := hpaActionCtxModel().openHPAScaleOverlay()
	content, _, _, ok := m.renderOverlayContent()
	require.True(t, ok)
	plain := stripANSI(content)
	assert.Contains(t, plain, "Scale HPA")
	assert.Contains(t, plain, "Min replicas")
	assert.Contains(t, plain, "Max replicas")
	assert.Contains(t, plain, "Target replicas")
}

func TestActionsForHPA_IncludesScale(t *testing.T) {
	actions := model.ActionsForKind("HorizontalPodAutoscaler")
	var found bool
	for _, a := range actions {
		if a.Label == "Scale" {
			found = true
			assert.Equal(t, "S", a.Key)
		}
	}
	assert.True(t, found, "HPA action menu must include Scale")
}
