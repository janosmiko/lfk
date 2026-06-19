package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/logagg"
)

func TestLogTopFilterMatch_EmptyTerms(t *testing.T) {
	f := logagg.Fields{"status": "200", "path": "/api"}
	if !logTopFilterMatch(f, nil) {
		t.Error("empty terms should match any line")
	}
	if !logTopFilterMatch(f, []logTopFilterTerm{}) {
		t.Error("empty terms slice should match any line")
	}
}

func TestLogTopFilterMatch_EqualOp(t *testing.T) {
	f := logagg.Fields{"status": "204", "host": "demo.example.com"}
	terms := parseLogTopFilter("status=204")
	if !logTopFilterMatch(f, terms) {
		t.Error("status=204 should match {status:204}")
	}
	terms2 := parseLogTopFilter("status=200")
	if logTopFilterMatch(f, terms2) {
		t.Error("status=200 should not match {status:204}")
	}
}

func TestLogTopFilterMatch_NotEqualOp(t *testing.T) {
	f := logagg.Fields{"status": "204"}
	terms := parseLogTopFilter("status!=200")
	if !logTopFilterMatch(f, terms) {
		t.Error("status!=200 should match {status:204}")
	}
	terms2 := parseLogTopFilter("status!=204")
	if logTopFilterMatch(f, terms2) {
		t.Error("status!=204 should not match {status:204}")
	}
}

func TestLogTopFilterMatch_ContainsOp(t *testing.T) {
	f := logagg.Fields{"host": "demo.example.com"}
	terms := parseLogTopFilter("host~example")
	if !logTopFilterMatch(f, terms) {
		t.Error("host~example should match {host:demo.example.com}")
	}
	terms2 := parseLogTopFilter("host~other")
	if logTopFilterMatch(f, terms2) {
		t.Error("host~other should not match {host:demo.example.com}")
	}
}

func TestLogTopFilterMatch_NumericOps(t *testing.T) {
	f := logagg.Fields{"status": "503"}
	if !logTopFilterMatch(f, parseLogTopFilter("status>=500")) {
		t.Error("status>=500 should match 503")
	}
	if logTopFilterMatch(f, parseLogTopFilter("status>=600")) {
		t.Error("status>=600 should not match 503")
	}
	if !logTopFilterMatch(f, parseLogTopFilter("status>500")) {
		t.Error("status>500 should match 503")
	}
	if !logTopFilterMatch(f, parseLogTopFilter("status<=503")) {
		t.Error("status<=503 should match 503")
	}
	if logTopFilterMatch(f, parseLogTopFilter("status<503")) {
		t.Error("status<503 should not match 503")
	}

	f2 := logagg.Fields{"status": "204"}
	if logTopFilterMatch(f2, parseLogTopFilter("status>=500")) {
		t.Error("status>=500 should not match 204")
	}
}

func TestLogTopFilterMatch_MultiTermAND(t *testing.T) {
	f := logagg.Fields{"status": "204", "host": "demo.example.com"}
	// Both must match.
	if !logTopFilterMatch(f, parseLogTopFilter("status=204 host~example")) {
		t.Error("status=204 host~example should match")
	}
	// Second term fails.
	if logTopFilterMatch(f, parseLogTopFilter("status=204 host~other")) {
		t.Error("status=204 host~other should not match")
	}
}

func TestLogTopFilterMatch_BareText(t *testing.T) {
	f := logagg.Fields{"path": "/api/users", "status": "200"}
	if !logTopFilterMatch(f, parseLogTopFilter("api")) {
		t.Error("bare 'api' should match path=/api/users")
	}
	if logTopFilterMatch(f, parseLogTopFilter("xyz")) {
		t.Error("bare 'xyz' should not match")
	}
}

// TestLogTopFilter_FieldExpression verifies that a field-expression filter applied
// via handleLogTopFilterKey filters lines BEFORE aggregation.
func TestLogTopFilter_FieldExpression(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogTop
	m.logView.rawLines = []string{
		`2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/only204","DownstreamStatus":204}`,
		`2026-06-18T10:00:01Z {"RequestMethod":"GET","RequestPath":"/only500","DownstreamStatus":500}`,
		`2026-06-18T10:00:02Z {"RequestMethod":"GET","RequestPath":"/only204","DownstreamStatus":204}`,
	}
	m.logTopResetAndParse()

	// Verify unfiltered state: both paths present.
	if len(m.logTop.rows) < 2 {
		t.Fatalf("unfiltered: expected >=2 rows, got %d", len(m.logTop.rows))
	}

	// Open filter and type "status=204".
	m.logTop.filterActive = true
	for _, ch := range "status=204" {
		mdl, _ := m.handleLogTopFilterKey(key(string(ch)))
		m = mdl.(Model)
	}

	// The aggregation should only contain 204 lines.
	if m.logTop.agg == nil {
		t.Fatal("agg is nil after filter")
	}
	if m.logTop.agg.Total() != 2 {
		t.Errorf("agg.Total() = %d, want 2 (only 204 lines)", m.logTop.agg.Total())
	}

	// The 500-only path must not appear in rows.
	for _, r := range m.logTop.rows {
		if r.Dims[logagg.FieldPath] == "/only500" {
			t.Error("/only500 row should not appear when filter is status=204")
		}
	}

	// The 204-only path must appear.
	found204 := false
	for _, r := range m.logTop.rows {
		if r.Dims[logagg.FieldPath] == "/only204" {
			found204 = true
		}
	}
	if !found204 {
		t.Error("/only204 row should appear when filter is status=204")
	}
}
