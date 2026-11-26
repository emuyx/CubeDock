package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// fieldBoxRow is one editable row inside a bordered field group.
type fieldBoxRow struct {
	label     string
	value     string
	active    bool
	clickable bool
	kind      pageHitKind
	index     int
}

func displayFieldValue(value []rune, cursor int, masked, active bool) string {
	cursor = clampInt(cursor, 0, len(value))
	display := append([]rune(nil), value...)
	if masked {
		for index := range display {
			display[index] = '*'
		}
	}
	if active {
		marked := make([]rune, 0, len(display)+1)
		marked = append(marked, display[:cursor]...)
		marked = append(marked, '▌')
		marked = append(marked, display[cursor:]...)
		display = marked
	}
	return string(display)
}

func (m Model) fieldBoxTop(b *pageBuilder, title string, width int, focused bool) {
	border := frameBorderStyle
	titleStyle := frameTitleStyle
	if focused {
		border = frameFocusStyle
		titleStyle = frameFocusStyle
	}
	maxTitle := width - 3
	if maxTitle < 1 {
		maxTitle = 1
	}
	titleText := truncate(" "+title+" ", maxTitle)
	fill := width - 3 - lipgloss.Width(titleText)
	if fill < 0 {
		fill = 0
	}
	b.add(border.Render("╭─") + titleStyle.Render(titleText) + border.Render(strings.Repeat("─", fill)+"╮"))
}

// addFieldBox draws a bordered group around one or more single-line fields.
// Each clickable row records a hit covering the full box width.
func (m Model) addFieldBox(b *pageBuilder, title string, rows []fieldBoxRow, width int) {
	if len(rows) == 0 {
		return
	}
	if width < 10 {
		for _, row := range rows {
			line := len(b.lines)
			text := truncate(row.label+": "+row.value, width)
			if row.active {
				text = fieldActiveStyle.Render(text)
			}
			b.add(text)
			if row.clickable {
				b.hit(line, 0, width, row.kind, row.index)
			}
		}
		return
	}
	labelWidth := 0
	for _, row := range rows {
		if got := lipgloss.Width(row.label); got > labelWidth {
			labelWidth = got
		}
	}
	labelWidth += 2
	if labelWidth > width/2 {
		labelWidth = width / 2
	}
	focused := false
	for _, row := range rows {
		if row.active {
			focused = true
			break
		}
	}
	border := frameBorderStyle
	if focused {
		border = frameFocusStyle
	}
	m.fieldBoxTop(b, title, width, focused)
	inner := width - 2
	for _, row := range rows {
		line := len(b.lines)
		marker := "  "
		if row.active {
			marker = "▌ "
		}
		label := row.label + strings.Repeat(" ", maxInt(0, labelWidth-lipgloss.Width(row.label)))
		content := marker + label + row.value
		if row.active {
			content = fieldActiveStyle.Render(padRightPlain(truncate(content, inner), inner))
		} else {
			content = marker + dimStyle.Render(label) + textStyle.Render(row.value)
			content = padRightPlain(truncate(content, inner), inner)
		}
		b.add(border.Render("│") + content + border.Render("│"))
		if row.clickable {
			b.hit(line, 0, width, row.kind, row.index)
		}
	}
	b.add(border.Render("╰" + strings.Repeat("─", maxInt(0, width-2)) + "╯"))
}

// addTextAreaBox draws a bordered multi-line input area.
func (m Model) addTextAreaBox(b *pageBuilder, title string, lines []string, width int, focused bool) {
	if width < 8 {
		for _, line := range lines {
			b.add(truncate(line, width))
		}
		return
	}
	m.fieldBoxTop(b, title, width, focused)
	border := frameBorderStyle
	if focused {
		border = frameFocusStyle
	}
	inner := width - 2
	if len(lines) == 0 {
		lines = []string{""}
	}
	for _, line := range lines {
		b.add(border.Render("│") + padRightPlain(truncate(line, inner), inner) + border.Render("│"))
	}
	b.add(border.Render("╰" + strings.Repeat("─", maxInt(0, width-2)) + "╯"))
}

// renderInputBar is the compact one-line input used where vertical space is
// tight (console command and log search).
func renderInputBar(text string, width int, focused bool) string {
	if width < 6 {
		return truncate(text, width)
	}
	inner := width - 2
	text = truncate(text, inner)
	pad := inner - lipgloss.Width(text)
	if pad < 0 {
		pad = 0
	}
	style := buttonStyle
	if focused {
		style = fieldActiveStyle
	}
	return "▐" + style.Render(text+strings.Repeat(" ", pad)) + "▌"
}
