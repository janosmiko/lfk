package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestJTogglesJSONPretty verifies Shift+J flips the per-tab
// logJSONPretty flag from false → true → false without touching
// the stream or any other rendering flag. The status message must
// reflect the new state so the user gets feedback.
func TestJTogglesJSONPretty(t *testing.T) {
	m := Model{
		mode:     modeLogs,
		logLines: []string{`{"a":1}`},
		tabs:     []TabState{{}},
		width:    80,
		height:   40,
	}

	// First press: off → on.
	ret, _ := m.handleLogKey(runeKey('J'))
	result := ret.(Model)
	assert.True(t, result.logJSONPretty,
		"J should flip logJSONPretty from false to true")
	assert.Contains(t, result.statusMessage, "JSON pretty-print: on")

	// Second press: on → off.
	ret2, _ := result.handleLogKey(runeKey('J'))
	result2 := ret2.(Model)
	assert.False(t, result2.logJSONPretty,
		"J should flip logJSONPretty back to false")
	assert.Contains(t, result2.statusMessage, "JSON pretty-print: off")
}

// TestJDoesNotDisturbOtherLogFlags guards against flag creep: the
// JSON-pretty toggle must be orthogonal to timestamps, relative
// timestamps, wrap, line numbers, and the stream state.
func TestJDoesNotDisturbOtherLogFlags(t *testing.T) {
	m := Model{
		mode:                  modeLogs,
		logLines:              []string{`{"a":1}`},
		logTimestamps:         true,
		logRelativeTimestamps: true,
		logWrap:               true,
		logLineNumbers:        true,
		logFollow:             true,
		tabs:                  []TabState{{}},
		width:                 80,
		height:                40,
	}
	ret, _ := m.handleLogKey(runeKey('J'))
	result := ret.(Model)
	assert.True(t, result.logJSONPretty)
	assert.True(t, result.logTimestamps, "logTimestamps must not change")
	assert.True(t, result.logRelativeTimestamps, "logRelativeTimestamps must not change")
	assert.True(t, result.logWrap, "logWrap must not change")
	assert.True(t, result.logLineNumbers, "logLineNumbers must not change")
	assert.True(t, result.logFollow, "logFollow must not change")
}
