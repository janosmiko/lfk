package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRebuildLogVisibleIndices(t *testing.T) {
	det, _ := newSeverityDetector(nil)
	m := Model{
		logLines:            []string{"[INFO] a", "[ERROR] b", "[INFO] c"},
		logSeverityDetector: det,
		logIncludeMode:      IncludeAny,
	}
	m.logFilterChain = NewFilterChain(nil, IncludeAny, m.logSeverityDetector)

	// No rules — rebuild leaves logVisibleIndices nil so the renderer
	// short-circuits to the raw buffer. logVisibleCount() still reports
	// all lines via its chain-inactive fallback.
	m.rebuildLogVisibleIndices()
	assert.Nil(t, m.logVisibleIndices, "no rules: indices stay nil")
	assert.Equal(t, 3, m.logVisibleCount(), "all lines visible")

	// Add severity floor warn — now the filter is active and the
	// indices slice holds exactly the passing source indices.
	m.logRules = []Rule{SeverityRule{SeverityWarn}}
	m.logFilterChain = NewFilterChain(m.logRules, IncludeAny, m.logSeverityDetector)
	m.rebuildLogVisibleIndices()
	assert.Equal(t, []int{1}, m.logVisibleIndices)
}

func TestAppendLogLinePreservesFilter(t *testing.T) {
	det, _ := newSeverityDetector(nil)
	m := Model{
		logLines:            []string{"[INFO] a"},
		logVisibleIndices:   []int{0},
		logSeverityDetector: det,
	}
	m.logRules = []Rule{SeverityRule{SeverityWarn}}
	m.logFilterChain = NewFilterChain(m.logRules, IncludeAny, m.logSeverityDetector)
	m.rebuildLogVisibleIndices()
	assert.Equal(t, []int{}, m.logVisibleIndices, "info filtered out")

	// Simulate the streaming append + visibility check.
	m.logLines = append(m.logLines, "[ERROR] b")
	m.maybeAppendVisibleIndex(len(m.logLines) - 1)
	assert.Equal(t, []int{1}, m.logVisibleIndices, "error appended")

	m.logLines = append(m.logLines, "[INFO] c")
	m.maybeAppendVisibleIndex(len(m.logLines) - 1)
	assert.Equal(t, []int{1}, m.logVisibleIndices, "info still filtered")
}

func TestLogCursorOverVisibleIndices(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	m := Model{
		logLines:            []string{"[INFO] a", "[ERROR] b", "[INFO] c", "[ERROR] d"},
		logSeverityDetector: d,
	}
	m.logRules = []Rule{SeverityRule{SeverityWarn}}
	m.logFilterChain = NewFilterChain(m.logRules, IncludeAny, d)
	m.rebuildLogVisibleIndices()
	assert.Equal(t, []int{1, 3}, m.logVisibleIndices)
	assert.Equal(t, 2, m.logVisibleCount())
	assert.Equal(t, 1, m.logCursorMax())

	// Source line 1 corresponds to visible cursor 0.
	assert.Equal(t, 1, m.logSourceLineAt(0))
	assert.Equal(t, 3, m.logSourceLineAt(1))
	assert.Equal(t, -1, m.logSourceLineAt(2)) // out of range
}

func TestJumpToNextError(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	m := Model{
		logLines: []string{
			"[INFO] a",
			"[ERROR] b",
			"[WARN] c",
			"[ERROR] d",
			"[INFO] e",
		},
		logSeverityDetector: d,
		logCursor:           0,
	}
	m.logFilterChain = NewFilterChain(nil, IncludeAny, d)
	m.rebuildLogVisibleIndices()

	rm := m.jumpToSeverity(+1, SeverityError)
	assert.Equal(t, 1, rm.logCursor, "first error at index 1")

	rm2 := rm.jumpToSeverity(+1, SeverityError)
	assert.Equal(t, 3, rm2.logCursor, "next error at index 3")

	// Wrap
	rm3 := rm2.jumpToSeverity(+1, SeverityError)
	assert.Equal(t, 1, rm3.logCursor, "wraps to first error")
}

func TestJumpToPrevWarning(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	m := Model{
		logLines: []string{
			"[INFO] a",
			"[WARN] b",
			"[INFO] c",
			"[WARN] d",
		},
		logSeverityDetector: d,
		logCursor:           3,
	}
	m.logFilterChain = NewFilterChain(nil, IncludeAny, d)
	m.rebuildLogVisibleIndices()

	rm := m.jumpToSeverity(-1, SeverityWarn)
	assert.Equal(t, 1, rm.logCursor, "prev warn at index 1")
}

