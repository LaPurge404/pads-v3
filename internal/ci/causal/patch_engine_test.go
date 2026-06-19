package causal

import (
	"testing"

	"pads-v3/internal/ci"
)

func TestGeneratePatch_CacheHit(t *testing.T) {
	engine := &PatchEngine{}
	node := &DivergenceNode{
		Type:   DiffCacheHit,
		JobID:  "test",
		StepID: "step1",
		Detail: "cache status mismatch",
	}

	action, err := engine.GeneratePatch(node)
	if err != nil {
		t.Fatal(err)
	}
	if action.Kind != "invalidate_cache" {
		t.Errorf("expected invalidate_cache, got %s", action.Kind)
	}
	if action.JobID != "test" || action.StepID != "step1" {
		t.Errorf("wrong job/step in action: %s/%s", action.JobID, action.StepID)
	}
}

func TestGeneratePatch_Artifact(t *testing.T) {
	engine := &PatchEngine{}
	node := &DivergenceNode{
		Type:   DiffArtifact,
		JobID:  "test",
		StepID: "step2",
	}

	action, err := engine.GeneratePatch(node)
	if err != nil {
		t.Fatal(err)
	}
	if action.Kind != "force_recompute" {
		t.Errorf("expected force_recompute, got %s", action.Kind)
	}
}

func TestApplyPatch_InvalidateCache(t *testing.T) {
	engine := &PatchEngine{}

	action := &PatchAction{
		Kind:   "invalidate_cache",
		JobID:  "test",
		StepID: "step1",
	}

	jobs := map[string]ci.Job{
		"test": {
			ID: "test",
			Steps: []ci.Step{
				{ID: "step1", Run: "echo hello"},
			},
		},
	}

	if err := engine.ApplyPatch(&jobs, action); err != nil {
		t.Fatal(err)
	}
}
