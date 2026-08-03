package app

import (
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

// TestViewers_QuestionMarkStillOpensHelp scopes the leader to the explorer:
// every fullscreen viewer keeps "?" as help, exactly as before.
func TestViewers_QuestionMarkStillOpensHelp(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	help := keyMsg("?")

	cases := []struct {
		name string
		run  func() Model
	}{
		{"describe", func() Model {
			m := whichKeyTestModel()
			m.mode = modeDescribe
			out, _ := m.handleDescribeKey(help)
			return out.(Model)
		}},
		{"logs", func() Model {
			m := whichKeyTestModel()
			m.mode = modeLogs
			out, _ := m.handleLogKey(help)
			return out.(Model)
		}},
		{"yaml", func() Model {
			m := whichKeyTestModel()
			m.mode = modeYAML
			out, _ := m.handleYAMLKey(help)
			return out.(Model)
		}},
		{"diff", func() Model {
			m := whichKeyTestModel()
			m.mode = modeDiff
			out, _ := m.handleDiffKey(help)
			return out.(Model)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.run(); got.mode != modeHelp {
				t.Fatalf("%s viewer: ? must still open help, got mode %v", tc.name, got.mode)
			}
		})
	}
}

// TestWhichKeyLeader_RebindingLeaderGivesQuestionMarkBackToHelp pins the
// collision rule: the leader is dispatched ahead of kb.Help, so "?" only stops
// opening help while the leader is actually bound to it.
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
}

// TestWhichKeyLeader_PagingLeavesSelectionAlone: with the leader off space,
// paging is purely a view operation.
func TestWhichKeyLeader_PagingLeavesSelectionAlone(t *testing.T) {
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

	for i := range 3 {
		out, _ := m.handleExplorerKey(leaderKey())
		m = out.(Model)
		if len(m.selectedItems) != 0 {
			t.Fatalf("press %d: paging must not change the selection; selected=%d", i+1, len(m.selectedItems))
		}
		if m.cursor() != 0 {
			t.Fatalf("press %d: paging must not move the cursor; cursor=%d", i+1, m.cursor())
		}
	}
	if m.whichKey.page != 2 {
		t.Fatalf("three leader presses must land on page index 2, got %d", m.whichKey.page)
	}
}
