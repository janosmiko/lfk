package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func podResizeOverlayModel() Model {
	m := basePush80Model()
	m.podResize = buildPodResizeState(podResizeRawFixture())
	m.overlay = overlayPodResize
	return m
}

func TestPodResizeOverlay_RendersPrefilledRows(t *testing.T) {
	m := podResizeOverlayModel()

	view := stripANSI(m.View().Content)
	assert.Contains(t, view, "web")
	assert.Contains(t, view, "200m")
}

func TestPodResizeOverlay_ShowsRestartWarning(t *testing.T) {
	m := podResizeOverlayModel()
	require.Equal(t, []string{"web"}, m.podResize.restartWarn)

	view := stripANSI(m.View().Content)
	assert.Contains(t, view, "restart")
}

func TestPodResizeOverlay_HintBarListsApply(t *testing.T) {
	m := podResizeOverlayModel()

	hints := stripANSI(m.overlayHintBar())
	assert.Contains(t, hints, "apply")
	assert.Contains(t, hints, "cancel")
}

func TestPodResizeOverlay_EscClearsState(t *testing.T) {
	m := podResizeOverlayModel()
	require.NotEmpty(t, m.podResize.containers)

	mdl, _ := m.handlePodResizeOverlayKey(keyMsg("esc"))
	m2, ok := mdl.(Model)
	require.True(t, ok)
	assert.Equal(t, overlayNone, m2.overlay)
	assert.Empty(t, m2.podResize.containers)
}
