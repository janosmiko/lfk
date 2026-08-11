package app

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	mdl "github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func undeliverableTestModel(rows int) Model {
	m := newTestModel()
	m.width = 120
	m.height = 30
	m.nav.Context = "test"
	pods := make([]k8s.UndeliverableItem, rows)
	for i := range pods {
		pods[i] = k8s.UndeliverableItem{
			Kind: "Pod", Namespace: "web", Name: "pod-" + string(rune('a'+i%26)),
			Reason: "FailedScheduling: no nodes",
		}
	}
	m.undeliverable.report = k8s.UndeliverableReport{Pods: pods}
	m.undeliverable.loadedFor = "test"
	return m
}

func pressUndeliverable(m Model, key string) Model {
	out, _ := m.handleUndeliverableKey(tea.KeyPressMsg{Text: key, Code: []rune(key)[0]})
	return out
}

func TestOpenUndeliverableOverlay_FiresScanWhenNoReportForContext(t *testing.T) {
	m := newTestModel()
	m.nav.Context = "test"

	updated, cmd := m.openUndeliverableOverlay()

	assert.Equal(t, overlayUndeliverable, updated.overlay)
	assert.True(t, updated.undeliverable.loading)
	require.NotNil(t, cmd)
}

// TestOpenUndeliverableOverlay_ReusesReportForSameContext is why the state
// carries the context it was loaded for: reopening must not re-scan the
// cluster, and must not flash the spinner over rows already on screen.
func TestOpenUndeliverableOverlay_ReusesReportForSameContext(t *testing.T) {
	m := undeliverableTestModel(3)

	updated, cmd := m.openUndeliverableOverlay()

	assert.Nil(t, cmd, "cached report must not trigger a second scan")
	assert.False(t, updated.undeliverable.loading)
}

func TestOpenUndeliverableOverlay_RescansAfterContextSwitch(t *testing.T) {
	m := undeliverableTestModel(3)
	m.nav.Context = "other"

	_, cmd := m.openUndeliverableOverlay()

	require.NotNil(t, cmd, "a different context needs its own scan")
}

func TestCloseUndeliverableOverlay_PreservesResumableState(t *testing.T) {
	m := undeliverableTestModel(20)
	m.overlay = overlayUndeliverable
	m.undeliverable.cursor = 7
	m.undeliverable.scroll = 4
	m.undeliverable.filter.Set("pod")
	m.undeliverable.filterActive = true

	updated := m.closeUndeliverableOverlay()

	assert.Equal(t, overlayNone, updated.overlay)
	assert.Equal(t, 7, updated.undeliverable.cursor, "cursor preserved across close")
	assert.Equal(t, 4, updated.undeliverable.scroll, "scroll preserved across close")
	assert.Equal(t, "pod", updated.undeliverable.filter.Value, "filter query preserved")
	assert.False(t, updated.undeliverable.filterActive, "filterActive forced false")
}

func TestUndeliverableKey_CursorMovementClampsAtBothEnds(t *testing.T) {
	m := undeliverableTestModel(5)

	m = pressUndeliverable(m, "k")
	assert.Equal(t, 0, m.undeliverable.cursor, "k at the top stays at 0")

	for range 10 {
		m = pressUndeliverable(m, "j")
	}
	assert.Equal(t, 4, m.undeliverable.cursor, "j past the end stops on the last row")

	m, _ = m.handleUndeliverableKey(tea.KeyPressMsg{Text: "G", Code: 'G'})
	assert.Equal(t, 4, m.undeliverable.cursor)
	m, _ = m.handleUndeliverableKey(tea.KeyPressMsg{Text: "g", Code: 'g'})
	assert.Equal(t, 0, m.undeliverable.cursor)
}

