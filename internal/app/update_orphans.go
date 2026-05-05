package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// orphansVisibleLines returns the body row capacity of the orphan
// overlay for the current Model state. MUST match the math in
// renderOrphansOverlay / ui.OrphanBodyHeight — when these disagree, the
// cursor lands on a row the renderer doesn't actually emit and the user
// sees the last item clipped past the bottom of the viewport.
//
// All 11 kind chips always render, so the strip wraps to 2 rows on a
// typical 100-col overlay and to 3+ rows on narrow terminals. Compute
// the wrap count via ui.OrphanChipLines so this matches the renderer's
// layout exactly — without it, a hardcoded worst-case would either
// reserve too few lines (cursor escapes the viewport) or too many
// (waste body rows on wide overlays).
func (m Model) orphansVisibleLines() int {
	hasPartial := m.orphans.partial != nil
	hasSearch := m.orphans.filterActive || m.orphans.filter.Value != ""
	innerW := max(m.orphansOverlayW()-4, 40) // mirror RenderOrphansOverlay
	chipLines := ui.OrphanChipLines(m.orphanCounts(), innerW)
	return ui.OrphanBodyHeight(m.orphansOverlayH(), chipLines, hasPartial, hasSearch)
}

// orphansOverlayH returns the outer overlay height used by both the
// renderer and the move handlers. Centralised so changing the overlay
// size only needs one edit.
func (m Model) orphansOverlayH() int {
	return min(30, m.height-6)
}

// orphansOverlayW returns the outer overlay width.
func (m Model) orphansOverlayW() int {
	return min(100, m.width-10)
}

// orphanCacheNamespace returns the namespace component of the orphan
// cache key for the model's current list scope. All-namespaces and
// multi-namespace selections collapse to the empty string so the
// cluster-wide report is reused — without that, a single-namespace
// cache slot would shadow the broader list and `:orphans <kind>` would
// miss valid rows in the default broad-scope flows.
func (m Model) orphanCacheNamespace() string {
	if m.allNamespaces || len(m.selectedNamespaces) > 1 {
		return ""
	}
	return m.namespace
}

// orphansMoveCursor moves the cursor by delta within the visible
// (post-filter, post-kind-chip) list, clamps at the edges, and updates
// the scroll offset to keep the cursor in view via vim-like
// only-scroll-when-leaving-viewport semantics.
func (m Model) orphansMoveCursor(delta int) (Model, tea.Cmd) {
	visible := m.orphans.visibleItems()
	if len(visible) == 0 {
		m.orphans.cursor = 0
		m.orphans.scroll = 0
		return m, nil
	}
	next := max(min(m.orphans.cursor+delta, len(visible)-1), 0)
	m.orphans.cursor = next
	m.orphans.scroll = ui.OrphanScrollForCursor(
		m.orphans.scroll, m.orphans.cursor,
		m.orphansVisibleLines(), len(visible),
	)
	return m, nil
}

// orphansJumpCursor positions the cursor at an absolute index (clamped
// into the visible list) and adjusts the scroll. Powers g/G/Home/End.
func (m Model) orphansJumpCursor(idx int) (Model, tea.Cmd) {
	visible := m.orphans.visibleItems()
	if len(visible) == 0 {
		m.orphans.cursor = 0
		m.orphans.scroll = 0
		return m, nil
	}
	idx = max(min(idx, len(visible)-1), 0)
	m.orphans.cursor = idx
	m.orphans.scroll = ui.OrphanScrollForCursor(
		m.orphans.scroll, m.orphans.cursor,
		m.orphansVisibleLines(), len(visible),
	)
	return m, nil
}

// orphansResetCursor moves the cursor to the top of the (now possibly
// shrunk) visible list and clamps the scroll. Called after the kind
// chip changes or the filter narrows the list, so a stale cursor/scroll
// pair from a longer prior list doesn't render past the end.
func (m Model) orphansResetCursor() Model {
	m.orphans.cursor = 0
	m.orphans.scroll = ui.OrphanClampScroll(
		0, len(m.orphans.visibleItems()), m.orphansVisibleLines(),
	)
	return m
}

