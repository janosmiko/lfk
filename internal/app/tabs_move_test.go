package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// tabContexts returns the per-tab context names in slice order so a reorder is
// easy to assert against. threeTabModel seeds prod / staging / dev.
func tabContexts(m Model) []string {
	out := make([]string, len(m.tabs))
	for i, t := range m.tabs {
		out[i] = t.nav.Context
	}
	return out
}

func TestMoveActiveTab_Right(t *testing.T) {
	m := threeTabModel(0) // active = prod

	moved := m.moveActiveTab(+1)

	require.True(t, moved)
	assert.Equal(t, 1, m.activeTab, "active tab follows the move")
	assert.Equal(t, []string{"staging", "prod", "dev"}, tabContexts(m))
}

func TestMoveActiveTab_Left(t *testing.T) {
	m := threeTabModel(2) // active = dev

	moved := m.moveActiveTab(-1)

	require.True(t, moved)
	assert.Equal(t, 1, m.activeTab)
	assert.Equal(t, []string{"prod", "dev", "staging"}, tabContexts(m))
}

func TestMoveActiveTab_RightAtEdgeIsNoOp(t *testing.T) {
	m := threeTabModel(2) // active = dev (rightmost)

	moved := m.moveActiveTab(+1)

	assert.False(t, moved, "rightmost tab cannot move further right (no wrap)")
	assert.Equal(t, 2, m.activeTab)
	assert.Equal(t, []string{"prod", "staging", "dev"}, tabContexts(m))
}

func TestMoveActiveTab_LeftAtEdgeIsNoOp(t *testing.T) {
	m := threeTabModel(0) // active = prod (leftmost)

	moved := m.moveActiveTab(-1)

	assert.False(t, moved, "leftmost tab cannot move further left (no wrap)")
	assert.Equal(t, 0, m.activeTab)
	assert.Equal(t, []string{"prod", "staging", "dev"}, tabContexts(m))
}

func TestMoveActiveTab_SingleTabIsNoOp(t *testing.T) {
	m := baseExplorerModel()
	m.tabs = []TabState{{nav: model.NavigationState{Context: "only"}}}
	m.activeTab = 0

	assert.False(t, m.moveActiveTab(+1))
	assert.False(t, m.moveActiveTab(-1))
	assert.Equal(t, 0, m.activeTab)
}

// TestActionKeyBraceMovesTab drives the reorder through the explorer key
// dispatcher (the path used in explorer/exec mode) with the default brace
// bindings.
func TestActionKeyBraceMovesTab(t *testing.T) {
	m := threeTabModel(0) // active = prod

	ret, _, handled := m.handleExplorerActionKey(runeKey('}'))
	require.True(t, handled)
	right := ret.(Model)
	assert.Equal(t, 1, right.activeTab)
	assert.Equal(t, []string{"staging", "prod", "dev"}, tabContexts(right))
	assert.Contains(t, right.statusMessage, "Tab moved to position 2")

	ret, _, handled = right.handleExplorerActionKey(runeKey('{'))
	require.True(t, handled)
	left := ret.(Model)
	assert.Equal(t, 0, left.activeTab)
	assert.Equal(t, []string{"prod", "staging", "dev"}, tabContexts(left))
}

// TestActionKeyBraceMoveTabAtEdgeStillHandled verifies the brace is swallowed
// even when the move is a no-op, so it never leaks to another handler.
func TestActionKeyBraceMoveTabAtEdgeStillHandled(t *testing.T) {
	m := threeTabModel(2) // active = dev (rightmost)

	_, _, handled := m.handleExplorerActionKey(runeKey('}'))
	assert.True(t, handled)
}

// TestMoveActiveTab_KeepsPreviewLogStream is a regression guard: a move is not a
// tab switch, so it must not cancel the live preview-log stream (which is never
// restarted afterward). moveActiveTab must avoid saveCurrentTab's
// cancelPreviewLogStream side effect.
func TestMoveActiveTab_KeepsPreviewLogStream(t *testing.T) {
	m := threeTabModel(0)
	cancelled := false
	m.previewLog.podKey = "ns/pod-a/"
	m.previewLog.lines = []string{"line-1", "line-2"}
	m.previewLog.cancel = func() { cancelled = true }

	require.True(t, m.moveActiveTab(+1))

	assert.False(t, cancelled, "move must not cancel the preview-log goroutine")
	assert.Equal(t, "ns/pod-a/", m.previewLog.podKey, "preview stream identity preserved")
	assert.Equal(t, []string{"line-1", "line-2"}, m.previewLog.lines, "preview buffer preserved")
}
