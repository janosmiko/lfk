package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// snapshotRowTintGlobals restores the tint mode and theme-derived styles after
// each test so mode changes can't leak across tests.
func snapshotRowTintGlobals(t *testing.T) {
	t.Helper()
	prevMode := ConfigRowStatusTint
	prevNoColor := ConfigNoColor
	t.Cleanup(func() {
		ConfigRowStatusTint = prevMode
		ConfigNoColor = prevNoColor
		if prevNoColor {
			applyNoColorTheme()
		} else {
			ApplyTheme(ActiveTheme)
		}
	})
}

func TestRowTintForStatus_ForegroundMode(t *testing.T) {
	snapshotRowTintGlobals(t)
	ConfigRowStatusTint = RowStatusTintForeground

	st, ok := RowTintForStatus("CrashLoopBackOff")
	assert.True(t, ok, "failed status must tint")
	assert.Equal(t, lipgloss.Color(ColorError), st.GetForeground(), "failed rows use the error color")

	st, ok = RowTintForStatus("Pending")
	assert.True(t, ok, "pending status must tint")
	assert.Equal(t, lipgloss.Color(ColorPrimary), st.GetForeground(), "progressing rows use the same color as the Status cell (primary/blue)")

	_, ok = RowTintForStatus("Running")
	assert.False(t, ok, "running rows never tint")
	_, ok = RowTintForStatus("")
	assert.False(t, ok, "blank status never tints")
}

func TestRowTintForStatus_BackgroundMode(t *testing.T) {
	snapshotRowTintGlobals(t)
	ConfigRowStatusTint = RowStatusTintBackground

	st, ok := RowTintForStatus("Failed")
	assert.True(t, ok)
	assert.NotEqual(t, lipgloss.NoColor{}, st.GetBackground(), "background mode sets a row background")
	assert.NotEqual(t, BaseBg, st.GetBackground(), "tint background differs from the base background")

	st, ok = RowTintForStatus("Terminating")
	assert.True(t, ok, "progressing statuses tint in background mode too")
	assert.NotEqual(t, lipgloss.NoColor{}, st.GetBackground())
}

func TestRowTintForStatus_OffMode(t *testing.T) {
	snapshotRowTintGlobals(t)
	ConfigRowStatusTint = RowStatusTintOff

	_, ok := RowTintForStatus("CrashLoopBackOff")
	assert.False(t, ok, "off mode never tints")
}

func TestRowTintForStatus_NoColor(t *testing.T) {
	snapshotRowTintGlobals(t)
	ConfigRowStatusTint = RowStatusTintForeground
	ConfigNoColor = true
	applyNoColorTheme()

	st, ok := RowTintForStatus("CrashLoopBackOff")
	assert.True(t, ok)
	assert.True(t, st.GetBold(), "no_color degrades failed tint to bold")

	st, ok = RowTintForStatus("Pending")
	assert.True(t, ok)
	assert.True(t, st.GetItalic(), "no_color degrades progressing tint to italic")
}

// withHiddenColumns forces the given built-in columns hidden for one test.
func withHiddenColumns(t *testing.T, cols ...string) {
	t.Helper()
	prev := ActiveHiddenBuiltinColumns
	m := make(map[string]bool, len(cols))
	for _, c := range cols {
		m[c] = true
	}
	ActiveHiddenBuiltinColumns = m
	t.Cleanup(func() { ActiveHiddenBuiltinColumns = prev })
}

func rowTintTestItems() []model.Item {
	return []model.Item{
		{Name: "pod-a", Namespace: "ns1", Kind: "Pod", Status: "Running", Age: "1d"},
		{Name: "pod-b", Namespace: "ns1", Kind: "Pod", Status: "CrashLoopBackOff", Age: "2d"},
	}
}

// TestRenderTable_ForegroundNoTintWhenStatusVisible: foreground mode with the
// Status column shown must render exactly like off mode — the Status cell
// already carries the color, so no whole-row or name tint is added (issue #540
// UAT refinement).
func TestRenderTable_ForegroundNoTintWhenStatusVisible(t *testing.T) {
	snapshotRowTintGlobals(t)

	ConfigRowStatusTint = RowStatusTintOff
	off := NewTableRenderer()
	_ = off.Render("NAME", rowTintTestItems(), 0, 80, 20, false, "", "", 0, 0)

	ConfigRowStatusTint = RowStatusTintForeground
	fg := NewTableRenderer()
	_ = fg.Render("NAME", rowTintTestItems(), 0, 80, 20, false, "", "", 0, 0)

	if off.rows[1] != fg.rows[1] {
		t.Fatalf("foreground mode with Status visible must not change the row.\noff=%q\nfg =%q", off.rows[1], fg.rows[1])
	}
}

