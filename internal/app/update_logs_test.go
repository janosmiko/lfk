package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
)

// --- findNextLogMatch ---

func TestFindNextLogMatch(t *testing.T) {
	t.Run("forward finds next match", func(t *testing.T) {
		m := Model{
			height:         30,
			width:          80,
			tabs:           []TabState{{}},
			logLines:       []string{"info: start", "error: failed", "info: ok", "error: timeout"},
			logSearchQuery: "error",
			logCursor:      0,
		}
		m.findNextLogMatch(true)
		assert.Equal(t, 1, m.logCursor)
	})

	t.Run("forward wraps around", func(t *testing.T) {
		m := Model{
			height:         30,
			width:          80,
			tabs:           []TabState{{}},
			logLines:       []string{"error: first", "info: ok", "info: ok2"},
			logSearchQuery: "error",
			logCursor:      2,
		}
		m.findNextLogMatch(true)
		assert.Equal(t, 0, m.logCursor)
	})

	t.Run("backward finds previous match", func(t *testing.T) {
		m := Model{
			height:         30,
			width:          80,
			tabs:           []TabState{{}},
			logLines:       []string{"error: first", "info: ok", "error: second", "info: ok2"},
			logSearchQuery: "error",
			logCursor:      3,
		}
		m.findNextLogMatch(false)
		assert.Equal(t, 2, m.logCursor)
	})

	t.Run("backward wraps around", func(t *testing.T) {
		m := Model{
			height:         30,
			width:          80,
			tabs:           []TabState{{}},
			logLines:       []string{"info: ok", "info: ok2", "error: last"},
			logSearchQuery: "error",
			logCursor:      0,
		}
		m.findNextLogMatch(false)
		assert.Equal(t, 2, m.logCursor)
	})

	t.Run("empty query does nothing", func(t *testing.T) {
		m := Model{
			height:         30,
			width:          80,
			tabs:           []TabState{{}},
			logLines:       []string{"error: test"},
			logSearchQuery: "",
			logCursor:      0,
		}
		m.findNextLogMatch(true)
		assert.Equal(t, 0, m.logCursor)
	})

	t.Run("no match keeps cursor", func(t *testing.T) {
		m := Model{
			height:         30,
			width:          80,
			tabs:           []TabState{{}},
			logLines:       []string{"info: ok", "debug: test"},
			logSearchQuery: "error",
			logCursor:      0,
		}
		m.findNextLogMatch(true)
		assert.Equal(t, 0, m.logCursor)
	})

	t.Run("case insensitive search", func(t *testing.T) {
		m := Model{
			height:         30,
			width:          80,
			tabs:           []TabState{{}},
			logLines:       []string{"info: ok", "ERROR: FAILED"},
			logSearchQuery: "error",
			logCursor:      0,
		}
		m.findNextLogMatch(true)
		assert.Equal(t, 1, m.logCursor)
	})

	t.Run("forward does not panic when cursor col exceeds line length", func(t *testing.T) {
		// Regression: logVisualCurCol carries over from a previously
		// focused long line. When `n` triggers a forward search and the
		// current (start) line is shorter than logVisualCurCol+1, the
		// rune-slice indexing must not panic.
		m := Model{
			height:          30,
			width:           80,
			tabs:            []TabState{{}},
			logLines:        []string{"short", "info: target here"},
			logSearchQuery:  "target",
			logCursor:       0,
			logVisualCurCol: 900, // far beyond "short"
		}
		assert.NotPanics(t, func() { m.findNextLogMatch(true) })
		assert.Equal(t, 1, m.logCursor)
	})

	t.Run("backward does not panic when cursor col exceeds line length", func(t *testing.T) {
		// Same regression for the backward path (N / shift-n).
		m := Model{
			height:          30,
			width:           80,
			tabs:            []TabState{{}},
			logLines:        []string{"info: target here", "short"},
			logSearchQuery:  "target",
			logCursor:       1,
			logVisualCurCol: 900, // far beyond "short"
		}
		assert.NotPanics(t, func() { m.findNextLogMatch(false) })
		assert.Equal(t, 0, m.logCursor)
	})

	t.Run("does not panic on multi-byte rune lines when cursor col exceeds rune count", func(t *testing.T) {
		// The bug is fundamentally about rune vs byte length divergence:
		// `[]rune(line)[:n]` panics when n > len(runes). Multi-byte content
		// (e.g. `こんにちは` is 5 runes / 15 bytes) exercises the rune path
		// distinct from len(line). Verify both forward and backward.
		m := Model{
			height:          30,
			width:           80,
			tabs:            []TabState{{}},
			logLines:        []string{"こんにちは", "info: target here"},
			logSearchQuery:  "target",
			logCursor:       0,
			logVisualCurCol: 900, // far beyond 5 runes
		}
		assert.NotPanics(t, func() { m.findNextLogMatch(true) })
		assert.Equal(t, 1, m.logCursor)

		m2 := Model{
			height:          30,
			width:           80,
			tabs:            []TabState{{}},
			logLines:        []string{"info: target here", "こんにちは"},
			logSearchQuery:  "target",
			logCursor:       1,
			logVisualCurCol: 900,
		}
		assert.NotPanics(t, func() { m2.findNextLogMatch(false) })
		assert.Equal(t, 0, m2.logCursor)
	})

	t.Run("disables log follow on match", func(t *testing.T) {
		m := Model{
			height:         30,
			width:          80,
			tabs:           []TabState{{}},
			logLines:       []string{"info: ok", "error: test"},
			logSearchQuery: "error",
			logCursor:      0,
			logFollow:      true,
		}
		m.findNextLogMatch(true)
		assert.Equal(t, 1, m.logCursor)
		assert.False(t, m.logFollow)
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
	m.logFollow = true
	m.logLines = []string{"line1", "line2", "line3"}
	m.logCursor = 0
	result, _ := m.handleLogKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.logCursor)
}

func TestPush4HandleLogKeyK(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logLines = []string{"line1", "line2", "line3"}
	m.logCursor = 2
	result, _ := m.handleLogKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.logCursor)
}

