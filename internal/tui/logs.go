package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/emuyx/cubedock/internal/fileutil"
)

// combinedLogs merges server events and the local console transcript, applies
// the room filter and the search query, and returns display order oldest first.
func (m Model) combinedLogs() []consoleLine {
	events := m.visibleEvents()
	out := make([]consoleLine, 0, len(events)+len(m.consoleTranscript))
	for _, event := range events {
		out = append(out, consoleLine{
			time: event.Time, level: event.Level,
			text: "[" + event.Source + "] " + event.Message,
		})
	}
	out = append(out, m.consoleTranscript...)
	needle := strings.ToLower(strings.TrimSpace(string(m.logQuery)))
	if needle == "" {
		return out
	}
	filtered := out[:0]
	for _, line := range out {
		if strings.Contains(strings.ToLower(line.time+" "+line.text), needle) {
			filtered = append(filtered, line)
		}
	}
	return filtered
}

func (m Model) handleLogSearchKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "enter":
		m.logSearch = false
	case "up":
		m.scrollLogs(1)
	case "down":
		m.scrollLogs(-1)
	default:
		applyRuneKey(&m.logQuery, &m.logQueryCursor, key, 128)
		m.logOffset = 0
	}
	return m, nil
}

func (m *Model) exportLogs() {
	if m.manager == nil {
		m.setNotice("房间管理器不可用")
		return
	}
	lines := m.combinedLogs()
	if len(lines) == 0 {
		m.setNotice("没有可导出的日志")
		return
	}
	var text strings.Builder
	text.WriteString("CubeDock 日志导出\n")
	text.WriteString("导出时间: " + time.Now().Format(time.RFC3339) + "\n")
	if query := strings.TrimSpace(string(m.logQuery)); query != "" {
		text.WriteString("搜索: " + query + "\n")
	}
	text.WriteString("筛选: " + m.logFilterLabel() + "\n\n")
	for _, line := range lines {
		text.WriteString(line.time + " " + line.text + "\n")
	}
	path := filepath.Join(m.manager.Dir(),
		"cubedock-logs-"+time.Now().Format("20060102-150405")+".txt")
	if err := fileutil.Replace(path, []byte(text.String()), 0600); err != nil {
		m.setNotice("导出日志失败：" + err.Error())
		return
	}
	m.setNotice("日志已导出：" + path)
}

func (m Model) logControlsInto(b *pageBuilder) {
	line := len(b.lines)
	col := 0
	var sb strings.Builder
	prefix := fmt.Sprintf("日志 · %d 条 ", len(m.combinedLogs()))
	sb.WriteString(dimStyle.Render(prefix))
	col = lipgloss.Width(prefix)
	add := func(text string, style lipgloss.Style, kind pageHitKind) {
		text = " " + text + " "
		width := lipgloss.Width(text)
		gap := 0
		if col > 0 {
			gap = 1
		}
		if col+gap+width > b.width {
			return
		}
		if gap > 0 {
			sb.WriteString(" ")
			col++
		}
		if b.hovered(line, col, width) {
			style = buttonHoverStyle
		}
		sb.WriteString(style.Render(text))
		b.hit(line, col, width, kind, 0)
		col += width
	}

	filterStyle := buttonStyle
	if m.logWatch {
		filterStyle = tabActiveStyle
	}
	add("筛选:"+m.logFilterLabel(), filterStyle, hitToggleLogFilter)

	searchText := "搜索"
	if query := strings.TrimSpace(string(m.logQuery)); query != "" && !m.logSearch {
		searchText = "搜索:" + query
	}
	searchStyle := buttonStyle
	if m.logSearch {
		searchStyle = buttonFocusStyle
	}
	room := b.width - col - 24
	if room < 8 {
		room = 8
	}
	add(truncate(searchText, room), searchStyle, hitLogSearch)
	add("导出", buttonStyle, hitLogExport)

	hint := "  ↑↓ 滚动  i 命令"
	if col+lipgloss.Width(hint) <= b.width {
		sb.WriteString(dimStyle.Render(hint))
	}
	b.add(truncate(sb.String(), b.width))
}