// TestRenderTable_ForegroundNameOnlyWhenStatusHidden: when the Status column is
// hidden but Name is shown, the failed row tints only the Name cell — the row
// carries the severity code (on the name) but does NOT wrap the whole line.
func TestRenderTable_ForegroundNameOnlyWhenStatusHidden(t *testing.T) {
	snapshotRowTintGlobals(t)
	withHiddenColumns(t, "Status")
	ConfigRowStatusTint = RowStatusTintForeground

	r := NewTableRenderer()
	_ = r.Render("NAME", rowTintTestItems(), 0, 80, 20, false, "", "", 0, 0)

	failedRow := r.rows[1]
	if !strings.Contains(failedRow, styleOpenCodes(RowTintFailedFg)) {
		t.Fatalf("name-only tint must color the name with the severity code; got %q", failedRow)
	}
	if strings.HasPrefix(failedRow, styleOpenCodes(RowTintFailedFg)) {
		t.Fatalf("name-only tint must NOT wrap the whole row; got %q", failedRow)
	}
}

// TestRenderTable_ForegroundBothHiddenFallsBackToWholeRow: with both Status and
// Name hidden there is no name cell to carry the tint, so foreground mode falls
// back to a whole-row wrap so the signal is not lost.
func TestRenderTable_ForegroundBothHiddenFallsBackToWholeRow(t *testing.T) {
	snapshotRowTintGlobals(t)
	withHiddenColumns(t, "Status", "Name")
	ConfigRowStatusTint = RowStatusTintForeground

	r := NewTableRenderer()
	_ = r.Render("NAME", rowTintTestItems(), 0, 80, 20, false, "", "", 0, 0)

	if !strings.HasPrefix(r.rows[1], styleOpenCodes(RowTintFailedFg)) {
		t.Fatalf("both columns hidden must fall back to a whole-row tint; got %q", r.rows[1])
	}
}

// TestRenderTable_BackgroundIgnoresColumnVisibility: background mode is the
// deliberately loud opt-in and tints the whole row regardless of whether the
// Status column is shown.
func TestRenderTable_BackgroundIgnoresColumnVisibility(t *testing.T) {
	snapshotRowTintGlobals(t)
	withHiddenColumns(t, "Status")
	ConfigRowStatusTint = RowStatusTintBackground

	r := NewTableRenderer()
	_ = r.Render("NAME", rowTintTestItems(), 0, 80, 20, false, "", "", 0, 0)

	if !strings.Contains(r.rows[1], styleOpenCodes(RowTintFailedBg)) {
		t.Fatalf("background mode must tint the row even with Status hidden; got %q", r.rows[1])
	}
}

// TestRenderTable_RowTintOffKeepsCellStyling asserts off mode renders rows
// exactly like today: no row-wide tint codes on a failed row.
func TestRenderTable_RowTintOffKeepsCellStyling(t *testing.T) {
	snapshotRowTintGlobals(t)
	ConfigRowStatusTint = RowStatusTintOff

	items := []model.Item{
		{Name: "pod-a", Namespace: "ns1", Kind: "Pod", Status: "CrashLoopBackOff", Age: "1d"},
		{Name: "pod-b", Namespace: "ns1", Kind: "Pod", Status: "CrashLoopBackOff", Age: "2d"},
	}
	r := NewTableRenderer()
	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)

	row := r.rows[1]
	if strings.HasPrefix(row, styleOpenCodes(RowTintFailedFg)) {
		t.Fatalf("off mode must not row-tint; got %q", row)
	}
}

