package app

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/logagg"
	"github.com/janosmiko/lfk/internal/ui"
)

func newLogTopModel(t *testing.T) Model {
	t.Helper()
	m := basePush80Model()
	m.mode = modeLogTop
	m.logView.rawLines = []string{
		`2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/a","DownstreamStatus":200}`,
		`2026-06-18T10:00:01Z {"RequestMethod":"GET","RequestPath":"/a","DownstreamStatus":500}`,
		`2026-06-18T10:00:02Z {"RequestMethod":"POST","RequestPath":"/b","DownstreamStatus":200}`,
	}
	m.logTopResetAndParse()
	return m
}

func TestLogTopKey_NavigateAndDrill(t *testing.T) {
	m := newLogTopModel(t)
	// move down
	mdl, _ := m.handleLogTopKey(key("j"))
	m = mdl.(Model)
	if m.logTop.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.logTop.cursor)
	}
	// drill into the selected group
	mdl, _ = m.handleLogTopKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = mdl.(Model)
	if len(m.logTop.drillStack) == 0 {
		t.Error("expected drill frame on stack after enter")
	}
}

func TestLogTopKey_EscReturnsToLogs(t *testing.T) {
	m := newLogTopModel(t)
	mdl, _ := m.handleLogTopKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mdl.(Model)
	if m.mode != modeLogs {
		t.Errorf("mode after esc = %v, want modeLogs", m.mode)
	}
}

// TestLogTopKey_EscRebuildsLogView verifies that returning from Log Top via esc
// immediately populates the log viewer projection (logView.lines) from rawLines,
// so the user does not see a blank screen on return.
func TestLogTopKey_EscRebuildsLogView(t *testing.T) {
	m := newLogTopModel(t)
	if len(m.logView.rawLines) == 0 {
		t.Fatal("precondition: rawLines must be non-empty for this test to be meaningful")
	}
	mdl, _ := m.handleLogTopKey(tea.KeyMsg{Type: tea.KeyEsc})
	got := mdl.(Model)
	if got.mode != modeLogs {
		t.Fatalf("mode after esc = %v, want modeLogs", got.mode)
	}
	if len(got.logView.lines) == 0 {
		t.Errorf("logView.lines is empty after esc from Log Top; viewer would appear blank")
	}
}

func TestLogTopKey_GOpensGroupByOverlay(t *testing.T) {
	m := newLogTopModel(t)
	mdl, _ := m.handleLogTopKey(key("g"))
	got := mdl.(Model)
	if got.overlay != overlayLogTopGroupBy {
		t.Fatalf("pressing g: overlay = %v, want overlayLogTopGroupBy", got.overlay)
	}
}

func TestLogTopKey_POpensProfileOverlay(t *testing.T) {
	m := newLogTopModel(t)
	mdl, _ := m.handleLogTopKey(key("p"))
	got := mdl.(Model)
	if got.overlay != overlayLogTopProfile {
		t.Fatalf("pressing p: overlay = %v, want overlayLogTopProfile", got.overlay)
	}
}

