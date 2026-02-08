package cmd

import (
	"fmt"
	r "runtime"

	c "github.com/miladrahimi/p-manager/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "p-manager",
}

func init() {
	cobra.OnInitialize(func() {
		fmt.Println(c.AppName, c.AppVersion, "/", r.Compiler, r.Version(), "/", r.GOOS, r.GOARCH)
	})
}

// Execute creates CLI and run the requested command.
func Execute() error {
	return rootCmd.Execute()
}
