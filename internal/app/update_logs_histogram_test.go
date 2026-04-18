package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHTogglesHistogram verifies Shift+H flips the per-tab
// logHistogram flag from false → true → false. The toggle does NOT
// touch the stream or any other rendering flag, and the status
// message reflects the new state so the user has feedback.
func TestHTogglesHistogram(t *testing.T) {
	m := Model{
		mode:     modeLogs,
		logLines: []string{"2026-04-17T12:00:00Z hello"},
		tabs:     []TabState{{}},
		width:    80,
		height:   40,
	}

	// First press: off → on.
	ret, _ := m.handleLogKey(runeKey('H'))
	result := ret.(Model)
	assert.True(t, result.logHistogram,
		"H should flip logHistogram from false to true")
	assert.Contains(t, result.statusMessage, "histogram: on")

	// Second press: on → off.
	ret2, _ := result.handleLogKey(runeKey('H'))
	result2 := ret2.(Model)
	assert.False(t, result2.logHistogram,
		"H should flip logHistogram back to false")
	assert.Contains(t, result2.statusMessage, "histogram: off")
}

// TestHDoesNotDisturbOtherLogFlags guards against flag creep: the
// histogram toggle must be orthogonal to timestamps, relative
// timestamps, JSON pretty, wrap, line numbers, and the stream state.
func TestHDoesNotDisturbOtherLogFlags(t *testing.T) {
	m := Model{
		mode:                  modeLogs,
		logLines:              []string{"2026-04-17T12:00:00Z hello"},
		logTimestamps:         true,
		logRelativeTimestamps: true,
		logJSONPretty:         true,
		logWrap:               true,
		logLineNumbers:        true,
		logFollow:             true,
		tabs:                  []TabState{{}},
		width:                 80,
		height:                40,
	}
	ret, _ := m.handleLogKey(runeKey('H'))
	result := ret.(Model)

	assert.True(t, result.logHistogram, "H should turn the histogram on")
	assert.True(t, result.logTimestamps, "logTimestamps must not change")
	assert.True(t, result.logRelativeTimestamps, "logRelativeTimestamps must not change")
	assert.True(t, result.logJSONPretty, "logJSONPretty must not change")
	assert.True(t, result.logWrap, "logWrap must not change")
	assert.True(t, result.logLineNumbers, "logLineNumbers must not change")
	assert.True(t, result.logFollow, "logFollow must not change")
}
