package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A rune index and a cell column diverge once a wide glyph precedes the
// selection, so a rune-based slice copies the wrong substring.
func TestErrorLogYankCharVisualWideRune(t *testing.T) {
	entries := []ui.ErrorLogEntry{
		{Time: time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC), Level: "ERR", Message: "日本語hello"},
	}
	// Plain text: "00:00:00 ERR 日本語hello" - prefix is 13 ASCII cells,
	// then 日(13-14) 本(15-16) 語(17-18) h(19) e(20) l(21) l(22) o(23).
	// Columns 15-18 cover "本語".
	m := Model{
		overlayErrorLog:        true,
		errorLog:               entries,
		errorLogVisualMode:     'v',
		errorLogVisualStart:    0,
		errorLogVisualStartCol: 15,
		errorLogCursorLine:     0,
		errorLogCursorCol:      18,
		tabs:                   []TabState{{}},
		width:                  80,
		height:                 40,
	}

	got := stubClipboardWriteAll(t, func(string) error { return nil })

	_, cmd := m.errorLogYank()
	require.NotNil(t, cmd)
	batch, ok := cmd().(tea.BatchMsg)
	require.True(t, ok, "expected a batch of copy + status clear")
	for _, sub := range batch {
		sub()
	}

	assert.Equal(t, "本語", *got)
}
