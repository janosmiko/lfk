package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/logagg"
)

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
