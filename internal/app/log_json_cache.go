package app

import (
	"hash/fnv"
	"sync"
)

// defaultJSONCacheCap is the default size of the log-line JSON detection
// cache. 5000 balances memory (~a few hundred KB for typical entries)
// against the common "rolling window of recent log lines" access pattern.
const defaultJSONCacheCap = 5000

// lruJSONCacheNode is a doubly-linked-list node holding one cache entry.
type lruJSONCacheNode struct {
	key        uint64
	value      JSONLine
	prev, next *lruJSONCacheNode
}

// lruJSONCache is a fixed-capacity LRU cache mapping raw log lines to
// their detected JSONLine. Keys are fnv64a hashes of the line so the
// cache survives slice reslicing of the underlying log buffer. Entries
// roll out naturally via LRU eviction when the buffer grows past cap.
//
// The cache is safe for concurrent use: every Get/Put takes the mutex.
// In practice almost all access is from the TUI main goroutine, but
// background log-stream goroutines may race on history prepend, so the
// mutex is non-negotiable.
type lruJSONCache struct {
	mu         sync.Mutex
	cap        int
	entries    map[uint64]*lruJSONCacheNode
	head, tail *lruJSONCacheNode
}

// newLRUJSONCache returns an empty cache with the given capacity. A
// non-positive cap is silently clamped to 1 so callers never get a
// "no-op" cache; JSON detection is on the hot path and a zero-cap
// cache would re-parse every line.
func newLRUJSONCache(cap int) *lruJSONCache {
	if cap < 1 {
		cap = 1
	}
	return &lruJSONCache{
		cap:     cap,
		entries: make(map[uint64]*lruJSONCacheNode, cap),
	}
}

// Get returns the cached JSONLine for line, or (JSONLine{}, false) on
// miss. On hit, the entry is promoted to the head of the LRU list.
func (c *lruJSONCache) Get(line string) (JSONLine, bool) {
	key := hashLine(line)
	c.mu.Lock()
	defer c.mu.Unlock()
	n, ok := c.entries[key]
	if !ok {
		return JSONLine{}, false
	}
	c.moveToHead(n)
	return n.value, true
}

// Put inserts or updates the cache entry for line. When the cache is
// over capacity, the least-recently-used entry is evicted.
func (c *lruJSONCache) Put(line string, v JSONLine) {
	key := hashLine(line)
	c.mu.Lock()
	defer c.mu.Unlock()
	if n, ok := c.entries[key]; ok {
		n.value = v
		c.moveToHead(n)
		return
	}
	n := &lruJSONCacheNode{key: key, value: v}
	c.entries[key] = n
	c.pushHead(n)
	if len(c.entries) > c.cap {
		c.evictTail()
	}
}

// Len returns the number of entries currently in the cache.
func (c *lruJSONCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// pushHead inserts n at the head of the linked list. Caller holds mu.
func (c *lruJSONCache) pushHead(n *lruJSONCacheNode) {
	n.prev = nil
	n.next = c.head
	if c.head != nil {
		c.head.prev = n
	}
	c.head = n
	if c.tail == nil {
		c.tail = n
	}
}

// moveToHead unlinks n from its current position and places it at head.
// Caller holds mu.
func (c *lruJSONCache) moveToHead(n *lruJSONCacheNode) {
	if n == c.head {
		return
	}
	// Detach n.
	if n.prev != nil {
		n.prev.next = n.next
	}
	if n.next != nil {
		n.next.prev = n.prev
	}
	if n == c.tail {
		c.tail = n.prev
	}
	// Insert at head.
	n.prev = nil
	n.next = c.head
	if c.head != nil {
		c.head.prev = n
	}
	c.head = n
	if c.tail == nil {
		c.tail = n
	}
}

// evictTail removes the least-recently-used entry. Caller holds mu.
func (c *lruJSONCache) evictTail() {
	if c.tail == nil {
		return
	}
	victim := c.tail
	if victim.prev != nil {
		victim.prev.next = nil
	}
	c.tail = victim.prev
	if c.head == victim {
		c.head = nil
	}
	delete(c.entries, victim.key)
}

// hashLine computes the fnv64a hash of line. Collisions are theoretically
// possible but extremely unlikely for log-line sized strings; a collision
// would result in returning a stale JSONLine for a different line, which
// is a minor visual/filter glitch rather than a correctness bug.
func hashLine(line string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(line))
	return h.Sum64()
}

// warmJSONCache runs DetectJSONLine on line and stores the result in the
// model's JSON cache. Safe to call with a nil cache (no-op) so tests that
// build a bare Model without calling NewModel don't need to remember to
// initialise it.
func (m *Model) warmJSONCache(line string) {
	if m.logJSONCache == nil {
		return
	}
	m.logJSONCache.Put(line, DetectJSONLine(line))
}

// jsonLineAt returns the cached JSONLine for m.logLines[idx], computing
// and caching it on a cache miss. Returns a zero JSONLine when idx is
// out of range so callers can blindly pass a cursor position without
// bounds-checking.
//
// Later filter/render code paths call this instead of DetectJSONLine
// directly so they reuse the parse work done by the stream-append
// path; the cache is the memoisation layer for those consumers.
func (m *Model) jsonLineAt(idx int) JSONLine {
	if idx < 0 || idx >= len(m.logLines) {
		return JSONLine{}
	}
	line := m.logLines[idx]
	if m.logJSONCache == nil {
		return DetectJSONLine(line)
	}
	if v, ok := m.logJSONCache.Get(line); ok {
		return v
	}
	v := DetectJSONLine(line)
	m.logJSONCache.Put(line, v)
	return v
}
