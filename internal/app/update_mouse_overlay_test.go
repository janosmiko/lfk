package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// actionOverlayModel returns a model with the action menu open and a few
// items so click-on-item resolution has something to point at. Geometry
// (width=120, height=40) and tabs ([]TabState{{}}) are inherited from
// baseExplorerModel so the overlay box lands at a known position:
//
//	overlayAction is rendered with ow=70, oh=15. With OverlayStyle's
//	1-char border on each side the visual outer box is 72x17, centered
//	at box.x = (120-72)/2 = 24, box.y = (40-17)/2 = 11. Inner content
//	(after border + 1-row top padding + 1-row title + 1 blank padding
//	row) starts on screen at y = 11 + 4 = 15, so item 0 is at y=15,
//	item 1 at y=16, etc.
func actionOverlayModel(itemCount int) Model {
	m := baseExplorerModel()
	m.overlay = overlayAction
	items := make([]model.Item, itemCount)
	for i := range items {
		items[i] = model.Item{Name: "act-" + string(rune('a'+i)), Status: ""}
	}
	m.overlayItems = items
	m.overlayCursor = 0
	return m
}

// --- click-outside dismisses any centered overlay ---

func TestOverlayMouseClickOutsideDismissesActionMenu(t *testing.T) {
	m := actionOverlayModel(3)

	// Click well to the left of the box (box.x = 24).
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      2,
		Y:      20,
	})
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay,
		"click outside the action overlay box must dismiss it")
}

func TestOverlayMouseClickAboveBoxDismisses(t *testing.T) {
	m := actionOverlayModel(3)

	// Click well above the box (box.y = 11).
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      60,
		Y:      2,
	})
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
}

func TestOverlayMouseRightClickOutsideAlsoDismisses(t *testing.T) {
	m := actionOverlayModel(3)

	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
		X:      2,
		Y:      20,
	})
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay,
		"right-click outside the box should dismiss the same way left-click does")
}

func TestOverlayMouseClickOutsideDismissesNamespaceOverlay(t *testing.T) {
	m := baseExplorerModel()
	m.overlay = overlayNamespace
	m.overlayItems = []model.Item{{Name: "default"}, {Name: "kube-system"}}

	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      2, // outside the centered overlay
		Y:      2,
	})
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay,
		"click-outside dismiss should work uniformly for any centered overlay")
}

// --- click inside action menu activates the row ---

func TestOverlayMouseClickActionMenuItemSetsCursorAndActivates(t *testing.T) {
	m := actionOverlayModel(3)

	// box.y = 11; first item at y = box.y + 4 = 15. Click row 1 (item idx 1).
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      50, // inside the box (x in [24,96))
		Y:      16,
	})
	result := ret.(Model)
	// executeAction sets overlay = overlayNone as its first step, so a
	// successful activation drops the overlay.
	assert.Equal(t, overlayNone, result.overlay,
		"click on an action row should activate it (executeAction clears the overlay)")
	assert.Equal(t, 1, result.overlayCursor,
		"cursor must move to the clicked item before activation so per-action context is consistent")
}

func TestOverlayMouseClickActionMenuTitleRowIsNoOp(t *testing.T) {
	m := actionOverlayModel(3)

	// Title row is the first inner row, screen y = box.y + 2 = 13.
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      50,
		Y:      13,
	})
	result := ret.(Model)
	assert.Equal(t, overlayAction, result.overlay,
		"clicking the title row must not dismiss or activate anything")
	assert.Equal(t, 0, result.overlayCursor,
		"clicking the title row must not move the cursor")
}

func TestOverlayMouseClickActionMenuPaddingRowIsNoOp(t *testing.T) {
	m := actionOverlayModel(2)

	// Inner content has title (row 0), blank (row 1), 2 items (rows 2,3),
	// then trailing blank rows up to inner H. Click a trailing blank row.
	// box.y=11, inner items run y=15..16, blanks at y=17..27.
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      50,
		Y:      22, // empty padding row inside the box
	})
	result := ret.(Model)
	assert.Equal(t, overlayAction, result.overlay,
		"clicking a blank padding row inside the box must not dismiss the overlay")
	assert.Equal(t, 0, result.overlayCursor,
		"clicking blank padding must not change the cursor")
}

func TestOverlayMouseClickActionMenuBorderIsNoOp(t *testing.T) {
	m := actionOverlayModel(3)

	// Top-left corner of the box is the border at (box.x, box.y) = (24, 11).
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      24,
		Y:      11,
	})
	result := ret.(Model)
	assert.Equal(t, overlayAction, result.overlay,
		"clicking the border ring must be treated as inside-but-non-interactive, not dismiss")
}

