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
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var dep string

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Generate a .dot file to be used with Graphviz's dot command.",
	Long: `A graph.dot file will be generated which can be used with Graphviz's dot command.
	For example to generate a svg image use:
	twopi -Tsvg -o dag.svg graph.dot`,
	RunE: func(cmd *cobra.Command, args []string) error {
		overview := getDepInfo()
		// strict ensures that there is only one edge between two vertices
		// overlap = false ensures the vertices don't overlap
		fileContents := "strict digraph {\ngraph [overlap=false];\n"

		// graph to be generated is based around input dep
		if dep != "" {
			var chains []Chain
			var temp Chain
			getAllChains(overview.MainModuleName, overview.Graph, temp, &chains)
			fileContents += getFileContentsForSingleDep(chains, dep)
		} else {
			fileContents += getFileContentsForAllDeps(overview)
		}
		fileContents += "}"
		fileContentsByte := []byte(fileContents)
		err := ioutil.WriteFile("./graph.dot", fileContentsByte, 0644)
		if err != nil {
			return err
		}
		fmt.Println("\nCreated graph.dot file!")
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
// when the -d flag is set
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
// of all dependencies (when -d is not set)
func getFileContentsForAllDeps(overview *DependencyOverview) string {

	// color the main module as yellow
	data := colorMainNode(overview.MainModuleName)
	allDeps := getAllDeps(overview.Graph[overview.MainModuleName], overview.TransDepList)
	allDeps = append(allDeps, overview.MainModuleName)
	sort.Strings(allDeps)
	for _, dep := range allDeps {
		_, ok := overview.Graph[dep]
		if !ok {
			continue
		}
		// main module can never be a neighbour
		for _, neighbour := range overview.Graph[dep] {
			if dep == overview.MainModuleName {
				// for the main module use a colored node
				data += fmt.Sprintf("\"MainNode\" -> \"%s\"\n", neighbour)
			} else {
				data += fmt.Sprintf("\"%s\" -> \"%s\"\n", dep, neighbour)
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
	graphCmd.Flags().StringVarP(&dep, "dep", "d", "", "Specify dependency to create a graph around")
}
