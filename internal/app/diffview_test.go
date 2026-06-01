package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDiffViewStateCopy validates the deep-copy semantics tabs.go relies on:
// scalars carry over, and the matchLines/foldState slices are cloned so a copy
// never aliases the source's backing arrays.
func TestDiffViewStateCopy(t *testing.T) {
	src := diffViewState{
		left:         "a: 1",
		right:        "a: 2",
		leftName:     "before",
		rightName:    "after",
		scroll:       10,
		cursor:       4,
		cursorSide:   1,
		unified:      true,
		wrap:         true,
		lineNumbers:  true,
		lineInput:    "12",
		searchMode:   true,
		searchText:   TextInput{Value: "needle", Cursor: 6},
		searchQuery:  "needle",
		matchLines:   []int{1, 5, 9},
		matchIdx:     2,
		foldState:    []bool{true, false, true},
		visualMode:   true,
		visualType:   'V',
		visualStart:  3,
		visualCol:    7,
		visualCurCol: 8,
		scrollOption: 5,
	}

	cp := src.copy()
	assert.Equal(t, src, cp)
	assert.Equal(t, 'V', cp.visualType)

	cp.matchLines[0] = 99
	assert.Equal(t, 1, src.matchLines[0], "matchLines must be cloned")

	cp.foldState[0] = false
	assert.True(t, src.foldState[0], "foldState must be cloned")

	cp.foldState = append(cp.foldState, true)
	assert.Len(t, src.foldState, 3, "foldState slice header must be independent")
}

// TestDiffViewStateCopyNilFields ensures copy leaves nil slices nil (no spurious
// allocation) for a zero-value state.
func TestDiffViewStateCopyNilFields(t *testing.T) {
	cp := diffViewState{}.copy()
	assert.Nil(t, cp.matchLines)
	assert.Nil(t, cp.foldState)
}
