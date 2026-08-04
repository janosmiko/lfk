package app

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestRenderWhichKey_ListsTargets(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyDelayMs = 0
	m := gotoTestModel()
	m.pendingG = true
	m.whichKey.shown = true
	out := stripANSI(m.renderWhichKey(strings.Repeat("\n", m.height)))
	for _, want := range []string{"Pods", "Deployments", "list top"} {
		if !strings.Contains(out, want) {
			t.Fatalf("which-key popup missing %q:\n%s", want, out)
		}
	}
}

// The modern preset anchors the panel to the bottom of the screen, so the
// content must land in the lower rows, not centered or at the top.
func TestRenderWhichKey_AnchoredToBottom(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyDelayMs = 0
	m := gotoTestModel()
	m.pendingG = true
	m.whichKey.shown = true
	lines := strings.Split(stripANSI(m.renderWhichKey(strings.Repeat("\n", m.height))), "\n")
	podsRow := -1
	for i, l := range lines {
		if strings.Contains(l, "Pods") {
			podsRow = i
			break
		}
	}
	if podsRow < 0 {
		t.Fatal("Pods not found in rendered panel")
	}
	if podsRow <= m.height/2 {
		t.Fatalf("panel must be bottom-anchored; Pods at row %d of %d", podsRow, m.height)
	}
}

func TestRenderWhichKey_HiddenWhenDisabled(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ConfigWhichKeyEnabled = false
	m := gotoTestModel()
	m.pendingG = true
	m.whichKey.shown = true
	bg := strings.Repeat("\n", m.height)
	if got := m.renderWhichKey(bg); got != bg {
		t.Fatal("popup must not render when which_key_enabled is false")
	}
}

// TestRenderWhichKeyPanel_RendersAFlatListWithNoHeaders replaces the old
// group-header assertion. neovim's which-key has no section headers at all —
// every entry is one row of a single list — so a panel that draws a bare label
// with no key next to it is drawing something neovim never would.
func TestRenderWhichKeyPanel_RendersAFlatListWithNoHeaders(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := gotoTestModel()
	cells := []whichKeyCell{{key: "d", desc: "Delete"}, {key: "e", desc: "Edit"}, {key: "s", desc: "Sort next column"}}
	out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), cells, 0))
	for _, want := range []string{"d Delete", "e Edit", "s Sort next column"} {
		if !strings.Contains(out, want) {
			t.Fatalf("panel missing %q (key, one space, label):\n%s", want, out)
		}
	}
	// The former group titles were rendered as their own row. Nothing may put a
	// section name on screen any more.
	for _, banned := range []string{"Actions", "Views", "Selection", "Settings", "Filter", "->"} {
		if strings.Contains(out, banned) {
			t.Fatalf("flat panel must not render %q:\n%s", banned, out)
		}
	}
}

func TestRenderWhichKeyPanel_EmptyCellsRenderNothing(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ConfigWhichKeyEnabled = true
	m := gotoTestModel()
	bg := strings.Repeat("\n", m.height)
	if got := m.renderWhichKeyPanel(bg, nil, 0); got != bg {
		t.Fatal("an empty entry list must render nothing, not an empty box")
	}
	if got := m.renderWhichKeyPanel(bg, []whichKeyCell{}, 0); got != bg {
		t.Fatal("a zero-length entry list must render nothing")
	}
}

func TestRenderWhichKeyPanel_TinyTerminalRendersNothing(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ConfigWhichKeyEnabled = true
	m := gotoTestModel()
	m.width, m.height = 10, 4
	bg := strings.Repeat("\n", m.height)
	cells := []whichKeyCell{{key: "d", desc: "Delete"}}
	if got := m.renderWhichKeyPanel(bg, cells, 0); got != bg {
		t.Fatal("panel must be skipped when the terminal is too small")
	}
}

