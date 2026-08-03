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
	m.whichKeyShown = true
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
	m.whichKeyShown = true
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
	m.whichKeyShown = true
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
	out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), groups))
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
	if got := m.renderWhichKeyPanel(bg, nil); got != bg {
		t.Fatal("an empty group list must render nothing, not an empty box")
	}
	empty := []whichKeyGroupCells{{Title: "Actions", Cells: nil}}
	if got := m.renderWhichKeyPanel(bg, empty); got != bg {
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
	if got := m.renderWhichKeyPanel(bg, groups); got != bg {
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
	out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), groups))
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
		out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), groups))
		for line := range strings.SplitSeq(out, "\n") {
			if got := lipgloss.Width(line); got > w {
				t.Fatalf("width %d: rendered line %q is %d columns wide", w, line, got)
			}
		}
	}
}
