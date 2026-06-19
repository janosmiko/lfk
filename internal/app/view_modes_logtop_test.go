package app

import (
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/logagg"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestViewLogTop_RendersBuffer(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogTop
	m.logView.rawLines = []string{
		`2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/api/users","DownstreamStatus":200}`,
		`2026-06-18T10:00:01Z {"RequestMethod":"GET","RequestPath":"/api/users","DownstreamStatus":500}`,
	}
	m.logTopResetAndParse()
	m.logTop.title = "Log Top: deploy/web"
	out := stripANSI(m.viewLogTop())
	if !strings.Contains(out, "/api/users") {
		t.Error("expected group rendered in viewLogTop")
	}
	_ = logagg.FieldMethod
}

// TestLogTopHintBar_DrillHint verifies the enter hint shows "group by <dim>"
// when a next-drill dim exists, and "drill" otherwise.
func TestLogTopHintBar_DrillHint(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogTop
	// Traefik JSON with method, path, status so multiple dims are available for drill.
	m.logView.rawLines = []string{
		`2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/api","DownstreamStatus":200,"RequestHost":"a.example.com"}`,
		`2026-06-18T10:00:01Z {"RequestMethod":"GET","RequestPath":"/api","DownstreamStatus":500,"RequestHost":"b.example.com"}`,
	}
	m.logTopResetAndParse()
	m.width = 300 // wide enough to show all hints including "enter: group by <dim>"

	// With method+path as groupBy and status/host available, next drill dim exists.
	next := m.logTopNextDrillDim()
	if next == "" {
		t.Fatal("expected a next drill dim to exist after parsing traefik logs with multiple fields")
	}
	hint := stripANSI(m.logTopHintBar())
	// The enter hint should read "group by <dimname>".
	if !strings.Contains(hint, "group by "+next) {
		t.Errorf("hint bar should contain 'group by %s' when next drill dim is %q; got: %s", next, next, hint)
	}

	// When all displayDims are pinned, next drill dim should be "" and enter shows "drill".
	m2 := m
	m2.logTop.displayDims = m2.logTop.groupBy // all dims are already in groupBy -> nothing to drill
	hint2 := stripANSI(m2.logTopHintBar())
	// The enter hint should be "drill", not "group by <dim>".
	if strings.Contains(hint2, "group by "+logagg.FieldStatus) || strings.Contains(hint2, "group by "+logagg.FieldHost) {
		t.Errorf("hint bar enter hint should not be 'group by <dim>' when no next drill dim; got: %s", hint2)
	}
	if !strings.Contains(hint2, "drill") {
		t.Errorf("hint bar enter hint should be 'drill' when no next drill dim; got: %s", hint2)
	}

	_ = ui.RenderHintBar // keep import
}
