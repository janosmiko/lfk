package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tea "charm.land/bubbletea/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	fake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/k8s"
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

// quarantineModelWithService wires a Model whose dynamic client serves
// quarantineFakePod and whose clientset serves the given Services, so the
// pre-patch QuarantineTargets re-check has real data to compare against.
func quarantineModelWithService(services ...runtime.Object) Model {
	cs := fake.NewClientset(services...)
	scheme := newFakeScheme()
	dyn := dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}: "PodList",
	}, quarantineFakePod())

	m := baseModelCov()
	m.client = k8s.NewTestClient(cs, dyn)
	m.nav.Context = "test-ctx"
	m.namespace = "default"
	m.reqCtx = context.Background()
	m.actionCtx = actionContext{
		context: "test-ctx", kind: "Pod", name: "pod-1", namespace: "default",
		raw: quarantineFakePod().Object,
	}
	return m
}

func TestQuarantinePodCmd_EndToEnd(t *testing.T) {
	m := quarantineModelWithService(&corev1.Service{
		Name: "svc-web", Namespace: "default",
		Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "web"}},
	})

	cmd := m.quarantinePodCmd([]string{"app"})

	msg, ok := execScheduled(t, m, cmd).(actionResultMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	assert.Contains(t, msg.message, "Quarantined pod-1")
}

// The confirm box showed keys ["app"], but by commit time the only
// matching Service selects on "tier" instead.
func TestQuarantineCommit_AbortsWhenTargetsChanged(t *testing.T) {
	m := quarantineModelWithService(&corev1.Service{
		Name: "svc-tier", Namespace: "default",
		Spec: corev1.ServiceSpec{Selector: map[string]string{"tier": "backend"}},
	})

	cmd := m.quarantinePodCmd([]string{"app"})

	msg := execScheduled(t, m, cmd)
	_, changed := msg.(quarantineTargetsChangedMsg)
	assert.True(t, changed, "expected quarantineTargetsChangedMsg, got %T", msg)

	mdl, _ := m.updateQuarantineTargetsChanged()
	out := mdl.(Model)
	assert.True(t, out.statusMessageErr)
	assert.Contains(t, out.statusMessage, "run Quarantine again")
	assert.Nil(t, out.quarantine.keys)
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
