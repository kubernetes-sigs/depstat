package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// analyzeDepsCmd represents the analyzeDeps command
var analyzeDepsCmd = &cobra.Command{
	Use:   "analyzeDeps",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("analyzeDeps called")

		// get output of "go mod graph" in a string
		goModGraph := exec.Command("go", "mod", "graph")
		goModGraphOutput, err := goModGraph.Output()
		if err != nil {
			return err
		}
		goModGraphOutputString := string(goModGraphOutput)

		// create a graph of dependencies from that output
		depGraph := make(map[string][]string)
		scanner := bufio.NewScanner(strings.NewReader(goModGraphOutputString))
		deps := make(map[string]bool) // since no contains method for slices
		moduleName := "notset"
		for scanner.Scan() {
			line := scanner.Text()
			words := strings.Fields(line)
			// remove same versions
			words[0] = (strings.Split(words[0], "@"))[0]
			words[1] = (strings.Split(words[1], "@"))[0]
			if moduleName == "notset" {
				moduleName = words[0]
			}
			_, ok := deps[words[0]]
			if !ok {
				deps[words[0]] = true
			}
			_, ok = deps[words[1]]
			if !ok {
				deps[words[1]] = true
			}
			depGraph[words[0]] = append(depGraph[words[0]], words[1])

		}

		for k := range deps {
			fmt.Println(k)
		}

		dp := make(map[string]int)
		visited := make(map[string]bool)
		for k := range depGraph {
			dp[k] = 0
			visited[k] = false
		}
		for k := range depGraph {
			if visited[k] == false {
				dfs(k, depGraph, dp, visited)
			}
		}
		//fmt.Println(dp["test-proj"])

		// get values
		totalDeps := len(deps) - 1 // -1 for main module name
		maxDepth := dp[moduleName]
		directDeps := len(depGraph[moduleName])
		transitiveDeps := totalDeps - directDeps

		// create json
		outputObj := struct {
			TotalDeps int `json:"totalDependencies"`
			MaxDepth  int `json:"maxDepthOfDependencies"`
			TransDeps int `json:"transitiveDependencies"`
		}{
			TotalDeps: totalDeps,
			MaxDepth:  maxDepth,
			TransDeps: transitiveDeps,
		}
		outputRaw, err := json.Marshal(outputObj)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile("analysis.json", outputRaw, 0644)
		if err != nil {
			return err
		}

		return nil

	},
}

func init() {
	rootCmd.AddCommand(analyzeDepsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// analyzeDepsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// analyzeDepsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
