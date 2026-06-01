package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- findNextLogMatch ---

func TestFindNextLogMatch(t *testing.T) {
	t.Run("forward finds next match", func(t *testing.T) {
		m := Model{
			height: 30,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:       []string{"info: start", "error: failed", "info: ok", "error: timeout"},
				searchQuery: "error",
				cursor:      0,
			},
		}
		m.findNextLogMatch(true)
		assert.Equal(t, 1, m.logView.cursor)
	})

	t.Run("forward wraps around", func(t *testing.T) {
		m := Model{
			height: 30,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:       []string{"error: first", "info: ok", "info: ok2"},
				searchQuery: "error",
				cursor:      2,
			},
		}
		m.findNextLogMatch(true)
		assert.Equal(t, 0, m.logView.cursor)
	})

	t.Run("backward finds previous match", func(t *testing.T) {
		m := Model{
			height: 30,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:       []string{"error: first", "info: ok", "error: second", "info: ok2"},
				searchQuery: "error",
				cursor:      3,
			},
		}
		m.findNextLogMatch(false)
		assert.Equal(t, 2, m.logView.cursor)
	})

	t.Run("backward wraps around", func(t *testing.T) {
		m := Model{
			height: 30,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:       []string{"info: ok", "info: ok2", "error: last"},
				searchQuery: "error",
				cursor:      0,
			},
		}
		m.findNextLogMatch(false)
		assert.Equal(t, 2, m.logView.cursor)
	})

	t.Run("empty query does nothing", func(t *testing.T) {
		m := Model{
			height: 30,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:       []string{"error: test"},
				searchQuery: "",
				cursor:      0,
			},
		}
		m.findNextLogMatch(true)
		assert.Equal(t, 0, m.logView.cursor)
	})

	t.Run("no match keeps cursor", func(t *testing.T) {
		m := Model{
			height: 30,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:       []string{"info: ok", "debug: test"},
				searchQuery: "error",
				cursor:      0,
			},
		}
		m.findNextLogMatch(true)
		assert.Equal(t, 0, m.logView.cursor)
	})

	t.Run("case insensitive search", func(t *testing.T) {
		m := Model{
			height: 30,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:       []string{"info: ok", "ERROR: FAILED"},
				searchQuery: "error",
				cursor:      0,
			},
		}
		m.findNextLogMatch(true)
		assert.Equal(t, 1, m.logView.cursor)
	})

	t.Run("forward does not panic when cursor col exceeds line length", func(t *testing.T) {
		// Regression: logVisualCurCol carries over from a previously
		// focused long line. When `n` triggers a forward search and the
		// current (start) line is shorter than logVisualCurCol+1, the
		// rune-slice indexing must not panic.
		m := Model{
			height: 30,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:        []string{"short", "info: target here"},
				searchQuery:  "target",
				cursor:       0,
				visualCurCol: 900, // far beyond "short"
			},
		}
		assert.NotPanics(t, func() { m.findNextLogMatch(true) })
		assert.Equal(t, 1, m.logView.cursor)
	})

	t.Run("backward does not panic when cursor col exceeds line length", func(t *testing.T) {
		// Same regression for the backward path (N / shift-n).
		m := Model{
			height: 30,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:        []string{"info: target here", "short"},
				searchQuery:  "target",
				cursor:       1,
				visualCurCol: 900, // far beyond "short"
			},
		}
		assert.NotPanics(t, func() { m.findNextLogMatch(false) })
		assert.Equal(t, 0, m.logView.cursor)
	})

	t.Run("does not panic on multi-byte rune lines when cursor col exceeds rune count", func(t *testing.T) {
		// The bug is fundamentally about rune vs byte length divergence:
		// `[]rune(line)[:n]` panics when n > len(runes). Multi-byte content
		// (e.g. `こんにちは` is 5 runes / 15 bytes) exercises the rune path
		// distinct from len(line). Verify both forward and backward.
		m := Model{
			height: 30,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:        []string{"こんにちは", "info: target here"},
				searchQuery:  "target",
				cursor:       0,
				visualCurCol: 900, // far beyond 5 runes
			},
		}
		assert.NotPanics(t, func() { m.findNextLogMatch(true) })
		assert.Equal(t, 1, m.logView.cursor)

		m2 := Model{
			height: 30,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:        []string{"info: target here", "こんにちは"},
				searchQuery:  "target",
				cursor:       1,
				visualCurCol: 900,
			},
		}
		assert.NotPanics(t, func() { m2.findNextLogMatch(false) })
		assert.Equal(t, 0, m2.logView.cursor)
	})

	t.Run("disables log follow on match", func(t *testing.T) {
		m := Model{
			height: 30,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:       []string{"info: ok", "error: test"},
				searchQuery: "error",
				cursor:      0,
				follow:      true,
			},
		}
		m.findNextLogMatch(true)
		assert.Equal(t, 1, m.logView.cursor)
		assert.False(t, m.logView.follow)
	})
}

