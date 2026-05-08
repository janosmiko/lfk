package app

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestBgTasksOverlayClosesOnEsc(t *testing.T) {
	t.Parallel()
	m := Model{
		overlay:   overlayBackgroundTasks,
		scheduler: scheduler.New(0),
	}
	ret, _ := m.handleBackgroundTasksOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
}

func TestBgTasksOverlayClosesOnQ(t *testing.T) {
	t.Parallel()
	m := Model{
		overlay:   overlayBackgroundTasks,
		scheduler: scheduler.New(0),
	}
	ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('q'))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
}

func TestBgTasksOverlayIgnoresOtherKeys(t *testing.T) {
	t.Parallel()
	m := Model{
		overlay:   overlayBackgroundTasks,
		scheduler: scheduler.New(0),
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
		scheduler:          scheduler.New(0),
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

// TestBgTasksOverlayScrollKeys verifies the scroll handlers. The
// scheduler is populated with enough completed tasks (60) so the row
// count exceeds the viewport at the test's terminal dimensions —
// otherwise every scroll-down would clamp to 0 immediately.
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
				overlay:                   overlayBackgroundTasks,
				scheduler:                 newSchedulerWithCompletedHistory(60),
				tasksOverlayShowCompleted: true,
				width:                     120,
				height:                    30,
				tasksOverlayScroll:        tc.start,
			}
			ret, _ := m.handleBackgroundTasksOverlayKey(tc.key)
			result := ret.(Model)
			assert.Equal(t, tc.start+tc.wantDiff, result.tasksOverlayScroll)
			// Scrolling must not close the overlay.
			assert.Equal(t, overlayBackgroundTasks, result.overlay)
		})
	}
}

// newSchedulerWithCompletedHistory returns a Registry whose completed
// history holds n entries with distinct (Kind, Name, Target) tuples
// AND Duration ≥ historyMinDuration so they survive the
// historyTasksForDisplay sub-second filter. Used by scroll-clamp
// tests that need the row count to be deterministic.
func newSchedulerWithCompletedHistory(n int) *scheduler.Registry {
	r := scheduler.New(0)
	base := time.Unix(1_000_000, 0)
	for i := range n {
		started := base.Add(time.Duration(i) * time.Second)
		r.InjectCompletedForTest(scheduler.CompletedTask{
			Task: scheduler.Task{
				Kind:      scheduler.KindResourceList,
				Name:      fmt.Sprintf("Task %d", i),
				Target:    fmt.Sprintf("ctx-%d", i),
				StartedAt: started,
			},
			FinishedAt: started.Add(time.Second),
		})
	}
	return r
}

// TestBgTasksOverlayScrollUpClampsAtZero verifies k doesn't go negative.
// The renderer also clamps, so this is belt-and-suspenders, but the
// handler should produce sensible values on its own.
func TestBgTasksOverlayScrollUpClampsAtZero(t *testing.T) {
	t.Parallel()
	m := Model{
		overlay:            overlayBackgroundTasks,
		scheduler:          scheduler.New(0),
		tasksOverlayScroll: 0,
	}
	ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('k'))
	result := ret.(Model)
	assert.Equal(t, 0, result.tasksOverlayScroll,
		"k at scroll=0 must not produce a negative offset")
}

// TestBgTasksOverlayJumpKeys verifies g jumps to top and G jumps to
// the real end (the handler clamps the sentinel down to
// max(0, total - viewport) immediately so a follow-up k responds on
// the first press).
func TestBgTasksOverlayJumpKeys(t *testing.T) {
	t.Parallel()
	t.Run("g jumps to top", func(t *testing.T) {
		m := Model{
			overlay:            overlayBackgroundTasks,
			scheduler:          scheduler.New(0),
			tasksOverlayScroll: 20,
		}
		ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('g'))
		result := ret.(Model)
		assert.Equal(t, 0, result.tasksOverlayScroll)
	})
	t.Run("G jumps to real end with content", func(t *testing.T) {
		const rows = 60
		m := Model{
			overlay:                   overlayBackgroundTasks,
			scheduler:                 newSchedulerWithCompletedHistory(rows),
			tasksOverlayShowCompleted: true,
			width:                     120,
			height:                    30,
			tasksOverlayScroll:        0,
		}
		ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('G'))
		result := ret.(Model)
		// Handler clamps to total - visible. With 60 rows and the
		// configured viewport, scroll lands well above 0 but well
		// below the 1M sentinel — concretely, == rows - viewport.
		assert.Greater(t, result.tasksOverlayScroll, 0,
			"G must scroll past the top when content exceeds the viewport")
		assert.Less(t, result.tasksOverlayScroll, rows,
			"G must clamp to (total - viewport), not the raw sentinel")
	})
	t.Run("G is a no-op when content fits", func(t *testing.T) {
		m := Model{
			overlay:                   overlayBackgroundTasks,
			scheduler:                 scheduler.New(0),
			tasksOverlayShowCompleted: true,
			width:                     120,
			height:                    30,
		}
		ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('G'))
		result := ret.(Model)
		assert.Equal(t, 0, result.tasksOverlayScroll,
			"G must not scroll past 0 when there are no rows")
	})
}