// TestRenderWhichKeyPanel_OverflowScrollsInsteadOfDropping replaces the old
// "+N more" footer assertion. The panel no longer pages or truncates: content
// past the viewport scrolls, so the invariant that matters is stronger — every
// entry must be REACHABLE, and the panel must still never overflow the screen.
func TestRenderWhichKeyPanel_OverflowScrollsInsteadOfDropping(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := gotoTestModel()
	m.height = 12 // deliberately short
	cells := make([]whichKeyCell, 0, 48)
	for i := range 48 {
		cells = append(cells, whichKeyCell{key: string(rune('a' + i%8)), desc: fmt.Sprintf("entry %02d", i)})
	}

	lay, ok := m.whichKeyLayoutFor(cells)
	if !ok || lay.maxScroll == 0 {
		t.Fatalf("precondition: 48 cells at height %d must overflow; maxScroll=%d", m.height, lay.maxScroll)
	}

	var seen strings.Builder
	for scroll := 0; scroll <= lay.maxScroll; scroll++ {
		out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), cells, scroll))
		if lines := strings.Split(out, "\n"); len(lines) > m.height+1 {
			t.Fatalf("scroll %d: panel overflowed the screen: %d lines for height %d", scroll, len(lines), m.height)
		}
		seen.WriteString(out)
		seen.WriteString("\n")
	}
	rendered := seen.String()
	for i := range 48 {
		want := fmt.Sprintf("entry %02d", i)
		if !strings.Contains(rendered, want) {
			t.Errorf("%q never appears at any scroll offset — unreachable", want)
		}
	}
}

// TestRenderWhichKeyPanel_ScrollClampsToTheEnd: an out-of-range offset must land
// on the last full screen, never past it or on a short/blank tail.
func TestRenderWhichKeyPanel_ScrollClampsToTheEnd(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := whichKeyTestModel()
	m.width, m.height = 80, 24

	cells := m.whichKeyLeaderCells()
	lay, ok := m.whichKeyLayoutFor(cells)
	if !ok || lay.maxScroll == 0 {
		t.Fatalf("precondition: the full catalog must overflow at 80x24; maxScroll=%d", lay.maxScroll)
	}
	bg := strings.Repeat("\n", m.height)
	atEnd := m.renderWhichKeyPanel(bg, cells, lay.maxScroll)
	if beyond := m.renderWhichKeyPanel(bg, cells, lay.maxScroll+50); beyond != atEnd {
		t.Fatal("an offset past the end must clamp to the last screen")
	}
	if before := m.renderWhichKeyPanel(bg, cells, -5); before != m.renderWhichKeyPanel(bg, cells, 0) {
		t.Fatal("a negative offset must clamp to the top")
	}
}

// whichKeyContentRows returns the panel's body rows with the border and the
// horizontal padding stripped, so a test can index straight into the grid.
// legendRows must match the layout's lay.legendRows: the legend now hugs the
// bottom border, with the vertical padding gap sitting above it rather than
// below, so the padding to drop is no longer symmetric top/bottom whenever a
// legend is present — it must be excised from the middle instead.
func whichKeyContentRows(t *testing.T, out string, container, legendRows int) []string {
	t.Helper()
	var rows []string
	for line := range strings.SplitSeq(out, "\n") {
		r := []rune(line)
		if len(r) < 2 || r[0] != '│' || r[len(r)-1] != '│' {
			continue // border row or untouched background
		}
		body := r[1 : len(r)-1]
		if len(body) < whichKeyPadH+container {
			t.Fatalf("body row is %d wide, want at least %d", len(body), whichKeyPadH+container)
		}
		rows = append(rows, string(body[whichKeyPadH:whichKeyPadH+container]))
	}
	// Drop the box's vertical padding rows; they are bordered but hold no grid.
	if len(rows) < 2*whichKeyPadV+legendRows {
		t.Fatalf("panel has %d bordered rows, fewer than its own padding", len(rows))
	}
	entries := append([]string{}, rows[whichKeyPadV:len(rows)-whichKeyPadV-legendRows]...)
	if legendRows > 0 {
		entries = append(entries, rows[len(rows)-legendRows:]...)
	}
	return entries
}

