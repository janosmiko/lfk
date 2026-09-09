package app

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func TestExecuteActionResize_PodOpensPodResizeOverlay(t *testing.T) {
	m := withActionCtx(baseModelWithFakeClient(), "my-pod", "default", "Pod", model.ResourceTypeEntry{})
	m.actionCtx.raw = podResizeRawFixture()

	mdl, _ := m.executeActionResize()
	m2, ok := mdl.(Model)
	require.True(t, ok)

	assert.Equal(t, overlayPodResize, m2.overlay)
	require.Len(t, m2.podResize.containers, 2)
	assert.Equal(t, "web", m2.podResize.containers[0].name)
}

func TestExecuteActionResize_PVCStillOpensOverlayPVCResize(t *testing.T) {
	m := withActionCtx(baseModelWithFakeClient(), "my-pvc", "default", "PersistentVolumeClaim", model.ResourceTypeEntry{})
	m.actionCtx.columns = []model.KeyValue{{Key: "Capacity", Value: "10Gi"}}

	mdl, _ := m.executeActionResize()
	m2, ok := mdl.(Model)
	require.True(t, ok)

	assert.Equal(t, overlayPVCResize, m2.overlay)
	assert.Equal(t, "10Gi", m2.pvcCurrentSize)
}

func podResizeSubmitModel() Model {
	m := withActionCtx(baseModelWithFakeClient(), "my-pod", "default", "Pod", model.ResourceTypeEntry{})
	m.actionCtx.raw = podResizeRawFixture()
	mdl, _ := m.executeActionResize()
	m2 := mdl.(Model)
	m2.podResize.containers[0].cpuReq.Set("500m")
	return m2
}

func TestPodResizeSubmit_BlockedByReadOnly(t *testing.T) {
	m := podResizeSubmitModel()
	m.cliReadOnly = true

	mdl, _ := m.handlePodResizeOverlayKey(keyMsg("enter"))
	m2, ok := mdl.(Model)
	require.True(t, ok)

	assert.Equal(t, overlayNone, m2.overlay)
	assert.True(t, m2.statusMessageErr)
	assert.Contains(t, m2.statusMessage, "Read-only")
}

func TestPodResizeSubmit_SchedulesOneMutation(t *testing.T) {
	m := podResizeSubmitModel()

	mdl, cmd := m.handlePodResizeOverlayKey(keyMsg("enter"))
	m2, ok := mdl.(Model)
	require.True(t, ok)

	assertSchedulesOne(t, m2, cmd)
	assert.Equal(t, overlayNone, m2.overlay)
}

func TestPodResizeSubmit_SurfacesAPIError(t *testing.T) {
	m := baseModelWithFakeClient()

	mdl, _ := m.updateActionResult(actionResultMsg{err: errors.New("Pod QoS is immutable")})
	m2, ok := mdl.(Model)
	require.True(t, ok)

	assert.True(t, m2.statusMessageErr)
	assert.Contains(t, m2.statusMessage, "Pod QoS is immutable")
}
