package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/ui"
)

func TestRebuildLogView_TextFilter(t *testing.T) {
	var m Model
	m.logView.rawLines = []string{"apple pie", "banana split", "apple tart"}
	m.logView.filterQuery = "apple"
	m.rebuildLogView()
	if len(m.logView.lines) != 2 {
		t.Fatalf("got %d lines, want 2: %v", len(m.logView.lines), m.logView.lines)
	}
}

func TestRebuildLogView_NoFilterAliasesRaw(t *testing.T) {
	var m Model
	m.logView.rawLines = []string{"a", "b", "c"}
	m.rebuildLogView()
	if len(m.logView.lines) != 3 {
		t.Fatalf("no filter should show all 3, got %d", len(m.logView.lines))
	}
}

func TestRebuildLogView_Severity(t *testing.T) {
	var m Model
	m.logView.rawLines = []string{
		`{"level":"info","msg":"a"}`,
		`{"level":"error","msg":"b"}`,
		"\tcontinuation of the error",
		`{"level":"debug","msg":"c"}`,
	}
	m.logView.sevThreshold = ui.SevError
	m.rebuildLogView()
	// error line + its continuation tail (inherits error) survive; info/debug drop.
	if len(m.logView.lines) != 2 {
		t.Fatalf("got %d lines, want 2 (error + continuation): %v", len(m.logView.lines), m.logView.lines)
	}
}

func TestRebuildLogView_ClampsCursor(t *testing.T) {
	var m Model
	m.logView.rawLines = []string{"keep one", "drop", "drop", "drop"}
	m.logView.cursor = 3
	m.logView.scroll = 3
	m.logView.filterQuery = "keep"
	m.rebuildLogView()
	if m.logView.cursor >= len(m.logView.lines) {
		t.Fatalf("cursor %d not clamped into %d lines", m.logView.cursor, len(m.logView.lines))
	}
}
