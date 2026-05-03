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

// TestHandleKey_CtrlK_RoutesToClusterColorPicker is the regression test
// for the bug the user hit when the default was Ctrl+L: most terminals
// intercept Ctrl+L for "redraw screen" before the app sees the key, so
// the binding moved to Ctrl+K which is terminal-safe across macOS
// Terminal, iTerm2, Alacritty, Ghostty, and gnome-terminal. Going
// through tea.KeyCtrlK (not the test-helper string fallback) catches
// dispatch wiring failures the direct-handler tests miss.
func TestHandleKey_CtrlK_RoutesToClusterColorPicker(t *testing.T) {
	m := newClusterPickerModel(t)
	ret, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlK})
	result := ret.(Model)
	assert.Equal(t, overlayClusterColor, result.overlay,
		"Ctrl+K at the cluster picker must route through handleKey to the cluster-color overlay")
	assert.Equal(t, "prod-eu", result.clusterColorOverlayContext)
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
