package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
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
	assert.Equal(t, lipgloss.Color(ColorWarning), st.GetForeground(), "progressing rows use the warning color")

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

// TestRenderTable_RowTintForeground asserts a failed non-cursor row renders as
// one uniformly tinted row (tint open codes present) instead of per-cell styles.
func TestRenderTable_RowTintForeground(t *testing.T) {
	setTestColorProfile(t)
	snapshotRowTintGlobals(t)
	ConfigRowStatusTint = RowStatusTintForeground

	items := []model.Item{
		{Name: "pod-a", Namespace: "ns1", Kind: "Pod", Status: "Running", Age: "1d"},
		{Name: "pod-b", Namespace: "ns1", Kind: "Pod", Status: "CrashLoopBackOff", Age: "2d"},
	}
	r := NewTableRenderer()
	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)

	failedRow := r.rows[1]
	if !strings.HasPrefix(failedRow, styleOpenCodes(RowTintFailedFg)) {
		t.Fatalf("failed row must start with the row tint open codes (uniform row); got %q", failedRow)
	}
}

// TestRenderTable_RowTintOffKeepsCellStyling asserts off mode renders rows
// exactly like today: no row-wide tint codes on a failed row.
func TestRenderTable_RowTintOffKeepsCellStyling(t *testing.T) {
	setTestColorProfile(t)
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
	setTestColorProfile(t)
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

// TestBlendHexToward pins the blend helper: 0 returns the base, 1 returns the
// tint, and midpoints move each channel linearly.
func TestBlendHexToward(t *testing.T) {
	assert.Equal(t, "#000000", blendHexToward("#000000", "#ffffff", 0))
	assert.Equal(t, "#ffffff", blendHexToward("#000000", "#ffffff", 1))
	assert.Equal(t, "#7f7f7f", blendHexToward("#000000", "#ffffff", 0.5))
	// Unparsable inputs fall back to the tint color unchanged.
	assert.Equal(t, "#ff0000", blendHexToward("not-a-color", "#ff0000", 0.2))
}
