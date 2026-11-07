//go:build !windows

package console

import (
	"os"

	"golang.org/x/term"
)

// supportsColor asks the terminal package instead of inspecting os.FileMode,
// which also covers PTYs and redirected descriptors correctly.
func supportsColor() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