func TestPush4HandleLogKeyG(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.pendingG = true
	m.logLines = []string{"line1", "line2"}
	m.logCursor = 1
	result, _ := m.handleLogKey(keyMsg("g"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.logCursor)
}

func TestPush4HandleLogKeyGBig(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logLines = []string{"line1", "line2", "line3"}
	result, _ := m.handleLogKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.logCursor)
	assert.True(t, rm.logFollow)
}

func TestPush4HandleLogKeyF(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logFollow = false
	result, _ := m.handleLogKey(keyMsg("f"))
	rm := result.(Model)
	assert.True(t, rm.logFollow)
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
	assert.True(t, rm.logSearchActive)
}

func TestPush4HandleLogKeyVisualMode(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logLines = []string{"line1", "line2"}
	result, _ := m.handleLogKey(keyMsg("v"))
	rm := result.(Model)
	assert.True(t, rm.logVisualMode)
}

func TestPush4HandleLogKeyVisualModeV(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logLines = []string{"line1", "line2"}
	result, _ := m.handleLogKey(keyMsg("V"))
	rm := result.(Model)
	assert.True(t, rm.logVisualMode)
}

func TestPush4HandleLogKeyHalfPageDown(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logLines = make([]string, 100)
	m.logCursor = 0
	kb := ui.ActiveKeybindings
	result, _ := m.handleLogKey(keyMsg(kb.PageDown))
	rm := result.(Model)
	assert.Greater(t, rm.logCursor, 0)
}

func TestPush4HandleLogKeyHalfPageUp(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	m.logLines = make([]string, 100)
	m.logCursor = 50
	kb := ui.ActiveKeybindings
	result, _ := m.handleLogKey(keyMsg(kb.PageUp))
	rm := result.(Model)
	assert.Less(t, rm.logCursor, 50)
}

func TestCovLogKeyHelp(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logLines = []string{"line1", "line2"}
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
	m.logLines = []string{"l1", "l2", "l3", "l4", "l5"}
	m.logCursor = 0
	m.logFollow = false
	result, _ := m.handleLogKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.logCursor)
}

func TestCovLogKeyUp(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logLines = []string{"l1", "l2", "l3"}
	m.logCursor = 2
	m.logFollow = false
	result, _ := m.handleLogKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.logCursor)
}

func TestCovLogKeyToggleFollow(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logFollow = false
	m.logLines = []string{"l1"}
	result, _ := m.handleLogKey(keyMsg("f"))
	rm := result.(Model)
	assert.True(t, rm.logFollow)
}

func TestCovLogKeyDigit(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logLines = []string{"l1"}
	result, _ := m.handleLogKey(keyMsg("5"))
	rm := result.(Model)
	assert.Equal(t, "5", rm.logLineInput)
}

func TestCovLogKeyCtrlF(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logLines = make([]string, 100)
	m.logCursor = 0
	m.logFollow = false
	result, _ := m.handleLogKey(keyMsg("ctrl+f"))
	rm := result.(Model)
	assert.Greater(t, rm.logCursor, 0)
}

func TestCovLogKeyCtrlB(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logLines = make([]string, 100)
	m.logCursor = 50
	m.logFollow = false
	result, _ := m.handleLogKey(keyMsg("ctrl+b"))
	rm := result.(Model)
	assert.Less(t, rm.logCursor, 50)
}

func TestCovLogKeyGG(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logCursor = 3
	m.logLines = []string{"l1", "l2", "l3", "l4"}
	m.logFollow = false
	result, _ := m.handleLogKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)
	result, _ = rm.handleLogKey(keyMsg("g"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.logCursor)
}

