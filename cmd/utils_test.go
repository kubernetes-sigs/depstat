package cmd

import (
	"testing"
)

func Test_dfs_simple(t *testing.T) {

	/*
		Graph:
				  A
				/ | \
			   B  C  D
				\/   |
				E	G
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
	graph["B"] = []string{"E"}

	dp := make(map[string]int)
	visited := make(map[string]bool)
	longestPath := make(map[string]string)
	for k := range graph {
		if visited[k] == false {
			dfs(k, graph, dp, visited, longestPath)
		}
	}
	if dp["A"] != 4 {
		t.Errorf("Max depth of dependencies was incorrect")
	}
	if longestPath["A"] != "C" || longestPath["C"] != "E" || longestPath["E"] != "F" || longestPath["F"] != "H" {
		t.Errorf("Longest path was incorrect")
	}
}

func Test_dfs_cycle(t *testing.T) {

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

	dp := make(map[string]int)
	visited := make(map[string]bool)
	longestPath := make(map[string]string)
	for k := range graph {
		if visited[k] == false {
			dfs(k, graph, dp, visited, longestPath)
		}
	}
	if dp["A"]-1 != 5 {
		t.Errorf("Max depth of dependencies was incorrect")
	}
	if longestPath["A"] != "B" || longestPath["B"] != "D" || longestPath["D"] != "F" || longestPath["F"] != "G" || longestPath["G"] != "H" || longestPath["H"] != "D" {
		t.Errorf("Longest path was incorrect")
	}
}

func Test_dfs_cycle_2(t *testing.T) {

	/*
		Graph:
					 A
				   /
				  B
				 ||
				  C
				/   \
				D	E
				 \ /
				  F
	*/

	graph := make(map[string][]string)
	graph["A"] = []string{"B"}
	graph["B"] = []string{"C"}
	graph["C"] = []string{"B", "D", "E"}
	graph["D"] = []string{"F"}
	graph["E"] = []string{"F"}

	dp := make(map[string]int)
	visited := make(map[string]bool)
	longestPath := make(map[string]string)
	for k := range graph {
		if visited[k] == false {
			dfs(k, graph, dp, visited, longestPath)
		}
	}
	if dp["A"] != 4 {
		t.Errorf("Max depth of dependencies was incorrect")
	}
	if longestPath["A"] != "B" || longestPath["B"] != "C" || longestPath["C"] != "D" || longestPath["D"] != "F" {
		t.Errorf("Longest path was incorrect")
	}
}
