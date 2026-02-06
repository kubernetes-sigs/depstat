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

	"github.com/spf13/cobra"
)

var listSplitTestOnly bool

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

		depGraph := getDepInfo(nil)
		allDeps := getAllDeps(depGraph.DirectDepList, depGraph.TransDepList)

		if listSplitTestOnly {
			testOnlySet, err := classifyTestDeps(allDeps)
			if err != nil {
				return fmt.Errorf("failed to classify dependencies: %w", err)
			}
			nonTest := filterDepsByTestStatus(allDeps, testOnlySet, false)
			testOnly := filterDepsByTestStatus(allDeps, testOnlySet, true)
			fmt.Printf("Non-test dependencies (%d):\n", len(nonTest))
			printDeps(nonTest)
			fmt.Printf("\nTest-only dependencies (%d):\n", len(testOnly))
			printDeps(testOnly)
		} else {
			fmt.Println("List of all dependencies:")
			printDeps(allDeps)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVar(&listSplitTestOnly, "split-test-only", false, "Split list into test-only and non-test sections (uses go mod why -m)")
}
