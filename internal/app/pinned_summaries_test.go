package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

// The state file round-trips through the same PinnedState shape the sidebar
// pins use, just at a different path.
func TestPinnedSummariesState_RoundTrip(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())

	s := loadPinnedSummariesState()
	require.NotNil(t, s)
	assert.Empty(t, s.Contexts)

	added := togglePinnedType(s, "ctx-a", "batch/jobs")
	assert.True(t, added)
	require.NoError(t, savePinnedSummariesState(s))

	reloaded := loadPinnedSummariesState()
	assert.Equal(t, []string{"batch/jobs"}, reloaded.Contexts["ctx-a"])

	removed := togglePinnedType(reloaded, "ctx-a", "batch/jobs")
	assert.False(t, removed)
	require.NoError(t, savePinnedSummariesState(reloaded))
	assert.Empty(t, loadPinnedSummariesState().Contexts["ctx-a"])
}

func TestPinnedSummariesState_CorruptFileStartsFresh(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LFK_STATE_DIR", dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pinned_summaries.yaml"), []byte("{not yaml"), 0o600))
	s := loadPinnedSummariesState()
	require.NotNil(t, s)
	assert.Empty(t, s.Contexts)
}

func TestEffectivePinnedSummaries_MergesConfigAndState(t *testing.T) {
	orig := ui.ConfigPinnedSummaries
	ui.ConfigPinnedSummaries = []string{"batch/jobs", "/pods"}
	t.Cleanup(func() { ui.ConfigPinnedSummaries = orig })

	m := Model{pinnedSummariesState: newPinnedState()}
	m.nav.Context = "ctx-a"
	m.pinnedSummariesState.Contexts["ctx-a"] = []string{"argoproj.io/applications", "batch/jobs"}

	got := m.effectivePinnedSummaries()
	assert.Equal(t, []string{"batch/jobs", "/pods", "argoproj.io/applications"}, got)
}

func TestEffectivePinnedSummaries_CapsAtMax(t *testing.T) {
	m := Model{pinnedSummariesState: newPinnedState()}
	m.nav.Context = "ctx-a"
	keys := make([]string, 0, maxPinnedSummaries+3)
	for i := range maxPinnedSummaries + 3 {
		keys = append(keys, fmt.Sprintf("group%d/things", i))
	}
	m.pinnedSummariesState.Contexts["ctx-a"] = keys
	assert.Len(t, m.effectivePinnedSummaries(), maxPinnedSummaries)
}

func TestIsSummaryPinned(t *testing.T) {
	m := Model{pinnedSummariesState: newPinnedState()}
	m.nav.Context = "ctx-a"
	m.pinnedSummariesState.Contexts["ctx-a"] = []string{"batch/jobs"}
	assert.True(t, m.isSummaryPinned("batch/jobs"))
	assert.False(t, m.isSummaryPinned("/pods"))
	assert.False(t, Model{}.isSummaryPinned("batch/jobs"), "nil state must not panic")
}
