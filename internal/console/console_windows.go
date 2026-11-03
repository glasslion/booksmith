//go:build windows

// Package console tweaks the process console so non-ASCII paths print correctly.
package console

import "syscall"

var setConsoleOutputCP = syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleOutputCP")

// SetupUTF8 switches the console output code page to UTF-8.
//
// cmd.exe and Windows PowerShell default to a legacy ANSI code page (936 on Chinese
// Windows), which turns UTF-8 output — book titles, calibre's own messages — into
// mojibake. Failing silently is fine: the call is meaningless when stdout is a pipe.
func SetupUTF8() {
	_, _, _ = setConsoleOutputCP.Call(65001)
}
