/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"jpm/db"
	"jpm/lib"
	"jpm/parser"

	"github.com/spf13/cobra"
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: install,
}

func init() {
	rootCmd.AddCommand(installCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// installCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// installCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func install(cmd *cobra.Command, args []string) {
	if len(args) > 1 || len(args) < 1 {
		fmt.Println("Invalid amount of arguments")
		return
	}
	name := args[0]
	rdb := db.NewRemoteDB()
	defer rdb.Close()
	data, err := rdb.GetOne(name)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = lib.Download(data.Url, "bin")
	if err != nil {
		fmt.Println(err)
		return
	}
	incs, err := parser.Parse(data.Instructions)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, inc := range incs {
		inc.Run()
	}
}