func TestPush4HandleLogKeyQ(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	result, _ := m.handleLogKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush4HandleLogKeyJ(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logView.follow = true
	m.logView.lines = []string{"line1", "line2", "line3"}
	m.logView.cursor = 0
	result, _ := m.handleLogKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.logView.cursor)
}

func TestPush4HandleLogKeyK(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logView.lines = []string{"line1", "line2", "line3"}
	m.logView.cursor = 2
	result, _ := m.handleLogKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.logView.cursor)
}

func TestPush4HandleLogKeyG(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.pendingG = true
	m.logView.lines = []string{"line1", "line2"}
	m.logView.cursor = 1
	result, _ := m.handleLogKey(keyMsg("g"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.logView.cursor)
}

func TestPush4HandleLogKeyGBig(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logView.lines = []string{"line1", "line2", "line3"}
	result, _ := m.handleLogKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.logView.cursor)
	assert.True(t, rm.logView.follow)
}

func TestPush4HandleLogKeyF(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logView.follow = false
	result, _ := m.handleLogKey(keyMsg("f"))
	rm := result.(Model)
	assert.True(t, rm.logView.follow)
}

func TestPush4HandleLogKeyW(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	result, _ := m.handleLogKey(keyMsg("w"))
	_ = result.(Model)
}

func TestPush4HandleLogKeyN(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	// n is search-next in log view.
	result, _ := m.handleLogKey(keyMsg("n"))
	_ = result.(Model)
}

func TestPush4HandleLogKeyEsc(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	result, _ := m.handleLogKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush4HandleLogKeySearch(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	result, _ := m.handleLogKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.logView.searchActive)
}

func TestPush4HandleLogKeyVisualMode(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logView.lines = []string{"line1", "line2"}
	result, _ := m.handleLogKey(keyMsg("v"))
	rm := result.(Model)
	assert.True(t, rm.logView.visualMode)
}

func TestPush4HandleLogKeyVisualModeV(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logView.lines = []string{"line1", "line2"}
	result, _ := m.handleLogKey(keyMsg("V"))
	rm := result.(Model)
	assert.True(t, rm.logView.visualMode)
}

func TestPush4HandleLogKeyHalfPageDown(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logView.lines = make([]string, 100)
	m.logView.cursor = 0
	kb := ui.ActiveKeybindings
	result, _ := m.handleLogKey(keyMsg(kb.PageDown))
	rm := result.(Model)
	assert.Greater(t, rm.logView.cursor, 0)
}

func TestPush4HandleLogKeyHalfPageUp(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logView.lines = make([]string, 100)
	m.logView.cursor = 50
	kb := ui.ActiveKeybindings
	result, _ := m.handleLogKey(keyMsg(kb.PageUp))
	rm := result.(Model)
	assert.Less(t, rm.logView.cursor, 50)
}

func TestCovLogKeyHelp(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logView.lines = []string{"line1", "line2"}
	result, _ := m.handleLogKey(keyMsg("?"))
	rm := result.(Model)
	assert.Equal(t, modeHelp, rm.mode)
}

func TestCovLogKeyEsc(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	result, _ := m.handleLogKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestCovLogKeyQ(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	result, _ := m.handleLogKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestCovLogKeyDown(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logView.lines = []string{"l1", "l2", "l3", "l4", "l5"}
	m.logView.cursor = 0
	m.logView.follow = false
	result, _ := m.handleLogKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.logView.cursor)
}

func TestCovLogKeyUp(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logView.lines = []string{"l1", "l2", "l3"}
	m.logView.cursor = 2
	m.logView.follow = false
	result, _ := m.handleLogKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.logView.cursor)
}