// TestRenderTable_RowTintBackground asserts background mode lays the blended
// background across the row and pads it to the full pane width.
func TestRenderTable_RowTintBackground(t *testing.T) {
	snapshotRowTintGlobals(t)
	ConfigRowStatusTint = RowStatusTintBackground

	items := []model.Item{
		{Name: "pod-a", Namespace: "ns1", Kind: "Pod", Status: "Running", Age: "1d"},
		{Name: "pod-b", Namespace: "ns1", Kind: "Pod", Status: "Failed", Age: "2d"},
	}
	r := NewTableRenderer()
	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)

	failedRow := r.rows[1]
	if !strings.Contains(failedRow, styleOpenCodes(RowTintFailedBg)) {
		t.Fatalf("failed row must carry the background tint codes; got %q", failedRow)
	}
	if w := lipgloss.Width(failedRow); w < 80 {
		t.Fatalf("background-tinted row must pad to pane width; width=%d", w)
	}
}

// TestRowTintSelectedForeground pins the selected-row treatment: selection owns
// the background, so a tinted cursor row conveys status via the severity
// foreground color instead (issue #540 UAT: selecting a tinted row must not
// erase the status signal).
func TestRowTintSelectedForeground(t *testing.T) {
	snapshotRowTintGlobals(t)

	ConfigRowStatusTint = RowStatusTintForeground
	st, ok := rowTintForeground("CrashLoopBackOff")
	assert.True(t, ok, "failed status tints the selected row")
	assert.Equal(t, lipgloss.Color(ColorError), st.GetForeground(), "failed selected row uses the error color")

	// Background config mode still uses the FOREGROUND treatment when selected,
	// because the selection background owns the background channel.
	ConfigRowStatusTint = RowStatusTintBackground
	st, ok = rowTintForeground("Pending")
	assert.True(t, ok)
	assert.Equal(t, lipgloss.Color(ColorPrimary), st.GetForeground(), "progressing selected row uses the Status-cell color (primary/blue)")

	ConfigRowStatusTint = RowStatusTintOff
	_, ok = rowTintForeground("CrashLoopBackOff")
	assert.False(t, ok, "off mode never tints the selected row")

	ConfigRowStatusTint = RowStatusTintForeground
	_, ok = rowTintForeground("Running")
	assert.False(t, ok, "healthy selected row is not tinted")
}

// TestRenderTable_BackgroundSelectedBlendsCursor asserts that in background mode
// the cursor row on a failed item uses the status-background-blended-with-cursor
// style (not the plain selection background), with no checkmark when the item is
// not multi-selected (issue #540 UAT: the cursor stays visible on a tinted row,
// the status hue is kept, and the checkmark means multi-selection only).
func TestRenderTable_BackgroundSelectedBlendsCursor(t *testing.T) {
	snapshotRowTintGlobals(t)
	ConfigRowStatusTint = RowStatusTintBackground

	items := []model.Item{
		{Name: "pod-a", Namespace: "ns1", Kind: "Pod", Status: "OOMKilled", Age: "1d"},
		{Name: "pod-b", Namespace: "ns1", Kind: "Pod", Status: "Running", Age: "2d"},
	}
	r := NewTableRenderer()
	// cursor on row 0 (the failed pod).
	out := r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)

	if !strings.Contains(out, styleOpenCodes(RowTintFailedCursorBg)) {
		t.Fatal("cursor failed row must use the status-blended-with-cursor background")
	}
	if strings.Contains(out, styleOpenCodes(SelectedStyle)) {
		t.Fatal("cursor failed row must NOT swap in the plain selection background")
	}
	// A blended cursor background must differ from both the plain status tint
	// and the plain selection background.
	if RowTintFailedCursorBg.GetBackground() == RowTintFailedBg.GetBackground() {
		t.Fatal("cursor blend background must differ from the non-cursor status background")
	}
	if RowTintFailedCursorBg.GetBackground() == SelectedStyle.GetBackground() {
		t.Fatal("cursor blend background must differ from the plain selection background")
	}
	// The cursor is not multi-selected here, so no checkmark should appear —
	// the checkmark is reserved for actual selection (issue #540 UAT).
	if strings.Contains(out, strings.TrimSpace(selectionMarker)) {
		t.Fatal("cursor row that is not multi-selected must not show the checkmark")
	}
}

