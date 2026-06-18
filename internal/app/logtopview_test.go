package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/logagg"
)

func TestColumnsOverlayRenderCursor(t *testing.T) {
	tests := []struct {
		name   string
		cursor int
		nDims  int
		nMets  int
		want   int
	}{
		{"cursor in dims, both groups present", 0, 3, 2, 0},
		{"cursor on last dim, both groups present", 2, 3, 2, 2},
		{"cursor on first metric, both groups present", 3, 3, 2, 4},
		{"cursor on second metric, both groups present", 4, 3, 2, 5},
		{"no dims, cursor on metric", 1, 0, 3, 1},
		{"no metrics, cursor in dims", 1, 3, 0, 1},
		{"only dims", 0, 3, 0, 0},
		{"only metrics", 0, 0, 2, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := columnsOverlayRenderCursor(tc.cursor, tc.nDims, tc.nMets)
			if got != tc.want {
				t.Errorf("columnsOverlayRenderCursor(%d, %d, %d) = %d, want %d",
					tc.cursor, tc.nDims, tc.nMets, got, tc.want)
			}
		})
	}
}

func TestLogTopState_Copy_DeepCopiesSlices(t *testing.T) {
	s := logTopState{
		groupBy: []string{logagg.FieldMethod, logagg.FieldPath},
		parsed:  []logagg.Fields{{logagg.FieldMethod: "GET"}},
	}
	c := s.copy()
	c.groupBy[0] = "mutated"
	if s.groupBy[0] != logagg.FieldMethod {
		t.Error("copy() must not share the groupBy slice")
	}
	c.parsed = append(c.parsed, logagg.Fields{logagg.FieldStatus: "500"})
	if len(s.parsed) != 1 {
		t.Error("copy() must not share the parsed slice backing array")
	}
}
