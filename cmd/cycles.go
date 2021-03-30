package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// analyzeDepsCmd represents the analyzeDeps command
var cyclesCmd = &cobra.Command{
	Use:   "cycles",
	Short: "Prints cycles in dependency chains.",
	Long:  `Will show all the cycles in the dependencies of the project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		depGraph, _, mainModule := getDepInfo()
		var cycleChains [][]string
		chains := make(map[int][]string)
		var temp []string
		getChains(mainModule, depGraph, temp, chains, &cycleChains)
		fmt.Println("All cycles in dependencies are: ")
		cycles := getCycles(cycleChains)

		for _, c := range cycles {
			printChain(c)
		}
		return nil
	},
}

// gets the cycles from the cycleChains
func getCycles(cycleChains [][]string) [][]string {
	var cycles [][]string
	for _, cycle := range cycleChains {
		var actualCycle []string
		start := false
		startDep := cycle[len(cycle)-1]
		for _, val := range cycle {
			if val == startDep {
				start = true
			}
			if start {
				actualCycle = append(actualCycle, val)
			}
		}
		if !sliceContains(cycles, actualCycle) {
			cycles = append(cycles, actualCycle)
		}
	}
	return cycles
}

func init() {
	rootCmd.AddCommand(cyclesCmd)

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// cyclesCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// cyclesCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
