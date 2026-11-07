//go:build windows

package console

import (
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/term"
)

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	procConsoleIn  = kernel32.NewProc("GetConsoleMode")
	procConsoleOut = kernel32.NewProc("SetConsoleMode")
)

// supportsColor enables VT escape processing when stdout is a console.
func supportsColor() bool {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return false
	}
	handle := syscall.Handle(os.Stdout.Fd())
	var mode uint32
	if ok, _, _ := procConsoleIn.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode))); ok == 0 {
		return false
	}
	const virtualTerminal = 0x0004
	ok, _, _ := procConsoleOut.Call(uintptr(handle), uintptr(mode|virtualTerminal))
	return ok != 0
}