func TestBracketEJumpsToNextError(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	m := Model{
		mode:                modeLogs,
		logLines:            []string{"[INFO] a", "[ERROR] b"},
		logSeverityDetector: d,
		logCursor:           0,
	}
	m.logFilterChain = NewFilterChain(nil, IncludeAny, d)
	m.rebuildLogVisibleIndices()

	// First keystroke: "]" primes the pending-bracket state.
	res1, _ := m.handleLogKey(keyMsg("]"))
	m1 := res1.(Model)
	assert.Equal(t, ']', m1.logPendingBracket, "] primes pending bracket")
	assert.Equal(t, 0, m1.logCursor, "cursor does not move yet")

	// Second keystroke: "e" jumps to next error.
	res2, _ := m1.handleLogKey(keyMsg("e"))
	m2 := res2.(Model)
	assert.Equal(t, rune(0), m2.logPendingBracket, "pending cleared")
	assert.Equal(t, 1, m2.logCursor, "jumped to error at index 1")
}

func TestBracketEReverseJumpsToPrevError(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	m := Model{
		mode:                modeLogs,
		logLines:            []string{"[ERROR] a", "[INFO] b", "[ERROR] c"},
		logSeverityDetector: d,
		logCursor:           2,
	}
	m.logFilterChain = NewFilterChain(nil, IncludeAny, d)
	m.rebuildLogVisibleIndices()

	res1, _ := m.handleLogKey(keyMsg("["))
	m1 := res1.(Model)
	assert.Equal(t, '[', m1.logPendingBracket)

	res2, _ := m1.handleLogKey(keyMsg("e"))
	m2 := res2.(Model)
	assert.Equal(t, 0, m2.logCursor, "jumped to prev error")
}

func TestBracketWJumpsToNextWarn(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	m := Model{
		mode:                modeLogs,
		logLines:            []string{"[INFO] a", "[WARN] b", "[INFO] c"},
		logSeverityDetector: d,
		logCursor:           0,
	}
	m.logFilterChain = NewFilterChain(nil, IncludeAny, d)
	m.rebuildLogVisibleIndices()

	res1, _ := m.handleLogKey(keyMsg("]"))
	m1 := res1.(Model)
	res2, _ := m1.handleLogKey(keyMsg("w"))
	m2 := res2.(Model)
	assert.Equal(t, 1, m2.logCursor, "jumped to warn")
}

func TestBracketWReverseJumpsToPrevWarn(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	m := Model{
		mode:                modeLogs,
		logLines:            []string{"[WARN] a", "[INFO] b", "[WARN] c"},
		logSeverityDetector: d,
		logCursor:           2,
	}
	m.logFilterChain = NewFilterChain(nil, IncludeAny, d)
	m.rebuildLogVisibleIndices()

	res1, _ := m.handleLogKey(keyMsg("["))
	m1 := res1.(Model)
	res2, _ := m1.handleLogKey(keyMsg("w"))
	m2 := res2.(Model)
	assert.Equal(t, 0, m2.logCursor, "jumped to prev warn")
}

func TestBracketThenNonSeverityKeyClearsPending(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	m := Model{
		mode:                modeLogs,
		logLines:            []string{"[INFO] a", "[INFO] b"},
		logSeverityDetector: d,
		logCursor:           0,
	}
	m.logFilterChain = NewFilterChain(nil, IncludeAny, d)
	m.rebuildLogVisibleIndices()

	res1, _ := m.handleLogKey(keyMsg("]"))
	m1 := res1.(Model)
	assert.Equal(t, ']', m1.logPendingBracket)

	// Non-severity key should clear the pending state.
	res2, _ := m1.handleLogKey(keyMsg("j"))
	m2 := res2.(Model)
	assert.Equal(t, rune(0), m2.logPendingBracket, "pending cleared on unrelated key")
}

func TestBracketEjumpsOnlyOverVisibleLines(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	m := Model{
		mode: modeLogs,
		logLines: []string{
			"[INFO] a",
			"[ERROR] b",
			"[WARN] c",
			"[ERROR] d",
		},
		logSeverityDetector: d,
		logCursor:           0,
	}
	// Filter out INFO — only WARN/ERROR remain visible.
	m.logRules = []Rule{SeverityRule{SeverityWarn}}
	m.logFilterChain = NewFilterChain(m.logRules, IncludeAny, d)
	m.rebuildLogVisibleIndices()
	assert.Equal(t, []int{1, 2, 3}, m.logVisibleIndices)

	res1, _ := m.handleLogKey(keyMsg("]"))
	m1 := res1.(Model)
	res2, _ := m1.handleLogKey(keyMsg("e"))
	m2 := res2.(Model)
	// With cursor at visible idx 0 (source 1, ERROR), next ERROR in +1 direction is visible idx 2 (source 3).
	assert.Equal(t, 2, m2.logCursor, "jumps to next ERROR visible position")
}