// TestLogTopDrill_DescendAndReturn exercises the full drill-down/pop cycle:
// enter descends to status rows pinned to the selected path, esc restores
// the original groupBy and rows, and a second esc returns to log viewer mode.
func TestLogTopDrill_DescendAndReturn(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogTop
	// Two paths; /multi has two different statuses.
	m.logView.rawLines = []string{
		`2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/single","DownstreamStatus":200}`,
		`2026-06-18T10:00:01Z {"RequestMethod":"GET","RequestPath":"/multi","DownstreamStatus":200}`,
		`2026-06-18T10:00:02Z {"RequestMethod":"GET","RequestPath":"/multi","DownstreamStatus":500}`,
	}
	m.logTopResetAndParse()

	// After parse: groupBy should be [method, path]; should have 2 rows.
	if len(m.logTop.groupBy) == 0 {
		t.Fatal("groupBy empty after reset")
	}
	if len(m.logTop.rows) != 2 {
		t.Fatalf("initial rows = %d, want 2", len(m.logTop.rows))
	}

	// Find the /multi row and position cursor on it.
	multiIdx := -1
	for i, r := range m.logTop.rows {
		for _, v := range r.Values {
			if v == "/multi" {
				multiIdx = i
			}
		}
	}
	if multiIdx < 0 {
		t.Fatal("/multi row not found in initial rows")
	}
	m.logTop.cursor = multiIdx

	// Press enter -> drill into /multi.
	mdl, _ := m.handleLogTopKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = mdl.(Model)

	if len(m.logTop.drillStack) != 1 {
		t.Fatalf("drillStack len = %d, want 1", len(m.logTop.drillStack))
	}
	if len(m.logTop.groupBy) != 1 || m.logTop.groupBy[0] != logagg.FieldStatus {
		t.Errorf("groupBy after drill = %v, want [status]", m.logTop.groupBy)
	}
	if len(m.logTop.rows) != 2 {
		t.Errorf("rows after drill = %d, want 2 (/multi has 200 and 500)", len(m.logTop.rows))
	}
	// Verify that only /multi's statuses appear (not /single's 200).
	for _, r := range m.logTop.rows {
		found := false
		for _, v := range r.Values {
			if v == "200" || v == "500" {
				found = true
			}
		}
		if !found {
			t.Errorf("unexpected row values in drill: %v", r.Values)
		}
	}

	// Press esc -> pop frame and restore.
	mdl, _ = m.handleLogTopKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mdl.(Model)

	if len(m.logTop.drillStack) != 0 {
		t.Errorf("drillStack not empty after esc: %v", m.logTop.drillStack)
	}
	// groupBy must be restored to method+path (2 fields).
	if len(m.logTop.groupBy) != 2 {
		t.Errorf("groupBy after esc = %v, want [method path]", m.logTop.groupBy)
	}
	if len(m.logTop.rows) != 2 {
		t.Errorf("rows after esc = %d, want 2 (full path list)", len(m.logTop.rows))
	}
	if m.mode != modeLogTop {
		t.Errorf("mode after first esc = %v, want modeLogTop", m.mode)
	}

	// Press esc again -> return to log viewer.
	mdl, _ = m.handleLogTopKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mdl.(Model)
	if m.mode != modeLogs {
		t.Errorf("mode after second esc = %v, want modeLogs", m.mode)
	}
}

// TestLogTopDrill_NextDimensionSkipsPinned verifies that successive drills
// advance through display dimensions without repeating already-pinned ones.
// With Traefik JSON logs that have method, path, status, and host:
//   - initial groupBy = [method, path]
//   - first drill on a row -> groupBy = [status]  (next unused dim)
//   - second drill on a status row -> groupBy = [host] (status now pinned)
//
// The test also confirms the drillStack breadcrumb has two frames and that
// no dimension is repeated.
func TestLogTopDrill_NextDimensionSkipsPinned(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogTop
	// Traefik JSON with method, path, status, host all present.
	m.logView.rawLines = []string{
		`2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/api","DownstreamStatus":200,"RequestHost":"a.example.com"}`,
		`2026-06-18T10:00:01Z {"RequestMethod":"GET","RequestPath":"/api","DownstreamStatus":500,"RequestHost":"b.example.com"}`,
		`2026-06-18T10:00:02Z {"RequestMethod":"POST","RequestPath":"/api","DownstreamStatus":200,"RequestHost":"a.example.com"}`,
	}
	m.logTopResetAndParse()

	// Confirm displayDims includes at least method, path, status.
	hasDim := func(d string) bool {
		return slices.Contains(m.logTop.displayDims, d)
	}
	if !hasDim(logagg.FieldMethod) || !hasDim(logagg.FieldPath) || !hasDim(logagg.FieldStatus) {
		t.Fatalf("displayDims missing expected dims: %v", m.logTop.displayDims)
	}

	// Drill 1: from groupBy=[method,path] into any row.
	m.logTop.cursor = 0
	mdl, _ := m.handleLogTopKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = mdl.(Model)

	if len(m.logTop.drillStack) != 1 {
		t.Fatalf("after drill 1: drillStack len = %d, want 1", len(m.logTop.drillStack))
	}
	afterDrill1 := m.logTop.groupBy
	if len(afterDrill1) != 1 {
		t.Fatalf("after drill 1: groupBy = %v, want 1 element", afterDrill1)
	}
	// The pinned dims after drill 1 are method and path; next should be status.
	if afterDrill1[0] != logagg.FieldStatus {
		t.Errorf("after drill 1: groupBy[0] = %q, want %q", afterDrill1[0], logagg.FieldStatus)
	}

	// Drill 2: from groupBy=[status] into any row.
	if len(m.logTop.rows) == 0 {
		t.Fatal("no rows after drill 1 to drill into")
	}
	m.logTop.cursor = 0
	mdl, _ = m.handleLogTopKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = mdl.(Model)

	if len(m.logTop.drillStack) != 2 {
		t.Fatalf("after drill 2: drillStack len = %d, want 2", len(m.logTop.drillStack))
	}
	afterDrill2 := m.logTop.groupBy
	if len(afterDrill2) != 1 {
		t.Fatalf("after drill 2: groupBy = %v, want 1 element", afterDrill2)
	}
	// status is now pinned; next should be host (if present) or any other unpinned dim.
	if afterDrill2[0] == logagg.FieldStatus || afterDrill2[0] == logagg.FieldMethod || afterDrill2[0] == logagg.FieldPath {
		t.Errorf("after drill 2: groupBy[0] = %q must not be a previously-pinned dim", afterDrill2[0])
	}

	// Confirm no breadcrumb filter field is repeated.
	seen := map[string]bool{}
	for _, fr := range m.logTop.drillStack {
		for _, flt := range fr.filters {
			if seen[flt.field] {
				t.Errorf("duplicate filter field %q in drillStack", flt.field)
			}
			seen[flt.field] = true
		}
	}
	// The current groupBy dim must not appear in any filter.
	for _, fr := range m.logTop.drillStack {
		for _, flt := range fr.filters {
			if flt.field == afterDrill2[0] {
				t.Errorf("current groupBy dim %q also appears as a drill filter (should be independent)", afterDrill2[0])
			}
		}
	}

	// Breadcrumb should contain filter entries for method+path (frame 1) and status (frame 2).
	var breadcrumb []string
	for _, fr := range m.logTop.drillStack {
		for _, flt := range fr.filters {
			breadcrumb = append(breadcrumb, flt.field+"="+flt.value)
		}
	}
	if !strings.Contains(strings.Join(breadcrumb, " "), logagg.FieldStatus) {
		t.Errorf("breadcrumb %v should contain %q filter from drill 2 frame", breadcrumb, logagg.FieldStatus)
	}
}

