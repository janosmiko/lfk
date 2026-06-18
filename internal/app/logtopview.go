package app

import "github.com/janosmiko/lfk/internal/logagg"

// logTopDrillFilter is a single field=value constraint applied during drill-down.
type logTopDrillFilter struct {
	field string
	value string
}

// logTopDrillFrame captures the groupBy active before a drill and the filters
// the drill added. Popping a frame restores groupBy and drops its filters.
type logTopDrillFrame struct {
	groupBy []string            // groupBy active before this drill (restored on pop)
	filters []logTopDrillFilter // constraints this drill added
}

// logTopState holds the Log Top aggregation viewer state. It reads from the
// same log stream as the log viewer; each new line is parsed once into parsed
// and folded into the live aggregation.
type logTopState struct {
	title     string             //nolint:unused // wired in later Log Top tasks
	profile   logagg.ProfileKind // detected or configured parser profile
	autoProf  bool               // true when profile was auto-detected
	groupBy   []string           // selected group-by field names
	sortCol   string             // active sort column name (e.g. "REQ", "ERR", or a dim name)
	sortAsc   bool               // true = ascending; false = descending (default)
	cursor    int                // selected row index
	scroll    int                //nolint:unused // wired in later Log Top tasks
	parsed    []logagg.Fields    // cache of parsed (matched) lines, for re-aggregation
	unmatched int                // count of lines that did not parse
	rows      []logagg.Row       // last computed rows (rebuilt on change)
	firstTS   int64              // nanosecond timestamp of first parsed line
	lastTS    int64              // nanosecond timestamp of last parsed line

	agg *logagg.Aggregation // live aggregation; folded incrementally, rebuilt on group/drill/profile change or parsed trim

	// displayDims is the cached result of computeDisplayDims(). It is set by
	// logTopRebuildRows before building the aggregation; dims only change when
	// parsed changes, and every parsed mutation funnels through logTopRebuildRows.
	displayDims []string

	// drillStack is the stack of drill frames. Each enter pushes a frame;
	// each esc pops one and restores its groupBy; empty stack means no drill active.
	drillStack []logTopDrillFrame

	// pendingGroup holds the transient multi-select state for the group-by overlay.
	// It is not copied in copy() because it is ephemeral overlay state.
	pendingGroup map[string]bool
}

func (s logTopState) copy() logTopState {
	c := s
	c.groupBy = append([]string(nil), s.groupBy...)
	c.parsed = append([]logagg.Fields(nil), s.parsed...)
	c.rows = append([]logagg.Row(nil), s.rows...)
	if s.drillStack != nil {
		c.drillStack = make([]logTopDrillFrame, len(s.drillStack))
		for i, fr := range s.drillStack {
			c.drillStack[i] = logTopDrillFrame{
				groupBy: append([]string(nil), fr.groupBy...),
				filters: append([]logTopDrillFilter(nil), fr.filters...),
			}
		}
	}
	c.displayDims = append([]string(nil), s.displayDims...)
	c.agg = nil // live aggregation is rebuilt lazily; never share the pointer across snapshots
	return c
}

func (m *Model) saveLogTopToTab(t *TabState) {
	t.logTopProfile = string(m.logTop.profile)
	t.logTopGroupBy = append([]string(nil), m.logTop.groupBy...)
	t.logTopSortCol = m.logTop.sortCol
	t.logTopSortAsc = m.logTop.sortAsc
	t.logTopAutoProf = m.logTop.autoProf
}

func (m *Model) loadLogTopFromTab(t TabState) {
	m.logTop = logTopState{
		profile:  logagg.ProfileKind(t.logTopProfile),
		groupBy:  append([]string(nil), t.logTopGroupBy...),
		sortCol:  t.logTopSortCol,
		sortAsc:  t.logTopSortAsc,
		autoProf: t.logTopAutoProf,
	}
	if m.mode == modeLogTop && len(m.logView.rawLines) > 0 {
		m.logTopReparseExisting()
	}
}
