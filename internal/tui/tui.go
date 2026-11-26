// Package tui renders CubeDock's terminal console. The web console keeps
// running at the same time: both views call the same core.Manager.
//
// The terminal UI is deliberately opt-in only by environment, not by a command
// line flag. It starts when stdin and stdout are interactive terminals; a
// redirected or dumb terminal keeps the plain banner and log output.
package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/emuyx/cubedock/internal/core"
)

// Options is everything the terminal view needs from the process entry point.
type Options struct {
	Manager       *core.Manager
	Version       string
	Commit        string
	AdminName     string
	SetupRequired bool
	LocalURL      string
	LANURLs       []string

	// OpenURL opens the browser for the "o" key. It is also used by the normal
	// startup path when -headless is not set.
	OpenURL func(string)

	// CreateAdmin is the first-run administrator callback. It is only usable
	// while SetupRequired is true.
	CreateAdmin func(username, password string) error

	// Output is optional and mainly useful for tests.
	Output io.Writer
}

// Enabled reports whether this process can host an interactive terminal UI.
// Both streams must be terminals; a pipeline such as `cubedock | tee` keeps the
// plain output instead of starting a full-screen program that cannot read keys.
func Enabled() bool {
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// Run starts the full-screen console and blocks until the user quits or ctx is
// cancelled. It restores the terminal before returning.
func Run(ctx context.Context, options Options) error {
	if options.Manager == nil {
		return errors.New("终端控制台缺少 room manager")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if options.OpenURL == nil {
		options.OpenURL = func(string) {}
	}
	out := options.Output
	if out == nil {
		out = os.Stdout
	}

	program := tea.NewProgram(
		newModel(options),
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
		tea.WithOutput(out),
		tea.WithContext(ctx),
	)
	if _, err := program.Run(); err != nil && !errors.Is(err, tea.ErrProgramKilled) {
		return fmt.Errorf("终端控制台退出: %w", err)
	}
	return nil
}
