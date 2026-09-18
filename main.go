package main

import (
	"os"

	"github.com/sagarc03/stowry/internal/cli"
)

// version is set at build time with -X main.version.
var version = "dev"

func main() {
	if err := cli.Execute(version); err != nil {
		os.Exit(1)
	}
}
