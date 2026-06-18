package logagg

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

// durBucketsMs are ascending millisecond upper-bounds for the latency
// histogram. A value falls in the first bucket whose bound is >= it; values
// above the last bound fall in the overflow bucket (index len(durBucketsMs)).
var durBucketsMs = []float64{
	1, 2, 3, 5, 8, 13, 21, 34, 55, 89, 144, 233, 377, 610, 987,
	1597, 2584, 4181, 6765, 10946, 17711, 28657, 46368, 75025,
}

// durBucketIndex returns the histogram bucket for v ms.
func durBucketIndex(v float64) int {
	for i, b := range durBucketsMs {
		if v <= b {
			return i
		}
	}
	return len(durBucketsMs) // overflow
}

// percentileFromHist returns the approximate p-quantile (0<p<1) in ms from a
// histogram with `total` samples, or -1 when total == 0. The result is the
// upper bound of the bucket where the cumulative count crosses p (overflow
// bucket reports the largest bound).
func percentileFromHist(hist []int, total int, p float64) float64 {
	if total == 0 {
		return -1
	}
	target := int(math.Ceil(p * float64(total)))
	cum := 0
	for i, c := range hist {
		cum += c
		if cum >= target {
			if i >= len(durBucketsMs) {
				return durBucketsMs[len(durBucketsMs)-1]
			}
			return durBucketsMs[i]
		}
	}
	return durBucketsMs[len(durBucketsMs)-1]
}

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
	P95      float64           // approximate p95 latency in ms; -1 when no duration data
	P99      float64           // approximate p99 latency in ms; -1 when no duration data
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
	durHist  []int                          // latency histogram (lazily allocated, len = len(durBucketsMs)+1)
	durTotal int                            // number of lines with a valid duration
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

	// Accumulate latency histogram.
	if ds := f[FieldDurationMS]; ds != "" {
		if v, err := strconv.ParseFloat(ds, 64); err == nil {
			if r.durHist == nil {
				r.durHist = make([]int, len(durBucketsMs)+1)
			}
			r.durHist[durBucketIndex(v)]++
			r.durTotal++
		}
	}

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
		if _, ok := r.dimFirst[d]; !ok {
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
			P95:      percentileFromHist(gr.durHist, gr.durTotal, 0.95),
			P99:      percentileFromHist(gr.durHist, gr.durTotal, 0.99),
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
