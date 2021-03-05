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
		fmt.Println(graph["test-proj"][0])

		return nil
	},
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