// orphansClampCursor keeps an existing cursor/scroll pair valid against
// the current visible list. Called on overlay open so a cursor remembered
// across a close/reopen cycle still points at a row that exists — when a
// background refresh shrinks the report (a now-mounted Secret leaves the
// orphan list) the old cursor index would otherwise render past the end.
// Unlike orphansResetCursor it preserves the user's position when valid.
func (m Model) orphansClampCursor() Model {
	visible := m.orphans.visibleItems()
	if len(visible) == 0 {
		m.orphans.cursor = 0
		m.orphans.scroll = 0
		return m
	}
	if m.orphans.cursor >= len(visible) {
		m.orphans.cursor = len(visible) - 1
	}
	if m.orphans.cursor < 0 {
		m.orphans.cursor = 0
	}
	m.orphans.scroll = ui.OrphanScrollForCursor(
		m.orphans.scroll, m.orphans.cursor,
		m.orphansVisibleLines(), len(visible),
	)
	return m
}

// handleOrphansKey routes one key press inside the orphan overlay.
// Returns the new model and any tea.Cmd (e.g. for refresh).
func (m Model) handleOrphansKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.orphans.filterActive {
		return m.handleOrphansFilterInput(msg)
	}

	key := msg.String()
	switch key {
	case "tab":
		m.orphans.visibleKind = (m.orphans.visibleKind + 1) % orphanKindMax
		return m.orphansResetCursor(), nil
	case "shift+tab":
		m.orphans.visibleKind = (m.orphans.visibleKind + orphanKindMax - 1) % orphanKindMax
		return m.orphansResetCursor(), nil
	case "j", "down":
		return m.orphansMoveCursor(+1)
	case "k", "up":
		return m.orphansMoveCursor(-1)
	case "g", "home":
		return m.orphansJumpCursor(0)
	case "G", "end":
		return m.orphansJumpCursor(len(m.orphans.visibleItems()) - 1)
	case "ctrl+d":
		return m.orphansMoveCursor(+max(m.orphansVisibleLines()/2, 1))
	case "ctrl+u":
		return m.orphansMoveCursor(-max(m.orphansVisibleLines()/2, 1))
	case "ctrl+f", "pgdown":
		return m.orphansMoveCursor(+max(m.orphansVisibleLines(), 1))
	case "ctrl+b", "pgup":
		return m.orphansMoveCursor(-max(m.orphansVisibleLines(), 1))
	case "/":
		m.orphans.filterActive = true
		return m, nil
	case "s":
		// Toggle strict ↔ lenient. Strict (default) hides items with
		// no live consumer but a workload-template ref; lenient
		// surfaces them so the user can audit "what's currently idle"
		// in addition to "what's truly unused". Visible-list size
		// changes, so re-clamp the cursor afterwards.
		m.orphans.strict = !m.orphans.strict
		return m.orphansClampCursor(), nil
	case "R":
		key := orphanCacheKey{kubeContext: m.nav.Context, namespace: ""}
		delete(m.orphanCache, key)
		m.orphans.loading = true
		return m, (&m).cmdLoadOrphans(key)
	case "enter":
		return m.jumpToOrphan()
	case "esc", "q":
		// Both close the overlay. q matches the convention used by
		// the Can-I / Who-Can overlay (closest sibling) — without it,
		// users coming from those views type q and watch nothing
		// happen, then have to remember Esc.
		updated := m.closeOrphansOverlay()
		return updated, nil
	}
	return m, nil
}

// handleOrphansFilterInput handles keys while the overlay is in /-filter
// input mode. Esc cancels (clears filter), Enter accepts (exits input
// mode but keeps the filter applied).
func (m Model) handleOrphansFilterInput(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.orphans.filter.Clear()
		m.orphans.filterActive = false
		return m.orphansResetCursor(), nil
	case "enter":
		m.orphans.filterActive = false
		return m, nil
	case "backspace":
		m.orphans.filter.Backspace()
		return m.orphansResetCursor(), nil
	default:
		// Accept printable single-rune keys.
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
			m.orphans.filter.Insert(string(msg.Runes[0]))
			return m.orphansResetCursor(), nil
		}
		return m, nil
	}
}

