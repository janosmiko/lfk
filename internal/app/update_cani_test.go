package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// --- filterAllowedResources ---

func TestFilterAllowedResources(t *testing.T) {
	resources := []model.CanIResource{
		{
			Resource: "pods",
			Kind:     "Pod",
			Verbs:    map[string]bool{"get": true, "list": true, "delete": false},
		},
		{
			Resource: "secrets",
			Kind:     "Secret",
			Verbs:    map[string]bool{"get": false, "list": false},
		},
		{
			Resource: "deployments",
			Kind:     "Deployment",
			Verbs:    map[string]bool{"get": true, "create": false},
		},
	}

	filtered := filterAllowedResources(resources)
	assert.Len(t, filtered, 2)
	assert.Equal(t, "pods", filtered[0].Resource)
	assert.Equal(t, "deployments", filtered[1].Resource)
}

func TestFilterAllowedResourcesEmpty(t *testing.T) {
	var resources []model.CanIResource
	filtered := filterAllowedResources(resources)
	assert.Empty(t, filtered)
}

func TestFilterAllowedResourcesAllDenied(t *testing.T) {
	resources := []model.CanIResource{
		{
			Resource: "secrets",
			Verbs:    map[string]bool{"get": false, "list": false},
		},
	}
	filtered := filterAllowedResources(resources)
	assert.Empty(t, filtered)
}

func TestFilterAllowedResourcesEmptyVerbs(t *testing.T) {
	resources := []model.CanIResource{
		{Resource: "configmaps", Verbs: map[string]bool{}},
	}
	filtered := filterAllowedResources(resources)
	assert.Empty(t, filtered)
}

// --- countAllowedResources ---

func TestCountAllowedResources(t *testing.T) {
	resources := []model.CanIResource{
		{Resource: "pods", Verbs: map[string]bool{"get": true}},
		{Resource: "secrets", Verbs: map[string]bool{"get": false}},
		{Resource: "deployments", Verbs: map[string]bool{"list": true, "get": false}},
	}

	assert.Equal(t, 2, countAllowedResources(resources))
}

func TestCountAllowedResourcesNone(t *testing.T) {
	resources := []model.CanIResource{
		{Resource: "secrets", Verbs: map[string]bool{"get": false}},
	}
	assert.Equal(t, 0, countAllowedResources(resources))
}

func TestCountAllowedResourcesEmpty(t *testing.T) {
	assert.Equal(t, 0, countAllowedResources(nil))
}

// --- canIVisibleGroups ---

func TestCanIVisibleGroups(t *testing.T) {
	groups := []model.CanIGroup{
		{Name: "apps"},
		{Name: ""}, // core group
		{Name: "batch"},
		{Name: "networking.k8s.io"},
	}

	t.Run("no query returns all", func(t *testing.T) {
		m := Model{canIGroups: groups}
		indices := m.canIVisibleGroups()
		assert.Len(t, indices, 4)
	})

	t.Run("query filters by group name", func(t *testing.T) {
		m := Model{
			canIGroups:      groups,
			canISearchQuery: "app",
		}
		indices := m.canIVisibleGroups()
		assert.Len(t, indices, 1)
		assert.Equal(t, 0, indices[0])
	})

	t.Run("empty group name matches core", func(t *testing.T) {
		m := Model{
			canIGroups:      groups,
			canISearchQuery: "core",
		}
		indices := m.canIVisibleGroups()
		assert.Len(t, indices, 1)
		assert.Equal(t, 1, indices[0])
	})

	t.Run("case insensitive search", func(t *testing.T) {
		m := Model{
			canIGroups:      groups,
			canISearchQuery: "BATCH",
		}
		indices := m.canIVisibleGroups()
		assert.Len(t, indices, 1)
		assert.Equal(t, 2, indices[0])
	})

	t.Run("active search input overrides query", func(t *testing.T) {
		m := Model{
			canIGroups:       groups,
			canISearchQuery:  "apps", // would match "apps"
			canISearchActive: true,
			canISearchInput:  TextInput{Value: "batch"}, // overrides to "batch"
		}
		indices := m.canIVisibleGroups()
		assert.Len(t, indices, 1)
		assert.Equal(t, 2, indices[0])
	})

	t.Run("no match returns empty", func(t *testing.T) {
		m := Model{
			canIGroups:      groups,
			canISearchQuery: "nonexistent",
		}
		indices := m.canIVisibleGroups()
		assert.Empty(t, indices)
	})
}

// --- canIVisibleLines ---

func TestCanIVisibleLines(t *testing.T) {
	t.Run("normal terminal size", func(t *testing.T) {
		m := Model{height: 40}
		lines := m.canIVisibleLines()
		assert.Greater(t, lines, 0)
	})

	t.Run("small terminal", func(t *testing.T) {
		m := Model{height: 10}
		lines := m.canIVisibleLines()
		assert.Greater(t, lines, 0)
	})

	t.Run("very small terminal", func(t *testing.T) {
		m := Model{height: 5}
		lines := m.canIVisibleLines()
		assert.Greater(t, lines, 0)
	})
}

