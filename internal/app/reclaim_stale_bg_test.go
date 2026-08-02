package app

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInvalidatePreviewForCursorChange_ReclaimsStaleDashboard reproduces the
// reported bug: a cursor move off the cluster-dashboard overview must drop the
// queued Low-priority dashboard sections from the now-stale generation so they
// stop blocking fresh work in the shared Low lane.
func TestInvalidatePreviewForCursorChange_ReclaimsStaleDashboard(t *testing.T) {
	m := newTestModelWithScheduler()
	m.nav.Context = "test-ctx"
	// Navigation always bumps requestGen before any load, so real dashboard
	// tasks carry a non-zero Gen. Mirror that here — Gen == 0 is the
	// intentional "non-navigational, never stale" exemption in staleByGen.
	m.requestGen = 1
	// No StartWorkers: keep tasks queued so we can observe the reclaim.

	// A Low dashboard section submitted at the current generation.
	cmd := m.scheduleK8sCall(scheduler.PriorityLow, scheduler.KindDashboard,
		"Dashboard: pods", "test-ctx#pods",
		func(context.Context) tea.Msg { return stubMsg{value: "stale"} })
	require.NotNil(t, cmd)
	require.Equal(t, 1, m.scheduler.QueueLen("test-ctx"))

	// Cursor move bumps requestGen and reclaims stale Low work.
	m.invalidatePreviewForCursorChange()

	assert.Equal(t, 0, m.scheduler.QueueLen("test-ctx"),
		"stale Low dashboard task must be reclaimed on cursor change")

	// The original cmd resolves to a nil msg (ErrSuperseded maps to nil).
	// Guard with a timeout so a regression that leaves the task queued fails
	// fast here instead of blocking on the future until the suite timeout.
	got := make(chan tea.Msg, 1)
	go func() { got <- cmd() }()
	select {
	case msg := <-got:
		assert.Nil(t, msg, "superseded task's cmd must return nil, not a stale message")
	case <-time.After(2 * time.Second):
		t.Fatal("cmd() blocked: superseded task's future was never resolved")
	}
}

// TestInvalidatePreviewForCursorChange_KeepsHighPriorityView guards the scope:
// a High-priority fetch (the resource list the user is waiting for) must
// survive the gen bump a cursor move triggers.
func TestInvalidatePreviewForCursorChange_KeepsHighPriorityView(t *testing.T) {
	m := newTestModelWithScheduler()
	m.nav.Context = "test-ctx"
	// Carry a non-zero Gen like a real navigation load, so the task's
	// survival is attributable to the High-priority exemption, not the
	// Gen==0 "never stale" special-case.
	m.requestGen = 1

	cmd := m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindResourceList,
		"List Events", "test-ctx",
		func(context.Context) tea.Msg { return stubMsg{value: "list"} })
	require.NotNil(t, cmd)

	m.invalidatePreviewForCursorChange()

	assert.Equal(t, 1, m.scheduler.QueueLen("test-ctx"),
		"High-priority view fetch must not be reclaimed by a cursor move")
}
