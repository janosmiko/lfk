package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCovOpenFinalizerSearch(t *testing.T) {
	m := baseModelCov()
	m.openFinalizerSearch()

	assert.Equal(t, overlayFinalizerSearch, m.overlay)
	assert.True(t, m.finalizerSearchFilterActive)
	assert.False(t, m.finalizerSearchLoading)
	assert.Empty(t, m.finalizerSearchPattern)
	assert.Empty(t, m.finalizerSearchResults)
	assert.NotNil(t, m.finalizerSearchSelected)
	assert.Equal(t, 0, m.finalizerSearchCursor)
}

func TestCovFinalizerKeyEsc(t *testing.T) {
	m := baseModelFinalizer()
	result, _ := m.handleFinalizerSearchKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.Nil(t, rm.finalizerSearchResults)
}

func TestCovFinalizerKeyDown(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchCursor = 0
	result, _ := m.handleFinalizerSearchKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.finalizerSearchCursor)
}

func TestCovFinalizerKeyUp(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchCursor = 2
	result, _ := m.handleFinalizerSearchKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.finalizerSearchCursor)
}

func TestCovFinalizerKeyGG(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchCursor = 2
	result, _ := m.handleFinalizerSearchKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)
	result, _ = rm.handleFinalizerSearchKey(keyMsg("g"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.finalizerSearchCursor)
}

func TestCovFinalizerKeyBigG(t *testing.T) {
	m := baseModelFinalizer()
	result, _ := m.handleFinalizerSearchKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.finalizerSearchCursor)
}

func TestCovFinalizerKeyCtrlD(t *testing.T) {
	m := baseModelFinalizer()
	result, _ := m.handleFinalizerSearchKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.LessOrEqual(t, rm.finalizerSearchCursor, 2)
}

func TestCovFinalizerKeyCtrlU(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchCursor = 2
	result, _ := m.handleFinalizerSearchKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.GreaterOrEqual(t, rm.finalizerSearchCursor, 0)
}

func TestCovFinalizerKeyCtrlF(t *testing.T) {
	m := baseModelFinalizer()
	result, _ := m.handleFinalizerSearchKey(keyMsg("ctrl+f"))
	rm := result.(Model)
	assert.LessOrEqual(t, rm.finalizerSearchCursor, 2)
}

func TestCovFinalizerKeyCtrlB(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchCursor = 2
	result, _ := m.handleFinalizerSearchKey(keyMsg("ctrl+b"))
	rm := result.(Model)
	assert.GreaterOrEqual(t, rm.finalizerSearchCursor, 0)
}

func TestCovFinalizerKeySpace(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchCursor = 0
	result, _ := m.handleFinalizerSearchKey(keyMsg(" "))
	rm := result.(Model)
	// Should have toggled selection on first item
	k := finalizerMatchKey(rm.finalizerSearchResults[0])
	assert.True(t, rm.finalizerSearchSelected[k])
}

func TestCovFinalizerKeySpaceDeselect(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchCursor = 0
	k := finalizerMatchKey(m.finalizerSearchResults[0])
	m.finalizerSearchSelected[k] = true
	result, _ := m.handleFinalizerSearchKey(keyMsg(" "))
	rm := result.(Model)
	assert.False(t, rm.finalizerSearchSelected[k])
}

func TestCovFinalizerKeyCtrlA(t *testing.T) {
	m := baseModelFinalizer()
	result, _ := m.handleFinalizerSearchKey(keyMsg("ctrl+a"))
	rm := result.(Model)
	// Should have selected all
	for _, match := range rm.finalizerSearchResults {
		assert.True(t, rm.finalizerSearchSelected[finalizerMatchKey(match)])
	}
}

func TestCovFinalizerKeyCtrlADeselectAll(t *testing.T) {
	m := baseModelFinalizer()
	for _, match := range m.finalizerSearchResults {
		m.finalizerSearchSelected[finalizerMatchKey(match)] = true
	}
	result, _ := m.handleFinalizerSearchKey(keyMsg("ctrl+a"))
	rm := result.(Model)
	assert.Empty(t, rm.finalizerSearchSelected)
}

