package app

import (
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
)

// dashboardAccumulator collects partial section results until all expected
// sections (msg.total: 6 fixed + one per pinned summary) have arrived, then
// composes the final dashboardLoadedMsg.
type dashboardAccumulator struct {
	gen      uint64
	data     dashboardData
	received map[string]bool // by dashboardPartialMsg.key
	expected int
	count    int
	// startedAt is when this accumulator began collecting. loadDashboardFor
	// reads it to tell a fan-out that is merely slow from one that is stuck.
	startedAt time.Time
}

// dashboardFanOutStuckAfter bounds how long loadDashboardFor waits for an
// incomplete fan-out before giving up on it and starting a fresh one. It sits
// above scheduler.DefaultRequestTimeout so a section that is merely slow gets
// its full run before the fan-out is written off as stuck.
const dashboardFanOutStuckAfter = 45 * time.Second

func dashboardAccKey(kctx string, gen uint64) string {
	return kctx + ":" + strconv.FormatUint(gen, 10)
}

// handleDashboardPartial accumulates a section result and emits a
// single dashboardLoadedMsg only after all expected sections have arrived.
// This avoids flickering the dashboard layout on every watch tick
// (each tick fires one partial fetch per section. Rendering on each one
// would repeatedly clear sections that haven't arrived yet).
//
// Stale messages (different context or different requestGen) are
// dropped silently AND any half-built accumulator for that stale
// (context, gen) is evicted — otherwise navigating away mid-refresh
// would leak partial entries in m.dashboardAcc forever.
func (m Model) handleDashboardPartial(msg dashboardPartialMsg) (Model, tea.Cmd) {
	if msg.context != m.dashboardPreviewTargetContext() || msg.gen != m.requestGen {
		// Drop any partial accumulator left behind for this stale
		// (context, gen). The guarded m.dashboardAcc init lets us skip
		// the delete when the map is nil (test fixtures).
		if m.dashboardAcc != nil {
			delete(m.dashboardAcc, dashboardAccKey(msg.context, msg.gen))
		}
		return m, nil
	}
	key := dashboardAccKey(msg.context, msg.gen)
	if m.dashboardAcc == nil {
		// Lazy-init: production app_init.go pre-allocates this map, but
		// test fixtures with bare Model{} don't. The stale-drop branch
		// above already guards a nil map. Mirror that here so a current
		// partial arriving before init can't panic.
		m.dashboardAcc = make(map[string]*dashboardAccumulator)
	}
	acc, ok := m.dashboardAcc[key]
	if !ok {
		// Start from the frame already on screen so a section that answers
		// nothing this cycle keeps the value it had instead of blanking. The
		// pinned summaries are dropped because mergeDashboardSection appends
		// them, so carrying them over would duplicate every pinned row.
		seed := m.dashboardData[msg.context]
		seed.pinnedSummaries = nil
		acc = &dashboardAccumulator{
			gen: msg.gen, received: make(map[string]bool),
			expected: msg.total, startedAt: time.Now(), data: seed,
		}
		m.dashboardAcc[key] = acc
	} else {
		// A coalesced old fan-out (smaller total) can race a fresh one (larger
		// total, e.g. a pin added mid-flight) on the same (context, gen).
		// Awaiting the larger fan-out guarantees a full frame. The smaller
		// fan-out's keys are a subset delivered by surviving coalesced tasks.
		acc.expected = max(acc.expected, msg.total)
	}
	if !acc.received[msg.key] {
		acc.received[msg.key] = true
		acc.count++
		mergeDashboardSection(&acc.data, msg.data)
	}

	// With a frame already on screen, hold the repaint until every section has
	// arrived: the user keeps a complete (if slightly stale) dashboard instead
	// of watching sections blink in one by one.
	//
	// With nothing on screen there is no frame to protect, and waiting is what
	// leaves the page on its loading placeholder for as long as the slowest
	// section takes, or forever when one never answers (#646). Paint each
	// section as it lands instead.
	done := acc.count >= acc.expected
	if !done && m.dashboardPreview != "" {
		return m, nil
	}

	data := acc.data
	if done {
		delete(m.dashboardAcc, key)
	}
	return m, func() tea.Msg {
		return dashboardLoadedMsg{data: data, context: msg.context}
	}
}
