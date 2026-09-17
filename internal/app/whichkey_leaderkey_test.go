package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/ui"
)

// leaderKey returns the which-key leader keypress for the active bindings.
func leaderKey() tea.KeyPressMsg { return keyMsg(ui.ActiveKeybindings.WhichKeyLeader) }

func TestWhichKeyLeader_DefaultBindingIsQuestionMark(t *testing.T) {
	if got := ui.DefaultKeybindings().WhichKeyLeader; got != "?" {
		t.Fatalf("which_key_leader default = %q, want %q", got, "?")
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

// TestExplorerHelpHintKey_MatchesTheKeyThatOpensHelp is the bar's half of the
// collision rule: it must name whichever key actually reaches the help screen,
// which is exactly whichKeyHelpKey's answer.
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
	// 220, not 200: whichKeyTestModel sits on a Pod row, where "~: cpu/mem
	// view" legitimately renders, and the hint occupies part of that budget.
	m.width, m.height = 220, 24
	bar := stripANSI(m.statusBar())
	if !strings.Contains(bar, "?: help") {
		t.Errorf("hint bar must advertise the key that opens help (%q):\n%s", "?: help", bar)
	}
	if strings.Contains(bar, "z: help") {
		t.Errorf("hint bar must not label the leader key as help:\n%s", bar)
	}
}
