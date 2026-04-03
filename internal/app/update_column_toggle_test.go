package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestCovColumnToggleOpenClose(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}
	m.columnToggleItems = []columnToggleEntry{
		{key: "IP", visible: true},
		{key: "Port", visible: false},
	}
	m.overlay = overlayColumnToggle

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Equal(t, overlayNone, r.(Model).overlay)
	assert.Nil(t, r.(Model).columnToggleItems)
}

func TestCovColumnToggleCloseWithFilter(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{{key: "IP", visible: true}}
	m.columnToggleFilter = "IP"
	m.overlay = overlayColumnToggle

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyEscape})
	// First esc clears filter.
	assert.Empty(t, r.(Model).columnToggleFilter)
	assert.Equal(t, overlayColumnToggle, r.(Model).overlay)
}

func TestCovColumnToggleNav(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "a", visible: true},
		{key: "b", visible: true},
		{key: "c", visible: false},
	}
	m.columnToggleCursor = 0

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, r.(Model).columnToggleCursor)

	m2 := r.(Model)
	r, _ = m2.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, r.(Model).columnToggleCursor)
}

func TestCovColumnTogglePageScroll(t *testing.T) {
	items := make([]columnToggleEntry, 30)
	for i := range items {
		items[i] = columnToggleEntry{key: "col", visible: true}
	}
	m := baseModelCov()
	m.columnToggleItems = items

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	assert.Greater(t, r.(Model).columnToggleCursor, 0)

	m.columnToggleCursor = 20
	r, _ = m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	assert.Less(t, r.(Model).columnToggleCursor, 20)

	m.columnToggleCursor = 0
	r, _ = m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	assert.Greater(t, r.(Model).columnToggleCursor, 0)

	m.columnToggleCursor = 25
	r, _ = m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	assert.Less(t, r.(Model).columnToggleCursor, 25)
}

func TestCovColumnToggleSpace(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "IP", visible: true},
		{key: "Port", visible: false},
	}
	m.columnToggleCursor = 0

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	// Toggle visibility of first item, cursor advances.
	assert.False(t, r.(Model).columnToggleItems[0].visible)
	assert.Equal(t, 1, r.(Model).columnToggleCursor)
}

func TestCovColumnToggleMoveUpDown(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "a", visible: true},
		{key: "b", visible: true},
		{key: "c", visible: true},
	}

	m.columnToggleCursor = 0
	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	assert.Equal(t, "b", r.(Model).columnToggleItems[0].key)
	assert.Equal(t, "a", r.(Model).columnToggleItems[1].key)

	m2 := r.(Model)
	m2.columnToggleCursor = 2
	r, _ = m2.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	assert.Equal(t, 1, r.(Model).columnToggleCursor)
}

func TestCovColumnToggleMoveWithFilter(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{{key: "a"}, {key: "b"}}
	m.columnToggleFilter = "active"

	// Move operations are no-op when filtering.
	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	assert.Equal(t, m.columnToggleItems, r.(Model).columnToggleItems)
}

func TestCovColumnToggleEnter(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "IP", visible: true},
		{key: "Port", visible: false},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.overlay = overlayColumnToggle

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, overlayNone, r.(Model).overlay)
	assert.Equal(t, []string{"IP"}, r.(Model).sessionColumns["pod"])
}

func TestCovColumnToggleEnterAllHidden(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "IP", visible: false},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.sessionColumns = map[string][]string{"pod": {"old"}}

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyEnter})
	_, exists := r.(Model).sessionColumns["pod"]
	assert.False(t, exists)
}

func TestCovColumnToggleSlash(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{{key: "a"}}
	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	assert.True(t, r.(Model).columnToggleFilterActive)
}

func TestCovColumnToggleReset(t *testing.T) {
	m := baseModelCov()
	m.sessionColumns = map[string][]string{"pod": {"IP"}}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.columnToggleItems = []columnToggleEntry{{key: "IP"}}

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	assert.Equal(t, overlayNone, r.(Model).overlay)
	_, exists := r.(Model).sessionColumns["pod"]
	assert.False(t, exists)
}

func TestCovColumnToggleFilterKey(t *testing.T) {
	m := baseModelCov()
	m.columnToggleFilterActive = true
	m.columnToggleFilter = ""

	// Type a character.
	r, _ := m.handleColumnToggleFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.Equal(t, "a", r.(Model).columnToggleFilter)

	// Backspace.
	m2 := r.(Model)
	r, _ = m2.handleColumnToggleFilterKey(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Empty(t, r.(Model).columnToggleFilter)

	// Enter.
	m.columnToggleFilterActive = true
	r, _ = m.handleColumnToggleFilterKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, r.(Model).columnToggleFilterActive)

	// Esc with filter text: clears.
	m.columnToggleFilter = "text"
	r, _ = m.handleColumnToggleFilterKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Empty(t, r.(Model).columnToggleFilter)

	// Esc without filter text: exits filter mode.
	m.columnToggleFilter = ""
	r, _ = m.handleColumnToggleFilterKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.False(t, r.(Model).columnToggleFilterActive)

	// Ctrl+W.
	m.columnToggleFilter = "hello world"
	r, _ = m.handleColumnToggleFilterKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	assert.Equal(t, "hello ", r.(Model).columnToggleFilter)
}

func TestCovFilteredColumnToggleItems(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "IP", visible: true},
		{key: "Port", visible: false},
		{key: "Image", visible: true},
	}

	// No filter: all items.
	assert.Len(t, m.filteredColumnToggleItems(), 3)

	// With filter.
	m.columnToggleFilter = "I"
	filtered := m.filteredColumnToggleItems()
	assert.GreaterOrEqual(t, len(filtered), 1)
}

func TestCovOpenColumnToggle(t *testing.T) {
	m := baseModelNav()
	m.middleItems = []model.Item{
		{Name: "pod-1", Columns: []model.KeyValue{{Key: "IP", Value: "10.0.0.1"}, {Key: "Node", Value: "node-1"}}},
	}
	m.openColumnToggle()
	assert.Equal(t, overlayColumnToggle, m.overlay)
}

func TestCovOpenColumnToggleEmpty(t *testing.T) {
	m := baseModelNav()
	m.middleItems = []model.Item{{
		Name: "item",
		Columns: []model.KeyValue{
			{Key: "IP", Value: "10.0.0.1"},
			{Key: "Node", Value: "node-1"},
		},
	}}
	m.openColumnToggle()
	assert.Equal(t, overlayColumnToggle, m.overlay)
}
