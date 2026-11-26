package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/core"
	"github.com/emuyx/cubedock/internal/netease"
)

const minWidth = 34
const minHeight = 10

func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "CubeDock 控制台启动中…"
	}
	if m.width < minWidth || m.height < minHeight {
		return fitBlock(m.tinyView(), m.width, m.height)
	}

	info := m.layoutInfo()
	body := m.bodyView(info)
	return fitBlock(info.header+"\n"+body+"\n"+info.footer, info.width, info.height)
}

func (m Model) bodyView(info screenLayout) string {
	if info.sidebar {
		nav := fitLines(m.navView(info), info.navArea.W, info.bodyH)
		content := fitLines(m.contentView(info), info.contentArea.W, info.bodyH)
		rows := make([]string, 0, info.bodyH)
		for index := 0; index < info.bodyH; index++ {
			rows = append(rows, nav[index]+borderStyle.Render("│")+content[index])
		}
		return strings.Join(rows, "\n")
	}
	nav := fitLines(m.navView(info), info.width, info.navArea.H)
	content := fitLines(m.contentView(info), info.width, info.contentArea.H)
	return strings.Join(append(nav, content...), "\n")
}

func (m Model) navView(info screenLayout) []string {
	sidebar := info.sidebar
	labels := info.navLabels
	lines := make([]string, 0, len(labels))
	if sidebar {
		for index, label := range labels {
			text := " " + label + " "
			style := navStyle
			if pageOrder[index] == m.page {
				text = "[" + label + "]"
				style = navActiveStyle
			}
			if index < len(info.navRects) && m.mouseActive && info.navRects[index].Contains(m.mouseX, m.mouseY) {
				if pageOrder[index] != m.page {
					style = navHoverStyle
				}
			}
			if pageOrder[index] == m.page && m.focus == focusNav {
				style = style.Underline(true)
			}
			lines = append(lines, style.Render(padRightPlain(truncate(text, info.navArea.W), info.navArea.W)))
		}
		return lines
	}
	rows := make(map[int][]int)
	order := make([]int, 0, len(labels))
	for index := range labels {
		if index >= len(info.navRects) {
			break
		}
		row := info.navRects[index].Y
		if _, exists := rows[row]; !exists {
			order = append(order, row)
		}
		rows[row] = append(rows[row], index)
	}
	for _, row := range order {
		segments := make([]string, 0, len(rows[row]))
		for _, index := range rows[row] {
			text := " " + labels[index] + " "
			style := navStyle
			if pageOrder[index] == m.page {
				text = "[" + labels[index] + "]"
				style = navActiveStyle
			}
			if m.mouseActive && info.navRects[index].Contains(m.mouseX, m.mouseY) && pageOrder[index] != m.page {
				style = navHoverStyle
			}
			if pageOrder[index] == m.page && m.focus == focusNav {
				style = style.Underline(true)
			}
			segments = append(segments, style.Render(text))
		}
		lines = append(lines, strings.Join(segments, " "))
	}
	return lines
}

func (m Model) contentView(info screenLayout) []string {
	lines := make([]string, 0, info.contentArea.H)
	if len(info.toolbarRects) > 0 {
		lines = append(lines, m.toolbarLines(info)...)
	}
	frameHeight := info.contentArea.H - len(lines)
	if frameHeight < 1 {
		frameHeight = 1
	}
	pageLines := m.pageLines(info)
	if info.framed {
		lines = append(lines, m.frameView(info, pageLines)...)
	} else {
		visible, _, _ := windowLines(pageLines, frameHeight, m.contentOffset)
		lines = append(lines, fitLines(visible, info.contentArea.W, frameHeight)...)
	}
	return fitLines(lines, info.contentArea.W, info.contentArea.H)
}

func (m Model) pageLines(info screenLayout) []string {
	if m.modal != modalNone {
		switch m.modal {
		case modalLogin:
			return m.loginView(info.content.W)
		case modalName:
			return m.nameView(info.content.W)
		case modalAdmin:
			return m.adminView(info.content.W)
		case modalRestore:
			return m.restoreView(info.content.W)
		case modalProfileTags:
			return m.tagsView(info.content.W)
		case modalProfileRoomName:
			return m.roomNameView(info.content.W)
		default:
			return m.importView(info.content.W, info.content.H)
		}
	}
	switch m.page {
	case pageHome:
		return m.homeView(info.content.W)
	case pageAccounts:
		return m.roomsView(info)
	case pageProfiles:
		if m.profileEditing {
			return m.profileFormView(info.content.W)
		}
		return m.profilesView(info.content.W)
	case pageLogs:
		return m.logsView(info.content.W, info.content.H)
	case pageDiagnostics:
		return m.diagnosticsView(info.content.W)
	case pageSettings:
		return m.settingsView(info.content.W)
	case pageFAQ:
		return m.helpView(info.content.W)
	default:
		return nil
	}
}

