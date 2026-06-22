package app

import (
	"strings"
	"testing"

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
