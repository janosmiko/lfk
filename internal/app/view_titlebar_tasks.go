package app

import (
	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/ui"
)

// renderTasksIndicator returns the styled string that lives in the title
// bar between the gap filler and the namespace badge. Empty string when
// no tasks are visible — the title bar then renders no indicator at
// all and the breadcrumb gets the full remaining width.
//
// The indicator is intentionally minimal: just the animated spinner
// glyph. Users who want details open the :tasks overlay. The spinner
// frame is passed in by the caller (typically m.spinner.View()), so the
// indicator animates at whatever cadence the caller's spinner is
// already running.
func renderTasksIndicator(spinnerFrame string, snapshot []bgtasks.Task) string {
	if len(snapshot) == 0 {
		return ""
	}
	return ui.BarDimStyle.Render(" " + spinnerFrame + " ")
}
