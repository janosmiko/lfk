package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/k8s"
)

// handleCrashInvestigatorOverlayKey routes overlay keys for the Crash
// Investigator: tab navigation, container switching, logs prev/curr,
// refresh, scroll, and close.
//
// Scroll bookkeeping is per-(tab, container): Logs and Describe are
// container-scoped, Summary and Events are pod-scoped (container blank).
// The dispatcher only mutates the offset; the renderer is responsible
// for clamping it to the active viewport, so writing 999999 here is
// sufficient to mean "jump to bottom".
func (m Model) handleCrashInvestigatorOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.crashInv = crashInvState{}
		return m, nil
	case "tab":
		m.crashInv.activeTab = nextCrashTab(m.crashInv.activeTab, +1)
		return m, nil
	case "shift+tab":
		m.crashInv.activeTab = nextCrashTab(m.crashInv.activeTab, -1)
		return m, nil
	case "1":
		m.crashInv.activeTab = crashInvTabSummary
		return m, nil
	case "2":
		m.crashInv.activeTab = crashInvTabEvents
		return m, nil
	case "3":
		m.crashInv.activeTab = crashInvTabLogs
		return m, nil
	case "4":
		m.crashInv.activeTab = crashInvTabDescribe
		return m, nil
	case "c":
		m.crashInv.activeContainer = nextCrashContainer(m.crashInv.data, m.crashInv.activeContainer)
		return m, nil
	case "p":
		if m.crashInv.activeTab == crashInvTabLogs {
			m.crashInv.showPrevious = !m.crashInv.showPrevious
		}
		return m, nil
	case "R":
		m.loading = true
		m.setStatusMessage("Refreshing crash investigation…", false)
		return m, m.loadCrashInvestigation()
	case "j", "down":
		m.crashInv.bumpScroll(m.scrollKey(), +1)
		return m, nil
	case "k", "up":
		m.crashInv.bumpScroll(m.scrollKey(), -1)
		return m, nil
	case "g":
		m.crashInv.setScroll(m.scrollKey(), 0)
		return m, nil
	case "G":
		// Sentinel — renderer clamps to maxScroll.
		m.crashInv.setScroll(m.scrollKey(), 999999)
		return m, nil
	case "ctrl+d", "shift+down":
		// Half-page (vim) — renderer clamps to maxScroll.
		m.crashInv.bumpScroll(m.scrollKey(), +10)
		return m, nil
	case "ctrl+u", "shift+up":
		m.crashInv.bumpScroll(m.scrollKey(), -10)
		return m, nil
	case "ctrl+f", "pgdown":
		// Full-page (vim) — renderer clamps to maxScroll.
		m.crashInv.bumpScroll(m.scrollKey(), +20)
		return m, nil
	case "ctrl+b", "pgup":
		m.crashInv.bumpScroll(m.scrollKey(), -20)
		return m, nil
	}
	return m, nil
}

// scrollKey returns the (tab, container) lookup used to index into
// crashInvState.scroll. Logs and Describe are container-scoped so the
// reader's position survives container switches; Summary and Events
// share a single key per tab.
func (m Model) scrollKey() crashInvScrollKey {
	container := ""
	if m.crashInv.activeTab == crashInvTabLogs || m.crashInv.activeTab == crashInvTabDescribe {
		container = m.crashInv.activeContainer
	}
	return crashInvScrollKey{tab: m.crashInv.activeTab, container: container}
}

// bumpScroll adjusts the scroll offset for key by delta, clamped to
// >= 0. Lazily initializes the underlying map. Upper-bound clamping
// happens in the renderer where the actual viewport height is known.
func (s *crashInvState) bumpScroll(key crashInvScrollKey, delta int) {
	if s.scroll == nil {
		s.scroll = map[crashInvScrollKey]int{}
	}
	v := max(s.scroll[key]+delta, 0)
	s.scroll[key] = v
}

// setScroll overwrites the scroll offset for key. Negative values are
// clamped to zero. Used for g (top) and G (bottom-sentinel) jumps.
func (s *crashInvState) setScroll(key crashInvScrollKey, v int) {
	if s.scroll == nil {
		s.scroll = map[crashInvScrollKey]int{}
	}
	if v < 0 {
		v = 0
	}
	s.scroll[key] = v
}

// nextCrashTab returns the tab `delta` steps forward (or backward when
// delta is negative) with wraparound.
func nextCrashTab(t crashInvTab, delta int) crashInvTab {
	const n = 4
	return crashInvTab((int(t) + delta + n) % n)
}

// nextCrashContainer returns the next container name in the combined
// init+app declaration order, wrapping at the end. Returns the input
// unchanged when info is nil or there are no containers.
func nextCrashContainer(info *k8s.CrashInvestigation, current string) string {
	if info == nil {
		return current
	}
	all := make([]string, 0, len(info.InitContainers)+len(info.AppContainers))
	for _, c := range info.InitContainers {
		all = append(all, c.Name)
	}
	for _, c := range info.AppContainers {
		all = append(all, c.Name)
	}
	if len(all) == 0 {
		return current
	}
	for i, name := range all {
		if name == current {
			return all[(i+1)%len(all)]
		}
	}
	return all[0]
}
