package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextTipFromFile_CyclesWithoutRepeats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tip-cursor")
	tips := []string{"one", "two", "three"}

	// Pin the start of the rotation; a fresh file starts at a random offset.
	require.NoError(t, os.WriteFile(path, []byte("0"), 0o600))

	seen := make([]string, 0, len(tips))
	for range tips {
		seen = append(seen, nextTipFromFile(path, tips))
	}
	assert.Equal(t, tips, seen, "every tip must appear once before any repeat")

	assert.Equal(t, "one", nextTipFromFile(path, tips), "rotation must wrap to the start")
}

func TestNextTipFromFile_CorruptCursorStartsFresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tip-cursor")
	require.NoError(t, os.WriteFile(path, []byte("not-a-number"), 0o600))
	tips := []string{"one", "two"}

	tip := nextTipFromFile(path, tips)
	assert.Contains(t, tips, tip)

	// The cursor must be repaired: subsequent calls cycle normally.
	next := nextTipFromFile(path, tips)
	assert.Contains(t, tips, next)
	assert.NotEqual(t, tip, next, "after repair the rotation must advance")
}

func TestNextTipFromFile_CursorBeyondListWraps(t *testing.T) {
	// A shrunken tips list (downgrade or trimmed list) must not panic.
	path := filepath.Join(t.TempDir(), "tip-cursor")
	require.NoError(t, os.WriteFile(path, []byte("99"), 0o600))
	tips := []string{"one", "two", "three"}

	tip := nextTipFromFile(path, tips)
	assert.Contains(t, tips, tip)
}

func TestNextTipFromFile_NoStatePathStillReturnsTip(t *testing.T) {
	tips := []string{"only"}
	assert.Equal(t, "only", nextTipFromFile("", tips))
}

func TestNextTipFromFile_EmptyTips(t *testing.T) {
	assert.Empty(t, nextTipFromFile("", nil))
}
