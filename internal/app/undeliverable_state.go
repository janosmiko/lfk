package app

import (
	"context"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

// undeliverableState bundles the Undeliverable-overlay UI state. Lives on
// Model so cursor, scroll, and filter query survive a close-and-reopen
// cycle - the same continuation the orphan overlay gives after jumping to
// a resource and coming back.
// One report per context is enough: the overlay is always cluster-wide, so
// there is no per-namespace slot to key on the way orphanCache needs.
type undeliverableState struct {
	loading      bool
	report       k8s.UndeliverableReport
	partial      error // non-nil => the scan was only partly authorised
	cursor       int
	scroll       int
	filter       TextInput
	filterActive bool

	loadedFor string // kubeContext the report belongs to
	inflight  bool
	cancel    context.CancelFunc
	gen       uint64 // bumped per scan so a superseded result is dropped on arrival
}

// visibleRows returns the rows the overlay shows right now: the whole
// report, narrowed by the filter query when one is set.
func (s undeliverableState) visibleRows() []ui.UndeliverableRow {
	items := s.report.All()
	out := make([]ui.UndeliverableRow, 0, len(items))
	for _, it := range items {
		row := ui.UndeliverableRow{
			Kind: it.Kind, Namespace: it.Namespace, Name: it.Name, Reason: it.Reason,
		}
		if ui.MatchesUndeliverableRow(row, s.filter.Value) {
			out = append(out, row)
		}
	}
	return out
}
