package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// newClusterPickerModel returns a Model parked at the cluster picker with
// two contexts and a temp XDG state dir so save() doesn't clobber the
// developer's real cluster-colors file.
func newClusterPickerModel(t *testing.T) Model {
	t.Helper()
	withClusterColorsStateDir(t)
	return Model{
		nav: model.NavigationState{Level: model.LevelClusters},
		middleItems: []model.Item{
			{Name: "prod-eu"},
			{Name: "dev-local"},
		},
		cursors:           [5]int{0, 0, 0, 0, 0},
		tabs:              []TabState{{}},
		itemCache:         map[string][]model.Item{},
		cacheFingerprints: map[string]string{},
		clusterColors:     map[string]string{},
		width:             80, height: 40,
	}
}

func TestHandleKeyClusterColorPicker_AtClusterPickerOpensOverlay(t *testing.T) {
	m := newClusterPickerModel(t)
	ret, _ := m.handleKeyClusterColorPicker()
	result := ret.(Model)
	assert.Equal(t, overlayClusterColor, result.overlay, "Ctrl+L at Level=Clusters opens the picker overlay")
	assert.Equal(t, "prod-eu", result.clusterColorOverlayContext, "overlay captures the highlighted context name")
}

// TestHandleKey_ShiftL_RoutesToClusterColorPicker is the regression
// test for the bug the user hit with Ctrl+L / Ctrl+K: every Ctrl+letter
// we tried got intercepted by either the terminal emulator (Ctrl+L =
// redraw screen) or the shell input layer (Ctrl+K = kill-to-eol in
// readline/ZLE). Capital "L" works because the picker only exists at
// Level=Clusters where Logs has no resource to act on — the dispatch
// case is gated on Level=Clusters and breaks out at deeper levels so
// "L" continues to open Logs everywhere else.
func TestHandleKey_ShiftL_RoutesToClusterColorPicker(t *testing.T) {
	m := newClusterPickerModel(t)
	ret, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	result := ret.(Model)
	assert.Equal(t, overlayClusterColor, result.overlay,
		"Shift+L at the cluster picker must route through handleKey to the cluster-color overlay")
	assert.Equal(t, "prod-eu", result.clusterColorOverlayContext)
}

// TestHandleKey_ShiftL_FallsThroughAboveClusterPicker ensures the
// dispatch's level gate lets "L" keep its Logs binding at deeper
// levels — pressing "L" on a Pod must reach the Logs handler instead
// of being eaten by the (gated) cluster-colour case.
func TestHandleKey_ShiftL_FallsThroughAboveClusterPicker(t *testing.T) {
	m := newClusterPickerModel(t)
	m.nav.Level = model.LevelResources
	m.nav.Context = "prod-eu"
	before := m.overlay
	ret, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	result := ret.(Model)
	assert.Equal(t, before, result.overlay,
		"above the cluster picker, Shift+L must fall through the cluster-colour case so the existing Logs handler still fires")
}

func TestHandleKeyClusterColorPicker_AbovePickerLevelIsNoOp(t *testing.T) {
	m := newClusterPickerModel(t)
	m.nav.Level = model.LevelResources
	m.nav.Context = "prod-eu"
	ret, _ := m.handleKeyClusterColorPicker()
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay, "outside the cluster picker the hotkey is a no-op")
}

func TestHandleKeyClusterColorPicker_NoSelectionIsNoOp(t *testing.T) {
	m := newClusterPickerModel(t)
	m.middleItems = nil
	ret, _ := m.handleKeyClusterColorPicker()
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay, "with no row to target, the overlay must not open")
}

func TestHandleKeyClusterColorPicker_PreseedsCursorToCurrentColor(t *testing.T) {
	m := newClusterPickerModel(t)
	// "yellow" is index 1 in ui.ClusterColorNames.
	m.clusterColors = map[string]string{"prod-eu": "yellow"}
	ret, _ := m.handleKeyClusterColorPicker()
	result := ret.(Model)
	wantIdx := -1
	for i, c := range ui.ClusterColorNames {
		if c == "yellow" {
			wantIdx = i
		}
	}
	require.GreaterOrEqual(t, wantIdx, 0)
	assert.Equal(t, wantIdx, result.clusterColorOverlayCursor,
		"opening on a coloured cluster pre-seeds the picker cursor to its current color")
}

