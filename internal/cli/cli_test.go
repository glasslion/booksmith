package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"booksmith/internal/calibre"
)

type recorder struct {
	calls []string
	err   error
}

func (r *recorder) Run(ctx context.Context, tool string, args ...string) error {
	r.calls = append(r.calls, tool+" "+strings.Join(args, " "))
	if r.err != nil {
		return r.err
	}
	if strings.Contains(filepath.Base(tool), "ebook-convert") && len(args) >= 2 {
		return os.WriteFile(args[1], []byte("x"), 0o644)
	}
	return nil
}

var fakeTools = calibre.Tools{EbookConvert: "/calibre/ebook-convert", CalibreDebug: "/calibre/calibre-debug"}

// stub installs a fake calibre lookup and a recording runner for the duration of the test.
func stub(t *testing.T, tools calibre.Tools, findErr error) *recorder {
	t.Helper()
	rec := &recorder{}
	oldFind, oldNew := findTools, newRunner
	t.Cleanup(func() { findTools, newRunner = oldFind, oldNew })
	findTools = func(string) (calibre.Tools, error) { return tools, findErr }
	newRunner = func(bool, io.Writer) calibre.Runner { return rec }
	return rec
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunWithoutArgumentsShowsUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(nil, &out, &errOut); code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "Usage:") {
		t.Errorf("usage not printed: %q", errOut.String())
	}
}

func TestRunHelpExitsZero(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"--help"}, &out, &errOut); code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "convert-recover") {
		t.Errorf("help should list the subcommands: %q", out.String())
	}
}

func TestRunVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"version"}, &out, &errOut); code != 0 {
		t.Errorf("exit = %d", code)
	}
	if !strings.Contains(out.String(), Version) {
		t.Errorf("output = %q", out.String())
	}
}

func TestRunUnknownSubcommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"frobnicate"}, &out, &errOut); code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "unknown subcommand") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestRunWithoutInputs(t *testing.T) {
	stub(t, fakeTools, nil)
	var out, errOut bytes.Buffer
	if code := Run([]string{"convert"}, &out, &errOut); code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "no inputs") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestRunConvertAcceptsFlagsAfterPositionals(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.mobi")
	writeFile(t, in, "book")
	rec := stub(t, fakeTools, nil)

	var out, errOut bytes.Buffer
	code := Run([]string{"convert", in, "--to", "azw3"}, &out, &errOut)

	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut.String())
	}
	if len(rec.calls) != 1 {
		t.Fatalf("calls = %v", rec.calls)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.azw3")); err != nil {
		t.Errorf("output missing: %v", err)
	}
}

func TestRunRecoverSkipsNonEpub(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.mobi")
	writeFile(t, in, "book")
	rec := stub(t, fakeTools, nil)

	var out, errOut bytes.Buffer
	code := Run([]string{"recover", in}, &out, &errOut)

	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if len(rec.calls) != 0 {
		t.Errorf("calibre should not run for a skipped input: %v", rec.calls)
	}
	if !strings.Contains(out.String(), "not an epub") {
		t.Errorf("stdout = %q", out.String())
	}
}

func TestRunPrintsResolvedCalibreWhenFromPath(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.mobi")
	writeFile(t, in, "book")
	tools := fakeTools
	tools.FromPath = true
	stub(t, tools, nil)

	var out, errOut bytes.Buffer
	Run([]string{"convert", in}, &out, &errOut)

	if !strings.Contains(out.String(), "calibre: using /calibre/ebook-convert (from PATH)") {
		t.Errorf("stdout = %q", out.String())
	}
}

func TestRunMissingCalibreIsUsageError(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.mobi")
	writeFile(t, in, "book")
	stub(t, calibre.Tools{}, errors.New("calibre not found: install calibre"))

	var out, errOut bytes.Buffer
	if code := Run([]string{"convert", in}, &out, &errOut); code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "calibre not found") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestRunFailureGivesExitCodeOne(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.mobi")
	writeFile(t, in, "book")
	rec := stub(t, fakeTools, nil)
	rec.err = &calibre.CmdError{Detail: "boom"}

	var out, errOut bytes.Buffer
	if code := Run([]string{"convert", in}, &out, &errOut); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "1 failed") {
		t.Errorf("stdout = %q", out.String())
	}
}

func TestRunOutDirIsCreated(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.mobi")
	writeFile(t, in, "book")
	stub(t, fakeTools, nil)
	outDir := filepath.Join(dir, "converted")

	var out, errOut bytes.Buffer
	if code := Run([]string{"convert", "-o", outDir, in}, &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(outDir, "a.epub")); err != nil {
		t.Errorf("output missing: %v", err)
	}
}

func TestRunDryRunDoesNotExecute(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.mobi")
	writeFile(t, in, "book")
	rec := stub(t, fakeTools, nil)

	var out, errOut bytes.Buffer
	if code := Run([]string{"convert-recover", "--dry-run", in}, &out, &errOut); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if len(rec.calls) != 0 {
		t.Errorf("dry run executed: %v", rec.calls)
	}
	if !strings.Contains(out.String(), "1 dry-run") {
		t.Errorf("stdout = %q", out.String())
	}
}

func TestSplitFlags(t *testing.T) {
	flags, pos := splitFlags([]string{"a.mobi", "-r", "-o", "out", "--to=azw3", "b.epub", "--"})
	if want := []string{"-r", "-o", "out", "--to=azw3"}; strings.Join(flags, " ") != strings.Join(want, " ") {
		t.Errorf("flags = %v, want %v", flags, want)
	}
	if want := []string{"a.mobi", "b.epub"}; strings.Join(pos, " ") != strings.Join(want, " ") {
		t.Errorf("positionals = %v, want %v", pos, want)
	}
}
