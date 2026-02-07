/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"testing"
)

func Test_getChains_simple(t *testing.T) {

	/*
		Graph:
				  A
				/ | \
			   B  C  D
				\/   |
				E	 G
				|
				F
				|
				H
	*/

	graph := make(map[string][]string)
	graph["A"] = []string{"B", "C", "D"}
	graph["B"] = []string{"E"}
	graph["C"] = []string{"E"}
	graph["D"] = []string{"G"}
	graph["E"] = []string{"F"}
	graph["F"] = []string{"H"}

	transDeps := []string{"E", "G", "F", "H"}
	directDeps := []string{"B", "C", "D"}
	mainModules := []string{"A"}
	overview := &DependencyOverview{
		Graph:         graph,
		TransDepList:  transDeps,
		DirectDepList: directDeps,
		MainModules:   mainModules,
	}

	var chains []Chain
	var temp Chain
	longestChain := getLongestChain("A", graph, temp, map[string]Chain{})
	maxDepth := len(longestChain)
	getAllChains("A", graph, temp, &chains)
	cycles := findAllCycles(graph)

	correctChains := [][]string{
		{"A", "B", "E", "F", "H"},
		{"A", "C", "E", "F", "H"},
		{"A", "D", "G"},
	}

	correctFileContentsForAllDeps := `MainNode [label="A", style="filled" color="yellow"]
"MainNode" -> "B"
"MainNode" -> "C"
"MainNode" -> "D"
"B" -> "E"
"C" -> "E"
"D" -> "G"
"E" -> "F"
"F" -> "H"
`
	if correctFileContentsForAllDeps != getFileContentsForAllDeps(overview) {
		t.Errorf("File contents for graph of all dependencies are wrong")
	}

	for i := range chains {
		if !isSliceSame(chains[i], correctChains[i]) {
			t.Errorf("Chains are not same")
		}
	}

	correctFileContentsForSingleDep := `MainNode [label="E", style="filled" color="yellow"]
"A" -> "B" -> MainNode -> "F" -> "H"
"A" -> "C" -> MainNode -> "F" -> "H"
`
	if correctFileContentsForSingleDep != getFileContentsForSingleDep(chains, "E") {
		t.Errorf("File contents for graph of a single dependency are wrong")
	}

	if len(cycles) != 0 {
		t.Errorf("There should be no cycles")
	}

	if maxDepth != 5 {
		t.Errorf("Max depth of dependencies was incorrect")
	}

	correctLongestChain := Chain{"A", "B", "E", "F", "H"}

	if !isSliceSame(correctLongestChain, longestChain) {
		t.Errorf("First longest path was incorrect")
	}

}

