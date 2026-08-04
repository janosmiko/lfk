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
			t.Fatalf("group %q has no key style; whichKeyGroupStyles must cover whichKeyGroupOrder", g)
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

// Red is the app's failure/destructive signal and the panel's most destructive
// entries sit in Actions, so no group may claim it.
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
	for _, c := range m.whichKeyLeaderCells() {
		if c.group == "" {
			t.Errorf("leader entry %q (%s) lost its group", c.key, c.desc)
		}
		groups[c.group] = true
	}
	if len(groups) < 2 {
		t.Fatalf("precondition: the panel must span several groups, got %v", groups)
	}
}

// The g-prefix goto popup has no groups and must keep the styling it always
// had: every key on the ungrouped accent, nothing tinted per entry.
func TestWhichKeyCells_GotoPopupStaysUngrouped(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := gotoTestModel()

	st := newWhichKeyCellStyles()
	for _, c := range m.whichKeyCells() {
		if c.group != "" {
			t.Fatalf("goto entry %q carries group %q; the popup must stay ungrouped", c.key, c.group)
		}
		if got, want := st.keyStyle(c.group).Render(c.key), st.key.Render(c.key); got != want {
			t.Fatalf("goto entry %q renders %q, want the ungrouped %q", c.key, got, want)
		}
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

// whichKeySymbolCells is a grid whose second column is all modifier chords, so
// the symbol form and the textual form differ in width by a lot.
func whichKeySymbolCells() []whichKeyCell {
	return []whichKeyCell{
		{key: "d", desc: "Delete"},
		{key: "e", desc: "Edit in $EDITOR"},
		{key: "ctrl+shift+x", desc: "Mouse capture"},
		{key: "ctrl+d", desc: "Scroll down"},
	}
}

// A symbol-rendered chord must land in the key field exactly like a plain key:
// right-aligned, one space, and the column widths sized from the drawn form.
func TestRenderWhichKeyPanel_SymbolChordStillAligns(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.IconMode = "unicode"

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
	if got := cells[2].keyText(); got != "⌃⇧X" {
		t.Fatalf("precondition: chord must render as symbols, got %q", got)
	}
	// The key field is sized from the SYMBOLS, not from "ctrl+shift+x".
	if want := lipgloss.Width("⌃⇧X"); lay.grid.keyW[1] != want {
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
		for _, size := range [][2]int{{20, 12}, {40, 16}, {80, 24}, {120, 40}} {
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
