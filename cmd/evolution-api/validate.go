package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ValidationError describes a field validation failure.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// EvolveRequest is the validated /evolve request body.
type EvolveRequest struct {
	Candidate int     `json:"candidate"`
	Current   int     `json:"current"`
	Weight    float64 `json:"weight"`
	Mode      string  `json:"mode"`
}

// AgentEvolveRequest is the validated /agent/evolve request body.
type AgentEvolveRequest struct {
	TargetFile string  `json:"target_file"`
	Patch      string  `json:"patch"`
	Confidence float64 `json:"confidence"`
	Mode       string  `json:"mode"`
}

var validModes = map[string]bool{"stable": true, "bandit": true, "locked": true}

// parseAndValidateEvolve parses and validates a /evolve JSON body.
// It writes a 400 error and returns false if validation fails.
func parseAndValidateEvolve(r *http.Request, w http.ResponseWriter) (*EvolveRequest, bool) {
	if r.Body == nil {
		http.Error(w, "Empty body", http.StatusBadRequest)
		return nil, false
	}
	var req EvolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return nil, false
	}
	if errs := validateEvolve(&req); len(errs) > 0 {
		body, _ := json.Marshal(map[string]interface{}{"errors": errs})
		http.Error(w, string(body), http.StatusBadRequest)
		return nil, false
	}
	return &req, true
}

// validateEvolve returns any validation errors for an EvolveRequest.
func validateEvolve(req *EvolveRequest) []ValidationError {
	var errs []ValidationError
	if req.Candidate < 0 {
		errs = append(errs, ValidationError{Field: "candidate", Message: "must be non-negative"})
	}
	if req.Current < 0 {
		errs = append(errs, ValidationError{Field: "current", Message: "must be non-negative"})
	}
	if req.Weight <= 0 {
		errs = append(errs, ValidationError{Field: "weight", Message: "must be positive"})
	}
	if req.Mode == "" {
		errs = append(errs, ValidationError{Field: "mode", Message: "required (allowed: stable, bandit, locked)"})
	} else if !validModes[req.Mode] {
		errs = append(errs, ValidationError{Field: "mode", Message: fmt.Sprintf("invalid mode %q (allowed: stable, bandit, locked)", req.Mode)})
	}
	return errs
}

// parseAndValidateAgentEvolve parses and validates an /agent/evolve JSON body.
func parseAndValidateAgentEvolve(r *http.Request, w http.ResponseWriter) (*AgentEvolveRequest, bool) {
	if r.Body == nil {
		http.Error(w, "Empty body", http.StatusBadRequest)
		return nil, false
	}
	var req AgentEvolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return nil, false
	}
	if errs := validateAgentEvolve(&req); len(errs) > 0 {
		body, _ := json.Marshal(map[string]interface{}{"errors": errs})
		http.Error(w, string(body), http.StatusBadRequest)
		return nil, false
	}
	return &req, true
}

// validateAgentEvolve returns any validation errors for an AgentEvolveRequest.
func validateAgentEvolve(req *AgentEvolveRequest) []ValidationError {
	var errs []ValidationError
	if req.TargetFile == "" {
		errs = append(errs, ValidationError{Field: "target_file", Message: "required"})
	}
	if req.Patch == "" {
		errs = append(errs, ValidationError{Field: "patch", Message: "required"})
	}
	if req.Confidence < 0 || req.Confidence > 1 {
		errs = append(errs, ValidationError{Field: "confidence", Message: "must be between 0 and 1"})
	}
	if req.Mode == "" {
		errs = append(errs, ValidationError{Field: "mode", Message: "required (allowed: stable, bandit, locked)"})
	} else if !validModes[req.Mode] {
		errs = append(errs, ValidationError{Field: "mode", Message: fmt.Sprintf("invalid mode %q (allowed: stable, bandit, locked)", req.Mode)})
	}
	return errs
}
