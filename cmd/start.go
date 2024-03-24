package cmd

import (
	"fmt"
	"github.com/miladrahimi/p-manager/internal/app"
	"github.com/miladrahimi/p-manager/pkg/utils"
	"github.com/spf13/cobra"
	"os"
)

var startCmd = &cobra.Command{
	Use: "start",
	Run: startFunc,
}

func startFunc(_ *cobra.Command, _ []string) {
	if utils.FileExist("./storage/database.json") {
		_ = os.Rename("./storage/database.json", "./storage/database/app.json")
	}

	a, err := app.New()
	defer a.Shutdown()
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
