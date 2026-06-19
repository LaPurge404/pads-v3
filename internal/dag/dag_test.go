package dag

import (
	"testing"
)

func TestResolveOrder_Linear(t *testing.T) {
	g := &Graph{
		Nodes: map[string]*Node{
			"a": {ID: "a"},
			"b": {ID: "b", Dependencies: []string{"a"}},
		},
	}
	order, err := g.ResolveOrder()
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(order))
	}
	if order[0].ID != "a" {
		t.Errorf("expected a first, got %s", order[0].ID)
	}
}

func TestResolveOrder_Cycle(t *testing.T) {
	g := &Graph{
		Nodes: map[string]*Node{
			"a": {ID: "a", Dependencies: []string{"b"}},
			"b": {ID: "b", Dependencies: []string{"a"}},
		},
	}
	_, err := g.ResolveOrder()
	if err == nil {
		t.Fatal("expected cycle error")
	}
}
