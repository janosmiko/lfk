package app

import (
	"github.com/janosmiko/lfk/internal/logagg"
	"github.com/janosmiko/lfk/internal/ui"
)

// httpProfile reports whether kind is an HTTP-access-log profile.
func httpProfile(kind logagg.ProfileKind) bool {
	return kind == logagg.ProfileTraefikJSON || kind == logagg.ProfileNginx
}

// defaultGroupBy returns sensible group-by fields for the detected profile.
func defaultGroupBy(kind logagg.ProfileKind, sample []logagg.Fields) []string {
	if httpProfile(kind) {
		return []string{logagg.FieldMethod, logagg.FieldPath}
	}
	for _, key := range []string{logagg.FieldPath, logagg.FieldHost, logagg.FieldMethod} {
		for _, f := range sample {
			if f[key] != "" {
				return []string{key}
			}
		}
	}
	return []string{logagg.FieldLevel}
}

// logTopErrorPredicate returns a predicate that matches error-level lines.
// HTTP profiles use the HTTP status code; all others use the log level field.
func (m *Model) logTopErrorPredicate() func(logagg.Fields) bool {
	if httpProfile(m.logTop.profile) {
		return logagg.IsHTTPError
	}
	return func(f logagg.Fields) bool {
		return ui.LineLogLevel(f[logagg.FieldLevel]) >= ui.LogError
	}
}

// parserForState returns the parser for the current profile.
func (m *Model) parserForState() logagg.Parser {
	return logagg.ParserFor(m.logTop.profile)
}

// logTopResetAndParse detects the profile (honoring ConfigLogTopDefaultProfile),
// resets state, and re-parses all rawLines.
func (m *Model) logTopResetAndParse() {
	kind := logagg.ProfileKind(ui.ConfigLogTopDefaultProfile)
	if ui.ConfigLogTopDefaultProfile == "auto" || ui.ConfigLogTopDefaultProfile == "" {
		sample := m.logTopSample()
		kind = logagg.DetectKind(sample)
		m.logTop.autoProf = true
	} else {
		m.logTop.autoProf = false
	}
	m.logTop.profile = kind
	m.logTop.parsed = m.logTop.parsed[:0]
	m.logTop.unmatched = 0
	m.logTop.firstTS = 0
	m.logTop.lastTS = 0
	for _, line := range m.logView.rawLines {
		m.logTopParseInto(line)
	}
	if len(m.logTop.groupBy) == 0 {
		m.logTop.groupBy = defaultGroupBy(kind, m.logTop.parsed)
	}
	m.logTopRebuildRows()
}

// logTopSample strips the kubectl timestamp from up to 200 head lines for
// profile detection.
func (m *Model) logTopSample() []string {
	n := min(len(m.logView.rawLines), 200)
	out := make([]string, 0, n)
	for _, line := range m.logView.rawLines[:n] {
		if _, rest, ok := logagg.SplitTimestamp(line); ok {
			out = append(out, rest)
		} else {
			out = append(out, line)
		}
	}
	return out
}

// logTopParseInto parses one raw line into parsed/unmatched and updates TS
// bounds. It does not rebuild rows.
func (m *Model) logTopParseInto(line string) {
	body := line
	if ts, rest, ok := logagg.SplitTimestamp(line); ok {
		body = rest
		ns := ts.UnixNano()
		if m.logTop.firstTS == 0 || ns < m.logTop.firstTS {
			m.logTop.firstTS = ns
		}
		if ns > m.logTop.lastTS {
			m.logTop.lastTS = ns
		}
	}
	if f, ok := m.parserForState().Parse(body); ok {
		m.logTop.parsed = append(m.logTop.parsed, f)
	} else {
		m.logTop.unmatched++
	}
}

