package cmd

import (
	"fmt"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "p-manager",
}

func init() {
	cobra.OnInitialize(func() { fmt.Println(config.AppName) })
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(versionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
