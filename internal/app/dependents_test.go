package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
)

func TestDependentsStateReset(t *testing.T) {
	s := dependentsState{count: &k8s.DependentCount{Total: 3}, loading: true}
	s.reset()
	if s.count != nil || s.loading {
		t.Fatalf("reset left state populated: %+v", s)
	}
}
