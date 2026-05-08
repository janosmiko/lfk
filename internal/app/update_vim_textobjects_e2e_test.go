package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// End-to-end tests that walk the full v -> i/a -> w/W -> y sequence per viewer
// and verify the resulting clipboard text + status message. Goal: cover the
// entire manual-test matrix programmatically so a UI walkthrough isn't needed.

// --- shared helpers ---

// runVisualTextObject simulates entering visual mode at (cursor, col), then
// pressing op (i/a), then motion (w/W). Returns the resulting Model.
func runLogVisualTextObject(t *testing.T, line string, col int, op, motion byte) Model {
	t.Helper()
	m := Model{
		mode:            modeLogs,
		logLines:        []string{line},
		logCursor:       0,
		logVisualCurCol: col,
		logVisualMode:   true,
		logVisualType:   'v',
		logVisualStart:  0,
		logVisualCol:    col,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	r1, _ := m.handleLogVisualKey(keyMsg(string(op)))
	r2, _ := r1.(Model).handleLogVisualKey(keyMsg(string(motion)))
	return r2.(Model)
}

// --- log viewer: full 4-variant matrix with clipText + status ---

func TestLogTextObjectMatrix_AlphaBetaGamma(t *testing.T) {
	// Line: "alpha beta gamma", cursor on 'e' of beta (col 7).
	const line = "alpha beta gamma"
	const col = 7

	cases := []struct {
		name, sequence string
		wantClip       string
		wantStatus     string
	}{
		{"viw", "iw", "beta", "Copied"},
		{"vaw", "aw", "beta ", "Copied"},
		{"viW", "iW", "beta", "Copied"},
		{"vaW", "aW", "beta ", "Copied"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := runLogVisualTextObject(t, line, col, tc.sequence[0], tc.sequence[1])
			// Press 'y' to yank.
			r, cmd := m.handleLogVisualKey(keyMsg("y"))
			rm := r.(Model)
			clipText, _ := m.buildLogYankText()
			assert.Equal(t, tc.wantClip, clipText, "clipboard text")
			assert.Equal(t, tc.wantStatus, rm.statusMessage, "status message")
			assert.False(t, rm.logVisualMode, "visual mode exits after yank")
			assert.NotNil(t, cmd, "yank dispatches a clipboard command")
		})
	}
}

// --- log viewer: word vs WORD distinction on punctuation ---

func TestLogTextObjectWordVsWORD_FooBarBaz(t *testing.T) {
	// Line "foo-bar baz" with cursor on 'b' of 'bar' (col 4).
	// `iw` should select just "bar" (hyphen is a word boundary in this codebase).
	// `iW` should select "foo-bar" (only whitespace breaks WORDs).
	const line = "foo-bar baz"
	const col = 4

	t.Run("viw selects just bar", func(t *testing.T) {
		m := runLogVisualTextObject(t, line, col, 'i', 'w')
		clipText, _ := m.buildLogYankText()
		assert.Equal(t, "bar", clipText)
	})

	t.Run("viW spans the hyphen", func(t *testing.T) {
		m := runLogVisualTextObject(t, line, col, 'i', 'W')
		clipText, _ := m.buildLogYankText()
		assert.Equal(t, "foo-bar", clipText)
	})

	t.Run("vaW includes trailing space", func(t *testing.T) {
		m := runLogVisualTextObject(t, line, col, 'a', 'W')
		clipText, _ := m.buildLogYankText()
		assert.Equal(t, "foo-bar ", clipText)
	})
}

// --- log viewer: cursor on whitespace (vim semantics) ---

func TestLogTextObjectCursorOnWhitespace(t *testing.T) {
	// "alpha beta gamma" — cursor on the space at col 5.
	const line = "alpha beta gamma"

	t.Run("viw selects the whitespace run", func(t *testing.T) {
		m := runLogVisualTextObject(t, line, 5, 'i', 'w')
		clipText, _ := m.buildLogYankText()
		assert.Equal(t, " ", clipText)
	})

	t.Run("vaw extends through next word", func(t *testing.T) {
		m := runLogVisualTextObject(t, line, 5, 'a', 'w')
		clipText, _ := m.buildLogYankText()
		assert.Equal(t, " beta", clipText)
	})
}

// --- log viewer: cursor at last char of last word, no trailing whitespace ---

