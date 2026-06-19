package shadow

import "sync"

// ShadowResult holds the outcome of a shadow evaluation.
type ShadowResult struct {
	Candidate float64
	Current   float64
}

// ShadowCache is a thread-safe cache for shadow evaluation results.
type ShadowCache struct {
	mu    sync.RWMutex
	store map[string]ShadowResult
}

// NewShadowCache creates a new cache.
func NewShadowCache() *ShadowCache {
	return &ShadowCache{store: make(map[string]ShadowResult)}
}

// Get retrieves a cached result, if present.
func (c *ShadowCache) Get(key string) (ShadowResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	res, ok := c.store[key]
	return res, ok
}

// Set stores a result in the cache.
func (c *ShadowCache) Set(key string, res ShadowResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = res
}