// TestLogTopSort_CyclesAllColumns verifies that SortNext/SortPrev cycle through
// all dimension and metric columns, SortFlip toggles ascending/descending, and
// SortReset returns to REQ descending.
func TestLogTopSort_CyclesAllColumns(t *testing.T) {
	m := newLogTopModel(t)

	// Default sortCol after first parse should be REQ (set by logTopRefreshRows).
	if m.logTop.sortCol != logTopMetricREQ {
		t.Fatalf("initial sortCol = %q, want %q", m.logTop.sortCol, logTopMetricREQ)
	}
	if m.logTop.sortAsc {
		t.Error("initial sortAsc should be false (descending)")
	}

	// Collect expected columns: displayDims + metrics.
	expected := m.logTopSortColumns()
	if len(expected) == 0 {
		t.Fatal("logTopSortColumns returned empty")
	}

	// Find starting index of REQ in the column list.
	startIdx := -1
	for i, c := range expected {
		if c == logTopMetricREQ {
			startIdx = i
			break
		}
	}
	if startIdx < 0 {
		t.Fatalf("REQ not in logTopSortColumns: %v", expected)
	}

	// Cycle forward through all columns and verify we visit each one.
	kb := ui.ActiveKeybindings
	cur := m
	for step := 1; step <= len(expected); step++ {
		mdl, _ := cur.handleLogTopKey(key(kb.SortNext))
		cur = mdl.(Model)
		wantIdx := (startIdx + step) % len(expected)
		if cur.logTop.sortCol != expected[wantIdx] {
			t.Errorf("after %d SortNext: sortCol = %q, want %q", step, cur.logTop.sortCol, expected[wantIdx])
		}
	}
	// After full cycle we should be back at REQ.
	if cur.logTop.sortCol != logTopMetricREQ {
		t.Errorf("full cycle: sortCol = %q, want %q", cur.logTop.sortCol, logTopMetricREQ)
	}

	// Test SortPrev goes backward.
	mdl, _ := cur.handleLogTopKey(key(kb.SortPrev))
	cur = mdl.(Model)
	wantPrev := expected[(startIdx-1+len(expected))%len(expected)]
	if cur.logTop.sortCol != wantPrev {
		t.Errorf("SortPrev from REQ: sortCol = %q, want %q", cur.logTop.sortCol, wantPrev)
	}

	// Test SortFlip toggles sortAsc.
	origAsc := cur.logTop.sortAsc
	mdl, _ = cur.handleLogTopKey(key(kb.SortFlip))
	cur = mdl.(Model)
	if cur.logTop.sortAsc == origAsc {
		t.Error("SortFlip did not toggle sortAsc")
	}

	// Test SortReset restores REQ descending.
	cur.logTop.sortCol = logTopMetricERR
	cur.logTop.sortAsc = true
	mdl, _ = cur.handleLogTopKey(key(kb.SortReset))
	cur = mdl.(Model)
	if cur.logTop.sortCol != logTopMetricREQ {
		t.Errorf("SortReset: sortCol = %q, want %q", cur.logTop.sortCol, logTopMetricREQ)
	}
	if cur.logTop.sortAsc {
		t.Error("SortReset: sortAsc should be false after reset")
	}
}
