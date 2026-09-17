package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
)

func TestParseKeyBinding_SimpleChars(t *testing.T) {
	for _, s := range []string{"j", "/", "."} {
		got := parseKeyBinding(s)
		assert.Equal(t, s, got.String(), "round trip for %q", s)
		assert.Equal(t, s, got.Text)
		assert.Zero(t, got.Mod)
	}
}

func TestParseKeyBinding_Uppercase(t *testing.T) {
	for _, s := range []string{"G", "Y"} {
		got := parseKeyBinding(s)
		assert.Equal(t, s, got.String(), "round trip for %q", s)
		assert.Equal(t, s, got.Text)
		assert.Zero(t, got.Mod)
	}
}

func TestParseKeyBinding_CtrlCombo(t *testing.T) {
	got := parseKeyBinding("ctrl+d")
	assert.Equal(t, "ctrl+d", got.String())
	assert.Equal(t, tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}, got)
}

func TestParseKeyBinding_AltCombo(t *testing.T) {
	got := parseKeyBinding("alt+d")
	assert.Equal(t, "alt+d", got.String())
	assert.Equal(t, tea.KeyPressMsg{Code: 'd', Mod: tea.ModAlt}, got)
}

func TestParseKeyBinding_MultiModifierCombo(t *testing.T) {
	got := parseKeyBinding("ctrl+alt+y")
	assert.Equal(t, "ctrl+alt+y", got.String())
	assert.Equal(t, tea.ModCtrl|tea.ModAlt, got.Mod)
	assert.Equal(t, rune('y'), got.Code)
}

func TestParseKeyBinding_ShiftCombo(t *testing.T) {
	got := parseKeyBinding("shift+tab")
	assert.Equal(t, "shift+tab", got.String())
	assert.Equal(t, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}, got)
}

func TestParseKeyBinding_ModifierWithSpecialKey(t *testing.T) {
	got := parseKeyBinding("ctrl+space")
	assert.Equal(t, "ctrl+space", got.String())
	assert.Equal(t, tea.KeyPressMsg{Code: tea.KeySpace, Mod: tea.ModCtrl}, got)
}

func TestParseKeyBinding_SpecialKeys(t *testing.T) {
	cases := map[string]rune{
		"enter":     tea.KeyEnter,
		"space":     tea.KeySpace,
		"esc":       tea.KeyEscape,
		"tab":       tea.KeyTab,
		"backspace": tea.KeyBackspace,
		"up":        tea.KeyUp,
		"down":      tea.KeyDown,
		"left":      tea.KeyLeft,
		"right":     tea.KeyRight,
		"pgup":      tea.KeyPgUp,
		"pgdown":    tea.KeyPgDown,
		"home":      tea.KeyHome,
		"end":       tea.KeyEnd,
		"delete":    tea.KeyDelete,
	}
	for s, code := range cases {
		got := parseKeyBinding(s)
		assert.Equal(t, s, got.String(), "round trip for %q", s)
		assert.Equal(t, tea.KeyPressMsg{Code: code}, got)
	}
}

func TestParseKeyBinding_FKeys(t *testing.T) {
	for _, s := range []string{"f1", "f2", "f9", "f10", "f12", "f63"} {
		got := parseKeyBinding(s)
		assert.Equal(t, s, got.String(), "round trip for %q", s)
	}
	assert.Equal(t, tea.KeyF1, parseKeyBinding("f1").Code)
	assert.Equal(t, tea.KeyF12, parseKeyBinding("f12").Code)
	assert.Equal(t, tea.KeyF63, parseKeyBinding("f63").Code)
}

func TestParseKeyBinding_CompoundUsesFirstAlternative(t *testing.T) {
	got := parseKeyBinding("j/Down")
	assert.Equal(t, "j", got.String())
	assert.Equal(t, parseKeyBinding("j"), got)
}

func TestSpecialKeyCode_KnownNames(t *testing.T) {
	cases := map[string]rune{
		"enter": tea.KeyEnter, "space": tea.KeySpace, "esc": tea.KeyEscape,
		"tab": tea.KeyTab, "backspace": tea.KeyBackspace,
		"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
		"pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown, "home": tea.KeyHome, "end": tea.KeyEnd,
		"delete": tea.KeyDelete, "f1": tea.KeyF1, "f63": tea.KeyF63,
	}
	for name, want := range cases {
		got, ok := specialKeyCode(name)
		assert.True(t, ok, "expected %q to be a special key", name)
		assert.Equal(t, want, got, "code for %q", name)
	}
}

func TestSpecialKeyCode_UnknownNames(t *testing.T) {
	for _, s := range []string{"d", "j", "G", "1", "f0", "f64", "f", "fx", ""} {
		_, ok := specialKeyCode(s)
		assert.False(t, ok, "expected %q to not be a special key", s)
	}
}
