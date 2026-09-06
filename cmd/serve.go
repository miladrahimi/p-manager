package cmd

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/app"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use: "serve",
		Run: serve,
	})
}

// serve runs the application and xray.
func serve(_ *cobra.Command, _ []string) {
	a, err := app.New()
	if err != nil {
		panic(fmt.Sprintf("%+v\n", errors.WithStack(err)))
	}
	defer func() {
		a.Close()
	}()

	if err = a.Start(); err != nil {
		panic(fmt.Sprintf("%+v\n", errors.WithStack(err)))
	}

	a.Wait()
}