// TestUndeliverableKey_ScrollKeepsCursorVisible is the invariant every
// movable cursor in this app has to hold: the cursor never leaves the
// rendered window.
func TestUndeliverableKey_ScrollKeepsCursorVisible(t *testing.T) {
	m := undeliverableTestModel(60)
	body := m.undeliverableVisibleLines()

	m, _ = m.handleUndeliverableKey(tea.KeyPressMsg{Text: "G", Code: 'G'})

	assert.GreaterOrEqual(t, m.undeliverable.cursor, m.undeliverable.scroll)
	assert.Less(t, m.undeliverable.cursor, m.undeliverable.scroll+body)
}

func TestUndeliverableKey_FilterNarrowsAndResetsCursor(t *testing.T) {
	m := undeliverableTestModel(26)
	m.undeliverable.cursor = 20

	m = pressUndeliverable(m, "/")
	require.True(t, m.undeliverable.filterActive)

	for _, r := range "pod-c" {
		m, _ = m.handleUndeliverableFilterInput(tea.KeyPressMsg{Text: string(r), Code: r})
	}
	assert.Equal(t, 0, m.undeliverable.cursor, "narrowing the list resets the cursor")
	assert.Len(t, m.undeliverable.visibleRows(), 1)

	m, _ = m.handleUndeliverableFilterInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Empty(t, m.undeliverable.filter.Value, "esc clears the query")
	assert.Len(t, m.undeliverable.visibleRows(), 26)
}

func TestUndeliverableKey_FilterMatchesKindAndReason(t *testing.T) {
	m := undeliverableTestModel(0)
	m.undeliverable.report = k8s.UndeliverableReport{
		Pods:      []k8s.UndeliverableItem{{Kind: "Pod", Name: "a", Reason: "FailedScheduling: x"}},
		Ingresses: []k8s.UndeliverableItem{{Kind: "Ingress", Name: "b", Reason: "no address"}},
	}

	m.undeliverable.filter.Set("failedscheduling")
	assert.Len(t, m.undeliverable.visibleRows(), 1)

	m.undeliverable.filter.Set("ingress")
	require.Len(t, m.undeliverable.visibleRows(), 1)
	assert.Equal(t, "b", m.undeliverable.visibleRows()[0].Name)
}

func TestUndeliverableKey_EscAndQClose(t *testing.T) {
	for _, key := range []string{"q"} {
		m := undeliverableTestModel(3)
		m.overlay = overlayUndeliverable
		assert.Equal(t, overlayNone, pressUndeliverable(m, key).overlay, key)
	}
	m := undeliverableTestModel(3)
	m.overlay = overlayUndeliverable
	out, _ := m.handleUndeliverableKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, overlayNone, out.overlay)
}

// TestHandleUndeliverableLoaded_DropsSupersededScan is the guard against a
// scan finishing after the user moved to another cluster and overwriting
// their current rows with the old cluster's.
func TestHandleUndeliverableLoaded_DropsSupersededScan(t *testing.T) {
	m := undeliverableTestModel(3)
	m.undeliverable.gen = 7

	out, _ := m.handleUndeliverableLoaded(undeliverableLoadedMsg{
		kubeContext: "test", gen: 6,
		report: k8s.UndeliverableReport{Pods: []k8s.UndeliverableItem{{Name: "stale"}}},
	})
	assert.Len(t, out.undeliverable.report.Pods, 3, "stale generation must not write back")

	out, _ = m.handleUndeliverableLoaded(undeliverableLoadedMsg{
		kubeContext: "elsewhere", gen: 7,
		report: k8s.UndeliverableReport{Pods: []k8s.UndeliverableItem{{Name: "stale"}}},
	})
	assert.Len(t, out.undeliverable.report.Pods, 3, "result for another context must not write back")

	out, _ = m.handleUndeliverableLoaded(undeliverableLoadedMsg{
		kubeContext: "test", gen: 7,
		report: k8s.UndeliverableReport{Pods: []k8s.UndeliverableItem{{Name: "fresh"}}},
	})
	require.Len(t, out.undeliverable.report.Pods, 1)
	assert.Equal(t, "fresh", out.undeliverable.report.Pods[0].Name)
}

