// Package ch09cache ports Chapter 9 of the PropEr Testing book: a bounded
// cache tested as a state machine, first sequentially and then in parallel,
// where the parallel test exposes a race in a write path that is not
// atomic.
package ch09cache

import (
	"runtime"
	"sync"
)

// Cache keeps at most max entries. When it is full, a new key overwrites the
// oldest entry; writing an existing key keeps its position. Flush empties
// it.
type Cache struct {
	mu      sync.Mutex
	max     int
	entries []entry
	racy    bool
}

type entry struct{ key, val int }

// New returns an empty cache of the given capacity.
func New(max int) *Cache { return &Cache{max: max} }

// NewRacy returns a cache whose Put reads and writes the entry list in two
// separate critical sections, the way the chapter's cache let every process
// write the ETS table directly. Two concurrent writers can then both see
// room for one more entry.
func NewRacy(max int) *Cache { return &Cache{max: max, racy: true} }

// Find returns the value stored for key.
func (c *Cache) Find(key int) (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.entries {
		if e.key == key {
			return e.val, true
		}
	}
	return 0, false
}

// Put stores a value for key.
func (c *Cache) Put(key, val int) {
	if !c.racy {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.put(key, val)
		return
	}
	c.mu.Lock()
	idx, full := c.index(key), len(c.entries) >= c.max
	snapshot := append([]entry(nil), c.entries...)
	c.mu.Unlock()
	runtime.Gosched() // the window in which another writer changes the entries
	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case idx >= 0:
		snapshot[idx].val = val
	case full:
		snapshot = append(snapshot[1:], entry{key, val})
	default:
		snapshot = append(snapshot, entry{key, val})
	}
	c.entries = snapshot
}

func (c *Cache) put(key, val int) {
	if idx := c.index(key); idx >= 0 {
		c.entries[idx].val = val
		return
	}
	if len(c.entries) >= c.max {
		c.entries = c.entries[1:]
	}
	c.entries = append(c.entries, entry{key, val})
}

func (c *Cache) index(key int) int {
	for i, e := range c.entries {
		if e.key == key {
			return i
		}
	}
	return -1
}

// Flush empties the cache.
func (c *Cache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = nil
}
