package change

import (
	"time"

	"pads-v3/internal/policy"
)

// PolicyChangeProposal describes a candidate config change and its validation outcome.
type PolicyChangeProposal struct {
	ID             string
	FromConfig     policy.TunedConfig
	ToConfig       policy.TunedConfig
	CandidateScore float64
	CurrentScore   float64
	Confidence     float64
	Accepted       bool
	Reason         string
	CreatedAt      time.Time
}
