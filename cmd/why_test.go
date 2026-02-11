package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestComputePathSubgraphDiamond(t *testing.T) {
	// Diamond: A→B→D, A→C→D
	graph := map[string][]string{
		"A": {"B", "C"},
		"B": {"D"},
		"C": {"D"},
	}
	reachable := computeReachableToTarget("D", graph)
	nodeSet, edgeSet := computePathSubgraph([]string{"A"}, graph, reachable)

	// All 4 nodes should be in the subgraph
	for _, n := range []string{"A", "B", "C", "D"} {
		if !nodeSet[n] {
			t.Errorf("expected node %s in subgraph", n)
		}
	}
	if len(nodeSet) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(nodeSet))
	}

	// All 4 edges: A→B, A→C, B→D, C→D
	expectedEdges := []svgEdge{
		{"A", "B"}, {"A", "C"}, {"B", "D"}, {"C", "D"},
	}
	for _, e := range expectedEdges {
		if !edgeSet[e] {
			t.Errorf("expected edge %s→%s in subgraph", e.From, e.To)
		}
	}
	if len(edgeSet) != 4 {
		t.Errorf("expected 4 edges, got %d", len(edgeSet))
	}
}

func TestComputePathSubgraphPrunesUnreachable(t *testing.T) {
	// A→B→D, A→C→E (E is a dead end, not connected to D)
	graph := map[string][]string{
		"A": {"B", "C"},
		"B": {"D"},
		"C": {"E"},
	}
	reachable := computeReachableToTarget("D", graph)
	nodeSet, edgeSet := computePathSubgraph([]string{"A"}, graph, reachable)

	// Only A, B, D should be in the subgraph (C and E can't reach D)
	for _, n := range []string{"A", "B", "D"} {
		if !nodeSet[n] {
			t.Errorf("expected node %s in subgraph", n)
		}
	}
	if nodeSet["C"] || nodeSet["E"] {
		t.Error("C and E should not be in subgraph (can't reach target D)")
	}
	if len(edgeSet) != 2 {
		t.Errorf("expected 2 edges (A→B, B→D), got %d", len(edgeSet))
	}
}

func TestComputePathSubgraphMultipleMainModules(t *testing.T) {
	graph := map[string][]string{
		"M1": {"A"},
		"M2": {"B"},
		"A":  {"C"},
		"B":  {"C"},
	}
	reachable := computeReachableToTarget("C", graph)
	nodeSet, edgeSet := computePathSubgraph([]string{"M1", "M2"}, graph, reachable)

	if len(nodeSet) != 5 {
		t.Errorf("expected 5 nodes, got %d", len(nodeSet))
	}
	if len(edgeSet) != 4 {
		t.Errorf("expected 4 edges, got %d", len(edgeSet))
	}
}

func TestComputePathSubgraphKeepsSameDepthEdges(t *testing.T) {
	// A→B→C→D and A→C (B and C same BFS depth from A).
	graph := map[string][]string{
		"A": {"B", "C"},
		"B": {"C"},
		"C": {"D"},
	}
	reachable := computeReachableToTarget("D", graph)
	nodeSet, edgeSet := computePathSubgraph([]string{"A"}, graph, reachable)

	for _, n := range []string{"A", "B", "C", "D"} {
		if !nodeSet[n] {
			t.Errorf("expected node %s in subgraph", n)
		}
	}
	if !edgeSet[svgEdge{"B", "C"}] {
		t.Error("expected same-depth edge B→C to be included")
	}
}

func TestFindAllPathsMaxDepth(t *testing.T) {
	// Chain: A→B→C→D→E (depth 4)
	graph := map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {"D"},
		"D": {"E"},
	}
	reachable := map[string]bool{"A": true, "B": true, "C": true, "D": true, "E": true}

	// With maxDepth=3, paths longer than 3 hops should be pruned
	oldMaxDepth := whyMaxDepth
	whyMaxDepth = 3
	defer func() { whyMaxDepth = oldMaxDepth }()

	var out [][]string
	findAllPaths("A", "E", graph, reachable, []string{}, map[string]bool{}, &out, 0)
	if len(out) != 0 {
		t.Fatalf("expected 0 paths with maxDepth=3 (path is 4 hops), got %d", len(out))
	}

	// With maxDepth=5, the path should be found
	whyMaxDepth = 5
	findAllPaths("A", "E", graph, reachable, []string{}, map[string]bool{}, &out, 0)
	if len(out) != 1 {
		t.Fatalf("expected 1 path with maxDepth=5, got %d", len(out))
	}
}

