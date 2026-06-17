package ui

import (
	"reflect"
	"testing"
)

// TestDefaultKeybindingsNoUnreachableControlAliases guards against a whole
// class of dead bindings: in a terminal, certain control-byte spellings are
// indistinguishable from named keys, so bubbletea never emits the "ctrl+X"
// form — a binding set to one can never fire.
//
//	ctrl+i == tab   (0x09)
//	ctrl+m == enter (0x0d)
//	ctrl+[ == esc   (0x1b)
//
// Only these three are rejected: their bytes map to a DIFFERENT key name in
// bubbletea's table, so the "ctrl+X" spelling is never emitted. ctrl+h
// (0x08->"ctrl+h"), ctrl+j (0x0a->"ctrl+j"), ctrl+@ (0x00->"ctrl+@") and
// ctrl+_ (0x1f->"ctrl+_") ARE emitted under those names and remain valid.
// The original SecurityIgnoreToggle = "ctrl+i" tripped exactly this trap.
func TestDefaultKeybindingsNoUnreachableControlAliases(t *testing.T) {
	unreachable := map[string]string{
		"ctrl+i": "tab",
		"ctrl+m": "enter",
		"ctrl+[": "esc",
	}

	kb := DefaultKeybindings()
	v := reflect.ValueOf(kb)
	for i := range v.NumField() {
		f := v.Field(i)
		if f.Kind() != reflect.String {
			continue
		}
		if named, bad := unreachable[f.String()]; bad {
			t.Errorf("keybinding %s = %q is unreachable in a terminal (collides with %q); use %q or pick another key",
				v.Type().Field(i).Name, f.String(), named, named)
		}
	}
}

// TestLogKeybindingSwap verifies the log key layout: the details-pane live-log
// toggle claims shift+l ("L") and the fullscreen log viewer uses ctrl+l
// (reachable on macOS without Option-as-Meta, unlike an alt+letter binding).
func TestLogKeybindingSwap(t *testing.T) {
	kb := DefaultKeybindings()
	if kb.TogglePreviewLogs != "L" {
		t.Errorf("TogglePreviewLogs = %q, want %q (details-pane live logs uses shift+l)", kb.TogglePreviewLogs, "L")
	}
	if kb.Logs != "ctrl+l" {
		t.Errorf("Logs = %q, want %q (fullscreen log viewer uses ctrl+l)", kb.Logs, "ctrl+l")
	}
}

func TestSeverityAndFollowDefaults(t *testing.T) {
	kb := DefaultKeybindings()
	if kb.ToggleFollow != "F" {
		t.Errorf("ToggleFollow = %q, want %q (f now opens the log filter)", kb.ToggleFollow, "F")
	}
	if kb.Filter != "f" {
		t.Errorf("Filter = %q, want %q", kb.Filter, "f")
	}
	if kb.SeverityDown != "i" {
		t.Errorf("SeverityDown = %q, want %q", kb.SeverityDown, "i")
	}
	if kb.SeverityUp != "o" {
		t.Errorf("SeverityUp = %q, want %q", kb.SeverityUp, "o")
	}
}
