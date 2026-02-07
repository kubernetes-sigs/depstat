package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestFindAllPathsHonorsLimit(t *testing.T) {
	graph := map[string][]string{
		"A": []string{"B", "C"},
		"B": []string{"D"},
		"C": []string{"D"},
	}
	var out [][]string
	findAllPaths("A", "D", graph, []string{}, map[string]bool{}, &out, 1)
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

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read capture: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	return buf.String()
}
