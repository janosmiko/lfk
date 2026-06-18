package app

import (
	"maps"
	"sort"

	"github.com/janosmiko/lfk/internal/logagg"
)

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
	scroll    int                // scroll offset: first visible row index
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

	// hasLatency is true when at least one parsed line has a duration_ms field.
	// Cached in logTopRebuildRows; drives the P95/P99 column visibility.
	hasLatency bool

	// drillStack is the stack of drill frames. Each enter pushes a frame;
	// each esc pops one and restores its groupBy; empty stack means no drill active.
	drillStack []logTopDrillFrame

	// colOrder is the user-ordered dimension column ids. Synced with
	// displayDims by logTopReconcileColOrder after each rebuild.
	colOrder []string
	// colHidden is the set of hidden column ids (dimension OR metric).
	colHidden map[string]bool
	// colSnapOrder/colSnapHidden snapshot colOrder/colHidden when the
	// column overlay opens so esc can cancel without persisting changes.
	colSnapOrder  []string
	colSnapHidden map[string]bool

	// pendingGroup holds the transient multi-select state for the group-by overlay.
	// It is not copied in copy() because it is ephemeral overlay state.
	pendingGroup map[string]bool

	// filterActive/filterInput/filterQuery drive the live row-text filter (f key).
	filterActive bool
	filterInput  TextInput
	filterQuery  string

	// searchActive/searchInput/searchQuery drive the / row-jumping search (no filter).
	searchActive bool
	searchInput  TextInput
	searchQuery  string
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
	c.colOrder = append([]string(nil), s.colOrder...)
	if s.colHidden != nil {
		c.colHidden = maps.Clone(s.colHidden)
	}
	c.agg = nil           // live aggregation is rebuilt lazily; never share the pointer across snapshots
	c.colSnapOrder = nil  // ephemeral: esc-cancel snapshot, valid only while the columns overlay is open
	c.colSnapHidden = nil // ephemeral: esc-cancel snapshot, valid only while the columns overlay is open
	return c
}

func (m *Model) saveLogTopToTab(t *TabState) {
	t.logTopProfile = string(m.logTop.profile)
	t.logTopGroupBy = append([]string(nil), m.logTop.groupBy...)
	t.logTopSortCol = m.logTop.sortCol
	t.logTopSortAsc = m.logTop.sortAsc
	t.logTopAutoProf = m.logTop.autoProf
	t.logTopFilterQuery = m.logTop.filterQuery
	t.logTopColOrder = append([]string(nil), m.logTop.colOrder...)
	hidden := make([]string, 0, len(m.logTop.colHidden))
	for k := range m.logTop.colHidden {
		hidden = append(hidden, k)
	}
	sort.Strings(hidden)
	t.logTopColHidden = hidden
}

func (m *Model) loadLogTopFromTab(t TabState) {
	m.logTop = logTopState{
		profile:     logagg.ProfileKind(t.logTopProfile),
		groupBy:     append([]string(nil), t.logTopGroupBy...),
		sortCol:     t.logTopSortCol,
		sortAsc:     t.logTopSortAsc,
		autoProf:    t.logTopAutoProf,
		filterQuery: t.logTopFilterQuery,
		colOrder:    append([]string(nil), t.logTopColOrder...),
	}
	if len(t.logTopColHidden) > 0 {
		m.logTop.colHidden = make(map[string]bool, len(t.logTopColHidden))
		for _, k := range t.logTopColHidden {
			m.logTop.colHidden[k] = true
		}
	}
	if m.mode == modeLogTop && len(m.logView.rawLines) > 0 {
		m.logTopReparseExisting()
	}
}
