package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderLogTopView_ShowsRowsAndRate(t *testing.T) {
	rows := []LogTopRow{
		{
			Dims:     map[string]string{"method": "GET", "path": "/api/users", "status": "200"},
			Count:    1180,
			ErrCount: 12,
			Pct:      23.6,
		},
		{
			Dims:     map[string]string{"method": "GET", "path": "/health", "status": "200"},
			Count:    980,
			ErrCount: 0,
			Pct:      19.6,
		},
	}
	dims := []string{"method", "path", "status"}
	out := RenderLogTopView("Log Top: deploy/web", dims, rows, 4.7, 5000, 0, 0, "hint", 100, 30)
	plain := stripANSI(out)
	if !strings.Contains(plain, "GET") {
		t.Error("expected method GET rendered")
	}
	if !strings.Contains(plain, "/api/users") {
		t.Error("expected path /api/users rendered")
	}
	if !strings.Contains(plain, "200") {
		t.Error("expected status 200 rendered")
	}
	if !strings.Contains(plain, "1180") {
		t.Error("expected REQ count rendered")
	}
	if !strings.Contains(plain, "REQ/s") {
		t.Error("expected REQ/s column header")
	}
}

// TestRenderLogTopView_NoLineOverflow guards against the ERR column wrapping:
// every rendered line must fit within the terminal width so the bordered box
// does not wrap the header/rows.
func TestRenderLogTopView_NoLineOverflow(t *testing.T) {
	rows := []LogTopRow{
		{
			Dims: map[string]string{
				"method": "GET",
				"path":   "/api/users/with/a/fairly/long/path/segment",
				"status": "200",
			},
			Count:    1180,
			ErrCount: 12,
			Pct:      23.6,
		},
	}
	dims := []string{"method", "path", "status"}
	for _, width := range []int{80, 100, 120, 200} {
		out := RenderLogTopView("Log Top: deploy/web", dims, rows, 4.7, 5000, 0, 0, "hint", width, 30)
		for line := range strings.SplitSeq(stripANSI(out), "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("width=%d: line exceeds terminal width (%d): %q", width, w, line)
			}
		}
	}
}
