package logagg

import (
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