func TestOutputWhySVGWithPrecomputedGraph(t *testing.T) {
	result := WhyResult{
		Target:      "D",
		Found:       true,
		MainModules: []string{"A"},
		DirectDeps:  []string{"B", "C"},
		TotalPaths:  4,
		NodeSet:     map[string]bool{"A": true, "B": true, "C": true, "D": true},
		EdgeSet: map[svgEdge]bool{
			{"A", "B"}: true, {"A", "C"}: true,
			{"B", "D"}: true, {"C", "D"}: true,
		},
	}

	output := captureStdout(t, func() {
		if err := outputWhySVG(result); err != nil {
			t.Fatalf("outputWhySVG returned error: %v", err)
		}
	})

	if !strings.Contains(output, "<svg") {
		t.Fatal("expected SVG output")
	}
	if !strings.Contains(output, "Why is D included?") {
		t.Fatal("expected title in SVG")
	}
	// Check all 4 nodes are rendered
	for _, node := range []string{"A", "B", "C", "D"} {
		if !strings.Contains(output, "<title>"+node+"</title>") {
			t.Errorf("expected node %s in SVG output", node)
		}
	}
}

func TestFindAllPathsHonorsLimit(t *testing.T) {
	graph := map[string][]string{
		"A": {"B", "C"},
		"B": {"D"},
		"C": {"D"},
	}
	reachable := map[string]bool{"A": true, "B": true, "C": true, "D": true}
	var out [][]string
	findAllPaths("A", "D", graph, reachable, []string{}, map[string]bool{}, &out, 1)
	if len(out) != 1 {
		t.Fatalf("expected exactly 1 path due to limit, got %d (%v)", len(out), out)
	}
}

func TestOutputWhyDOTDeterministicOrder(t *testing.T) {
	result := WhyResult{
		Target:      "D",
		Found:       true,
		MainModules: []string{"A"},
		Paths: []WhyPath{
			{Path: []string{"A", "C", "D"}},
			{Path: []string{"A", "B", "D"}},
		},
	}

	output := captureStdout(t, func() {
		if err := outputWhyDOT(result, nil); err != nil {
			t.Fatalf("outputWhyDOT returned error: %v", err)
		}
	})

	idxA := strings.Index(output, "\"A\" [fillcolor=")
	idxB := strings.Index(output, "\"B\" [fillcolor=")
	idxC := strings.Index(output, "\"C\" [fillcolor=")
	idxD := strings.Index(output, "\"D\" [fillcolor=")
	if !(idxA < idxB && idxB < idxC && idxC < idxD) {
		t.Fatalf("expected sorted node order A,B,C,D, got output:\n%s", output)
	}

	edgeAB := strings.Index(output, "\"A\" -> \"B\";")
	edgeAC := strings.Index(output, "\"A\" -> \"C\";")
	edgeBD := strings.Index(output, "\"B\" -> \"D\";")
	edgeCD := strings.Index(output, "\"C\" -> \"D\";")
	if !(edgeAB < edgeAC && edgeAC < edgeBD && edgeBD < edgeCD) {
		t.Fatalf("expected sorted edge order, got output:\n%s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	defer func() {
		os.Stdout = old
	}()

	var buf bytes.Buffer
	readDone := make(chan error, 1)
	go func() {
		_, readErr := io.Copy(&buf, r)
		_ = r.Close()
		readDone <- readErr
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if err := <-readDone; err != nil {
		t.Fatalf("read capture: %v", err)
	}
	return buf.String()
}
