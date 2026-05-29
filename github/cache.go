package github

import (
	"bytes"

	lru "github.com/hashicorp/golang-lru/v2"
)

// defaultCacheSize bounds the in-memory RR archive cache. Each archive is on
// the order of a few MB, so 32 entries covers practical workflow sets without
// uncomfortable memory pressure.
const defaultCacheSize = 32

// NewLRUCache returns a thread-safe Cache backed by hashicorp/golang-lru/v2.
// Size <= 0 falls back to defaultCacheSize.
func NewLRUCache(size int) Cache {
	if size <= 0 {
		size = defaultCacheSize
	}
	c, _ := lru.New[string, []byte](size)
	return &lruCache{inner: c}
}

type lruCache struct{ inner *lru.Cache[string, []byte] }

// Get returns a copy of the cached value so the caller cannot mutate the
// cached archive bytes through the returned slice.
func (c *lruCache) Get(key string) ([]byte, bool) {
	v, ok := c.inner.Get(key)
	if !ok {
		return nil, false
	}
	return bytes.Clone(v), true
}

// Add stores a copy of value so a subsequent caller-side mutation can't
// silently corrupt the cached archive.
func (c *lruCache) Add(key string, value []byte) {
	c.inner.Add(key, bytes.Clone(value))
}
