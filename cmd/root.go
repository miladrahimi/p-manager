package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"xray-manager/internal/config"
)

var rootCmd = &cobra.Command{
	Use: "xray-manager",
}

func init() {
	cobra.OnInitialize(func() { fmt.Println(config.AppName) })

	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(versionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
