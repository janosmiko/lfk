package app

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- statusBar: command bar active ---

func TestStatusBarCommandBarActive(t *testing.T) {
	m := Model{
		nav:              model.NavigationState{Level: model.LevelResources},
		commandBarActive: true,
		commandBarInput:  TextInput{Value: "get pods"},
		width:            120,
		height:           40,
		tabs:             []TabState{{}},
		selectedItems:    make(map[string]bool),
	}
	bar := m.statusBar()
	stripped := stripANSI(bar)
	assert.Contains(t, stripped, ":")
	assert.Contains(t, stripped, "get pods")
}

func TestStatusBarCommandBarWithSuggestionsNoInline(t *testing.T) {
	m := Model{
		nav:                          model.NavigationState{Level: model.LevelResources},
		commandBarActive:             true,
		commandBarInput:              TextInput{Value: "get"},
		commandBarSuggestions:        []ui.Suggestion{{Text: "get pods", Category: "subcommand"}, {Text: "get deployments", Category: "subcommand"}, {Text: "get services", Category: "subcommand"}},
		commandBarSelectedSuggestion: 1,
		width:                        120,
		height:                       40,
		tabs:                         []TabState{{}},
		selectedItems:                make(map[string]bool),
	}
	// Suggestions should NOT appear inline in the status bar anymore.
	bar := m.statusBar()
	stripped := stripANSI(bar)
	assert.NotContains(t, stripped, "get pods")
	assert.NotContains(t, stripped, "get deployments")

	// Suggestions should appear in the dropdown instead.
	dropdown := m.commandBarDropdown()
	assert.NotEmpty(t, dropdown)
	droppedStripped := stripANSI(dropdown)
	assert.Contains(t, droppedStripped, "get pods")
	assert.Contains(t, droppedStripped, "get deployments")
	assert.Contains(t, droppedStripped, "get services")
}

func TestCommandBarDropdownEmpty(t *testing.T) {
	m := Model{
		commandBarActive:      false,
		commandBarSuggestions: nil,
		width:                 120,
		height:                40,
	}
	assert.Empty(t, m.commandBarDropdown())
}

func TestCommandBarDropdownActiveNoSuggestions(t *testing.T) {
	m := Model{
		commandBarActive:      true,
		commandBarSuggestions: nil,
		width:                 120,
		height:                40,
	}
	assert.Empty(t, m.commandBarDropdown())
}

func TestCommandBarDropdownMaxHeight(t *testing.T) {
	m := Model{
		commandBarActive: true,
		commandBarSuggestions: []ui.Suggestion{
			{Text: "a", Category: "cmd"},
			{Text: "b", Category: "cmd"},
			{Text: "c", Category: "cmd"},
		},
		commandBarSelectedSuggestion: 0,
		width:                        80,
		height:                       6, // height/2 = 3, capped at 3
	}
	dropdown := m.commandBarDropdown()
	assert.NotEmpty(t, dropdown)
	lines := strings.Split(dropdown, "\n")
	assert.Equal(t, 3, len(lines))
}

// --- statusBar: filter active ---

func TestStatusBarFilterActive(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelResources},
		filterActive:  true,
		filterInput:   TextInput{Value: "nginx"},
		width:         120,
		height:        40,
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
	}
	bar := m.statusBar()
	stripped := stripANSI(bar)
	assert.Contains(t, stripped, "filter")
	assert.Contains(t, stripped, "nginx")
}

// --- statusBar: search active ---

func TestStatusBarSearchActive(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelResources},
		searchActive:  true,
		searchInput:   TextInput{Value: "redis"},
		width:         120,
		height:        40,
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
	}
	bar := m.statusBar()
	stripped := stripANSI(bar)
	assert.Contains(t, stripped, "search")
	assert.Contains(t, stripped, "redis")
}

// --- statusBar: status message ---

