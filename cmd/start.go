package cmd

import (
	"fmt"
	"github.com/miladrahimi/p-manager/internal/app"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use: "start",
		Run: func(_ *cobra.Command, _ []string) {
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
		},
	})
}
