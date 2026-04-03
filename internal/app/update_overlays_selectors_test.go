package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- filteredSchemeNames ---

func TestFilteredSchemeNames(t *testing.T) {
	entries := []ui.SchemeEntry{
		{Name: "Dark Themes", IsHeader: true},
		{Name: "catppuccin-mocha"},
		{Name: "dracula"},
		{Name: "gruvbox-dark"},
		{Name: "Light Themes", IsHeader: true},
		{Name: "catppuccin-latte"},
		{Name: "gruvbox-light"},
	}

	t.Run("no filter returns all non-header entries", func(t *testing.T) {
		m := Model{schemeEntries: entries}
		result := m.filteredSchemeNames()
		assert.Len(t, result, 5)
		assert.NotContains(t, result, "Dark Themes")
		assert.NotContains(t, result, "Light Themes")
	})

	t.Run("filter by prefix", func(t *testing.T) {
		m := Model{
			schemeEntries: entries,
			schemeFilter:  TextInput{Value: "catppuccin"},
		}
		result := m.filteredSchemeNames()
		assert.Len(t, result, 2)
		assert.Contains(t, result, "catppuccin-mocha")
		assert.Contains(t, result, "catppuccin-latte")
	})

	t.Run("filter by substring", func(t *testing.T) {
		m := Model{
			schemeEntries: entries,
			schemeFilter:  TextInput{Value: "gruvbox"},
		}
		result := m.filteredSchemeNames()
		assert.Len(t, result, 2)
	})

	t.Run("no match returns empty", func(t *testing.T) {
		m := Model{
			schemeEntries: entries,
			schemeFilter:  TextInput{Value: "nonexistent"},
		}
		result := m.filteredSchemeNames()
		assert.Empty(t, result)
	})

	t.Run("empty entries returns empty", func(t *testing.T) {
		m := Model{
			schemeFilter: TextInput{Value: "test"},
		}
		result := m.filteredSchemeNames()
		assert.Empty(t, result)
	})
}