func TestOverlayMouseRightClickInsideActionMenuIsNoOp(t *testing.T) {
	m := actionOverlayModel(3)

	// Right-click on item 0 (y = box.y + 4 = 15). We treat right-click
	// inside the overlay as a no-op so users can right-click on the
	// underlying explorer only after they dismiss the overlay.
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
		X:      50,
		Y:      15,
	})
	result := ret.(Model)
	assert.Equal(t, overlayAction, result.overlay,
		"right-click inside the box must not activate")
	assert.Equal(t, 0, result.overlayCursor)
}

// --- non-press / non-button events are ignored ---

func TestOverlayMouseReleaseIgnored(t *testing.T) {
	m := actionOverlayModel(3)

	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
		X:      2, // would be outside, but release should still be ignored
		Y:      2,
	})
	result := ret.(Model)
	assert.Equal(t, overlayAction, result.overlay,
		"release events must not dismiss the overlay")
}

func TestOverlayMouseWheelDownAdvancesOverlayCursor(t *testing.T) {
	m := actionOverlayModel(5)

	// Wheel down dispatches 3 "down" keys, so the action menu cursor
	// should advance from 0 toward 3 (clamped to len-1).
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonWheelDown,
		Action: tea.MouseActionPress,
		X:      50,
		Y:      15,
	})
	result := ret.(Model)
	assert.Equal(t, overlayAction, result.overlay,
		"wheel scroll must not dismiss the overlay")
	assert.Equal(t, 3, result.overlayCursor,
		"wheel down should move the overlay cursor 3 rows")
	assert.Equal(t, 0, result.cursor(),
		"wheel must not leak to the explorer cursor underneath")
}

func TestOverlayMouseWheelUpMovesOverlayCursorBack(t *testing.T) {
	m := actionOverlayModel(5)
	m.overlayCursor = 4

	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonWheelUp,
		Action: tea.MouseActionPress,
		X:      50,
		Y:      15,
	})
	result := ret.(Model)
	assert.Equal(t, 1, result.overlayCursor,
		"wheel up should move the overlay cursor 3 rows back from 4")
}

// --- fullscreen overlays have no bounding box ---

func TestOverlayMouseFullscreenOverlayClickIgnored(t *testing.T) {
	m := baseExplorerModel()
	m.overlay = overlaySecretEditor // fullscreen renderer, no centered box

	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      2,
		Y:      2,
	})
	result := ret.(Model)
	// Fullscreen overlays cover the whole screen — there is no "outside"
	// to click. Clicks must be ignored so they don't accidentally close
	// the editor and lose unsaved input.
	assert.Equal(t, overlaySecretEditor, result.overlay)
}

// --- centered box geometry sanity check ---

func TestCenteredOverlayBoxMatchesPlaceOverlayMath(t *testing.T) {
	m := actionOverlayModel(3)
	box, ok := m.centeredOverlayBox()
	assert.True(t, ok)
	// width=120, ow=min(70, 120-10)=70, visualW=72, x=(120-72)/2=24.
	assert.Equal(t, 24, box.x)
	assert.Equal(t, 72, box.w)
	// height=40, oh=min(15, 40-6)=15, visualH=17, y=(40-17)/2=11.
	assert.Equal(t, 11, box.y)
	assert.Equal(t, 17, box.h)
}

func TestCenteredOverlayBoxNoneWhenNoOverlay(t *testing.T) {
	m := baseExplorerModel()
	_, ok := m.centeredOverlayBox()
	assert.False(t, ok)
}

// At narrow heights the namespace overlay's full content (title + filter
// + items + scroll indicators) is taller than the requested
// min(20, m.height-6). Lipgloss expands the rendered box to fit instead
// of truncating, so a naive `visualH = oh + 2` formula puts box.y one
// row too low and clicks land on the row above what the user sees. This
// regression test pins the box geometry against PlaceOverlay's actual
// math across a range of terminal sizes.
func TestCenteredOverlayBoxMatchesActualRenderAtNarrowHeights(t *testing.T) {
	tests := []struct {
		name string
		w, h int
	}{
		{"narrow_height_overflows_oh", 40, 20},
		{"medium", 80, 30},
		{"large", 120, 40},
		{"very_large", 200, 60},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := baseExplorerModel()
			m.width = tc.w
			m.height = tc.h
			m.overlay = overlayNamespace
			// Enough items to force the full layout (title + filter +
			// scroll indicators + items + trailing blank).
			m.overlayItems = []model.Item{
				{Name: "All Namespaces", Status: "all"},
				{Name: "default"},
				{Name: "kube-system"},
				{Name: "monitoring"},
				{Name: "argocd"},
				{Name: "prod"},
				{Name: "staging"},
			}
			m.overlayCursor = 0
			ui.ResetOverlayNsScroll()

			box, ok := m.centeredOverlayBox()
			assert.True(t, ok)

			// Render the same way view_overlays.go does and measure the
			// actual rendered dimensions.
			content, ow, oh, _ := m.renderOverlayContent()
			rendered := ui.OverlayStyle.Width(ow).Height(oh).Render(content)
			lines := strings.Split(rendered, "\n")
			actualH := len(lines)
			actualW := lipgloss.Width(lines[0])
			expectedY := max((m.height-actualH)/2, 0)

			assert.Equal(t, actualW, box.w, "box.w must match rendered width")
			assert.Equal(t, actualH, box.h, "box.h must match rendered height")
			assert.Equal(t, expectedY, box.y, "box.y must match PlaceOverlay's centering")
		})
	}
}

