package batch

import (
	"os"
	"path/filepath"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func abs(t *testing.T, path string) string {
	t.Helper()
	p, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func equal(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestExpandDirectoryNonRecursiveFiltersExtensions(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "a.mobi"))
	touch(t, filepath.Join(dir, "b.epub"))
	touch(t, filepath.Join(dir, "notes.txt"))
	touch(t, filepath.Join(dir, "sub", "c.mobi"))

	got, err := Expand([]string{dir}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	equal(t, got, []string{abs(t, filepath.Join(dir, "a.mobi")), abs(t, filepath.Join(dir, "b.epub"))})
}

func TestExpandDirectoryRecursive(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "a.mobi"))
	touch(t, filepath.Join(dir, "sub", "c.mobi"))

	got, err := Expand([]string{dir}, Options{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	equal(t, got, []string{abs(t, filepath.Join(dir, "a.mobi")), abs(t, filepath.Join(dir, "sub", "c.mobi"))})
}

func TestExpandAllTakesEverything(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "notes.txt"))

	got, err := Expand([]string{dir}, Options{All: true})
	if err != nil {
		t.Fatal(err)
	}
	equal(t, got, []string{abs(t, filepath.Join(dir, "notes.txt"))})
}

func TestExpandExplicitFileWithAnyExtension(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "book.kepub")
	touch(t, file)

	got, err := Expand([]string{file}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	equal(t, got, []string{abs(t, file)})
}

func TestExpandGlob(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "a.mobi"))
	touch(t, filepath.Join(dir, "b.mobi"))
	touch(t, filepath.Join(dir, "c.epub"))

	got, err := Expand([]string{filepath.Join(dir, "*.mobi")}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	equal(t, got, []string{abs(t, filepath.Join(dir, "a.mobi")), abs(t, filepath.Join(dir, "b.mobi"))})
}

func TestExpandGlobWithoutMatchesIsAnError(t *testing.T) {
	if _, err := Expand([]string{filepath.Join(t.TempDir(), "*.mobi")}, Options{}); err == nil {
		t.Fatal("expected an error for a glob that matches nothing")
	}
}

func TestExpandMissingPathIsAnError(t *testing.T) {
	if _, err := Expand([]string{filepath.Join(t.TempDir(), "nope.epub")}, Options{}); err == nil {
		t.Fatal("expected an error for a missing path")
	}
}

func TestExpandDeduplicates(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.mobi")
	touch(t, file)

	got, err := Expand([]string{file, file}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	equal(t, got, []string{abs(t, file)})
}

func TestExtUsesLastSegment(t *testing.T) {
	cases := map[string]string{
		"a.kepub.epub": ".epub",
		"a.MOBI":       ".mobi",
		"a.mobi.bak":   ".bak",
		"no-extension": "",
		"dir.d/a.epub": ".epub",
		"a.v2.epub":    ".epub",
	}
	for name, want := range cases {
		if got := Ext(name); got != want {
			t.Errorf("Ext(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestIsEbookAndIsEpub(t *testing.T) {
	if !IsEbook("a.MOBI") || !IsEbook("b.epub") || IsEbook("c.txt") {
		t.Fatal("IsEbook mismatch")
	}
	if !IsEpub("a.KEPUB.epub") || IsEpub("a.mobi") {
		t.Fatal("IsEpub mismatch")
	}
}

func TestBase(t *testing.T) {
	cases := map[string]string{
		"dir/a.mobi":    "a",
		"dir/a.v2.epub": "a.v2",
		"a.kepub.epub":  "a.kepub",
		"noext":         "noext",
	}
	for name, want := range cases {
		if got := Base(name); got != want {
			t.Errorf("Base(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestGlobIsCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "a.MOBI"))
	touch(t, filepath.Join(dir, "b.mobi"))
	touch(t, filepath.Join(dir, "c.epub"))

	got := glob(filepath.Join(dir, "*.mobi"))
	equal(t, got, []string{abs(t, filepath.Join(dir, "a.MOBI")), abs(t, filepath.Join(dir, "b.mobi"))})
}

func TestGlobInSubdirectory(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "sub", "书名.MOBI"))

	got := glob(filepath.Join(dir, "sub", "*.mobi"))
	equal(t, got, []string{abs(t, filepath.Join(dir, "sub", "书名.MOBI"))})
}

func TestGlobCharacterClass(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "a.epub"))
	touch(t, filepath.Join(dir, "b.epub"))
	touch(t, filepath.Join(dir, "c.epub"))

	got := glob(filepath.Join(dir, "[ab].epub"))
	equal(t, got, []string{abs(t, filepath.Join(dir, "a.epub")), abs(t, filepath.Join(dir, "b.epub"))})
}

func TestExpandGlobIsCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "Book.MOBI"))

	got, err := Expand([]string{filepath.Join(dir, "*.mobi")}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	equal(t, got, []string{abs(t, filepath.Join(dir, "Book.MOBI"))})
}

func TestGlobRelativePattern(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "a.mobi"))
	t.Chdir(dir)

	got := glob("*.mobi")
	if len(got) != 1 {
		t.Fatalf("got %v", got)
	}
	if _, err := os.Stat(got[0]); err != nil {
		t.Errorf("glob returned %q: %v", got[0], err)
	}
}
