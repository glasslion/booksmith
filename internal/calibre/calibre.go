// Package calibre resolves and runs calibre's command-line tools.
//
// There is exactly one way to find calibre: it is specified, never guessed. Either the
// --calibre flag, or "calibre_dir" in config.json next to this binary. See ADR-0004.
package calibre

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ConfigFileName is the config file looked for next to the binary.
const ConfigFileName = "config.json"

// Config is the on-disk configuration.
type Config struct {
	CalibreDir string `json:"calibre_dir"`
}

// Tools are the resolved paths of the calibre executables we use.
type Tools struct {
	EbookConvert string
	CalibreDebug string
}

// Dir returns the directory the calibre tools live in.
func (t Tools) Dir() string {
	return filepath.Dir(t.EbookConvert)
}

// Resolve returns calibre's tools. The flag wins; otherwise config.json in exeDir is
// read. Nothing is probed: a missing or broken specification is an error.
func Resolve(flagDir, exeDir string) (Tools, error) {
	if flagDir != "" {
		return fromDir(flagDir)
	}
	path := filepath.Join(exeDir, ConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Tools{}, &NoConfigError{Path: path}
		}
		return Tools{}, fmt.Errorf("%s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Tools{}, &ConfigError{Path: path, Err: err}
	}
	dir := strings.TrimSpace(cfg.CalibreDir)
	if dir == "" {
		return Tools{}, &ConfigError{Path: path, Err: errors.New(`"calibre_dir" is empty`)}
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(filepath.Dir(path), dir)
	}
	return fromDir(dir)
}

func fromDir(dir string) (Tools, error) {
	ebookConvert, ok := findIn(dir, "ebook-convert")
	calibreDebug, debugOK := findIn(dir, "calibre-debug")
	if !ok || !debugOK {
		var missing []string
		if !ok {
			missing = append(missing, "ebook-convert")
		}
		if !debugOK {
			missing = append(missing, "calibre-debug")
		}
		return Tools{}, &IncompleteError{Dir: dir, Missing: missing}
	}
	return Tools{EbookConvert: ebookConvert, CalibreDebug: calibreDebug}, nil
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

// exampleConfig is what the error messages tell users to write, verbatim.
const exampleConfig = `{ "calibre_dir": "C:\\Program Files\\Calibre2" }`

// NoConfigError means calibre was never specified.
type NoConfigError struct {
	Path string
}

func (e *NoConfigError) Error() string {
	return fmt.Sprintf(`no calibre configured: %s does not exist

either pass the directory on the command line:

  booksmith convert --calibre "C:\Program Files\Calibre2" book.mobi

or create %s containing:

  %s`, e.Path, e.Path, exampleConfig)
}

// ConfigError means config.json exists but does not hold a usable calibre_dir.
type ConfigError struct {
	Path string
	Err  error
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("%s: invalid config: %v\nexpected: %s", e.Path, e.Err, exampleConfig)
}

// IncompleteError means the configured directory is not a calibre installation.
type IncompleteError struct {
	Dir     string
	Missing []string
}

func (e *IncompleteError) Error() string {
	return fmt.Sprintf("%s: not a calibre installation: no %s in that directory",
		e.Dir, strings.Join(e.Missing, " and no "))
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
