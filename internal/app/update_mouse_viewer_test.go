package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// Mouse wheel must scroll the active viewer mode's content the same way
// it already scrolls the resource list and log viewer. Each tick moves
// 3 lines (matching the log-viewer constant), routed through the
// existing per-mode j/k scroll logic so cursor-visible/clamp behavior
// stays consistent with keyboard navigation.

func TestMouseWheelScrollsYAMLViewer(t *testing.T) {
	m := baseModelCov()
	m.mode = modeYAML
	m.yamlContent = ""
	for range 30 {
		m.yamlContent += "line\n"
	}
	m.height = 20
	m.yamlCursor = 10

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	rm := ret.(Model)
	assert.Greater(t, rm.yamlCursor, 10, "wheel down must advance the YAML cursor")

	ret2, _ := rm.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	rm2 := ret2.(Model)
	assert.Less(t, rm2.yamlCursor, rm.yamlCursor, "wheel up must move the cursor back")
}

func TestMouseWheelScrollsDescribeViewer(t *testing.T) {
	m := baseModelDescribe()
	m.describeContent = ""
	for range 30 {
		m.describeContent += "line\n"
	}
	m.height = 20
	m.describeCursor = 10

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	rm := ret.(Model)
	assert.Greater(t, rm.describeCursor, 10, "wheel down must advance the describe cursor")
}

func TestMouseWheelScrollsHelpViewer(t *testing.T) {
	m := baseModelCov()
	m.mode = modeHelp
	m.helpScroll = 5

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	rm := ret.(Model)
	assert.Greater(t, rm.helpScroll, 5, "wheel down must scroll the help viewer")

	ret2, _ := rm.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	rm2 := ret2.(Model)
	assert.Less(t, rm2.helpScroll, rm.helpScroll)
}

func TestMouseWheelInExplainOutlineMode(t *testing.T) {
	m := baseModelExplain()
	m.mode = modeExplain
	// explain has many fields populated by baseModelExplain; just need
	// to confirm wheel doesn't no-op.
	before := m.explainCursor

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	rm := ret.(Model)
	// Either explainCursor or explainScroll should advance — the test
	// allows either depending on the explain sub-mode.
	moved := rm.explainCursor != before || rm.explainScroll != m.explainScroll
	assert.True(t, moved, "wheel down must change something in explain mode")
}