// --- click on the namespace badge in the title bar ---

func TestTitleBarClickOnNamespaceBadgeOpensSelector(t *testing.T) {
	m := baseExplorerModel()
	// Render the title bar so activeTitleBarLayout gets populated with
	// the live ns badge x-range (the click handler reads it back).
	_ = m.renderTitleBar()
	r := getTitleBarLayout()
	if r.nsEndX <= r.nsStartX {
		t.Fatalf("title bar layout did not record an ns region: %+v", r)
	}

	clickX := (r.nsStartX + r.nsEndX) / 2 // dead center of the badge
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      clickX,
		Y:      0,
	})
	result := ret.(Model)
	assert.Equal(t, overlayNamespace, result.overlay,
		"click on the ns badge in the title bar should open the namespace selector")
}

func TestTitleBarClickOutsideNamespaceBadgeIsNoOp(t *testing.T) {
	m := baseExplorerModel()
	_ = m.renderTitleBar()

	// Click far left on the title bar — outside the ns badge range.
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      2,
		Y:      0,
	})
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay,
		"a title-bar click that doesn't land on a known badge must be a no-op, not fall through to column-header sort")
	assert.Equal(t, model.LevelResources, result.nav.Level,
		"y=0 must not drill out via the left-pane navigation path")
}

func TestTitleBarRightClickIsNoOp(t *testing.T) {
	m := baseExplorerModel()
	_ = m.renderTitleBar()
	r := getTitleBarLayout()

	clickX := (r.nsStartX + r.nsEndX) / 2
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
		X:      clickX,
		Y:      0,
	})
	result := ret.(Model)
	// Right-click on the title bar shouldn't open anything; in
	// particular, it must not open the action menu via the right-pane
	// fallback (since the namespace badge sits in the rightmost band).
	assert.Equal(t, overlayNone, result.overlay)
}

// --- click-to-activate on namespace overlay ---

func TestOverlayMouseClickNamespaceItemSetsCursorAndApplies(t *testing.T) {
	m := baseExplorerModel()
	m.overlay = overlayNamespace
	m.overlayItems = []model.Item{
		{Name: "All Namespaces", Status: "all"},
		{Name: "default"},
		{Name: "kube-system"},
		{Name: "monitoring"},
	}
	m.overlayCursor = 0
	ui.ResetOverlayNsScroll()

	// Namespace overlay layout (RenderNamespaceOverlay):
	//   inner rows: 0=title, 1=title-pad, 2=filter, 3=blank,
	//               4=scroll-above, 5..=items.
	// box.y for namespace overlay (ow=60, oh=20 -> visualW=62, visualH=22):
	//   box.y = (40 - 22) / 2 = 9
	// Inner row 0 = box.y + 2 = 11.
	// Item 1 ("default") is at inner row 6, screen y = 11 + 6 = 17.
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      60, // inside the namespace overlay box
		Y:      17, // item index 1 ("default")
	})
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay,
		"clicking a namespace row should apply the selection and close the overlay")
	assert.Equal(t, "default", result.namespace,
		"clicking the 'default' row should switch the active namespace to 'default'")
}

func TestOverlayMouseClickNamespaceFilterRowIsNoOp(t *testing.T) {
	m := baseExplorerModel()
	m.overlay = overlayNamespace
	m.overlayItems = []model.Item{{Name: "default"}}
	m.overlayCursor = 0
	ui.ResetOverlayNsScroll()

	// Filter row is inner row 2, screen y = 9 + 2 + 2 = 13.
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      60,
		Y:      13,
	})
	result := ret.(Model)
	assert.Equal(t, overlayNamespace, result.overlay,
		"clicking the filter row must not activate or dismiss the overlay")
}
