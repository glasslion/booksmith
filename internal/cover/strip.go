// Package cover carries the calibre script that strips a cover from an EPUB.
package cover

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed strip_cover.py
var stripScript string

// WriteScript writes the strip script into dir and returns its path. The script is run
// through calibre-debug, so it must live on disk.
func WriteScript(dir string) (string, error) {
	path := filepath.Join(dir, "booksmith-strip-cover.py")
	if err := os.WriteFile(path, []byte(stripScript), 0o600); err != nil {
		return "", err
	}
	return path, nil
}