func Test_getChains_cycle(t *testing.T) {

	/*
		Graph:
					 A
				   /   \
				  B     C
				  |     |
				  D 	E
				/   \
				H	F
				 \ /
				  G
	*/

	graph := make(map[string][]string)
	graph["A"] = []string{"B", "C"}
	graph["B"] = []string{"D"}
	graph["C"] = []string{"E"}
	graph["D"] = []string{"F"}
	graph["F"] = []string{"G"}
	graph["G"] = []string{"H"}
	graph["H"] = []string{"D"}

	transDeps := []string{"D", "E", "F", "G", "H"}
	directDeps := []string{"B", "C"}
	mainModules := []string{"A"}
	overview := &DependencyOverview{
		Graph:         graph,
		TransDepList:  transDeps,
		DirectDepList: directDeps,
		MainModules:   mainModules,
	}

	var chains []Chain
	var temp Chain
	longestChain := getLongestChain("A", graph, temp, map[string]Chain{})
	maxDepth := len(longestChain)
	getAllChains("A", graph, temp, &chains)
	cycles := findAllCycles(graph)

	correctFileContentsForAllDeps := `MainNode [label="A", style="filled" color="yellow"]
"MainNode" -> "B"
"MainNode" -> "C"
"B" -> "D"
"C" -> "E"
"D" -> "F"
"F" -> "G"
"G" -> "H"
"H" -> "D"
`
	if correctFileContentsForAllDeps != getFileContentsForAllDeps(overview) {
		t.Errorf("File contents for graph of all dependencies are wrong")
	}

	correctChains := [][]string{
		{"A", "B", "D", "F", "G", "H"},
		{"A", "C", "E"},
	}
	for i := range chains {
		if !isSliceSame(chains[i], correctChains[i]) {
			t.Errorf("Chains are not same")
		}
	}

	correctFileContentsForSingleDep := `MainNode [label="H", style="filled" color="yellow"]
"A" -> "B" -> "D" -> "F" -> "G" -> MainNode
`
	if correctFileContentsForSingleDep != getFileContentsForSingleDep(chains, "H") {
		t.Errorf("File contents for graph of a single dependency are wrong")
	}

	// Johnson's algorithm finds one cycle: D -> F -> G -> H -> D
	if len(cycles) != 1 {
		t.Errorf("Number of cycles is not correct, expected 1, got %d: %v", len(cycles), cycles)
	}

	cyc := []string{"D", "F", "G", "H", "D"}
	if len(cycles) > 0 && !isSliceSame(cycles[0], cyc) {
		t.Errorf("Cycle is not correct, expected %v, got %v", cyc, cycles[0])
	}

	if maxDepth != 6 {
		t.Errorf("Max depth of dependencies was incorrect")
	}

	correctLongestChain := []string{"A", "B", "D", "F", "G", "H"}
	if !isSliceSame(longestChain, correctLongestChain) {
		t.Errorf("Longest path was incorrect")
	}
}

func Test_getChains_cycle_2(t *testing.T) {

	/*
		Graph:
					 A
				   /  |
				  B   |
				 ||   |
				  C --
				/   \
				D	E
				 \ /
				  F
	*/

	graph := make(map[string][]string)
	graph["A"] = []string{"B", "C"}
	graph["B"] = []string{"C"}
	graph["C"] = []string{"B", "E"}
	graph["E"] = []string{"F"}
	graph["F"] = []string{"D"}
	graph["D"] = []string{"C"}

	transDeps := []string{"C", "B", "E", "F", "D"}
	directDeps := []string{"B", "C"}
	mainModules := []string{"A"}
	overview := &DependencyOverview{
		Graph:         graph,
		TransDepList:  transDeps,
		DirectDepList: directDeps,
		MainModules:   mainModules,
	}

	var chains []Chain
	var temp Chain
	longestChain := getLongestChain("A", graph, temp, map[string]Chain{})
	maxDepth := len(longestChain)
	getAllChains("A", graph, temp, &chains)
	cycles := findAllCycles(graph)

	correctChains := [][]string{
		{"A", "B", "C"},
		{"A", "B", "C", "E", "F", "D"},
		{"A", "C", "B"},
		{"A", "C", "E", "F", "D"},
	}

	correctFileContentsForAllDeps := `MainNode [label="A", style="filled" color="yellow"]
"MainNode" -> "B"
"MainNode" -> "C"
"B" -> "C"
"C" -> "B"
"C" -> "E"
"D" -> "C"
"E" -> "F"
"F" -> "D"
`
	if correctFileContentsForAllDeps != getFileContentsForAllDeps(overview) {
		t.Errorf("File contents for graph of all dependencies are wrong")
	}

	for i := range chains {
		if !isSliceSame(chains[i], correctChains[i]) {
			t.Errorf("Chains are not same")
		}
	}
	correctFileContentsForSingleDep := `MainNode [label="B", style="filled" color="yellow"]
"A" -> MainNode -> "C"
"A" -> MainNode -> "C" -> "E" -> "F" -> "D"
"A" -> "C" -> MainNode
`
	if correctFileContentsForSingleDep != getFileContentsForSingleDep(chains, "B") {
		t.Errorf("File contents for graph of a single dependency are wrong")
	}
	if maxDepth != 6 {
		t.Errorf("Max depth of dependencies was incorrect")
	}

	// Johnson's algorithm finds 2 elementary cycles:
	// 1. B -> C -> B (the mutual dependency)
	// 2. C -> E -> F -> D -> C (the longer cycle)
	// Note: B-C-B and C-B-C are the same cycle, Johnson's reports each unique cycle once
	if len(cycles) != 2 {
		t.Errorf("Number of cycles is incorrect, expected 2, got %d: %v", len(cycles), cycles)
	}

	// Verify the cycles found
	cyc1 := []string{"B", "C", "B"}
	cyc2 := []string{"C", "E", "F", "D", "C"}
	foundCyc1 := false
	foundCyc2 := false
	for _, c := range cycles {
		if isSliceSame(c, cyc1) {
			foundCyc1 = true
		}
		if isSliceSame(c, cyc2) {
			foundCyc2 = true
		}
	}
	if !foundCyc1 {
		t.Errorf("Expected cycle B-C-B not found in %v", cycles)
	}
	if !foundCyc2 {
		t.Errorf("Expected cycle C-E-F-D-C not found in %v", cycles)
	}

	correctLongestChain := []string{"A", "B", "C", "E", "F", "D"}
	if !isSliceSame(longestChain, correctLongestChain) {
		t.Errorf("Longest path was incorrect")
	}
}

