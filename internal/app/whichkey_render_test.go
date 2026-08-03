package app

import (
	"fmt"
	"regexp"
	"strconv"
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
	out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), groups, ""))
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
	if got := m.renderWhichKeyPanel(bg, nil, ""); got != bg {
		t.Fatal("an empty group list must render nothing, not an empty box")
	}
	empty := []whichKeyGroupCells{{Title: "Actions", Cells: nil}}
	if got := m.renderWhichKeyPanel(bg, empty, ""); got != bg {
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
	if got := m.renderWhichKeyPanel(bg, groups, ""); got != bg {
		t.Fatal("panel must be skipped when the terminal is too small")
	}
}

func TestRenderWhichKeyPanel_OverflowShowsMoreFooter(t *testing.T) {
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
	out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), groups, ""))
	if !strings.Contains(out, "more") {
		t.Fatalf("overflow must be disclosed with a 'more' footer, never silently truncated:\n%s", out)
	}
	if lines := strings.Split(out, "\n"); len(lines) > m.height+1 {
		t.Fatalf("panel overflowed the screen: %d lines for height %d", len(lines), m.height)
	}

	// The footer count must match ground truth exactly, not just be present:
	// derive "actually shown" independently by counting visible cell labels
	// (each contains "action", none of the titles do) rather than trusting
	// the footer's own arithmetic.
	total := 6 * 8
	actualShown := strings.Count(out, "action")
	wantHidden := total - actualShown
	match := regexp.MustCompile(`\+(\d+) more`).FindStringSubmatch(out)
	if match == nil {
		t.Fatalf("footer missing a parseable '+N more' count:\n%s", out)
	}
	gotHidden, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatalf("footer count %q not a number: %v", match[1], err)
	}
	if gotHidden != wantHidden {
		t.Fatalf("footer says +%d more but %d of %d cells are actually visible (want +%d more)",
			gotHidden, actualShown, total, wantHidden)
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
		out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), groups, ""))
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

// BenchmarkWhichKeyLeaderRender measures the per-frame cost of the panel. The
// paging path lays out the whole catalog and measures every cell's display
// width, so it is the render hot spot the alloc guard below protects.
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
		b.Run(s.name+"/Page", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _, _ = m.whichKeyLeaderPage()
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

// wkLeaderPageAllocThreshold caps allocations for one pagination pass at
// 80x24 with the full catalog. There is an equivalent ceiling on the
// availability pass (TestAvailableWhichKeyActions_ResolvesRowOncePerCall) but
// none on rendering, which is where an O(n^2) width scan and a triple layout
// pass crept in unnoticed. Measured at 274 allocs/op after that fix; the
// threshold sits ~3.6x above it so CI hardware variance and unrelated drift
// can't flake it, while a reintroduced per-cell re-measure or re-layout
// (which was 409 allocs/op and 5x the wall time) still trips it.
const wkLeaderPageAllocThreshold = 1000

func TestWhichKeyLeaderPage_AllocationCeiling(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true

	m := whichKeyRenderBenchModel(80, 24)
	res := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_, _, _ = m.whichKeyLeaderPage()
		}
	})
	allocs := float64(res.MemAllocs) / float64(res.N)
	if allocs > wkLeaderPageAllocThreshold {
		t.Fatalf("whichKeyLeaderPage allocates %.0f times per call (threshold %d); the panel re-lays out on every frame, so a per-cell re-layout or a repeated width scan here costs a full render", allocs, wkLeaderPageAllocThreshold)
	}
	t.Logf("whichKeyLeaderPage: %.0f allocs/op (threshold %d)", allocs, wkLeaderPageAllocThreshold)
}

// TestFitCellsRows_RowCountIsNotMonotonic pins the counterexample that makes a
// binary search (or an early break) in fitCellsRows unsound. Adding a cell can
// let a wider column count fit, dropping the row count outright — so a search
// that assumes "once it stops fitting, it never fits again" silently truncates
// a page. Guarding the property directly, rather than only the search, is what
// stops the optimization from being reintroduced.
func TestFitCellsRows_RowCountIsNotMonotonic(t *testing.T) {
	widths := make([]int, 9)
	for i := range widths {
		widths[i] = 60 - i
	}
	const targetInner, maxInner = 60, 118

	rowsAt := func(k int) int { return layoutWhichKey(widths[:k], targetInner, maxInner).rows }
	if got := rowsAt(8); got != 8 {
		t.Fatalf("k=8 rows = %d, want 8 (single column)", got)
	}
	if got := rowsAt(9); got != 5 {
		t.Fatalf("k=9 rows = %d, want 5 (two columns finally fit)", got)
	}

	// The scan must therefore find k=9, not stop at the last monotonic fit.
	k, rows := fitCellsRows(widths, 5, targetInner, maxInner)
	if k != 9 || rows != 5 {
		t.Fatalf("fitCellsRows = (k=%d rows=%d), want (k=9 rows=5); a non-linear search skipped the real best fit", k, rows)
	}
}

// TestFitCellsRows_MatchesBruteForce is the broader sweep behind that one
// counterexample: over varied width shapes, terminal widths and row budgets,
// the helper must return the true largest fitting prefix.
func TestFitCellsRows_MatchesBruteForce(t *testing.T) {
	bruteForce := func(widths []int, dataRows, targetInner, maxInner int) (int, int) {
		if dataRows <= 0 {
			return 0, 0
		}
		best, bestRows := 0, 0
		for k := 1; k <= len(widths); k++ {
			if lay := layoutWhichKey(widths[:k], targetInner, maxInner); lay.rows <= dataRows {
				best, bestRows = k, lay.rows
			}
		}
		return best, bestRows
	}

	shapes := map[string]func(i int) int{
		"uniform":    func(int) int { return 8 },
		"ascending":  func(i int) int { return 3 + i },
		"descending": func(i int) int { return 60 - i },
		"spiky":      func(i int) int { return 4 + (i*37)%29 },
		"oneWide":    func(i int) int { return map[bool]int{true: 55, false: 5}[i == 7] },
	}

	for name, shape := range shapes {
		for n := 1; n <= 24; n++ {
			widths := make([]int, n)
			for i := range widths {
				widths[i] = shape(i)
			}
			for _, dataRows := range []int{1, 2, 3, 5, 9} {
				for _, inner := range [][2]int{{40, 78}, {60, 118}, {20, 20}} {
					wantK, wantRows := bruteForce(widths, dataRows, inner[0], inner[1])
					gotK, gotRows := fitCellsRows(widths, dataRows, inner[0], inner[1])
					if gotK != wantK || gotRows != wantRows {
						t.Fatalf("%s n=%d dataRows=%d inner=%v: got (k=%d rows=%d), want (k=%d rows=%d)",
							name, n, dataRows, inner, gotK, gotRows, wantK, wantRows)
					}
				}
			}
		}
	}
}
