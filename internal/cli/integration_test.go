//go:build !windows

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubCalibre writes a fake calibre directory whose tools log their argv. Real processes
// run, so this covers resolution, argument passing and stderr capture end to end.
func stubCalibre(t *testing.T) (calibreDir, logPath string) {
	t.Helper()
	root := t.TempDir()
	logPath = filepath.Join(root, "calls.log")
	cal := filepath.Join(root, "Calibre2")

	ebookConvert := `#!/bin/sh
echo "ebook-convert $*" >> "$BOOKSMITH_STUB_LOG"
if [ -n "$BOOKSMITH_STUB_FAIL" ]; then
    echo "boom failure" >&2
    exit 1
fi
: > "$2"
`
	calibreDebug := `#!/bin/sh
echo "calibre-debug $*" >> "$BOOKSMITH_STUB_LOG"
`
	for name, body := range map[string]string{"ebook-convert": ebookConvert, "calibre-debug": calibreDebug} {
		if err := os.MkdirAll(cal, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cal, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("BOOKSMITH_STUB_LOG", logPath)
	return cal, logPath
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no calls logged: %v", err)
	}
	return string(data)
}

func TestIntegrationConvert(t *testing.T) {
	calDir, logPath := stubCalibre(t)
	books := t.TempDir()
	in := filepath.Join(books, "a.mobi")
	if err := os.WriteFile(in, []byte("book"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if code := Run([]string{"convert", "--calibre", calDir, in}, &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(books, "a.epub")); err != nil {
		t.Fatalf("output missing: %v", err)
	}
	log := readLog(t, logPath)
	if !strings.Contains(log, "ebook-convert "+in) {
		t.Errorf("calibre was not called with the input: %q", log)
	}
	if strings.Contains(log, "calibre-debug") {
		t.Errorf("convert should not strip covers: %q", log)
	}
}

func TestIntegrationRecoverRunsBothToolsInOrder(t *testing.T) {
	calDir, logPath := stubCalibre(t)
	books := t.TempDir()
	in := filepath.Join(books, "a.epub")
	if err := os.WriteFile(in, []byte("book"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if code := Run([]string{"recover", "--calibre", calDir, in}, &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(books, "a.recover.epub")); err != nil {
		t.Fatalf("output missing: %v", err)
	}
	log := strings.TrimSpace(readLog(t, logPath))
	lines := strings.Split(log, "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 calibre calls, got %d: %q", len(lines), log)
	}
	if !strings.HasPrefix(lines[0], "calibre-debug -e ") || !strings.Contains(lines[0], "booksmith-strip-cover.py") {
		t.Errorf("first call should strip the cover: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "ebook-convert ") {
		t.Errorf("second call should convert: %q", lines[1])
	}
}

func TestIntegrationCalibreFailureIsReported(t *testing.T) {
	calDir, _ := stubCalibre(t)
	t.Setenv("BOOKSMITH_STUB_FAIL", "1")
	books := t.TempDir()
	in := filepath.Join(books, "a.mobi")
	if err := os.WriteFile(in, []byte("book"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run([]string{"convert", "--calibre", calDir, in}, &out, &errOut)

	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "calibre: boom failure") {
		t.Errorf("stdout should carry calibre's own error: %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(books, "a.epub")); err == nil {
		t.Error("a failed conversion must not leave an output file")
	}
	if _, err := os.Stat(in); err != nil {
		t.Error("the input must survive a failed conversion")
	}
}