// order matters
func Test_isSliceSame_Pass(t *testing.T) {
	a := []string{"A", "B", "C", "D"}
	b := []string{"A", "B", "C", "D"}
	if !isSliceSame(a, b) {
		t.Errorf("Slices should have been same")
	}
}

func Test_isSliceSame_Fail(t *testing.T) {
	a := []string{"A", "B", "C", "D"}
	b := []string{"A", "B", "D", "C"}
	if isSliceSame(a, b) {
		t.Errorf("Slices should have been different")
	}
}

func Test_sliceContains_Pass(t *testing.T) {
	var a []Chain
	a = append(a, Chain{"A", "B", "C"})
	a = append(a, Chain{"B", "C"})
	a = append(a, Chain{"C", "A", "B"})
	b := Chain{"B", "C"}
	if !sliceContains(a, b) {
		t.Errorf("Slice a should have b")
	}
}

func Test_sliceContains_Fail(t *testing.T) {
	var a []Chain
	a = append(a, Chain{"A", "B", "C"})
	a = append(a, Chain{"B", "C"})
	a = append(a, Chain{"C", "A", "B"})
	b := Chain{"E", "C"}
	if sliceContains(a, b) {
		t.Errorf("Slice a should not have b")
	}
}

func getGoModGraphTestData() string {
	/*
		Graph:
		         A
		       / | \
		     G   B   D
		     | \ |  / \
		     F   C     E
	*/
	goModGraphOutputString := `A@1.1 G@1.5
A@1.1 B@1.3
A@1.1 D@1.2
G@1.5 F@1.3
G@1.5 C@1.1
B@1.3 C@1.1
D@1.2 C@1.1
D@1.2 E@1.8`

	return goModGraphOutputString
}
func Test_generateGraph_empty_mainModule(t *testing.T) {
	depGraph := generateGraph(getGoModGraphTestData(), nil)

	transitiveDependencyList := []string{"F", "C", "E"}
	directDependencyList := []string{"G", "B", "D"}

	if depGraph.MainModules[0] != "A" {
		t.Errorf(`"A" must be the main module`)
	}

	if !isSliceSame(depGraph.DirectDepList, directDependencyList) {
		t.Errorf("Expected direct dependecies are %s but got %s", directDependencyList, depGraph.DirectDepList)
	}

	if !isSliceSame(depGraph.TransDepList, transitiveDependencyList) {
		t.Errorf("Expected transitive dependencies are %s but got %s", transitiveDependencyList, depGraph.TransDepList)
	}
}

