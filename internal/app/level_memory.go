package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/model"
)

// levelMemoryCap bounds how many views one tab remembers. Each entry carries a
// full item snapshot, so an uncapped map would hold every list the tab ever
// left. The oldest entry goes when a write would exceed the cap.
const levelMemoryCap = 24

// levelMemoryKey identifies one remembered view: the cluster the user was in
// and the level they left it at. Keying by cluster keeps one cluster's
// resource type out of another's.
type levelMemoryKey struct {
	context string
	level   model.Level
}

// levelMemory holds the view a tab left each cluster and level at, so the 1
// and 2 keys walk back down to them.
type levelMemory struct {
	views map[levelMemoryKey]navSnapshot
	order []levelMemoryKey // write order, oldest first, for the cap
	// lastContext is the cluster the user left last. The cluster picker has
	// no context of its own, so a restore from there needs this.
	lastContext string
}

// remember records the given view under its cluster and level.
func (l *levelMemory) remember(key levelMemoryKey, snap navSnapshot) {
	if l.views == nil {
		l.views = make(map[levelMemoryKey]navSnapshot)
	}
	// A rewrite moves the key back to the newest end, so the entry the user
	// just refreshed is not the next one evicted.
	for i, existing := range l.order {
		if existing == key {
			l.order = append(l.order[:i], l.order[i+1:]...)
			break
		}
	}
	for len(l.order) >= levelMemoryCap {
		delete(l.views, l.order[0])
		l.order = l.order[1:]
	}
	l.order = append(l.order, key)
	l.views[key] = snap
	l.lastContext = key.context
}

// clone deep-copies the memory so a tab save or restore shares no state with
// the live Model.
func (l levelMemory) clone() levelMemory {
	out := levelMemory{lastContext: l.lastContext}
	if l.views != nil {
		out.views = make(map[levelMemoryKey]navSnapshot, len(l.views))
		for k, v := range l.views {
			out.views[k] = v.clone()
		}
	}
	out.order = append([]levelMemoryKey(nil), l.order...)
	return out
}

// rememberLevel records the current view so the level keys can walk back down
// to it. navigateParent calls it for every level it leaves, which keeps the
// memory in step with plain back-navigation too.
func (m *Model) rememberLevel() {
	// Union mode holds the sentinel in nav.Context. restoreNavSnapshot checks
	// a snapshot's context against the kubeconfig, which the sentinel never
	// matches, so a union snapshot would only ever restore as an error.
	if m.nav.Context == "" || m.unionMode {
		return
	}
	m.levelMem.remember(levelMemoryKey{context: m.nav.Context, level: m.nav.Level}, m.captureNavSnapshot())
}

// restoreLevel replays the view remembered for the given level, and reports
// whether it had one. At the cluster picker the lookup uses the cluster the
// user last left, because nav.Context is empty there.
func (m *Model) restoreLevel(level model.Level) (tea.Cmd, bool) {
	ctx := m.nav.Context
	if m.nav.Level == model.LevelClusters {
		ctx = m.levelMem.lastContext
	}
	if ctx == "" {
		return nil, false
	}
	snap, ok := m.levelMem.views[levelMemoryKey{context: ctx, level: level}]
	if !ok {
		return nil, false
	}
	return m.restoreNavSnapshot(snap), true
}
