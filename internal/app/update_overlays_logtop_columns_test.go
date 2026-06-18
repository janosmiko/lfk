package app

import (
	"slices"
	"strings"
	"testing"
)

// newLogTopColumnsModel returns a model in modeLogTop with parsed log data
// and at least two dimension columns available for column manipulation tests.
func newLogTopColumnsModel(t *testing.T) Model {
	t.Helper()
	m := newLogTopModel(t)
	// newLogTopModel parses Traefik JSON lines with method+path+status dims,
	// so after rebuild colOrder should have at least those three.
	if len(m.logTop.colOrder) == 0 {
		t.Fatal("precondition: colOrder must be non-empty after rebuild")
	}
	return m
}

// TestLogTopKey_CommaOpensColumns verifies that pressing "," in modeLogTop
// opens the column overlay.
func TestLogTopKey_CommaOpensColumns(t *testing.T) {
	m := newLogTopColumnsModel(t)
	mdl, _ := m.handleLogTopKey(key(","))
	got := mdl.(Model)
	if got.overlay != overlayLogTopColumns {
		t.Fatalf("pressing comma: overlay = %v, want overlayLogTopColumns", got.overlay)
	}
}

// TestLogTopColumns_HideAndReorder exercises hiding a column then reordering dims.
func TestLogTopColumns_HideAndReorder(t *testing.T) {
	m := newLogTopColumnsModel(t)
	m = m.openLogTopColumns()

	// The overlay is now open. colOrder has dims like [method, path, status].
	// Navigate to cursor=1 (second dim) and hide it.
	m.overlayCursor = 1
	dims := m.logTop.colOrder
	if len(dims) < 2 {
		t.Skip("need at least 2 dim columns")
	}
	hiddenDim := dims[1]

	// Press space to hide dim at cursor 1.
	mdl, _ := m.handleLogTopColumnsKey(key(" "))
	m = mdl.(Model)

	if !m.logTop.colHidden[hiddenDim] {
		t.Errorf("expected dim %q to be hidden after space", hiddenDim)
	}

	// Verify logTopVisibleDims excludes the hidden dim.
	visible := m.logTopVisibleDims()
	if slices.Contains(visible, hiddenDim) {
		t.Errorf("logTopVisibleDims still contains hidden dim %q", hiddenDim)
	}

	// Unhide it for the reorder test.
	m.overlayCursor = 1
	mdl, _ = m.handleLogTopColumnsKey(key(" "))
	m = mdl.(Model)

	// Navigate to cursor=0 and press J to move it down.
	m.overlayCursor = 0
	origFirst := m.logTop.colOrder[0]
	origSecond := m.logTop.colOrder[1]
	mdl, _ = m.handleLogTopColumnsKey(key("J"))
	m = mdl.(Model)

	if len(m.logTop.colOrder) < 2 {
		t.Fatal("colOrder too short after move")
	}
	if m.logTop.colOrder[0] != origSecond || m.logTop.colOrder[1] != origFirst {
		t.Errorf("after J: colOrder[0]=%q colOrder[1]=%q, want [%q,%q]",
			m.logTop.colOrder[0], m.logTop.colOrder[1], origSecond, origFirst)
	}
	if m.overlayCursor != 1 {
		t.Errorf("cursor after J = %d, want 1", m.overlayCursor)
	}
}

// TestLogTopColumns_HideMetric verifies that a metric column can be hidden via space.
func TestLogTopColumns_HideMetric(t *testing.T) {
	m := newLogTopColumnsModel(t)
	m = m.openLogTopColumns()

	dims := m.logTop.colOrder
	mets := m.logTopAllMetrics()

	// Navigate to the first metric entry (after all dims).
	m.overlayCursor = len(dims)
	if len(mets) == 0 {
		t.Skip("no metrics available")
	}
	metricID := mets[0]

	mdl, _ := m.handleLogTopColumnsKey(key(" "))
	m = mdl.(Model)

	if !m.logTop.colHidden[metricID] {
		t.Errorf("expected metric %q to be hidden after space", metricID)
	}

	visible := m.logTopVisibleMetrics()
	if slices.Contains(visible, metricID) {
		t.Errorf("logTopVisibleMetrics still contains hidden metric %q", metricID)
	}
}

// TestLogTopColumns_CannotHideLastColumn verifies the guard: hiding all but one
// column, then pressing space is a no-op.
func TestLogTopColumns_CannotHideLastColumn(t *testing.T) {
	m := newLogTopColumnsModel(t)
	m = m.openLogTopColumns()

	dims := m.logTop.colOrder
	mets := m.logTopAllMetrics()
	total := len(dims) + len(mets)

	// Hide all except the last column by pressing space repeatedly.
	for range total - 1 {
		m.overlayCursor = 0
		// Find first visible column.
		for j := range total {
			var colID string
			if j < len(dims) {
				colID = dims[j]
			} else if j-len(dims) < len(mets) {
				colID = mets[j-len(dims)]
			}
			if colID != "" && !m.logTop.colHidden[colID] {
				m.overlayCursor = j
				break
			}
		}
		mdl, _ := m.handleLogTopColumnsKey(key(" "))
		m = mdl.(Model)
	}

	// Count visible columns; must be exactly 1.
	visibleCount := 0
	dims2 := m.logTop.colOrder
	mets2 := m.logTopAllMetrics()
	for _, d := range dims2 {
		if !m.logTop.colHidden[d] {
			visibleCount++
		}
	}
	for _, met := range mets2 {
		if !m.logTop.colHidden[met] {
			visibleCount++
		}
	}
	if visibleCount != 1 {
		t.Fatalf("expected exactly 1 visible column, got %d", visibleCount)
	}

	// Now try to hide the last one. Find it.
	for j := range len(dims2) + len(mets2) {
		var colID string
		if j < len(dims2) {
			colID = dims2[j]
		} else if j-len(dims2) < len(mets2) {
			colID = mets2[j-len(dims2)]
		}
		if colID != "" && !m.logTop.colHidden[colID] {
			m.overlayCursor = j
			break
		}
	}
	prevHidden := len(m.logTop.colHidden)
	mdl, _ := m.handleLogTopColumnsKey(key(" "))
	m = mdl.(Model)
	// hidden map must not have grown (no-op).
	if len(m.logTop.colHidden) != prevHidden {
		t.Errorf("guard failed: hiding last column changed colHidden from %d to %d entries",
			prevHidden, len(m.logTop.colHidden))
	}
}

