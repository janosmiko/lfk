package app

import (
	"fmt"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
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

// dependentsNotes builds the Dependents row for the confirm box. The row
// restates the same set of objects under whichever policy Tab has selected, so
// the user reads the result of the choice rather than its name.
//
// A nil count means the walk cannot answer for this kind, and the row is left
// out rather than printed as a confident zero.
func dependentsNotes(s dependentsState, policy model.DeletePropagation) []ui.ConfirmNote {
	if s.loading {
		return []ui.ConfirmNote{{Label: "Dependents", Text: "counting..."}}
	}
	if s.count == nil {
		return nil
	}

	summary := s.count.Summary()
	if summary == "" {
		return []ui.ConfirmNote{{
			Label: "Dependents",
			Text:  "none" + uncountedSuffix(s.count.Uncounted),
		}}
	}

	text, warn := summary+" also removed", false
	switch {
	case policy.OrphansDependents():
		text, warn = summary+" stay in the cluster", true
	case policy.DefersToServer():
		// None sends no policy at all. Most kinds default to background on the
		// server, so promising they stay would be a promise the dialog cannot
		// keep. Kept short enough to fit the box on one line.
		text, warn = summary+" may stay (server decides)", true
	}
	return []ui.ConfirmNote{{
		Label: "Dependents",
		Text:  text + uncountedSuffix(s.count.Uncounted),
		Warn:  warn,
	}}
}

// uncountedSuffix declares the rows the walk left out, so a bulk total never
// reads as complete when it is not.
func uncountedSuffix(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf(" (%d %s not counted)", n, plural(n, "row", "rows"))
}
