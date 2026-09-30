package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func pvItem(phase string, csi bool) model.Item {
	spec := map[string]any{"persistentVolumeReclaimPolicy": "Retain"}
	if csi {
		spec["csi"] = map[string]any{"driver": "csi.example.com", "volumeHandle": "vol-123"}
	}
	return model.Item{
		Name: "pv-1", Kind: "PersistentVolume",
		Raw: map[string]any{"spec": spec, "status": map[string]any{"phase": phase}},
	}
}

func pvMenuModel(item model.Item) Model {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "PersistentVolume", Resource: "persistentvolumes"}
	m.middleItems = []model.Item{item}
	m.setCursor(0)
	return m
}

func menuHas(m Model, label string) bool {
	for _, it := range m.overlayItems {
		if it.Name == label {
			return true
		}
	}
	return false
}

func TestPVDeleteWithVolume_MenuVisibility(t *testing.T) {
	tests := []struct {
		name  string
		phase string
		csi   bool
		want  bool
	}{
		{"released csi", "Released", true, true},
		{"released non-csi", "Released", false, false},
		{"bound csi", "Bound", true, false},
		{"available csi", "Available", true, false},
		{"pending csi", "Pending", true, false},
		{"failed csi", "Failed", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pvMenuModel(pvItem(tt.phase, tt.csi)).openActionMenu()
			assert.Equal(t, tt.want, menuHas(m, actionDeleteWithVolume))
		})
	}
}

func TestPVDeleteWithVolume_ReadOnlyHidesIt(t *testing.T) {
	m := pvMenuModel(pvItem("Released", true))
	m.readOnly = true
	assert.False(t, menuHas(m.openActionMenu(), actionDeleteWithVolume))
}

func TestPVDeleteWithVolume_KeyDoesNotCollide(t *testing.T) {
	m := pvMenuModel(pvItem("Released", true)).openActionMenu()
	seen := map[string]string{}
	for _, it := range m.overlayItems {
		if prev, dup := seen[it.Status]; dup && it.Status != "" {
			t.Fatalf("key %q used by %q and %q", it.Status, prev, it.Name)
		}
		seen[it.Status] = it.Name
	}
}

func TestPVDeleteWithVolume_OpensTypeConfirm(t *testing.T) {
	m := pvMenuModel(pvItem("Released", true)).openActionMenu()
	res, _ := m.executeAction(actionDeleteWithVolume)
	rm := res.(Model)
	require.Equal(t, overlayConfirmType, rm.overlay)
	assert.Equal(t, actionDeleteWithVolume, rm.pendingAction)
	assert.Contains(t, rm.confirmQuestion, "permanently")
	assert.Contains(t, rm.confirmQuestion, "csi.example.com")
	assert.Contains(t, rm.confirmQuestion, "vol-123")
	assert.Contains(t, stripANSI(rm.View().Content), "csi.example.com")
}

func TestPVDeleteWithVolume_IsMutatingAndUnionBlocked(t *testing.T) {
	assert.True(t, isMutatingAction(actionDeleteWithVolume))
	assert.False(t, isUnionAllowedActionForKind("PersistentVolume", actionDeleteWithVolume))
}

func TestPVDeleteWithVolume_PermissionQuery(t *testing.T) {
	q, ok := actionQueries["PersistentVolume"][actionDeleteWithVolume]
	require.True(t, ok)
	assert.Equal(t, "persistentvolumes", q.Resource)
	assert.Equal(t, "patch", q.Verb)
	assert.Empty(t, q.Group)
}

func TestPVReclaimPatchArgs(t *testing.T) {
	rt := model.ResourceTypeEntry{Kind: "PersistentVolume", Resource: "persistentvolumes"}
	args := pvReclaimPatchArgs(rt, "pv-1", "kctx")
	assert.Equal(t, []string{
		"patch", "persistentvolumes", "pv-1", "--context", "kctx",
		"--type", "merge", "-p", `{"spec":{"persistentVolumeReclaimPolicy":"Delete"}}`,
	}, args)
	assert.NotContains(t, args, "delete")
}

func TestPVDeleteWithVolume_ConfirmDispatchesPatch(t *testing.T) {
	m := pvMenuModel(pvItem("Released", true)).openActionMenu()
	res, _ := m.executeAction(actionDeleteWithVolume)
	rm := res.(Model)
	rm.confirmTypeInput.Insert("DELETE")
	res, cmd := rm.handleConfirmTypeOverlayKey(keyMsg("enter"))
	out := res.(Model)
	require.NotNil(t, cmd)
	assert.Empty(t, out.pendingAction)
	require.NotEmpty(t, out.errorLog)
	last := out.errorLog[len(out.errorLog)-1].Message
	assert.Contains(t, last, "kubectl patch persistentvolumes pv-1 --context")
	assert.Contains(t, last, "--type merge")
	assert.Contains(t, last, pvReclaimDeletePatch)
	assert.NotContains(t, last, "kubectl delete")
}

func TestPVDeleteWithVolume_SanitizesQuestion(t *testing.T) {
	item := pvItem("Released", true)
	item.Raw["spec"].(map[string]any)["csi"].(map[string]any)["volumeHandle"] = "vol\x1b[31m-evil"
	m := pvMenuModel(item).openActionMenu()
	res, _ := m.executeAction(actionDeleteWithVolume)
	assert.NotContains(t, res.(Model).confirmQuestion, "\x1b")
}

func TestReleasedCSIVolume_MissingData(t *testing.T) {
	_, _, ok := releasedCSIVolume(nil)
	assert.False(t, ok)
	_, _, ok = releasedCSIVolume(map[string]any{"spec": map[string]any{"csi": map[string]any{"driver": "d"}}})
	assert.False(t, ok, "missing status")
	m := pvMenuModel(model.Item{Name: "pv-1", Kind: "PersistentVolume"}).openActionMenu()
	assert.False(t, menuHas(m, actionDeleteWithVolume))
}
