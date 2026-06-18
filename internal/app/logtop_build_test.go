package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/logagg"
)

func TestLogTopResetAndParse_Traefik(t *testing.T) {
	m := &Model{}
	m.logView.rawLines = []string{
		`2026-06-18T10:00:00Z {"DownstreamStatus":200,"RequestMethod":"GET","RequestPath":"/a"}`,
		`2026-06-18T10:00:01Z {"DownstreamStatus":500,"RequestMethod":"GET","RequestPath":"/a"}`,
		`2026-06-18T10:00:02Z not parseable line`,
	}
	m.logTopResetAndParse()

	if m.logTop.profile != logagg.ProfileTraefikJSON {
		t.Errorf("profile = %q, want traefik-json", m.logTop.profile)
	}
	if len(m.logTop.parsed) != 2 {
		t.Errorf("parsed = %d, want 2", len(m.logTop.parsed))
	}
	if m.logTop.unmatched != 1 {
		t.Errorf("unmatched = %d, want 1", m.logTop.unmatched)
	}
	if len(m.logTop.rows) != 1 {
		t.Fatalf("rows = %d, want 1 (GET /a)", len(m.logTop.rows))
	}
	if m.logTop.rows[0].Count != 2 || m.logTop.rows[0].ErrCount != 1 {
		t.Errorf("row count=%d err=%d, want 2/1", m.logTop.rows[0].Count, m.logTop.rows[0].ErrCount)
	}
}

func TestLogTopAddLine_Incremental(t *testing.T) {
	m := &Model{}
	m.logView.rawLines = []string{`2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/a","DownstreamStatus":200}`}
	m.logTopResetAndParse()
	m.logTopAddLine(`2026-06-18T10:00:05Z {"RequestMethod":"GET","RequestPath":"/a","DownstreamStatus":200}`)
	if m.logTop.rows[0].Count != 2 {
		t.Errorf("count after add = %d, want 2", m.logTop.rows[0].Count)
	}
	if rps := m.logTopReqPerSec(); rps <= 0 {
		t.Errorf("req/s = %v, want > 0 over a 5s span", rps)
	}
}
