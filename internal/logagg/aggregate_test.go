package logagg

import (
	"math"
	"strings"
	"testing"
)

func TestAggregation_GroupCountAndErrors(t *testing.T) {
	a := NewAggregation([]string{FieldMethod, FieldPath}, nil, IsHTTPError)
	a.Add(Fields{FieldMethod: "GET", FieldPath: "/api/users", FieldStatus: "200"})
	a.Add(Fields{FieldMethod: "GET", FieldPath: "/api/users", FieldStatus: "500"})
	a.Add(Fields{FieldMethod: "POST", FieldPath: "/api/login", FieldStatus: "401"})

	if a.Total() != 3 {
		t.Fatalf("Total = %d, want 3", a.Total())
	}
	rows := a.Rows(SortReq)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	// GET /api/users has 2 -> sorts first.
	if rows[0].Values[0] != "GET" || rows[0].Values[1] != "/api/users" {
		t.Errorf("row0 = %#v", rows[0].Values)
	}
	if rows[0].Count != 2 || rows[0].ErrCount != 1 {
		t.Errorf("row0 count=%d err=%d, want 2/1", rows[0].Count, rows[0].ErrCount)
	}
	if rows[1].ErrCount != 1 {
		t.Errorf("row1 err=%d, want 1 (401)", rows[1].ErrCount)
	}
}

func TestIsHTTPError(t *testing.T) {
	if !IsHTTPError(Fields{FieldStatus: "500"}) {
		t.Error("500 should be error")
	}
	if IsHTTPError(Fields{FieldStatus: "200"}) {
		t.Error("200 should not be error")
	}
	if IsHTTPError(Fields{}) {
		t.Error("missing status should not be error")
	}
}

// TestAggregation_DimsUniformAndVaried verifies that Dims shows the uniform
// value when all rows share it, and "*N" when they differ.
func TestAggregation_DimsUniformAndVaried(t *testing.T) {
	// Group by method; track status as a dim.
	a := NewAggregation(
		[]string{FieldMethod},
		[]string{FieldStatus, FieldHost},
		IsHTTPError,
	)
	// GET rows: all status=200 (uniform), two different hosts.
	a.Add(Fields{FieldMethod: "GET", FieldStatus: "200", FieldHost: "a.example.com"})
	a.Add(Fields{FieldMethod: "GET", FieldStatus: "200", FieldHost: "b.example.com"})
	// POST rows: two different statuses.
	a.Add(Fields{FieldMethod: "POST", FieldStatus: "200", FieldHost: "a.example.com"})
	a.Add(Fields{FieldMethod: "POST", FieldStatus: "500", FieldHost: "a.example.com"})

	rows := a.Rows(SortReq)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}

	rowsByMethod := map[string]Row{}
	for _, r := range rows {
		rowsByMethod[r.Values[0]] = r
	}

	get := rowsByMethod["GET"]
	// status uniform: shows value "200"
	if get.Dims[FieldStatus] != "200" {
		t.Errorf("GET status dim = %q, want \"200\"", get.Dims[FieldStatus])
	}
	// host varied (2 distinct): shows "*2"
	if get.Dims[FieldHost] != "*2" {
		t.Errorf("GET host dim = %q, want \"*2\"", get.Dims[FieldHost])
	}

	post := rowsByMethod["POST"]
	// status varied (2 distinct): shows "*2"
	if post.Dims[FieldStatus] != "*2" {
		t.Errorf("POST status dim = %q, want \"*2\"", post.Dims[FieldStatus])
	}
	// host uniform: shows value "a.example.com"
	if post.Dims[FieldHost] != "a.example.com" {
		t.Errorf("POST host dim = %q, want \"a.example.com\"", post.Dims[FieldHost])
	}
}