// TestCursorKeptBackgroundTint_NoColorFallsBack pins that no-color background
// mode does NOT use the cursor-blend background (there is no color to blend);
// the cursor instead falls back to the reverse-video selection path.
func TestCursorKeptBackgroundTint_NoColorFallsBack(t *testing.T) {
	snapshotRowTintGlobals(t)
	ConfigRowStatusTint = RowStatusTintBackground

	ConfigNoColor = true
	applyNoColorTheme()
	if _, ok := cursorKeptBackgroundTint("CrashLoopBackOff"); ok {
		t.Fatal("no-color background mode must not use the cursor-blend background")
	}

	ConfigNoColor = false
	ApplyTheme(ActiveTheme)
	if _, ok := cursorKeptBackgroundTint("CrashLoopBackOff"); !ok {
		t.Fatal("color background mode must use the cursor-blend background")
	}
}

// TestRenderTable_BackgroundNoColorCursorDistinct asserts the no-color cursor
// stays distinguishable from a non-cursor failed row: the cursor falls back to
// the reverse-video selection, which adds underline for a failed row (bold is
// already the selection weight), while non-cursor failed rows are only bold.
func TestRenderTable_BackgroundNoColorCursorDistinct(t *testing.T) {
	snapshotRowTintGlobals(t)
	ConfigRowStatusTint = RowStatusTintBackground
	ConfigNoColor = true
	applyNoColorTheme()

	items := []model.Item{
		{Name: "pod-a", Namespace: "ns1", Kind: "Pod", Status: "OOMKilled", Age: "1d"},
		{Name: "pod-b", Namespace: "ns1", Kind: "Pod", Status: "OOMKilled", Age: "2d"},
	}
	r := NewTableRenderer()
	out := r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)

	if !strings.Contains(out, "\x1b[4m") {
		t.Fatal("no-color background cursor must add underline so it stays distinct from a non-cursor failed row")
	}
}

// TestTintedSelectionMarker_CarriesBackground pins that the multi-select
// checkmark carries the row's status background (UAT: no default-bg gap behind
// the checkmark on a tinted row).
func TestTintedSelectionMarker_CarriesBackground(t *testing.T) {
	snapshotRowTintGlobals(t)
	ConfigRowStatusTint = RowStatusTintBackground

	out := tintedSelectionMarker(RowTintFailedBg)
	if !strings.Contains(out, "48;5;") && !strings.Contains(out, "48;2;") {
		t.Fatalf("checkmark must set a background (48;5;... or 48;2;...); got %q", out)
	}
}

// TestRenderTable_ForegroundSelectedIsPlainSelection asserts that in foreground
// mode the focused (cursor) row is the plain selection highlight with no
// severity tint, even on a failed row with the Status column hidden. The tint
// on the focused row was either invisible (severity color == selection
// background) or flooded the whole bar, so foreground mode now leaves the
// cursor row untinted (issue #540).
func TestRenderTable_ForegroundSelectedIsPlainSelection(t *testing.T) {
	snapshotRowTintGlobals(t)
	withHiddenColumns(t, "Status")

	items := []model.Item{
		{Name: "pod-a", Namespace: "ns1", Kind: "Pod", Status: "CrashLoopBackOff", Age: "1d"},
		{Name: "pod-b", Namespace: "ns1", Kind: "Pod", Status: "Running", Age: "2d"},
	}

	// The focused failed row must render identically in off mode and foreground
	// mode: foreground adds no tint to the cursor row.
	ConfigRowStatusTint = RowStatusTintOff
	off := NewTableRenderer()
	offOut := off.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)

	ConfigRowStatusTint = RowStatusTintForeground
	fg := NewTableRenderer()
	fgOut := fg.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)

	cursorLine := func(out string) string {
		for line := range strings.SplitSeq(out, "\n") {
			if strings.Contains(stripANSI(line), "pod-a") {
				return line
			}
		}
		return ""
	}
	offLine, fgLine := cursorLine(offOut), cursorLine(fgOut)
	if fgLine == "" {
		t.Fatal("expected to find pod-a in the rendered output")
	}
	if offLine != fgLine {
		t.Fatalf("foreground mode must leave the focused row as the plain selection.\noff=%q\nfg =%q", offLine, fgLine)
	}
	// And it must not carry the severity color on the foreground channel.
	if strings.Contains(fgLine, styleOpenCodes(SelectedStyle.Foreground(lipgloss.Color(ColorError)))) {
		t.Fatal("foreground-mode focused row must not carry the severity tint")
	}
}

