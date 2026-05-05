package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/k8s"
	mdl "github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenOrphansOverlay_SetsOverlayAndFiresLoad(t *testing.T) {
	m := newTestModel()
	m.nav.Context = "test"

	updated, cmd := m.openOrphansOverlay()

	assert.Equal(t, overlayOrphans, updated.overlay)
	assert.True(t, updated.orphans.loading)
	require.NotNil(t, cmd)
}

// TestCloseOrphansOverlay_PreservesResumableState pins the contract
// the user reported as broken: closing the overlay must leave cursor /
// scroll / visibleKind / filter intact so Shift+O resumes where they
// were. Only filterActive flips to false so the next session doesn't
// start in input-capture mode.
func TestCloseOrphansOverlay_PreservesResumableState(t *testing.T) {
	m := newTestModel()
	m.overlay = overlayOrphans
	m.orphans.report = k8s.OrphanReport{Pods: []k8s.OrphanItem{{Name: "x"}}}
	m.orphans.cursor = 5
	m.orphans.scroll = 3
	m.orphans.visibleKind = orphanKindSecret
	m.orphans.filter.Set("foo")
	m.orphans.filterActive = true

	updated := m.closeOrphansOverlay()

	assert.Equal(t, overlayNone, updated.overlay)
	assert.Equal(t, "x", updated.orphans.report.Pods[0].Name, "report stays cached")
	assert.Equal(t, 5, updated.orphans.cursor, "cursor preserved across close")
	assert.Equal(t, 3, updated.orphans.scroll, "scroll preserved across close")
	assert.Equal(t, orphanKindSecret, updated.orphans.visibleKind, "kind chip preserved")
	assert.Equal(t, "foo", updated.orphans.filter.Value, "filter query preserved")
	assert.False(t, updated.orphans.filterActive, "filterActive forced false")
}

// TestOrphansOverlay_ResumeCursorAfterReopen is the headline regression:
// open → move cursor to row 5 → close (e.g. via Esc, or implicitly via
// Enter+jump+come-back-with-Shift+O) → reopen → cursor must still be on
// row 5. Cache is populated so the open path takes the cached branch
// (no async load), exercising the same code path as the user's flow.
func TestOrphansOverlay_ResumeCursorAfterReopen(t *testing.T) {
	m := newTestModel()
	m.height = 30
	m.width = 100
	m.nav.Context = "test"
	pods := make([]k8s.OrphanItem, 0, 20)
	for i := range 20 {
		pods = append(pods, k8s.OrphanItem{Kind: "Pod", Namespace: "ns", Name: fmt.Sprintf("p%02d", i)})
	}
	report := k8s.OrphanReport{Pods: pods}
	m.orphanCache[orphanCacheKey{kubeContext: "test", namespace: ""}] = &report

	// First open + walk cursor to 5.
	m, _ = m.openOrphansOverlay()
	m.orphans.visibleKind = orphanKindPod
	for range 5 {
		m, _ = m.handleOrphansKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}
	require.Equal(t, 5, m.orphans.cursor, "fixture sanity")

	// Close (e.g. Esc, or Enter that closes the overlay) and reopen.
	m = m.closeOrphansOverlay()
	m, _ = m.openOrphansOverlay()

	assert.Equal(t, 5, m.orphans.cursor, "cursor must resume at 5 after close+reopen")
	assert.Equal(t, orphanKindPod, m.orphans.visibleKind, "kind chip must resume")
}

// TestOpenOrphansOverlay_ClampsStaleCursor covers the data-changed case:
// cursor at 19 from a prior session, but the new report only has 5 rows.
// The clamp must pull the cursor into range so the overlay doesn't try
// to highlight a row that no longer exists.
func TestOpenOrphansOverlay_ClampsStaleCursor(t *testing.T) {
	m := newTestModel()
	m.height = 30
	m.width = 100
	m.nav.Context = "test"
	m.orphans.cursor = 19
	m.orphans.visibleKind = orphanKindPod
	smaller := k8s.OrphanReport{Pods: []k8s.OrphanItem{
		{Kind: "Pod", Name: "a"}, {Kind: "Pod", Name: "b"}, {Kind: "Pod", Name: "c"},
	}}
	m.orphanCache[orphanCacheKey{kubeContext: "test", namespace: ""}] = &smaller

	m, _ = m.openOrphansOverlay()

	assert.Equal(t, 2, m.orphans.cursor, "stale cursor pulled into the new list's range")
}

