package batch

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// glob expands a pattern one path segment at a time, matching case-insensitively.
//
// filepath.Glob matches case-sensitively even on Windows, where the filesystem does not,
// so "*.mobi" would miss "Book.MOBI". Matching is case-insensitive on every platform so
// a pattern means the same thing everywhere.
func glob(pattern string) []string {
	clean := filepath.Clean(pattern)
	volume := filepath.VolumeName(clean)
	rest := clean[len(volume):]

	start := volume
	if strings.HasPrefix(rest, string(filepath.Separator)) {
		start = volume + string(filepath.Separator)
		rest = rest[1:]
	} else {
		start = filepath.Join(volume, ".")
		if volume == "" {
			start = "."
		}
	}

	matches := []string{start}
	for _, segment := range strings.Split(rest, string(filepath.Separator)) {
		if segment == "" {
			continue
		}
		var next []string
		if !hasGlobMeta(segment) {
			for _, m := range matches {
				next = append(next, filepath.Join(m, segment))
			}
			matches = next
			continue
		}
		next = nil
		for _, m := range matches {
			entries, err := os.ReadDir(m)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if foldMatch(segment, e.Name()) {
					next = append(next, filepath.Join(m, e.Name()))
				}
			}
		}
		matches = next
	}

	var out []string
	for _, m := range matches {
		if _, err := os.Stat(m); err == nil {
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out
}

// foldMatch reports whether name matches pattern, ignoring case.
func foldMatch(pattern, name string) bool {
	ok, err := filepath.Match(strings.ToLower(pattern), strings.ToLower(name))
	return err == nil && ok
}
