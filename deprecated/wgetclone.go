package cmd

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var bytesToMegaBytes = 1048576.0

// PassThru code originally from
// http://stackoverflow.com/a/22422650/613575
type PassThru struct {
	io.Reader
	curr  int64
	total float64
}

// wgetcloneCmd represents the wgetclone command
var wgetcloneCmd = &cobra.Command{
	Use:   "wgetclone",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("wgetclone called")
		url, _ := cmd.Flags().GetString("url")
		file, _ := cmd.Flags().GetString("file")
		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("Hello")
			log.Fatalln(err)
		}
		defer resp.Body.Close() // we need to remember to close the response body otherwise might have resource leaks
		// defer -> defers the execution of a function until surrounding function returns

		out, _ := os.Create(file)
		defer out.Close()

		src := &PassThru{Reader: resp.Body, total: float64(resp.ContentLength)}

		size, err := io.Copy(out, src)
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Printf("\nFile Transferred. (%.1f MB)\n", float64(size)/bytesToMegaBytes)
	},
}

func init() {
	rootCmd.AddCommand(wgetcloneCmd)
	wgetcloneCmd.Flags().StringP("url", "u", "", "URL | URL to github patch")
	wgetcloneCmd.Flags().StringP("file", "f", "", "Filename | Name of txt file")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// wgetcloneCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// wgetcloneCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
