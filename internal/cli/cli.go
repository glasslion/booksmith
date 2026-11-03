// Package cli parses arguments and drives one batch.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"booksmith/internal/batch"
	"booksmith/internal/calibre"
	"booksmith/internal/job"
)

// Version is the program version.
const Version = "0.1.0"

// Swappable in tests.
var (
	findTools = func(dir string) (calibre.Tools, error) { return calibre.Find(dir) }
	newRunner = func(verbose bool, out io.Writer) calibre.Runner {
		return calibre.Executor{Verbose: verbose, Out: out}
	}
)

var modes = map[string]job.Mode{
	"convert":         job.Convert,
	"recover":         job.Recover,
	"convert-recover": job.ConvertRecover,
}

// valueFlags are the flags that consume the following argument.
var valueFlags = map[string]bool{
	"-o": true, "--out": true, "--to": true, "--calibre": true,
}

// Run executes one invocation and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "-h", "--help", "help":
		usage(stdout)
		return 0
	case "-V", "--version", "version":
		fmt.Fprintf(stdout, "booksmith %s\n", Version)
		return 0
	}

	mode, ok := modes[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "booksmith: unknown subcommand %q\n\n", args[0])
		usage(stderr)
		return 2
	}

	var (
		recursive, all, verbose, dryRun bool
		noSVGCover, keepTemp            bool
		to, outDir, calibreDir          string
	)
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&recursive, "r", false, "")
	fs.BoolVar(&recursive, "recursive", false, "")
	fs.BoolVar(&all, "all", false, "")
	fs.StringVar(&outDir, "o", "", "")
	fs.StringVar(&outDir, "out", "", "")
	fs.BoolVar(&verbose, "v", false, "")
	fs.BoolVar(&verbose, "verbose", false, "")
	fs.BoolVar(&dryRun, "dry-run", false, "")
	fs.StringVar(&calibreDir, "calibre", "", "")
	if mode == job.Convert {
		fs.StringVar(&to, "to", "epub", "")
	} else {
		fs.BoolVar(&noSVGCover, "no-svg-cover", false, "")
		fs.BoolVar(&keepTemp, "keep-temp", false, "")
	}

	flagArgs, positionals := splitFlags(args[1:])
	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if len(positionals) == 0 {
		fmt.Fprintf(stderr, "booksmith %s: no inputs given\n\n", args[0])
		usage(stderr)
		return 2
	}

	inputs, err := batch.Expand(positionals, batch.Options{Recursive: recursive, All: all})
	if err != nil {
		fmt.Fprintf(stderr, "booksmith: %v\n", err)
		return 2
	}

	searchDir := calibreDir
	if searchDir == "" {
		if searchDir, err = calibre.ExecutableDir(); err != nil {
			fmt.Fprintf(stderr, "booksmith: cannot determine own location: %v\n", err)
			return 2
		}
	}
	tools, err := findTools(searchDir)
	if err != nil {
		fmt.Fprintf(stderr, "booksmith: %v\n", err)
		return 2
	}
	if tools.FromPath {
		fmt.Fprintf(stdout, "calibre: using %s (from PATH)\n", tools.EbookConvert)
	}

	if outDir != "" {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			fmt.Fprintf(stderr, "booksmith: %v\n", err)
			return 2
		}
	}

	cfg := job.Config{
		Mode:       mode,
		To:         to,
		OutDir:     outDir,
		NoSVGCover: noSVGCover,
		KeepTemp:   keepTemp,
		DryRun:     dryRun,
	}
	summary := job.Execute(context.Background(), newRunner(verbose, stderr), tools, cfg,
		job.Build(cfg, inputs), stdout)
	fmt.Fprintln(stdout, summary)
	return summary.ExitCode()
}

// splitFlags pulls flags out of args so that flags may appear after positionals, which
// `flag` alone does not allow.
func splitFlags(args []string) (flags, positionals []string) {
	rest := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if rest || !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}
		if arg == "--" {
			rest = true
			continue
		}
		flags = append(flags, arg)
		if strings.ContainsRune(arg, '=') {
			continue
		}
		if valueFlags[arg] && i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return flags, positionals
}

func usage(w io.Writer) {
	fmt.Fprint(w, `booksmith - batch ebook operations on top of calibre

Usage:
  booksmith convert  [flags] <input>...
  booksmith recover  [flags] <input>...
  booksmith convert-recover [flags] <input>...

  convert          convert to another format (default epub), output <base>.<to>
  recover          remove the cover and let calibre generate a new one,
                   output <base>.recover.epub
  convert-recover  same, for non-epub inputs in one step, output <base>.epub
                   (epub inputs are skipped; use recover for those)

<input> may be a file, a directory, or a glob pattern. Globs are expanded by
booksmith, so they work in cmd.exe and PowerShell too. Nothing here is ever
destructive: existing outputs are skipped, inputs are never modified.

Flags:
  -r, --recursive      descend into directories
  -o, --out DIR        write outputs into DIR instead of next to the input
      --to FMT         convert: target format (default epub)
      --all            don't restrict directory scans to ebook extensions
      --no-svg-cover   recover: plain <img> cover page instead of calibre's SVG wrapper
      --keep-temp      recover: keep intermediate files for debugging
      --calibre DIR    look for calibre in DIR (default: next to this binary, then PATH)
      --dry-run        print the calibre commands instead of running them
  -v, --verbose        stream calibre's own output

Exit codes: 0 all good, 1 at least one input failed, 2 bad usage.

Examples:
  booksmith convert -r ~/Books
  booksmith recover ~/Books/*.epub
  booksmith convert-recover --dry-run *.mobi
`)
}