func TestCovFinalizerKeyEnterNoSelection(t *testing.T) {
	m := baseModelFinalizer()
	_, cmd := m.handleFinalizerSearchKey(keyMsg("enter"))
	assert.NotNil(t, cmd) // scheduleStatusClear
}

func TestCovFinalizerKeyEnterWithSelection(t *testing.T) {
	m := baseModelFinalizer()
	k := finalizerMatchKey(m.finalizerSearchResults[0])
	m.finalizerSearchSelected[k] = true
	result, _ := m.handleFinalizerSearchKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayConfirmType, rm.overlay)
	assert.Equal(t, "Finalizer Remove", rm.pendingAction)
}

func TestCovFinalizerKeySlash(t *testing.T) {
	m := baseModelFinalizer()
	result, _ := m.handleFinalizerSearchKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.finalizerSearchFilterActive)
}

func TestCovFinalizerFilterEscClearsFilter(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchFilterActive = true
	m.finalizerSearchFilter = "test"
	result, _ := m.handleFinalizerSearchFilterKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Empty(t, rm.finalizerSearchFilter)
}

func TestCovFinalizerFilterEscClosesOverlay(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchFilterActive = true
	m.finalizerSearchFilter = ""
	m.finalizerSearchResults = nil
	m.finalizerSearchPattern = ""
	result, _ := m.handleFinalizerSearchFilterKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovFinalizerFilterEscDeactivatesFilter(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchFilterActive = true
	m.finalizerSearchFilter = ""
	m.finalizerSearchPattern = "something"
	result, _ := m.handleFinalizerSearchFilterKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.finalizerSearchFilterActive)
}

func TestCovFinalizerFilterEnterInitialSearch(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchFilterActive = true
	m.finalizerSearchResults = nil
	m.finalizerSearchPattern = ""
	m.finalizerSearchFilter = "pv-protection"
	_, cmd := m.handleFinalizerSearchFilterKey(keyMsg("enter"))
	assert.NotNil(t, cmd)
}

func TestCovFinalizerFilterEnterEmptyPattern(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchFilterActive = true
	m.finalizerSearchResults = nil
	m.finalizerSearchPattern = ""
	m.finalizerSearchFilter = ""
	result, _ := m.handleFinalizerSearchFilterKey(keyMsg("enter"))
	_ = result.(Model)
}

func TestCovFinalizerFilterEnterWithExistingResults(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchFilterActive = true
	m.finalizerSearchPattern = "existing"
	result, _ := m.handleFinalizerSearchFilterKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.finalizerSearchFilterActive)
}

func TestCovFinalizerFilterBackspace(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchFilterActive = true
	m.finalizerSearchFilter = "abc"
	result, _ := m.handleFinalizerSearchFilterKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Equal(t, "ab", rm.finalizerSearchFilter)
}

func TestCovFinalizerFilterCtrlW(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchFilterActive = true
	m.finalizerSearchFilter = "foo bar"
	result, _ := m.handleFinalizerSearchFilterKey(keyMsg("ctrl+w"))
	rm := result.(Model)
	assert.NotEqual(t, "foo bar", rm.finalizerSearchFilter)
}

func TestCovFinalizerFilterTyping(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchFilterActive = true
	result, _ := m.handleFinalizerSearchFilterKey(keyMsg("x"))
	rm := result.(Model)
	assert.Equal(t, "x", rm.finalizerSearchFilter)
}

func TestCovFilteredFinalizerResultsNoFilter(t *testing.T) {
	m := baseModelFinalizer()
	results := m.filteredFinalizerResults()
	assert.Len(t, results, 3)
}

func TestCovFilteredFinalizerResultsWithFilter(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchFilter = "kube-system"
	results := m.filteredFinalizerResults()
	assert.Len(t, results, 1)
	assert.Equal(t, "pod-3", results[0].Name)
}

func TestCovFinalizerSearchKeyDispatchToFilter(t *testing.T) {
	m := baseModelFinalizer()
	m.finalizerSearchFilterActive = true
	result, _ := m.handleFinalizerSearchKey(keyMsg("x"))
	rm := result.(Model)
	assert.Contains(t, rm.finalizerSearchFilter, "x")
}