func Test_generateGraph_custom_mainModule(t *testing.T) {
	mainModules := []string{"A", "D"}
	depGraph := generateGraph(getGoModGraphTestData(), mainModules)

	transitiveDependencyList := []string{"F", "C"}
	directDependencyList := []string{"G", "B", "C", "E"}

	if !isSliceSame(depGraph.MainModules, mainModules) {
		t.Errorf("Expected mainModules are %s but got %s", mainModules, depGraph.MainModules)
	}

	if !isSliceSame(depGraph.DirectDepList, directDependencyList) {
		t.Errorf("Expected direct dependecies are %s but got %s", directDependencyList, depGraph.DirectDepList)
	}

	if !isSliceSame(depGraph.TransDepList, transitiveDependencyList) {
		t.Errorf("Expected transitive dependencies are %s but got %s", transitiveDependencyList, depGraph.TransDepList)
	}
}

func Test_parseModWhyOutput_testOnly(t *testing.T) {
	output := `# github.com/prod/dep
main/pkg
github.com/prod/dep/internal

# github.com/test/dep
main/pkg/foo.test
main/pkg/foo/testing
github.com/test/dep

# github.com/unused/dep
(main module does not need module github.com/unused/dep)

# github.com/another/prod
main/cmd
github.com/another/prod/lib
`
	result := parseModWhyOutput(output)

	// github.com/test/dep should be test-only (has .test in path)
	if !result["github.com/test/dep"] {
		t.Errorf("expected github.com/test/dep to be test-only")
	}

	// github.com/prod/dep should NOT be test-only
	if result["github.com/prod/dep"] {
		t.Errorf("expected github.com/prod/dep to NOT be test-only")
	}

	// github.com/another/prod should NOT be test-only
	if result["github.com/another/prod"] {
		t.Errorf("expected github.com/another/prod to NOT be test-only")
	}

	// github.com/unused/dep should NOT be in the set (it's unused, not test-only)
	if result["github.com/unused/dep"] {
		t.Errorf("expected github.com/unused/dep to NOT be in test-only set")
	}
}

func Test_parseModWhyOutput_empty(t *testing.T) {
	result := parseModWhyOutput("")
	if len(result) != 0 {
		t.Errorf("expected empty result for empty input, got %v", result)
	}
}

func Test_parseModWhyOutput_allTestOnly(t *testing.T) {
	output := `# github.com/a
main/pkg.test
github.com/a

# github.com/b
main/other.test
main/other/testutil
github.com/b/pkg
`
	result := parseModWhyOutput(output)
	if !result["github.com/a"] {
		t.Errorf("expected github.com/a to be test-only")
	}
	if !result["github.com/b"] {
		t.Errorf("expected github.com/b to be test-only")
	}
}

func Test_parseModWhyOutput_noTestOnly(t *testing.T) {
	output := `# github.com/a
main/pkg
github.com/a

# github.com/b
main/cmd
github.com/b/pkg
`
	result := parseModWhyOutput(output)
	if len(result) != 0 {
		t.Errorf("expected no test-only deps, got %v", result)
	}
}

func Test_parseVendorModulesTxt(t *testing.T) {
	content := `# github.com/foo/bar v1.2.3
## explicit; go 1.19
github.com/foo/bar
# github.com/baz/qux v0.5.0
## explicit; go 1.20
github.com/baz/qux
# github.com/replaced/mod => github.com/fork/mod v1.0.0
## explicit; go 1.21
github.com/replaced/mod
# github.com/another/one v2.0.0
## explicit; go 1.22
github.com/another/one
`
	modules := parseVendorModulesTxt(content)
	if len(modules) != 3 {
		t.Fatalf("expected 3 modules, got %d: %v", len(modules), modules)
	}
	if modules[0].Path != "github.com/foo/bar" || modules[0].Version != "v1.2.3" {
		t.Errorf("module 0: got %+v", modules[0])
	}
	if modules[1].Path != "github.com/baz/qux" || modules[1].Version != "v0.5.0" {
		t.Errorf("module 1: got %+v", modules[1])
	}
	if modules[2].Path != "github.com/another/one" || modules[2].Version != "v2.0.0" {
		t.Errorf("module 2: got %+v", modules[2])
	}
}