// openOrphansOverlay sets up the cluster-wide orphan overlay. Uses an
// empty namespace (cluster-wide) — the per-namespace path is the filter
// preset.
//
// Cursor / scroll / visibleKind / filter query are PRESERVED across
// close-and-reopen cycles so that pressing Enter to jump to a resource
// and then reopening (Shift+O) lands the user on the orphan they were
// last looking at — not at the top of the list. Only filterActive is
// forced to false so the next keypress drives navigation rather than
// being captured as filter input. Context-switch invalidation (via
// invalidateOrphanCacheForContext) blows away the cache; a fresh
// cmdLoadOrphans then arrives and orphansClampCursor pulls the cursor
// into the new list.
func (m Model) openOrphansOverlay() (Model, tea.Cmd) {
	m.overlay = overlayOrphans
	m.orphans.filterActive = false

	key := orphanCacheKey{kubeContext: m.nav.Context, namespace: ""}
	if cached := m.orphanCache[key]; cached != nil {
		m.orphans.report = *cached
		m.orphans.loading = false
		m = m.orphansClampCursor()
		return m, nil
	}
	m.orphans.loading = true
	return m, (&m).cmdLoadOrphans(key)
}

// closeOrphansOverlay returns the overlay to its closed state. The
// cached report stays in `Model.orphans.report` and the user's cursor /
// scroll / kind chip / filter query all stay in `Model.orphans` so
// reopening with Shift+O resumes exactly where the user was — that's
// what makes "Enter → jump → look around → Shift+O" feel like a
// continuation rather than starting from scratch.
//
// Only filterActive resets so pressing Esc out of the overlay doesn't
// leave the next session in input-capture mode.
func (m Model) closeOrphansOverlay() Model {
	m.overlay = overlayNone
	m.orphans.filterActive = false
	return m
}

// jumpToOrphan navigates to the highlighted orphan: switches namespace,
// opens the resource type with the cursor positioned on the orphan
// resource, and closes the overlay. Mirrors navigateToOwner's pattern
// (look up via discoveredResources, set pendingTarget) so the jump is
// reliable across cold-discovery / mid-load states. The earlier
// executeResourceJump-based implementation scanned middleItems by string
// match — when middleItems was empty (post-restart, just-entered
// context) the scan silently failed.
func (m Model) jumpToOrphan() (Model, tea.Cmd) {
	visible := m.orphans.visibleItems()
	if m.orphans.cursor < 0 || m.orphans.cursor >= len(visible) {
		return m, nil
	}
	target := visible[m.orphans.cursor]

	rt, ok := model.FindResourceTypeByKind(target.Kind, m.discoveredResources[m.nav.Context])
	if !ok {
		// Discovery hasn't completed yet (or the kind isn't a known one
		// in this cluster). Tell the user — silent no-op was the cause
		// of the "sometimes Enter doesn't jump" bug.
		m.setStatusMessage(
			fmt.Sprintf("Cannot jump to %s/%s — resource type not yet discovered, retry in a moment",
				target.Kind, target.Name), true)
		return m, scheduleStatusClear()
	}

	m = m.closeOrphansOverlay()

	// Switch namespace before navigating so the resource list loads
	// scoped correctly. Cluster-wide orphan list → single-namespace
	// resource list, so all-namespaces must be off and any prior
	// multi-namespace selection has to be replaced — otherwise the
	// resource load can include extra namespaces or even drop the
	// target one, making Enter-to-jump look broken.
	m.allNamespaces = false
	m.namespace = target.Namespace
	m.selectedNamespaces = map[string]bool{target.Namespace: true}

	// Climb back to LevelResourceTypes regardless of where the user was
	// when they opened the overlay.
	for m.nav.Level > model.LevelResourceTypes {
		ret, _ := m.navigateParent()
		m = ret.(Model)
	}
	if m.nav.Level < model.LevelResourceTypes {
		m.setStatusMessage("Cannot jump: enter a context first", true)
		return m, scheduleStatusClear()
	}

	// Position the parent-level cursor on the resource type by its
	// canonical group/version/resource ref — matches what navigateToOwner
	// does. Falls through to a status message if the kind isn't in the
	// current sidebar (rare-resource toggle hiding it, etc.).
	for i, item := range m.middleItems {
		if item.Extra == rt.ResourceRef() {
			m.setCursor(i)
			m.pendingTarget = target.Name
			ret, cmd := m.navigateChild()
			next, ok := ret.(Model)
			if !ok {
				return m, cmd
			}
			return next, cmd
		}
	}

	m.setStatusMessage(
		fmt.Sprintf("Cannot jump: %s not in sidebar (toggle rare resources with H?)", target.Kind),
		true)
	return m, scheduleStatusClear()
}
