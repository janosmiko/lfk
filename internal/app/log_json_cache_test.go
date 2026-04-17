package app

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLRUJSONCacheGetMiss(t *testing.T) {
	c := newLRUJSONCache(10)
	got, ok := c.Get(`{"a":1}`)
	assert.False(t, ok)
	assert.False(t, got.IsJSON)
	assert.Nil(t, got.Value)
	assert.Empty(t, got.Payload)
	assert.Equal(t, 0, c.Len())
}

func TestLRUJSONCachePutHit(t *testing.T) {
	c := newLRUJSONCache(10)
	line := `{"a":1}`
	j := DetectJSONLine(line)
	c.Put(line, j)

	got, ok := c.Get(line)
	require.True(t, ok)
	assert.Equal(t, j.IsJSON, got.IsJSON)
	assert.Equal(t, j.Payload, got.Payload)
	assert.NotNil(t, got.Value)
	assert.Equal(t, 1, c.Len())
}

func TestLRUJSONCachePutEvicts(t *testing.T) {
	c := newLRUJSONCache(3)
	lines := []string{"a", "b", "c", "d"}
	for _, l := range lines {
		c.Put(l, JSONLine{IsJSON: false})
	}
	assert.Equal(t, 3, c.Len(), "cap=3 means at most 3 entries")
	// Oldest key ("a") must have been evicted.
	_, ok := c.Get("a")
	assert.False(t, ok, "oldest entry evicted")
	// Remaining should still be present.
	for _, l := range []string{"b", "c", "d"} {
		_, ok := c.Get(l)
		assert.True(t, ok, "key %q expected present after eviction", l)
	}
}

func TestLRUJSONCachePromotesOnGet(t *testing.T) {
	c := newLRUJSONCache(3)
	c.Put("a", JSONLine{})
	c.Put("b", JSONLine{})
	c.Put("c", JSONLine{})
	// Access "a" so it becomes most-recently-used.
	_, ok := c.Get("a")
	require.True(t, ok)
	// Inserting a 4th entry should now evict "b" (the new LRU), not "a".
	c.Put("d", JSONLine{})
	_, aPresent := c.Get("a")
	assert.True(t, aPresent, "'a' must survive eviction after being promoted")
	_, bPresent := c.Get("b")
	assert.False(t, bPresent, "'b' should have been evicted")
}

func TestLRUJSONCachePutExistingPromotes(t *testing.T) {
	c := newLRUJSONCache(3)
	c.Put("a", JSONLine{})
	c.Put("b", JSONLine{})
	c.Put("c", JSONLine{})
	// Re-Put "a" — should NOT grow size, should promote.
	c.Put("a", JSONLine{IsJSON: true, Payload: "updated"})
	assert.Equal(t, 3, c.Len(), "Put on existing key must not grow size")
	got, ok := c.Get("a")
	require.True(t, ok)
	assert.Equal(t, "updated", got.Payload, "value should be overwritten on re-Put")

	// Next Put should evict "b", not "a".
	c.Put("d", JSONLine{})
	_, aPresent := c.Get("a")
	assert.True(t, aPresent, "'a' promoted by re-Put must survive eviction")
	_, bPresent := c.Get("b")
	assert.False(t, bPresent, "'b' should have been evicted")
}

func TestLRUJSONCacheLen(t *testing.T) {
	c := newLRUJSONCache(100)
	assert.Equal(t, 0, c.Len())
	c.Put("a", JSONLine{})
	assert.Equal(t, 1, c.Len())
	c.Put("b", JSONLine{})
	assert.Equal(t, 2, c.Len())
	c.Put("a", JSONLine{}) // re-put, no growth
	assert.Equal(t, 2, c.Len())
}

func TestLRUJSONCacheZeroCapClamped(t *testing.T) {
	// Non-positive cap must not result in a no-op cache; we clamp to 1.
	for _, cap := range []int{0, -5} {
		c := newLRUJSONCache(cap)
		c.Put("a", JSONLine{IsJSON: true})
		assert.Equal(t, 1, c.Len())
		got, ok := c.Get("a")
		require.True(t, ok, "clamped cap=1 should still hold one entry")
		assert.True(t, got.IsJSON)
	}
}

// TestLRUJSONCacheConcurrent stresses the mutex with parallel Get/Put.
// Passing under -race is the real assertion.
func TestLRUJSONCacheConcurrent(t *testing.T) {
	c := newLRUJSONCache(100)
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := range 500 {
				key := fmt.Sprintf("k-%d-%d", i, j%50)
				c.Put(key, JSONLine{IsJSON: j%2 == 0})
				_, _ = c.Get(key)
			}
		}(i)
	}
	wg.Wait()
	// Cache must not have exceeded cap.
	assert.LessOrEqual(t, c.Len(), 100, "cache must never exceed cap")
}