func TestCovLogKeyBigG(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logCursor = 0
	m.logLines = []string{"l1", "l2", "l3"}
	m.logFollow = false
	result, _ := m.handleLogKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.logCursor)
}

func TestCovLogKeyCtrlD(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logLines = make([]string, 100)
	m.logCursor = 0
	m.logFollow = false
	result, _ := m.handleLogKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.Greater(t, rm.logCursor, 0)
}

func TestCovLogKeyCtrlU(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logLines = make([]string, 100)
	m.logCursor = 50
	m.logFollow = false
	result, _ := m.handleLogKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.Less(t, rm.logCursor, 50)
}

func TestCovLogKeySlash(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logLines = []string{"l1"}
	result, _ := m.handleLogKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.logSearchActive)
}

func TestCovLogKeyVisualV(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logLines = []string{"l1", "l2"}
	m.logCursor = 0
	m.logFollow = false
	result, _ := m.handleLogKey(keyMsg("V"))
	rm := result.(Model)
	assert.True(t, rm.logVisualMode)
}

func TestCovLogSearchKeyEnter(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logSearchActive = true
	m.logSearchInput.Insert("error")
	m.logLines = []string{"error line", "ok line"}
	result, _ := m.handleLogKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.logSearchActive)
	assert.Equal(t, "error", rm.logSearchQuery)
}

func TestCovLogSearchKeyEsc(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logSearchActive = true
	m.logSearchInput.Insert("test")
	result, _ := m.handleLogKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.logSearchActive)
}

func TestCovLogSearchKeyBackspace(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logSearchActive = true
	m.logSearchInput.Insert("ab")
	m.logLines = []string{"abc"}
	result, _ := m.handleLogKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Equal(t, "a", rm.logSearchInput.Value)
}

func TestCovLogSearchKeyTyping(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logSearchActive = true
	m.logLines = []string{"test"}
	result, _ := m.handleLogKey(keyMsg("x"))
	rm := result.(Model)
	assert.Equal(t, "x", rm.logSearchInput.Value)
}

// Regression: typing into the log-viewer search input now updates
// logSearchQuery alongside logSearchInput so the highlight overlay
// paints in real time. Previously the highlight only appeared once the
// user pressed Enter to "commit" the query.
func TestLogSearchTypingUpdatesQueryLive(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logSearchActive = true
	m.logLines = []string{"some error here"}

	result, _ := m.handleLogKey(keyMsg("e"))
	rm := result.(Model)
	assert.Equal(t, "e", rm.logSearchInput.Value)
	assert.Equal(t, "e", rm.logSearchQuery,
		"logSearchQuery must mirror logSearchInput while typing so highlights paint live")

	result, _ = rm.handleLogKey(keyMsg("r"))
	rm = result.(Model)
	assert.Equal(t, "er", rm.logSearchQuery)

	result, _ = rm.handleLogKey(keyMsg("backspace"))
	rm = result.(Model)
	assert.Equal(t, "e", rm.logSearchQuery,
		"backspace must keep logSearchQuery in sync, not leave the highlight stale")
}

func TestCovLogVisualKeyEsc(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logVisualMode = true
	m.logLines = []string{"l1", "l2"}
	result, _ := m.handleLogKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.logVisualMode)
}

func TestCovLogVisualKeyYank(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logVisualMode = true
	m.logVisualStart = 0
	m.logCursor = 1
	m.logLines = []string{"l1", "l2"}
	_, cmd := m.handleLogKey(keyMsg("y"))
	assert.NotNil(t, cmd)
}

func TestCovLogVisualKeyDown(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logVisualMode = true
	m.logCursor = 0
	m.logLines = []string{"l1", "l2", "l3"}
	result, _ := m.handleLogKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.logCursor)
}

func TestCovLogVisualKeyUp(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logVisualMode = true
	m.logCursor = 2
	m.logLines = []string{"l1", "l2", "l3"}
	result, _ := m.handleLogKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.logCursor)
}

// --- handleLogKeyS2: Save loaded logs (S) ---

func TestHandleLogKeyS2CopiesPathToClipboard(t *testing.T) {
	// Issue #61: when the user presses S to save the loaded log buffer,
	// the destination path should be auto-copied to the system clipboard
	// so it survives the 5s status-clear, and the status message should
	// announce that explicitly.
	m := baseModel()
	m.mode = modeLogs
	m.logLines = []string{"line1", "line2"}
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
	m.logLines = []string{"line1"}

	ret, _ := m.handleLogKeyS2()
	rm := ret.(Model)

	assert.True(t, rm.statusMessageErr, "save failure should set the error flag")
	assert.NotContains(t, rm.statusMessage, "(copied to clipboard)",
		"clipboard suffix only makes sense on success")
}
