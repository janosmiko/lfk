package ui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

// Ctrl+Shift chords only became distinguishable from plain Ctrl chords once
// the terminal reports them separately, which Bubble Tea v2 requests on every
// render. These tests pin the three places a binding has to line up: the
// keystroke the terminal reports, the spelling stored in config, and the
// label shown in help.

func TestKeystrokeStringsForModifierChords(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyPressMsg
		want string
	}{
		// What a real terminal delivers: the shifted codepoint in Code.
		{"ctrl+shift+letter", tea.KeyPressMsg{Code: 'X', Mod: tea.ModCtrl | tea.ModShift}, "ctrl+shift+X"},
		{"shift alone is the bare character", tea.KeyPressMsg{Code: 'x', Text: "X", Mod: tea.ModShift}, "X"},
		{"plain ctrl stays distinct", tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl}, "ctrl+x"},
		{"ctrl+alt+shift", tea.KeyPressMsg{Code: 'Y', Mod: tea.ModCtrl | tea.ModAlt | tea.ModShift}, "ctrl+alt+shift+Y"},
		{"shift+tab", tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}, "shift+tab"},
		{"space reports its name", tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}, "space"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.key.String())
		})
	}
}

func TestNormalizeKeybinding(t *testing.T) {
	tests := []struct {
		name  string
		given string
		want  string
	}{
		// A terminal reports the SHIFTED character for a shift chord, so the
		// natural lowercase spelling has to be folded up to match.
		{"shift chord uppercases the letter", "ctrl+shift+x", "ctrl+shift+X"},
		{"already uppercase is untouched", "ctrl+shift+X", "ctrl+shift+X"},
		{"reorders and uppercases", "shift+ctrl+x", "ctrl+shift+X"},
		// Shift alone produces the bare uppercase character, with no modifier.
		{"shift-only letter collapses", "shift+x", "X"},
		{"shift-only letter already upper", "shift+X", "X"},
		// Named keys keep their shift modifier.
		{"shift+named key keeps modifier", "shift+tab", "shift+tab"},
		{"ctrl+shift+named key", "ctrl+shift+tab", "ctrl+shift+tab"},
		{"reorders legacy alt+ctrl", "alt+ctrl+y", "ctrl+alt+y"},
		{"full chord ordering", "shift+alt+ctrl+y", "ctrl+alt+shift+Y"},
		{"literal space becomes its name", " ", "space"},
		{"plain key untouched", "j", "j"},
		{"single modifier untouched", "ctrl+d", "ctrl+d"},
		{"punctuation key survives", "ctrl+@", "ctrl+@"},
		{"plus as the key survives", "ctrl++", "ctrl++"},
		{"unknown modifier left alone", "hyperctrl+x", "hyperctrl+x"},
		// A trailing plus with nothing after it is a typo, not the "+" key.
		// Rewriting "ctrl+alt+" into "ctrl++" would turn a malformed binding
		// into a DIFFERENT working one, which is the silent reordering this
		// function exists to avoid.
		{"trailing plus is not the plus key", "ctrl+", "ctrl+"},
		{"trailing plus after two modifiers", "ctrl+alt+", "ctrl+alt+"},
		{"trailing plus after a non-modifier", "ga+", "ga+"},
		// The three above pass for reasons that are not the rule: one modifier
		// needs no reordering, "ctrl+alt+" is already canonical, and "ga" is an
		// unknown modifier that early-returns. Only a malformed binding whose
		// modifiers are BOTH valid and out of order reaches the reordering
		// path, which used to hand back "ctrl+alt+".
		{"malformed input is not reordered either", "alt+ctrl+", "alt+ctrl+"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NormalizeKeybinding(tt.given))
		})
	}
}

// A binding a user writes in config must equal the string the terminal
// reports, or the key silently never fires.
func TestConfiguredChordMatchesReportedKeystroke(t *testing.T) {
	dst := DefaultKeybindings()
	src := Keybindings{ForceDelete: "shift+ctrl+x"} // deliberately non-canonical
	MergeKeybindings(&dst, &src)

	pressed := tea.KeyPressMsg{Code: 'X', Mod: tea.ModCtrl | tea.ModShift}
	assert.Equal(t, "ctrl+shift+X", dst.ForceDelete, "merge must canonicalise the stored spelling")
	assert.Equal(t, dst.ForceDelete, pressed.String(), "stored binding must match the reported keystroke")
}

func TestMergeKeybindingsNormalizesLegacySpace(t *testing.T) {
	dst := DefaultKeybindings()
	src := Keybindings{ToggleSelect: " "}
	MergeKeybindings(&dst, &src)

	pressed := tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	assert.Equal(t, "space", dst.ToggleSelect)
	assert.Equal(t, dst.ToggleSelect, pressed.String())
}

func TestHelpKeyDisplayFormatsModifiers(t *testing.T) {
	tests := []struct {
		given string
		want  string
	}{
		{"ctrl+shift+X", "Ctrl+Shift+X"},
		{"ctrl+alt+y", "Ctrl+Alt+Y"},
		{"shift+tab", "Shift+Tab"},
		{"ctrl+d", "Ctrl+D"},
		{"space", "space"},
		{"j", "j"},
	}
	for _, tt := range tests {
		t.Run(tt.given, func(t *testing.T) {
			assert.Equal(t, tt.want, helpKeyDisplay(tt.given))
		})
	}
}

// helpKeyDisplay must not panic or split a multibyte key mid-rune.
func TestHelpKeyDisplayEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		given string
		want  string
	}{
		{"plus as the key is left verbatim", "ctrl++", "ctrl++"},
		{"multibyte key is not split mid-rune", "ctrl+é", "Ctrl+É"},
		{"multibyte named key", "ctrl+ñabc", "Ctrl+Ñabc"},
		{"unknown modifier left verbatim", "hyperctrl+x", "hyperctrl+x"},
		{"bare plus", "+", "+"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() { helpKeyDisplay(tt.given) })
			assert.Equal(t, tt.want, helpKeyDisplay(tt.given))
		})
	}
}

// FullscreenBorderStyle must render at exactly the requested outer size.
// lipgloss v2 counts the border inside Width/Height, so the pre-v2 arithmetic
// silently produced a box two cells narrow and two rows short.
func TestFullscreenBorderStyleFillsRequestedBox(t *testing.T) {
	for _, tc := range []struct{ w, h int }{{40, 10}, {80, 24}, {120, 30}} {
		t.Run(fmt.Sprintf("%dx%d", tc.w, tc.h), func(t *testing.T) {
			out := FullscreenBorderStyle(tc.w, tc.h).Render("hello")
			assert.Equal(t, tc.w, lipgloss.Width(out), "rendered width must match the requested width")
			assert.Equal(t, tc.h+2, lipgloss.Height(out), "rendered height must be the content height plus both border rows")
		})
	}
}
