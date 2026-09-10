package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCovOpenFinalizerSearch(t *testing.T) {
	m := baseModelCov()
	m.openFinalizerSearch()

	assert.Equal(t, overlayFinalizerSearch, m.overlay)
	assert.True(t, m.finalizerSearch.filterActive)
	assert.False(t, m.finalizerSearch.loading)
	assert.Empty(t, m.finalizerSearch.pattern)
	assert.Empty(t, m.finalizerSearch.results)
	assert.NotNil(t, m.finalizerSearch.selected)
	assert.Equal(t, 0, m.finalizerSearch.cursor)
}

func TestCovFinalizerKeyEsc(t *testing.T) {
	m := baseModelFinalizer()
	result, _ := m.handleFinalizerSearchKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.Nil(t, rm.finalizerSearch.results)
}

func TestCovFinalizerKeyDown(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.cursor = 0
	result, _ := m.handleFinalizerSearchKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.finalizerSearch.cursor)
}

func TestCovFinalizerKeyUp(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.cursor = 2
	result, _ := m.handleFinalizerSearchKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.finalizerSearch.cursor)
}

func TestCovFinalizerKeyGG(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.cursor = 2
	result, _ := m.handleFinalizerSearchKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)
	result, _ = rm.handleFinalizerSearchKey(keyMsg("g"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.finalizerSearch.cursor)
}

func TestCovFinalizerKeyBigG(t *testing.T) {
	m := baseModelFinalizer()
	result, _ := m.handleFinalizerSearchKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.finalizerSearch.cursor)
}

func TestCovFinalizerKeyCtrlD(t *testing.T) {
	m := baseModelFinalizer()
	result, _ := m.handleFinalizerSearchKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.LessOrEqual(t, rm.finalizerSearch.cursor, 2)
}

func TestCovFinalizerKeyCtrlU(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.cursor = 2
	result, _ := m.handleFinalizerSearchKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.GreaterOrEqual(t, rm.finalizerSearch.cursor, 0)
}

func TestCovFinalizerKeyCtrlF(t *testing.T) {
	m := baseModelFinalizer()
	result, _ := m.handleFinalizerSearchKey(keyMsg("ctrl+f"))
	rm := result.(Model)
	assert.LessOrEqual(t, rm.finalizerSearch.cursor, 2)
}

func TestCovFinalizerKeyCtrlB(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.cursor = 2
	result, _ := m.handleFinalizerSearchKey(keyMsg("ctrl+b"))
	rm := result.(Model)
	assert.GreaterOrEqual(t, rm.finalizerSearch.cursor, 0)
}

func TestCovFinalizerKeySpace(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.cursor = 0
	result, _ := m.handleFinalizerSearchKey(keyMsg(" "))
	rm := result.(Model)
	// Should have toggled selection on first item
	k := finalizerMatchKey(rm.finalizerSearch.results[0])
	assert.True(t, rm.finalizerSearch.selected[k])
}

func TestCovFinalizerKeySpaceDeselect(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.cursor = 0
	k := finalizerMatchKey(m.finalizerSearch.results[0])
	m.finalizerSearch.selected[k] = true
	result, _ := m.handleFinalizerSearchKey(keyMsg(" "))
	rm := result.(Model)
	assert.False(t, rm.finalizerSearch.selected[k])
}

func TestCovFinalizerKeyCtrlA(t *testing.T) {
	m := baseModelFinalizer()
	result, _ := m.handleFinalizerSearchKey(keyMsg("ctrl+a"))
	rm := result.(Model)
	// Should have selected all
	for _, match := range rm.finalizerSearch.results {
		assert.True(t, rm.finalizerSearch.selected[finalizerMatchKey(match)])
	}
}

func TestCovFinalizerKeyCtrlADeselectAll(t *testing.T) {
	m := baseModelFinalizer()
	for _, match := range m.finalizerSearch.results {
		m.finalizerSearch.selected[finalizerMatchKey(match)] = true
	}
	result, _ := m.handleFinalizerSearchKey(keyMsg("ctrl+a"))
	rm := result.(Model)
	assert.Empty(t, rm.finalizerSearch.selected)
}

