package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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

		// get flags
		verbose, err := cmd.Flags().GetBool("verbose")
		if err != nil {
			return err
		}
		cyclesMode, err := cmd.Flags().GetBool("cycles")
		if err != nil {
			return err
		}

		depGraph, deps, mainModule := getDepInfo()

		if verbose {
			fmt.Println("All dependencies:")
			printDeps(deps)
		}

		// Get all chains starting from main module
		// also get all cycles
		// cycleChains stores the chain containing the cycles and
		// not the actual cycle itself
		cycleChains := make(map[int][]string)
		chains := make(map[int][]string)
		iter := 0
		var temp []string
		getChains(mainModule, depGraph, temp, chains, cycleChains, &iter)

		// get values
		totalDeps := len(deps)
		maxDepth := getMaxDepth(chains)
		directDeps := len(depGraph[mainModule])
		transitiveDeps := totalDeps - directDeps

		// print the longest chain
		if verbose {
			fmt.Println("Longest chain is: ")
			// maxDepth + 1 since maxDepth stores length of longest
			// chain and chains has number of deps in chain as keys
			printChain(chains[maxDepth+1])
		}

		// print all the cycles
		if cyclesMode {
			fmt.Println("All cycles in dependencies are: ")
			cycles := getCycles(cycleChains)

			for _, c := range cycles {
				fmt.Println()
				fmt.Println(strings.Join(c, " -> "))
				fmt.Println()
			}
		}

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
	analyzeDepsCmd.Flags().BoolP("verbose", "v", false, "Get additional details")
	analyzeDepsCmd.Flags().BoolP("cycles", "c", false, "Get all the cycles")

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// analyzeDepsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// analyzeDepsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