func TestRenderOrphansOverlay_Loading(t *testing.T) {
	m := newTestModel()
	m.height = 30
	m.width = 100
	m.orphans.loading = true
	out, _, _ := m.renderOrphansOverlay()
	assert.Contains(t, out, "Scanning")
}

func TestRenderOrphansOverlay_Empty(t *testing.T) {
	m := newTestModel()
	m.height = 30
	m.width = 100
	out, _, _ := m.renderOrphansOverlay()
	assert.Contains(t, out, "No orphans found")
}

func TestRenderOrphansOverlay_PartialBanner(t *testing.T) {
	m := newTestModel()
	m.height = 30
	m.width = 100
	m.orphans.partial = errors.New("listing ingresses: forbidden")
	out, _, _ := m.renderOrphansOverlay()
	assert.Contains(t, out, "partial result")
}

// TestOrphansOverlay_QClosesLikeEsc pins q as a close key — Can-I /
// Who-Can already accept q for close, so users coming from those views
// type q and expected nothing-happened was friction. Both q and esc go
// through closeOrphansOverlay; this test covers q since esc is already
// covered by the broader test suite.
func TestOrphansOverlay_QClosesOverlay(t *testing.T) {
	m := newTestModel()
	m.overlay = overlayOrphans
	m.orphans.cursor = 5

	updated, _ := m.handleOrphansKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	assert.Equal(t, overlayNone, updated.overlay, "q must close the overlay")
}

func TestOrphansOverlay_TabCyclesKind(t *testing.T) {
	m := newTestModel()
	m.overlay = overlayOrphans
	m.orphans.visibleKind = orphanKindAll
	m.orphans.cursor = 5
	m.orphans.scroll = 3

	updated, _ := m.handleOrphansKey(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, orphanKindPod, updated.orphans.visibleKind)
	assert.Equal(t, 0, updated.orphans.cursor, "kind switch resets cursor")
	assert.Equal(t, 0, updated.orphans.scroll)

	// Tab through every kind and wrap back to All. orphanKindMax is the
	// sentinel after the last real kind, so the cycle length is
	// orphanKindMax (= number of real kinds + 1 for All).
	for range int(orphanKindMax) - 1 {
		updated, _ = updated.handleOrphansKey(tea.KeyMsg{Type: tea.KeyTab})
	}
	assert.Equal(t, orphanKindAll, updated.orphans.visibleKind, "Tab wraps back to All")
}

func TestOrphansOverlay_JKMovesCursor(t *testing.T) {
	m := newTestModel()
	m.overlay = overlayOrphans
	m.orphans.report = k8s.OrphanReport{Pods: []k8s.OrphanItem{
		{Kind: "Pod", Name: "a"}, {Kind: "Pod", Name: "b"}, {Kind: "Pod", Name: "c"},
	}}
	m.orphans.visibleKind = orphanKindPod

	updated, _ := m.handleOrphansKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 1, updated.orphans.cursor)
	updated, _ = updated.handleOrphansKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	updated, _ = updated.handleOrphansKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // clamps at len-1
	assert.Equal(t, 2, updated.orphans.cursor)
	updated, _ = updated.handleOrphansKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	assert.Equal(t, 1, updated.orphans.cursor)
}

