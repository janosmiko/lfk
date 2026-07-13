// Package app - dashboard_pinned.go
// PINNED SUMMARIES section of the cluster dashboard (issue #525): per-kind
// status rollups the user pinned from the resource-type action menu, rendered
// with the same worst-first bars as the resource-type preview band.
package app

import (
	"sort"
	"strings"

	"github.com/janosmiko/lfk/internal/ui"
)

// dashboardPinnedSection renders one block per pinned resource type, in pin
// order. A pin whose type is absent from this cluster's discovery renders a
// dim "(not installed)" note; a failed list call renders "(unavailable)".
func dashboardPinnedSection(lines []string, data dashboardData, w dashboardWidths) []string {
	if len(data.pinnedSummaries) == 0 {
		return lines
	}
	// The fan-out delivers sections in arrival order; sort a copy by pin
	// index (never in place - the slice is shared via m.dashboardData).
	results := append([]pinnedSummaryResult(nil), data.pinnedSummaries...)
	sort.Slice(results, func(i, j int) bool { return results[i].index < results[j].index })

	lines = append(lines, ui.DimStyle.Render("  "+strings.Repeat("─", w.sep)))
	lines = append(lines, ui.DimStyle.Bold(true).Render("  PINNED SUMMARIES"))
	lines = append(lines, "")
	for _, r := range results {
		label := r.displayName
		if label == "" {
			label = r.key
		}
		switch {
		case r.notFound:
			lines = append(lines, "  "+ui.DimStyle.Bold(true).Render(label)+" "+ui.DimStyle.Render("(not installed in this cluster)"))
		case r.err != nil:
			lines = append(lines, "  "+ui.DimStyle.Bold(true).Render(label)+" "+ui.StatusProgressing.Render("(unavailable)"))
		default:
			for ln := range strings.SplitSeq(ui.RenderKindSummary(r.summary, label, w.content-2), "\n") {
				lines = append(lines, "  "+ln)
			}
		}
		lines = append(lines, "")
	}
	return lines
}
