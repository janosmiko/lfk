package scheduler

// Sig identifies an in-flight or queued scheduler task for coalescing.
// Two Submits with identical Sigs are treated as duplicates: the older
// queued entry is replaced by the newer one (newer wins). Sigs are
// compared by value; all five fields contribute to identity.
//
// Name is included so two distinct loads with the same Kind/Target/Gen
// do not collapse into each other — e.g. "List Deployments" (the main
// resource list at LevelResources) and "List Deployment children" (the
// owned-pod preview for the hovered Deployment) both run with
// Kind=ResourceList against the same context/namespace, but represent
// different requests that must both complete. Without Name in the Sig,
// each watch-tick refresh would drop whichever of the two was still
// queued, leaving the right-pane children stuck empty.
//
// Gen is the caller's requestGen at submission time so navigation that
// invalidates cached results also invalidates the coalesce signature
// (a stale-cancelled fetch and a fresh fetch live in different
// generations and therefore do not accidentally coalesce).
type Sig struct {
	KubeContext string
	Kind        Kind
	Name        string
	Target      string
	Gen         uint64
}

// NeverCoalesce returns true for Sigs whose Kind opts out of coalescing
// regardless of signature equality. Used for Mutations: two delete-pod
// calls with the same target must both run (defensive, since legitimate
// duplicates are rare but the cost of accidentally dropping a write is
// high).
func (s Sig) NeverCoalesce() bool {
	return s.Kind == KindMutation
}

// coalesceIgnoresGen reports whether two submissions of this Kind should
// coalesce even when their Gen differs. Dashboard sections re-fire on a
// fixed per-context target (e.g. "c1#metrics") every refresh; keeping Gen
// in their coalesce identity let a watch-tick or cursor-move resubmission
// stack a second full six-task batch behind the first instead of replacing
// it, so stale batches accumulated in the Low lane. Other Kinds keep Gen in
// their identity (see Sig's Gen doc) so a stale-cancelled fetch and a fresh
// fetch never collapse into one.
func (k Kind) coalesceIgnoresGen() bool {
	return k == KindDashboard
}

// CoalescesWith reports whether a queued task's Sig should be displaced by a
// newer submission with Sig s. Identity is Kind+Context+Name+Target; Gen is
// included only for Kinds that do not opt out via coalesceIgnoresGen.
func (s Sig) CoalescesWith(other Sig) bool {
	if s.Kind != other.Kind ||
		s.KubeContext != other.KubeContext ||
		s.Name != other.Name ||
		s.Target != other.Target {
		return false
	}
	if s.Kind.coalesceIgnoresGen() {
		return true
	}
	return s.Gen == other.Gen
}
