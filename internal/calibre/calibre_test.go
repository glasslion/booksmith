package calibre

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeTool(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestFindSiblingCalibreSubdirectory(t *testing.T) {
	dir := t.TempDir()
	writeTool(t, filepath.Join(dir, "calibre", "ebook-convert"), "")
	writeTool(t, filepath.Join(dir, "calibre", "calibre-debug"), "")

	tools, err := Find(dir)
	if err != nil {
		t.Fatal(err)
	}
	if tools.FromPath {
		t.Error("FromPath should be false for sibling tools")
	}
	if tools.Dir() != filepath.Join(dir, "calibre") {
		t.Errorf("Dir() = %q", tools.Dir())
	}
}

func TestFindPlainSibling(t *testing.T) {
	dir := t.TempDir()
	writeTool(t, filepath.Join(dir, "ebook-convert"), "")
	writeTool(t, filepath.Join(dir, "calibre-debug"), "")

	tools, err := Find(dir)
	if err != nil {
		t.Fatal(err)
	}
	if tools.FromPath || tools.Dir() != dir {
		t.Errorf("got %+v", tools)
	}
}

func TestFindSubdirectoryWins(t *testing.T) {
	dir := t.TempDir()
	writeTool(t, filepath.Join(dir, "ebook-convert"), "")
	writeTool(t, filepath.Join(dir, "calibre-debug"), "")
	writeTool(t, filepath.Join(dir, "calibre", "ebook-convert"), "")
	writeTool(t, filepath.Join(dir, "calibre", "calibre-debug"), "")

	tools, err := Find(dir)
	if err != nil {
		t.Fatal(err)
	}
	if tools.Dir() != filepath.Join(dir, "calibre") {
		t.Errorf("Dir() = %q, want the calibre/ subdirectory", tools.Dir())
	}
}

func TestFindFallsBackToPath(t *testing.T) {
	bin := t.TempDir()
	for _, name := range []string{"ebook-convert", "ebook-convert.exe", "calibre-debug", "calibre-debug.exe"} {
		writeTool(t, filepath.Join(bin, name), "")
	}
	t.Setenv("PATH", bin)

	tools, err := Find(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !tools.FromPath {
		t.Fatal("FromPath should be true when resolved through PATH")
	}
}

func TestFindReportsMissingCalibre(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := Find(t.TempDir())
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !contains(got, "calibre not found") {
		t.Errorf("unhelpful error: %s", got)
	}
}

func TestFindRequiresCalibreDebug(t *testing.T) {
	dir := t.TempDir()
	writeTool(t, filepath.Join(dir, "ebook-convert"), "")
	if _, err := Find(dir); err == nil {
		t.Fatal("expected an error when calibre-debug is missing")
	}
}

func TestExecutorSurfacesLastStderrLine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script")
	}
	dir := t.TempDir()
	tool := filepath.Join(dir, "failing")
	writeTool(t, tool, "#!/bin/sh\necho 'first line' >&2\necho 'Cannot detect cover image' >&2\nexit 3\n")

	err := Executor{}.Run(context.Background(), tool, "a", "b")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !IsCmdError(err) {
		t.Fatalf("expected a CmdError, got %T", err)
	}
	if got, want := err.Error(), "calibre: Cannot detect cover image"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExecutorSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script")
	}
	dir := t.TempDir()
	tool := filepath.Join(dir, "ok")
	writeTool(t, tool, "#!/bin/sh\nexit 0\n")
	if err := (Executor{}).Run(context.Background(), tool); err != nil {
		t.Fatal(err)
	}
}

func TestLastLine(t *testing.T) {
	cases := map[string]string{
		"":             "",
		"one\n":        "one",
		"one\ntwo\n\n": "two",
		"  spaced  \n": "spaced",
	}
	for in, want := range cases {
		if got := lastLine(in); got != want {
			t.Errorf("lastLine(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsCmdErrorRejectsOtherErrors(t *testing.T) {
	if IsCmdError(errors.New("plain")) {
		t.Fatal("plain errors are not CmdErrors")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