func TestHandleKeyClusterColorPicker_PreseedsCursorToNoneWhenUnset(t *testing.T) {
	m := newClusterPickerModel(t)
	ret, _ := m.handleKeyClusterColorPicker()
	result := ret.(Model)
	// "None / clear" lives at the end of the picker, after every named color.
	assert.Equal(t, len(ui.ClusterColorNames), result.clusterColorOverlayCursor,
		"opening on a non-coloured cluster pre-seeds cursor on the None / clear row")
}

func TestClusterColorOverlay_DownArrowMovesCursor(t *testing.T) {
	m := newClusterPickerModel(t)
	ret, _ := m.handleKeyClusterColorPicker()
	m = ret.(Model)
	start := m.clusterColorOverlayCursor

	ret, _ = m.handleClusterColorOverlayKey("down")
	result := ret.(Model)
	expected := (start + 1) % (len(ui.ClusterColorNames) + 1)
	assert.Equal(t, expected, result.clusterColorOverlayCursor, "down arrow advances the cursor (wraps at end)")
}

func TestClusterColorOverlay_UpArrowMovesCursor(t *testing.T) {
	m := newClusterPickerModel(t)
	ret, _ := m.handleKeyClusterColorPicker()
	m = ret.(Model)
	m.clusterColorOverlayCursor = 0

	ret, _ = m.handleClusterColorOverlayKey("up")
	result := ret.(Model)
	assert.Equal(t, len(ui.ClusterColorNames), result.clusterColorOverlayCursor,
		"up arrow at top wraps to the bottom (None row)")
}

func TestClusterColorOverlay_EnterAppliesColorAndCloses(t *testing.T) {
	m := newClusterPickerModel(t)
	ret, _ := m.handleKeyClusterColorPicker()
	m = ret.(Model)
	m.clusterColorOverlayCursor = 0 // first color = "red"

	ret, _ = m.handleClusterColorOverlayKey("enter")
	result := ret.(Model)
	assert.Equal(t, "red", result.clusterColors["prod-eu"], "Enter writes the selected color to the in-memory map")
	assert.Equal(t, overlayNone, result.overlay, "Enter closes the overlay")

	// Disk too — the file lookup hits the temp XDG dir set in
	// newClusterPickerModel via withClusterColorsStateDir.
	persisted := loadClusterColors()
	assert.Equal(t, "red", persisted["prod-eu"], "Enter persists the selection so it survives restart")
}

func TestClusterColorOverlay_NoneRowClearsAndPersists(t *testing.T) {
	m := newClusterPickerModel(t)
	m.clusterColors = map[string]string{"prod-eu": "yellow"}
	require.NoError(t, saveClusterColors(m.clusterColors))
	ret, _ := m.handleKeyClusterColorPicker()
	m = ret.(Model)
	m.clusterColorOverlayCursor = len(ui.ClusterColorNames) // "None" row

	ret, _ = m.handleClusterColorOverlayKey("enter")
	result := ret.(Model)
	_, present := result.clusterColors["prod-eu"]
	assert.False(t, present, "selecting None deletes the entry rather than storing an empty string")

	persisted := loadClusterColors()
	_, persistedPresent := persisted["prod-eu"]
	assert.False(t, persistedPresent, "deletion is persisted to the state file")
}

func TestClusterColorOverlay_SlashEntersFilterMode(t *testing.T) {
	m := newClusterPickerModel(t)
	ret, _ := m.handleKeyClusterColorPicker()
	m = ret.(Model)
	require.False(t, m.clusterColorFilterMode, "filter mode is off by default when the overlay opens")

	ret, _ = m.handleClusterColorOverlayKey("/")
	result := ret.(Model)
	assert.True(t, result.clusterColorFilterMode,
		"`/` flips into filter-input mode so the next keystrokes go into the filter buffer")
}

func TestClusterColorOverlay_FilterNarrowsList(t *testing.T) {
	m := newClusterPickerModel(t)
	ret, _ := m.handleKeyClusterColorPicker()
	m = ret.(Model)

	// Type "/", then two keys spelling "ll" — substring match against
	// the names list narrows down to "yellow" only. Single letters
	// match too liberally for an assertable test ("y" matches yellow,
	// cyan, gray) so use a longer disambiguating substring.
	ret, _ = m.handleClusterColorOverlayKey("/")
	m = ret.(Model)
	ret, _ = m.handleClusterColorOverlayKey("l")
	m = ret.(Model)
	ret, _ = m.handleClusterColorOverlayKey("l")
	result := ret.(Model)

	filtered := result.filteredClusterColorNames()
	assert.Equal(t, []string{"yellow"}, filtered,
		"substring filter must narrow the visible colour list to matches against the colour names")
	assert.Equal(t, 0, result.clusterColorOverlayCursor,
		"cursor resets to the first row when the filter changes so the highlight doesn't land on a stale index")
}

