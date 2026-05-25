package github

import (
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

func (c *lruCache) Get(key string) ([]byte, bool) { return c.inner.Get(key) }
func (c *lruCache) Add(key string, value []byte)  { c.inner.Add(key, value) }
