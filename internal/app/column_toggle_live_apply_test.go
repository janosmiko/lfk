package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// Live-apply contract: Space, J, K, c each persist their effect to
// sessionColumns / hiddenBuiltinColumns / columnOrder for the current
// kind immediately, so the table behind the overlay updates as the
// user explores. Esc reverts to the snapshot taken at overlay open;
// Enter commits and closes (state is already saved, this is just the
// close).

func newColumnToggleModel() Model {
	m := baseModelCov()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"}
	m.overlay = overlayColumnToggle
	m.columnToggleItems = []columnToggleEntry{
		{key: "Namespace", visible: true, builtin: true},
		{key: "Ready", visible: true, builtin: true},
		{key: "IP", visible: true, builtin: false},
		{key: "Node", visible: false, builtin: false},
	}
	m.columnToggleCursor = 0
	return m
}

func TestColumnToggleSpaceAppliesImmediately(t *testing.T) {
	t.Parallel()
	m := newColumnToggleModel()
	openColumnToggleSnapshot(&m)

	// Space on cursor (Namespace, builtin) should hide it AND persist.
	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	rm := r.(Model)

	assert.Equal(t, overlayColumnToggle, rm.overlay,
		"Space must keep the overlay open")
	assert.False(t, rm.columnToggleItems[0].visible, "overlay state flipped")
	assert.Contains(t, rm.hiddenBuiltinColumns["pod"], "Namespace",
		"Space must persist to hiddenBuiltinColumns immediately so the table updates live")
}

func TestColumnToggleSpaceOnExtraPersistsSession(t *testing.T) {
	t.Parallel()
	m := newColumnToggleModel()
	m.columnToggleCursor = 3 // "Node" (extra, currently hidden)
	openColumnToggleSnapshot(&m)

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	rm := r.(Model)

	assert.True(t, rm.columnToggleItems[3].visible)
	assert.Contains(t, rm.sessionColumns["pod"], "Node",
		"toggling an extra ON must persist to sessionColumns immediately")
}

func TestColumnToggleEscDiscardsChanges(t *testing.T) {
	t.Parallel()
	m := newColumnToggleModel()
	// Pre-existing user choice: Namespace was hidden, IP shown.
	m.hiddenBuiltinColumns = map[string][]string{"pod": {"Namespace"}}
	m.sessionColumns = map[string][]string{"pod": {"IP"}}
	m.columnOrder = map[string][]string{"pod": {"Ready", "IP"}}
	openColumnToggleSnapshot(&m)

	// Mutate while overlay is open.
	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	rm := r.(Model)
	// c clears all visible — should persist (live-apply).
	assert.NotEqual(t, []string{"IP"}, rm.sessionColumns["pod"],
		"clear must mutate state live")

	// Esc: revert to the snapshot.
	r2, _ := rm.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyEscape})
	rm2 := r2.(Model)

	assert.Equal(t, overlayNone, rm2.overlay)
	assert.Equal(t, []string{"Namespace"}, rm2.hiddenBuiltinColumns["pod"],
		"Esc must restore hiddenBuiltinColumns to the snapshot")
	assert.Equal(t, []string{"IP"}, rm2.sessionColumns["pod"],
		"Esc must restore sessionColumns to the snapshot")
	assert.Equal(t, []string{"Ready", "IP"}, rm2.columnOrder["pod"],
		"Esc must restore columnOrder to the snapshot")
}

func TestColumnToggleEnterClosesWithSavedState(t *testing.T) {
	t.Parallel()
	m := newColumnToggleModel()
	openColumnToggleSnapshot(&m)

	// Toggle off Namespace.
	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	rm := r.(Model)
	assert.Contains(t, rm.hiddenBuiltinColumns["pod"], "Namespace")

	// Enter closes; saved state is preserved.
	r2, _ := rm.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm2 := r2.(Model)

	assert.Equal(t, overlayNone, rm2.overlay)
	assert.Contains(t, rm2.hiddenBuiltinColumns["pod"], "Namespace",
		"Enter commits — the live-applied state survives the close")
}

func TestColumnToggleReorderJPersistsOrder(t *testing.T) {
	t.Parallel()
	m := newColumnToggleModel()
	openColumnToggleSnapshot(&m)

	// Move Namespace (cursor 0) down past Ready.
	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	rm := r.(Model)

	assert.Equal(t, "Ready", rm.columnToggleItems[0].key)
	assert.Equal(t, "Namespace", rm.columnToggleItems[1].key)
	saved := rm.columnOrder["pod"]
	assert.NotEmpty(t, saved, "J must persist columnOrder live")
	// Ready must come before Namespace in the persisted order.
	posReady, posNamespace := -1, -1
	for i, k := range saved {
		switch k {
		case "Ready":
			posReady = i
		case "Namespace":
			posNamespace = i
		}
	}
	assert.NotEqual(t, -1, posReady, "Ready in saved order")
	assert.NotEqual(t, -1, posNamespace, "Namespace in saved order")
	assert.Less(t, posReady, posNamespace,
		"new order must reflect the J reorder, not the original")
}

// openColumnToggleSnapshot mirrors what the production openColumnToggle
// will do — capture a snapshot of the per-kind column state so Esc can
// restore. Tests call this directly to isolate the snapshot+restore
// behavior from the entry collection logic.
func openColumnToggleSnapshot(m *Model) {
	m.columnToggleSnapshot = captureColumnToggleSnapshot(m, m.middleColumnKind())
}
