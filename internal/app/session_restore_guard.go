package app

// A restored session lands in two steps: the navigation to the saved resource
// type happens as soon as the context list arrives, but the saved row can only
// be selected once the resource list itself comes back, often a second later.
// restoreCursorAfterLoad applies pendingTarget at that point regardless of what
// the user did in between, so a keystroke during the gap used to be silently
// undone. The guard below settles the race in the user's favour.

// finishSessionRestore marks the restore complete. The saved cursor targets stay
// armed so the row the user quit on is still selected when the list lands.
func (m *Model) finishSessionRestore() {
	m.restoringSession = false
}

// finishSessionRestoreForContext marks the restore complete when the discovery
// reply that just landed belongs to the context the restore was waiting on.
// Replies for background contexts say nothing about that restore.
func (m *Model) finishSessionRestoreForContext(isCurrentContext bool) {
	if isCurrentContext {
		m.restoringSession = false
	}
}

// abandonSessionRestore drops everything the restore still wanted to do. Called
// on the first keystroke after launch: the user is steering now, and a load that
// lands seconds later must not pull the cursor back to the saved row.
//
// The restoringSession guard around every call site matters. pendingTarget also
// carries bookmark jumps and orphan drill-downs, so clearing it on a keystroke
// once the restore is over would break those.
func (m *Model) abandonSessionRestore() {
	m.restoringSession = false
	m.pendingTarget = ""
	m.pendingTargetNamespace = ""
	m.pendingSessionList = pendingSessionListState{}
	m.sessionResourceTypeAwaitingDiscovery = ""
	m.sessionResourceNameAwaitingDiscovery = ""
}
