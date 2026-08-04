package app

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- Change 1: group colors ---------------------------------------------

// Every declared group must resolve to its own accent. Two groups sharing a
// color is the same as having no cue for either, which is the state removing
// the headers left the panel in.
func TestWhichKeyGroupStyles_EveryGroupHasADistinctAccent(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ApplyTheme(ui.DefaultTheme())
	styles := whichKeyGroupStyles()

	seen := map[color.Color]whichKeyGroup{}
	for _, g := range whichKeyGroupOrder() {
		st, ok := styles[g]
		if !ok {
			t.Fatalf("group %q has no description style; whichKeyGroupStyles must cover whichKeyGroupOrder", g)
		}
		fg := st.GetForeground()
		if fg == nil {
			t.Errorf("group %q has no foreground; the accent IS the cue", g)
			continue
		}
		if other, dup := seen[fg]; dup {
			t.Errorf("groups %q and %q share foreground %v — no category cue for either", g, other, fg)
		}
		seen[fg] = g
	}
}

// Red is the app's failure/destructive signal, and the accent now sits on the
// DESCRIPTION — a whole red sentence reads as an error message, not as a
// category — so no group may claim it.
func TestWhichKeyGroupStyles_NeverUseTheErrorColor(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ApplyTheme(ui.DefaultTheme())
	bad := lipgloss.Color(ui.ColorError)
	for g, st := range whichKeyGroupStyles() {
		if st.GetForeground() == bad {
			t.Errorf("group %q uses the error color; red must stay reserved for failures", g)
		}
	}
}

// The ungrouped description style is the baseline every cell without a group
// draws in, so a group that reuses its color has no cue at all — the exact
// failure mode the accent moved onto the description to avoid (Settings used to
// be the plain text color, which was distinct only while the accent sat on the
// key). This is what keeps the goto-popup fallback below a real assertion
// rather than a tautology.
func TestWhichKeyGroupStyles_NeverMatchThePlainDescription(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ApplyTheme(ui.DefaultTheme())
	plain := newWhichKeyCellStyles().desc.GetForeground()
	if plain == nil {
		t.Fatal("precondition: the ungrouped description must have a foreground to compare against")
	}
	for g, st := range whichKeyGroupStyles() {
		if st.GetForeground() == plain {
			t.Errorf("group %q renders in the ungrouped description color %v; it has no category cue", g, plain)
		}
	}
}

// The catalog's Group must survive onto the cell — that is the only path by
// which the accent reaches the screen.
func TestWhichKeyLeaderCells_CarryTheCatalogGroup(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := whichKeyTestModel()

	byKey := map[string]whichKeyGroup{}
	kb := ui.ActiveKeybindings
	for _, a := range m.availableWhichKeyActions() {
		byKey[a.Key(kb)] = a.Group
	}
	groups := map[whichKeyGroup]bool{}
	st := newWhichKeyCellStyles()
	for _, c := range m.whichKeyLeaderCells() {
		if c.group == "" {
			t.Errorf("leader entry %q (%s) lost its group", c.key, c.desc)
		}
		// The key is uniform now, so the group must reach the DESCRIPTION or it
		// reaches nothing.
		if st.descStyle(c.group).Render(c.desc) == st.desc.Render(c.desc) {
			t.Errorf("leader entry %q (%s) draws its description in the ungrouped style", c.key, c.desc)
		}
		groups[c.group] = true
	}
	if len(groups) < 2 {
		t.Fatalf("precondition: the panel must span several groups, got %v", groups)
	}
}

// The g-prefix goto popup has no groups and must keep the styling it always
// had: every description on the plain style, nothing tinted per entry.
func TestWhichKeyCells_GotoPopupStaysUngrouped(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ApplyTheme(ui.DefaultTheme())
	m := gotoTestModel()

	st := newWhichKeyCellStyles()
	for _, c := range m.whichKeyCells() {
		if c.group != "" {
			t.Fatalf("goto entry %q carries group %q; the popup must stay ungrouped", c.key, c.group)
		}
		if got, want := st.descStyle(c.group).Render(c.desc), st.desc.Render(c.desc); got != want {
			t.Fatalf("goto entry %q renders %q, want the ungrouped %q", c.key, got, want)
		}
	}
	// The fallback only means something while a real group renders differently.
	grouped := st.descStyle(whichKeyGroupOrder()[0])
	if grouped.Render("Delete") == st.desc.Render("Delete") {
		t.Fatal("every group renders like the ungrouped style; the fallback assertion above proves nothing")
	}
}

