package ui

import (
	"strings"
	"testing"
)

func TestRenderLogTopView_ShowsRowsAndRate(t *testing.T) {
	rows := []LogTopRow{
		{Key: "GET /api/users", Count: 1180, ErrCount: 12, Pct: 23.6},
		{Key: "GET /health", Count: 980, ErrCount: 0, Pct: 19.6},
	}
	out := RenderLogTopView("Log Top: deploy/web", []string{"method+path"}, rows, 4.7, 5000, 0, 0, "hint", 100, 30)
	plain := stripANSI(out)
	if !strings.Contains(plain, "GET /api/users") {
		t.Error("expected first group rendered")
	}
	if !strings.Contains(plain, "1180") {
		t.Error("expected REQ count rendered")
	}
	if !strings.Contains(plain, "REQ/s") {
		t.Error("expected REQ/s column header")
	}
}