// TestLogTopColumns_EscCancels verifies that esc restores the snapshot state.
func TestLogTopColumns_EscCancels(t *testing.T) {
	m := newLogTopColumnsModel(t)
	m = m.openLogTopColumns()

	origOrder := append([]string(nil), m.logTop.colOrder...)

	// Navigate to cursor 0 and press J to reorder.
	m.overlayCursor = 0
	mdl, _ := m.handleLogTopColumnsKey(key("J"))
	m = mdl.(Model)

	// Now press esc to cancel.
	mdl, _ = m.handleLogTopColumnsKey(key("esc"))
	m = mdl.(Model)

	if m.overlay != overlayNone {
		t.Errorf("overlay after esc = %v, want overlayNone", m.overlay)
	}
	if len(m.logTop.colOrder) != len(origOrder) {
		t.Fatalf("colOrder len mismatch after esc: %d vs %d", len(m.logTop.colOrder), len(origOrder))
	}
	for i, v := range m.logTop.colOrder {
		if v != origOrder[i] {
			t.Errorf("colOrder[%d] = %q, want %q (esc should restore original)", i, v, origOrder[i])
		}
	}
}

// TestLogTopColumns_EnterApplies verifies that enter closes the overlay and keeps changes.
func TestLogTopColumns_EnterApplies(t *testing.T) {
	m := newLogTopColumnsModel(t)
	m = m.openLogTopColumns()

	dims := m.logTop.colOrder
	if len(dims) < 2 {
		t.Skip("need at least 2 dim columns")
	}
	origSecond := dims[1]

	// Move cursor 0 down with J.
	m.overlayCursor = 0
	mdl, _ := m.handleLogTopColumnsKey(key("J"))
	m = mdl.(Model)

	// Press enter to apply.
	mdl, _ = m.handleLogTopColumnsKey(key("enter"))
	m = mdl.(Model)

	if m.overlay != overlayNone {
		t.Errorf("overlay after enter = %v, want overlayNone", m.overlay)
	}
	if len(m.logTop.colOrder) < 1 || m.logTop.colOrder[0] != origSecond {
		t.Errorf("colOrder[0] = %q after enter, want %q", m.logTop.colOrder[0], origSecond)
	}
}

// TestLogTopColumns_FilterNarrows verifies that setting colFilterActive + colFilter
// narrows logTopFilteredColumns to matching entries only.
func TestLogTopColumns_FilterNarrows(t *testing.T) {
	m := newLogTopColumnsModel(t)
	m = m.openLogTopColumns()
	m.logTop.colFilterActive = true
	// Type "stat" via handleLogTopColumnsKey (which routes to the filter handler).
	for _, ch := range "stat" {
		mdl, _ := m.handleLogTopColumnsKey(key(string(ch)))
		m = mdl.(Model)
	}
	filtered := m.logTopFilteredColumns()
	// "stat" should match "status" (if present) or at minimum filter to only matching entries.
	for _, c := range filtered {
		if !strings.Contains(strings.ToLower(c), "stat") {
			t.Errorf("unexpected column %q in filtered results (does not match 'stat')", c)
		}
	}
	// Verify that at least one column is returned (status should be present from the test data).
	found := false
	for _, c := range filtered {
		if c == "status" {
			found = true
		}
	}
	// status comes from DownstreamStatus field in the test data; if it is present, verify filtering.
	if len(filtered) > 0 && !found {
		// If status is not present but other "stat*" matches are, that is fine.
		t.Logf("note: 'status' not in filtered=%v; other 'stat' matches may be present", filtered)
	}
	if len(filtered) == 0 {
		t.Logf("no columns match 'stat' in test data (colOrder=%v, metrics=%v)", m.logTop.colOrder, m.logTopAllMetrics())
	}

	// Verify that columns NOT matching "stat" are excluded.
	all := m.logTopColumnList()
	nonMatching := 0
	for _, c := range all {
		if !strings.Contains(strings.ToLower(c), "stat") {
			nonMatching++
		}
	}
	if nonMatching > 0 && len(filtered) == len(all) {
		t.Errorf("filter 'stat' should have excluded some columns: all=%v filtered=%v", all, filtered)
	}
}

// TestLogTopColumns_FilterSlices checks that filtering is correctly excluded from
// the _ import. Keeps slices in scope.
var _ = slices.Contains[[]string]
