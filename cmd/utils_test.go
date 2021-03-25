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

	chains := make(map[int][]string)
	var temp []string
	getChains("A", graph, temp, chains)
	maxDepth := getMaxDepth(chains)

	if maxDepth != 4 {
		t.Errorf("Max depth of dependencies was incorrect")
	}

	longestPath := []string{"A", "C", "E", "F", "H"}

	if !isSliceSame(chains[maxDepth+1], longestPath) {
		t.Errorf("Longest path was incorrect")
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

	chains := make(map[int][]string)
	var temp []string
	getChains("A", graph, temp, chains)
	maxDepth := getMaxDepth(chains)

	if maxDepth != 5 {
		t.Errorf("Max depth of dependencies was incorrect")
	}

	longestPath := []string{"A", "B", "D", "F", "G", "H"}
	if !isSliceSame(chains[maxDepth+1], longestPath) {
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

	chains := make(map[int][]string)
	var temp []string
	getChains("A", graph, temp, chains)
	maxDepth := getMaxDepth(chains)
	if maxDepth != 5 {
		t.Errorf("Max depth of dependencies was incorrect")
	}

	longestPath := []string{"A", "B", "C", "E", "F", "D"}
	if !isSliceSame(chains[maxDepth+1], longestPath) {
		t.Errorf("Longest path was incorrect")
	}
}
