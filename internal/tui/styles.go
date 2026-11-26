package tui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// colorEnabled follows the NO_COLOR convention. The TUI still works without
// colour; only the escape sequences are dropped.
var colorEnabled = os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"

// terminalColor returns an adaptive colour, or no colour when the user asked
// for a plain terminal. Lip Gloss measures styled strings correctly, so the
// layout code can keep using the same styles in both modes.
func terminalColor(light, dark string) lipgloss.TerminalColor {
	if !colorEnabled {
		return lipgloss.NoColor{}
	}
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

var (
	textStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#3a2b3a", "#f4f1fb"))
	dimStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#7c6b7f", "#b3b0c8"))
	borderStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#a86a8c", "#eee2ff")).Faint(true)
	brandStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#9e3e63", "#f2a0bc")).Bold(true)
	versionStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#7c6b7f", "#b3b0c8"))
	linkLabelStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#7c6b7f", "#b3b0c8"))
	linkStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#2d7d6c", "#8fe3d2")).Underline(true)
	onlineStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#2d7d6c", "#8fe3d2")).Bold(true)
	stoppedStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#7c6b7f", "#b3b0c8"))
	infoStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#4a6f9e", "#a8c8ff"))
	warnStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#9a6a1e", "#ffd79a")).Bold(true)
	errorStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#b83a55", "#ff9aae")).Bold(true)
	accentStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#6d5bb8", "#c6b4ff")).Bold(true)
	selectedRowStyle = lipgloss.NewStyle().
				Foreground(terminalColor("#3a2b3a", "#f4f1fb")).
				Background(terminalColor("#f3e3ee", "#2b2e46")).Bold(true)
	tabActiveStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#2d7d6c", "#8fe3d2")).
			Background(terminalColor("#edf8f5", "#26384a")).Bold(true)
	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(terminalColor("#7c6b7f", "#b3b0c8"))
	panelTitleStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#6d5bb8", "#c6b4ff")).Bold(true)
	frameBorderStyle = lipgloss.NewStyle().
				Foreground(terminalColor("#a86a8c", "#c6b4ff"))
	frameFocusStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#2d7d6c", "#8fe3d2")).Bold(true)
	frameTitleStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#6d5bb8", "#c6b4ff")).
			Background(terminalColor("#f3e3ee", "#2b2e46")).Bold(true)
	buttonStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#3a2b3a", "#f4f1fb")).
			Background(terminalColor("#ead9ea", "#30344e"))
	buttonHoverStyle = lipgloss.NewStyle().
				Foreground(terminalColor("#2b1621", "#ffffff")).
				Background(terminalColor("#dec3dd", "#3d4262")).Bold(true)
	buttonFocusStyle = lipgloss.NewStyle().
				Foreground(terminalColor("#ffffff", "#102924")).
				Background(terminalColor("#c95d86", "#8fe3d2")).Bold(true)
	rowHoverStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#3a2b3a", "#f4f1fb")).
			Background(terminalColor("#f7edf3", "#24263a"))
	rowAltStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#3a2b3a", "#f4f1fb")).
			Background(terminalColor("#faf3f8", "#202234"))
	fieldActiveStyle = lipgloss.NewStyle().
				Foreground(terminalColor("#2b1621", "#ffffff")).
				Background(terminalColor("#e7d4e7", "#343852")).Bold(true)
	sectionStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#9e3e63", "#c6b4ff")).Bold(true)
	navStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#7c6b7f", "#b3b0c8"))
	navActiveStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#ffffff", "#102924")).
			Background(terminalColor("#c95d86", "#8fe3d2")).Bold(true)
	navHoverStyle = lipgloss.NewStyle().
			Foreground(terminalColor("#3a2b3a", "#f4f1fb")).
			Background(terminalColor("#f3e3ee", "#343852"))
	navFocusStyle = lipgloss.NewStyle().Underline(true)
)

func phaseStyle(phase string) lipgloss.Style {
	switch phase {
	case "online":
		return onlineStyle
	case "retrying", "upstream_offline", "checking_upstream":
		return warnStyle
	case "starting", "authenticating", "creating", "stopping":
		return infoStyle
	default:
		return stoppedStyle
	}
}