// Without color the accents collapse to one bold style — the cue is gone, but
// the entries must still be there and readable.
func TestWhichKeyGroupStyles_NoColorCollapsesButStillRenders(t *testing.T) {
	restoreWhichKeyGlobals(t)
	noColor := ui.ConfigNoColor
	t.Cleanup(func() {
		ui.ConfigNoColor = noColor
		ui.ApplyTheme(ui.DefaultTheme())
	})
	ui.ConfigNoColor = true
	ui.ApplyTheme(ui.DefaultTheme())

	for _, g := range whichKeyGroupOrder() {
		if got := whichKeyGroupStyles()[g].Render("d"); !strings.Contains(got, "d") {
			t.Errorf("group %q renders %q in no-color mode; the key must survive", g, got)
		}
	}

	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := whichKeyTestModel()
	m.width, m.height = 120, 40
	cells := m.whichKeyLeaderCells()
	out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), cells, 0))
	if !strings.Contains(out, cells[0].desc) {
		t.Errorf("no-color panel dropped its first entry %q:\n%s", cells[0].desc, out)
	}
}

// --- Change 2: modifier symbols -----------------------------------------

// whichKeySymbolCells is a grid whose second column is all modifier chords and
// unprintable keys, so the glyph form and the textual form differ in width by a
// lot.
func whichKeySymbolCells() []whichKeyCell {
	cells := []whichKeyCell{
		{key: "d", desc: "Delete"},
		{key: "e", desc: "Edit in $EDITOR"},
		{key: "ctrl+shift+x", desc: "Mouse capture"},
		{key: "ctrl+space", desc: "Select range"},
	}
	fillWhichKeyDisplay(cells)
	return cells
}

// A glyph-rendered chord must land in the key field exactly like a plain key:
// right-aligned, one space, and the column widths sized from the drawn form.
// Both glyph modes are checked because the nerdfont form carries a pad cell
// after every modifier, so its drawn width is not the unicode form's.
func TestRenderWhichKeyPanel_SymbolChordStillAligns(t *testing.T) {
	modes := map[string]string{"unicode": "⌃⇧X", "nerdfont": "\U000F0634 \U000F0636 X"}
	for icons, wantChord := range modes {
		t.Run(icons, func(t *testing.T) {
			restoreWhichKeyGlobals(t)
			ui.ActiveKeybindings = ui.DefaultKeybindings()
			ui.ConfigWhichKeyEnabled = true
			ui.IconMode = icons

			m := gotoTestModel()
			m.width, m.height = 80, 20
			cells := whichKeySymbolCells()
			lay, ok := m.whichKeyLayoutFor(cells)
			if !ok {
				t.Fatal("precondition: the panel must lay out at 80x20")
			}
			if lay.grid.boxN < 2 {
				t.Fatalf("precondition: want at least 2 columns, got %d", lay.grid.boxN)
			}
			if got := cells[2].keyText(); got != wantChord {
				t.Fatalf("precondition: chord must render as glyphs, got %q", got)
			}
			// The key field is sized from the GLYPHS, not from "ctrl+shift+x".
			if want := lipgloss.Width(wantChord); lay.grid.keyW[1] != want {
				t.Errorf("column 1 key field = %d, want %d (the drawn width)", lay.grid.keyW[1], want)
			}

			out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), cells, 0))
			rows := whichKeyContentRows(t, out, lay.container)
			g := lay.grid
			for r, row := range rows {
				if r >= g.rowN {
					break // whichKeyMinRows padding: bordered, but no grid on it
				}
				runes := []rune(row)
				for b := range g.boxN {
					idx := b*g.rowN + r
					if idx >= len(cells) {
						break
					}
					start := b*g.boxW + g.lead
					field := string(runes[start : start+g.keyW[b]])
					if strings.TrimLeft(field, " ") != cells[idx].keyText() {
						t.Fatalf("row %d col %d: key field %q is not %q right-aligned", r, b, field, cells[idx].keyText())
					}
					if runes[start+g.keyW[b]] != ' ' {
						t.Fatalf("row %d col %d: want a single space after the key", r, b)
					}
				}
			}
		})
	}
}

