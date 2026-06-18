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

// logTopAddLine parses one raw line incrementally and rebuilds rows.
func (m *Model) logTopAddLine(line string) {
	m.logTopParseInto(line)
	m.logTopRebuildRows()
}

// logTopRebuildRows re-aggregates parsed through the current groupBy and drill
// constraints into rows.
func (m *Model) logTopRebuildRows() {
	agg := logagg.NewAggregation(m.logTop.groupBy, m.logTopErrorPredicate())
	for _, f := range m.logTop.parsed {
		if !m.logTopMatchesDrill(f) {
			continue
		}
		agg.Add(f)
	}
	m.logTop.rows = agg.Rows(m.logTop.sortKey)
	if m.logTop.cursor >= len(m.logTop.rows) {
		m.logTop.cursor = max(len(m.logTop.rows)-1, 0)
	}
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
