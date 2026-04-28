package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
)

// RenderRollbackOverlay renders the revision history picker for rollback.
func RenderRollbackOverlay(revisions []k8s.DeploymentRevision, cursor int, screenWidth, screenHeight int) string {
	if len(revisions) == 0 {
		return OverlayStyle.Render(DimStyle.Render("No revisions found"))
	}

	boxW := max(screenWidth*70/100, 50)
	boxH := max(screenHeight*60/100, 10)

	title := OverlayTitleStyle.Render("Rollback Deployment")

	var lines []string
	// Header.
	hdr := fmt.Sprintf("  %-8s  %-30s  %-8s  %-30s  %s", "REV", "REPLICASET", "PODS", "IMAGE", "AGE")
	lines = append(lines, DimStyle.Bold(true).Render(hdr))

	contentH := max(
		// borders, title, help
		boxH-6, 3)
	start := 0
	if cursor >= contentH {
		start = cursor - contentH + 1
	}
	end := min(start+contentH, len(revisions))

	for i := start; i < end; i++ {
		rev := revisions[i]
		img := ""
		if len(rev.Images) > 0 {
			img = rev.Images[0]
			if len(rev.Images) > 1 {
				img += fmt.Sprintf(" +%d", len(rev.Images)-1)
			}
		}
		age := FormatAge(rev.CreatedAt)
		line := fmt.Sprintf("  %-8d  %-30s  %-8d  %-30s  %s",
			rev.Revision, Truncate(rev.Name, 30), rev.Replicas, Truncate(img, 30), age)

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

// HelmRevision represents a single entry from `helm history` output.
type HelmRevision struct {
	Revision    int
	Status      string
	Chart       string
	AppVersion  string
	Description string
	Updated     string
}

// RenderHelmRollbackOverlay renders the Helm release revision history picker for rollback.
// When loading is true the overlay shows a loading placeholder instead of the
// empty-state message so users do not see a misleading "No revisions found"
// while the helm history subprocess is still running.
func RenderHelmRollbackOverlay(revisions []HelmRevision, cursor int, screenWidth, screenHeight int, loading bool) string {
	if loading {
		return OverlayStyle.Render(DimStyle.Render("Loading Helm release history..."))
	}
	if len(revisions) == 0 {
		return OverlayStyle.Render(DimStyle.Render("No revisions found"))
	}

	boxW := max(screenWidth*80/100, 60)
	boxH := max(screenHeight*60/100, 10)

	title := OverlayTitleStyle.Render("Rollback Helm Release")

	var lines []string
	// Header.
	hdr := fmt.Sprintf("  %-6s  %-12s  %-25s  %-12s  %-30s  %s",
		"REV", "STATUS", "CHART", "APP VER", "DESCRIPTION", "UPDATED")
	lines = append(lines, DimStyle.Bold(true).Render(hdr))

	contentH := max(
		// borders, title, help
		boxH-6, 3)
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

// FormatAge returns a human-readable age string.
func FormatAge(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