func Test_parseVendorModulesTxt_empty(t *testing.T) {
	modules := parseVendorModulesTxt("")
	if len(modules) != 0 {
		t.Errorf("expected 0 modules for empty input, got %d", len(modules))
	}
}

func Test_parseVendorModulesTxt_replacementWithoutVersion(t *testing.T) {
	content := `# example.com/mod => ../local/mod
## explicit
example.com/mod
# example.com/dep v1.2.3
## explicit
example.com/dep
`
	modules := parseVendorModulesTxt(content)
	if len(modules) != 1 {
		t.Fatalf("expected 1 module, got %d: %v", len(modules), modules)
	}
	if modules[0].Path != "example.com/dep" || modules[0].Version != "v1.2.3" {
		t.Errorf("module 0: got %+v", modules[0])
	}
}

func Test_computeVendorOnlyRemovals(t *testing.T) {
	vendorRemoved := []VendorModule{
		{Path: "github.com/a", Version: "v1.0.0"},
		{Path: "github.com/b", Version: "v1.0.0"},
		{Path: "github.com/c", Version: "v1.0.0"},
	}
	graphRemoved := []string{"github.com/b"}
	got := computeVendorOnlyRemovals(vendorRemoved, graphRemoved)
	if len(got) != 2 {
		t.Fatalf("expected 2 vendor-only removals, got %d: %v", len(got), got)
	}
	if got[0].Path != "github.com/a" || got[1].Path != "github.com/c" {
		t.Fatalf("unexpected vendor-only removals: %v", got)
	}
}

func Test_versionGreater(t *testing.T) {
	if !versionGreater("v1.10.0", "v1.9.0") {
		t.Fatalf("expected semver comparison to treat v1.10.0 > v1.9.0")
	}
	if versionGreater("v1.9.0", "v1.10.0") {
		t.Fatalf("expected semver comparison to treat v1.9.0 < v1.10.0")
	}
	if !versionGreater("z-non-semver", "a-non-semver") {
		t.Fatalf("expected lexical fallback to compare non-semver strings")
	}
	if versionGreater("v1.2.3", "v1.2.3") {
		t.Fatalf("equal versions should not compare as greater")
	}
}

func Test_computeVersionChanges(t *testing.T) {
	base := &DependencyOverview{
		DirectDepList: []string{"B", "C", "D"},
		TransDepList:  []string{"E"},
		MainModules:   []string{"A"},
		Versions: map[string]string{
			"B": "v1.0.0",
			"C": "v2.0.0",
			"D": "v1.5.0",
			"E": "v0.3.0",
		},
	}
	head := &DependencyOverview{
		DirectDepList: []string{"B", "C", "D"},
		TransDepList:  []string{"E"},
		MainModules:   []string{"A"},
		Versions: map[string]string{
			"B": "v1.1.0", // changed
			"C": "v2.0.0", // same
			"D": "v1.5.0", // same
			"E": "v0.4.0", // changed
		},
	}
	changes := computeVersionChanges(base, head)
	if len(changes) != 2 {
		t.Fatalf("expected 2 version changes, got %d: %v", len(changes), changes)
	}
	// Sorted by path
	if changes[0].Path != "B" || changes[0].Before != "v1.0.0" || changes[0].After != "v1.1.0" {
		t.Errorf("change 0: got %+v", changes[0])
	}
	if changes[1].Path != "E" || changes[1].Before != "v0.3.0" || changes[1].After != "v0.4.0" {
		t.Errorf("change 1: got %+v", changes[1])
	}
}

