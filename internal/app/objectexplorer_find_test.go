package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pressFind dispatches a key to the find overlay handler.
func pressFind(m Model, msg tea.KeyPressMsg) Model {
	mdl, _ := m.handleObjectExplorerFindKey(msg)
	return mdl.(Model)
}

func openFind(t *testing.T) Model {
	t.Helper()
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	m = pressTree(m, key("r")) // 'r' opens the recursive find overlay
	return m
}

func TestObjectExplorerFind_OpensWithAllPaths(t *testing.T) {
	m := openFind(t)
	assert.Equal(t, overlayObjectExplorerFind, m.overlay)
	assert.Equal(t, modeObjectExplorer, m.mode) // overlay sits on top of the tree
	// Seeded with every map-key node (metadata.name, status.phase, steps...).
	require.NotEmpty(t, m.objectExplorerView.findResults)
}

func TestObjectExplorerFind_FilterNarrows(t *testing.T) {
	m := openFind(t)
	m = pressFind(m, key("/")) // focus the filter input
	assert.True(t, m.objectExplorerView.findFilterActive)
	for _, r := range []string{"p", "h", "a", "s", "e"} {
		m = pressFind(m, key(r))
	}
	require.NotEmpty(t, m.objectExplorerView.findResults)
	for _, res := range m.objectExplorerView.findResults {
		assert.Contains(t, res.Segs[len(res.Segs)-1], "phase")
	}
}

func TestObjectExplorerFind_JumpNavigatesTree(t *testing.T) {
	m := openFind(t)
	m = pressFind(m, key("/"))
	for _, r := range []string{"s", "t", "e", "p", "s"} {
		m = pressFind(m, key(r))
	}
	require.NotEmpty(t, m.objectExplorerView.findResults)
	first := m.objectExplorerView.findResults[0].Segs

	m = pressFind(m, tea.KeyPressMsg{Code: tea.KeyEnter}) // exit filter focus
	m = pressFind(m, tea.KeyPressMsg{Code: tea.KeyEnter}) // jump

	assert.Equal(t, overlayNone, m.overlay)
	assert.Equal(t, first[:len(first)-1], m.objectExplorerView.path)
	sel, ok := m.objectExplorerView.selected()
	require.True(t, ok)
	assert.Equal(t, first[len(first)-1], sel.Key)
}

func TestObjectExplorerFind_EscClosesOverlay(t *testing.T) {
	m := openFind(t)
	m = pressFind(m, tea.KeyPressMsg{Code: tea.KeyEsc})
	assert.Equal(t, overlayNone, m.overlay)
	assert.Equal(t, modeObjectExplorer, m.mode)
	assert.Nil(t, m.objectExplorerView.findResults)
}

func TestObjectExplorerFind_EscFromFilterKeepsOverlay(t *testing.T) {
	m := openFind(t)
	m = pressFind(m, key("/"))
	m = pressFind(m, tea.KeyPressMsg{Code: tea.KeyEsc}) // leaves filter, not overlay
	assert.False(t, m.objectExplorerView.findFilterActive)
	assert.Equal(t, overlayObjectExplorerFind, m.overlay)
}

func TestObjectExplorerFind_OverlayRenders(t *testing.T) {
	m := openFind(t)
	c, w, h := m.renderOverlayObjectExplorerFind()
	assert.Contains(t, c, "Find in Chore chore-1")
	assert.Contains(t, c, "matches")      // subtitle count
	assert.NotContains(t, c, "esc close") // hotkeys live in the bottom hint bar, not the overlay
	assert.Positive(t, w)
	assert.Positive(t, h)
}
