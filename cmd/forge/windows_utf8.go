//go:build windows

package main

import "syscall"

// init sets the Windows console input/output code page to UTF-8 (CP 65001)
// so that Unicode symbols (✓ ✗ ⚠ — → •) display correctly in PowerShell,
// cmd.exe, and Windows Terminal without requiring the user to run `chcp 65001`
// manually. The kernel32 calls are no-ops when stdout is not a console (e.g.
// piped to a file), so this is safe in all execution contexts.
func init() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	_, _, _ = kernel32.NewProc("SetConsoleOutputCP").Call(65001)
	_, _, _ = kernel32.NewProc("SetConsoleCP").Call(65001)
}
