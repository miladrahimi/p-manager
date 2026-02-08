package cmd

import (
	"github.com/miladrahimi/p-manager/internal/app"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use: "serve",
		Run: serve,
	})

	// deprecated: use serve instead
	rootCmd.AddCommand(&cobra.Command{
		Use: "start",
		Run: serve,
	})
}

// serve runs the application and xray.
func serve(_ *cobra.Command, _ []string) {
	a, err := app.New()
	if err != nil {
		cobra.CheckErr(err)
		return
	}
	defer func() {
		a.Close()
	}()

	if err = a.Start(); err != nil {
		cobra.CheckErr(err)
		return
	}

	a.Wait()
}
