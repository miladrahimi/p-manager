package main

import (
	"fmt"
	"os"

	"github.com/miladrahimi/p-manager/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
	}
}
