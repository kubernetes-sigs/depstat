package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// analyzeDepsCmd represents the analyzeDeps command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all project dependencies.",
	Long: `Gives a list of all the dependencies of the project. These include
			both direct as well as transitive dependencies.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, deps, _ := getDepInfo()
		fmt.Println("List of all dependencies:")
		printDeps(deps)
		return nil
	},
}

func printDeps(deps []string) {
	fmt.Println()
	for _, dep := range deps {
		fmt.Println(dep)
	}
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(listCmd)

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// listCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// listCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
