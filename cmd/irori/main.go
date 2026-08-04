package main

import (
	"fmt"
	"os"

	"github.com/bx-team/irori/internal/cli"
)

var version = "dev"

func main() {
	cli.SetVersion(version)
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "irori:", err)
		os.Exit(1)
	}
}
