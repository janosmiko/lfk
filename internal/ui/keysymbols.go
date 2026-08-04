package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// modifierSymbols are the glyphs a modifier is drawn as when the terminal is
// expected to handle non-ASCII. Plain Unicode on purpose: neovim's which-key
// defaults sit in the Nerd Font private-use area (U+F0634 and friends), which
// renders as tofu for anyone without a patched font, while these codepoints
// ship with every modern font and measure one cell wide
// (TestKeyChordDisplay_SymbolsAreSingleWidth).
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
// "ctrl+shift+x" becomes "⌃⇧X" when the icon mode allows glyphs, and stays the
// verbatim binding otherwise. Unmodified bindings ("d", "?", "f1") are returned
// unchanged in every mode.
//
// No-color mode also keeps the textual form: it is the "this terminal is
// minimal" switch users reach for, and a symbol carries no meaning at all in a
// terminal that cannot draw it.
// The which-key panel calls this for every entry on every frame, so the
// overwhelmingly common unmodified binding must not even reach strings.Split
// (which allocates a slice header per call), and the chord path writes straight
// into one builder rather than through intermediate strings.
func KeyChordDisplay(key string) string {
	if strings.IndexByte(key, '+') < 0 || ConfigNoColor || !iconModeDrawsSymbols() {
		return key
	}
	mods, last, ok := splitModifierChord(key)
	if !ok {
		return key
	}
	var sb strings.Builder
	sb.Grow(len(mods)*3 + len(last))
	for _, m := range mods {
		sym, has := modifierSymbols[m]
		if !has {
			return key
		}
		sb.WriteString(sym)
	}
	r, size := utf8.DecodeRuneInString(last)
	sb.WriteRune(unicode.ToUpper(r))
	sb.WriteString(last[size:])
	return sb.String()
}
