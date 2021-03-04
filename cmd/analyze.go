package cmd

import (
	"encoding/json"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// totalDepCmd represents the totalDep command
var totalDepCmd = &cobra.Command{
	Use:   "totalDep",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		//fmt.Println(args)
		// TODO: allow taking an arg and running analysis in that dir
		totalDepCmd := exec.Command("go", "list", "-m", "all")

		output, err := totalDepCmd.Output()
		if err != nil {
			return err
		}
		outputString := string(output)
		totalDeps := strings.Count(outputString, "\n") - 1

		outputObj := struct {
			SA int `json:"totalDependencies"`
		}{
			SA: totalDeps,
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
	rootCmd.AddCommand(totalDepCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// totalDepCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// totalDepCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