func TestCovLogKeyToggleFollow(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logView.follow = false
	m.logView.lines = []string{"l1"}
	result, _ := m.handleLogKey(keyMsg("f"))
	rm := result.(Model)
	assert.True(t, rm.logView.follow)
}

func TestCovLogKeyDigit(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logView.lines = []string{"l1"}
	result, _ := m.handleLogKey(keyMsg("5"))
	rm := result.(Model)
	assert.Equal(t, "5", rm.logView.lineInput)
}

func TestCovLogKeyCtrlF(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logView.lines = make([]string, 100)
	m.logView.cursor = 0
	m.logView.follow = false
	result, _ := m.handleLogKey(keyMsg("ctrl+f"))
	rm := result.(Model)
	assert.Greater(t, rm.logView.cursor, 0)
}

func TestCovLogKeyCtrlB(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logView.lines = make([]string, 100)
	m.logView.cursor = 50
	m.logView.follow = false
	result, _ := m.handleLogKey(keyMsg("ctrl+b"))
	rm := result.(Model)
	assert.Less(t, rm.logView.cursor, 50)
}

func TestCovLogKeyGG(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logView.cursor = 3
	m.logView.lines = []string{"l1", "l2", "l3", "l4"}
	m.logView.follow = false
	result, _ := m.handleLogKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)
	result, _ = rm.handleLogKey(keyMsg("g"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.logView.cursor)
}

func TestCovLogKeyBigG(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logView.cursor = 0
	m.logView.lines = []string{"l1", "l2", "l3"}
	m.logView.follow = false
	result, _ := m.handleLogKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.logView.cursor)
}

func TestCovLogKeyCtrlD(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logView.lines = make([]string, 100)
	m.logView.cursor = 0
	m.logView.follow = false
	result, _ := m.handleLogKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.Greater(t, rm.logView.cursor, 0)
}

func TestCovLogKeyCtrlU(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logView.lines = make([]string, 100)
	m.logView.cursor = 50
	m.logView.follow = false
	result, _ := m.handleLogKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.Less(t, rm.logView.cursor, 50)
}

func TestCovLogKeySlash(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logView.lines = []string{"l1"}
	result, _ := m.handleLogKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.logView.searchActive)
}

func TestCovLogKeyVisualV(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logView.lines = []string{"l1", "l2"}
	m.logView.cursor = 0
	m.logView.follow = false
	result, _ := m.handleLogKey(keyMsg("V"))
	rm := result.(Model)
	assert.True(t, rm.logView.visualMode)
}

func TestCovLogSearchKeyEnter(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.searchActive = true
	m.logView.searchInput.Insert("error")
	m.logView.lines = []string{"error line", "ok line"}
	result, _ := m.handleLogKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.logView.searchActive)
	assert.Equal(t, "error", rm.logView.searchQuery)
}

func TestCovLogSearchKeyEsc(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.searchActive = true
	m.logView.searchInput.Insert("test")
	result, _ := m.handleLogKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.logView.searchActive)
}

func TestCovLogSearchKeyBackspace(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.searchActive = true
	m.logView.searchInput.Insert("ab")
	m.logView.lines = []string{"abc"}
	result, _ := m.handleLogKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Equal(t, "a", rm.logView.searchInput.Value)
}

// Ctrl+U is the standard readline "delete-to-line-start" shortcut and
// is supported by the explorer's `/` and `f` inputs (via DeleteLine).
// The log viewer's `/` input previously dropped Ctrl+U on the floor —
// it fell through to the printable-char branch in `default`, which
// rejected it. Verify both that the input is cleared and that
// logSearchQuery is kept in sync so the live highlight overlay stops
// painting matches for the now-empty query.
func TestCovLogSearchKeyCtrlU(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.searchActive = true
	m.logView.searchInput.Insert("error")
	m.logView.searchQuery = "error"
	m.logView.lines = []string{"error line"}

	result, _ := m.handleLogKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.Equal(t, "", rm.logView.searchInput.Value)
	assert.Equal(t, "", rm.logView.searchQuery,
		"Ctrl+U must clear logSearchQuery so the highlight overlay stops painting")
}

func TestCovLogSearchKeyTyping(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.searchActive = true
	m.logView.lines = []string{"test"}
	result, _ := m.handleLogKey(keyMsg("x"))
	rm := result.(Model)
	assert.Equal(t, "x", rm.logView.searchInput.Value)
}