// TestHandleUndeliverableLoaded_ReleasesInflightSlotForForeignContext is the
// deadlock guard: a scan that lands after the user switched cluster has its
// data dropped, but it must still free the inflight slot or every later scan
// is refused and the overlay never leaves its spinner.
func TestHandleUndeliverableLoaded_ReleasesInflightSlotForForeignContext(t *testing.T) {
	m := undeliverableTestModel(0)
	m.undeliverable.gen = 1
	m.undeliverable.inflight = true
	m.nav.Context = "current"

	out, _ := m.handleUndeliverableLoaded(undeliverableLoadedMsg{kubeContext: "old", gen: 1})
	require.False(t, out.undeliverable.inflight, "inflight slot must be released")

	_, cmd := out.openUndeliverableOverlay()
	assert.NotNil(t, cmd, "a later scan must still be possible")
}

// TestHandleUndeliverableLoaded_SanitizesPartialErrorIntoStatusBar covers the
// scan error's second sink. The overlay subtitle sanitizes it, but with the
// overlay closed the same string goes to the status bar instead, and that
// sink only folds newlines and truncates - it never strips escapes. One field,
// two sinks, and only one of them guarded is the exact shape of the bug
// TASK-873 and TASK-880 chased through sixteen sites.
func TestHandleUndeliverableLoaded_SanitizesPartialErrorIntoStatusBar(t *testing.T) {
	m := undeliverableTestModel(0)
	m.undeliverable.gen = 1
	m.overlay = overlayNone // force the status-bar path, not the subtitle

	hostile := errors.New("listing pvcs: \x1b]52;c;cHduZWQ=\a forbidden \u202e")
	out, cmd := m.handleUndeliverableLoaded(undeliverableLoadedMsg{
		kubeContext: "test", gen: 1, err: hostile,
	})
	require.NotNil(t, cmd, "a partial result still schedules the status clear")

	for _, bad := range []string{"\x1b]", "\a", "\u202e"} {
		assert.NotContains(t, out.statusMessage, bad, "status message leaked an escape")
	}
	// The printable remainder survives, so the message still names the failure.
	assert.Contains(t, out.statusMessage, "listing pvcs")
	assert.Contains(t, out.statusMessage, "forbidden")
}

func TestHandleUndeliverableLoaded_PartialErrorSurfacesInOverlay(t *testing.T) {
	m := undeliverableTestModel(0)
	m.undeliverable.gen = 1
	m.overlay = overlayUndeliverable

	out, _ := m.handleUndeliverableLoaded(undeliverableLoadedMsg{
		kubeContext: "test", gen: 1, err: errors.New("listing pvcs: forbidden"),
	})

	require.Error(t, out.undeliverable.partial)
	body, _, _ := out.renderUndeliverableOverlay()
	assert.Contains(t, body, "partial result")
}

// TestHandleUndeliverableLoaded_ClampsCursorIntoShrunkReport covers the
// background-refresh case: a resource that got unstuck leaves the list, and
// the remembered cursor index no longer exists.
func TestHandleUndeliverableLoaded_ClampsCursorIntoShrunkReport(t *testing.T) {
	m := undeliverableTestModel(20)
	m.undeliverable.gen = 1
	m.undeliverable.cursor = 18

	out, _ := m.handleUndeliverableLoaded(undeliverableLoadedMsg{
		kubeContext: "test", gen: 1,
		report: k8s.UndeliverableReport{Pods: []k8s.UndeliverableItem{{Name: "one"}}},
	})

	assert.Equal(t, 0, out.undeliverable.cursor)
	assert.Equal(t, 0, out.undeliverable.scroll)
}

func TestUndeliverableHintBar_SwitchesForFilterInput(t *testing.T) {
	m := undeliverableTestModel(3)
	m.overlay = overlayUndeliverable

	nav := m.overlayHintBar()
	assert.Contains(t, nav, "jump")
	assert.Contains(t, nav, "refresh")

	m.undeliverable.filterActive = true
	filtering := m.overlayHintBar()
	assert.Contains(t, filtering, "apply")
	assert.NotContains(t, filtering, "jump", "navigation hints hide while typing")
}