func TestStatusBarErrorMessage(t *testing.T) {
	m := Model{
		nav:              model.NavigationState{Level: model.LevelResources},
		middleItems:      []model.Item{{Name: "pod"}},
		statusMessage:    "Connection refused",
		statusMessageErr: true,
		statusMessageExp: time.Now().Add(5 * time.Second),
		width:            120,
		height:           40,
		tabs:             []TabState{{}},
		selectedItems:    make(map[string]bool),
	}
	bar := m.statusBar()
	stripped := stripANSI(bar)
	assert.Contains(t, stripped, "Connection refused")
}

func TestStatusBarOkMessage(t *testing.T) {
	m := Model{
		nav:              model.NavigationState{Level: model.LevelResources},
		middleItems:      []model.Item{{Name: "pod"}},
		statusMessage:    "Watch mode ON",
		statusMessageErr: false,
		statusMessageExp: time.Now().Add(5 * time.Second),
		width:            120,
		height:           40,
		tabs:             []TabState{{}},
		selectedItems:    make(map[string]bool),
	}
	bar := m.statusBar()
	stripped := stripANSI(bar)
	assert.Contains(t, stripped, "Watch mode ON")
}

// --- statusBar: active filter preset indicator ---

func TestStatusBarActiveFilterPreset(t *testing.T) {
	m := Model{
		nav:                model.NavigationState{Level: model.LevelResources},
		middleItems:        []model.Item{{Name: "pod"}},
		activeFilterPreset: &FilterPreset{Name: "Failing"},
		width:              120,
		height:             40,
		tabs:               []TabState{{}},
		selectedItems:      make(map[string]bool),
	}
	bar := m.statusBar()
	stripped := stripANSI(bar)
	assert.Contains(t, stripped, "filter: Failing")
}

// --- statusBar: dashboard hints ---

func TestStatusBarDashboardHints(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level: model.LevelResourceTypes,
		},
		middleItems: []model.Item{
			{Name: "Cluster Dashboard", Extra: "__overview__"},
		},
		width:         200,
		height:        40,
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
	}
	bar := m.statusBar()
	stripped := stripANSI(bar)
	// Dashboard hints are different from standard hints.
	assert.Contains(t, stripped, "scroll")
	assert.Contains(t, stripped, "namespace")
}

// --- statusBar: cluster-list hints omit "actions" ---

func TestStatusBarClusterListOmitsActionsHint(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelClusters},
		middleItems:   []model.Item{{Name: "ctx-a"}},
		width:         200,
		height:        40,
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
	}
	stripped := stripANSI(m.statusBar())
	assert.NotContains(t, stripped, "actions")
	assert.Contains(t, stripped, "create")
	assert.Contains(t, stripped, "filter")
}

func TestStatusBarResourceListShowsActionsHint(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelResources},
		middleItems:   []model.Item{{Name: "pod"}},
		width:         200,
		height:        40,
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
	}
	stripped := stripANSI(m.statusBar())
	assert.Contains(t, stripped, "actions")
}

// --- statusBar: small width ---

func TestStatusBarSmallWidth(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelResources},
		middleItems:   []model.Item{{Name: "pod"}},
		width:         15,
		height:        40,
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
	}
	bar := m.statusBar()
	assert.NotEmpty(t, bar)
}

// --- renderErrorLogOverlay ---

func TestRenderErrorLogOverlay(t *testing.T) {
	m := Model{
		width:  120,
		height: 40,
		errorLog: []ui.ErrorLogEntry{
			{Level: "ERR", Message: "test error", Time: time.Now()},
			{Level: "INF", Message: "test info", Time: time.Now()},
		},
	}
	bg := "background content\n"
	result := m.renderErrorLogOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderErrorLogOverlaySmall(t *testing.T) {
	m := Model{
		width:  20,
		height: 10,
	}
	bg := "bg\n"
	result := m.renderErrorLogOverlay(bg)
	assert.NotEmpty(t, result)
}
