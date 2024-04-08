package cmd

import (
	"fmt"
	c "github.com/miladrahimi/p-manager/internal/config"
	"github.com/spf13/cobra"
	r "runtime"
)

var rootCmd = &cobra.Command{
	Use: "p-manager",
}

func init() {
	cobra.OnInitialize(func() {
		fmt.Println(c.AppName, c.AppVersion, "(", r.Version(), r.Compiler, r.GOOS, "/", r.GOARCH, ")")
	})
}

func Execute() error {
	return rootCmd.Execute()
}
