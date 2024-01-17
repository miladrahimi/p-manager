package cmd

import (
	"github.com/miladrahimi/xray-manager/internal/app"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use: "start",
	Run: startFunc,
}

func startFunc(_ *cobra.Command, _ []string) {
	a, err := app.New()
	if err != nil {
		panic(err)
	}
	defer a.Shutdown()
	a.Boot()
	a.Wait()
}
