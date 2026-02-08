package cmd

import "testing"

func TestFindAllCyclesWithMaxLength(t *testing.T) {
	graph := map[string][]string{
		"A": []string{"B", "C"},
		"B": []string{"A", "C"},
		"C": []string{"A"},
	}

	all := findAllCyclesWithMaxLength(graph, 0)
	short := findAllCyclesWithMaxLength(graph, 2)

	if len(all) != 3 {
		t.Fatalf("expected 3 cycles, got %d (%v)", len(all), all)
	}
	if len(short) != 2 {
		t.Fatalf("expected only 2 cycles with max-length=2, got %d (%v)", len(short), short)
	}
	foundAB := false
	foundAC := false
	for _, c := range short {
		if isSliceSame(c, Chain{"A", "B", "A"}) {
			foundAB = true
		}
		if isSliceSame(c, Chain{"A", "C", "A"}) {
			foundAC = true
		}
	}
	if !foundAB || !foundAC {
		t.Fatalf("expected both A-B-A and A-C-A cycles, got %v", short)
	}
}

func TestSummarizeCycles(t *testing.T) {
	cycles := []Chain{
		{"A", "B", "A"},
		{"B", "C", "B"},
		{"A", "C", "D", "A"},
	}

	s := summarizeCycles(cycles, 10)
	if s.TotalCycles != 3 {
		t.Fatalf("expected 3 total cycles, got %d", s.TotalCycles)
	}
	if s.ByLength["2"] != 2 || s.ByLength["3"] != 1 {
		t.Fatalf("unexpected byLength: %v", s.ByLength)
	}
	if len(s.TwoNodeCycles) != 2 {
		t.Fatalf("expected 2 two-node cycles, got %d: %v", len(s.TwoNodeCycles), s.TwoNodeCycles)
	}
	if len(s.TopParticipants) == 0 || s.TopParticipants[0].Module != "A" || s.TopParticipants[0].CycleCount != 2 {
		t.Fatalf("unexpected top participants: %v", s.TopParticipants)
	}
}

func TestSummarizeCycles_TwoNodeDedup(t *testing.T) {
	// Test that reversed 2-node cycles are deduplicated
	cycles := []Chain{
		{"A", "B", "A"},      // 2-node
		{"B", "A", "B"},      // same pair reversed (should be same 2-node)
		{"A", "B", "C", "A"}, // 3-node
	}
	summary := summarizeCycles(cycles, 10)
	if summary.TotalCycles != 3 {
		t.Fatalf("expected 3 total cycles, got %d", summary.TotalCycles)
	}
	if summary.ByLength["2"] != 2 {
		t.Fatalf("expected 2 cycles of length 2, got %d", summary.ByLength["2"])
	}
	if summary.ByLength["3"] != 1 {
		t.Fatalf("expected 1 cycle of length 3, got %d", summary.ByLength["3"])
	}
	if len(summary.TwoNodeCycles) != 1 {
		t.Fatalf("expected 1 deduped two-node cycle (A-B pair), got %d: %v", len(summary.TwoNodeCycles), summary.TwoNodeCycles)
	}
}

func TestSummarizeCycles_TopN(t *testing.T) {
	// Create enough cycles to test topN truncation
	cycles := []Chain{
		{"A", "B", "A"},
		{"C", "D", "C"},
		{"E", "F", "E"},
	}
	summary := summarizeCycles(cycles, 2)
	if len(summary.TopParticipants) != 2 {
		t.Fatalf("expected 2 top participants with topN=2, got %d", len(summary.TopParticipants))
	}
}

func TestDiffSummary(t *testing.T) {
	summary := DiffSummary{
		AddedCount:          3,
		RemovedCount:        1,
		VersionChangesCount: 2,
	}
	if summary.AddedCount != 3 {
		t.Fatalf("expected AddedCount 3, got %d", summary.AddedCount)
	}
	if summary.RemovedCount != 1 {
		t.Fatalf("expected RemovedCount 1, got %d", summary.RemovedCount)
	}
	if summary.VersionChangesCount != 2 {
		t.Fatalf("expected VersionChangesCount 2, got %d", summary.VersionChangesCount)
	}
}