// The word "space" must never reach the panel in a glyph mode — it is the one
// binding whose name is longer than its whole cell deserves, and the change
// that replaced it is invisible to the width sweep (both forms fit).
func TestRenderWhichKeyPanel_SpaceDrawsAsAKeycap(t *testing.T) {
	tests := []struct{ icons, want string }{
		{"nerdfont", "\U000F1050"},
		{"unicode", "␣"},
		{"simple", "space"},
		{"none", "space"},
	}
	for _, tt := range tests {
		t.Run(tt.icons, func(t *testing.T) {
			restoreWhichKeyGlobals(t)
			ui.ActiveKeybindings = ui.DefaultKeybindings()
			ui.ConfigWhichKeyEnabled = true
			ui.IconMode = tt.icons

			m := gotoTestModel()
			m.width, m.height = 80, 20
			cells := []whichKeyCell{{key: "space", desc: "Toggle selection"}}
			fillWhichKeyDisplay(cells)
			if got := cells[0].keyText(); got != tt.want {
				t.Fatalf("space drawn as %q, want %q", got, tt.want)
			}
			out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), cells, 0))
			if !strings.Contains(out, tt.want+" Toggle selection") {
				t.Errorf("panel is missing %q:\n%s", tt.want+" Toggle selection", out)
			}
		})
	}
}

// The symbols change the display width of the key column, which feeds the whole
// grid. No rendered line may exceed the terminal width in any icon mode, at any
// size, at any scroll offset.
func TestRenderWhichKeyPanel_SymbolsNeverExceedTerminalWidth(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true

	for _, icons := range []string{"unicode", "nerdfont", "simple", "none"} {
		ui.IconMode = icons
		for _, size := range [][2]int{{20, 12}, {40, 16}, {80, 24}, {120, 40}, {200, 40}, {250, 40}} {
			m := whichKeyTestModel()
			m.width, m.height = size[0], size[1]
			cells := m.whichKeyLeaderCells()
			lay, ok := m.whichKeyLayoutFor(cells)
			if !ok {
				continue // too small to draw at all — covered elsewhere
			}
			for scroll := 0; scroll <= lay.maxScroll; scroll++ {
				out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), cells, scroll))
				for line := range strings.SplitSeq(out, "\n") {
					if got := lipgloss.Width(line); got > m.width {
						t.Fatalf("icons=%s %dx%d scroll=%d: line %q is %d columns wide",
							icons, size[0], size[1], scroll, line, got)
					}
				}
			}
		}
	}
}

// The panel is the only surface that draws symbols. Its cells must therefore
// carry the binding verbatim (that is what the sorter and every rebind lookup
// read) and convert only at draw time.
func TestWhichKeyCell_KeyStaysTheRawBinding(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.IconMode = "unicode"

	m := whichKeyTestModel()
	kb := ui.ActiveKeybindings
	bound := map[string]bool{}
	for _, a := range m.availableWhichKeyActions() {
		bound[a.Key(kb)] = true
	}
	for _, c := range m.whichKeyLeaderCells() {
		if !bound[c.key] {
			t.Errorf("cell key %q is not a binding; the symbol form must not be stored", c.key)
		}
		// The drawn form is resolved at build time, not re-parsed by the two
		// grid measurements and the write (see the layout allocation ceiling).
		if c.disp == "" {
			t.Errorf("cell %q has no cached display key; fillWhichKeyDisplay did not run", c.key)
		}
	}
}
