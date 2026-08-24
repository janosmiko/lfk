package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// leaderKey returns the which-key leader keypress for the active bindings.
func leaderKey() tea.KeyPressMsg { return keyMsg(ui.ActiveKeybindings.WhichKeyLeader) }

func TestWhichKeyLeader_DefaultBindingIsQuestionMark(t *testing.T) {
	if got := ui.DefaultKeybindings().WhichKeyLeader; got != "?" {
		t.Fatalf("which_key_leader default = %q, want %q", got, "?")
	}
}

// TestWhichKeyLeader_ArmsWithoutTouchingSelection is the point of moving the
// leader off space: the leader key is a pure overlay trigger with no side
// effect on the list.
func TestWhichKeyLeader_ArmsWithoutTouchingSelection(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := whichKeyTestModel()
	cursorBefore := m.cursor()

	out, _ := m.handleExplorerKey(leaderKey())
	got := out.(Model)
	if !got.whichKey.armed {
		t.Fatal("the leader key must arm the which-key panel")
	}
	if !got.whichKey.shown {
		t.Fatal("at the default zero delay the panel must appear immediately")
	}
	if len(got.selectedItems) != 0 {
		t.Fatalf("the leader key must not select anything; selected=%d", len(got.selectedItems))
	}
	if got.cursor() != cursorBefore {
		t.Fatalf("the leader key must not move the cursor: %d -> %d", cursorBefore, got.cursor())
	}
	if got.mode == modeHelp {
		t.Fatal("in the explorer the leader key must not open the help screen")
	}
}

// TestSpace_TogglesSelectionAndDoesNotArm gives space back: it is plain
// multi-selection again, with no which-key side effect.
func TestSpace_TogglesSelectionAndDoesNotArm(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := whichKeyTestModel()

	out, _ := m.handleExplorerKey(spaceKey())
	got := out.(Model)
	if len(got.selectedItems) != 1 {
		t.Fatalf("space must toggle selection; selected=%d", len(got.selectedItems))
	}
	if got.whichKey.armed || got.whichKey.shown {
		t.Fatal("space must no longer arm the which-key leader")
	}
}

// TestExplorer_F1StillOpensHelp: the leader takes "?" in the explorer, so f1
// is what is left to reach the help screen there.
func TestExplorer_F1StillOpensHelp(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()

	out, _ := m.handleExplorerKey(keyMsg("f1"))
	if out.(Model).mode != modeHelp {
		t.Fatal("f1 must still open the help screen from the explorer")
	}
}

// TestViewers_LeaderArmsThePanelAndF1OpensHelp is the phase-1 inversion of the
// old "viewers keep ? as help" rule. The leader now claims "?" in every mode
// that has a catalog, so the collision resolves the way the explorer already
// resolved it — f1 is the help key everywhere. Routed through handleKey so the
// real dispatch order (handleViewerWhichKeyLeader ahead of handleModeKey) is
// what is under test, not a per-viewer handler in isolation.
func TestViewers_LeaderArmsThePanelAndF1OpensHelp(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0

	for _, mode := range []viewMode{modeYAML, modeLogs, modeDescribe} {
		t.Run(whichKeyModeNames[mode], func(t *testing.T) {
			m := whichKeyTestModel()
			m.mode = mode

			out, _ := m.handleKey(leaderKey())
			armed := out.(Model)
			if !armed.whichKey.armed || !armed.whichKey.shown {
				t.Fatalf("the leader key must arm the panel in %s", whichKeyModeNames[mode])
			}
			if armed.mode == modeHelp {
				t.Fatalf("the leader key must not open help in %s", whichKeyModeNames[mode])
			}

			helped, _ := m.handleKey(keyMsg("f1"))
			if helped.(Model).mode != modeHelp {
				t.Fatalf("f1 must open the help screen from %s", whichKeyModeNames[mode])
			}
		})
	}
}

