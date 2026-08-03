package app

import (
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// whichKeyStripBox returns the rendered panel's row indices within a
// full-height render, so tests can assert both its height and where it sits.
func whichKeyStripBox(t *testing.T, m Model) (lines []string, first, last int) {
	t.Helper()
	bg := strings.Repeat("\n", m.height)
	out := stripANSI(m.renderWhichKeyLeader(bg))
	lines = strings.Split(out, "\n")
	first, last = -1, -1
	for i, l := range lines {
		if strings.ContainsAny(l, "╭╮╰╯│") {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		t.Fatalf("no panel rendered at %dx%d:\n%s", m.width, m.height, out)
	}
	return lines, first, last
}

// TestWhichKeyStrip_HeightCappedRegardlessOfTerminal is the user-facing
// complaint the rework fixes: the panel used to grow with the terminal until
// it dominated the screen. It must stay the same short strip at every height.
func TestWhichKeyStrip_HeightCappedRegardlessOfTerminal(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true

	wantHeight := 2 + 2*whichKeyPadV + whichKeyMaxBodyRows // border + padding + body
	for _, h := range []int{24, 40, 60, 120} {
		m := whichKeyTestModel()
		m.width, m.height = 120, h
		m.whichKey.armed = true
		m.whichKey.shown = true

		_, first, last := whichKeyStripBox(t, m)
		if got := last - first + 1; got != wantHeight {
			t.Errorf("height=%d: panel is %d rows, want a fixed %d-row strip", h, got, wantHeight)
		}
	}
}

// TestWhichKeyStrip_SitsDirectlyAboveTheStatusBar pins the anchoring: exactly
// one row (the status bar) is left uncovered below the panel.
func TestWhichKeyStrip_SitsDirectlyAboveTheStatusBar(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true

	for _, h := range []int{24, 40} {
		m := whichKeyTestModel()
		m.width, m.height = 120, h
		m.whichKey.armed = true
		m.whichKey.shown = true

		lines, _, last := whichKeyStripBox(t, m)
		if len(lines) < h {
			t.Fatalf("height=%d: render produced %d rows", h, len(lines))
		}
		// h-2 is the row above the status bar (h-1). Deliberately a literal,
		// not whichKeyBottomGap: an expectation derived from the constant
		// under test would follow it back up to the old five-row gap.
		if want := h - 2; last != want {
			t.Errorf("height=%d: panel's last row is %d, want %d (only the status-bar row uncovered)", h, last, want)
		}
	}
}

// TestWhichKeyStrip_IsWideNotTall: a bottom strip wants columns. Page 1 must
// lay its group out across more than two columns at a normal terminal width.
func TestWhichKeyStrip_IsWideNotTall(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true

	m := whichKeyTestModel()
	m.width, m.height = 120, 40
	m.whichKey.armed = true
	m.whichKey.shown = true

	lines, first, last := whichKeyStripBox(t, m)
	// The panel spans the full width, so its border row must too.
	if got := len([]rune(lines[first])); got != m.width {
		t.Errorf("panel width = %d, want the full %d columns", got, m.width)
	}
	// Count keys on the widest content row: >2 means the grid really went wide.
	widest := 0
	for _, l := range lines[first : last+1] {
		if n := len(strings.Fields(l)); n > widest {
			widest = n
		}
	}
	if widest < 6 {
		t.Errorf("widest content row has only %d tokens; the strip should pack several columns", widest)
	}
}

// TestWhichKeyPanels_NeverDimTheBackground covers both which-key surfaces: the
// leader strip and the g-prefix goto popup. They annotate the list behind them
// rather than taking over, so ConfigDimOverlay must not apply — while it still
// applies to real overlays (TestRenderOverlayDimAppliesWhenEnabled).
func TestWhichKeyPanels_NeverDimTheBackground(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigDimOverlay = true

	bg := strings.Repeat("explorer row\n", 40)

	leader := whichKeyTestModel()
	leader.width, leader.height = 120, 40
	leader.whichKey.armed = true
	leader.whichKey.shown = true
	if out := leader.renderWhichKeyLeader(bg); strings.Contains(out, "\x1b[2m") {
		t.Error("the leader strip must not dim the background")
	}

	goto_ := gotoTestModel()
	goto_.width, goto_.height = 120, 40
	goto_.nav.Level = model.LevelResources
	goto_.pendingG = true
	goto_.whichKey.shown = true
	if out := goto_.renderWhichKey(bg); out == bg {
		t.Fatal("precondition: the goto popup must render")
	} else if strings.Contains(out, "\x1b[2m") {
		t.Error("the goto popup must not dim the background")
	}
}
