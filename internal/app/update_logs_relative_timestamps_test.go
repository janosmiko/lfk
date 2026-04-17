package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestShiftRTogglesRelativeTimestamps exercises the happy path: when
// timestamps are already showing (press `s` first), pressing `R`
// flips logRelativeTimestamps without touching the stream, the
// absolute-timestamps flag, or anything else.
func TestShiftRTogglesRelativeTimestamps(t *testing.T) {
	m := Model{
		mode:          modeLogs,
		logLines:      []string{"2026-04-16T10:00:30Z hello"},
		logTimestamps: true,
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	// First press: off → on.
	ret, _ := m.handleLogKey(runeKey('R'))
	result := ret.(Model)
	assert.True(t, result.logRelativeTimestamps,
		"R should flip logRelativeTimestamps from false to true")
	assert.True(t, result.logTimestamps,
		"R must not touch logTimestamps")
	assert.Contains(t, result.statusMessage, "relative timestamps: on")

	// Second press: on → off.
	ret2, _ := result.handleLogKey(runeKey('R'))
	result2 := ret2.(Model)
	assert.False(t, result2.logRelativeTimestamps,
		"R should flip logRelativeTimestamps back from true to false")
	assert.Contains(t, result2.statusMessage, "relative timestamps: off")
}

// TestShiftRIsNoopWhenTimestampsHidden confirms the "enable timestamps
// first" guard fires when the user presses `R` while logTimestamps is
// off: the relative flag must stay at its zero value, and the user
// must see a hint directing them to press `s`.
func TestShiftRIsNoopWhenTimestampsHidden(t *testing.T) {
	m := Model{
		mode:                  modeLogs,
		logLines:              []string{"some plain log"},
		logTimestamps:         false,
		logRelativeTimestamps: false,
		tabs:                  []TabState{{}},
		width:                 80,
		height:                40,
	}
	ret, _ := m.handleLogKey(runeKey('R'))
	result := ret.(Model)

	assert.False(t, result.logRelativeTimestamps,
		"R must not flip the relative flag when timestamps are hidden")
	assert.False(t, result.logTimestamps,
		"R must not implicitly enable timestamps")
	assert.Contains(t, result.statusMessage, "press s",
		"status should tell the user to enable timestamps first")
}

// TestShiftRPreservesRelativeWhenTimestampsHidden is the symmetric
// guard: if relative is already on and the user toggles timestamps
// off via `s`, pressing `R` should NOT clear the relative flag. The
// flag is orthogonal — it only takes effect when timestamps are also
// on — so it should survive a timestamps round-trip.
func TestShiftRPreservesRelativeWhenTimestampsHidden(t *testing.T) {
	m := Model{
		mode:                  modeLogs,
		logLines:              []string{"2026-04-16T10:00:30Z hello"},
		logTimestamps:         false,
		logRelativeTimestamps: true,
		tabs:                  []TabState{{}},
		width:                 80,
		height:                40,
	}
	ret, _ := m.handleLogKey(runeKey('R'))
	result := ret.(Model)

	assert.True(t, result.logRelativeTimestamps,
		"R no-op must not reset the relative flag when timestamps are hidden")
}
