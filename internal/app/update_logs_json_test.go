package app

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStreamedLineEntersJSONCache drives a single streamed line through
// updateLogLine and verifies it lands in the JSON cache with the right
// detection result. Covers both a JSON line and a plain-text line so we
// know the cache stores negative results too (otherwise jsonLineAt would
// re-parse plain lines every time).
func TestStreamedLineEntersJSONCache(t *testing.T) {
	cases := []struct {
		name       string
		line       string
		wantIsJSON bool
	}{
		{"json object", `{"level":"info","msg":"hi"}`, true},
		{"pod-prefixed json", `[api/web] {"a":1}`, true},
		{"plain text", `hello world`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ch := make(chan string, 1)
			m := Model{
				tabs:         []TabState{{}},
				logJSONCache: newLRUJSONCache(100),
				logCh:        ch,
			}
			got, _ := m.updateLogLine(logLineMsg{line: c.line, ch: ch})
			out := got.(Model)

			// The line should be appended.
			require.Len(t, out.logLines, 1)
			assert.Equal(t, c.line, out.logLines[0])

			// The cache should now hold the detection result for the line.
			cached, ok := out.logJSONCache.Get(c.line)
			require.True(t, ok, "cache should hold streamed line")
			assert.Equal(t, c.wantIsJSON, cached.IsJSON)

			// jsonLineAt should return the cached result.
			via := out.jsonLineAt(len(out.logLines) - 1)
			assert.Equal(t, c.wantIsJSON, via.IsJSON)
		})
	}
}

// TestHistoryPrependWarmsJSONCache verifies updateLogHistory seeds the
// cache with the prepended older lines so the slow path doesn't leave
// filter/render consumers staring at a cold cache.
func TestHistoryPrependWarmsJSONCache(t *testing.T) {
	m := Model{
		mode:         modeLogs,
		tabs:         []TabState{{}},
		logJSONCache: newLRUJSONCache(100),
		logLines:     []string{`{"new":1}`, `{"new":2}`, `{"new":3}`},
	}
	historical := []string{
		`{"old":1}`,
		`plain old log line`,
		`{"old":2}`,
	}
	// updateLogHistory expects a msg whose lines include >=3-line overlap
	// with the current logLines so the prepend path uses the overlap idx.
	// Pass the new lines as the tail of msg.lines so overlap matches.
	msg := logHistoryMsg{lines: append(append([]string{}, historical...), m.logLines...)}

	got := m.updateLogHistory(msg)

	// Ensure historical lines were actually prepended.
	require.Len(t, got.logLines, len(historical)+3)
	assert.Equal(t, `{"old":1}`, got.logLines[0])
	assert.Equal(t, `plain old log line`, got.logLines[1])

	// All historical lines should be in the cache.
	for _, line := range historical {
		cached, ok := got.logJSONCache.Get(line)
		require.True(t, ok, "cache should hold prepended line %q", line)
		// JSON-shaped lines should be flagged; plain text should not.
		wantIsJSON := line != `plain old log line`
		assert.Equal(t, wantIsJSON, cached.IsJSON, "IsJSON for %q", line)
	}
}

// TestJSONLineAtMissComputesAndCaches confirms jsonLineAt doubles as a
// lazy memoiser when the cache is cold (callers who build a Model
// directly and bypass the stream path should still get correct results).
func TestJSONLineAtMissComputesAndCaches(t *testing.T) {
	m := Model{
		logJSONCache: newLRUJSONCache(100),
		logLines:     []string{`{"k":"v"}`, `plain`},
	}
	// Cache starts empty.
	assert.Equal(t, 0, m.logJSONCache.Len())

	// Miss: compute and cache.
	j0 := m.jsonLineAt(0)
	assert.True(t, j0.IsJSON)

	j1 := m.jsonLineAt(1)
	assert.False(t, j1.IsJSON)

	assert.Equal(t, 2, m.logJSONCache.Len(),
		"misses should have populated the cache")

	// Hit: returns the same value without re-adding.
	j0again := m.jsonLineAt(0)
	assert.Equal(t, j0.IsJSON, j0again.IsJSON)
	assert.Equal(t, 2, m.logJSONCache.Len())
}

// TestJSONLineAtOutOfRangeReturnsZero guards against panic when callers
// pass an invalid cursor position (common during filter-mode projection
// where the cursor may transiently reference a stale index).
func TestJSONLineAtOutOfRangeReturnsZero(t *testing.T) {
	m := Model{
		logJSONCache: newLRUJSONCache(100),
		logLines:     []string{`{"k":1}`},
	}
	assert.False(t, m.jsonLineAt(-1).IsJSON)
	assert.False(t, m.jsonLineAt(1).IsJSON)
	assert.False(t, m.jsonLineAt(999).IsJSON)
}

// TestJSONCacheCapOnLargeStream streams > cap lines through the cache
// and asserts the size is capped exactly at the configured limit.
// This is the Phase 4A deliverable's "cache sanity" check.
func TestJSONCacheCapOnLargeStream(t *testing.T) {
	const cap = 200
	const streamed = 500
	c := newLRUJSONCache(cap)
	for i := range streamed {
		line := fmt.Sprintf(`{"i":%d}`, i)
		c.Put(line, DetectJSONLine(line))
	}
	assert.Equal(t, cap, c.Len(),
		"cache must not grow past cap regardless of stream size")
}

// TestJSONCacheUnderCapHoldsAll confirms the cache holds every entry
// when the stream is smaller than cap.
func TestJSONCacheUnderCapHoldsAll(t *testing.T) {
	const cap = 500
	const streamed = 200
	c := newLRUJSONCache(cap)
	for i := range streamed {
		line := fmt.Sprintf(`{"i":%d}`, i)
		c.Put(line, DetectJSONLine(line))
	}
	assert.Equal(t, streamed, c.Len(),
		"below-cap streams must retain every entry")
}