func (m Model) toolbarLines(info screenLayout) []string {
	lines := make([]string, 0, maxInt(1, info.toolbarRows))
	segments := make([]string, 0, len(info.toolbar))
	currentRow := -1
	flush := func() {
		if len(segments) == 0 {
			return
		}
		lines = append(lines, truncate(strings.Join(segments, " "), info.contentArea.W))
		segments = segments[:0]
	}
	for index, item := range info.toolbar {
		if index >= len(info.toolbarRects) {
			break
		}
		row := info.toolbarRects[index].Y
		if currentRow == -1 {
			currentRow = row
		} else if row != currentRow {
			flush()
			currentRow = row
		}
		focused := m.focus == focusActions && index == m.actionIndex
		hovered := m.mouseActive && info.toolbarRects[index].Contains(m.mouseX, m.mouseY)
		text := " " + item.Label + " "
		style := buttonStyle
		if focused {
			text = "[" + item.Label + "]"
			style = buttonFocusStyle
		} else if hovered {
			style = buttonHoverStyle
		}
		segments = append(segments, style.Render(text))
	}
	flush()
	return fitLines(lines, info.contentArea.W, maxInt(1, info.toolbarRows))
}

func (m Model) frameView(info screenLayout, content []string) []string {
	if !info.framed {
		return content
	}
	title := pageLabel(m.page)
	switch m.modal {
	case modalImportSauth:
		title = "添加账号 · Sauth"
	case modalLogin:
		title = "登录网易账号"
	case modalName:
		title = "设置游戏名"
	case modalAdmin:
		title = "初始化管理员"
	case modalRestore:
		title = "恢复配置"
	case modalProfileTags:
		title = "大厅标签"
	case modalProfileRoomName:
		title = "房间名称"
	}
	if m.profileEditing {
		title = "配置方案 · 编辑"
		if m.profileDirty {
			title += " *"
		}
	}
	border := frameBorderStyle
	if m.focus == focusContent {
		border = frameFocusStyle
	}
	label := " " + title + " "
	if lipgloss.Width(label) > info.frame.W-2 {
		label = truncate(label, maxInt(1, info.frame.W-2))
	}
	fill := info.frame.W - 2 - lipgloss.Width(label)
	if fill < 0 {
		fill = 0
	}
	window, offset, maxOffset := windowLines(content, info.content.H, m.contentOffset)
	window = fitLines(window, info.content.W, info.content.H)
	thumbStart, thumbSize := scrollbarThumb(len(content), info.content.H, offset, maxOffset)
	lines := []string{
		border.Render("╭") + frameTitleStyle.Render(label) + border.Render(strings.Repeat("─", fill)+"╮"),
	}
	for index, line := range window {
		right := "│"
		style := border
		if maxOffset > 0 && index >= thumbStart && index < thumbStart+thumbSize {
			right = "┃"
			style = frameTitleStyle
		}
		lines = append(lines, border.Render("│")+line+style.Render(right))
	}
	lines = append(lines, border.Render("╰"+strings.Repeat("─", maxInt(0, info.frame.W-2))+"╯"))
	return lines
}

func windowLines(lines []string, height, offset int) ([]string, int, int) {
	if height < 1 {
		height = 1
	}
	maxOffset := len(lines) - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	end := offset + height
	if end > len(lines) {
		end = len(lines)
	}
	return lines[offset:end], offset, maxOffset
}

func scrollbarThumb(total, height, offset, maxOffset int) (int, int) {
	if maxOffset <= 0 || total <= height {
		return 0, 0
	}
	thumbSize := height * height / total
	if thumbSize < 1 {
		thumbSize = 1
	}
	if thumbSize > height {
		thumbSize = height
	}
	span := height - thumbSize
	if span < 0 {
		span = 0
	}
	thumbStart := 0
	if maxOffset > 0 {
		thumbStart = offset * span / maxOffset
	}
	return thumbStart, thumbSize
}

