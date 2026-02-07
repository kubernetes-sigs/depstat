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
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var dep string
var showEdgeTypes bool
var graphDotOutput bool
var graphJSONOutput bool
var graphOutputPath string

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Generate a .dot file to be used with Graphviz's dot command.",
	Long: `A graph.dot file will be generated which can be used with Graphviz's dot command.
	For example to generate a svg image use:
	twopi -Tsvg -o dag.svg graph.dot

	Use --show-edge-types to distinguish between direct and transitive dependencies:
	- Direct edges (solid blue): from main module(s) to their direct dependencies
	- Transitive edges (dashed gray): dependencies of dependencies`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if graphDotOutput && graphJSONOutput {
			return fmt.Errorf("--dot and --json are mutually exclusive")
		}
		overview := getDepInfo(mainModules)
		if len(overview.MainModules) == 0 {
			return fmt.Errorf("could not determine main module; run from a Go module directory or set --mainModules")
		}
		// strict ensures that there is only one edge between two vertices
		// overlap = false ensures the vertices don't overlap
		fileContents := "strict digraph {\ngraph [overlap=false];\n"

		// graph to be generated is based around input dep
		if dep != "" {
			var chains []Chain
			var temp Chain
			getAllChains(overview.MainModules[0], overview.Graph, temp, &chains)
			fileContents += getFileContentsForSingleDep(chains, dep)
		} else {
			fileContents += getFileContentsForAllDepsWithTypes(overview, showEdgeTypes)
		}
		fileContents += "}"
		if graphJSONOutput {
			edges := getEdges(overview.Graph)
			outputObj := struct {
				MainModules         []string            `json:"mainModules"`
				DirectDependencies  []string            `json:"directDependencies"`
				TransDependencies   []string            `json:"transitiveDependencies"`
				Graph               map[string][]string `json:"graph"`
				Edges               []string            `json:"edges"`
				FocusedDependency   string              `json:"focusedDependency,omitempty"`
				ShowEdgeTypes       bool                `json:"showEdgeTypes"`
				DirectCount         int                 `json:"directDependencyCount"`
				TransitiveCount     int                 `json:"transitiveDependencyCount"`
				TotalDependencyEdge int                 `json:"edgeCount"`
			}{
				MainModules:         overview.MainModules,
				DirectDependencies:  overview.DirectDepList,
				TransDependencies:   overview.TransDepList,
				Graph:               overview.Graph,
				Edges:               edges,
				FocusedDependency:   dep,
				ShowEdgeTypes:       showEdgeTypes,
				DirectCount:         len(overview.DirectDepList),
				TransitiveCount:     len(overview.TransDepList),
				TotalDependencyEdge: len(edges),
			}
			out, err := json.MarshalIndent(outputObj, "", "\t")
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		}
		if graphDotOutput {
			fmt.Print(fileContents)
			return nil
		}

		fileContentsByte := []byte(fileContents)
		err := os.WriteFile(graphOutputPath, fileContentsByte, 0644)
		if err != nil {
			return err
		}
		fmt.Printf("\nCreated %s file!\n", graphOutputPath)
		return nil
	},
}

// find all possible chains starting from currentDep
func getAllChains(currentDep string, graph map[string][]string, currentChain Chain, chains *[]Chain) {
	currentChain = append(currentChain, currentDep)
	_, ok := graph[currentDep]
	if ok {
		for _, dep := range graph[currentDep] {
			if !contains(currentChain, dep) {
				cpy := make(Chain, len(currentChain))
				copy(cpy, currentChain)
				getAllChains(dep, graph, cpy, chains)
			} else {
				*chains = append(*chains, currentChain)
			}
		}
	} else {
		*chains = append(*chains, currentChain)
	}
}

// get the contents of the .dot file for the graph
// when the --dep flag is set
func getFileContentsForSingleDep(chains []Chain, dep string) string {
	// to color the entered node as yellow
	data := colorMainNode(dep)

	// add all chains which have the input dep to the .dot file
	for _, chain := range chains {
		if chainContains(chain, dep) {
			for i := range chain {
				if chain[i] == dep {
					chain[i] = "MainNode"
				} else {
					chain[i] = "\"" + chain[i] + "\""
				}
			}
			data += strings.Join(chain, " -> ")
			data += "\n"
		}
	}
	return data
}

// get the contents of the .dot file for the graph
// of all dependencies (when --dep is not set)
func getFileContentsForAllDeps(overview *DependencyOverview) string {
	return getFileContentsForAllDepsWithTypes(overview, false)
}

// getFileContentsForAllDepsWithTypes generates DOT content with optional edge type annotations
func getFileContentsForAllDepsWithTypes(overview *DependencyOverview, showTypes bool) string {
	if len(overview.MainModules) == 0 {
		return ""
	}
	// color the main module as yellow
	data := colorMainNode(overview.MainModules[0])

	// Create a set of main modules for quick lookup
	mainModSet := make(map[string]bool)
	for _, m := range overview.MainModules {
		mainModSet[m] = true
	}

	// Create a set of direct dependencies for quick lookup
	directDepSet := make(map[string]bool)
	for _, d := range overview.DirectDepList {
		directDepSet[d] = true
	}

	allDeps := getAllDeps(overview.DirectDepList, overview.TransDepList)
	allDeps = append(allDeps, overview.MainModules[0])
	sort.Strings(allDeps)

	for _, dep := range allDeps {
		_, ok := overview.Graph[dep]
		if !ok {
			continue
		}
		// main module can never be a neighbour
		for _, neighbour := range overview.Graph[dep] {
			var edgeAttrs string
			if showTypes {
				if mainModSet[dep] {
					// Edge from main module = direct dependency
					edgeAttrs = " [color=\"blue\", style=\"bold\", edgetype=\"direct\"]"
				} else {
					// Edge from non-main module = transitive dependency
					edgeAttrs = " [color=\"gray\", style=\"dashed\", edgetype=\"transitive\"]"
				}
			}

			if mainModSet[dep] {
				// for the main module use a colored node
				data += fmt.Sprintf("\"MainNode\" -> \"%s\"%s\n", neighbour, edgeAttrs)
			} else {
				data += fmt.Sprintf("\"%s\" -> \"%s\"%s\n", dep, neighbour, edgeAttrs)
			}
		}
	}
	return data
}

func chainContains(chain Chain, dep string) bool {
	for _, d := range chain {
		if d == dep {
			return true
		}
	}
	return false
}

func colorMainNode(mainNode string) string {
	return fmt.Sprintf("MainNode [label=\"%s\", style=\"filled\" color=\"yellow\"]\n", mainNode)
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.Flags().StringVarP(&dir, "dir", "d", "", "Directory containing the module to evaluate. Defaults to the current directory.")
	graphCmd.Flags().StringVarP(&dep, "dep", "p", "", "Specify dependency to create a graph around")
	graphCmd.Flags().BoolVar(&showEdgeTypes, "show-edge-types", false, "Distinguish direct vs transitive edges with colors/styles")
	graphCmd.Flags().BoolVar(&graphDotOutput, "dot", false, "Output DOT graph to stdout")
	graphCmd.Flags().BoolVarP(&graphJSONOutput, "json", "j", false, "Output graph data in JSON format")
	graphCmd.Flags().StringVar(&graphOutputPath, "output", "graph.dot", "Path to DOT output file when not using --dot or --json")
	graphCmd.Flags().StringSliceVarP(&mainModules, "mainModules", "m", []string{}, "Specify main modules")
}