// TestMergeRowTintIntoSelected_NoColorFailedStaysDistinct guards the no-color
// cursor row: the selection style is already bold, so a failed row's bold cue
// must fall back to underline or a selected failed row would look identical to
// a selected healthy one (reviewer HIGH on the selection UAT fix).
func TestMergeRowTintIntoSelected_NoColorFailedStaysDistinct(t *testing.T) {
	snapshotRowTintGlobals(t)
	ConfigRowStatusTint = RowStatusTintForeground
	ConfigNoColor = true
	applyNoColorTheme()

	failed, ok := rowTintForeground("CrashLoopBackOff")
	if !ok {
		t.Fatal("failed status should tint")
	}
	merged := mergeRowTintIntoSelected(SelectedStyle, failed)
	if !merged.GetUnderline() {
		t.Fatal("no-color selected failed row must add underline (bold is taken by the selection style)")
	}
	if merged.GetUnderline() == SelectedStyle.GetUnderline() && !SelectedStyle.GetUnderline() {
		// sanity: the merge actually changed something vs plain selection
		t.Fatal("merge did not distinguish the failed selected row")
	}

	prog, _ := rowTintForeground("Pending")
	if !mergeRowTintIntoSelected(SelectedStyle, prog).GetItalic() {
		t.Fatal("no-color selected progressing row must add italic")
	}
}

// TestRowTintMatchesStatusCellColor locks the invariant that the foreground
// row tint uses the same color as the Status cell for that status, so the two
// signals never disagree (UAT: PodInitializing's cell is blue but the tint was
// orange).
func TestRowTintMatchesStatusCellColor(t *testing.T) {
	snapshotRowTintGlobals(t)
	ConfigRowStatusTint = RowStatusTintForeground

	for _, status := range []string{"CrashLoopBackOff", "Failed", "PodInitializing", "Pending", "Terminating"} {
		tint, ok := RowTintForStatus(status)
		if !ok {
			t.Fatalf("%q should tint", status)
		}
		cell := StatusStyle(status)
		if tint.GetForeground() != cell.GetForeground() {
			t.Fatalf("row tint fg for %q (%v) must equal the Status cell fg (%v)",
				status, tint.GetForeground(), cell.GetForeground())
		}
	}
}

// TestRenderTable_MultiSelectedTintedRowKeepsBg guards the non-cursor
// whole-row tint path: a multi-selected failed row must re-assert the tint
// after the checkmark so its background survives past the marker even with no
// cluster tile to restore it.
func TestRenderTable_MultiSelectedTintedRowKeepsBg(t *testing.T) {
	snapshotRowTintGlobals(t)
	ConfigRowStatusTint = RowStatusTintBackground

	items := []model.Item{
		{Name: "pod-a", Namespace: "ns1", Kind: "Pod", Status: "Running", Age: "1d"},
		{Name: "pod-b", Namespace: "ns1", Kind: "Pod", Status: "OOMKilled", Age: "2d"},
	}
	prev := ActiveSelectedItems
	ActiveSelectedItems = map[string]bool{items[1].SelectionKey(): true}
	t.Cleanup(func() { ActiveSelectedItems = prev })

	r := NewTableRenderer()
	// cursor on row 0 so row 1 goes through the non-cursor (multi-selected) path.
	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)

	row := r.rows[1]
	mark := strings.Index(row, strings.TrimSpace(selectionMarker))
	if mark < 0 {
		t.Fatal("multi-selected row must show the checkmark")
	}
	if !strings.Contains(row[mark:], styleOpenCodes(RowTintFailedBg)) {
		t.Fatalf("tint must be re-asserted after the checkmark; got %q", row)
	}
}

// TestBlendHexToward pins the blend helper: 0 returns the base, 1 returns the
// tint, and midpoints move each channel linearly.
func TestBlendHexToward(t *testing.T) {
	assert.Equal(t, "#000000", blendHexToward("#000000", "#ffffff", 0))
	assert.Equal(t, "#ffffff", blendHexToward("#000000", "#ffffff", 1))
	assert.Equal(t, "#7f7f7f", blendHexToward("#000000", "#ffffff", 0.5))
	// Unparsable inputs fall back to the tint color unchanged.
	assert.Equal(t, "#ff0000", blendHexToward("not-a-color", "#ff0000", 0.2))
}
