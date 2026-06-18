package app

import "github.com/janosmiko/lfk/internal/logagg"

// logTopState holds the Log Top aggregation viewer state. It reads from the
// same log stream as the log viewer; each new line is parsed once into parsed
// and folded into the live aggregation.
type logTopState struct {
	title     string             //nolint:unused // wired in later Log Top tasks
	profile   logagg.ProfileKind //nolint:unused // wired in later Log Top tasks
	autoProf  bool               //nolint:unused // wired in later Log Top tasks
	groupBy   []string           // selected group-by field names
	sortKey   logagg.SortKey     //nolint:unused // wired in later Log Top tasks
	cursor    int                //nolint:unused // wired in later Log Top tasks
	scroll    int                //nolint:unused // wired in later Log Top tasks
	parsed    []logagg.Fields    // cache of parsed (matched) lines, for re-aggregation
	unmatched int                //nolint:unused // wired in later Log Top tasks
	rows      []logagg.Row       // last computed rows (rebuilt on change)
	firstTS   int64              //nolint:unused // wired in later Log Top tasks
	lastTS    int64              //nolint:unused // wired in later Log Top tasks

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