// TestAggregation_DimsSentinelFirstValue verifies that a dimension whose first
// observed value is the empty-field sentinel "-" is recorded as uniform "-",
// not silently skipped. This guards the explicit presence-check in Add.
func TestAggregation_DimsSentinelFirstValue(t *testing.T) {
	a := NewAggregation([]string{FieldMethod}, []string{FieldStatus}, nil)
	// First line: status field is absent -> normalised to "-".
	a.Add(Fields{FieldMethod: "GET"})
	// Second line: same group, same absent status.
	a.Add(Fields{FieldMethod: "GET"})

	rows := a.Rows(SortReq)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Dims[FieldStatus] != "-" {
		t.Errorf("status dim = %q, want \"-\"", rows[0].Dims[FieldStatus])
	}
}

// TestAggregation_DimsMaxDistinct verifies that tracking is capped at
// maxDimDistinct and the display string uses the "*50+" sentinel.
func TestAggregation_DimsMaxDistinct(t *testing.T) {
	a := NewAggregation([]string{FieldMethod}, []string{FieldPath}, nil)
	for i := range maxDimDistinct + 5 {
		path := "/path/" + strings.Repeat("x", i+1)
		a.Add(Fields{FieldMethod: "GET", FieldPath: path})
	}
	rows := a.Rows(SortReq)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	want := "*50+"
	if rows[0].Dims[FieldPath] != want {
		t.Errorf("path dim = %q, want %q", rows[0].Dims[FieldPath], want)
	}
}

// TestAggregation_Percentiles verifies that P95/P99 are computed from the
// latency histogram, and that groups without any duration field report -1.
func TestAggregation_Percentiles(t *testing.T) {
	a := NewAggregation([]string{FieldMethod, FieldPath}, nil, IsHTTPError)

	// Group A: 96 requests at <=13ms and 4 at 500ms (total 100).
	// Bucket distribution: all 96 fast samples land in <=8ms or <=13ms buckets.
	// p95 target = ceil(0.95*100) = 95 -> cumulative at <=13ms bucket = 96 >= 95,
	// so p95 = 13 (the upper bound of that bucket).
	// p99 target = 99 -> the 99th sample is the 3rd outlier (500ms, bucket <=610),
	// so p99 = 610.
	for range 96 {
		a.Add(Fields{
			FieldMethod:     "GET",
			FieldPath:       "/fast",
			FieldStatus:     "200",
			FieldDurationMS: "10", // 10ms -> bucket index 5, bound=13
		})
	}
	for range 4 {
		a.Add(Fields{
			FieldMethod:     "GET",
			FieldPath:       "/fast",
			FieldStatus:     "200",
			FieldDurationMS: "500",
		})
	}

	// Group B: no duration field at all.
	a.Add(Fields{FieldMethod: "POST", FieldPath: "/nodur", FieldStatus: "200"})

	rows := a.Rows(SortReq)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}

	rowsByPath := map[string]Row{}
	for _, r := range rows {
		rowsByPath[r.Values[1]] = r
	}

	fast := rowsByPath["/fast"]
	// p50 target = ceil(0.50*100) = 50 -> 50th sample is in the <=13ms bucket (96 samples fill it).
	if fast.P50 < 0 {
		t.Errorf("/fast P50 = %v, want >= 0", fast.P50)
	}
	if fast.P50 > 13 {
		t.Errorf("/fast P50 = %v, want <= 13 (50th sample is within the 96 fast requests)", fast.P50)
	}
	if fast.P95 < 0 {
		t.Errorf("/fast P95 = %v, want >= 0", fast.P95)
	}
	// p95: 95th of 100 samples, 96 at <=13ms -> cumulative crosses 95 in the <=13ms bucket
	if fast.P95 > 13 {
		t.Errorf("/fast P95 = %v, want <= 13 (96 samples at 10ms fill p95 bucket)", fast.P95)
	}
	if fast.P99 < 0 {
		t.Errorf("/fast P99 = %v, want >= 0", fast.P99)
	}
	if fast.P99 > 610 {
		t.Errorf("/fast P99 = %v, want <= 610", fast.P99)
	}
	if fast.P99 <= 13 {
		t.Errorf("/fast P99 = %v, want > 13 (4 outliers push p99 into higher bucket)", fast.P99)
	}

	nodur := rowsByPath["/nodur"]
	if nodur.P50 != -1 {
		t.Errorf("/nodur P50 = %v, want -1 (no duration field)", nodur.P50)
	}
	if nodur.P95 != -1 {
		t.Errorf("/nodur P95 = %v, want -1 (no duration field)", nodur.P95)
	}
	if nodur.P99 != -1 {
		t.Errorf("/nodur P99 = %v, want -1 (no duration field)", nodur.P99)
	}
}

