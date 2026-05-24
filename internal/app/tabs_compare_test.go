package app

import (
	"sort"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
)

func TestComparePortsCmp_Numeric(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"99 before 10000", "99/TCP", "10000/TCP", -1},
		{"10000 after 99", "10000/TCP", "99/TCP", 1},
		{"equal leading port", "80/TCP", "80/UDP", -1},
		{"identical", "443/TCP", "443/TCP", 0},
		{"nodeport form", "8080:30000/TCP", "9000:30001/TCP", -1},
		{"non-numeric falls back lexicographic", "abc", "abd", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := comparePortsCmp(tt.a, tt.b)
			if (got < 0) != (tt.want < 0) || (got > 0) != (tt.want > 0) {
				t.Errorf("comparePortsCmp(%q, %q) = %d, want sign of %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestComparePortsCmp_SortOrder(t *testing.T) {
	values := []string{"10000/TCP", "99/TCP", "443/TCP", "8080/TCP"}
	sort.Slice(values, func(i, j int) bool {
		return comparePortsCmp(values[i], values[j]) < 0
	})
	want := []string{"99/TCP", "443/TCP", "8080/TCP", "10000/TCP"}
	for i := range want {
		if values[i] != want[i] {
			t.Fatalf("sorted = %v, want %v", values, want)
		}
	}
}

func TestCompareDurationCmp(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"5m before 10m", "5m0s", "10m0s", -1},
		{"10m after 5m", "10m0s", "5m0s", 1},
		{"equal", "1h2m3s", "1h2m3s", 0},
		{"seconds vs minutes", "90s", "1m0s", 1},
		{"non-duration falls back lexicographic", "abc", "abd", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareDurationCmp(tt.a, tt.b)
			if (got < 0) != (tt.want < 0) || (got > 0) != (tt.want > 0) {
				t.Errorf("compareDurationCmp(%q, %q) = %d, want sign of %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCompareIPCmp(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"9 before 10 in last octet", "10.0.0.9", "10.0.0.10", -1},
		{"10 after 9", "10.0.0.10", "10.0.0.9", 1},
		{"identical", "172.16.0.1", "172.16.0.1", 0},
		{"first token of list drives compare", "10.0.0.2, 10.0.0.99", "10.0.0.3, 10.0.0.1", -1},
		{"non-IP falls back lexicographic", "None", "abc", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareIPCmp(tt.a, tt.b)
			if (got < 0) != (tt.want < 0) || (got > 0) != (tt.want > 0) {
				t.Errorf("compareIPCmp(%q, %q) = %d, want sign of %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCompareIPCmp_SortOrder(t *testing.T) {
	values := []string{"10.0.0.10", "10.0.0.9", "10.0.0.1", "10.0.0.100"}
	sort.Slice(values, func(i, j int) bool {
		return compareIPCmp(values[i], values[j]) < 0
	})
	want := []string{"10.0.0.1", "10.0.0.9", "10.0.0.10", "10.0.0.100"}
	for i := range want {
		if values[i] != want[i] {
			t.Fatalf("sorted = %v, want %v", values, want)
		}
	}
}

func TestComparePercentCmp(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"5% before 50%", "5%", "50%", -1},
		{"50% before 100%", "50%", "100%", -1},
		{"9% before 10%", "9%", "10%", -1},
		{"equal", "42%", "42%", 0},
		{"n/a sorts after real value", "n/a", "0%", 1},
		{"real value before n/a", "0%", "n/a", -1},
		{"n/a equals n/a", "n/a", "n/a", 0},
		{"whitespace tolerated", " 7% ", "12%", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := comparePercentCmp(tt.a, tt.b)
			if (got < 0) != (tt.want < 0) || (got > 0) != (tt.want > 0) {
				t.Errorf("comparePercentCmp(%q, %q) = %d, want sign of %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestComparePercentCmp_SortOrder(t *testing.T) {
	values := []string{"50%", "5%", "100%", "9%", "n/a", "0%"}
	sort.Slice(values, func(i, j int) bool {
		return comparePercentCmp(values[i], values[j]) < 0
	})
	want := []string{"0%", "5%", "9%", "50%", "100%", "n/a"}
	for i := range want {
		if values[i] != want[i] {
			t.Fatalf("sorted = %v, want %v", values, want)
		}
	}
}

func TestComparePrimaryColumn_PercentColumns(t *testing.T) {
	cols := []string{"CPU%", "MEM%", "CPU/R", "CPU/L", "MEM/R", "MEM/L"}
	for _, col := range cols {
		t.Run(col, func(t *testing.T) {
			a := itemWithCol(col, "9%")
			b := itemWithCol(col, "50%")
			if got := comparePrimaryColumn(a, b, col); got >= 0 {
				t.Errorf("comparePrimaryColumn(9%%, 50%%) on %s = %d, want < 0", col, got)
			}
			c := itemWithCol(col, "n/a")
			d := itemWithCol(col, "0%")
			if got := comparePrimaryColumn(c, d, col); got <= 0 {
				t.Errorf("comparePrimaryColumn(n/a, 0%%) on %s = %d, want > 0", col, got)
			}
		})
	}
}

func itemWithCol(key, val string) model.Item {
	return model.Item{Columns: []model.KeyValue{{Key: key, Value: val}}}
}