// TestBgTasksOverlayScrollClampsAtEnd is the regression guard for the
// user-reported "scroll-down past the end leaves a stale value the
// user has to undo with many up presses" bug.
func TestBgTasksOverlayScrollClampsAtEnd(t *testing.T) {
	t.Parallel()
	const rows = 30
	m := Model{
		overlay:                   overlayBackgroundTasks,
		scheduler:                 newSchedulerWithCompletedHistory(rows),
		tasksOverlayShowCompleted: true,
		width:                     120,
		height:                    30,
	}
	// Spam scroll-down 1000 times — far past the real end.
	model := tea.Model(m)
	for range 1000 {
		ret, _ := model.(Model).handleBackgroundTasksOverlayKey(runeKey('j'))
		model = ret
	}
	scroll := model.(Model).tasksOverlayScroll
	assert.Greater(t, scroll, 0)
	assert.Less(t, scroll, rows,
		"scroll-down must clamp at (total - viewport); over-scroll must not strand the model state")

	// One press of k must move the viewport on the FIRST press —
	// without the clamp this fails because the model has drifted past
	// the real end and k just decrements a phantom number.
	ret, _ := model.(Model).handleBackgroundTasksOverlayKey(runeKey('k'))
	after := ret.(Model).tasksOverlayScroll
	assert.Equal(t, scroll-1, after,
		"a single k press must move the viewport up by exactly 1 row")
}

// TestBgTasksOverlayEscResetsMode verifies that closing the overlay
// resets tasksOverlayShowCompleted so the next open starts fresh in
// the running view regardless of where the user left off.
func TestBgTasksOverlayEscResetsMode(t *testing.T) {
	t.Parallel()
	m := Model{
		overlay:                   overlayBackgroundTasks,
		scheduler:                 scheduler.New(0),
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
		scheduler:                 scheduler.New(0),
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

// TestBgTasksOverlayToggleShowAll covers the `a` key behaviour:
// pressing `a` while in completed mode toggles tasksOverlayShowAll
// and resets the scroll; pressing it in running mode is a no-op.
// esc must reset the toggle so the overlay opens fresh next time.
func TestBgTasksOverlayToggleShowAll(t *testing.T) {
	t.Parallel()

	t.Run("toggles in completed mode and resets scroll", func(t *testing.T) {
		m := Model{
			overlay:                   overlayBackgroundTasks,
			scheduler:                 scheduler.New(0),
			tasksOverlayShowCompleted: true,
			tasksOverlayScroll:        12,
		}
		ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('a'))
		result := ret.(Model)
		assert.True(t, result.tasksOverlayShowAll)
		assert.Equal(t, 0, result.tasksOverlayScroll, "scroll resets so the expanded list starts from the top")

		ret2, _ := result.handleBackgroundTasksOverlayKey(runeKey('a'))
		result2 := ret2.(Model)
		assert.False(t, result2.tasksOverlayShowAll, "second press flips back")
	})

	t.Run("no-op in running mode", func(t *testing.T) {
		m := Model{
			overlay:                   overlayBackgroundTasks,
			scheduler:                 scheduler.New(0),
			tasksOverlayShowCompleted: false,
			tasksOverlayShowAll:       false,
		}
		ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('a'))
		result := ret.(Model)
		assert.False(t, result.tasksOverlayShowAll, "running mode never filters, so `a` does nothing")
		assert.Equal(t, overlayBackgroundTasks, result.overlay)
	})

	t.Run("esc resets the toggle", func(t *testing.T) {
		m := Model{
			overlay:                   overlayBackgroundTasks,
			scheduler:                 scheduler.New(0),
			tasksOverlayShowCompleted: true,
			tasksOverlayShowAll:       true,
		}
		ret, _ := m.handleBackgroundTasksOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
		result := ret.(Model)
		assert.False(t, result.tasksOverlayShowAll, "esc must reset showAll so the next open starts filtered")
	})
}

