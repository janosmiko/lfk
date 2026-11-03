package ui

import (
	"slices"

	"github.com/janosmiko/lfk/internal/model"
)

// conditionsByRecency returns a copy of conds with the newest transition first
// and untimed conditions last in API order. Only the details pane uses it: the
// list summary picks the "last" condition and needs the original API order.
func conditionsByRecency(conds []model.ConditionEntry) []model.ConditionEntry {
	out := slices.Clone(conds)
	slices.SortStableFunc(out, func(a, b model.ConditionEntry) int {
		switch {
		case a.LastTransitionTime.IsZero() || b.LastTransitionTime.IsZero():
			// Timed before untimed. Two untimed entries compare equal.
			return boolRank(a.LastTransitionTime.IsZero()) - boolRank(b.LastTransitionTime.IsZero())
		default:
			return b.LastTransitionTime.Compare(a.LastTransitionTime)
		}
	})
	return out
}

func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}
