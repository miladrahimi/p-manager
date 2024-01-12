package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"shadowsocks-manager/internal/config"
)

var rootCmd = &cobra.Command{
	Use: "shadowsocks-manager",
}

func init() {
	cobra.OnInitialize(func() { fmt.Println(config.AppName) })

	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(versionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