// TestRenderWhichKeyPanel_ColumnsAreUniformAndKeysRightAligned is the visual
// property the neovim port exists for: every column is the same width, and
// inside each column the key is right-aligned against a field sized to THAT
// column, followed by exactly one space. A global key field (the bug this
// replaces) shows up here as a column-0 field far wider than its own keys.
func TestRenderWhichKeyPanel_ColumnsAreUniformAndKeysRightAligned(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := whichKeyTestModel()
	m.width, m.height = 120, 40

	cells := m.whichKeyLeaderCells()
	lay, ok := m.whichKeyLayoutFor(cells)
	if !ok {
		t.Fatal("precondition: the panel must lay out at 120x40")
	}
	g := lay.grid
	if g.boxN < 2 {
		t.Fatalf("precondition: 120 columns must fit at least 2 grid columns, got %d", g.boxN)
	}

	out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), cells, 0))
	rows := whichKeyContentRows(t, out, lay.container, lay.legendRows)
	// The full catalog spans several groups at this size, so the legend
	// footer adds one more content row beyond the entry viewport.
	if want := lay.viewRows + lay.legendRows; len(rows) != want {
		t.Fatalf("panel drew %d body rows, want %d", len(rows), want)
	}
	widest := make([]int, g.boxN)
	// The legend footer (if any) is the last row and is not part of the grid
	// — it must not be walked as if it were an entry row, or its own content
	// gets measured against an unrelated column offset.
	for r, row := range rows[:lay.viewRows] {
		runes := []rune(row)
		for b := range g.boxN {
			idx := b*g.rowN + r
			if idx >= len(cells) {
				break
			}
			// Measure the key AS DRAWN: a symbol-rendered chord ("⌃D") is
			// narrower than its binding ("ctrl+d"), and the grid is sized
			// from the drawn form.
			key := cells[idx].keyText()
			widest[b] = max(widest[b], lipgloss.Width(key))
			start := b * g.boxW
			field := string(runes[start+g.lead : start+g.lead+g.keyW[b]])
			if strings.TrimLeft(field, " ") != key {
				t.Fatalf("row %d col %d: key field %q is not %q right-aligned", r, b, field, key)
			}
			if got := runes[start+g.lead+g.keyW[b]]; got != ' ' {
				t.Fatalf("row %d col %d: want a single space after the key, got %q", r, b, got)
			}
		}
	}
	// Each column's field is sized to its OWN widest key. A global field (the
	// bug this replaces) makes column 0 wider than anything it actually holds.
	for b, w := range widest {
		if w > 0 && g.keyW[b] != w {
			t.Errorf("column %d key field is %d wide but its widest key is %d", b, g.keyW[b], w)
		}
	}
}

// TestRenderWhichKeyPanel_OverlongLabelNeverExceedsWidth guards against a
// panel wider than the terminal: a single cell whose "key desc" text alone
// exceeds the available inner width must be ellipsized, not left to blow out
// the box. Uses a real catalog label ("Copy as (YAML/JSON/table)", 27 chars)
// at widths that pass whichKeyMinWidth but are still narrower than the label.
func TestRenderWhichKeyPanel_OverlongLabelNeverExceedsWidth(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := gotoTestModel()
	m.height = 20
	cells := []whichKeyCell{{key: "Y", desc: "Copy as (YAML/JSON/table)"}}
	for _, w := range []int{20, 25, 34} {
		m.width = w
		out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), cells, 0))
		for line := range strings.SplitSeq(out, "\n") {
			if got := lipgloss.Width(line); got > w {
				t.Fatalf("width %d: rendered line %q is %d columns wide", w, line, got)
			}
		}
	}
}

