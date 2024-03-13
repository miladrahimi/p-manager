package cmd

import (
	"fmt"
	"github.com/miladrahimi/xray-manager/internal/app"
	"github.com/miladrahimi/xray-manager/pkg/utils"
	"github.com/spf13/cobra"
	"os"
)

var startCmd = &cobra.Command{
	Use: "start",
	Run: startFunc,
}

func startFunc(_ *cobra.Command, _ []string) {
	// TODO: Remove this
	if utils.FileExist("./storage/database.json") {
		_ = os.Rename("./storage/database.json", "./storage/database/app.json")
	}

	a, err := app.New()
	defer a.Shutdown()
	if err != nil {
		panic(fmt.Sprintf("%+v\n", err))
	}
	a.Init()
	a.Coordinator.Run()
	a.HttpServer.Run()
	a.Wait()
}
