// Package batch turns command-line arguments into a list of concrete ebook files.
//
// Globs are expanded here rather than by the shell, because cmd.exe and PowerShell do
// not expand them. Directory scans only pick up known ebook extensions unless all is
// set; explicit file arguments are always accepted.
package batch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ebookExts is what a directory scan accepts by default.
var ebookExts = map[string]bool{
	".mobi": true,
	".azw":  true,
	".azw3": true,
	".epub": true,
	".pdf":  true,
}

// Options controls how arguments are expanded.
type Options struct {
	Recursive bool
	All       bool
}

// Ext returns the lower-cased final extension of name, so "book.kepub.epub" is ".epub".
func Ext(name string) string {
	return strings.ToLower(filepath.Ext(name))
}

// IsEbook reports whether name has a known ebook extension.
func IsEbook(name string) bool {
	return ebookExts[Ext(name)]
}

// IsEpub reports whether name is an EPUB.
func IsEpub(name string) bool {
	return Ext(name) == ".epub"
}

// Base returns name without its final extension.
func Base(name string) string {
	return strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
}

// Expand resolves each argument to concrete files, preserving argument order and
// de-duplicating. A glob that matches nothing, or a path that does not exist, is an
// error: silently converting zero files is never what the user wanted.
func Expand(args []string, o Options) ([]string, error) {
	var out []string
	seen := make(map[string]bool)

	add := func(p string) {
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		if seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}

	for _, arg := range args {
		if hasGlobMeta(arg) {
			matches := glob(arg)
			if len(matches) == 0 {
				return nil, fmt.Errorf("no files match %q", arg)
			}
			for _, m := range matches {
				if err := collect(m, o, add); err != nil {
					return nil, err
				}
			}
			continue
		}
		if err := collect(arg, o, add); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func hasGlobMeta(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

func collect(path string, o Options, add func(string)) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if !info.IsDir() {
		// An explicitly named file is always accepted, whatever its extension.
		add(path)
		return nil
	}
	if o.Recursive {
		return filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !accept(d.Name(), o) {
				return nil
			}
			add(p)
			return nil
		})
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !accept(e.Name(), o) {
			continue
		}
		add(filepath.Join(path, e.Name()))
	}
	return nil
}

func accept(name string, o Options) bool {
	return o.All || IsEbook(name)
}
