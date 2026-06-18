package logagg

import "testing"

func TestAggregation_GroupCountAndErrors(t *testing.T) {
	a := NewAggregation([]string{FieldMethod, FieldPath}, IsHTTPError)
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