// TestBgTasksOverlayFreezesHistoryWhenScrolled is the regression guard
// for the user-reported jank where scrolling through the completed
// history was disrupted by background completions reshuffling rows
// under the cursor. The scrolled-into state must capture a stable
// snapshot until the user returns to the top.
func TestBgTasksOverlayFreezesHistoryWhenScrolled(t *testing.T) {
	t.Parallel()
	const rows = 30

	t.Run("scrolling into list captures a snapshot", func(t *testing.T) {
		m := Model{
			overlay:                   overlayBackgroundTasks,
			scheduler:                 newSchedulerWithCompletedHistory(rows),
			tasksOverlayShowCompleted: true,
			width:                     120,
			height:                    30,
		}
		// Initially at top — no freeze.
		assert.Nil(t, m.tasksOverlayFrozenHistory)

		ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('j'))
		result := ret.(Model)
		assert.NotNil(t, result.tasksOverlayFrozenHistory,
			"j into the list must capture a frozen snapshot")
		// The captured count matches what historyTasksForDisplay would
		// produce on the same scheduler — that's what the renderer
		// will read while scrolled.
		assert.Equal(t, rows, len(result.tasksOverlayFrozenHistory))
	})

	t.Run("scroll back to top clears the freeze", func(t *testing.T) {
		m := Model{
			overlay:                   overlayBackgroundTasks,
			scheduler:                 newSchedulerWithCompletedHistory(rows),
			tasksOverlayShowCompleted: true,
			width:                     120,
			height:                    30,
			tasksOverlayScroll:        1,
			tasksOverlayFrozenHistory: []ui.BackgroundTaskRow{{Name: "stale"}},
		}
		ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 0, result.tasksOverlayScroll)
		assert.Nil(t, result.tasksOverlayFrozenHistory,
			"k back to the top must clear the snapshot so live updates resume")
	})

	t.Run("g jumps to top and clears freeze", func(t *testing.T) {
		m := Model{
			overlay:                   overlayBackgroundTasks,
			scheduler:                 newSchedulerWithCompletedHistory(rows),
			tasksOverlayShowCompleted: true,
			width:                     120,
			height:                    30,
			tasksOverlayScroll:        5,
			tasksOverlayFrozenHistory: []ui.BackgroundTaskRow{{Name: "stale"}},
		}
		ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('g'))
		result := ret.(Model)
		assert.Nil(t, result.tasksOverlayFrozenHistory,
			"g (jump to top) must clear the snapshot")
	})

	t.Run("Tab clears the freeze", func(t *testing.T) {
		m := Model{
			overlay:                   overlayBackgroundTasks,
			scheduler:                 newSchedulerWithCompletedHistory(rows),
			tasksOverlayShowCompleted: true,
			width:                     120,
			height:                    30,
			tasksOverlayScroll:        5,
			tasksOverlayFrozenHistory: []ui.BackgroundTaskRow{{Name: "stale"}},
		}
		ret, _ := m.handleBackgroundTasksOverlayKey(tea.KeyMsg{Type: tea.KeyTab})
		result := ret.(Model)
		assert.Nil(t, result.tasksOverlayFrozenHistory,
			"Tab must clear the snapshot — the new view has its own row set")
	})

	t.Run("`a` toggle clears the freeze", func(t *testing.T) {
		m := Model{
			overlay:                   overlayBackgroundTasks,
			scheduler:                 newSchedulerWithCompletedHistory(rows),
			tasksOverlayShowCompleted: true,
			width:                     120,
			height:                    30,
			tasksOverlayScroll:        5,
			tasksOverlayFrozenHistory: []ui.BackgroundTaskRow{{Name: "stale"}},
		}
		ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('a'))
		result := ret.(Model)
		assert.Nil(t, result.tasksOverlayFrozenHistory,
			"`a` changes the filter, so the frozen snapshot must be discarded")
	})

	t.Run("running view never freezes", func(t *testing.T) {
		m := Model{
			overlay:                   overlayBackgroundTasks,
			scheduler:                 newSchedulerWithCompletedHistory(rows),
			tasksOverlayShowCompleted: false,
			width:                     120,
			height:                    30,
		}
		ret, _ := m.handleBackgroundTasksOverlayKey(runeKey('j'))
		result := ret.(Model)
		assert.Nil(t, result.tasksOverlayFrozenHistory,
			"freeze is only meaningful in the completed view")
	})
}
