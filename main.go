package main

import (
	"fmt"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to execute the command: %+v\n", errors.WithStack(err))
	}
}