func TestOrphansOverlay_FilterShrinksList(t *testing.T) {
	m := newTestModel()
	m.overlay = overlayOrphans
	m.orphans.report = k8s.OrphanReport{Pods: []k8s.OrphanItem{
		{Kind: "Pod", Namespace: "default", Name: "naked"},
		{Kind: "Pod", Namespace: "kube-system", Name: "debug"},
	}}
	m.orphans.visibleKind = orphanKindAll

	// Press / to enter filter mode.
	updated, _ := m.handleOrphansKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	assert.True(t, updated.orphans.filterActive)

	// Type "kube" -- should narrow to debug.
	for _, r := range "kube" {
		updated, _ = updated.handleOrphansKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	visible := updated.orphans.visibleItems()
	require.Len(t, visible, 1)
	assert.Equal(t, "debug", visible[0].Name)

	// Esc cancels the filter, clears query, exits filter mode.
	updated, _ = updated.handleOrphansKey(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, updated.orphans.filterActive)
	assert.Equal(t, "", updated.orphans.filter.Value)
}

func TestOrphansOverlay_EnterJumpsToResource(t *testing.T) {
	m := newTestModel()
	m.overlay = overlayOrphans
	m.orphans.report = k8s.OrphanReport{Pods: []k8s.OrphanItem{
		{Kind: "Pod", Namespace: "kube-system", Name: "naked"},
	}}
	m.orphans.visibleKind = orphanKindPod
	m.orphans.cursor = 0
	m.nav.Context = "test"

	// Seed the discovery cache and the resource-types sidebar so
	// jumpToOrphan can resolve "Pod" via FindResourceTypeByKind and
	// position the parent cursor on the Pods row. The earlier
	// executeResourceJump-based implementation didn't need these but
	// also failed silently in real usage when discovery was cold.
	podRT := mdl.ResourceTypeEntry{
		Kind: "Pod", Resource: "pods", APIVersion: "v1",
	}
	m.discoveredResources = map[string][]mdl.ResourceTypeEntry{
		"test": {podRT},
	}
	m.middleItems = []mdl.Item{{Name: "Pods", Extra: podRT.ResourceRef()}}
	m.nav.Level = mdl.LevelResourceTypes

	updated, _ := m.handleOrphansKey(tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, overlayNone, updated.overlay, "overlay should close on jump")
	assert.Equal(t, "kube-system", updated.namespace)
	assert.Equal(t, "naked", updated.pendingTarget,
		"pendingTarget must be set so the cursor lands on the orphan after the resource list loads")
}

// TestOrphansOverlay_EnterShowsErrorWhenDiscoveryCold verifies the new
// failure path: when the user presses Enter but discovery hasn't
// completed, instead of silently no-op'ing (the old bug), we surface a
// status message so the user knows to retry.
func TestOrphansOverlay_EnterShowsErrorWhenDiscoveryCold(t *testing.T) {
	m := newTestModel()
	m.overlay = overlayOrphans
	m.orphans.report = k8s.OrphanReport{Pods: []k8s.OrphanItem{
		{Kind: "Pod", Namespace: "kube-system", Name: "naked"},
	}}
	m.orphans.visibleKind = orphanKindPod
	m.orphans.cursor = 0
	m.nav.Context = "test"
	// discoveredResources intentionally empty.

	updated, _ := m.handleOrphansKey(tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, overlayOrphans, updated.overlay,
		"overlay should stay open so the user can retry after discovery completes")
	assert.Contains(t, updated.statusMessage, "not yet discovered")
}

// TestOrphansOverlay_StrictToggle pins the strict/lenient switch.
// Initial state: strict=true, lenient-only items are hidden. Press `s`:
// strict flips to false, lenient-only items become visible. Press again:
// they're hidden again. Counts and cursor must stay coherent across the
// transition.
func TestOrphansOverlay_StrictToggle(t *testing.T) {
	m := newTestModel()
	m.overlay = overlayOrphans
	m.height = 30
	m.width = 100
	m.orphans.strict = true // explicit (matches NewModel default)
	m.orphans.report = k8s.OrphanReport{Secrets: []k8s.OrphanItem{
		{Kind: "Secret", Namespace: "ns", Name: "truly-orphan"}, // strict
		{Kind: "Secret", Namespace: "ns", Name: "cron-creds", LenientOnly: true},
		{Kind: "Secret", Namespace: "ns", Name: "deploy-cfg", LenientOnly: true},
	}}
	m.orphans.visibleKind = orphanKindSecret

	// Strict mode: only the truly unreferenced one is visible.
	visible := m.orphans.visibleItems()
	require.Len(t, visible, 1)
	assert.Equal(t, "truly-orphan", visible[0].Name)
	assert.Equal(t, 1, m.orphans.orphanKindCount(orphanKindSecret),
		"chip count must reflect strict-mode visibility")

	// Press s → toggle to lenient.
	updated, _ := m.handleOrphansKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	assert.False(t, updated.orphans.strict)
	visible = updated.orphans.visibleItems()
	assert.Len(t, visible, 3, "lenient mode shows strict + lenient-only items")
	assert.Equal(t, 3, updated.orphans.orphanKindCount(orphanKindSecret))

	// Press s again → back to strict.
	updated, _ = updated.handleOrphansKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	assert.True(t, updated.orphans.strict)
	visible = updated.orphans.visibleItems()
	assert.Len(t, visible, 1)
}

// TestOrphansOverlay_StrictToggleClampsCursor verifies that toggling
// from lenient back to strict shrinks the visible list, so a cursor
// that was on a now-hidden lenient-only row gets pulled back into
// range. Without orphansClampCursor here the cursor would render past
// the end and (worse) the move handlers' viewport math would break.
func TestOrphansOverlay_StrictToggleClampsCursor(t *testing.T) {
	m := newTestModel()
	m.overlay = overlayOrphans
	m.height = 30
	m.width = 100
	m.orphans.strict = false // start in lenient
	m.orphans.report = k8s.OrphanReport{Secrets: []k8s.OrphanItem{
		{Kind: "Secret", Namespace: "ns", Name: "truly-orphan"},
		{Kind: "Secret", Namespace: "ns", Name: "cron-a", LenientOnly: true},
		{Kind: "Secret", Namespace: "ns", Name: "cron-b", LenientOnly: true},
	}}
	m.orphans.visibleKind = orphanKindSecret
	m.orphans.cursor = 2 // on the second lenient-only item

	updated, _ := m.handleOrphansKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})

	assert.True(t, updated.orphans.strict)
	assert.Equal(t, 0, updated.orphans.cursor,
		"cursor pulled into range — only one strict orphan remains")
}

