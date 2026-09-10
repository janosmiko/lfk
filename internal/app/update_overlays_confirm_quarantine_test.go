package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tea "charm.land/bubbletea/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/janosmiko/lfk/internal/model"
)

func TestConfirmEnter_Quarantine_DispatchesPatch(t *testing.T) {
	m := baseModelWithFakeClient()
	m.overlay = overlayConfirm
	m.pendingAction = model.ActionLabelQuarantine
	m.actionCtx = actionContext{
		context:      "test-ctx",
		kind:         "Pod",
		name:         "pod-1",
		namespace:    "default",
		resourceType: model.ResourceTypeEntry{Resource: "pods", Namespaced: true},
	}
	m.quarantine.services = []string{"svc-web"}
	m.quarantine.keys = []string{"app"}

	mdl, cmd := m.handleConfirmOverlayKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	out := mdl.(Model)

	assertSchedulesOne(t, out, cmd)
	assert.Equal(t, overlayNone, out.overlay)
	assert.Empty(t, out.pendingAction)
	require.NotEmpty(t, out.errorLog)
	assert.Contains(t, out.errorLog[len(out.errorLog)-1].Message, "kubectl patch pod pod-1",
		"an unrecognized pendingAction must not silently fall through to the regular-delete branch")
}

func TestConfirmEnter_Restore_DispatchesPatch(t *testing.T) {
	m := baseModelWithFakeClient()
	m.overlay = overlayConfirm
	m.pendingAction = model.ActionLabelRestore
	m.actionCtx = actionContext{
		context:      "test-ctx",
		kind:         "Pod",
		name:         "pod-1",
		namespace:    "default",
		resourceType: model.ResourceTypeEntry{Resource: "pods", Namespaced: true},
	}
	m.quarantine.restore = map[string]string{"app": "web"}

	mdl, cmd := m.handleConfirmOverlayKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	out := mdl.(Model)

	assertSchedulesOne(t, out, cmd)
	assert.Equal(t, overlayNone, out.overlay)
	assert.Empty(t, out.pendingAction)
	require.NotEmpty(t, out.errorLog)
	assert.Contains(t, out.errorLog[len(out.errorLog)-1].Message, "kubectl patch pod pod-1",
		"an unrecognized pendingAction must not silently fall through to the regular-delete branch")
}

func quarantineFakePod() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":      "pod-1",
			"namespace": "default",
			"labels":    map[string]any{"app": "web"},
		},
	}}
}

func TestQuarantinePodCmd_EndToEnd(t *testing.T) {
	m := baseModelWithFakeDynamic(map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}: "PodList",
	}, quarantineFakePod())
	m.actionCtx = actionContext{context: "test-ctx", kind: "Pod", name: "pod-1", namespace: "default"}

	cmd := m.quarantinePodCmd([]string{"app"})

	msg, ok := execScheduled(t, m, cmd).(actionResultMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	assert.Contains(t, msg.message, "Quarantined pod-1")
}

func TestRestorePodCmd_EndToEnd(t *testing.T) {
	pod := quarantineFakePod()
	pod.Object["metadata"].(map[string]any)["annotations"] = map[string]any{
		"lfk.janosmiko.dev/quarantined-labels": `{"app":"web"}`,
	}
	m := baseModelWithFakeDynamic(map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}: "PodList",
	}, pod)
	m.actionCtx = actionContext{context: "test-ctx", kind: "Pod", name: "pod-1", namespace: "default"}

	cmd := m.restorePodCmd()

	msg, ok := execScheduled(t, m, cmd).(actionResultMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	assert.Contains(t, msg.message, "Restored pod-1")
}

func TestConfirmEsc_Quarantine_ResetsState(t *testing.T) {
	m := Model{overlay: overlayConfirm, pendingAction: model.ActionLabelQuarantine}
	m.quarantine.services = []string{"svc-web"}
	m.quarantine.keys = []string{"app"}

	mdl, cmd := m.handleConfirmOverlayKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	out := mdl.(Model)

	assert.Equal(t, overlayNone, out.overlay)
	assert.Nil(t, out.quarantine.services)
	assert.Nil(t, out.quarantine.keys)
	require.Nil(t, cmd)
}
