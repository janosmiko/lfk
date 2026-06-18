package logagg

import "testing"

func TestSplitTimestamp(t *testing.T) {
	ts, rest, ok := SplitTimestamp(`2026-06-18T10:00:01.123456789Z {"method":"GET"}`)
	if !ok {
		t.Fatal("expected timestamp to split")
	}
	if rest != `{"method":"GET"}` {
		t.Errorf("rest = %q", rest)
	}
	if ts.Year() != 2026 {
		t.Errorf("year = %d", ts.Year())
	}

	if _, _, ok := SplitTimestamp(`no timestamp here`); ok {
		t.Error("expected no split for plain line")
	}
}