func TestOrphansOverlay_RefreshInvalidatesCache(t *testing.T) {
	m := newTestModel()
	m.overlay = overlayOrphans
	m.nav.Context = "test"
	key := orphanCacheKey{kubeContext: "test", namespace: ""}
	m.orphanCache[key] = &k8s.OrphanReport{Pods: []k8s.OrphanItem{{Name: "a"}}}

	updated, cmd := m.handleOrphansKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})

	assert.NotContains(t, updated.orphanCache, key, "R clears cluster-wide cache")
	assert.True(t, updated.orphans.loading)
	assert.NotNil(t, cmd)
}

// TestOrphansOverlay_ScrollFollowsCursor verifies the headline regression
// from the broken Phase-2 implementation: cursor movement past the
// viewport edge MUST update scroll so the cursor stays visible. Without
// this, j/G/Ctrl+d move the cursor invisibly past the bottom.
func TestOrphansOverlay_ScrollFollowsCursor(t *testing.T) {
	m := newTestModel()
	m.overlay = overlayOrphans
	m.height = 30 // bodyHeight ≈ 30-2-5 = 23 in the no-banner/no-search case
	m.width = 100
	pods := make([]k8s.OrphanItem, 0, 100)
	for i := range 100 {
		pods = append(pods, k8s.OrphanItem{Kind: "Pod", Namespace: "ns", Name: fmt.Sprintf("p%03d", i)})
	}
	m.orphans.report = k8s.OrphanReport{Pods: pods}
	m.orphans.visibleKind = orphanKindPod

	bodyH := m.orphansVisibleLines()
	require.Greater(t, bodyH, 0)
	require.Less(t, bodyH, len(pods), "test fixture must have more rows than fit")

	// G jumps to last item; scroll must follow so cursor is visible.
	updated, _ := m.handleOrphansKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	assert.Equal(t, len(pods)-1, updated.orphans.cursor, "G jumps to last")
	assert.GreaterOrEqual(t, updated.orphans.cursor, updated.orphans.scroll, "cursor at or below scroll top")
	assert.Less(t, updated.orphans.cursor, updated.orphans.scroll+bodyH, "cursor within viewport")

	// g jumps to first; scroll resets.
	updated, _ = updated.handleOrphansKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	assert.Equal(t, 0, updated.orphans.cursor)
	assert.Equal(t, 0, updated.orphans.scroll)

	// Ctrl+d advances by half-viewport; cursor must remain visible.
	updated, _ = updated.handleOrphansKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	assert.GreaterOrEqual(t, updated.orphans.cursor, updated.orphans.scroll)
	assert.Less(t, updated.orphans.cursor, updated.orphans.scroll+bodyH)
}

