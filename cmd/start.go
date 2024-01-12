package cmd

import (
	"github.com/spf13/cobra"
	"shadowsocks-manager/internal/app"
)

var startCmd = &cobra.Command{
	Use: "start",
	Run: startFunc,
}

func startFunc(_ *cobra.Command, _ []string) {
	a, err := app.Init()
	if err != nil {
		panic(err)
	}
	defer a.Shutdown()
	a.Wait()
}
