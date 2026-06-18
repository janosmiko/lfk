package logagg

import (
	"sort"
	"strconv"
	"strings"
)

// maxDimDistinct caps the number of distinct values tracked per dimension per
// group to bound memory usage.
const maxDimDistinct = 50

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
	Dims     map[string]string // per-dimension display string: uniform value, or "*N", or "*50+"
	id       string            // cached "\x00"-joined Values, for a stable allocation-free sort tiebreak
}

// groupRow is the internal per-group accumulator including dim tracking.
type groupRow struct {
	values   []string
	id       string
	count    int
	errCount int
	dimSets  map[string]map[string]struct{} // per-dim distinct values (capped at maxDimDistinct)
	dimFirst map[string]string              // first value seen per dim (for the uniform case)
}

// Aggregation accumulates grouped counts. Memory is bounded by the number of
// distinct group-value combinations, not by the number of lines.
type Aggregation struct {
	groupBy []string
	dims    []string
	isError func(Fields) bool
	rows    map[string]*groupRow
	total   int
}

// NewAggregation creates an aggregation grouped by the given field names.
// dims lists the extra dimension fields to track for display (may be nil).
func NewAggregation(groupBy []string, dims []string, isError func(Fields) bool) *Aggregation {
	return &Aggregation{
		groupBy: append([]string(nil), groupBy...),
		dims:    append([]string(nil), dims...),
		isError: isError,
		rows:    make(map[string]*groupRow),
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
		r = &groupRow{values: vals, id: id}
		a.rows[id] = r
	}
	r.count++
	if a.isError != nil && a.isError(f) {
		r.errCount++
	}
	a.total++

	// Track distinct values per display dimension.
	for _, d := range a.dims {
		v := f[d]
		if v == "" {
			v = "-"
		}
		if r.dimFirst == nil {
			r.dimFirst = map[string]string{}
			r.dimSets = map[string]map[string]struct{}{}
		}
		if r.dimFirst[d] == "" && len(r.dimSets[d]) == 0 {
			r.dimFirst[d] = v
		}
		set := r.dimSets[d]
		if set == nil {
			set = map[string]struct{}{}
			r.dimSets[d] = set
		}
		if _, ok := set[v]; !ok && len(set) < maxDimDistinct {
			set[v] = struct{}{}
		}
	}
}

// Total returns the number of lines added.
func (a *Aggregation) Total() int { return a.total }

// Rows returns the aggregated rows sorted descending by the given key, with a
// stable tiebreak on the joined group key.
func (a *Aggregation) Rows(key SortKey) []Row {
	out := make([]Row, 0, len(a.rows))
	for _, gr := range a.rows {
		dims := map[string]string{}
		for _, d := range a.dims {
			n := len(gr.dimSets[d])
			switch {
			case n <= 1:
				dims[d] = gr.dimFirst[d]
			case n >= maxDimDistinct:
				dims[d] = "*" + strconv.Itoa(maxDimDistinct) + "+"
			default:
				dims[d] = "*" + strconv.Itoa(n)
			}
		}
		out = append(out, Row{
			Values:   gr.values,
			Count:    gr.count,
			ErrCount: gr.errCount,
			Dims:     dims,
			id:       gr.id,
		})
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
