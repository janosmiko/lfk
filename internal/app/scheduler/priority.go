package scheduler

// Priority classifies a task for scheduler dispatch. Lower numeric value =
// higher priority so a sort-ascending naturally puts Critical first.
//
// Critical: foundational work that gates other UI (API discovery, RBAC,
// namespaces) and destructive mutations. Has a reserved worker slot.
// High: the user's current view (resource list, drill-in, preview) — the
// thing the user is staring at right now.
// Low: decorative / background work (dashboard, metrics, resource tree)
// that may be preempted by a Critical or High submission.
type Priority int

const (
	PriorityCritical Priority = iota
	PriorityHigh
	PriorityLow
)

// String returns the human-readable label for a Priority.
func (p Priority) String() string {
	switch p {
	case PriorityCritical:
		return "Critical"
	case PriorityHigh:
		return "High"
	case PriorityLow:
		return "Low"
	default:
		return "Unknown"
	}
}

// DefaultPriorityFor maps a Kind to its default scheduler Priority.
// Callers can override at submission time (e.g., a watch-tick refresh of
// a non-active tab is downgraded to Low even though the Kind defaults to
// High). Unknown Kinds default to Low to keep the scheduler conservative.
func DefaultPriorityFor(k Kind) Priority {
	switch k {
	case KindAPIDiscovery, KindNamespaceList, KindRBACCheck, KindMutation:
		return PriorityCritical
	case KindResourceList, KindContainers, KindYAMLFetch:
		return PriorityHigh
	case KindMetrics, KindResourceTree, KindDashboard:
		return PriorityLow
	default:
		return PriorityLow
	}
}
