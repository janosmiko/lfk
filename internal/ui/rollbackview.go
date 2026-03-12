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

	boxW := screenWidth * 70 / 100
	if boxW < 50 {
		boxW = 50
	}
	boxH := screenHeight * 60 / 100
	if boxH < 10 {
		boxH = 10
	}

	title := OverlayTitleStyle.Render("Rollback Deployment")

	var lines []string
	// Header.
	hdr := fmt.Sprintf("  %-8s  %-30s  %-8s  %-30s  %s", "REV", "REPLICASET", "PODS", "IMAGE", "AGE")
	lines = append(lines, DimStyle.Bold(true).Render(hdr))

	contentH := boxH - 6 // borders, title, help
	if contentH < 3 {
		contentH = 3
	}
	start := 0
	if cursor >= contentH {
		start = cursor - contentH + 1
	}
	end := start + contentH
	if end > len(revisions) {
		end = len(revisions)
	}

	for i := start; i < end; i++ {
		rev := revisions[i]
		img := ""
		if len(rev.Images) > 0 {
			img = rev.Images[0]
			if len(rev.Images) > 1 {
				img += fmt.Sprintf(" +%d", len(rev.Images)-1)
			}
		}
		age := formatAge(rev.CreatedAt)
		line := fmt.Sprintf("  %-8d  %-30s  %-8d  %-30s  %s",
			rev.Revision, Truncate(rev.Name, 30), rev.Replicas, Truncate(img, 30), age)

		if i == cursor {
			lines = append(lines, OverlaySelectedStyle.Render(Truncate(line, boxW-6)))
		} else {
			lines = append(lines, OverlayNormalStyle.Render(Truncate(line, boxW-6)))
		}
	}

	helpLine := HelpKeyStyle.Render("jk") + DimStyle.Render(" nav") + "  " +
		HelpKeyStyle.Render("Enter") + DimStyle.Render(" rollback") + "  " +
		HelpKeyStyle.Render("esc") + DimStyle.Render(" cancel")

	body := title + "\n" + strings.Join(lines, "\n") + "\n\n" + helpLine

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
func RenderHelmRollbackOverlay(revisions []HelmRevision, cursor int, screenWidth, screenHeight int) string {
	if len(revisions) == 0 {
		return OverlayStyle.Render(DimStyle.Render("No revisions found"))
	}

	boxW := screenWidth * 80 / 100
	if boxW < 60 {
		boxW = 60
	}
	boxH := screenHeight * 60 / 100
	if boxH < 10 {
		boxH = 10
	}

	title := OverlayTitleStyle.Render("Rollback Helm Release")

	var lines []string
	// Header.
	hdr := fmt.Sprintf("  %-6s  %-12s  %-25s  %-12s  %-30s  %s",
		"REV", "STATUS", "CHART", "APP VER", "DESCRIPTION", "UPDATED")
	lines = append(lines, DimStyle.Bold(true).Render(hdr))

	contentH := boxH - 6 // borders, title, help
	if contentH < 3 {
		contentH = 3
	}
	start := 0
	if cursor >= contentH {
		start = cursor - contentH + 1
	}
	end := start + contentH
	if end > len(revisions) {
		end = len(revisions)
	}

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

	helpLine := HelpKeyStyle.Render("jk") + DimStyle.Render(" nav") + "  " +
		HelpKeyStyle.Render("Enter") + DimStyle.Render(" rollback") + "  " +
		HelpKeyStyle.Render("esc") + DimStyle.Render(" cancel")

	body := title + "\n" + strings.Join(lines, "\n") + "\n\n" + helpLine

	return OverlayStyle.
		Width(boxW).
		Render(body)
}

// formatAge returns a human-readable age string.
func formatAge(t time.Time) string {
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
