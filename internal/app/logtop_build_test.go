package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/logagg"
	"github.com/janosmiko/lfk/internal/ui"
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

func TestLogTopAddLine_IncrementalEqualsBatch(t *testing.T) {
	lines := []string{
		`2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/a","DownstreamStatus":200}`,
		`2026-06-18T10:00:01Z {"RequestMethod":"GET","RequestPath":"/a","DownstreamStatus":500}`,
		`2026-06-18T10:00:02Z {"RequestMethod":"POST","RequestPath":"/b","DownstreamStatus":200}`,
		`2026-06-18T10:00:03Z {"RequestMethod":"GET","RequestPath":"/a","DownstreamStatus":200}`,
	}
	// Incremental path.
	inc := &Model{}
	inc.logView.rawLines = []string{lines[0]}
	inc.logTopResetAndParse()
	for _, ln := range lines[1:] {
		inc.logTopAddLine(ln)
	}
	// Batch path: parse all at once.
	batch := &Model{}
	batch.logView.rawLines = append([]string(nil), lines...)
	batch.logTopResetAndParse()

	if len(inc.logTop.rows) != len(batch.logTop.rows) {
		t.Fatalf("row count incremental=%d batch=%d", len(inc.logTop.rows), len(batch.logTop.rows))
	}
	for i := range inc.logTop.rows {
		if inc.logTop.rows[i].Count != batch.logTop.rows[i].Count ||
			inc.logTop.rows[i].ErrCount != batch.logTop.rows[i].ErrCount {
			t.Errorf("row %d mismatch: inc=%+v batch=%+v", i, inc.logTop.rows[i], batch.logTop.rows[i])
		}
	}
}

func TestLogTopCapParsed_BoundsMemory(t *testing.T) {
	orig := ui.ConfigLogMaxLines
	ui.ConfigLogMaxLines = 100
	defer func() { ui.ConfigLogMaxLines = orig }()

	m := &Model{}
	m.logView.rawLines = []string{`2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/a","DownstreamStatus":200}`}
	m.logTopResetAndParse()
	for range ui.ConfigLogMaxLines + logBufferTrimSlack + 50 {
		m.logTopAddLine(`2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/a","DownstreamStatus":200}`)
	}
	if len(m.logTop.parsed) > ui.ConfigLogMaxLines+logBufferTrimSlack {
		t.Fatalf("parsed not bounded: len=%d cap=%d", len(m.logTop.parsed), ui.ConfigLogMaxLines+logBufferTrimSlack)
	}
	// Counts still reflect the retained window (all identical lines -> one row).
	if len(m.logTop.rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(m.logTop.rows))
	}
}

func BenchmarkLogTopIngest(b *testing.B) {
	line := `2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/a","DownstreamStatus":200}`
	m := &Model{}
	m.logView.rawLines = []string{line}
	m.logTopResetAndParse()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		m.logTopAddLine(line)
	}
}
