package ui

import (
	"reflect"
	"strings"
	"testing"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

// c0EmittedKeys is what Bubble Tea v2 emits for each C0 control byte, mirroring
// the C0 block of ultraviolet's buildKeysTable (key_table.go) under this app's
// encoding. lfk never calls LegacyKeyEncoding, so the CtrlAt / CtrlI / CtrlM /
// CtrlOpenBracket opt-ins are off and 0x00, 0x09, 0x0d, 0x1b decode to space,
// tab, enter and esc rather than to ctrl+@, ctrl+i, ctrl+m, ctrl+[.
var c0EmittedKeys = map[byte]tea.KeyPressMsg{
	0x00: {Code: tea.KeySpace, Mod: tea.ModCtrl},
	0x01: {Code: 'a', Mod: tea.ModCtrl}, 0x02: {Code: 'b', Mod: tea.ModCtrl},
	0x03: {Code: 'c', Mod: tea.ModCtrl}, 0x04: {Code: 'd', Mod: tea.ModCtrl},
	0x05: {Code: 'e', Mod: tea.ModCtrl}, 0x06: {Code: 'f', Mod: tea.ModCtrl},
	0x07: {Code: 'g', Mod: tea.ModCtrl}, 0x08: {Code: 'h', Mod: tea.ModCtrl},
	0x09: {Code: tea.KeyTab},
	0x0a: {Code: 'j', Mod: tea.ModCtrl}, 0x0b: {Code: 'k', Mod: tea.ModCtrl},
	0x0c: {Code: 'l', Mod: tea.ModCtrl},
	0x0d: {Code: tea.KeyEnter},
	0x0e: {Code: 'n', Mod: tea.ModCtrl}, 0x0f: {Code: 'o', Mod: tea.ModCtrl},
	0x10: {Code: 'p', Mod: tea.ModCtrl}, 0x11: {Code: 'q', Mod: tea.ModCtrl},
	0x12: {Code: 'r', Mod: tea.ModCtrl}, 0x13: {Code: 's', Mod: tea.ModCtrl},
	0x14: {Code: 't', Mod: tea.ModCtrl}, 0x15: {Code: 'u', Mod: tea.ModCtrl},
	0x16: {Code: 'v', Mod: tea.ModCtrl}, 0x17: {Code: 'w', Mod: tea.ModCtrl},
	0x18: {Code: 'x', Mod: tea.ModCtrl}, 0x19: {Code: 'y', Mod: tea.ModCtrl},
	0x1a: {Code: 'z', Mod: tea.ModCtrl},
	0x1b: {Code: tea.KeyEscape},
	0x1c: {Code: '\\', Mod: tea.ModCtrl}, 0x1d: {Code: ']', Mod: tea.ModCtrl},
	0x1e: {Code: '^', Mod: tea.ModCtrl}, 0x1f: {Code: '_', Mod: tea.ModCtrl},
}

// emittedNameForCtrlChord reports the key name Bubble Tea actually prints when
// the chord spelled by binding is typed, for the bindings that are a real C0
// chord: "ctrl+" plus one of @ A-Z [ \ ] ^ _ (case-insensitive), which a
// terminal encodes as the single byte c&0x1f. ok is false for anything else
// (ctrl+alt+y, ctrl+f5, named keys) — those are not C0 chords and this guard
// says nothing about them.
func emittedNameForCtrlChord(binding string) (string, bool) {
	rest, isCtrl := strings.CutPrefix(binding, "ctrl+")
	if !isCtrl {
		return "", false
	}
	r := []rune(rest)
	if len(r) != 1 {
		return "", false
	}
	u := unicode.ToUpper(r[0])
	if u < '@' || u > '_' {
		return "", false
	}
	emitted, ok := c0EmittedKeys[byte(u)&0x1f]
	return emitted.String(), ok
}

// TestDefaultKeybindingsNoUnreachableControlAliases guards a whole class of
// dead bindings: a default whose spelling no real keypress can produce. Several
// C0 control bytes decode to a NAMED key rather than to the "ctrl+X" form that
// produced them, so a binding written in the "ctrl+X" spelling never fires —
//
//	ctrl+@ == ctrl+space (0x00)
//	ctrl+i == tab        (0x09)
//	ctrl+m == enter      (0x0d)
//	ctrl+[ == esc        (0x1b)
//
// Rather than listing those four, every "ctrl+<char>" default is round-tripped
// through the byte a terminal sends for it and compared against the name Bubble
// Tea prints back. SecurityIgnoreToggle = "ctrl+i" and SelectRange = "ctrl+@"
// both tripped this trap; the generic form catches the next one too.
func TestDefaultKeybindingsNoUnreachableControlAliases(t *testing.T) {
	kb := DefaultKeybindings()
	v := reflect.ValueOf(kb)
	for i := range v.NumField() {
		f := v.Field(i)
		if f.Kind() != reflect.String {
			continue
		}
		emitted, checked := emittedNameForCtrlChord(f.String())
		if checked && emitted != f.String() {
			t.Errorf("keybinding %s = %q is unreachable in a terminal (that chord is emitted as %q); use %q or pick another key",
				v.Type().Field(i).Name, f.String(), emitted, emitted)
		}
	}
}

// TestEmittedNameForCtrlChord_FiresOnKnownDeadSpellings validates the detector
// itself before the guard above is trusted: a check that reports "all clear"
// proves nothing until it is shown to fire on known-bad input.
func TestEmittedNameForCtrlChord_FiresOnKnownDeadSpellings(t *testing.T) {
	dead := map[string]string{
		"ctrl+@": "ctrl+space",
		"ctrl+i": "tab",
		"ctrl+I": "tab",
		"ctrl+m": "enter",
		"ctrl+[": "esc",
	}
	for binding, want := range dead {
		emitted, ok := emittedNameForCtrlChord(binding)
		if !ok {
			t.Errorf("%q: detector skipped a real C0 chord", binding)
			continue
		}
		if emitted != want {
			t.Errorf("%q: emitted %q, want %q", binding, emitted, want)
		}
	}

	// Reachable chords must NOT be flagged, or the guard is unusable.
	for _, binding := range []string{"ctrl+a", "ctrl+h", "ctrl+j", "ctrl+_", "ctrl+\\"} {
		emitted, ok := emittedNameForCtrlChord(binding)
		if !ok {
			t.Errorf("%q: detector skipped a real C0 chord", binding)
			continue
		}
		if emitted != binding {
			t.Errorf("%q: wrongly flagged as unreachable (emitted %q)", binding, emitted)
		}
	}

	// Non-C0 spellings are out of scope and must be reported as unchecked.
	for _, binding := range []string{"ctrl+space", "ctrl+alt+y", "ctrl+f5", "j", ""} {
		if _, ok := emittedNameForCtrlChord(binding); ok {
			t.Errorf("%q: detector claimed to check a non-C0 spelling", binding)
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