func Test_computeVersionChanges_removedModule(t *testing.T) {
	base := &DependencyOverview{
		DirectDepList: []string{"B", "C"},
		TransDepList:  []string{},
		MainModules:   []string{"A"},
		Versions: map[string]string{
			"B": "v1.0.0",
			"C": "v2.0.0",
		},
	}
	head := &DependencyOverview{
		DirectDepList: []string{"B"},
		TransDepList:  []string{},
		MainModules:   []string{"A"},
		Versions: map[string]string{
			"B": "v1.1.0",
		},
	}
	changes := computeVersionChanges(base, head)
	// C was removed — should not appear as a version change
	if len(changes) != 1 {
		t.Fatalf("expected 1 version change, got %d: %v", len(changes), changes)
	}
	if changes[0].Path != "B" {
		t.Errorf("expected B, got %s", changes[0].Path)
	}
}

func Test_generateGraph_versions(t *testing.T) {
	depGraph := generateGraph(getGoModGraphTestData(), nil)
	if depGraph.Versions["G"] != "1.5" {
		t.Errorf("expected G version 1.5, got %s", depGraph.Versions["G"])
	}
	if depGraph.Versions["B"] != "1.3" {
		t.Errorf("expected B version 1.3, got %s", depGraph.Versions["B"])
	}
	if depGraph.Versions["E"] != "1.8" {
		t.Errorf("expected E version 1.8, got %s", depGraph.Versions["E"])
	}
}

func Test_generateGraph_overridden_versions(t *testing.T) {
	mainModules := []string{"A", "D"}
	// obsolete C@v1 has a cycle with D@v1 and a transitive ref to unwanted dependency E@v1
	// effective version C@v2 updates to D@v2, which still has a cycle back to C@v2, but no dependency on E
	depGraph := generateGraph(`A B@v2
A C@v2
A D@v2
B@v2 C@v1
C@v1 D@v1
D@v1 C@v1
D@v1 E@v1
C@v2 D@v2
C@v2 F@v2
D@v2 C@v2
D@v2 G@v2`, mainModules)

	transitiveDependencyList := []string{"C", "D", "F"}
	directDependencyList := []string{"B", "C", "G"}

	if !isSliceSame(depGraph.MainModules, mainModules) {
		t.Errorf("Expected mainModules are %s but got %s", mainModules, depGraph.MainModules)
	}

	if !isSliceSame(depGraph.DirectDepList, directDependencyList) {
		t.Errorf("Expected direct dependecies are %s but got %s", directDependencyList, depGraph.DirectDepList)
	}

	if !isSliceSame(depGraph.TransDepList, transitiveDependencyList) {
		t.Errorf("Expected transitive dependencies are %s but got %s", transitiveDependencyList, depGraph.TransDepList)
	}
}

func Test_generateGraph_skipsMalformedLines(t *testing.T) {
	depGraph := generateGraph(`A B@v1.0.0
malformed

go@1.22.0 toolchain@go1.22.0
A C@v1.0.0`, nil)

	if len(depGraph.MainModules) == 0 || depGraph.MainModules[0] != "A" {
		t.Fatalf("expected main module A, got %v", depGraph.MainModules)
	}
	if !contains(depGraph.DirectDepList, "B") || !contains(depGraph.DirectDepList, "C") {
		t.Fatalf("expected B and C to be direct deps, got %v", depGraph.DirectDepList)
	}
}

func Test_computeStats_noMainModule(t *testing.T) {
	stats := computeStats(&DependencyOverview{
		Graph:         map[string][]string{"A": []string{"B"}},
		DirectDepList: []string{"B"},
		TransDepList:  []string{},
		MainModules:   nil,
	})
	if stats.MaxDepth != 0 {
		t.Fatalf("expected max depth 0 when no main module, got %d", stats.MaxDepth)
	}
	if stats.TotalDeps != 1 || stats.DirectDeps != 1 || stats.TransDeps != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}
