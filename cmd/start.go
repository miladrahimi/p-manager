package cmd

import (
	"fmt"
	"github.com/miladrahimi/xray-manager/internal/app"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use: "start",
	Run: startFunc,
}

func startFunc(_ *cobra.Command, _ []string) {
	a, err := app.New()
	defer a.Shutdown()
	if err != nil {
		panic(fmt.Sprintf("%+v\n", err))
	}
	a.Boot()
	a.Wait()
}
