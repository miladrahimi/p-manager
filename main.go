package main

import (
	"fmt"
	"os"
	"shadowsocks-manager/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
	}
}
