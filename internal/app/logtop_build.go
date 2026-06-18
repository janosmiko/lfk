package app

import (
	"sort"
	"strings"

	"github.com/janosmiko/lfk/internal/logagg"
	"github.com/janosmiko/lfk/internal/ui"
)

// Metric column names used as sort keys.
const (
	logTopMetricREQ = "REQ"
	logTopMetricRPS = "REQ/s"
	logTopMetricPct = "%"
	logTopMetricERR = "ERR"
)

// httpProfile reports whether kind is an HTTP-access-log profile.
func httpProfile(kind logagg.ProfileKind) bool {
	return kind == logagg.ProfileTraefikJSON || kind == logagg.ProfileNginx ||
		kind == logagg.ProfileIngressNginx || kind == logagg.ProfileEnvoy
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

// logTopSample strips the kubectl pod prefix and timestamp from up to 200 head
// lines for profile detection.
func (m *Model) logTopSample() []string {
	n := min(len(m.logView.rawLines), 200)
	out := make([]string, 0, n)
	for _, line := range m.logView.rawLines[:n] {
		line = ui.StripPodPrefix(line)
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
	// Strip the "[pod/name/container]" prefix that kubectl --prefix prepends to
	// every line (all-containers/group-resource streams) so the timestamp and
	// the log body can be parsed.
	line = ui.StripPodPrefix(line)
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
	m.logTop.displayDims = m.computeDisplayDims()
	agg := logagg.NewAggregation(m.logTop.groupBy, m.logTop.displayDims, m.logTopErrorPredicate())
	for _, f := range m.logTop.parsed {
		if !m.logTopMatchesDrill(f) {
			continue
		}
		agg.Add(f)
	}
	m.logTop.agg = agg
	m.logTopRefreshRows()
}

// logTopRefreshRows re-snapshots rows from the live aggregation, applies the
// user-selected sort, and clamps the cursor. Cheap: O(groups log groups), no
// re-parse.
func (m *Model) logTopRefreshRows() {
	if m.logTop.agg == nil {
		m.logTopRebuildRows()
		return
	}
	m.logTop.rows = m.logTop.agg.Rows(logagg.SortReq)
	if m.logTop.sortCol == "" {
		m.logTop.sortCol = logTopMetricREQ
	}
	m.logTopSortRows()
	if m.logTop.cursor >= len(m.logTop.rows) {
		m.logTop.cursor = max(len(m.logTop.rows)-1, 0)
	}
}

// logTopSortColumns returns the ordered list of sortable column names:
// dimension columns first, then metric columns.
func (m *Model) logTopSortColumns() []string {
	return append(append([]string(nil), m.logTop.displayDims...), logTopMetricREQ, logTopMetricRPS, logTopMetricPct, logTopMetricERR)
}

// logTopSortRows sorts m.logTop.rows by the active sortCol / sortAsc.
func (m *Model) logTopSortRows() {
	col := m.logTop.sortCol
	rows := m.logTop.rows
	sort.SliceStable(rows, func(i, j int) bool {
		var c int
		switch col {
		case logTopMetricERR:
			c = rows[i].ErrCount - rows[j].ErrCount
		case logTopMetricREQ, logTopMetricRPS, logTopMetricPct:
			c = rows[i].Count - rows[j].Count
		default: // a dimension column: compare its display string
			c = strings.Compare(rows[i].Dims[col], rows[j].Dims[col])
		}
		if c == 0 {
			// stable tiebreak by joined group values
			c = strings.Compare(strings.Join(rows[i].Values, "\x00"), strings.Join(rows[j].Values, "\x00"))
		}
		if m.logTop.sortAsc {
			return c < 0
		}
		return c > 0
	})
}

// logTopCycleSort advances the active sort column by dir (+1 or -1), wrapping
// around the full column list, then refreshes rows.
func (m *Model) logTopCycleSort(dir int) {
	cols := m.logTopSortColumns()
	if len(cols) == 0 {
		return
	}
	idx := 0
	for i, c := range cols {
		if c == m.logTop.sortCol {
			idx = i
			break
		}
	}
	idx = (idx + dir + len(cols)) % len(cols)
	m.logTop.sortCol = cols[idx]
	m.logTopRefreshRows()
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
// constraints (every frame's filters must all pass).
func (m *Model) logTopMatchesDrill(f logagg.Fields) bool {
	for _, fr := range m.logTop.drillStack {
		for _, flt := range fr.filters {
			got := f[flt.field]
			if got == "" {
				got = "-"
			}
			if got != flt.value {
				return false
			}
		}
	}
	return true
}

// httpDimOrder is the fixed display order for HTTP dimension columns.
var httpDimOrder = []string{
	logagg.FieldMethod,
	logagg.FieldPath,
	logagg.FieldStatus,
	logagg.FieldHost,
	logagg.FieldRouter,
	logagg.FieldService,
}

// logTopDisplayDims returns the cached dimension columns for the current render.
// The cache is populated by logTopRebuildRows; dims only change when parsed
// changes, and every parsed mutation funnels through logTopRebuildRows.
func (m *Model) logTopDisplayDims() []string {
	return m.logTop.displayDims
}

// computeDisplayDims computes the dimension columns to show in the table by
// scanning parsed once. It returns the subset of httpDimOrder that appears in
// at least one parsed line. If no HTTP dimensions are present (generic logs),
// it falls back to the current groupBy fields so the table always has something
// to display.
func (m *Model) computeDisplayDims() []string {
	seen := map[string]bool{}
	for _, f := range m.logTop.parsed {
		for _, d := range httpDimOrder {
			if f[d] != "" {
				seen[d] = true
			}
		}
	}
	var out []string
	for _, d := range httpDimOrder {
		if seen[d] {
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		return append([]string(nil), m.logTop.groupBy...)
	}
	return out
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
