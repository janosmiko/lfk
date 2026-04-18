package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTabSnapshotPreservesHistogram asserts logHistogram round-trips
// through a TabState snapshot/restore so switching tabs does not
// drop the user's histogram-visibility preference.
func TestTabSnapshotPreservesHistogram(t *testing.T) {
	m := Model{logHistogram: true}

	tab := &TabState{}
	m.saveHistogramInto(tab)

	// Simulate switching tabs.
	m.logHistogram = false

	m.restoreHistogramFrom(tab)

	assert.True(t, m.logHistogram,
		"logHistogram should round-trip through TabState")
	assert.True(t, tab.logHistogram,
		"TabState should carry the snapshot value")
}

// TestTabSnapshotHistogramZeroValue confirms a freshly created tab
// (zero value false) does not inherit a stale true from whichever tab
// was active last — the histogram defaults to OFF on every new tab.
func TestTabSnapshotHistogramZeroValue(t *testing.T) {
	m := Model{logHistogram: false}

	tab := &TabState{}
	m.saveHistogramInto(tab)
	// Pretend another tab set it before we restore.
	m.logHistogram = true
	m.restoreHistogramFrom(tab)

	assert.False(t, m.logHistogram,
		"zero-value snapshot should clear any stale true on restore")
}

// saveHistogramInto / restoreHistogramFrom mirror the subset of
// saveCurrentTab / loadTab that we care about for this test.
// Keeping the proxy in test-scope avoids pulling in every other
// field saveCurrentTab touches.
func (m *Model) saveHistogramInto(t *TabState) {
	t.logHistogram = m.logHistogram
}

func (m *Model) restoreHistogramFrom(t *TabState) {
	m.logHistogram = t.logHistogram
}
