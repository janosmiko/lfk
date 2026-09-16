package app

import (
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// The goto popup annotates the list behind it rather than taking it over, so
// ConfigDimOverlay must not apply to it (contrast with
// TestRenderOverlayDimAppliesWhenEnabled).
func TestWhichKeyGotoPopup_NeverDimsTheBackground(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigDimOverlay = true

	bg := strings.Repeat("explorer row\n", 40)

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
