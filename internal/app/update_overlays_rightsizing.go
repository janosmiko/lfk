package app

import (
	"fmt"
	"math"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// handleRightsizingOverlayKey services the overlayRightsizing overlay.
// Vim-style nav matches lfk's other read-only overlays (NetworkPolicy,
// Help, Describe): j/k single-step, gg/G top/bottom, Ctrl+D/U half-
// page, Ctrl+F/B + PgDn/PgUp full-page. All scroll moves clamp via
// clampRightsizingScroll so spamming a key never pushes the table
// past the data.
//
//   - q / esc → close (clears state so re-open doesn't flash stale data)
//   - r       → invalidate cache + force-refresh
//   - y       → copy as strategic-merge YAML container block
//   - [ / ]   → cycle strategy (vim-style wrap, no-op when 1 strategy)
//   - < / >   → cycle headroom multiplier (vim-style wrap, snap-to-nearest)
func (m Model) handleRightsizingOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.rightsizing.data = nil
		m.rightsizing.err = nil
		m.rightsizing.loading = false
		m.rightsizing.scroll = 0
		return m, nil
	case "r":
		// Bust the cache entry for the active strategy + headroom and
		// re-fetch. Other entries stay so the user can `[ / ]` (strategy)
		// or `< / >` (headroom) back to them without re-hitting the cluster.
		key := rightsizingCacheKey(m.actionCtx.context, m.actionCtx.namespace, m.actionCtx.kind, m.actionCtx.name, m.rightsizing.strategy, m.rightsizing.headroom)
		delete(m.rightsizingCache, key)
		m.rightsizing.data = nil
		m.rightsizing.err = nil
		m.rightsizing.loading = true
		m.rightsizing.gen++
		return m, m.loadRightsizing()
	case "y":
		yaml := buildRightsizingYAML(m.rightsizing.data)
		if yaml == "" {
			m.setStatusMessage("Nothing to copy: no recommendations available", true)
			return m, scheduleStatusClear()
		}
		count := countContainersWithRecs(m.rightsizing.data)
		m.setStatusMessage(fmt.Sprintf("Copied right-sizing for %d container(s) as YAML", count), false)
		return m, tea.Batch(copyToSystemClipboard(yaml), scheduleStatusClear())
	case "j", "down":
		m.rightsizing.scroll = clampRightsizingScroll(m, m.rightsizing.scroll+1)
		return m, nil
	case "k", "up":
		m.rightsizing.scroll = clampRightsizingScroll(m, m.rightsizing.scroll-1)
		return m, nil
	case "g", "home":
		m.rightsizing.scroll = 0
		return m, nil
	case "G", "end":
		m.rightsizing.scroll = clampRightsizingScroll(m, math.MaxInt32)
		return m, nil
	case "ctrl+d":
		m.rightsizing.scroll = clampRightsizingScroll(m, m.rightsizing.scroll+rightsizingVisibleRows(m)/2)
		return m, nil
	case "ctrl+u":
		m.rightsizing.scroll = clampRightsizingScroll(m, m.rightsizing.scroll-rightsizingVisibleRows(m)/2)
		return m, nil
	case "ctrl+f", "pgdown":
		m.rightsizing.scroll = clampRightsizingScroll(m, m.rightsizing.scroll+rightsizingVisibleRows(m))
		return m, nil
	case "ctrl+b", "pgup":
		m.rightsizing.scroll = clampRightsizingScroll(m, m.rightsizing.scroll-rightsizingVisibleRows(m))
		return m, nil
	case "]":
		return m.cycleRightsizingStrategy(+1)
	case "[":
		return m.cycleRightsizingStrategy(-1)
	case ">":
		return m.cycleRightsizingHeadroom(+1)
	case "<":
		return m.cycleRightsizingHeadroom(-1)
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// cycleRightsizingHeadroom moves the active headroom multiplier
// forward or backward through model.RightsizingHeadrooms with vim-
// like wrap on either end. The direction sign is +1 for `>` and
// -1 for `<`.
//
// Snap-to-nearest semantics: if the current headroom doesn't match
// any preset value (e.g. a legacy 1.2 from a hardcoded constant or
// a hand-set value), the press snaps to the nearest neighbor in the
// press direction — `>` snaps UP to the smallest preset > current,
// `<` snaps DOWN to the largest preset < current. Pressing past the
// wrap edge from a snapped value lands on the wrap target as usual.
//
// Fast paths (in priority order) so the table never flashes through
// "Computing right-sizing…" on what is pure local arithmetic:
//
//  1. Cache hit for the (strategy, newHeadroom) pair → swap data
//     pointer in place, no fetch.
//  2. s.data populated → call k8s.RescaleRightsizing to multiply
//     every recommendation by newH/oldH locally; cache the result
//     under the new key for future revisits.
//  3. Fallback (cache miss AND s.data == nil) → async load. This
//     path is only hit on the first frame before
//     executeActionRightsizing's initial fetch lands.
func (m Model) cycleRightsizingHeadroom(direction int) (tea.Model, tea.Cmd) {
	headrooms := model.RightsizingHeadrooms
	if len(headrooms) < 2 {
		return m, nil
	}
	cur := m.rightsizing.headroom
	idx := indexOfHeadroom(cur, headrooms)
	var next float64
	if idx < 0 {
		// Snap-to-nearest in the press direction.
		next = snapHeadroomInDirection(cur, direction, headrooms)
	} else {
		nextIdx := (idx + direction + len(headrooms)) % len(headrooms)
		next = headrooms[nextIdx]
	}
	if next == cur {
		return m, nil
	}

	m.rightsizing.headroom = next
	m.rightsizing.scroll = 0

	cacheKey := rightsizingCacheKey(
		m.actionCtx.context, m.actionCtx.namespace, m.actionCtx.kind, m.actionCtx.name,
		m.rightsizing.strategy, next,
	)

	// Fast path 1: cache hit — swap to the cluster-fresh entry without
	// flashing through a loading state. Bump gen so any in-flight prior
	// fetch (e.g. from the previous strategy/headroom open) is dropped
	// on arrival rather than overwriting the cache hit.
	if cached, ok := m.rightsizingCache[cacheKey]; ok && cached != nil {
		m.rightsizing.gen++
		m.rightsizing.data = cached
		m.rightsizing.err = nil
		m.rightsizing.loading = false
		return m, nil
	}

	// Fast path 2: data in memory — every recommendation is just
	// base × headroom, so multiply locally and skip the fetch. Only
	// safe when the in-memory payload actually matches the currently-
	// selected strategy AND was generated at the current headroom AND
	// is not mid-fetch. During a strategy switch the renderer keeps
	// the previous strategy's data on screen with loading=true, and
	// rescaling that stale payload would store a cross-strategy
	// rescale under the new headroom key — then the older async
	// response can land and overwrite the newly selected headroom.
	canRescale := !m.rightsizing.loading &&
		m.rightsizing.data != nil &&
		m.rightsizing.data.Strategy == m.rightsizing.strategy &&
		math.Abs(m.rightsizing.data.Headroom-cur) < 1e-9
	if canRescale {
		rescaled := k8s.RescaleRightsizing(m.rightsizing.data, next)
		// RescaleRightsizing returns the input pointer unchanged when
		// the source has no headroom recorded (legacy fixture). Detect
		// that and fall through to the async path so the cluster
		// supplies an authoritative payload.
		if rescaled != nil && rescaled.Headroom == next {
			if m.rightsizingCache == nil {
				m.rightsizingCache = make(map[string]*model.Rightsizing)
			}
			m.rightsizingCache[cacheKey] = rescaled
			m.rightsizing.data = rescaled
			m.rightsizing.err = nil
			m.rightsizing.loading = false
			return m, nil
		}
	}

	// Fall back to an async fetch — rare, only the first-frame cold
	// path before executeActionRightsizing's initial load completes.
	m.rightsizing.data = nil
	m.rightsizing.err = nil
	m.rightsizing.loading = true
	m.rightsizing.gen++
	return m, m.loadRightsizing()
}

// indexOfHeadroom returns the position of `h` in `headrooms`, or -1
// if it doesn't match any preset. Uses an epsilon comparison so a
// value that round-tripped through %.2f / parse stays matchable.
func indexOfHeadroom(h float64, headrooms []float64) int {
	for i, v := range headrooms {
		if math.Abs(v-h) < 1e-9 {
			return i
		}
	}
	return -1
}

// snapHeadroomInDirection picks the preset to land on when the
// current headroom is not in the preset list. `direction == +1` →
// smallest preset > current (or the first preset when current is
// already past the top); `direction == -1` → largest preset < current
// (or the last preset when current is below the bottom). Mirrors the
// vim wrap behavior at the edges so a snapped press still cycles.
func snapHeadroomInDirection(current float64, direction int, headrooms []float64) float64 {
	if direction > 0 {
		for _, v := range headrooms {
			if v > current {
				return v
			}
		}
		return headrooms[0]
	}
	for i, v := range slices.Backward(headrooms) {
		if v < current {
			return headrooms[i]
		}
	}
	return headrooms[len(headrooms)-1]
}

// cycleRightsizingStrategy moves the active strategy forward or
// backward in the available list (vim-like wrap on either end). No-op
// when fewer than 2 strategies are available — there's nothing to
// cycle, and firing a load against the same key would just thrash the
// UI for no benefit.
//
// The direction sign is +1 for `]` and -1 for `[`.
//
// Two paths:
//
//  1. Cache hit on (newStrategy, headroom) → swap data pointer in
//     place; no async fetch and no flash.
//  2. Cache miss → kick the async load BUT keep the previous
//     strategy's data in s.data so the renderer can keep showing it
//     while the new fetch runs. The renderer combines `loading=true`
//     + `data != nil` into a "fetching new strategy" view (with a
//     subtle hint in the header) instead of wiping the table to
//     "Computing right-sizing…".
func (m Model) cycleRightsizingStrategy(direction int) (tea.Model, tea.Cmd) {
	avail := m.rightsizing.available
	if len(avail) < 2 {
		return m, nil
	}
	idx := -1
	for i, s := range avail {
		if s == m.rightsizing.strategy {
			idx = i
			break
		}
	}
	if idx < 0 {
		// Active strategy isn't in the available list (shouldn't
		// happen, but defend against state drift) — reset to the head.
		m.rightsizing.strategy = avail[0]
	} else {
		next := (idx + direction + len(avail)) % len(avail)
		m.rightsizing.strategy = avail[next]
	}

	cacheKey := rightsizingCacheKey(
		m.actionCtx.context, m.actionCtx.namespace, m.actionCtx.kind, m.actionCtx.name,
		m.rightsizing.strategy, m.rightsizing.headroom,
	)

	// Fast path: cache hit on the new strategy + same headroom — swap
	// to the cluster-fresh entry without flashing. Bump gen so any
	// in-flight prior strategy fetch is dropped on arrival rather
	// than overwriting the cache hit (the late response would still
	// match the old generation otherwise and silently win).
	if cached, ok := m.rightsizingCache[cacheKey]; ok && cached != nil {
		m.rightsizing.gen++
		m.rightsizing.data = cached
		m.rightsizing.err = nil
		m.rightsizing.loading = false
		m.rightsizing.scroll = 0
		return m, nil
	}

	// Cache miss — kick the async load but DO NOT wipe s.data. Keeping
	// the previous strategy's data on screen gives the user visual
	// continuity (the table fingerprint barely changes between
	// strategies) and the renderer adds a subtle header hint to signal
	// the fetch in progress. Scroll resets so the new strategy's
	// (potentially different) row count starts at the top.
	m.rightsizing.err = nil
	m.rightsizing.scroll = 0
	m.rightsizing.loading = true
	m.rightsizing.gen++
	return m, m.loadRightsizing()
}

// clampRightsizingScroll bounds a scroll target to [0, totalRows -
// visibleRows] so spamming j/G/Ctrl+D doesn't push content off the
// top of the visible window without a corresponding bring-back key.
// Mirrors the nav semantics used by the NetworkPolicy + Help
// overlays — same overlay family, same vim feel.
func clampRightsizingScroll(m Model, target int) int {
	if m.rightsizing.data == nil {
		return 0
	}
	totalRows := len(m.rightsizing.data.Containers) * 2
	maxScroll := max(0, totalRows-rightsizingVisibleRows(m))
	if target < 0 {
		return 0
	}
	if target > maxScroll {
		return maxScroll
	}
	return target
}

// buildRightsizingYAML serialises the recommendations into a
// strategic-merge `containers[]` block ready for kubectl patch.
// Containers with no non-empty recommendation are skipped so the
// output never contains an empty `resources:` stanza.
//
// Returns "" when the payload is nil or has no actionable
// recommendations — caller surfaces "nothing to copy."
func buildRightsizingYAML(data *model.Rightsizing) string {
	if data == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("containers:\n")
	hit := false
	for _, c := range data.Containers {
		if !containerHasAnyRecommendation(c) {
			continue
		}
		hit = true
		fmt.Fprintf(&b, "- name: %s\n", c.Name)
		b.WriteString("  resources:\n")
		writeResourceGroup(&b, "requests", c.CPU.RecommendedRequest, c.Mem.RecommendedRequest)
		writeResourceGroup(&b, "limits", c.CPU.RecommendedLimit, c.Mem.RecommendedLimit)
	}
	if !hit {
		return ""
	}
	return b.String()
}

func writeResourceGroup(b *strings.Builder, label, cpu, mem string) {
	if cpu == "" && mem == "" {
		return
	}
	fmt.Fprintf(b, "    %s:\n", label)
	if cpu != "" {
		fmt.Fprintf(b, "      cpu: %s\n", cpu)
	}
	if mem != "" {
		fmt.Fprintf(b, "      memory: %s\n", mem)
	}
}

func containerHasAnyRecommendation(c model.ContainerRec) bool {
	return c.CPU.RecommendedRequest != "" || c.CPU.RecommendedLimit != "" ||
		c.Mem.RecommendedRequest != "" || c.Mem.RecommendedLimit != ""
}

func countContainersWithRecs(data *model.Rightsizing) int {
	if data == nil {
		return 0
	}
	n := 0
	for _, c := range data.Containers {
		if containerHasAnyRecommendation(c) {
			n++
		}
	}
	return n
}