func TestCovFinalizerKeyEnterNoSelection(t *testing.T) {
	m := baseModelFinalizer()
	_, cmd := m.handleFinalizerSearchKey(keyMsg("enter"))
	assert.NotNil(t, cmd) // scheduleStatusClear
}

func TestCovFinalizerKeyEnterWithSelection(t *testing.T) {
	m := baseModelFinalizer()
	k := finalizerMatchKey(m.finalizerSearch.results[0])
	m.finalizerSearch.selected[k] = true
	result, _ := m.handleFinalizerSearchKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayConfirmType, rm.overlay)
	assert.Equal(t, "Finalizer Remove", rm.pendingAction)
}

func TestCovFinalizerKeySlash(t *testing.T) {
	m := baseModelFinalizer()
	result, _ := m.handleFinalizerSearchKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.finalizerSearch.filterActive)
}

func TestCovFinalizerFilterEscClearsFilter(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.filterActive = true
	m.finalizerSearch.filter = "test"
	result, _ := m.handleFinalizerSearchFilterKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Empty(t, rm.finalizerSearch.filter)
}

func TestCovFinalizerFilterEscClosesOverlay(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.filterActive = true
	m.finalizerSearch.filter = ""
	m.finalizerSearch.results = nil
	m.finalizerSearch.pattern = ""
	result, _ := m.handleFinalizerSearchFilterKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovFinalizerFilterEscDeactivatesFilter(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.filterActive = true
	m.finalizerSearch.filter = ""
	m.finalizerSearch.pattern = "something"
	result, _ := m.handleFinalizerSearchFilterKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.finalizerSearch.filterActive)
}

func TestCovFinalizerFilterEnterInitialSearch(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.filterActive = true
	m.finalizerSearch.results = nil
	m.finalizerSearch.pattern = ""
	m.finalizerSearch.filter = "pv-protection"
	_, cmd := m.handleFinalizerSearchFilterKey(keyMsg("enter"))
	assert.NotNil(t, cmd)
}

func TestCovFinalizerFilterEnterEmptyPattern(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.filterActive = true
	m.finalizerSearch.results = nil
	m.finalizerSearch.pattern = ""
	m.finalizerSearch.filter = ""
	result, _ := m.handleFinalizerSearchFilterKey(keyMsg("enter"))
	_ = result.(Model)
}

func TestCovFinalizerFilterEnterWithExistingResults(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.filterActive = true
	m.finalizerSearch.pattern = "existing"
	result, _ := m.handleFinalizerSearchFilterKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.finalizerSearch.filterActive)
}

func TestCovFinalizerFilterBackspace(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.filterActive = true
	m.finalizerSearch.filter = "abc"
	result, _ := m.handleFinalizerSearchFilterKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Equal(t, "ab", rm.finalizerSearch.filter)
}

func TestCovFinalizerFilterCtrlW(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.filterActive = true
	m.finalizerSearch.filter = "foo bar"
	result, _ := m.handleFinalizerSearchFilterKey(keyMsg("ctrl+w"))
	rm := result.(Model)
	assert.NotEqual(t, "foo bar", rm.finalizerSearch.filter)
}

func TestCovFinalizerFilterTyping(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.filterActive = true
	result, _ := m.handleFinalizerSearchFilterKey(keyMsg("x"))
	rm := result.(Model)
	assert.Equal(t, "x", rm.finalizerSearch.filter)
}

func TestCovFilteredFinalizerResultsNoFilter(t *testing.T) {
	m := baseModelFinalizer()
	results := m.filteredFinalizerResults()
	assert.Len(t, results, 3)
}

func TestCovFilteredFinalizerResultsWithFilter(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.filter = "kube-system"
	results := m.filteredFinalizerResults()
	assert.Len(t, results, 1)
	assert.Equal(t, "pod-3", results[0].Name)
}

func TestCovFinalizerSearchKeyDispatchToFilter(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearch.filterActive = true
	result, _ := m.handleFinalizerSearchKey(keyMsg("x"))
	rm := result.(Model)
	assert.Contains(t, rm.finalizerSearch.filter, "x")
}
