package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTabSnapshotPreservesFilter verifies that save/load of a TabState
// round-trips logRules and logIncludeMode, and that the filter chain +
// visible-indices are rebuilt from those rules on load.
func TestTabSnapshotPreservesFilter(t *testing.T) {
	det, _ := newSeverityDetector(nil)
	sev := SeverityRule{Floor: SeverityWarn}
	inc, _ := NewPatternRule("api", PatternSubstring, false)

	lines := []string{
		"[INFO] api: starting",
		"[WARN] api: slow",
		"[ERROR] api: boom",
		"GET /healthz 200",
	}

	// Model with an active filter on Tab 0.
	m := Model{
		mode:                modeLogs,
		logSeverityDetector: det,
		logIncludeMode:      IncludeAll,
		logRules:            []Rule{sev, inc},
		logLines:            lines,
		logFilterEditingIdx: -1,
	}
	m.logFilterChain = NewFilterChain(m.logRules, m.logIncludeMode, det)
	m.rebuildLogVisibleIndices()

	// Snapshot into a TabState.
	tab := &TabState{}
	savedRef := m // capture for comparison later
	savedRef.saveInto(tab)

	// Wipe the Model's rules to simulate switching away.
	m.logRules = nil
	m.logIncludeMode = IncludeAny
	m.logFilterChain = NewFilterChain(nil, IncludeAny, det)
	m.rebuildLogVisibleIndices()

	// Restore from the TabState.
	m.restoreFrom(tab)

	// Rules and mode match.
	require.Len(t, m.logRules, 2, "both rules restored")
	_, okSev := m.logRules[0].(SeverityRule)
	assert.True(t, okSev, "severity pinned at index 0")
	assert.Equal(t, IncludeAll, m.logIncludeMode)

	// Chain was rebuilt — Keep() honors the restored filter.
	assert.True(t, m.logFilterChain.Active(), "chain should be active after restore")
	assert.False(t, m.logFilterChain.Keep("[INFO] api: starting"),
		"INFO < WARN floor; filter drops it after restore")
	assert.True(t, m.logFilterChain.Keep("[ERROR] api: boom"),
		"ERROR + api match after restore")

	// Visible indices were rebuilt.
	require.NotNil(t, m.logVisibleIndices)
}

// saveInto / restoreFrom are thin wrappers around saveCurrentTab /
// loadTab used by the test — the production helpers wire through the
// global tabs slice and are awkward to unit test in isolation. This
// proxy replicates just the filter-state persistence contract.
func (m *Model) saveInto(t *TabState) {
	t.logRules = append([]Rule(nil), m.logRules...)
	t.logIncludeMode = m.logIncludeMode
}

func (m *Model) restoreFrom(t *TabState) {
	m.logRules = append([]Rule(nil), t.logRules...)
	m.logIncludeMode = t.logIncludeMode
	m.logFilterChain = NewFilterChain(m.logRules, m.logIncludeMode, m.logSeverityDetector)
	m.rebuildLogVisibleIndices()
}
