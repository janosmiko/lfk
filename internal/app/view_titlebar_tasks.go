package app

import (
	"fmt"

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
	// Count tasks that are NOT already shown by renderMutationProgress
	// (mutation tasks with Total > 0 get their own progress indicator).
	n := 0
	for _, t := range snapshot {
		if t.Kind == bgtasks.KindMutation && t.Total > 0 {
			continue
		}
		n++
	}
	if n == 0 {
		return ""
	}
	return ui.BarDimStyle.Render(" " + spinnerFrame + " ")
}

// renderMutationProgress returns a styled progress string for active
// KindMutation tasks that have a non-zero Total. Shown in the title bar
// to the left of the background tasks spinner to give real-time feedback
// during bulk operations (e.g., "Deleting 3/10 ⠋").
//
// When multiple mutation tasks run concurrently, only the first one with
// progress is shown (bulk operations are typically serial).
// Returns empty string when no mutation task has progress.
func renderMutationProgress(spinnerFrame string, snapshot []bgtasks.Task) string {
	for _, t := range snapshot {
		if t.Kind != bgtasks.KindMutation || t.Total == 0 {
			continue
		}
		label := shortMutationLabel(t.Name)
		text := fmt.Sprintf(" %s %d/%d %s ", label, t.Current, t.Total, spinnerFrame)
		return ui.StatusMessageOkStyle.Render(text)
	}
	return ""
}

// shortMutationLabel extracts the verb from a mutation task name like
// "Delete pods (10)" -> "Deleting", "Scale deployments (3)" -> "Scaling".
// Falls back to the full name if the pattern is unrecognized.
func shortMutationLabel(name string) string {
	switch {
	case len(name) >= 6 && name[:6] == "Delete":
		return "Deleting"
	case len(name) >= 12 && name[:12] == "Force delete":
		return "Force deleting"
	case len(name) >= 5 && name[:5] == "Scale":
		return "Scaling"
	case len(name) >= 7 && name[:7] == "Restart":
		return "Restarting"
	case len(name) >= 5 && name[:5] == "Patch":
		return "Patching"
	default:
		return name
	}
}