// TestUndeliverableSidebarEntry_IsUnderDashboards pins the user-facing
// placement: the entry sits with Cluster and Monitoring, and carries all
// four icon variants so no icon mode renders a blank cell.
func TestUndeliverableSidebarEntry_IsUnderDashboards(t *testing.T) {
	var found *mdl.Item
	for _, it := range mdl.BuildSidebarItems(nil) {
		if it.Extra == mdl.PseudoUndeliverable {
			found = &it
			break
		}
	}
	require.NotNil(t, found, "Undeliverable is missing from the sidebar")
	assert.Equal(t, "Dashboards", found.Category)
	assert.Equal(t, "Undeliverable", found.Name)
	assert.NotEmpty(t, found.Icon.Unicode)
	assert.NotEmpty(t, found.Icon.Simple)
	assert.NotEmpty(t, found.Icon.Emoji)
	assert.NotEmpty(t, found.Icon.NerdFont)
}

// TestUndeliverableSidebarEntry_OpensOverlayOnEnter is the other half of the
// placement: the row is in the Dashboards section but its content is an
// overlay, so Enter must open it instead of navigating into a resource list.
func TestUndeliverableSidebarEntry_OpensOverlayOnEnter(t *testing.T) {
	m := newTestModel()
	m.nav.Context = "test"
	m.nav.Level = mdl.LevelResourceTypes
	sel := mdl.Item{Name: "Undeliverable", Kind: mdl.PseudoUndeliverable, Extra: mdl.PseudoUndeliverable}

	ret, _ := m.navigateChildResourceType(&sel)
	out, ok := ret.(Model)
	require.True(t, ok)
	assert.Equal(t, overlayUndeliverable, out.overlay)
}

// TestUndeliverableKeybinding_DispatchesFromExplorer checks the binding end
// to end. A default that no dispatcher case picks up is the failure mode this
// catches - the overlay would only ever be reachable via the command bar.
func TestUndeliverableKeybinding_DispatchesFromExplorer(t *testing.T) {
	key := ui.DefaultKeybindings().UndeliverableOverlay
	require.Equal(t, "V", key)

	m := newTestModel()
	m.nav.Context = "test"
	ret, _, handled := m.handleExplorerUIKey(tea.KeyPressMsg{Text: key, Code: 'V'})
	require.True(t, handled, "%q was not handled by the explorer dispatcher", key)
	out, ok := ret.(Model)
	require.True(t, ok)
	assert.Equal(t, overlayUndeliverable, out.overlay)
}

func TestUndeliverableCommand_OpensOverlay(t *testing.T) {
	assert.Equal(t, "undeliverable", builtinCommands["undeliverable"])
	assert.Equal(t, "undeliverable", builtinCommands["stuck"], ":stuck is the short alias")

	m := newTestModel()
	m.nav.Context = "test"
	ret, _ := m.executeBuiltinCommand("undeliverable")
	out, ok := ret.(Model)
	require.True(t, ok)
	assert.Equal(t, overlayUndeliverable, out.overlay)
}

// TestUndeliverableOverlay_RowsCarryTheReason is the whole point of the view:
// a row without its reason is not actionable.
func TestUndeliverableOverlay_RowsCarryTheReason(t *testing.T) {
	m := undeliverableTestModel(0)
	m.overlay = overlayUndeliverable
	m.undeliverable.report = k8s.UndeliverableReport{
		PVCs: []k8s.UndeliverableItem{{
			Kind: "PersistentVolumeClaim", Namespace: "data", Name: "cache",
			Reason: "no storage class set: no provisioner, and no PersistentVolume matched",
		}},
	}

	body, _, _ := m.renderUndeliverableOverlay()
	plain := stripANSI(body)
	assert.Contains(t, plain, "cache")
	assert.True(t, strings.Contains(plain, "no storage class set"),
		"reason column missing from the row:\n%s", plain)
}
