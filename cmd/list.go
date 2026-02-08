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
	"sort"

	"github.com/spf13/cobra"
)

var listSplitTestOnly bool
var listJSONOutput bool

// analyzeDepsCmd represents the analyzeDeps command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists all project dependencies",
	Long: `Gives a list of all the dependencies of the project.
	These include both direct as well as transitive dependencies.`,
	RunE: func(cmd *cobra.Command, args []string) error {

		if len(args) != 0 {
			return fmt.Errorf("list does not take any arguments")
		}

		depGraph := getDepInfo(mainModules)
		if len(depGraph.MainModules) == 0 {
			return fmt.Errorf("no main modules remain after exclusions; adjust --exclude-modules or --mainModules")
		}
		allDeps := getAllDeps(depGraph.DirectDepList, depGraph.TransDepList)
		sort.Strings(allDeps)

		if listSplitTestOnly {
			testOnlySet, err := classifyTestDeps(allDeps)
			if err != nil {
				return fmt.Errorf("failed to classify dependencies: %w", err)
			}
			nonTest := filterDepsByTestStatus(allDeps, testOnlySet, false)
			testOnly := filterDepsByTestStatus(allDeps, testOnlySet, true)
			sort.Strings(nonTest)
			sort.Strings(testOnly)
			if listJSONOutput {
				outputObj := struct {
					All       []string `json:"allDependencies"`
					NonTest   []string `json:"nonTestDependencies"`
					TestOnly  []string `json:"testOnlyDependencies"`
					MainMods  []string `json:"mainModules"`
					Total     int      `json:"totalDependencies"`
					NonTestN  int      `json:"nonTestCount"`
					TestOnlyN int      `json:"testOnlyCount"`
				}{
					All:       allDeps,
					NonTest:   nonTest,
					TestOnly:  testOnly,
					MainMods:  depGraph.MainModules,
					Total:     len(allDeps),
					NonTestN:  len(nonTest),
					TestOnlyN: len(testOnly),
				}
				outputRaw, err := json.MarshalIndent(outputObj, "", "\t")
				if err != nil {
					return err
				}
				fmt.Print(string(outputRaw))
				return nil
			}
			fmt.Printf("Non-test dependencies (%d):\n", len(nonTest))
			printDeps(nonTest)
			fmt.Printf("\nTest-only dependencies (%d):\n", len(testOnly))
			printDeps(testOnly)
		} else {
			if listJSONOutput {
				outputObj := struct {
					All      []string `json:"allDependencies"`
					MainMods []string `json:"mainModules"`
					Total    int      `json:"totalDependencies"`
				}{
					All:      allDeps,
					MainMods: depGraph.MainModules,
					Total:    len(allDeps),
				}
				outputRaw, err := json.MarshalIndent(outputObj, "", "\t")
				if err != nil {
					return err
				}
				fmt.Print(string(outputRaw))
				return nil
			}
			fmt.Println("List of all dependencies:")
			printDeps(allDeps)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&dir, "dir", "d", "", "Directory containing the module to evaluate. Defaults to the current directory.")
	listCmd.Flags().StringSliceVarP(&mainModules, "mainModules", "m", []string{}, "Specify main modules")
	listCmd.Flags().StringSliceVar(&excludeModules, "exclude-modules", []string{}, "Exclude module path patterns (repeatable, supports * wildcard)")
	listCmd.Flags().BoolVarP(&listJSONOutput, "json", "j", false, "Get the output in JSON format")
	listCmd.Flags().BoolVar(&listSplitTestOnly, "split-test-only", false, "Split list into test-only and non-test sections (uses go mod why -m)")
}
