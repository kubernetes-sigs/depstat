package cmd

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// analyzeCmd represents the analyze command
var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("analyze called")
		totalDepCmd := exec.Command("go", "list", "-m", "all")
		// totalDepCmd.Stdout = os.Stdout
		// totalDepCmd.Stderr = os.Stdout
		// if err := totalDepCmd.Run(); err != nil {
		// 	log.Fatalln("Error: ", err)
		// }
		output, err := totalDepCmd.Output()
		if err != nil {
			log.Fatal(err)
		}
		outputString := string(output)
		totalDeps := strings.Count(outputString, "\n") - 1

		fmt.Println("Total Dependencies: ", totalDeps)

	},
}

func init() {
	rootCmd.AddCommand(analyzeCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// analyzeCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// analyzeCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
