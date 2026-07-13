package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