// TestOrphansOverlay_RenderHeightStable asserts that the rendered string
// has exactly height-2 lines regardless of how many items are in the
// report. This is what keeps the OverlayStyle frame from growing as the
// list grows.
func TestOrphansOverlay_RenderHeightStable(t *testing.T) {
	m := newTestModel()
	m.height = 30
	m.width = 100

	for _, n := range []int{0, 5, 50, 500} {
		pods := make([]k8s.OrphanItem, 0, n)
		for i := range n {
			pods = append(pods, k8s.OrphanItem{Kind: "Pod", Namespace: "ns", Name: fmt.Sprintf("p%03d", i)})
		}
		m.orphans.report = k8s.OrphanReport{Pods: pods}
		m.orphans.visibleKind = orphanKindPod
		body, _, h := m.renderOrphansOverlay()
		gotLines := strings.Count(body, "\n") + 1
		assert.Equal(t, h-2, gotLines, "n=%d: expected %d content lines, got %d", n, h-2, gotLines)
	}
}

// TestOrphansOverlay_LastVisibleRowNotClipped is the regression test for
// the off-by-2 bodyHeight bug. With chrome=5 (the original buggy value),
// orphansVisibleLines() overestimated capacity by 2, so the cursor could
// move into rows that PadToHeight then clipped from the rendered output —
// the user would press j past the apparent bottom and the cursor would
// disappear. With chrome=7 (correct), every row the move handler thinks
// is visible MUST appear in the rendered string.
//
// We assert by rendering with cursor at every visible position and
// checking the row's name shows up in the output.
func TestOrphansOverlay_LastVisibleRowNotClipped(t *testing.T) {
	m := newTestModel()
	m.height = 30
	m.width = 100
	pods := make([]k8s.OrphanItem, 0, 100)
	for i := range 100 {
		pods = append(pods, k8s.OrphanItem{Kind: "Pod", Namespace: "ns", Name: fmt.Sprintf("pod-%03d", i)})
	}
	m.orphans.report = k8s.OrphanReport{Pods: pods}
	m.orphans.visibleKind = orphanKindPod

	bodyH := m.orphansVisibleLines()
	require.Greater(t, bodyH, 0)

	// Walk j from cursor=0 to the last visible row before scroll kicks
	// in (cursor=bodyH-1, scroll still 0). Every cursor position must
	// produce a body that contains the cursor's row.
	for i := range bodyH {
		body, _, _ := m.renderOrphansOverlay()
		want := fmt.Sprintf("pod-%03d", i)
		assert.Contains(t, body, want,
			"cursor=%d (bodyH=%d): expected row %q to be visible", i, bodyH, want)
		m, _ = m.handleOrphansKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}
}
