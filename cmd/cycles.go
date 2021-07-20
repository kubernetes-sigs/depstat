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

var jsonOutputCycles bool

// analyzeDepsCmd represents the analyzeDeps command
var cyclesCmd = &cobra.Command{
	Use:   "cycles",
	Short: "Prints cycles in dependency chains.",
	Long:  `Will show all the cycles in the dependencies of the project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		overview := getDepInfo([]string{})
		var cycleChains []Chain
		var temp Chain
		getCycleChains(overview.MainModuleName, overview.Graph, temp, &cycleChains)
		cycles := getCycles(cycleChains)

		if !jsonOutputCycles {
			fmt.Println("All cycles in dependencies are: ")
			for _, c := range cycles {
				printChain(c)
			}
		} else {
			// create json
			outputObj := struct {
				Cycles []Chain `json:"cycles"`
			}{
				Cycles: cycles,
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

// get all chains which have a cycle
func getCycleChains(currentDep string, graph map[string][]string, currentChain Chain, cycleChains *[]Chain) {
	currentChain = append(currentChain, currentDep)
	_, ok := graph[currentDep]
	if ok {
		for _, dep := range graph[currentDep] {
			if !contains(currentChain, dep) {
				cpy := make(Chain, len(currentChain))
				copy(cpy, currentChain)
				getCycleChains(dep, graph, cpy, cycleChains)
			} else {
				*cycleChains = append(*cycleChains, append(currentChain, dep))
			}
		}
	}
}

// gets the cycles from the cycleChains
func getCycles(cycleChains []Chain) []Chain {
	var cycles []Chain
	for _, chain := range cycleChains {
		var cycle Chain
		start := false
		startDep := chain[len(chain)-1]
		for _, val := range chain {
			if val == startDep {
				start = true
			}
			if start {
				cycle = append(cycle, val)
			}
		}
		if !sliceContains(cycles, cycle) {
			cycles = append(cycles, cycle)
		}
	}
	return cycles
}

func init() {
	rootCmd.AddCommand(cyclesCmd)
	cyclesCmd.Flags().BoolVarP(&jsonOutputCycles, "json", "j", false, "Get the output in JSON format")
}