// Regression: typing into the log-viewer search input now updates
// logSearchQuery alongside logSearchInput so the highlight overlay
// paints in real time. Previously the highlight only appeared once the
// user pressed Enter to "commit" the query.
func TestLogSearchTypingUpdatesQueryLive(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.searchActive = true
	m.logView.lines = []string{"some error here"}

	result, _ := m.handleLogKey(keyMsg("e"))
	rm := result.(Model)
	assert.Equal(t, "e", rm.logView.searchInput.Value)
	assert.Equal(t, "e", rm.logView.searchQuery,
		"logSearchQuery must mirror logSearchInput while typing so highlights paint live")

	result, _ = rm.handleLogKey(keyMsg("r"))
	rm = result.(Model)
	assert.Equal(t, "er", rm.logView.searchQuery)

	result, _ = rm.handleLogKey(keyMsg("backspace"))
	rm = result.(Model)
	assert.Equal(t, "e", rm.logView.searchQuery,
		"backspace must keep logSearchQuery in sync, not leave the highlight stale")
}

// --- log search history (Up/Down recall, persisted to log-search-history) ---

// Enter commits the query into the per-viewer log search history so
// later sessions can recall it via Up. Mirrors the explorer's `/`
// behaviour (queryHistory.add+save in handleSearchKey) but writes to a
// separate file because log search runs on raw log bytes, not resource
// names.
func TestLogSearchHistoryEnterAdds(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.searchHistory = &commandHistory{cursor: -1}
	m.logView.searchActive = true
	m.logView.searchInput.Insert("error")
	m.logView.lines = []string{"error line"}

	result, _ := m.handleLogKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, []string{"error"}, rm.logView.searchHistory.entries)
}

// Esc cancels without committing — the input is wiped and the history
// stays untouched. Otherwise an aborted exploratory query would clutter
// recall.
func TestLogSearchHistoryEscDoesNotAdd(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.searchHistory = &commandHistory{cursor: -1}
	m.logView.searchActive = true
	m.logView.searchInput.Insert("typo")

	result, _ := m.handleLogKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Empty(t, rm.logView.searchHistory.entries)
}

// Up replays the most recent entry into the input and mirrors it into
// logSearchQuery so the live highlight repaints (same contract as
// typing). Down past the newest entry restores the draft the user was
// composing before they started recalling.
func TestLogSearchHistoryUpDownRecall(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.searchHistory = &commandHistory{
		cursor:  -1,
		entries: []string{"first", "second"},
	}
	m.logView.searchActive = true
	m.logView.searchInput.Insert("draft")
	m.logView.searchQuery = "draft"

	// Up -> newest entry, draft saved.
	result, _ := m.handleLogKey(keyMsg("up"))
	rm := result.(Model)
	assert.Equal(t, "second", rm.logView.searchInput.Value)
	assert.Equal(t, "second", rm.logView.searchQuery)

	// Up again -> older entry.
	result, _ = rm.handleLogKey(keyMsg("up"))
	rm = result.(Model)
	assert.Equal(t, "first", rm.logView.searchInput.Value)

	// Down -> back toward newer.
	result, _ = rm.handleLogKey(keyMsg("down"))
	rm = result.(Model)
	assert.Equal(t, "second", rm.logView.searchInput.Value)

	// Down past newest -> restores the original draft.
	result, _ = rm.handleLogKey(keyMsg("down"))
	rm = result.(Model)
	assert.Equal(t, "draft", rm.logView.searchInput.Value)
	assert.Equal(t, "draft", rm.logView.searchQuery)
}

// Editing a recalled entry must drop the user out of history-browsing
// mode (cursor -> -1, draft cleared). Otherwise a subsequent Down would
// snap back to the original pre-recall draft and silently discard the
// edits the user just typed.
func TestLogSearchHistoryEditAfterRecallResets(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.searchHistory = &commandHistory{
		cursor:  -1,
		entries: []string{"old"},
	}
	m.logView.searchActive = true

	// Recall the entry.
	result, _ := m.handleLogKey(keyMsg("up"))
	rm := result.(Model)
	assert.Equal(t, "old", rm.logView.searchInput.Value)
	assert.NotEqual(t, -1, rm.logView.searchHistory.cursor)

	// Type a char -> resets cursor to -1.
	result, _ = rm.handleLogKey(keyMsg("x"))
	rm = result.(Model)
	assert.Equal(t, -1, rm.logView.searchHistory.cursor)
	assert.Equal(t, "oldx", rm.logView.searchInput.Value)
}

