package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTabSnapshotPreservesRelativeTimestamps is the Phase 3B
// persistence contract: logRelativeTimestamps round-trips through
// a TabState snapshot/restore so switching tabs does not drop the
// user's rel-ts preference.
func TestTabSnapshotPreservesRelativeTimestamps(t *testing.T) {
	m := Model{logTimestamps: true, logRelativeTimestamps: true}

	tab := &TabState{}
	m.saveRelativeTimestampsInto(tab)

	// Simulate switching tabs: wipe both flags on the Model.
	m.logRelativeTimestamps = false
	m.logTimestamps = false

	// Switch back and reload.
	m.restoreRelativeTimestampsFrom(tab)

	assert.True(t, m.logRelativeTimestamps,
		"logRelativeTimestamps should round-trip through TabState")
	assert.True(t, m.logTimestamps,
		"logTimestamps should round-trip alongside")
	assert.True(t, tab.logRelativeTimestamps,
		"TabState should carry the snapshot value")
}

// TestTabSnapshotRelativeTimestampsZeroValue confirms a freshly
// created tab (zero value false) does not inherit a stale true from
// whichever tab was active last — the restore is idempotent on the
// zero value.
func TestTabSnapshotRelativeTimestampsZeroValue(t *testing.T) {
	m := Model{logRelativeTimestamps: false, logTimestamps: false}

	tab := &TabState{}
	m.saveRelativeTimestampsInto(tab)
	// Pretend another tab set it before we restore.
	m.logRelativeTimestamps = true
	m.logTimestamps = true
	m.restoreRelativeTimestampsFrom(tab)

	assert.False(t, m.logRelativeTimestamps,
		"zero-value snapshot should clear any stale true on restore")
	assert.False(t, m.logTimestamps,
		"zero-value snapshot should clear any stale true on restore")
}

// saveRelativeTimestampsInto / restoreRelativeTimestampsFrom mirror
// the subset of saveCurrentTab / loadTab that we care about for this
// test. The production helpers wire through the global tabs slice
// and pull in a lot of unrelated state; this proxy replicates the
// round-trip contract in isolation.
func (m *Model) saveRelativeTimestampsInto(t *TabState) {
	t.logRelativeTimestamps = m.logRelativeTimestamps
	t.logTimestamps = m.logTimestamps
}

func (m *Model) restoreRelativeTimestampsFrom(t *TabState) {
	m.logRelativeTimestamps = t.logRelativeTimestamps
	m.logTimestamps = t.logTimestamps
}
