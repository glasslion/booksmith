// Command booksmith is a thin batch front end for calibre's command-line tools.
package main

import (
	"os"

	"booksmith/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