func TestClusterColorOverlay_FilterModeEscClearsFilter(t *testing.T) {
	m := newClusterPickerModel(t)
	ret, _ := m.handleKeyClusterColorPicker()
	m = ret.(Model)
	ret, _ = m.handleClusterColorOverlayKey("/")
	m = ret.(Model)
	ret, _ = m.handleClusterColorOverlayKey("y")
	m = ret.(Model)

	ret, _ = m.handleClusterColorOverlayKey("esc")
	result := ret.(Model)
	assert.False(t, result.clusterColorFilterMode, "Esc in filter mode exits filter mode")
	assert.Empty(t, result.clusterColorFilter.Value, "Esc in filter mode clears the buffer")
	assert.Equal(t, overlayClusterColor, result.overlay,
		"Esc in filter mode does NOT close the overlay — only clears the filter")
}

func TestClusterColorOverlay_NormalModeEscClearsFilterFirstThenCloses(t *testing.T) {
	// Mirrors the colorscheme overlay's two-stage Esc: with a filter
	// active, first Esc clears the filter; second Esc closes.
	m := newClusterPickerModel(t)
	ret, _ := m.handleKeyClusterColorPicker()
	m = ret.(Model)
	ret, _ = m.handleClusterColorOverlayKey("/")
	m = ret.(Model)
	ret, _ = m.handleClusterColorOverlayKey("y")
	m = ret.(Model)
	// Exit filter mode with Enter so we're back in normal mode but with
	// the filter buffer still set.
	ret, _ = m.handleClusterColorOverlayKey("enter")
	m = ret.(Model)
	require.NotEmpty(t, m.clusterColorFilter.Value, "filter buffer survives an Enter from filter mode")

	// First Esc in normal mode: clears the filter, leaves overlay open.
	ret, _ = m.handleClusterColorOverlayKey("esc")
	m = ret.(Model)
	assert.Empty(t, m.clusterColorFilter.Value)
	assert.Equal(t, overlayClusterColor, m.overlay)

	// Second Esc: closes.
	ret, _ = m.handleClusterColorOverlayKey("esc")
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
}

func TestClusterColorOverlay_FilterAffectsApply(t *testing.T) {
	// With the filter narrowed to "yellow", cursor=0 must apply
	// "yellow", not "red" (which would be index 0 in the unfiltered
	// list). Catches the off-by-everything bug where Apply would read
	// from the unfiltered slice.
	m := newClusterPickerModel(t)
	ret, _ := m.handleKeyClusterColorPicker()
	m = ret.(Model)
	ret, _ = m.handleClusterColorOverlayKey("/")
	m = ret.(Model)
	ret, _ = m.handleClusterColorOverlayKey("y")
	m = ret.(Model)
	// Exit filter mode but keep the filter active.
	ret, _ = m.handleClusterColorOverlayKey("enter")
	m = ret.(Model)
	// Cursor is at 0 = first filtered match = "yellow".
	require.Equal(t, 0, m.clusterColorOverlayCursor)

	ret, _ = m.handleClusterColorOverlayKey("enter")
	result := ret.(Model)
	assert.Equal(t, "yellow", result.clusterColors["prod-eu"],
		"Apply with a filter active must read from the filtered list, not from ClusterColorNames directly")
}

func TestClusterColorOverlay_EscDoesNotMutate(t *testing.T) {
	m := newClusterPickerModel(t)
	m.clusterColors = map[string]string{"prod-eu": "yellow"}
	require.NoError(t, saveClusterColors(m.clusterColors))
	ret, _ := m.handleKeyClusterColorPicker()
	m = ret.(Model)
	m.clusterColorOverlayCursor = 0 // would have applied "red"

	ret, _ = m.handleClusterColorOverlayKey("esc")
	result := ret.(Model)
	assert.Equal(t, "yellow", result.clusterColors["prod-eu"], "Esc must leave the in-memory state untouched")
	assert.Equal(t, overlayNone, result.overlay, "Esc closes the overlay")

	persisted := loadClusterColors()
	assert.Equal(t, "yellow", persisted["prod-eu"], "Esc must not persist the would-be-applied color")
}
