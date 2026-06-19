package ci

import (
	"testing"
)

func TestResolveJobOrder_NoDependencies(t *testing.T) {
	jobs := map[string]Job{
		"a": {ID: "a"},
		"b": {ID: "b"},
	}
	order, err := ResolveJobOrder(jobs)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(order))
	}
}

func TestResolveJobOrder_LinearDependency(t *testing.T) {
	jobs := map[string]Job{
		"a": {ID: "a"},
		"b": {ID: "b", Needs: []string{"a"}},
	}
	order, err := ResolveJobOrder(jobs)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(order))
	}
	// a must come before b
	if order[0].ID != "a" || order[1].ID != "b" {
		t.Errorf("expected order [a b], got %v", jobIDs(order))
	}
}

func TestResolveJobOrder_DiamondDependency(t *testing.T) {
	jobs := map[string]Job{
		"a": {ID: "a"},
		"b": {ID: "b", Needs: []string{"a"}},
		"c": {ID: "c", Needs: []string{"a"}},
		"d": {ID: "d", Needs: []string{"b", "c"}},
	}
	order, err := ResolveJobOrder(jobs)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 4 {
		t.Fatalf("expected 4 jobs, got %d", len(order))
	}
	// verify that dependencies appear before dependents
	indices := make(map[string]int)
	for i, j := range order {
		indices[j.ID] = i
	}
	if indices["b"] <= indices["a"] || indices["c"] <= indices["a"] {
		t.Error("b and c must come after a")
	}
	if indices["d"] <= indices["b"] || indices["d"] <= indices["c"] {
		t.Error("d must come after b and c")
	}
}

func TestResolveJobOrder_Cycle(t *testing.T) {
	jobs := map[string]Job{
		"a": {ID: "a", Needs: []string{"b"}},
		"b": {ID: "b", Needs: []string{"a"}},
	}
	_, err := ResolveJobOrder(jobs)
	if err == nil {
		t.Error("expected error for cyclic dependency")
	}
}

func jobIDs(jobs []Job) []string {
	ids := make([]string, len(jobs))
	for i, j := range jobs {
		ids[i] = j.ID
	}
	return ids
}
