//go:build windows

package console

import (
	"os"

	"golang.org/x/term"
)

var procSetConsoleOutput = kernel32.NewProc("SetConsoleOutputCP")

// EnableUTF8 switches an interactive Windows console to UTF-8 so the Chinese
// banner and TUI do not depend on the user's legacy code page. Redirected
// output and services are left alone.
func EnableUTF8() {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return
	}
	const utf8CodePage = 65001
	_, _, _ = procSetConsoleOutput.Call(utf8CodePage)
}
