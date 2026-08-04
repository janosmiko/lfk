package ui

import (
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
)

// helpKeyDisplayModifiers is the display casing for each modifier name.
var helpKeyDisplayModifiers = map[string]string{
	"ctrl": "Ctrl", "alt": "Alt", "shift": "Shift",
	"meta": "Meta", "hyper": "Hyper", "super": "Super",
}

// isChordRune reports whether r can be part of a chord token. Everything
// else (spaces, slashes, angle brackets, parentheses) separates tokens and
// is copied through untouched, so a composite catalog key like
// "ctrl+d/ctrl+u" or "m<a-z/0-9>" keeps its exact shape.
func isChordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '+' || r == '_' || r == '[' || r == ']'
}

// mapChordTokens rewrites every chord-shaped token in key using f, leaving
// the separators between tokens verbatim. Catalog keys are not always a
// single binding ("ctrl+d/ctrl+u", "ctrl+] g/G"), so formatting has to work
// token-wise rather than on the whole string.
func mapChordTokens(key string, f func(string) string) string {
	var sb strings.Builder
	sb.Grow(len(key))
	start := -1
	flush := func(end int) {
		if start < 0 {
			return
		}
		sb.WriteString(f(key[start:end]))
		start = -1
	}
	for i, r := range key {
		if isChordRune(r) {
			if start < 0 {
				start = i
			}
			continue
		}
		flush(i)
		sb.WriteRune(r)
	}
	flush(len(key))
	return sb.String()
}

// helpKeyDisplay formats a keybinding value as text for the help screen.
// A modified chord gets its modifiers capitalized and its key uppercased
// ("ctrl+shift+x" -> "Ctrl+Shift+X", "shift+tab" -> "Shift+Tab"); anything
// that is not a chord displays verbatim.
//
// This is also the form the help screen's search index is built from, so a
// query of "ctrl" still finds a row whose key column draws "⌃D".
func helpKeyDisplay(key string) string {
	return mapChordTokens(key, chordText)
}

// chordText is helpKeyDisplay's per-token worker.
func chordText(tok string) string {
	mods, last, ok := splitModifierChord(strings.ToLower(tok))
	if !ok {
		return tok
	}
	display := make([]string, 0, len(mods)+1)
	for _, p := range mods {
		display = append(display, helpKeyDisplayModifiers[p])
	}
	display = append(display, titleKeyName(last))
	return strings.Join(display, "+")
}

// helpKeySymbols formats a keybinding the way the key column draws it:
// modifier chords become the same glyphs the which-key panel uses
// ("ctrl+d" -> "⌃D"), everything else stays textual. Falls back to
// helpKeyDisplay whenever the icon mode cannot promise non-ASCII, so
// plain and no-color terminals still read "Ctrl+D".
func helpKeySymbols(key string) string {
	if ConfigNoColor || !iconModeDrawsSymbols() {
		return helpKeyDisplay(key)
	}
	return mapChordTokens(key, func(tok string) string {
		lower := strings.ToLower(tok)
		if _, _, ok := splitModifierChord(lower); !ok {
			return tok
		}
		return KeyChordDisplay(lower)
	})
}

// padKeyLeft right-aligns s in a cell of width w. Padding is measured with
// lipgloss.Width, not len: the modifier glyphs are multibyte and a
// byte-counted %*s pad would leave the column ragged.
func padKeyLeft(s string, w int) string {
	if pad := w - lipgloss.Width(s); pad > 0 {
		return strings.Repeat(" ", pad) + s
	}
	return s
}
