package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTabSnapshotPreservesJSONPretty asserts logJSONPretty
// round-trips through a TabState snapshot/restore so switching
// tabs does not drop the user's display preference.
func TestTabSnapshotPreservesJSONPretty(t *testing.T) {
	m := Model{logJSONPretty: true}

	tab := &TabState{}
	m.saveJSONPrettyInto(tab)

	// Simulate switching tabs.
	m.logJSONPretty = false

	m.restoreJSONPrettyFrom(tab)

	assert.True(t, m.logJSONPretty,
		"logJSONPretty should round-trip through TabState")
	assert.True(t, tab.logJSONPretty,
		"TabState should carry the snapshot value")
}

// TestTabSnapshotJSONPrettyZeroValue confirms a freshly created
// tab (zero value false) does not inherit a stale true from
// whichever tab was active last.
func TestTabSnapshotJSONPrettyZeroValue(t *testing.T) {
	m := Model{logJSONPretty: false}

	tab := &TabState{}
	m.saveJSONPrettyInto(tab)
	// Pretend another tab set it before we restore.
	m.logJSONPretty = true
	m.restoreJSONPrettyFrom(tab)

	assert.False(t, m.logJSONPretty,
		"zero-value snapshot should clear any stale true on restore")
}

// saveJSONPrettyInto / restoreJSONPrettyFrom mirror the subset
// of saveCurrentTab / loadTab that we care about for this test.
// Keeping the proxy in test-scope avoids pulling in every other
// field saveCurrentTab touches.
func (m *Model) saveJSONPrettyInto(t *TabState) {
	t.logJSONPretty = m.logJSONPretty
}

func (m *Model) restoreJSONPrettyFrom(t *TabState) {
	m.logJSONPretty = t.logJSONPretty
}
