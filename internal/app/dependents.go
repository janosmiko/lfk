package app

import (
	"fmt"

	"github.com/janosmiko/lfk/internal/k8s"
)

// dependentsState is what one delete confirm knows about the objects the
// cascade would take with the target. It is separate from blastRadiusState
// because the two answer different questions and arrive independently: one
// counts pods against budgets, this one walks the owner graph.
type dependentsState struct {
	count   *k8s.DependentCount
	loading bool
	// req numbers each walk, so a reply for a dialog the user already closed
	// and reopened cannot land on the new one.
	req uint64
}

func (s *dependentsState) reset() {
	s.count = nil
	s.loading = false
}

// uncountedSuffix declares the rows the walk left out, so a bulk total never
// reads as complete when it is not.
func uncountedSuffix(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf(" (%d %s not counted)", n, plural(n, "row", "rows"))
}