func (m Model) tinyView() string {
	lines := []string{
		panelTitleStyle.Render("CubeDock"),
		fmt.Sprintf("终端太小：%d×%d", m.width, m.height),
		fmt.Sprintf("至少需要 %d×%d", minWidth, minHeight),
		"请放大终端窗口",
	}
	if m.options.LocalURL != "" {
		lines = append(lines, truncate("WebUI "+m.options.LocalURL, maxInt(1, m.width)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) headerView() string {
	width := maxInt(1, m.width)
	// The clock keeps the static header line changing every second. Some
	// terminals clear the very first frame while entering the alternate screen;
	// a live header makes sure the brand and WebUI address are repainted.
	clock := time.Now().Format("15:04:05")
	left := brandStyle.Render(" CubeDock ") +
		versionStyle.Render(" "+m.options.Version) +
		dimStyle.Render("  "+clock)
	right := linkLabelStyle.Render("WebUI ") + linkStyle.Render(m.options.LocalURL)
	lines := make([]string, 0, 5)
	if lipgloss.Width(left)+lipgloss.Width(right)+1 <= width {
		lines = append(lines, joinLR(left, right, width))
	} else {
		lines = append(lines, truncate(left, width))
		lines = append(lines, truncate(dimStyle.Render(clock+"  ")+right, width))
	}
	lines = append(lines, m.statusLine(width))
	if m.options.SetupRequired {
		lines = append(lines, warnStyle.Render("管理员尚未初始化：打开 WebUI 创建管理员"))
	}
	lines = append(lines, borderStyle.Render(strings.Repeat("─", width)))
	return strings.Join(lines, "\n")
}

func (m Model) spinner() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	return frames[m.frame%len(frames)]
}

func (m Model) busyLabel(label string) string {
	return infoStyle.Render(m.spinner() + " " + label)
}

func (m Model) statusLine(width int) string {
	phase := m.status.Phase
	if phase == "" {
		phase = "stopped"
	}
	style := phaseStyle(phase)
	symbol := "●"
	if !colorEnabled {
		symbol = "*"
	}
	parts := []string{
		style.Render(symbol + " " + phaseLabel(phase)),
		dimStyle.Render(fmt.Sprintf("%d 个房间 · %d 位玩家", activeRoomCount(m.status), m.status.Players)),
	}
	if m.busy {
		parts = append(parts, m.busyLabel("处理中"))
	}
	if m.options.AdminName != "" && width >= 110 {
		parts = append(parts, dimStyle.Render("管理员 "+m.options.AdminName))
	}
	if len(m.options.LANURLs) > 0 && width >= 90 {
		parts = append(parts, linkLabelStyle.Render("局域网 ")+linkStyle.Render(m.options.LANURLs[0]))
	}
	return truncate(strings.Join(parts, dimStyle.Render("  ·  ")), width)
}

func (m Model) footerView() string {
	width := maxInt(1, m.width)
	lines := []string{borderStyle.Render(strings.Repeat("─", width))}
	if m.confirmQuit {
		lines = append(lines, warnStyle.Render("确定退出 CubeDock？ y 确认 / n 取消"))
		return strings.Join(lines, "\n")
	}
	if m.notice != "" {
		text := m.notice
		if m.busy {
			text = "处理中… " + text
		}
		lines = append(lines, truncate(accentStyle.Render(text), width))
	} else if m.busy {
		lines = append(lines, m.busyLabel("处理中"))
	}
	lines = append(lines, truncate(dimStyle.Render(m.keyHints(width)), width))
	return strings.Join(lines, "\n")
}

func (m Model) keyHints(width int) string {
	switch m.modal {
	case modalImportSauth:
		if width >= 60 {
			return "Enter 导入  Esc 取消  直接粘贴 sauth/session JSON"
		}
		return "Enter 导入  Esc 取消"
	case modalLogin:
		if width >= 70 {
			return "Tab/↑↓ 切换字段  ←→ 切换平台  Enter 登录  Esc 取消"
		}
		return "Tab 字段  ←→ 平台  Enter 登录  Esc"
	case modalName:
		if width >= 60 {
			return "Enter 保存游戏名  Esc 跳过（稍后可用 WebUI 设置）"
		}
		return "Enter 保存  Esc 跳过"
	case modalAdmin:
		if width >= 60 {
			return "Tab/↑↓ 切换字段  Enter 创建管理员  Esc 稍后"
		}
		return "Tab 字段  Enter 创建  Esc 稍后"
	case modalRestore:
		if width >= 60 {
			return "Enter 恢复配置  Esc 取消  相对路径按数据目录解析"
		}
		return "Enter 恢复  Esc 取消"
	}
	suffix := ""
	if m.modal == modalProfileTags {
		if width >= 60 {
			return "↑↓ 选择  Space 切换  Enter/Esc 完成"
		}
		return "↑↓ Space Enter"
	}
	if m.modal == modalProfileRoomName {
		if width >= 60 {
			return "↑↓ 选择内置房间名  Enter 使用  Esc 取消"
		}
		return "↑↓ Enter Esc"
	}
	if m.profileEditing {
		if width >= 90 {
			return "↑↓ 字段  Enter 编辑/切换  Ctrl+S 保存  Esc 取消  PgUp/PgDn 滚动"
		}
		return "↑↓ 字段 Enter 编辑 Ctrl+S 保存 Esc"
	}
	switch m.page {
	case pageProfiles:
		suffix = "  Enter 编辑"
	case pageAccounts:
		suffix = "  s/x/r 操作"
	case pageLogs:
		suffix = "  f 筛选"
	case pageDiagnostics:
		suffix = "  Enter/R 诊断"
	case pageSettings:
		if m.options.SetupRequired {
			suffix = "  a 初始化管理员"
		}
	}
	switch {
	case width >= 120:
		return "Tab 焦点  ↑↓ 选择/滚动  ←→ 页面/选项  Enter 执行  鼠标可点击" + suffix + "  q 退出"
	case width >= 90:
		return "Tab 焦点  ↑↓ 选择  ←→ 切换  Enter 执行  鼠标可点" + suffix + "  q 退出"
	case width >= 60:
		return "Tab 焦点 ↑↓选择 ←→切换 Enter执行 鼠标" + suffix + " q退出"
	default:
		return "Tab ↑↓ ←→ Enter 鼠标 ? q"
	}
}

func (m Model) roomsView(info screenLayout) []string {
	width, height := info.content.W, info.content.H
	if width <= 0 || height <= 0 {
		return nil
	}
	if roomsWide(width) {
		leftWidth := clampInt(width/3, 24, 38)
		rightWidth := width - leftWidth - 3
		if rightWidth >= 24 {
			left := fitLines(m.roomListLines(leftWidth, info.roomRows, info.roomRowIndex), leftWidth, height)
			right := fitLines(m.roomDetailLines(rightWidth), rightWidth, height)
			rows := make([]string, 0, height)
			for index := 0; index < height; index++ {
				rows = append(rows, left[index]+borderStyle.Render(" │ ")+right[index])
			}
			return rows
		}
	}
	return m.roomsNarrow(info)
}

func (m Model) roomsNarrow(info screenLayout) []string {
	width, height := info.content.W, info.content.H
	listHeight := compactListHeight(len(m.status.Accounts), height)
	detailHeight := height - listHeight
	lines := fitLines(m.roomListLines(width, info.roomRows, info.roomRowIndex), width, listHeight)
	lines = append(lines, fitLines(m.roomDetailLines(width), width, detailHeight)...)
	return lines
}

func (m Model) roomListLines(width int, rowRects []rect, indexes []int) []string {
	if len(m.status.Accounts) == 0 {
		return []string{
			dimStyle.Render("还没有账号。"),
			dimStyle.Render("按 o 打开 WebUI 添加。"),
		}
	}
	lines := make([]string, 0, len(indexes))
	for row, index := range indexes {
		if index < 0 || index >= len(m.status.Accounts) {
			continue
		}
		account := m.status.Accounts[index]
		room := core.RoomStatus{Phase: "stopped"}
		if index < len(m.status.Rooms) {
			room = m.status.Rooms[index]
		}
		phase := room.Phase
		if phase == "" {
			phase = "stopped"
		}
		suffix := " " + phaseLabel(phase)
		if room.RoomID != 0 {
			suffix += fmt.Sprintf(" %d", room.RoomID)
		}
		marker := " "
		if index == m.selectedIndex() {
			marker = "▌"
		}
		prefix := fmt.Sprintf("%s%2d ", marker, index+1)
		nameWidth := width - lipgloss.Width(prefix) - lipgloss.Width(suffix)
		if nameWidth < 4 {
			nameWidth = 4
		}
		line := prefix + truncate(accountLabel(account), nameWidth) + suffix
		line = padRightPlain(truncate(line, width), width)
		hovered := m.mouseActive && row < len(rowRects) && rowRects[row].Contains(m.mouseX, m.mouseY)
		switch {
		case index == m.selectedIndex():
			lines = append(lines, selectedRowStyle.Render(line))
		case hovered:
			lines = append(lines, rowHoverStyle.Render(line))
		default:
			lines = append(lines, textStyle.Render(line))
		}
	}
	return lines
}

func (m Model) roomDetailLines(width int) []string {
	room, account, ok := m.selectedRoom()
	if !ok {
		return []string{dimStyle.Render("没有选中账号。")}
	}
	lines := []string{
		panelTitleStyle.Render(truncate(fmt.Sprintf("房间 %d · %s", m.selectedIndex()+1, accountLabel(account)), width)),
	}
	phase := room.Phase
	if phase == "" {
		phase = "stopped"
	}
	lines = append(lines, phaseStyle(phase).Render(truncate("状态: "+phaseLabel(phase), width)))
	if room.RoomID != 0 {
		lines = append(lines, textStyle.Render(truncate(fmt.Sprintf("房间号: %d", room.RoomID), width)))
	}
	profile := m.profileFor(account)
	if profile != nil {
		lines = append(lines, textStyle.Render(truncate(fmt.Sprintf("配置: %s（%s）", profile.Name, connectionLabel(profile.ConnectionMode)), width)))
		if profile.ConnectionMode == config.ConnectionTransfer {
			lines = append(lines, dimStyle.Render(truncate(fmt.Sprintf("中转: %s → %s", profile.LimboUpstream, profile.Upstream), width)))
		} else {
			lines = append(lines, dimStyle.Render(truncate("服务器: "+profile.Upstream, width)))
		}
	}
	if room.ConnectedAt != "" {
		lines = append(lines, textStyle.Render(truncate("在线: "+durationText(room.ConnectedAt), width)))
	}
	lines = append(lines, m.roomPlayerLine(width, room, profile))
	if room.UpstreamOnline {
		line := "服务器已连接"
		if room.UpstreamVersion != "" {
			line += " · " + room.UpstreamVersion
		}
		lines = append(lines, onlineStyle.Render(truncate(line, width)))
	} else if phase != "stopped" {
		lines = append(lines, warnStyle.Render("服务器未连接"))
	}
	if room.RetryAt != "" {
		lines = append(lines, warnStyle.Render(truncate("等待重连："+localTime(room.RetryAt), width)))
	}
	if room.LastError != "" {
		lines = append(lines, errorStyle.Render(truncate("上次错误："+room.LastError, width)))
	}
	if account.Email != "" {
		lines = append(lines, dimStyle.Render(truncate("账号: "+account.Email, width)))
	}
	if account.AuthMethod != "" {
		lines = append(lines, dimStyle.Render(truncate("登录方式: "+authMethodLabel(account.AuthMethod), width)))
	}
	lines = append(lines, dimStyle.Render(truncate("s 启动 · x 停止 · r 重连", width)))
	return lines
}

func (m Model) roomPlayerLine(width int, room core.RoomStatus, profile *config.Profile) string {
	if profile == nil {
		return dimStyle.Render(truncate("玩家: —", width))
	}
	if profile.ConnectionMode == config.ConnectionTransfer {
		return dimStyle.Render(truncate("玩家: 转服模式，人数由目标服务器管理", width))
	}
	maxPlayers := profile.MaxPlayers
	if maxPlayers <= 0 {
		maxPlayers = 10
	}
	return textStyle.Render(truncate(fmt.Sprintf("玩家: %d / %d（含房主占位）", room.Players, maxPlayers), width))
}

func (m Model) profileFor(account core.AccountStatus) *config.Profile {
	id := account.ProfileID
	if id == "" {
		id = m.status.Config.DefaultProfileID()
	}
	for index := range m.status.Config.Profiles {
		if m.status.Config.Profiles[index].ID == id {
			profile := m.status.Config.Profiles[index]
			return &profile
		}
	}
	if len(m.status.Config.Profiles) > 0 {
		profile := m.status.Config.Profiles[0]
		return &profile
	}
	return nil
}

func (m Model) logsView(width, height int) []string {
	events := m.visibleEvents()
	header := fmt.Sprintf("日志 · %s · %d 条 · ↑↓ 滚动", m.logFilterLabel(), len(events))
	lines := []string{dimStyle.Render(truncate(header, width))}
	visible := height - 1
	if visible < 1 {
		return lines
	}
	if len(events) == 0 {
		return append(lines, dimStyle.Render("暂无日志。"))
	}
	end := len(events) - m.logOffset
	if end > len(events) {
		end = len(events)
	}
	if end < 0 {
		end = 0
	}
	start := end - visible
	if start < 0 {
		start = 0
	}
	for _, event := range events[start:end] {
		line := fmt.Sprintf("%s [%s] %s", event.Time, event.Source, event.Message)
		line = truncate(line, width)
		switch {
		case event.Level == "error":
			line = errorStyle.Render(line)
		case event.Level == "warning":
			line = warnStyle.Render(line)
		case event.Level == "debug" || strings.HasPrefix(event.Message, "[DEBUG]"):
			line = dimStyle.Render(line)
		default:
			line = textStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return lines
}

func (m Model) loginView(width int) []string {
	platformLabel := "互通版"
	if m.loginPlatform == string(netease.AuthMobileNetEase) {
		platformLabel = "网易手游"
	}
	platformText := "平台: " + platformLabel
	if m.loginField == 2 {
		if m.loginPlatform == string(netease.AuthMobileNetEase) {
			platformText = "平台: 互通版 [网易手游]"
		} else {
			platformText = "平台: [互通版] 网易手游"
		}
	}
	lines := []string{
		textStyle.Render("使用网易邮箱和密码登录；互通版或网易手游。"),
		dimStyle.Render("4399 账号/图形验证码请使用 WebUI。"),
		dimStyle.Render(truncate("配置方案: "+m.defaultProfileLabel(), width)),
		dimStyle.Render(""),
		fieldLine("邮箱", m.loginEmail, m.loginEmailCursor, false, m.loginField == 0, width),
		fieldLine("密码", m.loginPassword, m.loginPasswordCursor, true, m.loginField == 1, width),
		textStyle.Render(truncate(platformText, width)),
		dimStyle.Render("Tab/↑↓ 切换字段，←→ 切换平台，Enter 登录"),
	}
	if m.modalBusy {
		lines = append(lines, m.busyLabel("正在登录并验证网易账号"))
	}
	if m.loginVerifyNote != "" || m.loginVerifyURL != "" {
		lines = append(lines, warnStyle.Render(truncate(m.loginVerifyNote, width)))
		if m.loginVerifyURL != "" {
			for _, line := range wrapDisplay(m.loginVerifyURL, maxInt(1, width-2)) {
				lines = append(lines, linkStyle.Render(truncate(line, width)))
			}
			lines = append(lines, dimStyle.Render("Ctrl+O 打开验证链接；完成后再次按 Enter。"))
		}
	}
	if m.modalErr != "" {
		lines = append(lines, errorStyle.Render(truncate(m.modalErr, width)))
	}
	return lines
}

func (m Model) restoreView(width int) []string {
	lines := []string{
		textStyle.Render("从 CubeDock 配置备份恢复方案和全局设置。"),
		dimStyle.Render("不恢复网易登录凭证；房间必须已停止。"),
		dimStyle.Render("相对路径按数据目录解析，例如 cubedock-backup-20260101-120000.json。"),
		dimStyle.Render(""),
		fieldLine("备份路径", m.input, m.inputCursor, false, true, width),
		dimStyle.Render("Enter 恢复，Esc 取消。"),
	}
	if m.modalBusy {
		lines = append(lines, m.busyLabel("正在读取并恢复配置"))
	}
	if m.modalErr != "" {
		lines = append(lines, errorStyle.Render(truncate(m.modalErr, width)))
	}
	return lines
}

func (m Model) adminView(width int) []string {
	lines := []string{
		textStyle.Render("创建本机管理员账号，用于登录 WebUI 和终端控制台。"),
		dimStyle.Render("管理员密码只保存在本机 admin.json；初始化只允许在本机完成。"),
		dimStyle.Render(""),
		fieldLine("管理员账号", m.adminUser, m.adminUserCursor, false, m.adminField == 0, width),
		fieldLine("密码", m.adminPassword, m.adminPasswordCursor, true, m.adminField == 1, width),
		fieldLine("确认密码", m.adminConfirm, m.adminConfirmCursor, true, m.adminField == 2, width),
		dimStyle.Render("密码 8–72 字节；Tab/↑↓ 切换字段，Enter 创建。"),
	}
	if m.modalBusy {
		lines = append(lines, m.busyLabel("正在创建管理员"))
	}
	if m.modalErr != "" {
		lines = append(lines, errorStyle.Render(truncate(m.modalErr, width)))
	}
	if m.options.AdminName != "" && !m.options.SetupRequired {
		lines = append(lines, dimStyle.Render("当前管理员: "+m.options.AdminName))
	}
	return lines
}

func (m Model) nameView(width int) []string {
	lines := []string{
		textStyle.Render("账号已添加，但网易还没有返回游戏名。"),
		dimStyle.Render("游戏名长度 3–12 个字符；中文按 1.5 个字符计算。"),
		dimStyle.Render(""),
		fieldLine("游戏名", m.nameInput, m.nameCursor, false, true, width),
		dimStyle.Render("Enter 保存，Esc 跳过（稍后可用 WebUI 设置）。"),
	}
	if m.modalBusy {
		lines = append(lines, m.busyLabel("正在保存游戏名"))
	}
	if m.modalErr != "" {
		lines = append(lines, errorStyle.Render(truncate(m.modalErr, width)))
	}
	return lines
}

func fieldLine(label string, value []rune, cursor int, masked, active bool, width int) string {
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
	return textStyle.Render(truncate(label+": "+string(display), width))
}

func (m Model) importView(width, height int) []string {
	lines := []string{
		textStyle.Render("粘贴 sauth/session JSON（互通版 / 网易手游 / 4399）："),
		dimStyle.Render(truncate("配置方案: "+m.defaultProfileLabel(), width)),
		dimStyle.Render("Enter 导入，Esc 取消；邮箱/密码登录仍可在 WebUI 完成。"),
		dimStyle.Render(""),
	}
	if m.modalBusy {
		lines = append(lines, m.busyLabel("正在验证会话并添加账号"))
	}
	if m.modalErr != "" {
		lines = append(lines, errorStyle.Render(truncate(m.modalErr, width)))
	}
	inputHeight := height - len(lines) - 2
	if inputHeight < 2 {
		inputHeight = 2
	}
	text := ""
	if len(m.input) > 0 {
		cursor := m.inputCursor
		if cursor < 0 {
			cursor = 0
		}
		if cursor > len(m.input) {
			cursor = len(m.input)
		}
		text = string(m.input[:cursor]) + "▌" + string(m.input[cursor:])
	} else {
		text = "在这里粘贴…▌"
	}
	wrapped := wrapDisplay(text, maxInt(1, width-2))
	if len(wrapped) > inputHeight {
		wrapped = wrapped[len(wrapped)-inputHeight:]
		if len(wrapped) > 0 {
			wrapped[0] = "…" + wrapped[0]
		}
	}
	for _, line := range wrapped {
		lines = append(lines, textStyle.Render(truncate(line, width)))
	}
	lines = append(lines, dimStyle.Render("支持多行粘贴；内容不会写入日志。"))
	return lines
}

func (m Model) defaultProfileLabel() string {
	id := m.status.Config.DefaultProfileID()
	for _, profile := range m.status.Config.Profiles {
		if profile.ID == id {
			return profile.Name + "（" + id + "）"
		}
	}
	return id
}

func wrapDisplay(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	lines := make([]string, 0, 4)
	var builder strings.Builder
	used := 0
	for _, r := range text {
		if r == '\n' {
			lines = append(lines, builder.String())
			builder.Reset()
			used = 0
			continue
		}
		runeWidth := lipgloss.Width(string(r))
		if runeWidth > 0 && used+runeWidth > width {
			lines = append(lines, builder.String())
			builder.Reset()
			used = 0
		}
		builder.WriteRune(r)
		used += runeWidth
	}
	lines = append(lines, builder.String())
	return lines
}

func (m Model) helpView(width int) []string {
	lines := []string{
		panelTitleStyle.Render("常见问题与终端操作"),
		dimStyle.Render("TUI 与 WebUI 同时运行，操作同一套房间。"),
	}
	if m.options.LocalURL != "" {
		lines = append(lines, textStyle.Render(truncate("WebUI: "+m.options.LocalURL, width)))
	}
	lines = append(lines,
		textStyle.Render("登录/验证码、复杂配置、备份恢复在哪里？→ WebUI"),
		textStyle.Render("房间启动失败？→ 检查服务器是否支持网易 RakNet v8、地址和会话"),
		textStyle.Render("鼠标无效？→ 终端需支持鼠标；Windows 推荐 Windows Terminal"),
		textStyle.Render("窗口太小？→ 调大窗口/字号；Termux 建议横屏"),
		dimStyle.Render(""),
		panelTitleStyle.Render("按键"),
		textStyle.Render("1–7 直接切页    Tab/Shift+Tab 切换焦点"),
		textStyle.Render("↑↓ 选择/滚动    ←→ 切换页面或按钮"),
		textStyle.Render("Enter 执行     鼠标点击导航/房间/按钮"),
		textStyle.Render("s/x/r 启停/重连  S/X 全部启停  d 调试"),
		textStyle.Render("o WebUI  f 日志筛选  R 诊断  q 退出"),
	)
	if m.options.SetupRequired {
		lines = append(lines, warnStyle.Render("管理员尚未初始化：打开 WebUI 创建。"))
	}
	return lines
}

func (m Model) homeView(width int) []string {
	lines := []string{}
	phase := m.status.Phase
	if phase == "" {
		phase = "stopped"
	}
	lines = append(lines, phaseStyle(phase).Render(truncate("● "+phaseLabel(phase)+"  ·  "+
		fmt.Sprintf("%d 个房间  ·  %d 位玩家", activeRoomCount(m.status), m.status.Players), width)))
	if m.options.LocalURL != "" {
		lines = append(lines, textStyle.Render(truncate("WebUI: "+m.options.LocalURL, width)))
	}
	admin := m.options.AdminName
	if admin == "" {
		admin = "未设置"
	}
	state := "已初始化"
	if m.options.SetupRequired {
		state = "尚未初始化，请用 WebUI 创建"
	}
	lines = append(lines, dimStyle.Render(truncate("管理员: "+admin+"（"+state+"）", width)))
	lines = append(lines, dimStyle.Render(""))
	if len(m.status.Accounts) == 0 {
		lines = append(lines, dimStyle.Render("还没有账号。请用 WebUI 登录或导入账号。"))
		return lines
	}
	lines = append(lines, panelTitleStyle.Render("运行中的房间"))
	active := 0
	for index, room := range m.status.Rooms {
		if room.Phase == "" || room.Phase == "stopped" {
			continue
		}
		active++
		account := "—"
		if index < len(m.status.Accounts) {
			account = accountLabel(m.status.Accounts[index])
		}
		roomText := fmt.Sprintf("房间 %d · %s · %s", index+1, account, phaseLabel(room.Phase))
		if room.RoomID != 0 {
			roomText += fmt.Sprintf(" · %d", room.RoomID)
		}
		lines = append(lines, phaseStyle(room.Phase).Render(truncate(roomText, width)))
	}
	if active == 0 {
		lines = append(lines, dimStyle.Render("当前没有运行中的房间。"))
	}
	lines = append(lines, dimStyle.Render(""))
	lines = append(lines, panelTitleStyle.Render("最近事件"))
	events := m.status.Events
	if len(events) > 6 {
		events = events[len(events)-6:]
	}
	if len(events) == 0 {
		lines = append(lines, dimStyle.Render("暂无事件。"))
	}
	for _, event := range events {
		lines = append(lines, textStyle.Render(truncate(
			fmt.Sprintf("%s [%s] %s", event.Time, event.Source, event.Message), width)))
	}
	return lines
}

func (m Model) profilesView(width int) []string {
	profiles := m.status.Config.Profiles
	lines := []string{
		panelTitleStyle.Render(fmt.Sprintf("配置方案 · %d 个", len(profiles))),
		dimStyle.Render("每个账号选择一个方案；复杂编辑请在 WebUI「配置方案」中完成。"),
		dimStyle.Render(""),
	}
	if len(profiles) == 0 {
		lines = append(lines, dimStyle.Render("还没有配置方案，请打开 WebUI 添加。"))
		return lines
	}
	for index, profile := range profiles {
		count := m.accountsUsingProfile(profile.ID)
		marker := "  "
		if index == m.profileIndex {
			marker = "▌ "
		}
		line := fmt.Sprintf("%s● %s（%s）· %s · %d 个账号",
			marker, profile.Name, profile.ID, connectionLabel(profile.ConnectionMode), count)
		line = padRightPlain(truncate(line, width), width)
		if index == m.profileIndex {
			lines = append(lines, selectedRowStyle.Render(line))
		} else {
			lines = append(lines, textStyle.Render(line))
		}
		lines = append(lines, dimStyle.Render(truncate("  "+profile.Upstream, width)))
	}
	lines = append(lines, dimStyle.Render(""))
	lines = append(lines, dimStyle.Render("Enter 编辑当前方案；工具栏可新增/复制/删除。"))
	return lines
}

func (m Model) diagnosticsView(width int) []string {
	if m.diagnosticsBusy {
		return []string{m.busyLabel("正在运行诊断"), dimStyle.Render("诊断不会登录网易账号。")}
	}
	if len(m.diagnostics) == 0 {
		return []string{
			panelTitleStyle.Render("运行诊断"),
			dimStyle.Render("按 Enter 或点击「运行诊断」检查配置、服务器、中转房间和资源包。"),
			dimStyle.Render("诊断只读，不会登录网易账号。"),
		}
	}
	counts := map[string]int{}
	for _, check := range m.diagnostics {
		counts[check.Status]++
	}
	lines := []string{
		panelTitleStyle.Render(fmt.Sprintf("诊断 · 通过 %d · 注意 %d · 失败 %d · 跳过 %d",
			counts[core.DiagnosticOK], counts[core.DiagnosticWarn],
			counts[core.DiagnosticFail], counts[core.DiagnosticSkip])),
	}
	if !m.diagnosticsAt.IsZero() {
		lines = append(lines, dimStyle.Render("运行时间: "+m.diagnosticsAt.Local().Format("15:04:05")))
	}
	lines = append(lines, dimStyle.Render(""))
	for _, check := range m.diagnostics {
		style := textStyle
		symbol := "○"
		switch check.Status {
		case core.DiagnosticOK:
			style = onlineStyle
			symbol = "●"
		case core.DiagnosticWarn:
			style = warnStyle
			symbol = "!"
		case core.DiagnosticFail:
			style = errorStyle
			symbol = "×"
		}
		lines = append(lines, style.Render(truncate(symbol+" "+check.Label+" · "+check.Category, width)))
		if check.Detail != "" {
			lines = append(lines, dimStyle.Render(truncate("  "+check.Detail, width)))
		}
		if check.Hint != "" {
			lines = append(lines, warnStyle.Render(truncate("  "+check.Hint, width)))
		}
	}
	return lines
}

func (m Model) settingsView(width int) []string {
	cfg := m.status.Config
	dataDir := "—"
	if m.manager != nil {
		dataDir = m.manager.Dir()
	}
	lines := []string{
		panelTitleStyle.Render("全局设置"),
		dimStyle.Render("详细编辑请在 WebUI「全局设置」中完成。"),
		dimStyle.Render(""),
		textStyle.Render(truncate(fmt.Sprintf("控制台端口: %d", cfg.WebPort), width)),
		textStyle.Render(truncate("数据目录: "+dataDir, width)),
		textStyle.Render(truncate("自动启动全部房间: "+boolLabel(cfg.AutoStart), width)),
		textStyle.Render(truncate(fmt.Sprintf("空闲账号保活: %s（%d 分钟）", boolLabel(cfg.KeepAlive), cfg.KeepAliveMinutes), width)),
		textStyle.Render(truncate("Debug 日志: "+boolLabel(cfg.Debug), width)),
	}
	if cfg.LimboPath != "" {
		lines = append(lines, textStyle.Render(truncate("limbo 路径: "+cfg.LimboPath, width)))
	} else {
		lines = append(lines, dimStyle.Render("limbo 路径: 自动查找主程序同目录"))
	}
	admin := m.options.AdminName
	if admin == "" {
		admin = "未设置"
	}
	lines = append(lines, textStyle.Render(truncate("管理员: "+admin, width)))
	if m.options.SetupRequired {
		lines = append(lines, warnStyle.Render("管理员尚未初始化：按 a 在 TUI 创建，或打开 WebUI 创建。"))
	}
	if m.lastBackup != "" {
		lines = append(lines, dimStyle.Render(truncate("最近备份: "+m.lastBackup, width)))
	}
	lines = append(lines, dimStyle.Render("配置备份为 JSON；恢复只应用配置，不恢复网易登录凭证。"))
	return lines
}

func (m Model) accountsUsingProfile(profileID string) int {
	fallback := m.status.Config.DefaultProfileID()
	count := 0
	for _, account := range m.status.Accounts {
		id := account.ProfileID
		if id == "" {
			id = fallback
		}
		if id == profileID {
			count++
		}
	}
	return count
}

func boolLabel(value bool) string {
	if value {
		return "开启"
	}
	return "关闭"
}

func activeRoomCount(status core.Status) int {
	count := 0
	for _, room := range status.Rooms {
		if room.Phase != "" && room.Phase != "stopped" {
			count++
		}
	}
	return count
}

func accountLabel(account core.AccountStatus) string {
	for _, value := range []string{account.Name, account.Label, account.Email} {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return fmt.Sprintf("房间 %d", account.Index+1)
}

func phaseLabel(phase string) string {
	switch phase {
	case "stopped":
		return "已停止"
	case "starting":
		return "启动中"
	case "checking_upstream":
		return "检查服务器"
	case "authenticating":
		return "鉴权中"
	case "creating":
		return "创建房间"
	case "online":
		return "在线"
	case "stopping":
		return "停止中"
	case "retrying":
		return "等待重连"
	case "upstream_offline":
		return "后端离线"
	default:
		return phase
	}
}

func connectionLabel(mode string) string {
	if mode == "transfer" {
		return "转服"
	}
	return "代理"
}

func authMethodLabel(method string) string {
	switch method {
	case "mobile_netease":
		return "网易手游"
	case "mobile_4399":
		return "4399 手游"
	default:
		return "基岩互通版"
	}
}

func durationText(iso string) string {
	started, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return "—"
	}
	elapsed := time.Since(started)
	if elapsed < 0 {
		elapsed = 0
	}
	seconds := int(elapsed.Seconds())
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	if hours > 0 {
		return fmt.Sprintf("%d 时 %d 分", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%d 分 %d 秒", minutes, seconds%60)
	}
	return fmt.Sprintf("%d 秒", seconds)
}

func localTime(iso string) string {
	value, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	return value.Local().Format("15:04:05")
}

func fitBlock(block string, width, height int) string {
	return strings.Join(fitLines(strings.Split(block, "\n"), width, height), "\n")
}

func fitLines(lines []string, width, height int) []string {
	width = maxInt(1, width)
	if height < 0 {
		height = 0
	}
	out := make([]string, 0, height)
	for index := 0; index < height; index++ {
		line := ""
		if index < len(lines) {
			line = lines[index]
		}
		if lipgloss.Width(line) > width {
			line = ansi.Truncate(line, width, "…")
		}
		if used := lipgloss.Width(line); used < width {
			line += strings.Repeat(" ", width-used)
		}
		out = append(out, line)
	}
	return out
}

func joinLR(left, right string, width int) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	if leftWidth+rightWidth+1 > width {
		right = truncate(right, maxInt(1, width-1))
		rightWidth = lipgloss.Width(right)
		left = truncate(left, maxInt(1, width-rightWidth-1))
		leftWidth = lipgloss.Width(left)
	}
	gap := width - leftWidth - rightWidth
	if gap < 0 {
		gap = 0
	}
	return left + strings.Repeat(" ", gap) + right
}

func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	return ansi.Truncate(text, width, "…")
}

func padRightPlain(text string, width int) string {
	if used := lipgloss.Width(text); used < width {
		return text + strings.Repeat(" ", width-used)
	}
	return text
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
