package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestYAMLViewStateCopy validates the deep-copy semantics that tabs.go will
// rely on once the YAML viewer state is extracted: scalars carry over, and
// the slices and map are cloned so a copied value never aliases the source's
// backing storage.
func TestYAMLViewStateCopy(t *testing.T) {
	src := yamlViewState{
		content:      "apiVersion: v1",
		scroll:       10,
		cursor:       3,
		scrollOption: 5,
		lineInput:    "12",
		searchMode:   true,
		searchText:   TextInput{Value: "needle", Cursor: 6},
		matchLines:   []int{1, 4, 9},
		matchIdx:     2,
		visualMode:   true,
		visualStart:  4,
		visualType:   'V',
		visualCol:    7,
		visualCurCol: 8,
		wrap:         true,
		sections:     []yamlSection{{}, {}},
		collapsed:    map[string]bool{"spec": true},
	}

	cp := src.copy()

	// Scalars and the TextInput value carry over by assignment.
	assert.Equal(t, src, cp)
	assert.Equal(t, 'V', cp.visualType)
	assert.Equal(t, "needle", cp.searchText.Value)

	// Mutating the copy's slices/map must not affect the source.
	cp.matchLines[0] = 99
	assert.Equal(t, 1, src.matchLines[0], "matchLines must be cloned")

	cp.collapsed["spec"] = false
	cp.collapsed["status"] = true
	assert.True(t, src.collapsed["spec"], "collapsed must be cloned")
	assert.NotContains(t, src.collapsed, "status")

	cp.sections = append(cp.sections, yamlSection{})
	assert.Len(t, src.sections, 2, "sections slice header must be independent")
}

// TestYAMLViewStateCopyNilFields ensures copy handles a zero-value state
// without allocating spurious non-nil slices, while still producing a usable
// (non-nil) collapsed map so callers can write to it without a nil check.
func TestYAMLViewStateCopyNilFields(t *testing.T) {
	cp := yamlViewState{}.copy()

	assert.Nil(t, cp.matchLines)
	assert.Nil(t, cp.sections)
	// copyMapStringBool returns an empty, ready-to-use map for nil input,
	// matching how loadTab rehydrates yamlCollapsed today.
	assert.NotNil(t, cp.collapsed)
	assert.Empty(t, cp.collapsed)
}
