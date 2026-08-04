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

// TestKeyChordDisplay_SymbolsAreSingleWidth is the guard the which-key grid
// depends on. Its column arithmetic reserves one cell per rune of the key
// field, so an ambiguous- or double-width glyph would push every column to its
// right out of alignment. Nerd Font private-use codepoints are rejected for the
// same reason plus tofu; these are plain Unicode.
func TestKeyChordDisplay_SymbolsAreSingleWidth(t *testing.T) {
	for name, sym := range modifierSymbols {
		assert.Equal(t, 1, lipgloss.Width(sym), "%s symbol %q must measure one cell", name, sym)
		assert.Len(t, []rune(sym), 1, "%s symbol %q must be a single rune", name, sym)
	}
}

func TestKeyChordDisplay_SymbolModes(t *testing.T) {
	restoreKeySymbolGlobals(t)
	ConfigNoColor = false
	tests := []struct {
		icons, given, want string
	}{
		{"unicode", "ctrl+d", "⌃D"},
		{"nerdfont", "ctrl+d", "⌃D"},
		{"emoji", "ctrl+d", "⌃D"},
		{"unicode", "ctrl+shift+x", "⌃⇧X"},
		{"unicode", "alt+left", "⌥Left"},
		{"unicode", "meta+k", "⌘K"},
		{"unicode", "super+k", "⌘K"},
		// No settled symbol for hyper, so the whole chord stays textual rather
		// than mixing a glyph and a word.
		{"unicode", "hyper+k", "hyper+k"},
		// Unmodified bindings are never touched, in any mode.
		{"unicode", "d", "d"},
		{"unicode", "?", "?"},
		{"unicode", "f1", "f1"},
		// A binding that merely contains "+" is not a chord.
		{"unicode", "+", "+"},
		{"unicode", "ctrl++", "ctrl++"},
		// Modes that promise nothing beyond ASCII keep the binding verbatim.
		{"simple", "ctrl+d", "ctrl+d"},
		{"none", "ctrl+d", "ctrl+d"},
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