func TestCovHandleCanIKeyHelp(t *testing.T) {
	m := baseModelCov()
	m.canIGroups = []model.CanIGroup{{Name: "core"}}
	r, _ := m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	assert.Equal(t, modeHelp, r.(Model).mode)
}

func TestCovHandleCanIKeyToggleAllowed(t *testing.T) {
	m := baseModelCov()
	r, _ := m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.True(t, r.(Model).canIAllowedOnly)
}

func TestCovHandleCanIKeyQuit(t *testing.T) {
	m := baseModelCov()
	m.canISearchQuery = "apps"
	r, _ := m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	assert.Empty(t, r.(Model).canISearchQuery)

	m.canISearchQuery = ""
	r, _ = m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	assert.Equal(t, overlayNone, r.(Model).overlay)
}

func TestCovHandleCanIKeyNavigation(t *testing.T) {
	m := baseModelCov()
	m.canIGroups = []model.CanIGroup{{Name: "a"}, {Name: "b"}, {Name: "c"}}

	r, _ := m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, r.(Model).canIGroupCursor)

	m.canIGroupCursor = 2
	r, _ = m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 1, r.(Model).canIGroupCursor)

	m.canIGroupCursor = 0
	r, _ = m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	assert.Equal(t, 2, r.(Model).canIGroupCursor)

	m.canIGroupCursor = 2
	m.pendingG = true
	r, _ = m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	assert.Equal(t, 0, r.(Model).canIGroupCursor)
}

func TestCovHandleCanIKeyScrollResource(t *testing.T) {
	m := baseModelCov()
	m.canIGroups = []model.CanIGroup{{
		Name:      "core",
		Resources: make([]model.CanIResource, 20),
	}}
	r, _ := m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	assert.GreaterOrEqual(t, r.(Model).canIResourceScroll, 0)

	m.canIResourceScroll = 1
	r, _ = m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	assert.Equal(t, 0, r.(Model).canIResourceScroll)
}

func TestCovHandleCanIKeyPages(t *testing.T) {
	groups := make([]model.CanIGroup, 50)
	for i := range groups {
		groups[i] = model.CanIGroup{Name: "g"}
	}
	m := baseModelCov()
	m.canIGroups = groups

	r, _ := m.handleCanIKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	assert.Greater(t, r.(Model).canIGroupCursor, 0)

	m.canIGroupCursor = 25
	r, _ = m.handleCanIKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	assert.Less(t, r.(Model).canIGroupCursor, 25)

	m.canIGroupCursor = 0
	r, _ = m.handleCanIKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	assert.Greater(t, r.(Model).canIGroupCursor, 0)

	m.canIGroupCursor = 40
	r, _ = m.handleCanIKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	assert.Less(t, r.(Model).canIGroupCursor, 40)
}

func TestCovHandleCanISearchKey(t *testing.T) {
	m := baseModelCov()
	m.canISearchActive = true
	m.canISearchInput = TextInput{Value: "core"}
	r, _ := m.handleCanISearchKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, "core", r.(Model).canISearchQuery)

	m.canISearchActive = true
	r, _ = m.handleCanISearchKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.False(t, r.(Model).canISearchActive)

	m.canISearchActive = true
	m.canISearchInput = TextInput{Value: "co", Cursor: 2}
	r, _ = m.handleCanISearchKey(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "c", r.(Model).canISearchInput.Value)

	m.canISearchInput = TextInput{Value: "hello", Cursor: 5}
	r, _ = m.handleCanISearchKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	assert.Empty(t, r.(Model).canISearchInput.Value)

	m.canISearchInput = TextInput{}
	r, _ = m.handleCanISearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.Equal(t, "a", r.(Model).canISearchInput.Value)
}

func TestCovProcessCanIRulesWildcard(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx"
	m.discoveredResources["ctx"] = nil

	m.processCanIRules([]k8s.AccessRule{
		{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"*"}},
	})
	assert.NotEmpty(t, m.canIGroups)
}

func TestCovProcessCanIRulesWildcardAll(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx"
	m.discoveredResources["ctx"] = []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}

	m.processCanIRules([]k8s.AccessRule{
		{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get", "list"}},
	})
	assert.NotEmpty(t, m.canIGroups)
}

func TestCovProcessCanIRulesEmpty(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx"
	m.discoveredResources["ctx"] = []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}

	m.processCanIRules(nil)
	assert.NotEmpty(t, m.canIGroups)
}

func TestCovProcessCanIRulesWithCRDs(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx"
	m.discoveredResources["ctx"] = []model.ResourceTypeEntry{
		{Kind: "Application", Resource: "applications", APIGroup: "argoproj.io", APIVersion: "v1alpha1"},
	}

	m.processCanIRules([]k8s.AccessRule{
		{APIGroups: []string{"argoproj.io"}, Resources: []string{"applications"}, Verbs: []string{"get", "list"}},
	})

	var found bool
	for _, g := range m.canIGroups {
		if g.Name == "argoproj.io" {
			for _, r := range g.Resources {
				if r.Resource == "applications" {
					found = true
					assert.True(t, r.Verbs["get"])
					assert.True(t, r.Verbs["list"])
				}
			}
		}
	}
	assert.True(t, found, "should find argoproj.io/applications")
}

