package cache

import (
	"bytes"
	"io"
	"sync"
)

type RRCache struct {
	mu   *sync.RWMutex
	data map[string]*bytes.Buffer
}

// NewRRCache creates a new thread-safe cache for storing RoadRunner template buffers.
func NewRRCache() *RRCache {
	return &RRCache{
		mu:   &sync.RWMutex{},
		data: make(map[string]*bytes.Buffer),
	}
}

// Get retrieves a cached buffer by key, returning a copy to prevent external mutations.
// Returns nil if the key is not found.
func (c *RRCache) Get(key string) *bytes.Buffer {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if cache, ok := c.data[key]; ok {
		buf := new(bytes.Buffer)
		buf.Grow(cache.Len())
		_, err := io.Copy(buf, bytes.NewReader(cache.Bytes()))
		if err != nil {
			panic(err)
		}
		return buf
	}

	return nil
}

// Set stores a buffer in the cache, creating a copy to prevent external mutations.
// If the key already exists, this method returns immediately to preserve in-use buffers.
// Panics if buf is nil.
func (c *RRCache) Set(key string, buf *bytes.Buffer) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if buf == nil {
		panic("cannot set nil value in cache")
	}
	if _, ok := c.data[key]; ok {
		// don't do anything, since we already have a cached value
		// we should not touch that buffer here, since it may be in use (pointer)
		return
	}
	cache := new(bytes.Buffer)
	cache.Grow(buf.Len())
	_, err := io.Copy(cache, bytes.NewReader(buf.Bytes()))
	if err != nil {
		panic(err)
	}
	c.data[key] = cache
}
