package cmd

import (
	"reflect"
	"testing"
)

func TestFilterDepsByTestStatus(t *testing.T) {
	deps := []string{"a", "b", "c", "d"}
	testOnlySet := map[string]bool{"b": true, "d": true}

	testOnly := filterDepsByTestStatus(deps, testOnlySet, true)
	if !reflect.DeepEqual(testOnly, []string{"b", "d"}) {
		t.Fatalf("expected [b d], got %v", testOnly)
	}

	nonTestOnly := filterDepsByTestStatus(deps, testOnlySet, false)
	if !reflect.DeepEqual(nonTestOnly, []string{"a", "c"}) {
		t.Fatalf("expected [a c], got %v", nonTestOnly)
	}

	empty := filterDepsByTestStatus(nil, testOnlySet, true)
	if len(empty) != 0 {
		t.Fatalf("expected empty result for nil deps, got %v", empty)
	}
}

func TestFilterEdgesByTestStatus(t *testing.T) {
	edges := []string{"a -> b", "c -> d", "b -> e", "malformed"}
	testOnlySet := map[string]bool{"b": true}

	testOnly := filterEdgesByTestStatus(edges, testOnlySet, true)
	if !reflect.DeepEqual(testOnly, []string{"a -> b", "b -> e"}) {
		t.Fatalf("expected edges touching b, got %v", testOnly)
	}

	nonTestOnly := filterEdgesByTestStatus(edges, testOnlySet, false)
	if !reflect.DeepEqual(nonTestOnly, []string{"c -> d"}) {
		t.Fatalf("expected [c -> d], got %v", nonTestOnly)
	}
}

func TestFilterVersionChangesByTestStatus(t *testing.T) {
	changes := []VersionChange{
		{Path: "a", Before: "v1.0.0", After: "v1.1.0"},
		{Path: "b", Before: "v2.0.0", After: "v2.1.0"},
	}
	testOnlySet := map[string]bool{"b": true}

	testOnly := filterVersionChangesByTestStatus(changes, testOnlySet, true)
	if len(testOnly) != 1 || testOnly[0].Path != "b" {
		t.Fatalf("expected only b, got %v", testOnly)
	}

	nonTestOnly := filterVersionChangesByTestStatus(changes, testOnlySet, false)
	if len(nonTestOnly) != 1 || nonTestOnly[0].Path != "a" {
		t.Fatalf("expected only a, got %v", nonTestOnly)
	}
}

func TestComputeFilteredCounts(t *testing.T) {
	depGraph := &DependencyOverview{
		DirectDepList: []string{"a", "b"},
		TransDepList:  []string{"b", "c"},
	}
	testOnlySet := map[string]bool{"b": true}

	testOnlyCounts := computeFilteredCounts(depGraph, testOnlySet, true)
	if testOnlyCounts.DirectDeps != 1 || testOnlyCounts.TransDeps != 1 || testOnlyCounts.TotalDeps != 1 {
		t.Fatalf("unexpected test-only counts: %+v", testOnlyCounts)
	}

	nonTestOnlyCounts := computeFilteredCounts(depGraph, testOnlySet, false)
	if nonTestOnlyCounts.DirectDeps != 1 || nonTestOnlyCounts.TransDeps != 1 || nonTestOnlyCounts.TotalDeps != 2 {
		t.Fatalf("unexpected non-test-only counts: %+v", nonTestOnlyCounts)
	}
}

func TestBuildSplitSection(t *testing.T) {
	beforeGraph := &DependencyOverview{
		DirectDepList: []string{"a"},
		TransDepList:  []string{"b"},
	}
	afterGraph := &DependencyOverview{
		DirectDepList: []string{"a"},
		TransDepList:  []string{"b", "c"},
	}
	result := DiffResult{
		Added:      []string{"c"},
		EdgesAdded: []string{"a -> c"},
	}
	beforeTestOnly := map[string]bool{"b": true}
	afterTestOnly := map[string]bool{"b": true, "c": true}

	section := buildSplitSection(result, beforeGraph, afterGraph, beforeTestOnly, afterTestOnly, true)
	if section.Before.TransDeps != 1 || section.After.TransDeps != 2 {
		t.Fatalf("unexpected before/after test-only trans counts: %+v", section)
	}
	if section.Delta.TransDeps != 1 {
		t.Fatalf("expected delta of 1 trans dep, got %d", section.Delta.TransDeps)
	}
	if !reflect.DeepEqual(section.Added, []string{"c"}) {
		t.Fatalf("expected added [c], got %v", section.Added)
	}
	if !reflect.DeepEqual(section.EdgesAdded, []string{"a -> c"}) {
		t.Fatalf("expected edgesAdded [a -> c], got %v", section.EdgesAdded)
	}
}

func TestBuildSplitResult(t *testing.T) {
	beforeGraph := &DependencyOverview{DirectDepList: []string{"a"}, TransDepList: []string{"b"}}
	afterGraph := &DependencyOverview{DirectDepList: []string{"a"}, TransDepList: []string{"b"}}
	result := DiffResult{}
	beforeTestOnly := map[string]bool{"b": true}
	afterTestOnly := map[string]bool{"b": true}

	split := buildSplitResult(result, beforeGraph, afterGraph, beforeTestOnly, afterTestOnly)
	if split.TestOnly.Before.TransDeps != 1 {
		t.Fatalf("expected 1 test-only trans dep, got %d", split.TestOnly.Before.TransDeps)
	}
	if split.NonTestOnly.Before.DirectDeps != 1 {
		t.Fatalf("expected 1 non-test-only direct dep, got %d", split.NonTestOnly.Before.DirectDeps)
	}
}

func TestDiffSlices(t *testing.T) {
	a := []string{"x", "y"}
	b := []string{"y", "z", "x"}

	diff := diffSlices(a, b)
	if !reflect.DeepEqual(diff, []string{"z"}) {
		t.Fatalf("expected [z], got %v", diff)
	}

	same := diffSlices(a, a)
	if len(same) != 0 {
		t.Fatalf("expected no diff between identical slices, got %v", same)
	}

	fromEmpty := diffSlices(nil, b)
	if !reflect.DeepEqual(fromEmpty, []string{"x", "y", "z"}) {
		t.Fatalf("expected all of b sorted, got %v", fromEmpty)
	}
}

func TestGetEdges(t *testing.T) {
	graph := map[string][]string{
		"a": {"b", "c"},
		"b": {"c"},
	}
	edges := getEdges(graph)
	expected := []string{"a -> b", "a -> c", "b -> c"}
	if !reflect.DeepEqual(edges, expected) {
		t.Fatalf("expected %v, got %v", expected, edges)
	}

	if len(getEdges(map[string][]string{})) != 0 {
		t.Fatalf("expected no edges for empty graph")
	}
}
