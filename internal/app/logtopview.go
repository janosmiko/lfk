package app

import "github.com/janosmiko/lfk/internal/logagg"

// logTopState holds the Log Top aggregation viewer state. It reads from the
// same log stream as the log viewer; each new line is parsed once into parsed
// and folded into the live aggregation.
type logTopState struct {
	title     string             //nolint:unused // wired in later Log Top tasks
	profile   logagg.ProfileKind // detected or configured parser profile
	autoProf  bool               // true when profile was auto-detected
	groupBy   []string           // selected group-by field names
	sortKey   logagg.SortKey     // active sort column
	cursor    int                // selected row index
	scroll    int                //nolint:unused // wired in later Log Top tasks
	parsed    []logagg.Fields    // cache of parsed (matched) lines, for re-aggregation
	unmatched int                // count of lines that did not parse
	rows      []logagg.Row       // last computed rows (rebuilt on change)
	firstTS   int64              // nanosecond timestamp of first parsed line
	lastTS    int64              // nanosecond timestamp of last parsed line

	// Drill-down stack: each entry pins a (field,value) constraint.
	drillField []string
	drillValue []string
}

func (s logTopState) copy() logTopState {
	c := s
	c.groupBy = append([]string(nil), s.groupBy...)
	c.parsed = append([]logagg.Fields(nil), s.parsed...)
	c.rows = append([]logagg.Row(nil), s.rows...)
	c.drillField = append([]string(nil), s.drillField...)
	c.drillValue = append([]string(nil), s.drillValue...)
	return c
}
