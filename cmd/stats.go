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
	3. Total Dependencies: Total number of dependencies of the project
	4. Max Depth of Dependencies: Number of dependencies in the longest dependency chain`,
	RunE: func(cmd *cobra.Command, args []string) error {
		depGraph := getDepInfo(mainModules)

		// get the longest chain
		var longestChain Chain
		var temp Chain
		getLongestChain(depGraph.MainModuleName, depGraph.Graph, temp, &longestChain)

		// get values
		maxDepth := len(longestChain)
		directDeps := 0
		if len(mainModules) == 0 {
			directDeps = len(depGraph.Graph[depGraph.MainModuleName])
		} else {
			var dirDeps []string
			for _, mainModule := range mainModules {
				dirDeps = getAllDeps(dirDeps, depGraph.Graph[mainModule])
			}
			directDeps = len(dirDeps)
		}
		totalDeps := 0
		if len(mainModules) == 0 {
			totalDeps = len(getAllDeps(depGraph.Graph[depGraph.MainModuleName], depGraph.TransDepList))
		} else {
			allDeps := depGraph.TransDepList
			for _, mainModule := range mainModules {
				allDeps = getAllDeps(allDeps, depGraph.Graph[mainModule])
			}
			totalDeps = len(allDeps)
		}
		transitiveDeps := len(depGraph.TransDepList)

		if !jsonOutput {
			fmt.Printf("Direct Dependencies: %d \n", directDeps)
			fmt.Printf("Transitive Dependencies: %d \n", transitiveDeps)
			fmt.Printf("Total Dependencies: %d \n", totalDeps)
			fmt.Printf("Max Depth Of Dependencies: %d \n", maxDepth)
		}

		if verbose {
			fmt.Println("All dependencies:")
			printDeps(append(depGraph.Graph[depGraph.MainModuleName], depGraph.TransDepList...))
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
func getLongestChain(currentDep string, graph map[string][]string, currentChain Chain, longestChain *Chain) {
	currentChain = append(currentChain, currentDep)
	_, ok := graph[currentDep]
	if ok {
		for _, dep := range graph[currentDep] {
			if !contains(currentChain, dep) {
				cpy := make(Chain, len(currentChain))
				copy(cpy, currentChain)
				getLongestChain(dep, graph, cpy, longestChain)
			} else {
				if len(currentChain) > len(*longestChain) {
					*longestChain = currentChain
				}
			}
		}
	} else {
		if len(currentChain) > len(*longestChain) {
			*longestChain = currentChain
		}
	}
}

func init() {
	rootCmd.AddCommand(statsCmd)
	statsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Get additional details")
	statsCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Get the output in JSON format")
	statsCmd.Flags().StringSliceVarP(&mainModules, "mainModules", "m", []string{}, "Enter packages you want to be considered as main modules")
}
