package logagg

import (
	"sort"
	"strconv"
	"strings"
)

// SortKey selects the column to sort aggregation rows by.
type SortKey int

const (
	SortReq SortKey = iota
	SortErr
)

// Row is one aggregated group.
type Row struct {
	Values   []string // group-key values, in groupBy order
	Count    int
	ErrCount int
	id       string // cached "\x00"-joined Values, for a stable allocation-free sort tiebreak
}

// Aggregation accumulates grouped counts. Memory is bounded by the number of
// distinct group-value combinations, not by the number of lines.
type Aggregation struct {
	groupBy []string
	isError func(Fields) bool
	rows    map[string]*Row
	total   int
}

// NewAggregation creates an aggregation grouped by the given field names.
func NewAggregation(groupBy []string, isError func(Fields) bool) *Aggregation {
	return &Aggregation{
		groupBy: append([]string(nil), groupBy...),
		isError: isError,
		rows:    make(map[string]*Row),
	}
}

// Add folds one parsed line into the aggregation.
func (a *Aggregation) Add(f Fields) {
	vals := make([]string, len(a.groupBy))
	for i, key := range a.groupBy {
		v := f[key]
		if v == "" {
			v = "-"
		}
		vals[i] = v
	}
	id := strings.Join(vals, "\x00")
	r := a.rows[id]
	if r == nil {
		r = &Row{Values: vals, id: id}
		a.rows[id] = r
	}
	r.Count++
	if a.isError != nil && a.isError(f) {
		r.ErrCount++
	}
	a.total++
}

// Total returns the number of lines added.
func (a *Aggregation) Total() int { return a.total }

// Rows returns the aggregated rows sorted descending by the given key, with a
// stable tiebreak on the joined group key.
func (a *Aggregation) Rows(key SortKey) []Row {
	out := make([]Row, 0, len(a.rows))
	for _, r := range a.rows {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		vi, vj := metric(out[i], key), metric(out[j], key)
		if vi != vj {
			return vi > vj
		}
		return out[i].id < out[j].id
	})
	return out
}

func metric(r Row, key SortKey) int {
	if key == SortErr {
		return r.ErrCount
	}
	return r.Count
}

// IsHTTPError reports whether the line has an HTTP status >= 400.
func IsHTTPError(f Fields) bool {
	code, err := strconv.Atoi(f[FieldStatus])
	return err == nil && code >= 400
}