func TestCovCanISubjectNormalMode(t *testing.T) {
	m := baseModelCov()
	m.overlayItems = []model.Item{{Name: "sa1"}, {Name: "sa2"}}

	r, _ := m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, r.(Model).overlayCursor)

	m.overlayCursor = 1
	r, _ = m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, r.(Model).overlayCursor)

	m.overlay = overlayCanISubject
	r, _ = m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Equal(t, overlayCanI, r.(Model).overlay)

	m.overlayFilter = TextInput{Value: "admin"}
	r, _ = m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Empty(t, r.(Model).overlayFilter.Value)

	r, _ = m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	assert.True(t, r.(Model).canISubjectFilterMode)

	// Reset filter so filteredOverlayItems returns all items for page scroll tests.
	m.overlayFilter = TextInput{}
	r, _ = m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyCtrlD})
	assert.GreaterOrEqual(t, r.(Model).overlayCursor, 0)

	r, _ = m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyCtrlU})
	assert.GreaterOrEqual(t, r.(Model).overlayCursor, 0)

	r, _ = m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyCtrlF})
	assert.GreaterOrEqual(t, r.(Model).overlayCursor, 0)

	r, _ = m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyCtrlB})
	assert.GreaterOrEqual(t, r.(Model).overlayCursor, 0)
}

func TestCovCanISubjectFilterMode(t *testing.T) {
	m := baseModelCov()
	m.canISubjectFilterMode = true

	r, _ := m.handleCanISubjectFilterMode(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, r.(Model).canISubjectFilterMode)

	m.overlayFilter = TextInput{Value: "admin"}
	r, _ = m.handleCanISubjectFilterMode(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Empty(t, r.(Model).overlayFilter.Value)

	m.overlayFilter = TextInput{}
	r, _ = m.handleCanISubjectFilterMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.Equal(t, "a", r.(Model).overlayFilter.Value)

	m.overlayFilter = TextInput{Value: "ab", Cursor: 2}
	r, _ = m.handleCanISubjectFilterMode(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "a", r.(Model).overlayFilter.Value)

	m.overlayFilter = TextInput{Value: "hello", Cursor: 5}
	r, _ = m.handleCanISubjectFilterMode(tea.KeyMsg{Type: tea.KeyCtrlW})
	assert.Empty(t, r.(Model).overlayFilter.Value)
}

func TestCovCanISubjectKeyEscClearsFilter(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayCanISubject
	m.overlayFilter.Insert("test")
	m.overlayItems = []model.Item{{Name: "sa1"}}
	result, _ := m.handleCanISubjectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Empty(t, rm.overlayFilter.Value)
}

func TestCovCanISubjectKeyEscClosesOverlay(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayCanISubject
	result, _ := m.handleCanISubjectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayCanI, rm.overlay)
}

func TestCovCanISubjectKeyEnter(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayCanISubject
	m.overlayItems = []model.Item{{Name: "sa1", Extra: "sa:default:sa1"}}
	m.overlayCursor = 0
	_, cmd := m.handleCanISubjectOverlayKey(keyMsg("enter"))
	assert.NotNil(t, cmd)
}

func TestCovCanISubjectKeyEnterCurrentUser(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayCanISubject
	m.overlayItems = []model.Item{{Name: "Current User", Extra: ""}}
	m.overlayCursor = 0
	_, cmd := m.handleCanISubjectOverlayKey(keyMsg("enter"))
	assert.NotNil(t, cmd)
}

func TestCovCanISubjectKeySlash(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayCanISubject
	result, _ := m.handleCanISubjectOverlayKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.canISubjectFilterMode)
}

func TestCovCanISubjectKeyNavigation(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayCanISubject
	m.overlayItems = []model.Item{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	result, _ := m.handleCanISubjectOverlayKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.overlayCursor)

	result, _ = rm.handleCanISubjectOverlayKey(keyMsg("k"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.overlayCursor)

	result, _ = rm.handleCanISubjectOverlayKey(keyMsg("ctrl+d"))
	rm = result.(Model)

	result, _ = rm.handleCanISubjectOverlayKey(keyMsg("ctrl+u"))
	rm = result.(Model)

	result, _ = rm.handleCanISubjectOverlayKey(keyMsg("ctrl+f"))
	rm = result.(Model)

	result, _ = rm.handleCanISubjectOverlayKey(keyMsg("ctrl+b"))
	rm = result.(Model)
}

func TestCovCanISubjectFilterModeDelegation(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayCanISubject
	m.canISubjectFilterMode = true
	// Should delegate to filter key handler
	result, _ := m.handleCanISubjectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.canISubjectFilterMode)
}

func TestCovOpenCanIBrowser(t *testing.T) {
	m := baseModelOverlay()
	_, cmd := m.openCanIBrowser()
	assert.NotNil(t, cmd)
}
