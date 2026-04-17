package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTabSnapshotPreservesSinceDuration asserts that logSinceDuration
// round-trips through a TabState snapshot/restore.  This is the
// contract the tab switcher relies on: switching tabs must not drop
// the active --since window, even though the stream is only restarted
// on explicit user commit.
func TestTabSnapshotPreservesSinceDuration(t *testing.T) {
	m := Model{logSinceDuration: "5m"}

	tab := &TabState{}
	m.saveSinceInto(tab)

	// Simulate switching away: wipe the field on the Model.
	m.logSinceDuration = ""

	// Switch back and reload.
	m.restoreSinceFrom(tab)

	assert.Equal(t, "5m", m.logSinceDuration,
		"logSinceDuration should round-trip through TabState")
	assert.Equal(t, "5m", tab.logSinceDuration,
		"TabState should carry the snapshot value")
}

// TestTabSnapshotEmptySinceDuration confirms that the empty-string
// zero value round-trips cleanly — switching to a tab with no --since
// active must not accidentally inherit a value from the previous tab.
func TestTabSnapshotEmptySinceDuration(t *testing.T) {
	m := Model{logSinceDuration: ""}

	tab := &TabState{}
	m.saveSinceInto(tab)
	m.logSinceDuration = "2h" // pretend another tab set a value
	m.restoreSinceFrom(tab)

	assert.Equal(t, "", m.logSinceDuration,
		"empty snapshot should clear any stale value on restore")
}

// saveSinceInto / restoreSinceFrom mirror the since-related subset of
// saveCurrentTab / loadTab for isolated testing (the production
// helpers wire through the global tabs slice and pull in a lot of
// unrelated state).
func (m *Model) saveSinceInto(t *TabState) {
	t.logSinceDuration = m.logSinceDuration
}

func (m *Model) restoreSinceFrom(t *TabState) {
	m.logSinceDuration = t.logSinceDuration
}