// TestViewers_QuestionMarkOpensHelpWithTheLeaderRebound is the other half of
// the collision rule inside a viewer: with the leader moved off "?", the key
// reaches the viewer's own kb.Help case again.
func TestViewers_QuestionMarkOpensHelpWithTheLeaderRebound(t *testing.T) {
	restoreWhichKeyGlobals(t)
	kb := ui.DefaultKeybindings()
	kb.WhichKeyLeader = "ctrl+k"
	ui.ActiveKeybindings = kb
	ui.ConfigWhichKeyEnabled = true

	for _, mode := range []viewMode{modeYAML, modeLogs, modeDescribe, modeDiff} {
		name := whichKeyModeNames[mode]
		if name == "" {
			name = "diff"
		}
		t.Run(name, func(t *testing.T) {
			m := whichKeyTestModel()
			m.mode = mode
			out, _ := m.handleKey(keyMsg("?"))
			if out.(Model).mode != modeHelp {
				t.Fatalf("%s: with the leader rebound, ? must open help", name)
			}
		})
	}
}

// TestViewers_LeaderIsLiteralWhileATextInputIsFocused: inside a search or
// filter prompt "?" is a character the user is typing. Each catalog declares
// its own input hook (wkCatalog.input) rather than a switch in the dispatcher,
// so a phase-2 viewer cannot forget the gate and still be wired up.
func TestViewers_LeaderIsLiteralWhileATextInputIsFocused(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true

	cases := []struct {
		name  string
		build func() Model
	}{
		{"yaml search", func() Model {
			m := whichKeyTestModel()
			m.mode = modeYAML
			m.yamlView.searchMode = true
			return m
		}},
		{"log search", func() Model {
			m := whichKeyTestModel()
			m.mode = modeLogs
			m.logView.searchActive = true
			return m
		}},
		{"log filter", func() Model {
			m := whichKeyTestModel()
			m.mode = modeLogs
			m.logView.filterActive = true
			return m
		}},
		{"describe search", func() Model {
			m := whichKeyTestModel()
			m.mode = modeDescribe
			m.describeView.searchActive = true
			return m
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, _ := tc.build().handleKey(leaderKey())
			if got := out.(Model); got.whichKey.armed {
				t.Fatalf("%s: the leader must stay a literal character while the input is focused", tc.name)
			}
		})
	}
}

// TestWhichKeyLeader_RebindingLeaderGivesQuestionMarkBackToHelp pins the
// collision rule: the leader is dispatched ahead of kb.Help, so "?" only stops
// opening help while the leader is actually bound to it.
//
// The hint bar is asserted alongside the dispatch, because the two used to
// disagree: the bar handed its "help" slot to the leader key unconditionally,
// so a rebound leader ("z") was advertised as help while "z" opened the panel
// and "?" opened help — the bar named a key that did something else AND hid
// the key that did the job.
func TestWhichKeyLeader_RebindingLeaderGivesQuestionMarkBackToHelp(t *testing.T) {
	restoreWhichKeyGlobals(t)
	kb := ui.DefaultKeybindings()
	kb.WhichKeyLeader = "ctrl+k"
	ui.ActiveKeybindings = kb
	ui.ConfigWhichKeyEnabled = true

	m := whichKeyTestModel()
	out, _ := m.handleExplorerKey(keyMsg("?"))
	if out.(Model).mode != modeHelp {
		t.Fatal("with the leader rebound, ? must open help in the explorer again")
	}

	m2 := whichKeyTestModel()
	out2, _ := m2.handleExplorerKey(keyMsg("ctrl+k"))
	if !out2.(Model).whichKey.armed {
		t.Fatal("the rebound leader key must arm the panel")
	}

	if got := helpHintKey(kb); got != "?" {
		t.Fatalf("hint bar advertises %q as help; with the leader rebound the key that opens help is %q", got, "?")
	}
}

