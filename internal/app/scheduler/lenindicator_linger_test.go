package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestLenIndicator_SkipsFinishedLingering locks in the title-bar
// indicator's narrow semantic: LenIndicator must only count tasks
// whose work is actually in progress. Finished tasks stay in r.tasks
// for DefaultLingerDuration so the :tasks overlay's Running list can
// show "what just ran" — but those entries are not work-in-progress
// and the title-bar spinner must not spin for them.
//
// Before this fix lenLocked counted finished-within-linger entries,
// so the spinner kept spinning for the full 10-second linger window
// after every navigation, even though all the underlying K8s calls
// had already returned. Reported by the user: "I select a pod. The
// pod's details load, the metrics load, etc and after this the upper
// right spinner still spins for additional ~10 seconds".
//
// Len() (overlay-facing) keeps its broader semantic — see the
// existing TestRunTask_SilentTrackAppearsInHistoryButNotIndicator
// assertion that "Len() returns the full count (overlay-facing)" for
// the analogous silent-task split.
func TestLenIndicator_SkipsFinishedLingering(t *testing.T) {
	r := New(0)
	r.SetLingerDurationForTest(500 * time.Millisecond)

	id := r.Start(KindResourceList, "List Pods", "default")
	assert.Equal(t, 1, r.LenIndicator(), "running task must count")
	assert.Equal(t, 1, r.Len(), "running task must count via Len too")

	r.Finish(id)

	// Within the linger window: task is still in r.tasks so the overlay
	// can render it under "Running", but the title bar must show no
	// spinner — work is done, the linger is purely for UX history.
	assert.Equal(t, 0, r.LenIndicator(),
		"finished-lingering task must NOT count toward the title-bar indicator")
	assert.Equal(t, 1, r.Len(),
		"finished-lingering task still counts via Len (overlay-facing snapshot still includes it)")

	// And of course past the linger window both drop to zero.
	time.Sleep(600 * time.Millisecond)
	assert.Equal(t, 0, r.LenIndicator(), "past-linger: nothing visible to the indicator")
	assert.Equal(t, 0, r.Len(), "past-linger: nothing visible at all")
}