// Ctrl+W (delete-word) after recall must also drop out of history
// browsing, for the same reason as backspace and typing — a follow-up
// Down past newest should restore the post-edit text, not the
// pre-recall draft.
func TestLogSearchHistoryCtrlWAfterRecallResets(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.searchHistory = &commandHistory{
		cursor:  -1,
		entries: []string{"hello world"},
	}
	m.logView.searchActive = true

	result, _ := m.handleLogKey(keyMsg("up"))
	rm := result.(Model)
	assert.NotEqual(t, -1, rm.logView.searchHistory.cursor)

	result, _ = rm.handleLogKey(keyMsg("ctrl+w"))
	rm = result.(Model)
	assert.Equal(t, -1, rm.logView.searchHistory.cursor)
}

// Ctrl+U (delete-to-line-start) after recall must also drop out of
// history browsing, same rationale as backspace, ctrl+w, and typing.
func TestLogSearchHistoryCtrlUAfterRecallResets(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.searchHistory = &commandHistory{
		cursor:  -1,
		entries: []string{"hello world"},
	}
	m.logView.searchActive = true

	result, _ := m.handleLogKey(keyMsg("up"))
	rm := result.(Model)
	assert.NotEqual(t, -1, rm.logView.searchHistory.cursor)

	result, _ = rm.handleLogKey(keyMsg("ctrl+u"))
	rm = result.(Model)
	assert.Equal(t, -1, rm.logView.searchHistory.cursor)
	assert.Equal(t, "", rm.logView.searchInput.Value)
}

// Backspace after recall also resets the history cursor for the same
// reason as the typing case above.
func TestLogSearchHistoryBackspaceAfterRecallResets(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.searchHistory = &commandHistory{
		cursor:  -1,
		entries: []string{"old"},
	}
	m.logView.searchActive = true

	result, _ := m.handleLogKey(keyMsg("up"))
	rm := result.(Model)
	assert.NotEqual(t, -1, rm.logView.searchHistory.cursor)

	result, _ = rm.handleLogKey(keyMsg("backspace"))
	rm = result.(Model)
	assert.Equal(t, -1, rm.logView.searchHistory.cursor)
}

// TestLogSearchHistoryEditThenDownRestoresPreRecallDraft pins the fix
// from issue #115 for the log viewer's / search: after Up→edit, Down
// past newest must restore the original pre-recall draft, not "".
func TestLogSearchHistoryEditThenDownRestoresPreRecallDraft(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.searchHistory = &commandHistory{
		cursor:  -1,
		entries: []string{"err"},
	}
	m.logView.searchActive = true
	m.logView.searchInput.Set("ngi")

	// Up: recall "err", draft "ngi" saved.
	result, _ := m.handleLogKey(keyMsg("up"))
	rm := result.(Model)
	require.Equal(t, "err", rm.logView.searchInput.Value)

	// Edit: type a char. Must leave browse but preserve draft.
	result, _ = rm.handleLogKey(keyMsg("x"))
	rm = result.(Model)
	require.Equal(t, "errx", rm.logView.searchInput.Value)
	require.Equal(t, -1, rm.logView.searchHistory.cursor)

	// Down past newest: pre-recall draft "ngi" must come back.
	result, _ = rm.handleLogKey(keyMsg("down"))
	rm = result.(Model)
	assert.Equal(t, "ngi", rm.logView.searchInput.Value, "Down past newest must restore pre-recall draft")
}

// Opening search via `/` resets any leftover cursor from a prior
// recall so the next Up starts at the newest entry, not partway through
// the previous browse.
func TestLogSearchHistorySlashResetsCursor(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.searchHistory = &commandHistory{
		cursor:  0, // simulating a prior in-progress recall
		entries: []string{"a", "b"},
		draft:   "stale",
	}

	result, _ := m.handleLogKey(keyMsg("/"))
	rm := result.(Model)
	assert.Equal(t, -1, rm.logView.searchHistory.cursor)
	assert.Equal(t, "", rm.logView.searchHistory.draft)
}

func TestCovLogVisualKeyEsc(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.visualMode = true
	m.logView.lines = []string{"l1", "l2"}
	result, _ := m.handleLogKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.logView.visualMode)
}

