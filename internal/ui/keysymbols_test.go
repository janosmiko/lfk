package ui

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func restoreKeySymbolGlobals(t *testing.T) {
	t.Helper()
	icons, noColor := IconMode, ConfigNoColor
	t.Cleanup(func() {
		IconMode = icons
		ConfigNoColor = noColor
	})
}

// TestKeyGlyphs_AreSingleCell is the guard the which-key grid depends on. Its
// column arithmetic reserves cells from lipgloss.Width, so a glyph the width
// table disagrees with pushes every column to its right out of alignment. The
// Nerd Font entries are private-use codepoints, where a go-runewidth bump could
// silently change the answer — pin it here so that fails loudly rather than as
// a crooked panel.
//
// A measured width of 1 is not a promise the FONT draws them in one cell: the
// proportional Nerd Font variants draw the keycaps wider. That is what
// nerdGlyphPad is for; see its comment.
func TestKeyGlyphs_AreSingleCell(t *testing.T) {
	tables := map[string]map[string]string{
		"modifierSymbols":    modifierSymbols,
		"nerdModifierGlyphs": nerdModifierGlyphs,
		"nerdKeyGlyphs":      nerdKeyGlyphs,
		"unicodeKeyGlyphs":   unicodeKeyGlyphs,
	}
	for table, glyphs := range tables {
		for name, sym := range glyphs {
			assert.Equal(t, 1, lipgloss.Width(sym), "%s[%s] = %q must measure one cell", table, name, sym)
			assert.Len(t, []rune(sym), 1, "%s[%s] = %q must be a single rune", table, name, sym)
		}
	}
}

// The drawn width of a chord is what the grid sizes its key field from, so pin
// it per mode: the nerdfont form is one cell per glyph PLUS a pad cell per
// modifier, the unicode form is one cell per glyph flat.
func TestKeyChordDisplay_DrawnWidths(t *testing.T) {
	restoreKeySymbolGlobals(t)
	ConfigNoColor = false
	tests := []struct {
		icons, given string
		want         int
	}{
		{"nerdfont", "ctrl+d", 3},         // glyph + pad + D
		{"nerdfont", "ctrl+shift+x", 5},   // 2x (glyph + pad) + X
		{"nerdfont", "ctrl+space", 3},     // glyph + pad + space keycap
		{"nerdfont", "space", 1},          // a trailing glyph carries no pad
		{"unicode", "ctrl+d", 2},          //
		{"unicode", "ctrl+shift+x", 3},    //
		{"unicode", "ctrl+space", 2},      //
		{"unicode", "space", 1},           //
		{"simple", "ctrl+space", 10},      // verbatim
		{"nerdfont", "hyper+k", 7},        // verbatim: no glyph for hyper
		{"nerdfont", "ctrl+shift+f10", 7}, // 2x (glyph + pad) + F10
	}
	for _, tt := range tests {
		IconMode = tt.icons
		got := KeyChordDisplay(tt.given)
		assert.Equal(t, tt.want, lipgloss.Width(got), "icons=%s given=%s drawn=%q", tt.icons, tt.given, got)
	}
}

func TestKeyChordDisplay_SymbolModes(t *testing.T) {
	restoreKeySymbolGlobals(t)
	ConfigNoColor = false
	tests := []struct {
		icons, given, want string
	}{
		{"unicode", "ctrl+d", "⌃D"},
		{"emoji", "ctrl+d", "⌃D"},
		{"unicode", "ctrl+shift+x", "⌃⇧X"},
		{"unicode", "alt+left", "⌥Left"},
		{"unicode", "meta+k", "⌘K"},
		{"unicode", "super+k", "⌘K"},
		// nerdfont draws which-key.nvim's keycaps, each modifier followed by the
		// pad cell that keeps a proportional font off the next character.
		{"nerdfont", "ctrl+d", "\U000F0634 D"},
		{"nerdfont", "ctrl+shift+x", "\U000F0634 \U000F0636 X"},
		{"nerdfont", "alt+left", "\U000F0635 Left"},
		{"nerdfont", "meta+k", "\U000F0633 K"},
		// Keys that print nothing become a keycap instead of their name.
		{"nerdfont", "space", "\U000F1050"},
		{"nerdfont", "ctrl+space", "\U000F0634 \U000F1050"},
		{"nerdfont", "tab", "\U000F0312"},
		{"nerdfont", "enter", "\U000F0311"},
		{"nerdfont", "esc", "\U000F12B7"},
		{"nerdfont", "backspace", "\U000F006E"},
		{"nerdfont", "shift+tab", "\U000F0636 \U000F0312"},
		// Unicode has no legible keycap for anything but space.
		{"unicode", "space", "␣"},
		{"unicode", "ctrl+space", "⌃␣"},
		{"unicode", "tab", "tab"},
		{"unicode", "enter", "enter"},
		{"unicode", "esc", "esc"},
		{"unicode", "shift+tab", "⇧Tab"},
		// No settled symbol for hyper, so the whole chord stays textual rather
		// than mixing a glyph and a word.
		{"unicode", "hyper+k", "hyper+k"},
		{"nerdfont", "hyper+k", "hyper+k"},
		// Bindings with no glyph form are never touched, in any mode.
		{"unicode", "d", "d"},
		{"unicode", "?", "?"},
		{"unicode", "f1", "f1"},
		{"nerdfont", "d", "d"},
		{"nerdfont", "f1", "f1"},
		// A binding that merely contains "+" is not a chord.
		{"unicode", "+", "+"},
		{"unicode", "ctrl++", "ctrl++"},
		// Modes that promise nothing beyond ASCII keep the binding verbatim.
		{"simple", "ctrl+d", "ctrl+d"},
		{"simple", "space", "space"},
		{"none", "ctrl+d", "ctrl+d"},
		{"none", "space", "space"},
	}
	for _, tt := range tests {
		IconMode = tt.icons
		assert.Equal(t, tt.want, KeyChordDisplay(tt.given), "icons=%s given=%s", tt.icons, tt.given)
	}
}

// No-color is the "this terminal is minimal" switch, so it also opts out of
// glyphs even when the icon mode would allow them.
func TestKeyChordDisplay_NoColorKeepsTextual(t *testing.T) {
	restoreKeySymbolGlobals(t)
	IconMode = "nerdfont"
	ConfigNoColor = true
	assert.Equal(t, "ctrl+d", KeyChordDisplay("ctrl+d"))
	assert.Equal(t, "space", KeyChordDisplay("space"))
}

// helpKeyDisplay was refactored onto the shared chord splitter; its output must
// not have moved. The help screen deliberately keeps spelling modifiers out.
func TestHelpKeyDisplay_UnaffectedBySymbolMode(t *testing.T) {
	restoreKeySymbolGlobals(t)
	for _, icons := range []string{"nerdfont", "unicode", "simple", "none", "emoji"} {
		IconMode = icons
		assert.Equal(t, "Ctrl+Shift+X", helpKeyDisplay("ctrl+shift+x"), "icons=%s", icons)
		assert.Equal(t, "Shift+Tab", helpKeyDisplay("shift+tab"), "icons=%s", icons)
		assert.Equal(t, "d", helpKeyDisplay("d"), "icons=%s", icons)
	}
}
