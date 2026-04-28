package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// sampleTemplates returns a small set of templates for testing.
func sampleTemplates() []model.ResourceTemplate {
	return []model.ResourceTemplate{
		{Name: "Deployment", Description: "Create a Deployment", Category: "Workloads"},
		{Name: "Service", Description: "Create a Service", Category: "Networking"},
		{Name: "ConfigMap", Description: "Create a ConfigMap", Category: "Config"},
		{Name: "Secret", Description: "Create a Secret", Category: "Config"},
		{Name: "Ingress", Description: "Create an Ingress", Category: "Networking"},
	}
}

// --- filteredTemplates ---

func TestFilteredTemplatesNoFilter(t *testing.T) {
	templates := sampleTemplates()
	m := Model{templateItems: templates}
	result := m.filteredTemplates()
	assert.Equal(t, templates, result)
}

func TestFilteredTemplatesMatchByName(t *testing.T) {
	m := Model{
		templateItems:  sampleTemplates(),
		templateFilter: TextInput{Value: "deploy"},
	}
	result := m.filteredTemplates()
	assert.Len(t, result, 1)
	assert.Equal(t, "Deployment", result[0].Name)
}

func TestFilteredTemplatesMatchByDescription(t *testing.T) {
	m := Model{
		templateItems:  sampleTemplates(),
		templateFilter: TextInput{Value: "ingress"},
	}
	result := m.filteredTemplates()
	assert.Len(t, result, 1)
	assert.Equal(t, "Ingress", result[0].Name)
}

func TestFilteredTemplatesMatchByCategory(t *testing.T) {
	m := Model{
		templateItems:  sampleTemplates(),
		templateFilter: TextInput{Value: "config"},
	}
	result := m.filteredTemplates()
	assert.Len(t, result, 2)
	assert.Equal(t, "ConfigMap", result[0].Name)
	assert.Equal(t, "Secret", result[1].Name)
}

func TestFilteredTemplatesNoMatch(t *testing.T) {
	m := Model{
		templateItems:  sampleTemplates(),
		templateFilter: TextInput{Value: "nonexistent"},
	}
	result := m.filteredTemplates()
	assert.Empty(t, result)
}

func TestFilteredTemplatesCaseInsensitive(t *testing.T) {
	m := Model{
		templateItems:  sampleTemplates(),
		templateFilter: TextInput{Value: "DEPLOY"},
	}
	result := m.filteredTemplates()
	assert.Len(t, result, 1)
	assert.Equal(t, "Deployment", result[0].Name)
}

// --- handleTemplateOverlayKey: slash enters filter mode ---

