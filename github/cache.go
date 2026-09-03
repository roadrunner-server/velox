package github

import (
	"bytes"

	lru "github.com/hashicorp/golang-lru/v2"
)

// defaultCacheSize bounds the in-memory archive cache; each archive holds a few MB.
const defaultCacheSize = 32

// NewLRUCache returns a thread-safe Cache; a size of 0 or less falls back to defaultCacheSize.
func NewLRUCache(size int) Cache {
	if size <= 0 {
		size = defaultCacheSize
	}
	c, _ := lru.New[string, []byte](size)
	return &lruCache{inner: c}
}

type lruCache struct{ inner *lru.Cache[string, []byte] }

// Get returns a copy so the caller cannot mutate the cached archive bytes.
func (c *lruCache) Get(key string) ([]byte, bool) {
	v, ok := c.inner.Get(key)
	if !ok {
		return nil, false
	}
	return bytes.Clone(v), true
}

// Add stores a copy so a later caller-side mutation cannot corrupt the cached archive.
func (c *lruCache) Add(key string, value []byte) {
	c.inner.Add(key, bytes.Clone(value))
}
