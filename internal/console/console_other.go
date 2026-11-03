//go:build !windows

// Package console tweaks the process console so non-ASCII paths print correctly.
package console

// SetupUTF8 is a no-op outside Windows.
func SetupUTF8() {}
