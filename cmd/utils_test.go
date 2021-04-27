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

	deps := []string{"A", "B", "C", "D", "E", "F", "H"}

	var cycleChains []Chain
	var longestChain Chain
	var chains []Chain
	var temp Chain
	getLongestChain("A", graph, temp, &longestChain)
	maxDepth := len(longestChain)
	getCycleChains("A", graph, temp, &cycleChains)
	getAllChains("A", graph, temp, &chains)
	cycles := getCycles(cycleChains)

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

	if correctFileContentsForAllDeps != getFileContentsForAllDeps(deps, graph, "A") {
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

	deps := []string{"A", "B", "C", "D", "E", "F", "G", "H"}

	var cycleChains []Chain
	var longestChain Chain
	var chains []Chain
	var temp Chain
	getLongestChain("A", graph, temp, &longestChain)
	maxDepth := len(longestChain)
	getCycleChains("A", graph, temp, &cycleChains)
	getAllChains("A", graph, temp, &chains)
	cycles := getCycles(cycleChains)

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
	if correctFileContentsForAllDeps != getFileContentsForAllDeps(deps, graph, "A") {
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

	cyc := []string{"D", "F", "G", "H", "D"}

	if len(cycles) != 1 {
		t.Errorf("Number of cycles is not correct")
	}

	if !isSliceSame(cycles[0], cyc) {
		t.Errorf("Cycle is not correct")
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

	deps := []string{"A", "B", "C", "D", "E", "F"}

	var cycleChains []Chain
	var longestChain Chain
	var chains []Chain
	var temp Chain
	getLongestChain("A", graph, temp, &longestChain)
	maxDepth := len(longestChain)
	getCycleChains("A", graph, temp, &cycleChains)
	getAllChains("A", graph, temp, &chains)
	cycles := getCycles(cycleChains)

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
	if correctFileContentsForAllDeps != getFileContentsForAllDeps(deps, graph, "A") {
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

	if len(cycles) != 3 {
		t.Errorf("Number of cycles is incorrect")
	}
	cyc1 := []string{"B", "C", "B"}
	cyc2 := []string{"C", "E", "F", "D", "C"}
	cyc3 := []string{"C", "B", "C"}

	if !isSliceSame(cycles[0], cyc1) {
		t.Errorf("B C B cycle is incorrect")
	}

	if !isSliceSame(cycles[1], cyc2) {
		t.Errorf("C E F D C cycle is incorrect")
	}

	if !isSliceSame(cycles[2], cyc3) {
		t.Errorf("C B C cycle is incorrect")
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
