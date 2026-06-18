package app

import (
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/logagg"
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