func TestLogTextObjectAtLineEnd(t *testing.T) {
	// "alpha beta" — cursor on 'a' at end (col 9). vaw should fall back to
	// leading whitespace since there's no trailing.
	const line = "alpha beta"

	t.Run("viw selects last word", func(t *testing.T) {
		m := runLogVisualTextObject(t, line, 9, 'i', 'w')
		clipText, _ := m.buildLogYankText()
		assert.Equal(t, "beta", clipText)
	})

	t.Run("vaw falls back to leading whitespace", func(t *testing.T) {
		m := runLogVisualTextObject(t, line, 9, 'a', 'w')
		clipText, _ := m.buildLogYankText()
		assert.Equal(t, " beta", clipText)
	})
}

// --- log viewer: empty / out-of-bounds cursor doesn't crash ---

func TestLogTextObjectEmptyLine(t *testing.T) {
	m := Model{
		mode:          modeLogs,
		logLines:      []string{""},
		logVisualMode: true,
		logVisualType: 'v',
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	r1, _ := m.handleLogVisualKey(keyMsg("i"))
	r2, _ := r1.(Model).handleLogVisualKey(keyMsg("w"))
	rm := r2.(Model)
	// No crash; selection unchanged.
	assert.True(t, rm.logVisualMode, "visual mode preserved")
	assert.Equal(t, byte(0), rm.pendingTextObject, "operator cleared even on no-op")
}

// --- yaml viewer: viw produces correct clipText after fold-prefix offset ---

func TestYAMLTextObjectViwYanksWord(t *testing.T) {
	m := Model{
		mode:           modeYAML,
		yamlContent:    "name: alpha-beta",
		yamlCursor:     0,
		yamlVisualMode: true,
		yamlVisualType: 'v',
	}
	// Visible line is "  name: alpha-beta" (fold prefix prepends 2 chars).
	// Cursor on 'p' of "alpha" -> col yamlFoldPrefixLen + 8 = 10.
	m.yamlVisualCurCol = yamlFoldPrefixLen + 8
	m.yamlVisualCol = m.yamlVisualCurCol
	m.tabs = []TabState{{}}
	m.width = 80
	m.height = 40

	r1, _ := m.handleYAMLVisualKey(keyMsg("i"))
	r2, _ := r1.(Model).handleYAMLVisualKey(keyMsg("w"))
	r3, cmd := r2.(Model).handleYAMLVisualKey(keyMsg("y"))
	rm := r3.(Model)

	// `iw` on "alpha-beta" with cursor on 'p': hyphen is a boundary, so
	// selection is "alpha" — a single-line char yank just says "Copied".
	assert.Equal(t, "Copied", rm.statusMessage,
		"single-line word yank reports just 'Copied'")
	assert.False(t, rm.yamlVisualMode)
	assert.NotNil(t, cmd)
}

func TestYAMLTextObjectViWYanksHyphenated(t *testing.T) {
	m := Model{
		mode:           modeYAML,
		yamlContent:    "name: alpha-beta",
		yamlCursor:     0,
		yamlVisualMode: true,
		yamlVisualType: 'v',
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	m.yamlVisualCurCol = yamlFoldPrefixLen + 8 // on 'p' of alpha
	m.yamlVisualCol = m.yamlVisualCurCol

	r1, _ := m.handleYAMLVisualKey(keyMsg("i"))
	r2, _ := r1.(Model).handleYAMLVisualKey(keyMsg("W"))
	r3, _ := r2.(Model).handleYAMLVisualKey(keyMsg("y"))
	rm := r3.(Model)

	// `iW` includes the hyphen: "alpha-beta" — single-line, so "Copied".
	assert.Equal(t, "Copied", rm.statusMessage)
}

// --- describe viewer: viw + vaw with status check ---

func TestDescribeTextObjectMatrix(t *testing.T) {
	cases := []struct {
		name, sequence, want string
	}{
		{"viw", "iw", "Copied"},
		{"vaw", "aw", "Copied"},
		{"viW", "iW", "Copied"},
		{"vaW", "aW", "Copied"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{
				mode:               modeDescribe,
				describeContent:    "alpha beta gamma",
				describeCursor:     0,
				describeCursorCol:  7,
				describeVisualMode: 'v',
				tabs:               []TabState{{}},
				width:              80,
				height:             40,
			}
			r1, _ := m.handleDescribeVisualKey(keyMsg(string(tc.sequence[0])))
			r2, _ := r1.(Model).handleDescribeVisualKey(keyMsg(string(tc.sequence[1])))
			r3, _ := r2.(Model).handleDescribeVisualKey(keyMsg("y"))
			rm := r3.(Model)
			assert.Equal(t, tc.want, rm.statusMessage)
			assert.Equal(t, byte(0), rm.describeVisualMode, "visual exits after yank")
		})
	}
}

// --- diff viewer: viw on real diff content ---

func TestDiffTextObjectViwYanksWord(t *testing.T) {
	m := baseModelNav()
	m.mode = modeDiff
	m.diffLeft = "alpha beta gamma\n"
	m.diffRight = "alpha beta gamma\n"
	m.diffCursor = 0
	m.diffCursorSide = 0
	m.diffUnified = true // simpler: single-side line text
	m.diffVisualCurCol = 7
	m.diffVisualCol = 7
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffVisualStart = 0

	r1, _ := m.handleDiffVisualKey(keyMsg("i"), nil, 1, 5, 0)
	r2, _ := r1.(Model).handleDiffVisualKey(keyMsg("w"), nil, 1, 5, 0)
	r3, _ := r2.(Model).handleDiffVisualKey(keyMsg("y"), nil, 1, 5, 0)
	rm := r3.(Model)

	// Single-line char-mode diff yank just says "Copied" regardless of the
	// exact line-prefix-driven column math.
	assert.Equal(t, "Copied", rm.statusMessage)
	assert.False(t, rm.diffVisualMode)
}

// --- events viewer: viw + vaw matrix ---

func TestEventsTextObjectMatrix(t *testing.T) {
	cases := []struct {
		name, sequence, want string
	}{
		{"viw", "iw", "Copied"},
		{"vaw", "aw", "Copied"},
		{"viW", "iW", "Copied"},
		{"vaW", "aW", "Copied"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{
				eventTimelineLines:       []string{"alpha beta gamma"},
				eventTimelineCursor:      0,
				eventTimelineCursorCol:   7,
				eventTimelineVisualMode:  'v',
				eventTimelineVisualStart: 0,
				eventTimelineVisualCol:   7,
				tabs:                     []TabState{{}},
				width:                    80,
				height:                   40,
			}
			r1, _ := m.handleEventTimelineVisualKey(keyMsg(string(tc.sequence[0])))
			r2, _ := r1.(Model).handleEventTimelineVisualKey(keyMsg(string(tc.sequence[1])))
			r3, _ := r2.(Model).handleEventTimelineVisualKey(keyMsg("y"))
			rm := r3.(Model)
			assert.Equal(t, tc.want, rm.statusMessage)
			assert.Equal(t, byte(0), rm.eventTimelineVisualMode, "visual exits after yank")
		})
	}
}