// ingestLogTopLine appends a streamed raw line to rawLines and updates the
// aggregation. On the first line it detects the profile and seeds groupBy.
func (m *Model) ingestLogTopLine(line string) {
	m.logView.rawLines = append(m.logView.rawLines, line)
	if m.logTop.profile == "" {
		m.logTopResetAndParse() // first line: detect + seed groupBy + build
		return
	}
	m.logTopAddLine(line)
}

// logTopRebuildRows rebuilds the live aggregation from parsed (applying drill
// constraints) and refreshes the displayed rows. Used on reset and on any
// change that invalidates incremental accumulation (group-by, drill, profile,
// or a parsed trim).
func (m *Model) logTopRebuildRows() {
	agg := logagg.NewAggregation(m.logTop.groupBy, m.logTopErrorPredicate())
	for _, f := range m.logTop.parsed {
		if !m.logTopMatchesDrill(f) {
			continue
		}
		agg.Add(f)
	}
	m.logTop.agg = agg
	m.logTopRefreshRows()
}

// logTopRefreshRows re-snapshots rows from the live aggregation and clamps the
// cursor. Cheap: O(groups log groups), no re-parse.
func (m *Model) logTopRefreshRows() {
	if m.logTop.agg == nil {
		m.logTopRebuildRows()
		return
	}
	m.logTop.rows = m.logTop.agg.Rows(m.logTop.sortKey)
	if m.logTop.cursor >= len(m.logTop.rows) {
		m.logTop.cursor = max(len(m.logTop.rows)-1, 0)
	}
}

// logTopAddLine parses one raw line and folds it into the live aggregation in
// O(1) amortized time, then bounds the parsed cache.
func (m *Model) logTopAddLine(line string) {
	before := len(m.logTop.parsed)
	m.logTopParseInto(line)
	if m.logTop.agg == nil {
		m.logTopRebuildRows()
		return
	}
	if len(m.logTop.parsed) > before { // a matched line was appended
		f := m.logTop.parsed[len(m.logTop.parsed)-1]
		if m.logTopMatchesDrill(f) {
			m.logTop.agg.Add(f)
		}
	}
	if m.logTopCapParsed() {
		// Oldest lines were dropped; the live aggregation is now stale.
		m.logTopRebuildRows()
		return
	}
	m.logTopRefreshRows()
}

// logTopCapParsed bounds m.logTop.parsed the same way capLogLines bounds the
// raw buffer: it trims to ui.ConfigLogMaxLines once length exceeds
// ConfigLogMaxLines+logBufferTrimSlack, so trimming (and the rebuild it forces)
// is amortized across logBufferTrimSlack lines. Returns true when it trimmed.
// Note: firstTS is not adjusted when old lines are dropped (parsed Fields do
// not retain timestamps), so REQ/s can slightly under-report after the buffer
// wraps. This is an accepted MVP limitation.
func (m *Model) logTopCapParsed() bool {
	maxLines := ui.ConfigLogMaxLines
	if maxLines <= 0 || len(m.logTop.parsed) <= maxLines+logBufferTrimSlack {
		return false
	}
	drop := len(m.logTop.parsed) - maxLines
	trimmed := make([]logagg.Fields, maxLines)
	copy(trimmed, m.logTop.parsed[drop:])
	m.logTop.parsed = trimmed
	return true
}

// logTopMatchesDrill reports whether a parsed line matches all active drill
// constraints.
func (m *Model) logTopMatchesDrill(f logagg.Fields) bool {
	for i, field := range m.logTop.drillField {
		want := m.logTop.drillValue[i]
		got := f[field]
		if got == "" {
			got = "-"
		}
		if got != want {
			return false
		}
	}
	return true
}

// logTopReqPerSec returns the throughput in requests per second over the
// observed time span. Returns 0 if the span is unknown.
func (m *Model) logTopReqPerSec() float64 {
	spanNS := m.logTop.lastTS - m.logTop.firstTS
	if spanNS <= 0 {
		return 0
	}
	return float64(len(m.logTop.parsed)) / (float64(spanNS) / 1e9)
}
