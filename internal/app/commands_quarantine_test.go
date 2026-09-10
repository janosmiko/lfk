package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
)

func quarantineTestActionCtx() actionContext {
	return actionContext{
		kind:      "Pod",
		name:      "pod-1",
		namespace: "default",
		context:   "test-ctx",
		raw: map[string]any{
			"metadata": map[string]any{
				"name":      "pod-1",
				"namespace": "default",
				"labels":    map[string]any{"app": "web"},
			},
		},
	}
}

func TestExecuteActionQuarantine_SchedulesOneFetch(t *testing.T) {
	m := baseModelWithFakeClient(&corev1.Service{
		Name: "svc-web", Namespace: "default",
		Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "web"}},
	})
	m.actionCtx = quarantineTestActionCtx()

	mdl, cmd := m.executeActionQuarantine()
	out := mdl.(Model)

	assertSchedulesOne(t, out, cmd)
	assert.Equal(t, overlayNone, out.overlay, "the overlay opens only after the fetch replies")
}

func TestUpdateQuarantineTargets_NoServices_ReportsAndOpensNoOverlay(t *testing.T) {
	m := baseModelWithFakeClient()
	m.actionCtx = quarantineTestActionCtx()
	m.quarantine.reset()

	mdl, cmd := m.updateQuarantineTargets(quarantineTargetsMsg{req: m.quarantine.req})
	out := mdl.(Model)

	assert.Equal(t, overlayNone, out.overlay)
	assert.Contains(t, out.statusMessage, "Nothing to quarantine")
	assert.Contains(t, out.statusMessage, "pod-1")
	require.NotNil(t, cmd, "status clear is still scheduled")
}

func TestUpdateQuarantineTargets_StaleReqIgnored(t *testing.T) {
	m := baseModelWithFakeClient()
	m.actionCtx = quarantineTestActionCtx()
	m.quarantine.reset()
	stale := m.quarantine.req
	m.quarantine.reset() // dialog closed and reopened: a newer req is now current

	mdl, cmd := m.updateQuarantineTargets(quarantineTargetsMsg{
		req:      stale,
		services: []string{"svc-web"},
		keys:     []string{"app"},
	})
	out := mdl.(Model)

	assert.Equal(t, overlayNone, out.overlay)
	assert.Nil(t, out.quarantine.services)
	assert.Nil(t, cmd)
}