// TestPercentileFromHist_ZeroTotal verifies the no-data sentinel.
func TestPercentileFromHist_ZeroTotal(t *testing.T) {
	hist := make([]int, len(durBucketsMs)+1)
	if got := percentileFromHist(hist, 0, 0.95); got != -1 {
		t.Errorf("got %v, want -1 for zero total", got)
	}
}

// TestAggregation_AvgMaxStatus tests Avg, Max, Status4xx, Status5xx fields.
func TestAggregation_AvgMaxStatus(t *testing.T) {
	a := NewAggregation([]string{FieldMethod, FieldPath}, nil, IsHTTPError)

	// Group A: durations {10, 10, 20, 500} -> avg=135, max=500
	for _, d := range []string{"10", "10", "20", "500"} {
		a.Add(Fields{FieldMethod: "GET", FieldPath: "/a", FieldStatus: "200", FieldDurationMS: d})
	}
	// Group B: statuses {200, 404, 404, 503} -> 4xx=2, 5xx=1
	for _, s := range []string{"200", "404", "404", "503"} {
		a.Add(Fields{FieldMethod: "POST", FieldPath: "/b", FieldStatus: s})
	}

	rows := a.Rows(SortReq)
	rowsByPath := map[string]Row{}
	for _, r := range rows {
		rowsByPath[r.Values[1]] = r
	}

	ga := rowsByPath["/a"]
	wantAvg := (10.0 + 10.0 + 20.0 + 500.0) / 4
	if ga.Avg < wantAvg-0.1 || ga.Avg > wantAvg+0.1 {
		t.Errorf("/a Avg = %v, want ~%.1f", ga.Avg, wantAvg)
	}
	if ga.Max != 500 {
		t.Errorf("/a Max = %v, want 500", ga.Max)
	}
	if ga.Status4xx != 0 {
		t.Errorf("/a Status4xx = %d, want 0", ga.Status4xx)
	}

	gb := rowsByPath["/b"]
	if gb.Status4xx != 2 {
		t.Errorf("/b Status4xx = %d, want 2 (two 404s)", gb.Status4xx)
	}
	if gb.Status5xx != 1 {
		t.Errorf("/b Status5xx = %d, want 1 (one 503)", gb.Status5xx)
	}
	if gb.Avg != -1 {
		t.Errorf("/b Avg = %v, want -1 (no duration data)", gb.Avg)
	}
	if gb.Max != -1 {
		t.Errorf("/b Max = %v, want -1 (no duration data)", gb.Max)
	}
}

// TestAggregation_RejectsNaNInfDuration ensures "NaN"/"Inf" duration strings
// (which strconv.ParseFloat accepts) do not poison Avg/Max metrics.
func TestAggregation_RejectsNaNInfDuration(t *testing.T) {
	a := NewAggregation([]string{FieldPath}, nil, IsHTTPError)
	for _, d := range []string{"10", "NaN", "Inf", "-Inf", "30"} {
		a.Add(Fields{FieldPath: "/a", FieldStatus: "200", FieldDurationMS: d})
	}
	r := a.Rows(SortReq)[0]
	// Only 10 and 30 should count: avg=20, max=30.
	if math.IsNaN(r.Avg) || math.IsInf(r.Avg, 0) {
		t.Fatalf("Avg poisoned by NaN/Inf: %v", r.Avg)
	}
	if r.Avg < 19.9 || r.Avg > 20.1 {
		t.Errorf("Avg = %v, want ~20 (NaN/Inf ignored)", r.Avg)
	}
	if r.Max != 30 {
		t.Errorf("Max = %v, want 30 (NaN/Inf ignored)", r.Max)
	}
}
