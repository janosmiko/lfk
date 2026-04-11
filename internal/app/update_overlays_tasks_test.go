package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
)

func TestBgTasksOverlayClosesOnEsc(t *testing.T) {
	t.Parallel()
	m := Model{
		overlay: overlayBackgroundTasks,
		bgtasks: bgtasks.New(0),
	}
	ret, _ := m.handleBackgroundTasksOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
}

func TestBgTasksOverlayClosesOnQ(t *testing.T) {
	t.Parallel()
	m := Model{
		overlay: overlayBackgroundTasks,
		bgtasks: bgtasks.New(0),
	}
	ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('q'))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
}

func TestBgTasksOverlayIgnoresOtherKeys(t *testing.T) {
	t.Parallel()
	m := Model{
		overlay: overlayBackgroundTasks,
		bgtasks: bgtasks.New(0),
	}
	ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, overlayBackgroundTasks, result.overlay,
		"unrelated keys must not close the overlay")
}
