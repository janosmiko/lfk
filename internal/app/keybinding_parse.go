package app

import (
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// parseKeyBinding builds the tea.KeyPressMsg a real keypress produces for a
// config keybinding string, for replay through msg.String() dispatch. A
// compound binding ("j/Down") uses only its first alternative.
func parseKeyBinding(s string) tea.KeyPressMsg {
	if idx := strings.IndexByte(s, '/'); idx > 0 {
		s = s[:idx]
	}

	parts := strings.Split(s, "+")
	key := parts[len(parts)-1]

	var mod tea.KeyMod
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "ctrl":
			mod |= tea.ModCtrl
		case "alt":
			mod |= tea.ModAlt
		case "shift":
			mod |= tea.ModShift
		}
	}

	if code, ok := specialKeyCode(key); ok {
		return tea.KeyPressMsg{Code: code, Mod: mod}
	}

	r, _ := utf8.DecodeRuneInString(key)
	k := tea.KeyPressMsg{Code: r, Mod: mod}
	if mod == 0 {
		k.Text = key
	}
	return k
}

// specialKeyCode maps a lowercase key name to the tea.Key* constant it
// stands for. It reports false for anything that is a plain character
// keypress rather than a named key.
func specialKeyCode(s string) (rune, bool) {
	switch s {
	case "enter":
		return tea.KeyEnter, true
	case "space":
		return tea.KeySpace, true
	case "esc":
		return tea.KeyEscape, true
	case "tab":
		return tea.KeyTab, true
	case "backspace":
		return tea.KeyBackspace, true
	case "up":
		return tea.KeyUp, true
	case "down":
		return tea.KeyDown, true
	case "left":
		return tea.KeyLeft, true
	case "right":
		return tea.KeyRight, true
	case "pgup":
		return tea.KeyPgUp, true
	case "pgdown":
		return tea.KeyPgDown, true
	case "home":
		return tea.KeyHome, true
	case "end":
		return tea.KeyEnd, true
	case "delete":
		return tea.KeyDelete, true
	}
	// tea.KeyF1..KeyF63 are consecutive constants, so f-keys are computed
	// from the number rather than listed one by one.
	if n, ok := fKeyNumber(s); ok {
		return tea.KeyF1 + rune(n-1), true
	}
	return 0, false
}

func fKeyNumber(s string) (int, bool) {
	if len(s) < 2 || s[0] != 'f' {
		return 0, false
	}
	n, err := strconv.Atoi(s[1:])
	if err != nil || n < 1 || n > 63 {
		return 0, false
	}
	return n, true
}