func TestCovLogVisualKeyYank(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.visualMode = true
	m.logView.visualStart = 0
	m.logView.cursor = 1
	m.logView.lines = []string{"l1", "l2"}
	_, cmd := m.handleLogKey(keyMsg("y"))
	assert.NotNil(t, cmd)
}

func TestCovLogVisualKeyDown(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.visualMode = true
	m.logView.cursor = 0
	m.logView.lines = []string{"l1", "l2", "l3"}
	result, _ := m.handleLogKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.logView.cursor)
}

func TestCovLogVisualKeyUp(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.visualMode = true
	m.logView.cursor = 2
	m.logView.lines = []string{"l1", "l2", "l3"}
	result, _ := m.handleLogKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.logView.cursor)
}

// --- handleLogKeyS2: Save loaded logs (S) ---

func TestHandleLogKeyS2CopiesPathToClipboard(t *testing.T) {
	// Issue #61: when the user presses S to save the loaded log buffer,
	// the destination path should be auto-copied to the system clipboard
	// so it survives the 5s status-clear, and the status message should
	// announce that explicitly.
	m := baseModel()
	m.mode = modeLogs
	m.logView.lines = []string{"line1", "line2"}
	m.actionCtx = actionContext{name: "test-pod"}

	ret, cmd := m.handleLogKeyS2()
	rm := ret.(Model)

	assert.False(t, rm.statusMessageErr, "save success should not be an error status")
	assert.Contains(t, rm.statusMessage, "Loaded logs saved to ")
	assert.Contains(t, rm.statusMessage, "(copied to clipboard)",
		"status should announce the clipboard copy so the user knows the path is recoverable")
	assert.NotNil(t, cmd, "cmd should batch the clipboard write with the status-clear timer")
}

// Pressing \ in the log viewer for a single Pod must NOT open the container
// filter overlay until the container list has loaded. Setting the overlay
// up-front and rendering it with empty/loading state caused a visible
// flash before the real data arrived; users perceived the brief overlay
// (especially when stale items from a prior namespace selector use were
// still in m.overlayItems) as "the namespace selector flashing". Mirrors
// the group-resource branch which only sets overlayLogPodSelect from
// updatePodLogSelect after the pods have loaded.
func TestHandleLogKeyOtherSinglePodDefersOverlayUntilContainersLoad(t *testing.T) {
	m := baseModel()
	m.mode = modeLogs
	m.actionCtx.kind = "Pod"
	m.actionCtx.name = "my-pod"
	// Stale items from a previous namespace selector use must not bleed
	// into the next overlay either.
	m.overlayItems = []model.Item{
		{Name: "All Namespaces", Status: "all"},
		{Name: "default"},
		{Name: "kube-system"},
	}
	ret, cmd := m.handleLogKeyOther()
	rm := ret.(Model)
	assert.Equal(t, overlayNone, rm.overlay,
		"overlay must stay closed while the container list is loading; updateLogContainersLoaded opens it once data arrives")
	assert.Nil(t, rm.overlayItems,
		"stale overlay items must be cleared so any later overlay open does not see leftover content")
	assert.True(t, rm.loading,
		"loading flag must be set so the user gets visual feedback that work is happening")
	assert.Contains(t, rm.statusMessage, "Loading containers",
		"status bar must announce the load so the user knows something is happening")
	assert.NotNil(t, cmd, "loadContainersForLogFilter command must be returned")
}

func TestHandleLogKeyS2ErrorPathDoesNotCopy(t *testing.T) {
	// On save failure there is nothing useful to copy; the status should
	// still report the error and the cmd should remain just the clear timer.
	m := baseModel()
	m.mode = modeLogs
	// actionCtx.name is empty; saveLoadedLogs will still try /tmp/lfk-logs--<unix>.log
	// which works on writable filesystems, so we instead force a write failure by
	// pointing TMPDIR at a non-existent directory.
	t.Setenv("TMPDIR", "/this/path/does/not/exist/lfk-test")
	m.logView.lines = []string{"line1"}

	ret, _ := m.handleLogKeyS2()
	rm := ret.(Model)

	assert.True(t, rm.statusMessageErr, "save failure should set the error flag")
	assert.NotContains(t, rm.statusMessage, "(copied to clipboard)",
		"clipboard suffix only makes sense on success")
}
