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
		depGraph, deps, mainModule := getDepInfo()
		// strict ensures that there is only one edge between two vertices
		// overlap = false ensures the vertices don't overlap
		fileContents := "strict digraph {\ngraph [overlap=false];\n"

		// graph to be generated is based around input dep
		if dep != "" {
			var cycleChains []Chain
			var chains []Chain
			var temp Chain
			getChains(mainModule, depGraph, temp, &chains, &cycleChains)
			// to color the entered node as yellow
			fileContents += fmt.Sprintf("MainNode [label=\"%s\", style=\"filled\" color=\"yellow\"]", dep)

			// add all chains which have the input dep to the .dot file
			for _, chain := range chains {
				if chainContains(chain, dep) {
					for i, d := range chain {
						if i == len(chain)-1 {
							if d == dep {
								// for the input dep use a colored node
								fileContents += "MainNode\n"
							} else {
								fileContents += fmt.Sprintf("\"%s\"\n", d)
							}
						} else {
							if d == dep {
								// for the input dep use a colored node
								fileContents += "MainNode -> "
							} else {
								fileContents += fmt.Sprintf("\"%s\" -> ", d)
							}
						}
					}
				}
			}
		} else {
			// color the main module as yellow
			fileContents += fmt.Sprintf("MainNode [label=\"%s\", style=\"filled\" color=\"yellow\"]", mainModule)
			for _, dep := range deps {
				_, ok := depGraph[dep]
				if !ok {
					continue
				}
				// main module can never be a neighbour
				for _, neighbour := range depGraph[dep] {
					if dep == mainModule {
						// for the main module use a colored node
						fileContents += fmt.Sprintf("\"MainNode\" -> \"%s\"\n", neighbour)
					} else {
						fileContents += fmt.Sprintf("\"%s\" -> \"%s\"\n", dep, neighbour)
					}
				}
			}
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

func chainContains(chain Chain, dep string) bool {
	for _, d := range chain {
		if d == dep {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.Flags().StringVarP(&dep, "dep", "d", "", "Specify dependency to create a graph around")
}
