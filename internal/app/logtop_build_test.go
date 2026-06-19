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

// TestLogTop_RedetectsWhenAccessLogsArriveLater guards the real-world case: a
// Traefik deployment emits operational/console lines (which look like logfmt)
// before/among its JSON access logs. Detection must re-evaluate as lines stream
// in so it does not lock the wrong profile based on an early non-access line.
func TestLogTop_RedetectsWhenAccessLogsArriveLater(t *testing.T) {
	orig := ui.ConfigLogTopDefaultProfile
	ui.ConfigLogTopDefaultProfile = "auto"
	defer func() { ui.ConfigLogTopDefaultProfile = orig }()

	m := &Model{}
	m.mode = modeLogTop
	// First line is an operational/error line that the logfmt matcher catches.
	m.ingestLogTopLine(`2026-06-19T03:41:30Z level=error msg="tcp" error="conn reset"`)
	if m.logTop.profile != logagg.ProfileLogfmt {
		t.Fatalf("first line: profile = %q, want logfmt", m.logTop.profile)
	}
	// Then JSON access logs stream in.
	for range 5 {
		m.ingestLogTopLine(`2026-06-19T03:41:31Z {"DownstreamStatus":200,"RequestMethod":"GET","RequestPath":"/api/x"}`)
	}
	if m.logTop.profile != logagg.ProfileTraefikJSON {
		t.Fatalf("after access logs streamed: profile = %q, want traefik-json (must re-detect)", m.logTop.profile)
	}
	// Grouping re-seeded to method+path for the new format, and rows appear.
	if len(m.logTop.groupBy) != 2 || m.logTop.groupBy[0] != logagg.FieldMethod || m.logTop.groupBy[1] != logagg.FieldPath {
		t.Errorf("groupBy = %v, want [method path]", m.logTop.groupBy)
	}
	if len(m.logTop.rows) == 0 {
		t.Fatal("expected request rows after re-detecting traefik-json")
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

// TestLogTopDefaultHiddenMetrics verifies the curated default-hidden seeding.
func TestLogTopDefaultHiddenMetrics(t *testing.T) {
	orig := ui.ConfigLogTopDefaultProfile
	ui.ConfigLogTopDefaultProfile = "auto"
	defer func() { ui.ConfigLogTopDefaultProfile = orig }()

	m := &Model{}
	// Traefik JSON lines with latency so hasLatency=true and httpProfile=true.
	m.logView.rawLines = []string{
		`2026-06-18T10:00:00Z {"DownstreamStatus":200,"RequestMethod":"GET","RequestPath":"/a","Duration":10000000}`,
		`2026-06-18T10:00:01Z {"DownstreamStatus":200,"RequestMethod":"GET","RequestPath":"/a","Duration":20000000}`,
	}
	m.logTopResetAndParse()

	// colInit must be set after first build.
	if !m.logTop.colInit {
		t.Error("colInit should be true after first logTopRebuildRows")
	}
	// Default hidden: %, ERR, 4XX, 5XX, AVG, MAX.
	hidden := m.logTop.colHidden
	for _, id := range []string{logTopMetricPct, logTopMetricERR, logTopMetric4xx, logTopMetric5xx, logTopMetricAvg, logTopMetricMax} {
		if !hidden[id] {
			t.Errorf("metric %q should be hidden by default", id)
		}
	}
	// Default visible: REQ, REQ/s, ERR%.
	for _, id := range []string{logTopMetricREQ, logTopMetricRPS, logTopMetricErrRate} {
		if hidden[id] {
			t.Errorf("metric %q should be visible by default", id)
		}
	}
	// Second rebuild must NOT re-seed (colInit guard).
	m.logTop.colHidden[logTopMetricREQ] = true // simulate user hiding REQ
	m.logTopRebuildRows()
	if !m.logTop.colHidden[logTopMetricREQ] {
		t.Error("colInit guard failed: second rebuild must not re-seed colHidden")
	}
}

// TestCmpLatency_NALast verifies that n/a rows sort last in both directions.
func TestCmpLatency_NALast(t *testing.T) {
	orig := ui.ConfigLogTopDefaultProfile
	ui.ConfigLogTopDefaultProfile = "auto"
	defer func() { ui.ConfigLogTopDefaultProfile = orig }()

	m := &Model{}
	// /a has 13ms duration, /b has no duration, /c has 100ms.
	m.logView.rawLines = []string{
		`2026-06-18T10:00:00Z {"DownstreamStatus":200,"RequestMethod":"GET","RequestPath":"/a","Duration":13000000}`,
		`2026-06-18T10:00:01Z {"DownstreamStatus":200,"RequestMethod":"POST","RequestPath":"/b"}`,
		`2026-06-18T10:00:02Z {"DownstreamStatus":200,"RequestMethod":"GET","RequestPath":"/c","Duration":100000000}`,
	}
	m.logTopResetAndParse()
	m.logTop.colInit = true // skip re-seeding to avoid affecting sort

	// Sort ascending by P95 - no-latency row must be last.
	m.logTop.sortCol = logTopMetricP95
	m.logTop.sortAsc = true
	m.logTopRefreshRows()

	lastIdx := len(m.logTop.rows) - 1
	foundNA := false
	for i := range m.logTop.rows {
		if m.logTop.rows[i].P95 < 0 {
			foundNA = true
			if i != lastIdx {
				t.Errorf("ascending P95: no-latency row is at index %d, want last (%d)", i, lastIdx)
			}
		}
	}
	if !foundNA {
		t.Error("no row with P95=-1 found")
	}

	// Sort descending by P95 - no-latency row must still be last.
	m.logTop.sortAsc = false
	m.logTopRefreshRows()

	lastIdx = len(m.logTop.rows) - 1
	for i := range m.logTop.rows {
		if m.logTop.rows[i].P95 < 0 {
			if i != lastIdx {
				t.Errorf("descending P95: no-latency row is at index %d, want last (%d)", i, lastIdx)
			}
		}
	}
}

// TestLogTopMetricErrrateSort verifies ERR% sort uses rate not raw count.
func TestLogTopMetricErrrateSort(t *testing.T) {
	orig := ui.ConfigLogTopDefaultProfile
	ui.ConfigLogTopDefaultProfile = "auto"
	defer func() { ui.ConfigLogTopDefaultProfile = orig }()

	m := &Model{}
	// /a: 10 requests, 9 errors -> 90% err rate
	m.logView.rawLines = []string{
		`2026-06-18T10:00:00Z {"DownstreamStatus":200,"RequestMethod":"GET","RequestPath":"/a"}`,
		`2026-06-18T10:00:01Z {"DownstreamStatus":500,"RequestMethod":"GET","RequestPath":"/a"}`,
		`2026-06-18T10:00:02Z {"DownstreamStatus":500,"RequestMethod":"GET","RequestPath":"/a"}`,
		`2026-06-18T10:00:03Z {"DownstreamStatus":500,"RequestMethod":"GET","RequestPath":"/a"}`,
		`2026-06-18T10:00:04Z {"DownstreamStatus":500,"RequestMethod":"GET","RequestPath":"/a"}`,
		`2026-06-18T10:00:05Z {"DownstreamStatus":500,"RequestMethod":"GET","RequestPath":"/a"}`,
		`2026-06-18T10:00:06Z {"DownstreamStatus":500,"RequestMethod":"GET","RequestPath":"/a"}`,
		`2026-06-18T10:00:07Z {"DownstreamStatus":500,"RequestMethod":"GET","RequestPath":"/a"}`,
		`2026-06-18T10:00:08Z {"DownstreamStatus":500,"RequestMethod":"GET","RequestPath":"/a"}`,
		`2026-06-18T10:00:09Z {"DownstreamStatus":500,"RequestMethod":"GET","RequestPath":"/a"}`,
	}
	// /b: 100 requests, 50 errors -> 50% err rate
	for range 50 {
		m.logView.rawLines = append(m.logView.rawLines,
			`2026-06-18T10:00:10Z {"DownstreamStatus":500,"RequestMethod":"GET","RequestPath":"/b"}`,
		)
	}
	for range 50 {
		m.logView.rawLines = append(m.logView.rawLines,
			`2026-06-18T10:00:10Z {"DownstreamStatus":200,"RequestMethod":"GET","RequestPath":"/b"}`,
		)
	}
	m.logTopResetAndParse()
	m.logTop.colInit = true

	m.logTop.sortCol = logTopMetricErrRate
	m.logTop.sortAsc = false // descending: highest error rate first
	m.logTopRefreshRows()

	if len(m.logTop.rows) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(m.logTop.rows))
	}
	// /a (90%) should be first (higher rate), /b (50%) second.
	if m.logTop.rows[0].Values[1] != "/a" {
		t.Errorf("first row path = %v, want /a (90%% err rate)", m.logTop.rows[0].Values)
	}
}
