package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/emuyx/cubedock/internal/core"
)

// accountActionBarRows reports how many rows the global account action bar
// needs at this width (it sits at the bottom of the accounts page).
func (m Model) accountActionBarRows(width int) int {
	labels := []string{"登录", "导入", "批量"}
	if m.selectedIndex() >= 0 {
		labels = append(labels, "管理")
	}
	col := lipgloss.Width("账号: ")
	rows := 1
	for index, label := range labels {
		button := buttonWidth(label)
		gap := 0
		if index > 0 {
			gap = 1
		}
		if col > 0 && col+gap+button > width {
			rows++
			col = 0
			gap = 0
		}
		col += gap + button
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m Model) accountFilterMode() string {
	if m.accountFilter == "" {
		return "all"
	}
	return m.accountFilter
}

func (m Model) accountSortMode() string {
	if m.accountSort == "" {
		return "room"
	}
	return m.accountSort
}

func accountFilterLabel(mode string) string {
	switch mode {
	case "online":
		return "在线"
	case "error":
		return "异常"
	case "stopped":
		return "未运行"
	default:
		return "全部"
	}
}

func accountSortLabel(mode string) string {
	switch mode {
	case "name":
		return "名称"
	case "expiry":
		return "到期"
	default:
		return "房间"
	}
}

func cycleAccountFilter(mode string) string {
	switch mode {
	case "all":
		return "online"
	case "online":
		return "error"
	case "error":
		return "stopped"
	default:
		return "all"
	}
}

func cycleAccountSort(mode string) string {
	switch mode {
	case "room":
		return "name"
	case "name":
		return "expiry"
	default:
		return "room"
	}
}

func (m Model) accountRoom(index int) core.RoomStatus {
	if index >= 0 && index < len(m.status.Rooms) {
		return m.status.Rooms[index]
	}
	return core.RoomStatus{Phase: "stopped"}
}

func (m Model) accountHasError(index int, account core.AccountStatus) bool {
	if account.ProfileError != "" {
		return true
	}
	if room := m.accountRoom(index); room.LastError != "" {
		return true
	}
	if account.ExpiresAt != "" {
		if expires, err := time.Parse(time.RFC3339, account.ExpiresAt); err == nil && time.Now().After(expires) {
			return true
		}
	}
	return false
}

func (m Model) visibleAccountIndexes() []int {
	accounts := m.status.Accounts
	needle := strings.ToLower(strings.TrimSpace(string(m.accountQuery)))
	filter := m.accountFilterMode()
	out := make([]int, 0, len(accounts))
	for index, account := range accounts {
		phase := m.accountRoom(index).Phase
		if phase == "" {
			phase = "stopped"
		}
		hasError := m.accountHasError(index, account)
		switch filter {
		case "online":
			if phase != "online" {
				continue
			}
		case "error":
			if !hasError {
				continue
			}
		case "stopped":
			if phase != "stopped" || hasError {
				continue
			}
		}
		if needle != "" {
			room := m.accountRoom(index)
			haystack := strings.ToLower(strings.Join([]string{
				account.Name, account.Email, account.Label, account.UID, strconv.FormatUint(uint64(room.RoomID), 10),
			}, " "))
			if !strings.Contains(haystack, needle) {
				continue
			}
		}
		out = append(out, index)
	}
	switch m.accountSortMode() {
	case "name":
		sort.SliceStable(out, func(left, right int) bool {
			return strings.ToLower(accountLabel(accounts[out[left]])) <
				strings.ToLower(accountLabel(accounts[out[right]]))
		})
	case "expiry":
		expiryKey := func(index int) time.Time {
			text := accounts[index].ExpiresAt
			if text == "" {
				return time.Unix(1<<62, 0)
			}
			parsed, err := time.Parse(time.RFC3339, text)
			if err != nil {
				return time.Unix(1<<62, 0)
			}
			return parsed
		}
		sort.SliceStable(out, func(left, right int) bool {
			return expiryKey(out[left]).Before(expiryKey(out[right]))
		})
	}
	return out
}

func (m *Model) normalizeAccountSelection() {
	visible := m.visibleAccountIndexes()
	if len(visible) == 0 {
		m.selected = -1
		return
	}
	for _, index := range visible {
		if index == m.selected {
			return
		}
	}
	m.selected = visible[0]
}

func (m *Model) moveAccountSelection(delta int) {
	visible := m.visibleAccountIndexes()
	if len(visible) == 0 {
		m.selected = -1
		return
	}
	position := 0
	for index, candidate := range visible {
		if candidate == m.selected {
			position = index
			break
		}
	}
	position = clampInt(position+delta, 0, len(visible)-1)
	m.selected = visible[position]
}

func (m Model) handleAccountSearchKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "enter":
		m.accountSearch = false
		m.normalizeAccountSelection()
	case "up":
		m.moveAccountSelection(-1)
	case "down":
		m.moveAccountSelection(1)
	default:
		applyRuneKey(&m.accountQuery, &m.accountQueryCursor, key, 128)
		m.normalizeAccountSelection()
	}
	return m, nil
}

func (m Model) accountControlsInto(b *pageBuilder) {
	line := len(b.lines)
	col := 0
	var sb strings.Builder
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

	query := strings.TrimSpace(string(m.accountQuery))
	searchText := "搜索"
	searchStyle := buttonStyle
	if m.accountSearch {
		searchStyle = buttonFocusStyle
		cursor := clampInt(m.accountQueryCursor, 0, len(m.accountQuery))
		searchText = "搜索> " + string(m.accountQuery[:cursor]) + "▌" + string(m.accountQuery[cursor:])
	} else if query != "" {
		searchText = "搜索:" + query
	}
	// Keep the search segment short enough for the filter/sort chips.
	room := b.width - 30
	if room < 8 {
		room = 8
	}
	searchText = truncate(searchText, room)
	add(searchText, searchStyle, hitAccountSearch)

	filterStyle := buttonStyle
	if m.accountFilterMode() != "all" {
		filterStyle = tabActiveStyle
	}
	add("筛选:"+accountFilterLabel(m.accountFilterMode()), filterStyle, hitAccountFilter)

	sortStyle := buttonStyle
	if m.accountSortMode() != "room" {
		sortStyle = tabActiveStyle
	}
	add("排序:"+accountSortLabel(m.accountSortMode()), sortStyle, hitAccountSort)

	count := fmt.Sprintf("%d/%d", len(m.visibleAccountIndexes()), len(m.status.Accounts))
	if col > 0 && col+1+len(count) <= b.width {
		sb.WriteString(" ")
		sb.WriteString(dimStyle.Render(count))
		col += 1 + len(count)
	}
	if b.width >= 60 && len(m.status.Accounts) > 0 {
		online, failed := 0, 0
		for index, account := range m.status.Accounts {
			if m.accountRoom(index).Phase == "online" {
				online++
			}
			if m.accountHasError(index, account) {
				failed++
			}
		}
		chips := make([]string, 0, 2)
		if online > 0 {
			chips = append(chips, onlineStyle.Render(fmt.Sprintf("● 在线 %d", online)))
		}
		if failed > 0 {
			chips = append(chips, errorStyle.Render(fmt.Sprintf("× 异常 %d", failed)))
		}
		if len(chips) > 0 {
			text := "  " + strings.Join(chips, " ")
			if col+lipgloss.Width(text) <= b.width {
				sb.WriteString(text)
			}
		}
	}
	b.add(truncate(sb.String(), b.width))
}
