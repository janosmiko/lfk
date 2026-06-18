package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderLogTopView_NextDrillMarker(t *testing.T) {
	prev := ActiveLogTopNextDrill
	t.Cleanup(func() { ActiveLogTopNextDrill = prev })

	rows := []LogTopRow{{Dims: map[string]string{"method": "GET", "path": "/a", "status": "200"}, Count: 10}}
	dims := []string{"method", "path", "status"}
	metrics := []string{"REQ", "REQ/s", "%", "ERR"}

	// With a next-drill target set, that column's header carries the marker.
	ActiveLogTopNextDrill = "status"
	out := stripANSI(RenderLogTopView("t", dims, metrics, rows, 1.0, 10, 0, 0, "hint", 120, 30))
	if !strings.Contains(out, logTopDrillMarker+"STATUS") {
		t.Errorf("expected next-drill marker on STATUS header; got header line not containing %q", logTopDrillMarker+"STATUS")
	}

	// With no target, no marker is rendered.
	ActiveLogTopNextDrill = ""
	out = stripANSI(RenderLogTopView("t", dims, metrics, rows, 1.0, 10, 0, 0, "hint", 120, 30))
	if strings.Contains(out, logTopDrillMarker) {
		t.Errorf("expected no drill marker when ActiveLogTopNextDrill is empty")
	}
}

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
	metrics := []string{"REQ", "REQ/s", "%", "ERR"}
	out := RenderLogTopView("Log Top: deploy/web", dims, metrics, rows, 4.7, 5000, 0, 0, "hint", 100, 30)
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

// TestRenderLogTopView_LatencyColumns verifies that passing P95/P99 in metrics renders
// P95/P99 headers and that rows with P95/P99 < 0 show "n/a".
func TestRenderLogTopView_LatencyColumns(t *testing.T) {
	dims := []string{"method", "path"}
	rows := []LogTopRow{
		{Dims: map[string]string{"method": "GET", "path": "/api"}, Count: 10, ErrCount: 0, Pct: 100, P95: 13, P99: 610},
		{Dims: map[string]string{"method": "POST", "path": "/nodur"}, Count: 5, ErrCount: 0, Pct: 50, P95: -1, P99: -1},
	}
	metrics := []string{"REQ", "REQ/s", "%", "ERR", "P95", "P99"}
	out := RenderLogTopView("title", dims, metrics, rows, 1.0, 10, 0, 0, "hint", 120, 30)
	plain := stripANSI(out)
	if !strings.Contains(plain, "P95") {
		t.Error("expected P95 header when metrics includes P95")
	}
	if !strings.Contains(plain, "P99") {
		t.Error("expected P99 header when metrics includes P99")
	}
	if !strings.Contains(plain, "n/a") {
		t.Error("expected n/a for row with P95/P99 = -1")
	}
	if !strings.Contains(plain, "13") {
		t.Error("expected 13ms P95 rendered")
	}
	if !strings.Contains(plain, "610") {
		t.Error("expected 610ms P99 rendered")
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
	metrics := []string{"REQ", "REQ/s", "%", "ERR"}
	for _, width := range []int{80, 100, 120, 200} {
		out := RenderLogTopView("Log Top: deploy/web", dims, metrics, rows, 4.7, 5000, 0, 0, "hint", width, 30)
		for line := range strings.SplitSeq(stripANSI(out), "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("width=%d: line exceeds terminal width (%d): %q", width, w, line)
			}
		}
	}
	// Also verify latency columns do not overflow.
	latRows := []LogTopRow{
		{
			Dims:     map[string]string{"method": "GET", "path": "/api/users/with/a/fairly/long/path/segment", "status": "200"},
			Count:    1180,
			ErrCount: 12,
			Pct:      23.6,
			P95:      13,
			P99:      610,
		},
	}
	latMetrics := []string{"REQ", "REQ/s", "%", "ERR", "P95", "P99"}
	for _, width := range []int{80, 100, 120, 200} {
		out := RenderLogTopView("Log Top: deploy/web", dims, latMetrics, latRows, 4.7, 5000, 0, 0, "hint", width, 30)
		for line := range strings.SplitSeq(stripANSI(out), "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("latency width=%d: line exceeds terminal width (%d): %q", width, w, line)
			}
		}
	}
}
