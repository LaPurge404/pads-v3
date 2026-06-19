package change

import (
	"sync"
)

type PolicyChangeRegistry struct {
	mu        sync.RWMutex
	proposals map[string]PolicyChangeProposal
}

func NewPolicyChangeRegistry() *PolicyChangeRegistry {
	return &PolicyChangeRegistry{
		proposals: make(map[string]PolicyChangeProposal),
	}
}

func (r *PolicyChangeRegistry) Add(p PolicyChangeProposal) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.proposals[p.ID] = p
}

func (r *PolicyChangeRegistry) Get(id string) (PolicyChangeProposal, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.proposals[id]
	return p, ok
}

func (r *PolicyChangeRegistry) List() []PolicyChangeProposal {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]PolicyChangeProposal, 0, len(r.proposals))
	for _, p := range r.proposals {
		out = append(out, p)
	}
	return out
}

func (r *PolicyChangeRegistry) MarkAccepted(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p := r.proposals[id]
	p.Accepted = true
	r.proposals[id] = p
}

func (r *PolicyChangeRegistry) MarkRejected(id string, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p := r.proposals[id]
	p.Accepted = false
	p.Reason = reason
	r.proposals[id] = p
}
