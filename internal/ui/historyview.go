package ui

import (
	"fmt"
	"strings"
)

// RenderHelmHistoryOverlay renders a read-only view of Helm release revision
// history. It mirrors the rollback overlay layout so users have a familiar
// view, but with a distinct title and no destructive-action styling. When
// loading is true the overlay shows a loading placeholder so the empty-state
// message is not flashed while the helm history subprocess is still running.
func RenderHelmHistoryOverlay(revisions []HelmRevision, cursor int, screenWidth, screenHeight int, loading bool) string {
	if loading {
		return OverlayStyle.Render(DimStyle.Render("Loading Helm release history..."))
	}
	if len(revisions) == 0 {
		return OverlayStyle.Render(DimStyle.Render("No revisions found"))
	}

	boxW := max(screenWidth*80/100, 60)
	boxH := max(screenHeight*60/100, 10)

	title := OverlayTitleStyle.Render("Helm Release History")

	var lines []string
	hdr := fmt.Sprintf("  %-6s  %-12s  %-25s  %-12s  %-30s  %s",
		"REV", "STATUS", "CHART", "APP VER", "DESCRIPTION", "UPDATED")
	lines = append(lines, DimStyle.Bold(true).Render(hdr))

	contentH := max(boxH-6, 3)
	start := 0
	if cursor >= contentH {
		start = cursor - contentH + 1
	}
	end := min(start+contentH, len(revisions))

	for i := start; i < end; i++ {
		rev := revisions[i]
		line := fmt.Sprintf("  %-6d  %-12s  %-25s  %-12s  %-30s  %s",
			rev.Revision,
			Truncate(rev.Status, 12),
			Truncate(rev.Chart, 25),
			Truncate(rev.AppVersion, 12),
			Truncate(rev.Description, 30),
			Truncate(rev.Updated, 25))

		if i == cursor {
			lines = append(lines, OverlaySelectedStyle.Render(Truncate(line, boxW-6)))
		} else {
			lines = append(lines, OverlayNormalStyle.Render(Truncate(line, boxW-6)))
		}
	}

	body := title + "\n" + strings.Join(lines, "\n")

	return OverlayStyle.
		Width(boxW).
		Render(body)
}
