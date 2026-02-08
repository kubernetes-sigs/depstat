package cmd

import (
	"testing"
)

func Test_shortestDepthByModule(t *testing.T) {
	graph := map[string][]string{
		"main": {"A", "B"},
		"A":    {"C"},
		"B":    {"C", "D"},
		"C":    {"E"},
	}
	depth := shortestDepthByModule([]string{"main"}, graph)

	tests := map[string]int{
		"main": 0,
		"A":    1, "B": 1,
		"C": 2, "D": 2,
		"E": 3,
	}
	for mod, expected := range tests {
		if got := depth[mod]; got != expected {
			t.Errorf("depth[%s] = %d, want %d", mod, got, expected)
		}
	}
}

func Test_buildGraphTopology(t *testing.T) {
	overview := &DependencyOverview{
		MainModules:   []string{"main"},
		DirectDepList: []string{"A", "B"},
		TransDepList:  []string{"C"},
		Graph: map[string][]string{
			"main": {"A", "B"},
			"A":    {"C"},
			"B":    {"C"},
		},
	}
	nodes, edges := buildGraphTopology(overview)

	// Should have 4 nodes: main, A, B, C
	if len(nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(nodes))
	}

	// C should have inDegree 2 (from A and B)
	for _, n := range nodes {
		if n.Module == "C" && n.InDegree != 2 {
			t.Errorf("C inDegree = %d, want 2", n.InDegree)
		}
		if n.Module == "main" && !n.IsMainModule {
			t.Errorf("main should be marked as main module")
		}
		if n.Module == "main" && n.Depth != 0 {
			t.Errorf("main depth = %d, want 0", n.Depth)
		}
	}

	// Should have 4 edges: main->A, main->B, A->C, B->C
	if len(edges) != 4 {
		t.Errorf("expected 4 edges, got %d", len(edges))
	}

	// Verify edges have no Type field (struct only has From/To)
	for _, e := range edges {
		if e.From == "" || e.To == "" {
			t.Errorf("edge has empty From or To: %+v", e)
		}
	}
}

func Test_buildRankings(t *testing.T) {
	nodes := []graphNode{
		{Module: "A", InDegree: 5, OutDegree: 1},
		{Module: "B", InDegree: 3, OutDegree: 4},
		{Module: "C", InDegree: 10, OutDegree: 2},
	}
	// n=0 should return empty slices
	if got := buildRankings(nodes, "both", 0); len(got.In) != 0 || len(got.Out) != 0 {
		t.Fatalf("expected empty rankings for n=0, got in=%d out=%d", len(got.In), len(got.Out))
	}
	r := buildRankings(nodes, "both", 2)
	if len(r.In) != 2 {
		t.Errorf("expected 2 in rankings, got %d", len(r.In))
	}
	if r.In[0].Module != "C" {
		t.Errorf("top in-degree should be C, got %s", r.In[0].Module)
	}
	if len(r.Out) != 2 {
		t.Errorf("expected 2 out rankings, got %d", len(r.Out))
	}
	if r.Out[0].Module != "B" {
		t.Errorf("top out-degree should be B, got %s", r.Out[0].Module)
	}
}

func Test_buildRankings_inOnly(t *testing.T) {
	nodes := []graphNode{
		{Module: "A", InDegree: 5, OutDegree: 1},
		{Module: "B", InDegree: 3, OutDegree: 4},
	}
	r := buildRankings(nodes, "in", 2)
	if r.In == nil {
		t.Fatal("expected In rankings to be populated")
	}
	if r.Out != nil {
		t.Errorf("expected Out rankings to be nil for mode=in, got %v", r.Out)
	}
	if r.Mode != "in" {
		t.Errorf("expected mode=in, got %s", r.Mode)
	}
}

func Test_buildRankings_outOnly(t *testing.T) {
	nodes := []graphNode{
		{Module: "A", InDegree: 5, OutDegree: 1},
		{Module: "B", InDegree: 3, OutDegree: 4},
	}
	r := buildRankings(nodes, "out", 2)
	if r.Out == nil {
		t.Fatal("expected Out rankings to be populated")
	}
	if r.In != nil {
		t.Errorf("expected In rankings to be nil for mode=out, got %v", r.In)
	}
}

func Test_shortestDepthByModule_unreachable(t *testing.T) {
	// Node "X" is not reachable from "main"
	graph := map[string][]string{
		"main": {"A"},
		"X":    {"Y"},
	}
	depth := shortestDepthByModule([]string{"main"}, graph)
	if _, ok := depth["X"]; ok {
		t.Errorf("X should not be in depth map (unreachable)")
	}
	if _, ok := depth["Y"]; ok {
		t.Errorf("Y should not be in depth map (unreachable)")
	}
	if depth["main"] != 0 {
		t.Errorf("main depth = %d, want 0", depth["main"])
	}
	if depth["A"] != 1 {
		t.Errorf("A depth = %d, want 1", depth["A"])
	}
}

func Test_buildGraphTopology_unreachableDepth(t *testing.T) {
	// Node "X" exists in graph but is not reachable from main
	overview := &DependencyOverview{
		MainModules:   []string{"main"},
		DirectDepList: []string{"A"},
		TransDepList:  []string{},
		Graph: map[string][]string{
			"main": {"A"},
			"X":    {"Y"},
		},
	}
	nodes, _ := buildGraphTopology(overview)

	for _, n := range nodes {
		if n.Module == "X" && n.Depth != -1 {
			t.Errorf("unreachable node X depth = %d, want -1", n.Depth)
		}
		if n.Module == "Y" && n.Depth != -1 {
			t.Errorf("unreachable node Y depth = %d, want -1", n.Depth)
		}
	}
}
