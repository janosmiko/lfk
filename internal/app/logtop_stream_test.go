package app

import "testing"

func TestLogTop_StreamUpdatesAggregation(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogTop
	m.logTop = logTopState{}
	// Simulate two streamed lines via the same path the Update loop uses.
	for _, ln := range []string{
		`2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/a","DownstreamStatus":200}`,
		`2026-06-18T10:00:01Z {"RequestMethod":"GET","RequestPath":"/a","DownstreamStatus":500}`,
	} {
		m.ingestLogTopLine(ln)
	}
	if len(m.logTop.rows) != 1 || m.logTop.rows[0].Count != 2 {
		t.Fatalf("rows=%+v", m.logTop.rows)
	}
	if m.logTop.rows[0].ErrCount != 1 {
		t.Errorf("err=%d, want 1", m.logTop.rows[0].ErrCount)
	}
}
