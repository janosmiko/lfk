package app

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- handleNamespaceOverlayKey: normal mode ---

func TestNamespaceOverlayEscClosesWithNoFilter(t *testing.T) {
	m := Model{
		overlay:       overlayNamespace,
		overlayItems:  []model.Item{{Name: "All Namespaces", Status: "all"}, {Name: "default"}},
		overlayFilter: TextInput{Value: ""},
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleNamespaceOverlayKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
}

func TestNamespaceOverlayEscClearsFilterFirst(t *testing.T) {
	m := Model{
		overlay:       overlayNamespace,
		overlayItems:  []model.Item{{Name: "All Namespaces", Status: "all"}, {Name: "default"}},
		overlayFilter: TextInput{Value: "def"},
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleNamespaceOverlayKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	// First esc clears filter, does not close
	assert.Equal(t, overlayNamespace, result.overlay)
	assert.Empty(t, result.overlayFilter.Value)
}

func TestNamespaceOverlayJKNavigation(t *testing.T) {
	items := []model.Item{
		{Name: "All Namespaces", Status: "all"},
		{Name: "default"},
		{Name: "kube-system"},
	}

	t.Run("j moves down", func(t *testing.T) {
		m := Model{
			overlay:       overlayNamespace,
			overlayItems:  items,
			overlayCursor: 0,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleNamespaceNormalMode(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 1, result.overlayCursor)
	})

	t.Run("k moves up", func(t *testing.T) {
		m := Model{
			overlay:       overlayNamespace,
			overlayItems:  items,
			overlayCursor: 2,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleNamespaceNormalMode(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 1, result.overlayCursor)
	})

	t.Run("ctrl+d page down", func(t *testing.T) {
		m := Model{
			overlay:       overlayNamespace,
			overlayItems:  items,
			overlayCursor: 0,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleNamespaceNormalMode(tea.KeyMsg{Type: tea.KeyCtrlD})
		result := ret.(Model)
		assert.Equal(t, 2, result.overlayCursor) // clamps to max
	})

	t.Run("ctrl+u page up", func(t *testing.T) {
		m := Model{
			overlay:       overlayNamespace,
			overlayItems:  items,
			overlayCursor: 2,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleNamespaceNormalMode(tea.KeyMsg{Type: tea.KeyCtrlU})
		result := ret.(Model)
		assert.Equal(t, 0, result.overlayCursor)
	})
}

func TestNamespaceOverlaySlashEntersFilter(t *testing.T) {
	m := Model{
		overlay:      overlayNamespace,
		overlayItems: []model.Item{{Name: "default"}},
		tabs:         []TabState{{}},
		width:        80,
		height:       40,
	}
	ret, _ := m.handleNamespaceNormalMode(runeKey('/'))
	result := ret.(Model)
	assert.True(t, result.nsFilterMode)
}

func TestNamespaceOverlaySpaceTogglesSelection(t *testing.T) {
	items := []model.Item{
		{Name: "All Namespaces", Status: "all"},
		{Name: "default"},
		{Name: "kube-system"},
	}
	m := Model{
		overlay:       overlayNamespace,
		overlayItems:  items,
		overlayCursor: 1,
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleNamespaceNormalMode(specialKey(tea.KeySpace))
	result := ret.(Model)
	assert.True(t, result.nsSelectionModified)
	assert.True(t, result.selectedNamespaces["default"])
	assert.Equal(t, 2, result.overlayCursor) // advances cursor
}

func TestNamespaceOverlaySpaceOnAllClearsSelection(t *testing.T) {
	items := []model.Item{
		{Name: "All Namespaces", Status: "all"},
		{Name: "default"},
	}
	m := Model{
		overlay:            overlayNamespace,
		overlayItems:       items,
		overlayCursor:      0,
		selectedNamespaces: map[string]bool{"default": true},
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	ret, _ := m.handleNamespaceNormalMode(specialKey(tea.KeySpace))
	result := ret.(Model)
	assert.Nil(t, result.selectedNamespaces)
	assert.True(t, result.allNamespaces)
}

// A inside the namespace selector mirrors A outside (kb.AllNamespaces):
// flip to all-namespaces mode. The user expectation is muscle memory —
// the same key shouldn't change meaning just because the overlay is open.
// The cursor must also jump to the All-Namespaces row, otherwise a
// follow-up Enter will treat the cursor's previous position as a
// single-namespace selection and undo the all-ns flip.
func TestNamespaceOverlayAEnablesAllNamespaces(t *testing.T) {
	m := Model{
		overlay:            overlayNamespace,
		overlayItems:       []model.Item{{Name: "All Namespaces", Status: "all"}, {Name: "default"}, {Name: "kube-system"}},
		selectedNamespaces: map[string]bool{"default": true},
		allNamespaces:      false,
		overlayCursor:      2, // standing on "kube-system"
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	ret, _ := m.handleNamespaceNormalMode(runeKey('A'))
	result := ret.(Model)
	assert.Nil(t, result.selectedNamespaces, "A must clear individual selections")
	assert.True(t, result.allNamespaces, "A must enable all-namespaces mode")
	assert.Equal(t, 0, result.overlayCursor,
		"A must move cursor to the All-Namespaces row so Enter applies all-ns")
}

// --- handleNamespaceFilterMode ---

func TestNamespaceFilterModeEscExits(t *testing.T) {
	m := Model{
		overlay:       overlayNamespace,
		nsFilterMode:  true,
		overlayFilter: TextInput{Value: "test"},
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleNamespaceFilterMode(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.nsFilterMode)
	assert.Empty(t, result.overlayFilter.Value)
}

func TestNamespaceFilterModeEnterCommits(t *testing.T) {
	m := Model{
		overlay:       overlayNamespace,
		nsFilterMode:  true,
		overlayFilter: TextInput{Value: "kube"},
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleNamespaceFilterMode(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.False(t, result.nsFilterMode)
	// Filter text is preserved after enter
}

// Filter narrows the list to a single concrete namespace: Enter must apply
// it and close the overlay so the user does not have to press Enter again
// on a one-row list.
func TestNamespaceFilterModeEnterAutoSelectsSoleResult(t *testing.T) {
	items := []model.Item{
		{Name: "All Namespaces", Status: "all"},
		{Name: "default"},
		{Name: "kube-system"},
	}
	m := Model{
		overlay:       overlayNamespace,
		nsFilterMode:  true,
		overlayItems:  items,
		overlayFilter: TextInput{Value: "kube"},
		allNamespaces: true,
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, cmd := m.handleNamespaceFilterMode(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.False(t, result.nsFilterMode)
	assert.Equal(t, overlayNone, result.overlay)
	assert.Equal(t, "kube-system", result.namespace)
	assert.True(t, result.selectedNamespaces["kube-system"])
	assert.False(t, result.allNamespaces)
	assert.NotNil(t, cmd)
}

// Two filtered results: Enter must keep the legacy behavior — exit filter
// mode and let the user pick. Auto-applying would be a guess.
func TestNamespaceFilterModeEnterDoesNotAutoApplyMultipleResults(t *testing.T) {
	items := []model.Item{
		{Name: "All Namespaces", Status: "all"},
		{Name: "kube-system"},
		{Name: "kube-public"},
	}
	m := Model{
		overlay:       overlayNamespace,
		nsFilterMode:  true,
		overlayItems:  items,
		overlayFilter: TextInput{Value: "kube"},
		allNamespaces: true,
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleNamespaceFilterMode(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.False(t, result.nsFilterMode)
	assert.Equal(t, overlayNamespace, result.overlay)
	assert.Empty(t, result.selectedNamespaces)
	assert.True(t, result.allNamespaces)
}

// User has been Space-toggling selections and then opens filter to refine.
// Even if the filter narrows to one result, do not silently replace their
// in-progress multi-selection — they pressed Enter to exit filter mode,
// not to abandon the selections they already made.
func TestNamespaceFilterModeEnterPreservesMultiSelect(t *testing.T) {
	items := []model.Item{
		{Name: "All Namespaces", Status: "all"},
		{Name: "default"},
		{Name: "kube-system"},
	}
	m := Model{
		overlay:             overlayNamespace,
		nsFilterMode:        true,
		nsSelectionModified: true,
		selectedNamespaces:  map[string]bool{"default": true},
		overlayItems:        items,
		overlayFilter:       TextInput{Value: "kube"},
		tabs:                []TabState{{}},
		width:               80,
		height:              40,
	}
	ret, _ := m.handleNamespaceFilterMode(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.False(t, result.nsFilterMode)
	assert.Equal(t, overlayNamespace, result.overlay)
	assert.True(t, result.selectedNamespaces["default"], "existing Space-toggled selection must survive")
	assert.False(t, result.selectedNamespaces["kube-system"], "single filter match must not be auto-applied during multi-select")
}

func TestNamespaceFilterModeTyping(t *testing.T) {
	m := Model{
		overlay:       overlayNamespace,
		nsFilterMode:  true,
		overlayFilter: TextInput{Value: ""},
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleNamespaceFilterMode(runeKey('d'))
	result := ret.(Model)
	assert.Equal(t, "d", result.overlayFilter.Value)
	assert.Equal(t, 0, result.overlayCursor)
}

func TestNamespaceFilterModeBackspace(t *testing.T) {
	m := Model{
		overlay:       overlayNamespace,
		nsFilterMode:  true,
		overlayFilter: TextInput{Value: "abc", Cursor: 3},
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleNamespaceFilterMode(specialKey(tea.KeyBackspace))
	result := ret.(Model)
	assert.Equal(t, "ab", result.overlayFilter.Value)
}

// --- handleTemplateOverlayKey ---

func TestTemplateOverlayEscCloses(t *testing.T) {
	m := Model{
		overlay: overlayTemplates,
		tabs:    []TabState{{}},
		width:   80,
		height:  40,
	}
	ret, _ := m.handleTemplateOverlayKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
}

func TestTemplateOverlayJKNavigation(t *testing.T) {
	templates := []model.ResourceTemplate{{Name: "tmpl1"}, {Name: "tmpl2"}, {Name: "tmpl3"}}
	m := Model{
		overlay:        overlayTemplates,
		templateItems:  templates,
		templateCursor: 0,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}

	// j moves down
	ret, _ := m.handleTemplateOverlayKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, 1, result.templateCursor)

	// k moves up
	ret2, _ := result.handleTemplateOverlayKey(runeKey('k'))
	result2 := ret2.(Model)
	assert.Equal(t, 0, result2.templateCursor)
}

func TestTemplateOverlayEnterEmptyListNoOp(t *testing.T) {
	m := Model{
		overlay:        overlayTemplates,
		templateItems:  []model.ResourceTemplate{},
		templateCursor: 0,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	ret, cmd := m.handleTemplateOverlayKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.Equal(t, overlayTemplates, result.overlay)
	assert.Nil(t, cmd)
}

func TestTemplateOverlayCtrlDPageDown(t *testing.T) {
	templates := make([]model.ResourceTemplate, 25)
	for i := range templates {
		templates[i] = model.ResourceTemplate{Name: fmt.Sprintf("tmpl%d", i)}
	}
	m := Model{
		overlay:        overlayTemplates,
		templateItems:  templates,
		templateCursor: 0,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	ret, _ := m.handleTemplateOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	result := ret.(Model)
	assert.Equal(t, 10, result.templateCursor)
}

func TestTemplateOverlayCtrlDClampsToEnd(t *testing.T) {
	templates := make([]model.ResourceTemplate, 5)
	for i := range templates {
		templates[i] = model.ResourceTemplate{Name: fmt.Sprintf("tmpl%d", i)}
	}
	m := Model{
		overlay:        overlayTemplates,
		templateItems:  templates,
		templateCursor: 0,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	ret, _ := m.handleTemplateOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	result := ret.(Model)
	assert.Equal(t, 4, result.templateCursor)
}

func TestTemplateOverlayCtrlUPageUp(t *testing.T) {
	templates := make([]model.ResourceTemplate, 25)
	for i := range templates {
		templates[i] = model.ResourceTemplate{Name: fmt.Sprintf("tmpl%d", i)}
	}
	m := Model{
		overlay:        overlayTemplates,
		templateItems:  templates,
		templateCursor: 15,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	ret, _ := m.handleTemplateOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result := ret.(Model)
	assert.Equal(t, 5, result.templateCursor)
}

func TestTemplateOverlayCtrlUClampsToZero(t *testing.T) {
	templates := make([]model.ResourceTemplate, 25)
	for i := range templates {
		templates[i] = model.ResourceTemplate{Name: fmt.Sprintf("tmpl%d", i)}
	}
	m := Model{
		overlay:        overlayTemplates,
		templateItems:  templates,
		templateCursor: 3,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	ret, _ := m.handleTemplateOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result := ret.(Model)
	assert.Equal(t, 0, result.templateCursor)
}

func TestTemplateOverlayCtrlFFullPage(t *testing.T) {
	templates := make([]model.ResourceTemplate, 30)
	for i := range templates {
		templates[i] = model.ResourceTemplate{Name: fmt.Sprintf("tmpl%d", i)}
	}
	m := Model{
		overlay:        overlayTemplates,
		templateItems:  templates,
		templateCursor: 0,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	ret, _ := m.handleTemplateOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	result := ret.(Model)
	assert.Equal(t, 20, result.templateCursor)
}

func TestTemplateOverlayCtrlBFullPageUp(t *testing.T) {
	templates := make([]model.ResourceTemplate, 30)
	for i := range templates {
		templates[i] = model.ResourceTemplate{Name: fmt.Sprintf("tmpl%d", i)}
	}
	m := Model{
		overlay:        overlayTemplates,
		templateItems:  templates,
		templateCursor: 25,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	ret, _ := m.handleTemplateOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	result := ret.(Model)
	assert.Equal(t, 5, result.templateCursor)
}

func TestTemplateOverlayGGJumpsToTop(t *testing.T) {
	templates := make([]model.ResourceTemplate, 10)
	for i := range templates {
		templates[i] = model.ResourceTemplate{Name: fmt.Sprintf("tmpl%d", i)}
	}
	m := Model{
		overlay:        overlayTemplates,
		templateItems:  templates,
		templateCursor: 8,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	// First g sets pendingG.
	ret, _ := m.handleTemplateOverlayKey(runeKey('g'))
	result := ret.(Model)
	assert.True(t, result.pendingG)
	assert.Equal(t, 8, result.templateCursor) // cursor unchanged yet

	// Second g jumps to top.
	ret2, _ := result.handleTemplateOverlayKey(runeKey('g'))
	result2 := ret2.(Model)
	assert.False(t, result2.pendingG)
	assert.Equal(t, 0, result2.templateCursor)
}

func TestTemplateOverlayShiftGJumpsToBottom(t *testing.T) {
	templates := make([]model.ResourceTemplate, 10)
	for i := range templates {
		templates[i] = model.ResourceTemplate{Name: fmt.Sprintf("tmpl%d", i)}
	}
	m := Model{
		overlay:        overlayTemplates,
		templateItems:  templates,
		templateCursor: 0,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	ret, _ := m.handleTemplateOverlayKey(runeKey('G'))
	result := ret.(Model)
	assert.Equal(t, 9, result.templateCursor)
}

func TestTemplateOverlayShiftGEmptyListNoOp(t *testing.T) {
	m := Model{
		overlay:        overlayTemplates,
		templateItems:  []model.ResourceTemplate{},
		templateCursor: 0,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	ret, _ := m.handleTemplateOverlayKey(runeKey('G'))
	result := ret.(Model)
	assert.Equal(t, 0, result.templateCursor)
}

// --- handleRollbackOverlayKey ---

func TestRollbackOverlayEscCloses(t *testing.T) {
	m := Model{
		overlay:           overlayRollback,
		rollbackRevisions: []k8s.DeploymentRevision{{Revision: 1, Name: "rs-1"}},
		tabs:              []TabState{{}},
		width:             80,
		height:            40,
	}
	ret, _ := m.handleRollbackOverlayKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.Nil(t, result.rollbackRevisions)
}

func TestRollbackOverlayJKNavigation(t *testing.T) {
	revisions := []k8s.DeploymentRevision{{Revision: 1}, {Revision: 2}, {Revision: 3}}
	m := Model{
		overlay:           overlayRollback,
		rollbackRevisions: revisions,
		rollbackCursor:    0,
		tabs:              []TabState{{}},
		width:             80,
		height:            40,
	}

	ret, _ := m.handleRollbackOverlayKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, 1, result.rollbackCursor)

	ret2, _ := result.handleRollbackOverlayKey(runeKey('k'))
	result2 := ret2.(Model)
	assert.Equal(t, 0, result2.rollbackCursor)
}

func TestRollbackOverlayCtrlDPageDown(t *testing.T) {
	revisions := make([]k8s.DeploymentRevision, 20)
	m := Model{
		overlay:           overlayRollback,
		rollbackRevisions: revisions,
		rollbackCursor:    0,
		tabs:              []TabState{{}},
		width:             80,
		height:            40,
	}
	ret, _ := m.handleRollbackOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	result := ret.(Model)
	assert.Equal(t, 10, result.rollbackCursor)
}

func TestRollbackOverlayCtrlUPageUp(t *testing.T) {
	revisions := make([]k8s.DeploymentRevision, 20)
	m := Model{
		overlay:           overlayRollback,
		rollbackRevisions: revisions,
		rollbackCursor:    15,
		tabs:              []TabState{{}},
		width:             80,
		height:            40,
	}
	ret, _ := m.handleRollbackOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result := ret.(Model)
	assert.Equal(t, 5, result.rollbackCursor)
}

// --- handleHelmRollbackOverlayKey ---

func TestHelmRollbackOverlayEscCloses(t *testing.T) {
	m := Model{
		overlay:               overlayHelmRollback,
		helmRollbackRevisions: []ui.HelmRevision{{Revision: 1}},
		tabs:                  []TabState{{}},
		width:                 80,
		height:                40,
	}
	ret, _ := m.handleHelmRollbackOverlayKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.Nil(t, result.helmRollbackRevisions)
}

func TestHelmRollbackOverlayJKNavigation(t *testing.T) {
	revisions := []ui.HelmRevision{{Revision: 1}, {Revision: 2}, {Revision: 3}}
	m := Model{
		overlay:               overlayHelmRollback,
		helmRollbackRevisions: revisions,
		helmRollbackCursor:    0,
		tabs:                  []TabState{{}},
		width:                 80,
		height:                40,
	}

	ret, _ := m.handleHelmRollbackOverlayKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, 1, result.helmRollbackCursor)

	ret2, _ := result.handleHelmRollbackOverlayKey(runeKey('k'))
	result2 := ret2.(Model)
	assert.Equal(t, 0, result2.helmRollbackCursor)
}

func TestHelmRollbackOverlayCtrlDPageDown(t *testing.T) {
	revisions := make([]ui.HelmRevision, 20)
	m := Model{
		overlay:               overlayHelmRollback,
		helmRollbackRevisions: revisions,
		helmRollbackCursor:    0,
		tabs:                  []TabState{{}},
		width:                 80,
		height:                40,
	}
	ret, _ := m.handleHelmRollbackOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	result := ret.(Model)
	assert.Equal(t, 10, result.helmRollbackCursor)
}

// --- handleHelmHistoryOverlayKey ---

func TestHelmHistoryOverlayEscCloses(t *testing.T) {
	m := Model{
		overlay:              overlayHelmHistory,
		helmHistoryRevisions: []ui.HelmRevision{{Revision: 1}},
		tabs:                 []TabState{{}},
		width:                80,
		height:               40,
	}
	ret, _ := m.handleHelmHistoryOverlayKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.Nil(t, result.helmHistoryRevisions)
}

func TestHelmHistoryOverlayJKNavigation(t *testing.T) {
	revisions := []ui.HelmRevision{{Revision: 1}, {Revision: 2}, {Revision: 3}}
	m := Model{
		overlay:              overlayHelmHistory,
		helmHistoryRevisions: revisions,
		helmHistoryCursor:    0,
		tabs:                 []TabState{{}},
		width:                80,
		height:               40,
	}

	ret, _ := m.handleHelmHistoryOverlayKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, 1, result.helmHistoryCursor)

	ret2, _ := result.handleHelmHistoryOverlayKey(runeKey('k'))
	result2 := ret2.(Model)
	assert.Equal(t, 0, result2.helmHistoryCursor)
}

func TestHelmHistoryOverlayPageNavigation(t *testing.T) {
	revisions := make([]ui.HelmRevision, 30)
	m := Model{
		overlay:              overlayHelmHistory,
		helmHistoryRevisions: revisions,
		helmHistoryCursor:    0,
		tabs:                 []TabState{{}},
		width:                80,
		height:               40,
	}
	ret, _ := m.handleHelmHistoryOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	result := ret.(Model)
	assert.Equal(t, 10, result.helmHistoryCursor)

	ret, _ = result.handleHelmHistoryOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result = ret.(Model)
	assert.Equal(t, 0, result.helmHistoryCursor)

	ret, _ = result.handleHelmHistoryOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	result = ret.(Model)
	assert.Equal(t, 20, result.helmHistoryCursor)

	ret, _ = result.handleHelmHistoryOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	result = ret.(Model)
	assert.Equal(t, 0, result.helmHistoryCursor)
}

func TestHelmHistoryOverlayEnterIsNoop(t *testing.T) {
	// Enter must NOT trigger a rollback from the history view. The overlay
	// stays open and no command is returned.
	revisions := []ui.HelmRevision{{Revision: 1}, {Revision: 2}}
	m := Model{
		overlay:              overlayHelmHistory,
		helmHistoryRevisions: revisions,
		helmHistoryCursor:    1,
		tabs:                 []TabState{{}},
		width:                80,
		height:               40,
	}
	ret, cmd := m.handleHelmHistoryOverlayKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.Equal(t, overlayHelmHistory, result.overlay)
	assert.Equal(t, 1, result.helmHistoryCursor)
	assert.Nil(t, cmd)
}

// --- handleLogPodFilterMode ---

func TestLogPodFilterModeEscExits(t *testing.T) {
	m := Model{
		overlay:            overlayPodSelect,
		logPodFilterActive: true,
		logPodFilterText:   "test",
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	ret, _ := m.handleLogPodFilterMode(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.logPodFilterActive)
	assert.Empty(t, result.logPodFilterText)
}

func TestLogPodFilterModeEnterCommits(t *testing.T) {
	m := Model{
		overlay:            overlayPodSelect,
		logPodFilterActive: true,
		logPodFilterText:   "kube",
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	ret, _ := m.handleLogPodFilterMode(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.False(t, result.logPodFilterActive)
}

func TestLogPodFilterModeTyping(t *testing.T) {
	m := Model{
		overlay:            overlayPodSelect,
		logPodFilterActive: true,
		logPodFilterText:   "",
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	ret, _ := m.handleLogPodFilterMode(runeKey('a'))
	result := ret.(Model)
	assert.Equal(t, "a", result.logPodFilterText)
}

func TestLogPodFilterModeBackspace(t *testing.T) {
	m := Model{
		overlay:            overlayPodSelect,
		logPodFilterActive: true,
		logPodFilterText:   "abc",
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	ret, _ := m.handleLogPodFilterMode(specialKey(tea.KeyBackspace))
	result := ret.(Model)
	assert.Equal(t, "ab", result.logPodFilterText)
}

func TestLogPodFilterModeCtrlW(t *testing.T) {
	m := Model{
		overlay:            overlayPodSelect,
		logPodFilterActive: true,
		logPodFilterText:   "hello world",
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	ret, _ := m.handleLogPodFilterMode(tea.KeyMsg{Type: tea.KeyCtrlW})
	result := ret.(Model)
	assert.Equal(t, "hello ", result.logPodFilterText)
}

// --- handleLogContainerFilterMode ---

func TestLogContainerFilterModeEscExits(t *testing.T) {
	m := Model{
		overlay:                  overlayLogContainerSelect,
		logContainerFilterActive: true,
		logContainerFilterText:   "test",
		tabs:                     []TabState{{}},
		width:                    80,
		height:                   40,
	}
	ret, _ := m.handleLogContainerFilterMode(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.logContainerFilterActive)
	assert.Empty(t, result.logContainerFilterText)
}

func TestLogContainerFilterModeTyping(t *testing.T) {
	m := Model{
		overlay:                  overlayLogContainerSelect,
		logContainerFilterActive: true,
		logContainerFilterText:   "",
		tabs:                     []TabState{{}},
		width:                    80,
		height:                   40,
	}
	ret, _ := m.handleLogContainerFilterMode(runeKey('x'))
	result := ret.(Model)
	assert.Equal(t, "x", result.logContainerFilterText)
}

func TestLogContainerFilterModeBackspace(t *testing.T) {
	m := Model{
		overlay:                  overlayLogContainerSelect,
		logContainerFilterActive: true,
		logContainerFilterText:   "abc",
		tabs:                     []TabState{{}},
		width:                    80,
		height:                   40,
	}
	ret, _ := m.handleLogContainerFilterMode(specialKey(tea.KeyBackspace))
	result := ret.(Model)
	assert.Equal(t, "ab", result.logContainerFilterText)
}

func TestLogContainerFilterModeCtrlW(t *testing.T) {
	m := Model{
		overlay:                  overlayLogContainerSelect,
		logContainerFilterActive: true,
		logContainerFilterText:   "hello world",
		tabs:                     []TabState{{}},
		width:                    80,
		height:                   40,
	}
	ret, _ := m.handleLogContainerFilterMode(tea.KeyMsg{Type: tea.KeyCtrlW})
	result := ret.(Model)
	assert.Equal(t, "hello ", result.logContainerFilterText)
}

// --- handleColorschemeOverlayKey ---

func TestColorschemeOverlaySlashEntersFilter(t *testing.T) {
	m := Model{
		overlay: overlayColorscheme,
		tabs:    []TabState{{}},
		width:   80,
		height:  40,
	}
	ret, _ := m.handleColorschemeNormalMode(runeKey('/'))
	result := ret.(Model)
	assert.True(t, result.schemeFilterMode)
}

func TestColorschemeFilterModeEscExits(t *testing.T) {
	m := Model{
		overlay:          overlayColorscheme,
		schemeFilterMode: true,
		schemeFilter:     TextInput{Value: "test"},
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
	}
	ret, _ := m.handleColorschemeFilterMode(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.schemeFilterMode)
	assert.Empty(t, result.schemeFilter.Value)
}

func TestColorschemeFilterModeTyping(t *testing.T) {
	m := Model{
		overlay:          overlayColorscheme,
		schemeFilterMode: true,
		schemeFilter:     TextInput{Value: ""},
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
	}
	ret, _ := m.handleColorschemeFilterMode(runeKey('d'))
	result := ret.(Model)
	assert.Equal(t, "d", result.schemeFilter.Value)
}

func TestColorschemeFilterModeBackspace(t *testing.T) {
	m := Model{
		overlay:          overlayColorscheme,
		schemeFilterMode: true,
		schemeFilter:     TextInput{Value: "abc", Cursor: 3},
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
	}
	ret, _ := m.handleColorschemeFilterMode(specialKey(tea.KeyBackspace))
	result := ret.(Model)
	assert.Equal(t, "ab", result.schemeFilter.Value)
}

func TestColorschemeFilterModeEnterCommits(t *testing.T) {
	m := Model{
		overlay:          overlayColorscheme,
		schemeFilterMode: true,
		schemeFilter:     TextInput{Value: "dark"},
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
	}
	ret, _ := m.handleColorschemeFilterMode(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.False(t, result.schemeFilterMode)
}