// TestExplorerHelpHintKey_MatchesTheKeyThatOpensHelp is the bar's half of the
// collision rule: it must name whichever key actually reaches the help screen,
// which is exactly whichKeyHelpKey's answer (the panel's own "Full help" row
// renders from that function, and the two sat one row apart contradicting each
// other).
func TestExplorerHelpHintKey_MatchesTheKeyThatOpensHelp(t *testing.T) {
	restoreWhichKeyGlobals(t)

	cases := []struct {
		name    string
		leader  string
		help    string
		enabled bool
		want    string
	}{
		// Defaults collide on "?", so f1 is the only key left that opens help.
		{"default collision", "?", "?", true, "f1"},
		// Rebound leader: "?" reaches help again, and the bar must say so.
		{"leader rebound", "z", "?", true, "?"},
		// Help rebound off the leader: same story from the other side.
		{"help rebound", "?", "f2", true, "f2"},
		// Panel disabled: the leader key does nothing, help keeps "?".
		{"panel disabled", "?", "?", false, "?"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kb := ui.DefaultKeybindings()
			kb.WhichKeyLeader = tc.leader
			kb.Help = tc.help
			ui.ActiveKeybindings = kb
			ui.ConfigWhichKeyEnabled = tc.enabled

			if got := helpHintKey(kb); got != tc.want {
				t.Errorf("helpHintKey = %q, want %q", got, tc.want)
			}
			if tc.enabled {
				if got := whichKeyHelpKey(kb); got != tc.want {
					t.Errorf("whichKeyHelpKey = %q, want %q (the bar and the panel must agree)", got, tc.want)
				}
			}
		})
	}
}

// TestExplorerHintBar_AdvertisesTheWorkingHelpKey drives the assertion through
// the rendered bar rather than the helper, so a future caller that bypasses
// helpHintKey is still caught.
func TestExplorerHintBar_AdvertisesTheWorkingHelpKey(t *testing.T) {
	restoreWhichKeyGlobals(t)
	kb := ui.DefaultKeybindings()
	kb.WhichKeyLeader = "z"
	kb.Help = "?"
	ui.ActiveKeybindings = kb
	ui.ConfigWhichKeyEnabled = true

	m := whichKeyTestModel()
	// 220, not 200: the "~: cpu/mem view" hint now occupies part of that budget.
	m.width, m.height = 220, 24
	bar := stripANSI(m.statusBar())
	if !strings.Contains(bar, "?: help") {
		t.Errorf("hint bar must advertise the key that opens help (%q):\n%s", "?: help", bar)
	}
	if strings.Contains(bar, "z: help") {
		t.Errorf("hint bar must not label the leader key as help:\n%s", bar)
	}
}

// TestWhichKeyLeader_ScrollingLeavesTheListAlone: with the leader off space,
// scrolling the panel is purely a view operation — in particular ctrl+d must
// move the PANEL, not the explorer cursor, while the panel is up.
func TestWhichKeyLeader_ScrollingLeavesTheListAlone(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := whichKeyTestModel()
	m.width, m.height = 80, 24
	m.setMiddleItems([]model.Item{
		{Name: "p1", Kind: "Pod", Namespace: "default"},
		{Name: "p2", Kind: "Pod", Namespace: "default"},
		{Name: "p3", Kind: "Pod", Namespace: "default"},
	})
	m.setCursor(0)

	out, _ := m.handleExplorerKey(leaderKey())
	m = out.(Model)
	for i := range 3 {
		out, _ = m.handleExplorerKey(keyMsg(ui.ActiveKeybindings.PageDown))
		m = out.(Model)
		if len(m.selectedItems) != 0 {
			t.Fatalf("scroll %d: must not change the selection; selected=%d", i+1, len(m.selectedItems))
		}
		if m.cursor() != 0 {
			t.Fatalf("scroll %d: must not move the explorer cursor; cursor=%d", i+1, m.cursor())
		}
	}
	if m.whichKey.scroll == 0 {
		t.Fatal("three ctrl+d presses must have scrolled the panel")
	}
}