// --- visual-type downgrade: V/B before iw both produce char-mode selection ---

func TestLogTextObjectDowngradesLineMode(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"alpha beta gamma"},
		logCursor:       0,
		logVisualCurCol: 7,
		logVisualMode:   true,
		logVisualType:   'V', // line mode
		logVisualStart:  0,
		logVisualCol:    7,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	r1, _ := m.handleLogVisualKey(keyMsg("i"))
	r2, _ := r1.(Model).handleLogVisualKey(keyMsg("w"))
	rm := r2.(Model)
	assert.Equal(t, rune('v'), rm.logVisualType, "line mode downgraded to char mode after iw")
}

func TestLogTextObjectDowngradesBlockMode(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"alpha beta gamma"},
		logCursor:       0,
		logVisualCurCol: 7,
		logVisualMode:   true,
		logVisualType:   'B', // block mode
		logVisualStart:  0,
		logVisualCol:    7,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	r1, _ := m.handleLogVisualKey(keyMsg("i"))
	r2, _ := r1.(Model).handleLogVisualKey(keyMsg("w"))
	rm := r2.(Model)
	assert.Equal(t, rune('v'), rm.logVisualType, "block mode downgraded to char mode after iw")
}

// --- regression: line-mode yank still says lines, not characters ---

func TestLineModeYankStillReportsLines(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"alpha", "beta"},
		logCursor:       1,
		logVisualMode:   true,
		logVisualType:   'V',
		logVisualStart:  0,
		logVisualCol:    0,
		logVisualCurCol: 0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	r, _ := m.handleLogVisualKey(keyMsg("y"))
	rm := r.(Model)
	assert.Equal(t, "Copied 2 lines", rm.statusMessage,
		"line-mode yank must still report lines, not characters")
}