// whichKeyRenderBenchModel is a full-catalog leader panel at the given size,
// armed and shown, i.e. exactly what View() renders on every frame while the
// user is paging.
func whichKeyRenderBenchModel(w, h int) Model {
	m := whichKeyTestModel()
	m.width, m.height = w, h
	m.whichKey = whichKeyState{armed: true, shown: true}
	return m
}

// BenchmarkWhichKeyLeaderRender measures the per-frame cost of the panel.
// Layout is what the hint bar and the scroll keys also pay, so it is measured
// on its own alongside the full styled render.
func BenchmarkWhichKeyLeaderRender(b *testing.B) {
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true

	sizes := []struct {
		name string
		w, h int
	}{
		{"80x24", 80, 24},
		{"120x40", 120, 40},
	}
	for _, s := range sizes {
		m := whichKeyRenderBenchModel(s.w, s.h)
		bg := strings.Repeat("x\n", s.h)
		b.Run(s.name+"/Layout", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = m.whichKeyLayoutFor(m.whichKeyLeaderCells())
			}
		})
		b.Run(s.name+"/Full", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = m.renderWhichKeyLeader(bg)
			}
		})
	}
}

// wkLeaderRenderAllocPerRow caps allocations per RENDERED ROW of the panel.
// Normalised per row on purpose: the panel is now as tall as its content, so a
// flat per-call ceiling would move every time the catalog or the default
// terminal size changes, and would say nothing about whether the layout itself
// got more expensive. Measured at 61 allocs/row for the uniform-column grid,
// against 162/row for the per-column-widest + gap-spreading layout it replaced
// — that old layout would fail this guard, which is the point. The threshold
// sits ~2x above the measurement so CI hardware variance can't flake it, while
// a reintroduced per-cell re-measure or a whole-catalog render (instead of just
// the viewport) still trips it.
const wkLeaderRenderAllocPerRow = 140

func TestWhichKeyLeaderRender_AllocationCeiling(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true

	m := whichKeyRenderBenchModel(80, 24)
	lay, ok := m.whichKeyLayoutFor(m.whichKeyLeaderCells())
	if !ok {
		t.Fatal("precondition: the panel must lay out at 80x24")
	}
	bg := strings.Repeat("x\n", 24)
	res := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = m.renderWhichKeyLeader(bg)
		}
	})
	perRow := float64(res.MemAllocs) / float64(res.N) / float64(lay.viewRows)
	if perRow > wkLeaderRenderAllocPerRow {
		t.Fatalf("renderWhichKeyLeader allocates %.0f times per rendered row (threshold %d); the panel re-lays out on every frame, so a per-cell re-measure or a render of the whole catalog costs a full frame", perRow, wkLeaderRenderAllocPerRow)
	}
	t.Logf("renderWhichKeyLeader: %.0f allocs/op over %d rows = %.0f/row (threshold %d)",
		float64(res.MemAllocs)/float64(res.N), lay.viewRows, perRow, wkLeaderRenderAllocPerRow)
}

// wkLeaderLayoutAllocThreshold caps the layout-only pass, which the hint bar
// runs on every frame in ADDITION to the render above just to decide whether to
// advertise the scroll keys. Measured at 61 allocs/op.
const wkLeaderLayoutAllocThreshold = 200

func TestWhichKeyLeaderLayout_AllocationCeiling(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true

	m := whichKeyRenderBenchModel(80, 24)
	res := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_, _ = m.whichKeyLayoutFor(m.whichKeyLeaderCells())
		}
	})
	allocs := float64(res.MemAllocs) / float64(res.N)
	if allocs > wkLeaderLayoutAllocThreshold {
		t.Fatalf("whichKeyLayoutFor allocates %.0f times per call (threshold %d); it must stay a plain row count, never a styled render", allocs, wkLeaderLayoutAllocThreshold)
	}
	t.Logf("whichKeyLayoutFor: %.0f allocs/op (threshold %d)", allocs, wkLeaderLayoutAllocThreshold)
}
