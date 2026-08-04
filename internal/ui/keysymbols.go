package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// modifierSymbols are the glyphs a modifier is drawn as in the icon modes that
// promise non-ASCII but not a patched font. Plain Unicode on purpose: these
// codepoints ship with every modern font, where the Nerd Font keycaps below
// render as tofu without one.
//
// "hyper" is deliberately absent: it has no settled symbol, and KeyChordDisplay
// falls back to the textual chord rather than invent one.
var modifierSymbols = map[string]string{
	"ctrl":  "⌃", // UP ARROWHEAD
	"shift": "⇧", // UPWARDS WHITE ARROW
	"alt":   "⌥", // OPTION KEY
	"meta":  "⌘", // PLACE OF INTEREST SIGN
	"super": "⌘",
}

// nerdModifierGlyphs are which-key.nvim's own modifier glyphs (its
// icons.keys C/S/M/D entries): Nerd Font Material Design Icons drawn as full
// keycaps rather than the small arrowheads above. Reachable only in "nerdfont"
// icon mode, which is exactly the mode that promises a patched font.
var nerdModifierGlyphs = map[string]string{
	"ctrl":  "\U000F0634", // md-apple-keyboard-control
	"shift": "\U000F0636", // md-apple-keyboard-shift
	"alt":   "\U000F0635", // md-apple-keyboard-option
	"meta":  "\U000F0633", // md-apple-keyboard-command
	"super": "\U000F0633",
}

// nerdKeyGlyphs replaces the spelled-out name of a key that prints nothing with
// which-key.nvim's keycap for it. Keys whose name already reads as a key
// ("left", "pgup", "f1") stay textual: a glyph there trades a legible word for
// a lookup.
var nerdKeyGlyphs = map[string]string{
	"space":     "\U000F1050", // md-keyboard-space
	"tab":       "\U000F0312", // md-keyboard-tab
	"enter":     "\U000F0311", // md-keyboard-return
	"esc":       "\U000F12B7", // md-keyboard-esc
	"backspace": "\U000F006E", // md-backspace
}

// unicodeKeyGlyphs is the same substitution for the fonts that have no keycaps.
// Only space qualifies: OPEN BOX is unmistakable, whereas ⇥ / ⏎ / ⎋ are small,
// easily confused with each other, and lose to the words "tab", "enter", "esc".
var unicodeKeyGlyphs = map[string]string{
	"space": "␣", // OPEN BOX
}

// nerdGlyphPad separates a Nerd Font glyph from whatever the chord writes after
// it. go-runewidth measures these private-use codepoints as one cell
// (TestKeyGlyphs_AreSingleCell pins that), but the proportional Nerd Font
// variants draw them wider than one cell, so the next character gets painted
// over. which-key.nvim pads the same glyphs for the same reason. A glyph that
// ENDS the chord is left unpadded: the panel already puts a space between a key
// and its label, which is the same protection.
const nerdGlyphPad = " "

// keyGlyphs returns the substitution tables and the glyph padding for the
// active icon mode. Splitting them here keeps KeyChordDisplay free of mode
// branching in its loop.
func keyGlyphs() (mods, names map[string]string, pad string) {
	if IconMode == "nerdfont" {
		return nerdModifierGlyphs, nerdKeyGlyphs, nerdGlyphPad
	}
	return modifierSymbols, unicodeKeyGlyphs, ""
}

// iconModeDrawsSymbols reports whether the active icon mode promises a terminal
// that can draw non-ASCII glyphs. Reuses the same modes detectIconMode resolves,
// so the symbols follow the icons setting rather than adding a second knob.
func iconModeDrawsSymbols() bool {
	switch IconMode {
	case "nerdfont", "unicode", "emoji":
		return true
	}
	return false
}

// splitModifierChord splits a modified chord into its modifier names and the
// key they apply to. ok is false for anything that is not one — a plain binding,
// a binding that merely contains "+" (the literal "+" key, "ctrl++"), or a
// segment that is not a known modifier.
func splitModifierChord(key string) (mods []string, last string, ok bool) {
	parts := strings.Split(key, "+")
	if len(parts) < 2 {
		return nil, "", false
	}
	for _, p := range parts[:len(parts)-1] {
		if _, isMod := helpKeyDisplayModifiers[p]; !isMod {
			return nil, "", false
		}
	}
	last = parts[len(parts)-1]
	if last == "" {
		return nil, "", false
	}
	return parts[:len(parts)-1], last, true
}

// titleKeyName uppercases a chord's key rune-wise, so a multibyte key is never
// split mid-rune ("ctrl+é" -> "É", "shift+tab" -> "Tab").
func titleKeyName(last string) string {
	r := []rune(last)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// KeyChordDisplay renders a keybinding the way the which-key panel draws it:
// "ctrl+shift+x" becomes "󰘴 󰘶 X" in nerdfont mode and "⌃⇧X" in unicode mode,
// "space" becomes the space keycap, and everything stays the verbatim binding
// in the modes that promise only ASCII. Bindings with no glyph form ("d", "?",
// "f1") are returned unchanged in every mode.
//
// No-color mode also keeps the textual form: it is the "this terminal is
// minimal" switch users reach for, and a symbol carries no meaning at all in a
// terminal that cannot draw it.
// The which-key panel calls this for every entry on every build, so the
// overwhelmingly common unmodified binding must not even reach strings.Split
// (which allocates a slice header per call), and the chord path writes straight
// into one builder rather than through intermediate strings.
func KeyChordDisplay(key string) string {
	if ConfigNoColor || !iconModeDrawsSymbols() {
		return key
	}
	mods, names, pad := keyGlyphs()
	if strings.IndexByte(key, '+') < 0 {
		if g, isNamed := names[key]; isNamed {
			return g
		}
		return key
	}
	chord, last, ok := splitModifierChord(key)
	if !ok {
		return key
	}
	var sb strings.Builder
	sb.Grow(len(chord)*(4+len(pad)) + len(last))
	for _, m := range chord {
		sym, has := mods[m]
		if !has {
			return key
		}
		sb.WriteString(sym)
		sb.WriteString(pad)
	}
	if g, isNamed := names[last]; isNamed {
		sb.WriteString(g)
		return sb.String()
	}
	r, size := utf8.DecodeRuneInString(last)
	sb.WriteRune(unicode.ToUpper(r))
	sb.WriteString(last[size:])
	return sb.String()
}
