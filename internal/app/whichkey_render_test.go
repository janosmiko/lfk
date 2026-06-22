package app

import (
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/ui"
)

func TestRenderWhichKey_ListsTargets(t *testing.T) {
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyDelayMs = 0
	m := gotoTestModel()
	m.pendingG = true
	m.whichKeyShown = true
	out := stripANSI(m.renderWhichKey(strings.Repeat("\n", m.height)))
	if !strings.Contains(out, "Pods") || !strings.Contains(out, "gp") {
		t.Fatalf("which-key popup missing goto targets:\n%s", out)
	}
}

func TestRenderWhichKey_HiddenWhenDisabled(t *testing.T) {
	ui.ConfigWhichKeyEnabled = false
	m := gotoTestModel()
	m.pendingG = true
	m.whichKeyShown = true
	bg := strings.Repeat("\n", m.height)
	if got := m.renderWhichKey(bg); got != bg {
		t.Fatal("popup must not render when which_key_enabled is false")
	}
}
