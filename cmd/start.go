package cmd

import (
	"fmt"
	"github.com/miladrahimi/p-manager/internal/app"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use: "start",
	Run: startFunc,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func startFunc(_ *cobra.Command, _ []string) {
	a, err := app.New()
	defer a.Close()
	if err != nil {
		panic(fmt.Sprintf("%+v\n", err))
	}
	if err = a.Init(); err != nil {
		panic(fmt.Sprintf("%+v\n", err))
	}
	a.Coordinator.Run()
	a.HttpServer.Run()
	a.Wait()
}
