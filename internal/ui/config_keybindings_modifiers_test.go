package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
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
