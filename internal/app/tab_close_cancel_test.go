package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// recordCancelFn returns a context.CancelFunc-shaped closure that pushes a
// label onto the supplied slice when invoked. Lets tests assert which cancel
// funcs were actually called without depending on real contexts/subprocesses.
func recordCancelFn(label string, log *[]string) func() {
	return func() { *log = append(*log, label) }
}

// modelWithLogCancels builds a 3-tab Model where the active tab has both
// stream and history cancels populated, and each inactive tab has its
// per-tab logCancel populated. (TabState only carries logCancel today;
// logHistoryCancel lives only on Model — see app.go.) Mirrors the real
// shape after a user has streamed logs in several tabs and switched
// between them.
func modelWithLogCancels(t *testing.T, calls *[]string) Model {
	t.Helper()
	tabs := []TabState{
		{
			nav:       model.NavigationState{Context: "prod", Level: model.LevelResources},
			logCancel: recordCancelFn("tab0-stream", calls),
		},
		{
			nav:       model.NavigationState{Context: "staging", Level: model.LevelResources},
			logCancel: recordCancelFn("tab1-stream", calls),
		},
		{
			nav:       model.NavigationState{Context: "dev", Level: model.LevelResources},
			logCancel: recordCancelFn("tab2-stream", calls),
		},
	}
	return Model{
		tabs:               tabs,
		activeTab:          1,
		nav:                tabs[1].nav,
		logCancel:          recordCancelFn("active-stream", calls),
		logHistoryCancel:   recordCancelFn("active-history", calls),
		width:              120,
		height:             30,
		mode:               modeExplorer,
		selectedItems:      make(map[string]bool),
		cursorMemory:       make(map[string]int),
		itemCache:          make(map[string][]model.Item),
		yamlCollapsed:      make(map[string]bool),
		selectedNamespaces: make(map[string]bool),
		selectionAnchor:    -1,
	}
}

// --- cancelAllTabLogStreams: covers active + all per-tab cancels ---

func TestCancelAllTabLogStreams_CancelsEveryStreamAndHistory(t *testing.T) {
	var calls []string
	m := modelWithLogCancels(t, &calls)

	m.cancelAllTabLogStreams()

	// Every cancel func should have fired exactly once. The order is not
	// load-bearing, just the set.
	assert.ElementsMatch(t,
		[]string{
			"active-stream", "active-history",
			"tab0-stream",
			"tab1-stream",
			"tab2-stream",
		},
		calls,
		"cancelAllTabLogStreams must cancel active stream + history AND every per-tab logCancel")

	// All cancel pointers must be nil after calling so a second invocation
	// is a no-op (idempotent).
	assert.Nil(t, m.logCancel)
	assert.Nil(t, m.logHistoryCancel)
	for i := range m.tabs {
		assert.Nilf(t, m.tabs[i].logCancel, "tab %d logCancel must be nil after helper", i)
	}
}

func TestCancelAllTabLogStreams_IsIdempotent(t *testing.T) {
	var calls []string
	m := modelWithLogCancels(t, &calls)

	m.cancelAllTabLogStreams()
	m.cancelAllTabLogStreams() // second call must not double-fire any cancel

	assert.Len(t, calls, 5, "second invocation must not re-fire any cancel")
}

// --- :quit / :q command-bar path must cancel log streams ---

func TestQuitBuiltinCommandCancelsAllLogStreams(t *testing.T) {
	tests := []string{"quit", "q", "q!"}
	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			var calls []string
			m := modelWithLogCancels(t, &calls)

			_, teaCmd := m.executeBuiltinCommand(cmd)

			assert.NotNil(t, teaCmd, ":%s should return a tea.Quit command", cmd)
			// Whatever the order, every stream/history cancel must have fired.
			assert.ElementsMatch(t,
				[]string{
					"active-stream", "active-history",
					"tab0-stream",
					"tab1-stream",
					"tab2-stream",
				},
				calls,
				":%s must cancel every active and per-tab log stream and history fetch — issue #48 leak path", cmd)
		})
	}
}

// --- Tab-close branch only cancels the closing tab's streams ---
// Closing ONE tab when multiple exist must not kill streams in the OTHER
// tabs. The user only asked to close one tab — sibling streams continue.

func TestCloseTabOrQuit_LeavesOtherTabsStreamsRunning(t *testing.T) {
	var calls []string
	m := modelWithLogCancels(t, &calls)
	// activeTab is 1 (staging). Closing it must cancel ONLY the active
	// tab's cancels, not tab 0 or tab 2.

	_, _ = m.closeTabOrQuit()

	assert.ElementsMatch(t,
		[]string{"active-stream", "active-history"},
		calls,
		"tab close must cancel only the active tab's stream + history; sibling tabs keep streaming")
}

// --- Single-tab quit path (no confirm) cancels everything ---

func TestCloseTabOrQuit_NoConfirmQuitCancelsAllStreams(t *testing.T) {
	orig := ui.ConfigConfirmOnExit
	ui.ConfigConfirmOnExit = false
	t.Cleanup(func() { ui.ConfigConfirmOnExit = orig })

	var calls []string
	m := Model{
		tabs: []TabState{{
			nav:       model.NavigationState{Context: "prod", Level: model.LevelResources},
			logCancel: recordCancelFn("tab0-stream", &calls),
		}},
		activeTab:          0,
		nav:                model.NavigationState{Context: "prod", Level: model.LevelResources},
		logCancel:          recordCancelFn("active-stream", &calls),
		logHistoryCancel:   recordCancelFn("active-history", &calls),
		width:              120,
		height:             30,
		selectedItems:      make(map[string]bool),
		cursorMemory:       make(map[string]int),
		itemCache:          make(map[string][]model.Item),
		yamlCollapsed:      make(map[string]bool),
		selectedNamespaces: make(map[string]bool),
	}

	_, teaCmd := m.closeTabOrQuit()
	assert.NotNil(t, teaCmd, "single-tab close with confirm-off must return tea.Quit")
	assert.ElementsMatch(t,
		[]string{
			"active-stream", "active-history",
			"tab0-stream",
		},
		calls,
		"no-confirm quit must cancel everything (active + per-tab)")
}
