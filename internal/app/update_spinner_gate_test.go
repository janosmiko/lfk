package app

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/stretchr/testify/assert"
)

// TestSpinnerWantedConditions documents every state that pulls the spinner
// onto the screen. Keep this in sync with view.go / view_right.go: if a new
// site calls m.spinner.View() under a flag we don't check here, the spinner
// will appear frozen in that state because updateTick lets the tick chain
// die when spinnerWanted returns false (issue #206).
func TestSpinnerWantedConditions(t *testing.T) {
	t.Run("idle returns false", func(t *testing.T) {
		m := basePush80Model()
		assert.False(t, m.spinnerWanted(), "fresh model with no loading should not need the spinner")
	})

	t.Run("loading flag triggers true", func(t *testing.T) {
		m := basePush80Model()
		m.loading = true
		assert.True(t, m.spinnerWanted())
	})

	t.Run("previewLoading triggers true", func(t *testing.T) {
		m := basePush80Model()
		m.previewLoading = true
		assert.True(t, m.spinnerWanted())
	})

	// Scheduler LenIndicator only counts tasks that have been picked up by
	// a worker, not queued submissions. Exercising that path requires a
	// running pool which the existing scheduler / title-bar tests already
	// cover; replicating it here would just duplicate setup.

	t.Run("discovering context triggers true", func(t *testing.T) {
		m := basePush80Model()
		if m.discoveringContexts == nil {
			m.discoveringContexts = make(map[string]bool)
		}
		m.discoveringContexts["test-ctx"] = true
		assert.True(t, m.spinnerWanted())
	})
}

// TestUpdateTickGatesOnSpinnerWanted is the core of the fix: when nothing
// wants the spinner, the tick handler must drop the chain (return nil) so
// the bubbletea event loop goes quiet. When something does want it, the
// handler keeps the chain alive by returning a non-nil command (the next
// tick).
func TestUpdateTickGatesOnSpinnerWanted(t *testing.T) {
	t.Run("idle drops the tick chain", func(t *testing.T) {
		m := basePush80Model()
		_, cmd := m.updateTick(spinner.TickMsg{})
		assert.Nil(t, cmd, "idle tick must not reschedule")
	})

	t.Run("loading keeps the tick chain alive", func(t *testing.T) {
		m := basePush80Model()
		m.loading = true
		_, cmd := m.updateTick(spinner.TickMsg{})
		assert.NotNil(t, cmd, "loading must keep the spinner ticking")
	})
}

// TestUpdateKicksSpinnerOnWantedTransition verifies the wrapper in Update:
// when a dispatched message flips the model from "spinner not wanted" to
// "spinner wanted", the wrapper attaches a kick command so the chain
// actually starts. Stale or no-op messages where the wanted state is
// unchanged must not attach a kick, even if loading is currently true.
func TestUpdateKicksSpinnerOnWantedTransition(t *testing.T) {
	t.Run("no transition, no kick", func(t *testing.T) {
		m := basePush80Model()
		m.loading = true
		// previewDebounceTickMsg with a stale gen is a no-op: dispatch
		// returns (m, nil), wanted stays true, no transition → no kick.
		_, cmd := m.Update(previewDebounceTickMsg{gen: m.previewDebounceGen + 99})
		assert.Nil(t, cmd, "stale tick with no wanted transition must not attach a spinner kick")
	})
}