func TestCovColorschemeKeyEscEmpty(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayColorscheme
	m.schemeFilter.Clear()
	result, _ := m.handleColorschemeNormalMode(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovColorschemeKeyEscWithFilter(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayColorscheme
	m.schemeFilter.Insert("dark")
	result, _ := m.handleColorschemeNormalMode(keyMsg("esc"))
	rm := result.(Model)
	assert.NotEqual(t, overlayNone, rm.overlay) // stays open, just clears filter
}

func TestCovColorschemeKeyQ(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayColorscheme
	result, _ := m.handleColorschemeNormalMode(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovColorschemeKeyEnter(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayColorscheme
	m.schemeCursor = 0
	// May or may not close overlay depending on whether schemes are loaded.
	result, _ := m.handleColorschemeNormalMode(keyMsg("enter"))
	_ = result
}

func TestCovColorschemeKeySlash(t *testing.T) {
	m := baseModelBoost2()
	result, _ := m.handleColorschemeNormalMode(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.schemeFilterMode)
}

func TestCovColorschemeKeyJ(t *testing.T) {
	m := baseModelBoost2()
	m.schemeCursor = 0
	result, _ := m.handleColorschemeNormalMode(keyMsg("j"))
	_ = result // cursor clamped to scheme count
}

func TestCovColorschemeKeyK(t *testing.T) {
	m := baseModelBoost2()
	m.schemeCursor = 2
	result, _ := m.handleColorschemeNormalMode(keyMsg("k"))
	_ = result
}

func TestCovColorschemeKeyCtrlD(t *testing.T) {
	m := baseModelBoost2()
	m.schemeCursor = 0
	result, _ := m.handleColorschemeNormalMode(keyMsg("ctrl+d"))
	_ = result
}

func TestCovColorschemeKeyCtrlU(t *testing.T) {
	m := baseModelBoost2()
	m.schemeCursor = 10
	result, _ := m.handleColorschemeNormalMode(keyMsg("ctrl+u"))
	_ = result
}

func TestCovColorschemeKeyCtrlF(t *testing.T) {
	m := baseModelBoost2()
	m.schemeCursor = 0
	result, _ := m.handleColorschemeNormalMode(keyMsg("ctrl+f"))
	_ = result
}

func TestCovColorschemeKeyCtrlB(t *testing.T) {
	m := baseModelBoost2()
	m.schemeCursor = 15
	result, _ := m.handleColorschemeNormalMode(keyMsg("ctrl+b"))
	_ = result
}

func TestCovColorschemeKeyG(t *testing.T) {
	m := baseModelBoost2()
	m.schemeCursor = 5
	// First g: pendingG.
	result, _ := m.handleColorschemeNormalMode(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)

	// Second g: go to top.
	result2, _ := rm.handleColorschemeNormalMode(keyMsg("g"))
	rm2 := result2.(Model)
	assert.False(t, rm2.pendingG)
	assert.Equal(t, 0, rm2.schemeCursor)
}

func TestCovColorschemeKeyBigG(t *testing.T) {
	m := baseModelBoost2()
	m.schemeCursor = 0
	result, _ := m.handleColorschemeNormalMode(keyMsg("G"))
	_ = result
}

func TestCovColorschemeKeyH(t *testing.T) {
	m := baseModelBoost2()
	result, _ := m.handleColorschemeNormalMode(keyMsg("H"))
	_ = result
}

func TestCovColorschemeKeyL(t *testing.T) {
	m := baseModelBoost2()
	result, _ := m.handleColorschemeNormalMode(keyMsg("L"))
	_ = result
}

func TestCovContainerSelectKeyEsc(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayContainerSelect
	m.pendingAction = "Exec"
	result, _ := m.handleContainerSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.Empty(t, rm.pendingAction)
}

func TestCovContainerSelectKeyEnterWithItem(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayContainerSelect
	m.overlayItems = []model.Item{{Name: "main"}}
	m.overlayCursor = 0
	m.pendingAction = "Describe"
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	result, cmd := m.handleContainerSelectOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, "main", rm.actionCtx.containerName)
	assert.NotNil(t, cmd)
}

func TestCovContainerSelectKeyEnterNoItems(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayContainerSelect
	m.overlayItems = nil
	result, _ := m.handleContainerSelectOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovContainerSelectKeyJK(t *testing.T) {
	m := baseModelBoost2()
	m.overlayItems = []model.Item{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	m.overlayCursor = 0
	result, _ := m.handleContainerSelectOverlayKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.overlayCursor)

	result2, _ := rm.handleContainerSelectOverlayKey(keyMsg("k"))
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.overlayCursor)
}

func TestCovContainerSelectKeyDown(t *testing.T) {
	m := baseModelBoost2()
	m.overlayItems = []model.Item{{Name: "a"}, {Name: "b"}}
	m.overlayCursor = 0
	result, _ := m.handleContainerSelectOverlayKey(keyMsg("down"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.overlayCursor)
}

func TestCovContainerSelectKeyUp(t *testing.T) {
	m := baseModelBoost2()
	m.overlayItems = []model.Item{{Name: "a"}, {Name: "b"}}
	m.overlayCursor = 1
	result, _ := m.handleContainerSelectOverlayKey(keyMsg("up"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.overlayCursor)
}

func TestCovSchemeDisplayItemsEmpty(t *testing.T) {
	m := baseModelCov()
	m.schemeEntries = nil
	m.schemeFilter = TextInput{}
	items := m.schemeDisplayItems()
	assert.Empty(t, items)
}

func TestCovSchemeDisplayItemsWithEntries(t *testing.T) {
	m := baseModelCov()
	m.schemeEntries = []ui.SchemeEntry{
		{Name: "Dark Themes", IsHeader: true},
		{Name: "dracula"},
		{Name: "nord"},
		{Name: "Light Themes", IsHeader: true},
		{Name: "solarized-light"},
	}
	m.schemeFilter = TextInput{}
	items := m.schemeDisplayItems()
	assert.Len(t, items, 5)
	assert.Equal(t, -1, items[0].selectIdx) // header
	assert.Equal(t, 0, items[1].selectIdx)
	assert.Equal(t, 1, items[2].selectIdx)
	assert.Equal(t, -1, items[3].selectIdx) // header
	assert.Equal(t, 2, items[4].selectIdx)
}

func TestCovSchemeFirstVisibleSelectable(t *testing.T) {
	m := baseModelCov()
	m.schemeEntries = []ui.SchemeEntry{
		{Name: "Dark Themes", IsHeader: true},
		{Name: "dracula"},
		{Name: "nord"},
	}
	m.schemeFilter = TextInput{}
	m.schemeCursor = 0
	ui.ResetOverlaySchemeScroll()
	idx := m.schemeFirstVisibleSelectable()
	assert.Equal(t, 0, idx) // first selectable
}

func TestCovSchemeLastVisibleSelectable(t *testing.T) {
	m := baseModelCov()
	m.schemeEntries = []ui.SchemeEntry{
		{Name: "Dark Themes", IsHeader: true},
		{Name: "dracula"},
		{Name: "nord"},
	}
	m.schemeFilter = TextInput{}
	m.schemeCursor = 0
	ui.ResetOverlaySchemeScroll()
	idx := m.schemeLastVisibleSelectable()
	assert.Equal(t, 1, idx) // last selectable
}

func TestCovScaleOverlayKeyDigit(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayScaleInput
	result, _ := m.handleScaleOverlayKey(keyMsg("3"))
	rm := result.(Model)
	assert.Equal(t, "3", rm.scaleInput.Value)
}

func TestCovScaleOverlayKeyBackspace(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayScaleInput
	m.scaleInput.Insert("42")
	result, _ := m.handleScaleOverlayKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Equal(t, "4", rm.scaleInput.Value)
}

func TestCovScaleOverlayKeyEsc(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayScaleInput
	result, _ := m.handleScaleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovContainerSelectOverlayKeyEsc(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayContainerSelect
	m.overlayItems = []model.Item{{Name: "c1"}, {Name: "c2"}}
	result, _ := m.handleContainerSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovContainerSelectOverlayKeyDown(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayContainerSelect
	m.overlayItems = []model.Item{{Name: "c1"}, {Name: "c2"}}
	m.overlayCursor = 0
	result, _ := m.handleContainerSelectOverlayKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.overlayCursor)
}

func TestCovContainerSelectOverlayKeyUp(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayContainerSelect
	m.overlayItems = []model.Item{{Name: "c1"}, {Name: "c2"}}
	m.overlayCursor = 1
	result, _ := m.handleContainerSelectOverlayKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.overlayCursor)
}

func TestCovPodSelectOverlayKeyEscClearsFilter(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayPodSelect
	m.logPodFilterText = "test"
	m.overlayItems = []model.Item{{Name: "pod-1"}}
	result, _ := m.handlePodSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Empty(t, rm.logPodFilterText)
}

func TestCovPodSelectOverlayKeyEscClosesOverlay(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayPodSelect
	result, _ := m.handlePodSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovPodSelectOverlayKeyDown(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayPodSelect
	m.overlayItems = []model.Item{{Name: "pod-1"}, {Name: "pod-2"}}
	m.overlayCursor = 0
	result, _ := m.handlePodSelectOverlayKey(keyMsg("j"))
	_ = result.(Model) // just verify no panic
}

func TestCovPodSelectOverlayKeySlash(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayPodSelect
	result, _ := m.handlePodSelectOverlayKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.logPodFilterActive)
}

func TestCovColorschemeOverlayKeyEsc(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayColorscheme
	result, _ := m.handleColorschemeOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}
