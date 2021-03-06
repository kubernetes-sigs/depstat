package cmd

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// maxDepthCmd represents the maxDepth command
var maxDepthCmd = &cobra.Command{
	Use:   "maxDepth",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("maxDepth called")
		maxDepthCmd := exec.Command("go", "mod", "graph")
		output, err := maxDepthCmd.Output()
		if err != nil {
			return err
		}
		outputString := string(output)
		//fmt.Println(outputString)
		graph := make(map[string][]string)
		scanner := bufio.NewScanner(strings.NewReader(outputString))
		for scanner.Scan() {
			line := scanner.Text()
			words := strings.Fields(line)
			graph[words[0]] = append(graph[words[0]], words[1])
			//fmt.Println(scanner.Text())
		}
		// for k, v := range graph {
		// 	fmt.Println(k, v)
		// }
		//fmt.Println(graph["test-proj"][0])

		// get number of keys in graph

		// dp := make(map[string]int)
		// visited := make(map[string]bool)
		// for k := range graph {
		// 	dp[k] = 0
		// 	visited[k] = false
		// }
		// for k := range graph {
		// 	if visited[k] == false {
		// 		dfs(k, graph, dp, visited)
		// 	}
		// }
		//fmt.Println(dp["test-proj"])
		fmt.Println(getLen("test-proj", graph))
		return nil
	},
}

// Longest Path in Acyclic Graph:
// func dfs(k string, graph map[string][]string, dp map[string]int, visited map[string]bool) {

// 	visited[k] = true
// 	for _, u := range graph[k] {
// 		if visited[u] == false {
// 			dfs(u, graph, dp, visited)
// 		}
// 		dp[k] = Max(dp[k], 1+dp[u])
// 	}
// }

// My Logic:
func getLen(node string, graph map[string][]string) int {
	if _, ok := graph[node]; !ok {
		return 0
	}
	len := 0
	for _, nextNode := range graph[node] {
		len = Max(len, getLen(nextNode, graph))
	}
	return len + 1
}

// Max finds max of two numbers
func Max(x, y int) int {
	if x < y {
		return y
	}
	return x
}
func init() {
	rootCmd.AddCommand(maxDepthCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// maxDepthCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// maxDepthCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
