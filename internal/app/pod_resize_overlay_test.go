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

// Names from the API object are untrusted terminal text and must not reach
// the screen with their escape sequences intact.
func TestPodResizeOverlay_SanitizesNamesFromTheObject(t *testing.T) {
	m := podResizeOverlayModel()
	m.podResize.name = "pod\x1b]0;evil\x07"
	m.podResize.containers[0].name = "web\x1b[31m"
	m.podResize.restartWarn = []string{"web\x1b[31m"}

	view := m.View().Content
	assert.NotContains(t, view, "\x1b]0;evil")
	assert.NotContains(t, view, "web\x1b[31m")
}

func TestPodResizeOverlay_CapsFieldLength(t *testing.T) {
	m := podResizeOverlayModel()
	for range podResizeMaxLen + 5 {
		ret, _ := m.handlePodResizeOverlayKey(runeKey('1'))
		m = ret.(Model)
	}
	assert.LessOrEqual(t, len(m.podResize.active().Value), podResizeMaxLen)
}

func TestPodResizeOverlay_AcceptsKiSuffix(t *testing.T) {
	m := podResizeOverlayModel()
	m.podResize.active().Clear()

	for _, r := range "1Ki" {
		ret, _ := m.handlePodResizeOverlayKey(runeKey(r))
		m = ret.(Model)
	}

	assert.Equal(t, "1Ki", m.podResize.active().Value)
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
