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

func TestRenderWhichKeyPanel_ShowsGroupHeaders(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := gotoTestModel()
	groups := []whichKeyGroupCells{
		{Title: "Actions", Cells: []whichKeyCell{{"d", "Delete"}, {"e", "Edit"}}},
		{Title: "Sort", Cells: []whichKeyCell{{"s", "Sort next column"}}},
	}
	out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), groups, 0))
	for _, want := range []string{"Actions", "Delete", "Edit", "Sort", "Sort next column"} {
		if !strings.Contains(out, want) {
			t.Fatalf("panel missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "Actions") > strings.Index(out, "Sort next column") {
		t.Fatal("groups must render in the given order")
	}
}

func TestRenderWhichKeyPanel_EmptyGroupsRenderNothing(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ConfigWhichKeyEnabled = true
	m := gotoTestModel()
	bg := strings.Repeat("\n", m.height)
	if got := m.renderWhichKeyPanel(bg, nil, 0); got != bg {
		t.Fatal("an empty group list must render nothing, not an empty box")
	}
	empty := []whichKeyGroupCells{{Title: "Actions", Cells: nil}}
	if got := m.renderWhichKeyPanel(bg, empty, 0); got != bg {
		t.Fatal("groups with no cells must render nothing")
	}
}

func TestRenderWhichKeyPanel_TinyTerminalRendersNothing(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ConfigWhichKeyEnabled = true
	m := gotoTestModel()
	m.width, m.height = 10, 4
	bg := strings.Repeat("\n", m.height)
	groups := []whichKeyGroupCells{{Title: "Actions", Cells: []whichKeyCell{{"d", "Delete"}}}}
	if got := m.renderWhichKeyPanel(bg, groups, 0); got != bg {
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
	groups := make([]whichKeyGroupCells, 0, 6)
	for g := range 6 {
		cells := make([]whichKeyCell, 0, 8)
		for i := range 8 {
			cells = append(cells, whichKeyCell{string(rune('a' + i)), fmt.Sprintf("group %d action %d", g, i)})
		}
		groups = append(groups, whichKeyGroupCells{Title: fmt.Sprintf("G%d", g), Cells: cells})
	}

	lay, ok := m.whichKeyLayoutFor(groups)
	if !ok || lay.maxScroll == 0 {
		t.Fatalf("precondition: 48 cells at height %d must overflow; maxScroll=%d", m.height, lay.maxScroll)
	}

	var seen strings.Builder
	for scroll := 0; scroll <= lay.maxScroll; scroll++ {
		out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), groups, scroll))
		if lines := strings.Split(out, "\n"); len(lines) > m.height+1 {
			t.Fatalf("scroll %d: panel overflowed the screen: %d lines for height %d", scroll, len(lines), m.height)
		}
		seen.WriteString(out)
		seen.WriteString("\n")
	}
	rendered := seen.String()
	for g := range 6 {
		for i := range 8 {
			want := fmt.Sprintf("group %d action %d", g, i)
			if !strings.Contains(rendered, want) {
				t.Errorf("%q never appears at any scroll offset — unreachable", want)
			}
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

	groups := m.whichKeyLeaderGroups()
	lay, ok := m.whichKeyLayoutFor(groups)
	if !ok || lay.maxScroll == 0 {
		t.Fatalf("precondition: the full catalog must overflow at 80x24; maxScroll=%d", lay.maxScroll)
	}
	bg := strings.Repeat("\n", m.height)
	atEnd := m.renderWhichKeyPanel(bg, groups, lay.maxScroll)
	if beyond := m.renderWhichKeyPanel(bg, groups, lay.maxScroll+50); beyond != atEnd {
		t.Fatal("an offset past the end must clamp to the last screen")
	}
	if before := m.renderWhichKeyPanel(bg, groups, -5); before != m.renderWhichKeyPanel(bg, groups, 0) {
		t.Fatal("a negative offset must clamp to the top")
	}
}

// TestRenderWhichKeyPanel_ColumnsAreUniformAndAligned is the visual property the
// neovim port exists for: every column is the same width, so the keys line up
// in one vertical edge across group boundaries.
func TestRenderWhichKeyPanel_ColumnsAreUniformAndAligned(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := whichKeyTestModel()
	m.width, m.height = 120, 40

	groups := m.whichKeyLeaderGroups()
	lay, ok := m.whichKeyLayoutFor(groups)
	if !ok {
		t.Fatal("precondition: the panel must lay out at 120x40")
	}
	if lay.grid.boxN < 2 {
		t.Fatalf("precondition: 120 columns must fit at least 2 grid columns, got %d", lay.grid.boxN)
	}

	out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), groups, 0))
	sepCols := map[int]bool{}
	for line := range strings.SplitSeq(out, "\n") {
		body, ok := strings.CutPrefix(strings.TrimSuffix(line, "│"), "│")
		if !ok {
			continue
		}
		for idx := 0; ; {
			i := strings.Index(body[idx:], whichKeySep)
			if i < 0 {
				break
			}
			sepCols[idx+i] = true
			idx += i + len(whichKeySep)
		}
	}
	// One separator column per grid column, at a fixed offset each.
	if len(sepCols) != lay.grid.boxN {
		t.Fatalf("separators sit at %d distinct columns, want exactly %d (one per grid column): %v",
			len(sepCols), lay.grid.boxN, sepCols)
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
	groups := []whichKeyGroupCells{
		{Cells: []whichKeyCell{{"Y", "Copy as (YAML/JSON/table)"}}},
	}
	for _, w := range []int{20, 25, 34} {
		m.width = w
		out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), groups, 0))
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
				_, _ = m.whichKeyLayoutFor(m.whichKeyLeaderGroups())
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
	lay, ok := m.whichKeyLayoutFor(m.whichKeyLeaderGroups())
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
			_, _ = m.whichKeyLayoutFor(m.whichKeyLeaderGroups())
		}
	})
	allocs := float64(res.MemAllocs) / float64(res.N)
	if allocs > wkLeaderLayoutAllocThreshold {
		t.Fatalf("whichKeyLayoutFor allocates %.0f times per call (threshold %d); it must stay a plain row count, never a styled render", allocs, wkLeaderLayoutAllocThreshold)
	}
	t.Logf("whichKeyLayoutFor: %.0f allocs/op (threshold %d)", allocs, wkLeaderLayoutAllocThreshold)
}