func TestTemplateOverlaySlashEntersFilterMode(t *testing.T) {
	m := Model{
		overlay:        overlayTemplates,
		templateItems:  sampleTemplates(),
		templateCursor: 0,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	ret, _ := m.handleTemplateOverlayKey(runeKey('/'))
	result := ret.(Model)
	assert.True(t, result.templateSearchMode)
	assert.Empty(t, result.templateFilter.Value)
}

// --- handleTemplateFilterMode: typing ---

func TestTemplateFilterModeTyping(t *testing.T) {
	m := Model{
		overlay:            overlayTemplates,
		templateItems:      sampleTemplates(),
		templateSearchMode: true,
		templateFilter:     TextInput{Value: ""},
		templateCursor:     0,
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	ret, _ := m.handleTemplateFilterMode(runeKey('d'))
	result := ret.(Model)
	assert.Equal(t, "d", result.templateFilter.Value)
	assert.Equal(t, 0, result.templateCursor)
}

// --- handleTemplateFilterMode: backspace ---

func TestTemplateFilterModeBackspace(t *testing.T) {
	m := Model{
		overlay:            overlayTemplates,
		templateItems:      sampleTemplates(),
		templateSearchMode: true,
		templateFilter:     TextInput{Value: "dep", Cursor: 3},
		templateCursor:     0,
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	ret, _ := m.handleTemplateFilterMode(specialKey(tea.KeyBackspace))
	result := ret.(Model)
	assert.Equal(t, "de", result.templateFilter.Value)
}

// --- handleTemplateFilterMode: esc exits filter and clears ---

func TestTemplateFilterModeEscExitsAndClears(t *testing.T) {
	m := Model{
		overlay:            overlayTemplates,
		templateItems:      sampleTemplates(),
		templateSearchMode: true,
		templateFilter:     TextInput{Value: "deploy"},
		templateCursor:     0,
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	ret, _ := m.handleTemplateFilterMode(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.templateSearchMode)
	assert.Empty(t, result.templateFilter.Value)
	assert.Equal(t, 0, result.templateCursor)
}

// --- handleTemplateFilterMode: enter confirms filter ---

// "config" matches two sample templates (ConfigMap, Secret), so Enter must
// keep the legacy behavior: exit filter mode and preserve the filter text so
// the user can navigate the narrowed list. Auto-applying would be a guess.
func TestTemplateFilterModeEnterConfirmsWithMultipleResults(t *testing.T) {
	m := Model{
		overlay:            overlayTemplates,
		templateItems:      sampleTemplates(),
		templateSearchMode: true,
		templateFilter:     TextInput{Value: "config"},
		templateCursor:     0,
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	ret, cmd := m.handleTemplateFilterMode(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.False(t, result.templateSearchMode)
	assert.Equal(t, overlayTemplates, result.overlay, "overlay must stay open")
	// Filter text is preserved after enter
	assert.Equal(t, "config", result.templateFilter.Value)
	assert.Nil(t, cmd, "no command must be issued when more than one match")
}

// Filter narrows to a single template: Enter must apply it and close the
// overlay so the user does not have to press Enter again on a one-row list.
func TestTemplateFilterModeEnterAutoSelectsSoleResult(t *testing.T) {
	m := Model{
		overlay:            overlayTemplates,
		templateItems:      sampleTemplates(),
		templateSearchMode: true,
		templateFilter:     TextInput{Value: "deploy"},
		templateCursor:     0,
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	ret, cmd := m.handleTemplateFilterMode(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.False(t, result.templateSearchMode)
	assert.Equal(t, overlayNone, result.overlay, "overlay must close")
	assert.Empty(t, result.templateFilter.Value, "filter must be cleared after commit")
	assert.NotNil(t, cmd, "applyTemplate command must be returned")
}

// --- handleTemplateFilterMode: ctrl+w deletes word ---

func TestTemplateFilterModeCtrlW(t *testing.T) {
	m := Model{
		overlay:            overlayTemplates,
		templateItems:      sampleTemplates(),
		templateSearchMode: true,
		templateFilter:     TextInput{Value: "hello world", Cursor: 11},
		templateCursor:     0,
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	ret, _ := m.handleTemplateFilterMode(tea.KeyMsg{Type: tea.KeyCtrlW})
	result := ret.(Model)
	assert.Equal(t, "hello ", result.templateFilter.Value)
}

// --- Navigation works on filtered list ---

func TestTemplateOverlayNavigationOnFilteredList(t *testing.T) {
	m := Model{
		overlay:        overlayTemplates,
		templateItems:  sampleTemplates(),
		templateFilter: TextInput{Value: "config"},
		templateCursor: 0,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	// With filter "config", only ConfigMap and Secret match (2 items)
	// j should move cursor to 1
	ret, _ := m.handleTemplateOverlayKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, 1, result.templateCursor)

	// j again should not move past the end of filtered list
	ret2, _ := result.handleTemplateOverlayKey(runeKey('j'))
	result2 := ret2.(Model)
	assert.Equal(t, 1, result2.templateCursor)
}

// --- Enter selects the correct filtered template ---

func TestTemplateOverlayEnterSelectsFilteredTemplate(t *testing.T) {
	m := Model{
		overlay:        overlayTemplates,
		templateItems:  sampleTemplates(),
		templateFilter: TextInput{Value: "network"},
		templateCursor: 1, // second match: Ingress (Service=0, Ingress=1)
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	// Filtered list for "network" matches "Networking" category: Service, Ingress
	// Cursor at 1 should select Ingress
	ret, _ := m.handleTemplateOverlayKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	// When a template is selected, the overlay closes
	assert.Equal(t, overlayNone, result.overlay)
}

// --- Esc in normal mode with active filter clears filter first ---

func TestTemplateOverlayEscClearsFilterBeforeClosing(t *testing.T) {
	m := Model{
		overlay:        overlayTemplates,
		templateItems:  sampleTemplates(),
		templateFilter: TextInput{Value: "deploy"},
		templateCursor: 0,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	// First esc should clear filter, not close
	ret, _ := m.handleTemplateOverlayKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, overlayTemplates, result.overlay)
	assert.Empty(t, result.templateFilter.Value)

	// Second esc should close
	ret2, _ := result.handleTemplateOverlayKey(specialKey(tea.KeyEsc))
	result2 := ret2.(Model)
	assert.Equal(t, overlayNone, result2.overlay)
}

// --- handleTemplateOverlayKey dispatches to filter mode when templateSearchMode is true ---

func TestTemplateOverlayDispatchesToFilterMode(t *testing.T) {
	m := Model{
		overlay:            overlayTemplates,
		templateItems:      sampleTemplates(),
		templateSearchMode: true,
		templateFilter:     TextInput{Value: ""},
		templateCursor:     0,
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	// Typing 's' in filter mode should add to the filter, not navigate
	ret, _ := m.handleTemplateOverlayKey(runeKey('s'))
	result := ret.(Model)
	assert.Equal(t, "s", result.templateFilter.Value)
}
