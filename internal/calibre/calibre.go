// Package calibre locates and runs calibre's command-line tools.
//
// A calibre installation is a whole directory (Qt, Python, app/), so we reference it
// where it already lives instead of vendoring anything. See ADR-0001.
package calibre

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Tools are the resolved paths of the calibre executables we use.
type Tools struct {
	EbookConvert string
	CalibreDebug string
	// FromPath is true when the tools were found on PATH rather than next to our binary.
	FromPath bool
}

// Dir returns the directory the calibre tools live in.
func (t Tools) Dir() string {
	return filepath.Dir(t.EbookConvert)
}

// Find resolves calibre's tools. Search order: dir/calibre, dir, then PATH.
func Find(dir string) (Tools, error) {
	if path, ok := findIn(filepath.Join(dir, "calibre"), "ebook-convert"); ok {
		return withDebug(path, false)
	}
	if path, ok := findIn(dir, "ebook-convert"); ok {
		return withDebug(path, false)
	}
	if path, err := exec.LookPath("ebook-convert"); err == nil {
		return withDebug(path, true)
	}
	return Tools{}, fmt.Errorf("calibre not found: no ebook-convert in %s, %s, or PATH "+
		"(install calibre and put it in ./calibre/ next to this binary)", filepath.Join(dir, "calibre"), dir)
}

func withDebug(ebookConvert string, fromPath bool) (Tools, error) {
	debug, ok := findIn(filepath.Dir(ebookConvert), "calibre-debug")
	if !ok {
		return Tools{}, fmt.Errorf("found %s but not calibre-debug in the same directory "+
			"(both ship with calibre)", ebookConvert)
	}
	return Tools{EbookConvert: ebookConvert, CalibreDebug: debug, FromPath: fromPath}, nil
}

// findIn looks for name (or name.exe) in dir and returns a path that exists.
func findIn(dir, name string) (string, bool) {
	for _, candidate := range []string{name, name + ".exe"} {
		path := filepath.Join(dir, candidate)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

// ExecutableDir returns the directory of the running binary.
func ExecutableDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// Runner runs one external command. Tests substitute a fake.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) error
}

// CmdError is a failed calibre invocation, carrying enough detail to explain itself.
type CmdError struct {
	Tool   string
	Detail string
	Err    error
}

func (e *CmdError) Error() string {
	tool := strings.TrimSuffix(filepath.Base(e.Tool), ".exe")
	if e.Detail != "" {
		return fmt.Sprintf("calibre: %s", e.Detail)
	}
	return fmt.Sprintf("calibre: %s: %v", tool, e.Err)
}

func (e *CmdError) Unwrap() error { return e.Err }

// Executor runs calibre for real.
type Executor struct {
	// Verbose streams calibre's own output to Out.
	Verbose bool
	Out     io.Writer
}

// Run implements Runner.
func (e Executor) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if e.Verbose {
		cmd.Stdout = e.Out
		cmd.Stderr = e.Out
		if err := cmd.Run(); err != nil {
			return &CmdError{Tool: name, Err: err}
		}
		return nil
	}
	var stderr strings.Builder
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return &CmdError{Tool: name, Detail: lastLine(stderr.String()), Err: err}
	}
	return nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\r\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return ""
}

// IsCmdError reports whether err came from a failed calibre invocation.
func IsCmdError(err error) bool {
	var ce *CmdError
	return errors.As(err, &ce)
}
