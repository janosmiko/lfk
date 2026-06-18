package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/logagg"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestLogTopResetAndParse_PrefixedCLF covers the real-world case: a Deployment
// streamed with kubectl --all-containers --prefix --timestamps, where Traefik
// logs in Common Log Format. Each rawLine is
// "[pod/<pod>/<container>] <RFC3339> <CLF>". The pod prefix and timestamp must
// be stripped before the nginx/CLF parser sees the body.
func TestLogTopResetAndParse_PrefixedCLF(t *testing.T) {
	orig := ui.ConfigLogTopDefaultProfile
	ui.ConfigLogTopDefaultProfile = "auto"
	defer func() { ui.ConfigLogTopDefaultProfile = orig }()

	m := &Model{}
	m.logView.rawLines = []string{
		`[pod/traefik-99f4b987c-6rnkn/traefik] 2026-06-18T15:18:46.679395292Z 10.42.2.19 - - [18/Jun/2026:15:18:46 +0000] "POST /api/v4/jobs/request HTTP/1.1" 204 0 "-" "-" 8086 "websecure-gitlab@kubernetes" "http://10.42.13.162:8181" 2ms`,
		`[pod/traefik-99f4b987c-6rnkn/traefik] 2026-06-18T15:18:48.031270306Z 10.42.2.2 - - [18/Jun/2026:15:18:48 +0000] "POST /api/v4/jobs/request HTTP/1.1" 204 0 "-" "-" 8088 "websecure-gitlab@kubernetes" "http://10.42.15.65:8181" 1ms`,
		`[pod/traefik-99f4b987c-6rnkn/traefik] 2026-06-18T15:18:49.068676261Z 10.42.8.184 - - [18/Jun/2026:15:18:49 +0000] "HEAD /v2/m2community/magento/manifests/feature-ai-modules HTTP/1.1" 401 0 "-" "-" 8090 "websecure-harbor@kubernetes" "http://10.42.20.197:8080" 5ms`,
	}
	m.logTopResetAndParse()

	if m.logTop.profile != logagg.ProfileNginx {
		t.Errorf("profile = %q, want nginx-combined", m.logTop.profile)
	}
	if len(m.logTop.parsed) != 3 || m.logTop.unmatched != 0 {
		t.Fatalf("parsed=%d unmatched=%d, want 3/0", len(m.logTop.parsed), m.logTop.unmatched)
	}
	// Default HTTP group-by is method+path: two POST /api/v4/jobs/request, one HEAD.
	var post *logagg.Row
	for i := range m.logTop.rows {
		if m.logTop.rows[i].Values[0] == "POST" && m.logTop.rows[i].Values[1] == "/api/v4/jobs/request" {
			post = &m.logTop.rows[i]
		}
	}
	if post == nil || post.Count != 2 {
		t.Fatalf("POST /api/v4/jobs/request row = %v, want Count 2", post)
	}
	if post.ErrCount != 0 {
		t.Errorf("POST row ErrCount = %d, want 0 (204s)", post.ErrCount)
	}
	// REQ/s must be derivable: timestamps were parsed (firstTS<lastTS).
	if m.logTopReqPerSec() <= 0 {
		t.Errorf("req/s = %v, want > 0 (timestamps should parse after prefix strip)", m.logTopReqPerSec())
	}
}

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
