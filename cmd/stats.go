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

	"github.com/spf13/cobra"
)

var dir string
var jsonOutput bool
var verbose bool
var mainModules []string

type Chain []string

// statsCmd represents the statsDeps command
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Shows metrics about dependency chains",
	Long: `Provides the following metrics:
	1. Direct Dependencies: Total number of dependencies required by the mainModule(s) directly
	2. Transitive Dependencies: Total number of transitive dependencies (dependencies which are further needed by direct dependencies of the project)
	3. Total Dependencies: Total number of dependencies of the mainModule(s)
	4. Max Depth of Dependencies: Length of the longest chain starting from the first mainModule; defaults to length from the first module encountered in "go mod graph" output`,
	RunE: func(cmd *cobra.Command, args []string) error {
		depGraph := getDepInfo(mainModules)

		if len(args) != 0 {
			return fmt.Errorf("stats does not take any arguments")
		}

		// get the longest chain
		var temp Chain
		longestChain := getLongestChain(depGraph.MainModules[0], depGraph.Graph, temp, map[string]Chain{})
		// get values
		maxDepth := len(longestChain)
		directDeps := len(depGraph.DirectDepList)
		transitiveDeps := len(depGraph.TransDepList)
		totalDeps := len(getAllDeps(depGraph.DirectDepList, depGraph.TransDepList))

		if !jsonOutput {
			fmt.Printf("Direct Dependencies: %d \n", directDeps)
			fmt.Printf("Transitive Dependencies: %d \n", transitiveDeps)
			fmt.Printf("Total Dependencies: %d \n", totalDeps)
			fmt.Printf("Max Depth Of Dependencies: %d \n", maxDepth)
		}

		if verbose {
			fmt.Println("All dependencies:")
			printDeps(getAllDeps(depGraph.DirectDepList, depGraph.TransDepList))
		}

		// print the longest chain
		if verbose {
			fmt.Println("Longest chain/s: ")
			printChain(longestChain)
		}

		if jsonOutput {
			// create json
			outputObj := struct {
				DirectDeps int `json:"directDependencies"`
				TransDeps  int `json:"transitiveDependencies"`
				TotalDeps  int `json:"totalDependencies"`
				MaxDepth   int `json:"maxDepthOfDependencies"`
			}{
				DirectDeps: directDeps,
				TransDeps:  transitiveDeps,
				TotalDeps:  totalDeps,
				MaxDepth:   maxDepth,
			}
			outputRaw, err := json.MarshalIndent(outputObj, "", "\t")
			if err != nil {
				return err
			}
			fmt.Print(string(outputRaw))
		}
		return nil
	},
}

// get the longest chain starting from currentDep
func getLongestChain(currentDep string, graph map[string][]string, currentChain Chain, longestChains map[string]Chain) Chain {
	// fmt.Println(strings.Repeat("  ", len(currentChain)), currentDep)

	// already computed
	if longestChain, ok := longestChains[currentDep]; ok {
		return longestChain
	}

	deps := graph[currentDep]

	if len(deps) == 0 {
		// we have no dependencies, our longest chain is just us
		longestChains[currentDep] = Chain{currentDep}
		return longestChains[currentDep]
	}

	if contains(currentChain, currentDep) {
		// we've already been visited in the current chain, avoid cycles but also don't record a longest chain for currentDep
		return nil
	}

	currentChain = append(currentChain, currentDep)
	// find the longest dependency chain
	var longestDepChain Chain
	for _, dep := range deps {
		depChain := getLongestChain(dep, graph, currentChain, longestChains)
		if len(depChain) > len(longestDepChain) {
			longestDepChain = depChain
		}
	}
	// prepend ourselves to the longest of our dependencies' chains and persist
	longestChains[currentDep] = append(Chain{currentDep}, longestDepChain...)
	return longestChains[currentDep]
}

func init() {
	rootCmd.AddCommand(statsCmd)
	statsCmd.Flags().StringVarP(&dir, "dir", "d", "", "Directory containing the module to evaluate. Defaults to the current directory.")
	statsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Get additional details")
	statsCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Get the output in JSON format")
	statsCmd.Flags().StringSliceVarP(&mainModules, "mainModules", "m", []string{}, "Enter modules whose dependencies should be considered direct dependencies; defaults to the first module encountered in `go mod graph` output")
}
