// Command booksmith is a thin batch front end for calibre's command-line tools.
package main

import (
	"os"

	"booksmith/internal/cli"
	"booksmith/internal/console"
)

func main() {
	console.SetupUTF8()
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
