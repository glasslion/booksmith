package calibre

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

// calibreDir makes a directory that looks like a calibre installation.
func calibreDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ebook-convert"), "")
	writeFile(t, filepath.Join(dir, "calibre-debug"), "")
	return dir
}

func TestResolveFromFlag(t *testing.T) {
	dir := calibreDir(t)
	tools, err := Resolve(dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if tools.Dir() != dir {
		t.Errorf("Dir() = %q, want %q", tools.Dir(), dir)
	}
}

func TestResolveFromConfig(t *testing.T) {
	dir := calibreDir(t)
	exeDir := t.TempDir()
	writeFile(t, filepath.Join(exeDir, ConfigFileName), `{"calibre_dir": `+jsonString(dir)+`}`)

	tools, err := Resolve("", exeDir)
	if err != nil {
		t.Fatal(err)
	}
	if tools.Dir() != dir {
		t.Errorf("Dir() = %q, want %q", tools.Dir(), dir)
	}
}

func TestResolveRelativeConfigPathIsRelativeToConfigFile(t *testing.T) {
	exeDir := t.TempDir()
	writeFile(t, filepath.Join(exeDir, "Calibre2", "ebook-convert"), "")
	writeFile(t, filepath.Join(exeDir, "Calibre2", "calibre-debug"), "")
	writeFile(t, filepath.Join(exeDir, ConfigFileName), `{"calibre_dir": "Calibre2"}`)

	tools, err := Resolve("", exeDir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(exeDir, "Calibre2"); tools.Dir() != want {
		t.Errorf("Dir() = %q, want %q", tools.Dir(), want)
	}
}

func TestFlagBeatsConfig(t *testing.T) {
	fromFlag := calibreDir(t)
	exeDir := t.TempDir()
	writeFile(t, filepath.Join(exeDir, ConfigFileName), `{"calibre_dir": `+jsonString(calibreDir(t))+`}`)

	tools, err := Resolve(fromFlag, exeDir)
	if err != nil {
		t.Fatal(err)
	}
	if tools.Dir() != fromFlag {
		t.Errorf("Dir() = %q, want the flag's directory", tools.Dir())
	}
}

func TestResolveWithoutConfigExplainsHowToFixIt(t *testing.T) {
	exeDir := t.TempDir()
	_, err := Resolve("", exeDir)
	if err == nil {
		t.Fatal("expected an error")
	}
	var noConfig *NoConfigError
	if !errors.As(err, &noConfig) {
		t.Fatalf("expected a NoConfigError, got %T", err)
	}
	msg := err.Error()
	for _, want := range []string{
		filepath.Join(exeDir, ConfigFileName),
		"--calibre",
		`{ "calibre_dir": "C:\\Program Files\\Calibre2" }`,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message is missing %q:\n%s", want, msg)
		}
	}
}

func TestResolveInvalidJSON(t *testing.T) {
	exeDir := t.TempDir()
	writeFile(t, filepath.Join(exeDir, ConfigFileName), `{"calibre_dir": "x",}`)

	_, err := Resolve("", exeDir)
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected a ConfigError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "invalid config") {
		t.Errorf("message = %q", err.Error())
	}
	if !strings.Contains(err.Error(), filepath.Join(exeDir, ConfigFileName)) {
		t.Errorf("message should name the file: %q", err.Error())
	}
}

func TestResolveEmptyCalibreDir(t *testing.T) {
	exeDir := t.TempDir()
	writeFile(t, filepath.Join(exeDir, ConfigFileName), `{"calibre_dir": "   "}`)

	_, err := Resolve("", exeDir)
	if err == nil || !strings.Contains(err.Error(), `"calibre_dir" is empty`) {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveMissingDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	_, err := Resolve(missing, t.TempDir())
	var incomplete *IncompleteError
	if !errors.As(err, &incomplete) {
		t.Fatalf("expected an IncompleteError, got %T", err)
	}
	if !strings.Contains(err.Error(), "ebook-convert and no calibre-debug") {
		t.Errorf("message = %q", err.Error())
	}
}

func TestResolveNamesTheMissingTool(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ebook-convert"), "")

	_, err := Resolve(dir, t.TempDir())
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	if strings.Contains(msg, "no ebook-convert") || !strings.Contains(msg, "calibre-debug") {
		t.Errorf("only the missing tool should be named: %q", msg)
	}
}

func TestResolveWinsWithExeSuffix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ebook-convert.exe"), "")
	writeFile(t, filepath.Join(dir, "calibre-debug.exe"), "")

	tools, err := Resolve(dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(tools.EbookConvert) != "ebook-convert.exe" {
		t.Errorf("EbookConvert = %q", tools.EbookConvert)
	}
}

func TestExecutorSurfacesLastStderrLine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script")
	}
	dir := t.TempDir()
	tool := filepath.Join(dir, "failing")
	writeFile(t, tool, "#!/bin/sh\necho 'first line' >&2\necho 'Cannot detect cover image' >&2\nexit 3\n")

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
	writeFile(t, tool, "#!/bin/sh\nexit 0\n")
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

// jsonString quotes s for embedding in a JSON document.
func jsonString(s string) string {
	return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"`
}
