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

// TestBgTasksOverlayTabTogglesMode verifies that Tab flips between the
// running-tasks view (default) and the completed-history view without
// closing the overlay, and also resets the scroll offset to 0 so the
// switched view starts from its top.
func TestBgTasksOverlayTabTogglesMode(t *testing.T) {
	t.Parallel()
	m := Model{
		overlay:            overlayBackgroundTasks,
		bgtasks:            bgtasks.New(0),
		tasksOverlayScroll: 7, // user had scrolled down in running view
	}
	// Start: running view (flag is false).
	assert.False(t, m.tasksOverlayShowCompleted)

	ret, _ := m.handleBackgroundTasksOverlayKey(tea.KeyMsg{Type: tea.KeyTab})
	result := ret.(Model)
	assert.Equal(t, overlayBackgroundTasks, result.overlay,
		"Tab must not close the overlay")
	assert.True(t, result.tasksOverlayShowCompleted,
		"first Tab must switch to the completed view")
	assert.Equal(t, 0, result.tasksOverlayScroll,
		"Tab must reset scroll so the new view starts from its top")

	// A second Tab flips back.
	ret2, _ := result.handleBackgroundTasksOverlayKey(tea.KeyMsg{Type: tea.KeyTab})
	result2 := ret2.(Model)
	assert.False(t, result2.tasksOverlayShowCompleted,
		"second Tab must switch back to the running view")
}

// TestBgTasksOverlayScrollKeys verifies the scroll handlers.
func TestBgTasksOverlayScrollKeys(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		key      tea.KeyMsg
		start    int
		wantDiff int // expected change from start; >=0 means down
	}{
		{"j scrolls down", runeKey('j'), 3, +1},
		{"down arrow scrolls down", tea.KeyMsg{Type: tea.KeyDown}, 3, +1},
		{"k scrolls up", runeKey('k'), 3, -1},
		{"up arrow scrolls up", tea.KeyMsg{Type: tea.KeyUp}, 3, -1},
		{"ctrl+d half-page down", tea.KeyMsg{Type: tea.KeyCtrlD}, 3, +5},
		{"ctrl+u half-page up", tea.KeyMsg{Type: tea.KeyCtrlU}, 10, -5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{
				overlay:            overlayBackgroundTasks,
				bgtasks:            bgtasks.New(0),
				tasksOverlayScroll: tc.start,
			}
			ret, _ := m.handleBackgroundTasksOverlayKey(tc.key)
			result := ret.(Model)
			assert.Equal(t, tc.start+tc.wantDiff, result.tasksOverlayScroll)
			// Scrolling must not close the overlay.
			assert.Equal(t, overlayBackgroundTasks, result.overlay)
		})
	}
}

// TestBgTasksOverlayScrollUpClampsAtZero verifies k doesn't go negative.
// The renderer also clamps, so this is belt-and-suspenders, but the
// handler should produce sensible values on its own.
func TestBgTasksOverlayScrollUpClampsAtZero(t *testing.T) {
	t.Parallel()
	m := Model{
		overlay:            overlayBackgroundTasks,
		bgtasks:            bgtasks.New(0),
		tasksOverlayScroll: 0,
	}
	ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('k'))
	result := ret.(Model)
	assert.Equal(t, 0, result.tasksOverlayScroll,
		"k at scroll=0 must not produce a negative offset")
}

// TestBgTasksOverlayJumpKeys verifies g jumps to top and G jumps
// relative to "as far down as possible"; the handler sets a sentinel
// and the renderer clamps it to the true end.
func TestBgTasksOverlayJumpKeys(t *testing.T) {
	t.Parallel()
	t.Run("g jumps to top", func(t *testing.T) {
		m := Model{
			overlay:            overlayBackgroundTasks,
			bgtasks:            bgtasks.New(0),
			tasksOverlayScroll: 20,
		}
		ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('g'))
		result := ret.(Model)
		assert.Equal(t, 0, result.tasksOverlayScroll)
	})
	t.Run("G jumps to bottom (large sentinel)", func(t *testing.T) {
		m := Model{
			overlay:            overlayBackgroundTasks,
			bgtasks:            bgtasks.New(0),
			tasksOverlayScroll: 0,
		}
		ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('G'))
		result := ret.(Model)
		// Handler picks a large sentinel; renderer clamps it at draw time.
		assert.Greater(t, result.tasksOverlayScroll, 1000,
			"G should set scroll to a large value the renderer can clamp")
	})
}

// TestBgTasksOverlayEscResetsMode verifies that closing the overlay
// resets tasksOverlayShowCompleted so the next open starts fresh in
// the running view regardless of where the user left off.
func TestBgTasksOverlayEscResetsMode(t *testing.T) {
	t.Parallel()
	m := Model{
		overlay:                   overlayBackgroundTasks,
		bgtasks:                   bgtasks.New(0),
		tasksOverlayShowCompleted: true,
	}
	ret, _ := m.handleBackgroundTasksOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.False(t, result.tasksOverlayShowCompleted,
		"closing the overlay must reset the Tab toggle so reopen is fresh")
}

// TestTasksOverlayHotkeyOpensFresh verifies that the direct hotkey
// action opens the overlay in running mode, even if the flag happened
// to be left set by some other code path.
func TestTasksOverlayHotkeyOpensFresh(t *testing.T) {
	t.Parallel()
	m := Model{
		bgtasks:                   bgtasks.New(0),
		tasksOverlayShowCompleted: true, // stale
	}
	ret, _, handled := m.handleExplorerActionKeyTasksOverlay()
	require := assert.New(t)
	require.True(handled)
	result := ret.(Model)
	require.Equal(overlayBackgroundTasks, result.overlay)
	require.False(result.tasksOverlayShowCompleted,
		"hotkey must open the overlay fresh in running mode")
}
