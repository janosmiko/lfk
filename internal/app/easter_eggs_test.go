package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestKonamiSequence(t *testing.T) {
	m := baseModelCov()

	// Send the full Konami Code sequence.
	keys := []tea.KeyMsg{
		{Type: tea.KeyUp},
		{Type: tea.KeyUp},
		{Type: tea.KeyDown},
		{Type: tea.KeyDown},
		{Type: tea.KeyLeft},
		{Type: tea.KeyRight},
		{Type: tea.KeyLeft},
		{Type: tea.KeyRight},
		{Type: tea.KeyRunes, Runes: []rune{'b'}},
		{Type: tea.KeyRunes, Runes: []rune{'a'}},
	}

	for _, key := range keys {
		m = m.checkKonami(key)
	}

	assert.True(t, m.konamiActive, "konamiActive should be true after full sequence")
	assert.Equal(t, 0, m.konamiProgress, "konamiProgress should reset to 0 after activation")
	assert.Contains(t, m.statusMessage, "Cheat code activated. +30 extra life!")
}

func TestKonamiResetOnWrongKey(t *testing.T) {
	m := baseModelCov()

	// Send partial sequence then a wrong key.
	m = m.checkKonami(tea.KeyMsg{Type: tea.KeyUp})
	m = m.checkKonami(tea.KeyMsg{Type: tea.KeyUp})
	m = m.checkKonami(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 3, m.konamiProgress)

	// Wrong key resets progress.
	m = m.checkKonami(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	assert.Equal(t, 0, m.konamiProgress)
	assert.False(t, m.konamiActive)
}

func TestKonamiResetOnWrongKeyRestartsIfMatchesFirst(t *testing.T) {
	m := baseModelCov()

	// Send partial sequence, then "up" which is wrong at position 2 but matches position 0.
	m = m.checkKonami(tea.KeyMsg{Type: tea.KeyUp})
	m = m.checkKonami(tea.KeyMsg{Type: tea.KeyUp})
	m = m.checkKonami(tea.KeyMsg{Type: tea.KeyDown})
	// Now expecting "down" at position 3, but send "up" instead.
	m = m.checkKonami(tea.KeyMsg{Type: tea.KeyUp})
	// Should restart at 1 because "up" matches konamiSequence[0].
	assert.Equal(t, 1, m.konamiProgress)
}

func TestKonamiClear(t *testing.T) {
	m := baseModelCov()
	m.konamiActive = true

	m = m.clearKonami()
	assert.False(t, m.konamiActive, "clearKonami should reset konamiActive to false")
}

func TestNyanToggle(t *testing.T) {
	m := baseModelCov()
	assert.False(t, m.nyanMode)

	// Toggle on.
	m, _ = m.toggleNyan()
	assert.True(t, m.nyanMode)
	assert.Contains(t, m.statusMessage, "Nyan mode activated")

	// Toggle off.
	m, _ = m.toggleNyan()
	assert.False(t, m.nyanMode)
	assert.Contains(t, m.statusMessage, "Nyan mode deactivated")
}

func TestExplainLifeContent(t *testing.T) {
	content := explainLifeContent()
	assert.Contains(t, content, "KIND:     Life")
	assert.Contains(t, content, "spec.coffee")
	assert.Contains(t, content, "CrashLoopBackOff")
	assert.Contains(t, content, "spec.sleep")
	assert.Contains(t, content, "status.phase")
	assert.Contains(t, content, "metadata.labels")
	assert.Contains(t, content, "It's not a bug, it's a feature")
}

func TestCreditsContent(t *testing.T) {
	lines := creditsLines("v1.2.3")
	joined := strings.Join(lines, "\n")

	assert.Contains(t, joined, "LFK")
	assert.Contains(t, joined, "v1.2.3")
	assert.Contains(t, joined, "Janos Miko")
	assert.Contains(t, joined, "BubbleTea")
	assert.Contains(t, joined, "github.com/janosmiko/lfk")
}

func TestCreditsScrollTick(t *testing.T) {
	m := baseModelCov()
	m.creditsScroll = 40 // start well above center
	m.height = 40

	m, stopped := m.tickCredits()
	assert.False(t, stopped)
	assert.Equal(t, 39, m.creditsScroll, "tickCredits should decrement creditsScroll by 1")

	m, stopped = m.tickCredits()
	assert.False(t, stopped)
	assert.Equal(t, 38, m.creditsScroll)
}

func TestCreditsStopsAtCenter(t *testing.T) {
	m := baseModelCov()
	m.height = 40
	lines := creditsLines(m.version)
	centerStop := (m.height - len(lines)) / 2
	m.creditsScroll = centerStop + 1

	m, stopped := m.tickCredits()
	assert.False(t, stopped)

	m, stopped = m.tickCredits()
	assert.True(t, stopped)
	assert.True(t, m.creditsStopped)
}
