package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTabKeysInertWhileTypingInFullscreenInput guards #795: a tab hotkey typed
// into a fullscreen viewer's text input must not open or switch tabs.
func TestTabKeysInertWhileTypingInFullscreenInput(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*Model)
	}{
		{"object explorer filter", func(m *Model) {
			m.mode = modeObjectExplorer
			m.objectExplorerView.filterActive = true
		}},
		{"help filter", func(m *Model) {
			m.mode = modeHelp
			m.helpFilterActive = true
		}},
	}
	for _, tc := range cases {
		for _, key := range []rune{'t', ']', '['} {
			t.Run(tc.name+"/"+string(key), func(t *testing.T) {
				m := basePush80Model()
				m.tabs = []TabState{{}, {}}
				tc.setup(&m)

				mdl, _ := m.handleKey(runeKey(key))
				got := mdl.(Model)

				assert.Len(t, got.tabs, 2)
				assert.Equal(t, 0, got.activeTab)
			})
		}
	}
}

// TestTabKeysWorkWithStaleFilterFlagInOtherMode checks that a filter flag left
// set by another mode (for example after a mouse tab switch) does not block tabs.
func TestTabKeysWorkWithStaleFilterFlagInOtherMode(t *testing.T) {
	m := basePush80Model()
	m.tabs = []TabState{{}, {}}
	m.mode = modeYAML
	m.objectExplorerView.filterActive = true
	m.helpFilterActive = true

	mdl, _ := m.handleKey(runeKey(']'))

	assert.Equal(t, 1, mdl.(Model).activeTab)
}
