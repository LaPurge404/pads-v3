package change

import "sync"

// ProposalCache memoizes recent change-validation results.
type ProposalCache struct {
	mu    sync.RWMutex
	store map[string]PolicyChangeProposal
}

func NewProposalCache() *ProposalCache {
	return &ProposalCache{
		store: make(map[string]PolicyChangeProposal),
	}
}

func (c *ProposalCache) Get(key string) (PolicyChangeProposal, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	res, ok := c.store[key]
	return res, ok
}

func (c *ProposalCache) Set(key string, res PolicyChangeProposal) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.store[key] = res
}
