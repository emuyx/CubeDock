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
		width := info.navArea.W
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render(padRightPlain(truncate(" 我的房间", width), width)))
		accounts := m.status.Accounts
		if len(accounts) == 0 {
			lines = append(lines, dimStyle.Render(padRightPlain(truncate("  还没有账号", width), width)))
			return lines
		}
		for row, index := range info.accountRowIndex {
			if index < 0 || index >= len(accounts) {
				continue
			}
			account := accounts[index]
			phase := m.accountRoom(index).Phase
			if phase == "" {
				phase = "stopped"
			}
			symbol := "○"
			style := textStyle
			switch {
			case phase == "online":
				symbol, style = "●", onlineStyle
			case phase != "stopped":
				symbol, style = "◐", warnStyle
			}
			marker := " "
			if index == m.selectedIndex() {
				marker = "▌"
			}
			prefix := fmt.Sprintf("%s%d ", marker, index+1)
			suffix := " " + symbol
			nameWidth := width - lipgloss.Width(prefix) - lipgloss.Width(suffix)
			if nameWidth < 4 {
				nameWidth = 4
			}
			text := padRightPlain(prefix+truncate(accountLabel(account), nameWidth)+suffix, width)
			switch {
			case m.modal == modalAccountDashboard && m.accountActionIndex == index:
				style = navActiveStyle
			case row < len(info.accountRects) && m.mouseActive && info.accountRects[row].Contains(m.mouseX, m.mouseY):
				style = navHoverStyle
			}
			lines = append(lines, style.Render(text))
		}
		if len(accounts) > len(info.accountRowIndex) {
			overflow := fmt.Sprintf(" … +%d", len(accounts)-len(info.accountRowIndex))
			lines = append(lines, dimStyle.Render(padRightPlain(truncate(overflow, width), width)))
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
	frameHeight := info.contentArea.H
	if frameHeight < 1 {
		frameHeight = 1
	}
	if info.framed {
		return fitLines(m.frameView(info, info.pageLines), info.contentArea.W, info.contentArea.H)
	}
	visible, _, _ := windowLines(info.pageLines, frameHeight, m.contentOffset)
	return fitLines(visible, info.contentArea.W, frameHeight)
}

// buildPage renders the active page (or modal) and returns its lines plus the
// local clickable regions. layoutInfo calls it exactly once per layout, so the
// rendered buttons and their hitboxes can never drift apart.
func (m Model) buildPage(info screenLayout) ([]string, []pageHit) {
	if info.content.W < 1 || info.content.H < 1 {
		return nil, nil
	}
	width := info.content.W
	if m.modal != modalNone {
		b := newPageBuilder(m, width, info.content.X, info.content.Y)
		m.modalInto(b, info)
		return b.lines, b.hits
	}
	switch m.page {
	case pageHome:
		b := newPageBuilder(m, width, info.content.X, info.content.Y)
		m.homeInto(b)
		return b.lines, b.hits
	case pageAccounts:
		return m.accountsPage(info)
	case pageProfiles:
		b := newPageBuilder(m, width, info.content.X, info.content.Y)
		if m.profileEditing {
			m.profileFormInto(b)
		} else {
			m.profilesInto(b)
		}
		return b.lines, b.hits
	case pageLogs:
		b := newPageBuilder(m, width, info.content.X, info.content.Y)
		m.logsInto(b, info.content.H)
		return b.lines, b.hits
	case pageDiagnostics:
		b := newPageBuilder(m, width, info.content.X, info.content.Y)
		m.diagnosticsInto(b)
		return b.lines, b.hits
	case pageSettings:
		b := newPageBuilder(m, width, info.content.X, info.content.Y)
		m.settingsInto(b)
		return b.lines, b.hits
	case pageFAQ:
		b := newPageBuilder(m, width, info.content.X, info.content.Y)
		m.helpInto(b)
		return b.lines, b.hits
	}
	return nil, nil
}

func (m Model) modalInto(b *pageBuilder, info screenLayout) {
	switch m.modal {
	case modalLogin:
		m.loginInto(b)
	case modalName:
		m.nameInto(b)
	case modalAdmin:
		m.adminInto(b)
	case modalRestore:
		m.restoreInto(b)
	case modalProfileTags:
		m.tagsInto(b)
	case modalProfileRoomName:
		m.roomNameInto(b)
	case modalAccountActions:
		m.accountActionsInto(b)
	case modalAssignProfile:
		m.assignProfileInto(b)
	case modalPlayers:
		m.playersInto(b)
	case modalRoomMembers:
		m.membersInto(b)
	case modalFriends:
		m.friendsInto(b)
	case modalPlayerTransfer:
		m.transferInto(b)
	case modalResourcePacks:
		m.packsInto(b)
	case modalSettings:
		m.settingsEditorInto(b)
	case modalAdminPassword:
		m.adminPasswordInto(b)
	case modalBulkImport:
		m.bulkInto(b, info.content.H)
	case modalAccountDashboard:
		m.accountDashboardInto(b)
	default:
		m.importInto(b, info.content.H)
	}
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
		if m.nameRename {
			title = "修改游戏名"
		} else {
			title = "设置游戏名"
		}
	case modalAdmin:
		title = "初始化管理员"
	case modalRestore:
		title = "恢复配置"
	case modalProfileTags:
		title = "大厅标签"
	case modalProfileRoomName:
		title = "房间名称"
	case modalAccountActions:
		title = "账号管理"
	case modalAssignProfile:
		title = "切换配置方案"
	case modalPlayers:
		title = "玩家管理"
	case modalRoomMembers:
		title = "房间成员"
	case modalFriends:
		title = "好友申请"
	case modalPlayerTransfer:
		title = "转服玩家"
	case modalResourcePacks:
		title = "资源包"
	case modalSettings:
		title = "全局设置 · 编辑"
	case modalAdminPassword:
		title = "修改管理员密码"
	case modalBulkImport:
		title = "批量导入账号"
	case modalAccountDashboard:
		title = "房间仪表盘"
	}
	if m.modal == modalLogin && m.loginReplace {
		title = "重新登录账号"
	}
	if m.modal == modalNone && m.profileEditing {
		title = "配置方案 · 编辑"
		if m.profileDirty {
			title += " *"
		}
	}
	border := frameBorderStyle
	tl, tr, hz, vt, bl, br := "╭", "╮", "─", "│", "╰", "╯"
	if m.focus == focusContent {
		border = frameFocusStyle
		// A double border gives the active panel a raised, beveled edge.
		tl, tr, hz, vt, bl, br = "╔", "╗", "═", "║", "╚", "╝"
	}
	label := " " + title + " "
	if lipgloss.Width(label) > info.frame.W-2 {
		label = truncate(label, maxInt(1, info.frame.W-2))
	}
	window, offset, maxOffset := windowLines(content, info.content.H, m.contentOffset)
	window = fitLines(window, info.content.W, info.content.H)
	scrollLabel := ""
	if maxOffset > 0 {
		visibleEnd := offset + info.content.H
		if visibleEnd > len(content) {
			visibleEnd = len(content)
		}
		scrollLabel = fmt.Sprintf(" %d/%d ", visibleEnd, len(content))
		if lipgloss.Width(label)+lipgloss.Width(scrollLabel) > info.frame.W-2 {
			scrollLabel = ""
		}
	}
	fill := info.frame.W - 2 - lipgloss.Width(label) - lipgloss.Width(scrollLabel)
	if fill < 0 {
		fill = 0
	}
	thumbStart, thumbSize := scrollbarThumb(len(content), info.content.H, offset, maxOffset)
	lines := []string{
		border.Render(tl) + frameTitleStyle.Render(label) +
			border.Render(strings.Repeat(hz, fill)) + dimStyle.Render(scrollLabel) + border.Render(tr),
	}
	for index, line := range window {
		right := vt
		style := border
		if maxOffset > 0 && index >= thumbStart && index < thumbStart+thumbSize {
			right = "┃"
			style = frameTitleStyle
		}
		lines = append(lines, border.Render(vt)+line+style.Render(right))
	}
	lines = append(lines, border.Render(bl+strings.Repeat(hz, maxInt(0, info.frame.W-2))+br))
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
	lines = append(lines, borderStyle.Render(strings.Repeat("━", width)))
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
	lines := []string{borderStyle.Render(strings.Repeat("━", width))}
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
		if width >= 80 {
			return "Tab/↑↓ 切换字段  ←→ 切换平台  Enter 登录  Ctrl+O 验证/图片  Esc 取消"
		}
		return "Tab 字段  ←→ 平台  Enter 登录  Ctrl+O  Esc"
	case modalName:
		if m.nameRename {
			if width >= 60 {
				return "Enter 保存新游戏名  Esc 取消  随机名可点击上方按钮"
			}
			return "Enter 保存  Esc 取消"
		}
		if width >= 60 {
			return "Enter 保存游戏名  Esc 跳过（稍后可再设置）"
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
	case modalAccountActions:
		if width >= 60 {
			return "↑↓ 选择  Enter 执行  Esc 关闭  删除需要操作两次"
		}
		return "↑↓ Enter Esc"
	case modalAssignProfile:
		if width >= 60 {
			return "↑↓ 选择  Enter 应用  Esc 取消  房间运行中不能切换"
		}
		return "↑↓ Enter Esc"
	case modalPlayers:
		if width >= 60 {
			return "↑↓ 选择  x 踢出  t 转服  r 刷新  Esc 关闭"
		}
		return "↑↓ x t r Esc"
	case modalRoomMembers:
		if width >= 60 {
			return "↑↓ 选择  x 踢出  r 刷新  Esc 关闭"
		}
		return "↑↓ x r Esc"
	case modalFriends:
		if width >= 60 {
			return "↑↓ 选择  a 同意  r 刷新  Esc 关闭"
		}
		return "↑↓ a r Esc"
	case modalPlayerTransfer:
		return "Enter 发送  Esc 返回"
	case modalResourcePacks:
		if width >= 60 {
			return "↑↓ 选择  r 刷新列表  p 拉取  Esc 关闭"
		}
		return "r 刷新  p 拉取  Esc"
	case modalSettings:
		if width >= 60 {
			return "↑↓ 字段  Enter 编辑/切换  Ctrl+S 保存  Esc 取消"
		}
		return "↑↓ Enter Ctrl+S Esc"
	case modalAdminPassword:
		if width >= 60 {
			return "Tab/↑↓ 切换字段  Enter 保存  Esc 取消"
		}
		return "Tab Enter Esc"
	case modalBulkImport:
		if m.modalBusy {
			return "正在导入…  Esc 停止"
		}
		return "Enter 开始导入  Esc 关闭"
	case modalAccountDashboard:
		if width >= 90 {
			return "s/x/r 启停/重连  d 诊断  u 刷新  n 起名  p/m/f 互动  Esc 关闭"
		}
		return "s/x/r d u n p/m/f Esc"
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
	if m.logSearch {
		return "输入搜索词  Enter/Esc 退出  Ctrl+U 清空"
	}
	switch m.page {
	case pageAccounts:
		suffix = "  s/x/r 操作  m 管理  / 搜索  f 筛选  v 排序"
	case pageLogs:
		suffix = "  / 搜索  f 筛选  e 导出  i 命令"
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

const accountControlsRows = 1

func (m Model) accountsPage(info screenLayout) ([]string, []pageHit) {
	width, height := info.content.W, info.content.H
	if width <= 0 || height <= 0 {
		return nil, nil
	}
	bottomRows := m.accountActionBarRows(width)
	bodyHeight := height - accountControlsRows - bottomRows
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	if roomsWide(width) {
		leftWidth := clampInt(width/3, 24, 38)
		rightWidth := width - leftWidth - 3
		if rightWidth >= 24 {
			left := fitLines(m.roomListLines(leftWidth, info.roomRows, info.roomRowIndex), leftWidth, bodyHeight)
			right := newPageBuilder(m, rightWidth, info.content.X, info.content.Y+accountControlsRows)
			right.colBase = leftWidth + 3
			m.roomDetailInto(right)
			rightLines := fitLines(right.lines, rightWidth, bodyHeight)
			b := newPageBuilder(m, width, info.content.X, info.content.Y)
			m.accountControlsInto(b)
			for index := 0; index < bodyHeight; index++ {
				b.add(left[index] + borderStyle.Render(" │ ") + rightLines[index])
			}
			for index := range right.hits {
				right.hits[index].Y += accountControlsRows
			}
			m.accountActionBarInto(b)
			return b.lines, append(b.hits, right.hits...)
		}
	}
	b := newPageBuilder(m, width, info.content.X, info.content.Y)
	m.accountControlsInto(b)
	listHeight := compactListHeight(len(m.visibleAccountIndexes()), bodyHeight)
	list := fitLines(m.roomListLines(width, info.roomRows, info.roomRowIndex), width, listHeight)
	for _, line := range list {
		b.add(line)
	}
	m.roomDetailInto(b)
	bodyLimit := height - bottomRows
	if bodyLimit < 1 {
		bodyLimit = 1
	}
	if len(b.lines) > bodyLimit {
		// The narrow layout has no separate detail scroller; clip like the old
		// fit-to-remaining-height behaviour instead of hiding content below an
		// unreachable fold.
		b.lines = b.lines[:bodyLimit]
		visible := b.hits[:0]
		for _, hit := range b.hits {
			if hit.Y < bodyLimit {
				visible = append(visible, hit)
			}
		}
		b.hits = visible
	}
	for len(b.lines) < bodyLimit {
		b.add("")
	}
	m.accountActionBarInto(b)
	return b.lines, b.hits
}

func (m Model) accountActionBarInto(b *pageBuilder) {
	items := m.toolbarItems()
	manage := -1
	if m.selectedIndex() >= 0 {
		manage = toolbarIndexSpecial(items, specialAccountManage)
	}
	b.buttonFlow("账号: ",
		toolbarIndexSpecial(items, specialLogin),
		toolbarIndexSpecial(items, specialImport),
		toolbarIndexSpecial(items, specialBulk),
		manage,
	)
}

func (m Model) roomListLines(width int, rowRects []rect, indexes []int) []string {
	if len(m.status.Accounts) == 0 {
		return []string{
			dimStyle.Render("还没有账号。"),
			dimStyle.Render("用「登录」或「导入」添加。"),
		}
	}
	if len(indexes) == 0 {
		return []string{
			dimStyle.Render("没有匹配的账号。"),
			dimStyle.Render("按 / 搜索，f 筛选，v 排序。"),
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
		case index%2 == 1:
			lines = append(lines, rowAltStyle.Render(line))
		default:
			lines = append(lines, textStyle.Render(line))
		}
	}
	return lines
}

func (m Model) roomDetailInto(b *pageBuilder) {
	width := b.width
	items := m.toolbarItems()
	if m.searchFilteredEmpty() {
		b.add(panelTitleStyle.Render("没有匹配的账号"))
		b.add(dimStyle.Render("按 / 修改搜索，f 切换筛选，v 排序；底部可登录/导入。"))
		return
	}
	room, account, ok := m.selectedRoom()
	if !ok {
		// Account actions live in the bottom bar so they are not mixed with
		// the selected room's controls.
		b.add(panelTitleStyle.Render("添加账号"))
		b.add(dimStyle.Render("还没有账号：用底部「登录」「导入」或「批量」添加。"))
		b.blank()
		b.buttonFlow("", toolbarIndexSpecial(items, specialWebUI))
		return
	}
	title := truncate(fmt.Sprintf("房间 %d · %s", m.selectedIndex()+1, accountLabel(account)), width)
	titleLine := len(b.lines)
	b.add(panelTitleStyle.Render(title))
	b.hit(titleLine, 0, lipgloss.Width(title), hitAccountDashboard, 0)
	phase := room.Phase
	if phase == "" {
		phase = "stopped"
	}
	b.add(phaseStyle(phase).Render(truncate("状态: "+phaseLabel(phase), width)))

	start := toolbarIndexAction(items, actionStart)
	stop := toolbarIndexAction(items, actionStop)
	restart := toolbarIndexAction(items, actionRestart)
	startAll := toolbarIndexAction(items, actionStartAll)
	stopAll := toolbarIndexAction(items, actionStopAll)
	web := toolbarIndexSpecial(items, specialWebUI)
	debug := toolbarIndexSpecial(items, specialDebug)
	b.buttonFlow("房间: ", start, stop, restart, debug)
	b.buttonFlow("全部: ", startAll, stopAll, web)
	b.blank()

	if room.RoomID != 0 {
		b.add(textStyle.Render(truncate(fmt.Sprintf("房间号: %d", room.RoomID), width)))
	}
	profile := m.profileFor(account)
	if profile != nil {
		b.add(textStyle.Render(truncate(fmt.Sprintf("配置: %s（%s）", profile.Name, connectionLabel(profile.ConnectionMode)), width)))
		if profile.ConnectionMode == config.ConnectionTransfer {
			b.add(dimStyle.Render(truncate(fmt.Sprintf("中转: %s → %s", profile.LimboUpstream, profile.Upstream), width)))
		} else {
			b.add(dimStyle.Render(truncate("服务器: "+profile.Upstream, width)))
		}
	}
	if room.ConnectedAt != "" {
		b.add(textStyle.Render(truncate("在线: "+durationText(room.ConnectedAt), width)))
	}
	b.add(m.roomPlayerLine(width, room, profile))
	if room.UpstreamOnline {
		line := "服务器已连接"
		if room.UpstreamVersion != "" {
			line += " · " + room.UpstreamVersion
		}
		b.add(onlineStyle.Render(truncate(line, width)))
	} else if phase != "stopped" {
		b.add(warnStyle.Render("服务器未连接"))
	}
	if room.RetryAt != "" {
		b.add(warnStyle.Render(truncate("等待重连："+localTime(room.RetryAt), width)))
	}
	if room.LastError != "" {
		b.add(errorStyle.Render(truncate("上次错误："+room.LastError, width)))
	}
	if account.Email != "" {
		b.add(dimStyle.Render(truncate("账号: "+account.Email, width)))
	}
	if account.AuthMethod != "" {
		b.add(dimStyle.Render(truncate("登录方式: "+authMethodLabel(account.AuthMethod), width)))
	}
	b.add(dimStyle.Render(truncate("s 启动 · x 停止 · r 重连", width)))
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

func (m Model) logsInto(b *pageBuilder, height int) {
	width := b.width
	m.logControlsInto(b)
	combined := m.combinedLogs()

	visible := height - 2
	if visible < 1 {
		visible = 1
	}
	end := len(combined) - m.logOffset
	if end > len(combined) {
		end = len(combined)
	}
	if end < 0 {
		end = 0
	}
	start := end - visible
	if start < 0 {
		start = 0
	}
	if end == 0 {
		if strings.TrimSpace(string(m.logQuery)) != "" {
			b.add(dimStyle.Render("没有匹配的日志。"))
		} else {
			b.add(dimStyle.Render("暂无日志。"))
		}
	} else {
		for _, entry := range combined[start:end] {
			lineText := truncate(entry.time+" "+entry.text, width)
			switch {
			case entry.level == "error":
				lineText = errorStyle.Render(lineText)
			case entry.level == "warning":
				lineText = warnStyle.Render(lineText)
			case entry.level == "input":
				lineText = accentStyle.Render(lineText)
			case entry.level == "debug" || strings.HasPrefix(entry.text, "[DEBUG]"):
				lineText = dimStyle.Render(lineText)
			default:
				lineText = textStyle.Render(lineText)
			}
			b.add(lineText)
		}
	}
	for len(b.lines) < height-1 {
		b.add("")
	}
	if len(b.lines) < height {
		line := len(b.lines)
		switch {
		case m.logSearch:
			cursor := clampInt(m.logQueryCursor, 0, len(m.logQuery))
			input := " 搜索> " + string(m.logQuery[:cursor]) + "▌" + string(m.logQuery[cursor:])
			b.add(renderInputBar(input, width, true))
			b.hit(line, 0, width, hitLogSearch, 0)
		case m.consoleMode:
			cursor := clampInt(m.consoleCursor, 0, len(m.consoleInput))
			input := " 命令> " + string(m.consoleInput[:cursor]) + "▌" + string(m.consoleInput[cursor:])
			b.add(renderInputBar(input, width, true))
			b.hit(line, 0, width, hitConsoleInput, 0)
		default:
			hint := " 按 i 输入命令，/ 搜索日志，e 导出"
			hintStyle := dimStyle
			if m.consoleBusy {
				hint = " 命令执行中…"
				hintStyle = infoStyle
			}
			b.add(hintStyle.Render(truncate(hint, width)))
			b.hit(line, 0, width, hitConsoleInput, 0)
		}
	}
}

func (m Model) loginInto(b *pageBuilder) {
	width := b.width
	// On narrow screens a captcha challenge needs the room for the field and
	// buttons; the frame title already says what this screen is.
	compactCaptcha := m.loginCaptchaID != "" && width < 60
	if !compactCaptcha {
		b.add(textStyle.Render(truncate("邮箱/密码登录 · 互通版 / 网易手游 / 4399", width)))
		profileText := "配置方案: " + m.defaultProfileLabel()
		if m.loginReplace && m.loginReplaceIndex >= 0 && m.loginReplaceIndex < len(m.status.Accounts) {
			if profile := m.profileFor(m.status.Accounts[m.loginReplaceIndex]); profile != nil {
				profileText = "配置方案: " + profile.Name
			}
		}
		b.add(dimStyle.Render(truncate(profileText, width)))
		if m.loginReplace {
			b.add(warnStyle.Render(truncate(fmt.Sprintf("将替换房间 %d 的现有登录会话。", m.loginReplaceIndex+1), width)))
		}
		b.blank()
	}

	if m.loginCaptchaID != "" {
		b.add(warnStyle.Render(truncate(m.loginCaptchaNote, width)))
		if m.loginCaptchaPath != "" {
			line := len(b.lines)
			text := truncate("验证码: "+m.loginCaptchaPath, width)
			b.add(linkStyle.Render(text))
			b.hit(line, 0, lipgloss.Width(text), hitOpenLoginURL, 0)
			if width >= 60 {
				b.add(dimStyle.Render("Ctrl+O 打开验证码图片；输入后再次按 Enter。"))
			}
		}
	}
	rows := []fieldBoxRow{
		{label: "账号", value: displayFieldValue(m.loginEmail, m.loginEmailCursor, false, m.loginField == 0),
			active: m.loginField == 0, clickable: true, kind: hitLoginField, index: 0},
		{label: "密码", value: displayFieldValue(m.loginPassword, m.loginPasswordCursor, true, m.loginField == 1),
			active: m.loginField == 1, clickable: true, kind: hitLoginField, index: 1},
	}
	if m.loginCaptchaID != "" {
		rows = append(rows, fieldBoxRow{
			label: "验证码", value: displayFieldValue(m.loginCaptchaInput, m.loginCaptchaCursor, false, m.loginField == 4),
			active: m.loginField == 4, clickable: true, kind: hitLoginField, index: 4,
		})
	}
	m.addFieldBox(b, "登录信息", rows, width)

	b.customButtonFlow("平台: ",
		customButton{label: "互通版", kind: hitLoginPlatform, index: 0, active: m.loginPlatform == string(netease.AuthInterop)},
		customButton{label: "网易手游", kind: hitLoginPlatform, index: 1, active: m.loginPlatform == string(netease.AuthMobileNetEase)},
		customButton{label: "4399", kind: hitLoginPlatform, index: 2, active: m.loginPlatform == string(netease.AuthMobile4399)},
	)
	b.customButtonFlow("",
		customButton{label: "保存密码到本机", kind: hitLoginRemember, index: 3, active: m.loginRemember, focused: m.loginField == 3},
	)
	if width >= 60 {
		b.add(dimStyle.Render("Tab/↑↓ 切换字段，←→ 切换平台，Space 保存密码，Enter 登录"))
		b.blank()
	}
	b.customButtonFlow("",
		customButton{label: "登录", kind: hitModalSubmit},
		customButton{label: "取消", kind: hitModalCancel},
	)
	if m.modalBusy {
		b.add(m.busyLabel("正在登录并验证网易账号"))
	}
	if m.loginVerifyNote != "" || m.loginVerifyURL != "" {
		b.add(warnStyle.Render(truncate(m.loginVerifyNote, width)))
		if m.loginVerifyURL != "" {
			for _, part := range wrapDisplay(m.loginVerifyURL, maxInt(1, width-2)) {
				line := len(b.lines)
				text := truncate(part, width)
				b.add(linkStyle.Render(text))
				b.hit(line, 0, lipgloss.Width(text), hitOpenLoginURL, 0)
			}
			b.add(dimStyle.Render("Ctrl+O 打开验证链接；完成后再次按 Enter。"))
		}
	}
	if m.modalErr != "" {
		b.add(errorStyle.Render(truncate(m.modalErr, width)))
	}
}

func (m Model) restoreInto(b *pageBuilder) {
	width := b.width
	b.add(textStyle.Render("从 CubeDock 配置备份恢复方案和全局设置。"))
	b.add(dimStyle.Render("不恢复网易登录凭证；房间必须已停止。"))
	b.add(dimStyle.Render("相对路径按数据目录解析，例如 cubedock-backup-20260101-120000.json。"))
	if m.restoreRevision != m.status.ConfigRevision {
		b.add(warnStyle.Render("配置已在其他界面修改；恢复会被拒绝，请关闭后重新打开。"))
	}
	b.blank()
	m.addFieldBox(b, "备份文件", []fieldBoxRow{
		{label: "路径", value: displayFieldValue(m.input, m.inputCursor, false, true), active: true},
	}, width)
	b.blank()
	b.customButtonFlow("",
		customButton{label: "恢复", kind: hitModalSubmit},
		customButton{label: "取消", kind: hitModalCancel},
	)
	if m.modalBusy {
		b.add(m.busyLabel("正在读取并恢复配置"))
	}
	if m.modalErr != "" {
		b.add(errorStyle.Render(truncate(m.modalErr, width)))
	}
}

func (m Model) adminInto(b *pageBuilder) {
	width := b.width
	b.add(dimStyle.Render("密码只保存在本机 admin.json；仅允许本机初始化。"))
	m.addFieldBox(b, "管理员账号", []fieldBoxRow{
		{label: "账号", value: displayFieldValue(m.adminUser, m.adminUserCursor, false, m.adminField == 0),
			active: m.adminField == 0, clickable: true, kind: hitAdminField, index: 0},
		{label: "密码", value: displayFieldValue(m.adminPassword, m.adminPasswordCursor, true, m.adminField == 1),
			active: m.adminField == 1, clickable: true, kind: hitAdminField, index: 1},
		{label: "确认", value: displayFieldValue(m.adminConfirm, m.adminConfirmCursor, true, m.adminField == 2),
			active: m.adminField == 2, clickable: true, kind: hitAdminField, index: 2},
	}, width)
	b.add(dimStyle.Render("密码 8–72 字节；Tab/↑↓ 切换字段，Enter 创建。"))
	b.customButtonFlow("",
		customButton{label: "创建管理员", kind: hitModalSubmit},
		customButton{label: "稍后", kind: hitModalCancel},
	)
	if m.modalBusy {
		b.add(m.busyLabel("正在创建管理员"))
	}
	if m.modalErr != "" {
		b.add(errorStyle.Render(truncate(m.modalErr, width)))
	}
	if m.options.AdminName != "" && !m.options.SetupRequired {
		b.add(dimStyle.Render("当前管理员: " + m.options.AdminName))
	}
}

func (m Model) nameInto(b *pageBuilder) {
	width := b.width
	if m.nameRename {
		b.add(textStyle.Render(truncate("修改已有游戏名；网易限制 30 天内不能再次修改。", width)))
	} else {
		b.add(textStyle.Render(truncate("账号还没有游戏名；设置后网易会返回完整资料。", width)))
	}
	weight := nameWeight(string(m.nameInput))
	m.addFieldBox(b, fmt.Sprintf("游戏名 %.1f/12", weight), []fieldBoxRow{
		{label: "游戏名", value: displayFieldValue(m.nameInput, m.nameCursor, false, true), active: true},
	}, width)
	b.add(dimStyle.Render("3–12 个字符；中文按 1.5 个字符计算。"))
	b.customButtonFlow("随机: ",
		customButton{label: "中文名", kind: hitNameRandomCN},
		customButton{label: "英文+数字", kind: hitNameRandomEN},
		customButton{label: "换一批", kind: hitNameRefresh},
	)
	if width >= 55 {
		b.customButtonFlow("中文: ", nameCandidateButtons(m.nameSuggestionsCN, hitNameCandidateCN)...)
		b.customButtonFlow("英文: ", nameCandidateButtons(m.nameSuggestionsEN, hitNameCandidateEN)...)
	}
	if m.modalErr != "" {
		b.add(errorStyle.Render(truncate(m.modalErr, width)))
	}
	submitLabel, cancelLabel := "保存游戏名", "跳过"
	if m.nameRename {
		submitLabel, cancelLabel = "保存新游戏名", "取消"
	}
	b.customButtonFlow("",
		customButton{label: submitLabel, kind: hitModalSubmit},
		customButton{label: cancelLabel, kind: hitModalCancel},
	)
	if m.modalBusy {
		b.add(m.busyLabel("正在保存游戏名"))
	}
}

func (m Model) accountActionsInto(b *pageBuilder) {
	width := b.width
	index := m.accountActionIndex
	accounts := m.status.Accounts
	if index < 0 || index >= len(accounts) {
		b.add(errorStyle.Render("没有选中账号。"))
		return
	}
	account := accounts[index]
	b.add(sectionTitle(fmt.Sprintf("房间 %d · %s", index+1, accountLabel(account))))
	if profile := m.profileFor(account); profile != nil {
		b.add(textStyle.Render(truncate(fmt.Sprintf("配置方案: %s（%s）", profile.Name, connectionLabel(profile.ConnectionMode)), width)))
	}
	if account.Email != "" {
		b.add(dimStyle.Render(truncate("账号: "+account.Email, width)))
	}
	if account.AuthMethod != "" {
		b.add(dimStyle.Render(truncate("登录方式: "+authMethodLabel(account.AuthMethod), width)))
	}
	if !m.accountActionStillValid() {
		b.add(warnStyle.Render("账号列表已在 WebUI/其他界面变化；请关闭后重新打开。"))
	}
	b.blank()
	buttonAt := func(position int) customButton {
		if position < 0 || position >= len(accountActionSpecs) {
			return customButton{}
		}
		button := accountActionSpecs[position]
		button.active = position == m.accountActionCursor
		if button.kind == hitAccountName {
			button.label = accountNameActionLabel(account)
		}
		if button.kind == hitAccountDelete && m.accountDeleteArmed {
			button.label = "确认删除"
		}
		return button
	}
	b.customButtonFlow("资料: ", buttonAt(0), buttonAt(1), buttonAt(2))
	b.customButtonFlow("方案: ", buttonAt(3))
	b.customButtonFlow("互动: ", buttonAt(4), buttonAt(5), buttonAt(6))
	b.customButtonFlow("其他: ", buttonAt(7), buttonAt(8), buttonAt(9))
	if m.accountDeleteArmed {
		b.add(warnStyle.Render("再次点击「确认删除」将移除账号；Esc 关闭。"))
	}
	if m.modalBusy {
		b.add(m.busyLabel("正在处理账号"))
	}
	if m.modalErr != "" {
		b.add(errorStyle.Render(truncate(m.modalErr, width)))
	}
	b.add(dimStyle.Render("↑↓ 选择 Enter 执行 Esc 关闭；删除需要操作两次。"))
}

func (m Model) assignProfileInto(b *pageBuilder) {
	width := b.width
	profiles := m.status.Config.Profiles
	b.add(sectionTitle("切换配置方案"))
	b.add(dimStyle.Render("点击方案立即应用；房间运行中不能切换。"))
	b.blank()
	if len(profiles) == 0 {
		b.add(dimStyle.Render("还没有配置方案。"))
		return
	}
	current := ""
	if m.accountActionIndex >= 0 && m.accountActionIndex < len(m.status.Accounts) {
		current = m.status.Accounts[m.accountActionIndex].ProfileID
	}
	if current == "" {
		current = m.status.Config.DefaultProfileID()
	}
	for position, profile := range profiles {
		marker := "  "
		if position == m.assignProfileCursor {
			marker = "▌ "
		}
		lineText := fmt.Sprintf("%s● %s（%s）· %s", marker, profile.Name, profile.ID, connectionLabel(profile.ConnectionMode))
		if profile.ID == current {
			lineText += "  ●当前"
		}
		line := len(b.lines)
		lineText = padRightPlain(truncate(lineText, width), width)
		style := textStyle
		switch {
		case position == m.assignProfileCursor:
			style = selectedRowStyle
		case b.hovered(line, 0, width):
			style = rowHoverStyle
		case position%2 == 1:
			style = rowAltStyle
		}
		b.add(style.Render(lineText))
		b.hit(line, 0, width, hitAssignProfile, position)
	}
	if m.modalErr != "" {
		b.add(errorStyle.Render(truncate(m.modalErr, width)))
	}
	b.blank()
	b.customButtonFlow("", customButton{label: "取消", kind: hitModalCancel})
}

func (m Model) playersInto(b *pageBuilder) {
	width := b.width
	b.add(sectionTitle("在线玩家（代理模式）"))
	b.add(dimStyle.Render("↑↓ 选择；x 踢出；t 转服；r 刷新。"))
	b.blank()
	if m.playersBusy {
		b.add(m.busyLabel("正在刷新玩家"))
	}
	if len(m.players) == 0 && !m.playersBusy {
		b.add(dimStyle.Render("当前没有在线玩家。转服模式下玩家由目标服务器管理。"))
	}
	for index, player := range m.players {
		line := len(b.lines)
		lineText := padRightPlain(truncate(playerText(player), width), width)
		style := textStyle
		switch {
		case index == m.playersCursor:
			style = selectedRowStyle
		case b.hovered(line, 0, width):
			style = rowHoverStyle
		case index%2 == 1:
			style = rowAltStyle
		}
		b.add(style.Render(lineText))
		b.hit(line, 0, width, hitPlayerSelect, index)
	}
	if m.playersNotice != "" {
		b.add(dimStyle.Render(truncate(m.playersNotice, width)))
	}
	if m.playersErr != "" {
		b.add(errorStyle.Render(truncate(m.playersErr, width)))
	}
	b.blank()
	b.customButtonFlow("",
		customButton{label: "刷新", kind: hitPlayerRefresh},
		customButton{label: "踢出", kind: hitPlayerKick},
		customButton{label: "转服", kind: hitPlayerTransfer},
		customButton{label: "关闭", kind: hitModalCancel},
	)
}

func (m Model) membersInto(b *pageBuilder) {
	width := b.width
	b.add(sectionTitle(fmt.Sprintf("房间 %d 成员", m.accountActionIndex+1)))
	b.add(dimStyle.Render("↑↓ 选择；x 踢出；r 刷新。"))
	b.blank()
	if m.membersBusy {
		b.add(m.busyLabel("正在刷新房间成员"))
	}
	if len(m.members) == 0 && !m.membersBusy {
		b.add(dimStyle.Render("当前没有房间成员。"))
	}
	for index, member := range m.members {
		state := "未连接"
		if member.Connected {
			state = "已连接"
		}
		if member.InWorld {
			state += " · 已进服"
		}
		text := memberDisplayName(member) + " · " + state
		if member.RoomPlayerID != 0 {
			text += fmt.Sprintf(" · 成员 %d", member.RoomPlayerID)
		}
		if member.JoinedAt != "" {
			text += " · 加入 " + localTime(member.JoinedAt)
		}
		line := len(b.lines)
		lineText := padRightPlain(truncate(text, width), width)
		style := textStyle
		switch {
		case index == m.membersCursor:
			style = selectedRowStyle
		case b.hovered(line, 0, width):
			style = rowHoverStyle
		case index%2 == 1:
			style = rowAltStyle
		}
		b.add(style.Render(lineText))
		b.hit(line, 0, width, hitMemberSelect, index)
	}
	if m.membersNotice != "" {
		b.add(dimStyle.Render(truncate(m.membersNotice, width)))
	}
	if m.membersErr != "" {
		b.add(errorStyle.Render(truncate(m.membersErr, width)))
	}
	b.blank()
	b.customButtonFlow("",
		customButton{label: "刷新", kind: hitMemberRefresh},
		customButton{label: "踢出", kind: hitMemberKick},
		customButton{label: "关闭", kind: hitModalCancel},
	)
}

func (m Model) friendsInto(b *pageBuilder) {
	width := b.width
	b.add(sectionTitle(fmt.Sprintf("房间 %d 好友申请", m.accountActionIndex+1)))
	b.add(dimStyle.Render("↑↓ 选择；a 同意；r 刷新。"))
	b.blank()
	if m.friendsBusy {
		b.add(m.busyLabel("正在读取好友申请"))
	}
	if len(m.friendRequests) == 0 && !m.friendsBusy {
		b.add(dimStyle.Render("当前没有待处理的好友申请。"))
	}
	for index, request := range m.friendRequests {
		text := request.Name
		if request.UID != 0 {
			text += fmt.Sprintf("（UID %d）", request.UID)
		}
		if request.ApplyTime != "" {
			text += " · " + request.ApplyTime
		}
		line := len(b.lines)
		lineText := padRightPlain(truncate(text, width), width)
		style := textStyle
		switch {
		case index == m.friendsCursor:
			style = selectedRowStyle
		case b.hovered(line, 0, width):
			style = rowHoverStyle
		case index%2 == 1:
			style = rowAltStyle
		}
		b.add(style.Render(lineText))
		b.hit(line, 0, width, hitFriendSelect, index)
	}
	if m.friendsNotice != "" {
		b.add(dimStyle.Render(truncate(m.friendsNotice, width)))
	}
	if m.friendsErr != "" {
		b.add(errorStyle.Render(truncate(m.friendsErr, width)))
	}
	b.blank()
	b.customButtonFlow("",
		customButton{label: "刷新", kind: hitFriendRefresh},
		customButton{label: "同意", kind: hitFriendAccept},
		customButton{label: "关闭", kind: hitModalCancel},
	)
}

func (m Model) transferInto(b *pageBuilder) {
	width := b.width
	b.add(sectionTitle("转服玩家"))
	b.add(textStyle.Render(truncate("玩家: "+m.transferTarget, width)))
	b.add(dimStyle.Render("实验功能；目标服务器仍需支持网易协议。"))
	b.blank()
	m.addFieldBox(b, "目标服务器", []fieldBoxRow{
		{label: "地址", value: displayFieldValue(m.transferAddress, m.transferCursor, false, true), active: true},
	}, width)
	b.add(dimStyle.Render("例如 192.168.1.2:19132；Enter 发送，Esc 返回。"))
	if m.modalErr != "" {
		b.add(errorStyle.Render(truncate(m.modalErr, width)))
	}
	b.blank()
	b.customButtonFlow("",
		customButton{label: "发送", kind: hitTransferSend},
		customButton{label: "取消", kind: hitModalCancel},
	)
}

func (m Model) packsInto(b *pageBuilder) {
	width := b.width
	b.add(sectionTitle("资源包 · " + m.packsProfileName))
	b.add(dimStyle.Render(truncate("目录: "+m.packs.Dir, width)))
	b.add(dimStyle.Render("↑↓ 选择；r 刷新列表；p 从服务器拉取。"))
	b.blank()
	if m.packsBusy {
		b.add(m.busyLabel("正在读取或拉取资源包"))
	}
	if len(m.packs.Packs) == 0 && !m.packsBusy {
		b.add(dimStyle.Render("暂无资源包。转服开房时可自动拉取，也可以按 p 手动拉取。"))
	}
	for index, pack := range m.packs.Packs {
		name := pack.Name
		if name == "" {
			name = pack.File
		}
		text := name
		if pack.Version != "" {
			text += " · " + pack.Version
		}
		if pack.Size > 0 {
			text += " · " + formatBytes(pack.Size)
		}
		if pack.Error != "" {
			text += " · ⚠ " + pack.Error
		}
		line := len(b.lines)
		lineText := padRightPlain(truncate(text, width), width)
		style := textStyle
		switch {
		case index == m.packsCursor:
			style = selectedRowStyle
		case pack.Error != "":
			style = errorStyle
		case b.hovered(line, 0, width):
			style = rowHoverStyle
		case index%2 == 1:
			style = rowAltStyle
		}
		b.add(style.Render(lineText))
		b.hit(line, 0, width, hitPackSelect, index)
	}
	if m.packsNotice != "" {
		b.add(dimStyle.Render(truncate(m.packsNotice, width)))
	}
	if m.packsErr != "" {
		b.add(errorStyle.Render(truncate(m.packsErr, width)))
	}
	b.blank()
	b.customButtonFlow("",
		customButton{label: "刷新列表", kind: hitPacksRefresh},
		customButton{label: "拉取", kind: hitPacksPull},
		customButton{label: "关闭", kind: hitModalCancel},
	)
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGT"[exp])
}

func (m Model) importInto(b *pageBuilder, height int) {
	width := b.width
	b.add(textStyle.Render("粘贴 sauth/session JSON（互通版 / 网易手游 / 4399）："))
	b.add(dimStyle.Render(truncate("配置方案: "+m.defaultProfileLabel(), width)))
	b.add(dimStyle.Render("Enter 导入，Esc 取消；也可以点「登录」用邮箱/密码登录。"))
	b.customButtonFlow("",
		customButton{label: "导入", kind: hitModalSubmit},
		customButton{label: "取消", kind: hitModalCancel},
	)
	b.blank()
	if m.modalBusy {
		b.add(m.busyLabel("正在验证会话并添加账号"))
	}
	if m.modalErr != "" {
		b.add(errorStyle.Render(truncate(m.modalErr, width)))
	}
	inputHeight := height - len(b.lines) - 4
	if inputHeight < 1 {
		inputHeight = 1
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
	wrapped := wrapDisplay(text, maxInt(1, width-4))
	if len(wrapped) > inputHeight {
		wrapped = wrapped[len(wrapped)-inputHeight:]
		if len(wrapped) > 0 {
			wrapped[0] = "…" + wrapped[0]
		}
	}
	m.addTextAreaBox(b, "sauth / session JSON", wrapped, width, true)
	b.add(dimStyle.Render("支持多行粘贴；内容不会写入日志。"))
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

func (m Model) helpInto(b *pageBuilder) {
	width := b.width
	b.add(sectionTitle("终端操作与说明"))
	b.add(dimStyle.Render("TUI 与 WebUI 同时运行，操作同一套房间。"))
	if m.options.LocalURL != "" {
		line := len(b.lines)
		text := truncate("WebUI: "+m.options.LocalURL, width)
		b.add(textStyle.Render(text))
		b.hit(line, 0, lipgloss.Width(text), hitOpenWebUI, 0)
	}
	b.add(textStyle.Render("4399 图形验证码、复杂配置在哪里？→ 仍需要 WebUI"))
	b.add(textStyle.Render("房间启动失败？→ 检查服务器是否支持网易 RakNet v8、地址和会话"))
	b.add(textStyle.Render("鼠标无效？→ 终端需支持鼠标；Windows 推荐 Windows Terminal"))
	b.add(textStyle.Render("窗口太小？→ 调大窗口/字号；Termux 建议横屏"))
	b.blank()
	b.add(panelTitleStyle.Render("按键"))
	b.add(textStyle.Render("1–7 直接切页    Tab/Shift+Tab 切换焦点"))
	b.add(textStyle.Render("↑↓ 选择/滚动    ←→ 切换页面或按钮"))
	b.add(textStyle.Render("Enter 执行     鼠标点击导航/房间/按钮"))
	b.add(textStyle.Render("s/x/r 启停/重连  S/X 全部启停  d 调试"))
	b.add(textStyle.Render("账号页：m 管理  / 搜索  f 筛选  v 排序"))
	b.add(textStyle.Render("日志页：i 命令  f 筛选    o WebUI  R 诊断  q 退出"))
	if m.options.SetupRequired {
		b.add(warnStyle.Render("管理员尚未初始化：打开 WebUI 创建。"))
	}
}

func (m Model) homeInto(b *pageBuilder) {
	width := b.width
	phase := m.status.Phase
	if phase == "" {
		phase = "stopped"
	}
	b.add(phaseStyle(phase).Render(truncate("● "+phaseLabel(phase)+"  ·  "+
		fmt.Sprintf("%d 个房间  ·  %d 位玩家", activeRoomCount(m.status), m.status.Players), width)))
	if m.options.LocalURL != "" {
		line := len(b.lines)
		text := truncate("WebUI: "+m.options.LocalURL, width)
		b.add(textStyle.Render(text))
		b.hit(line, 0, lipgloss.Width(text), hitOpenWebUI, 0)
	}
	admin := m.options.AdminName
	if admin == "" {
		admin = "未设置"
	}
	state := "已初始化"
	if m.options.SetupRequired {
		state = "尚未初始化，请用 WebUI 创建"
	}
	b.add(dimStyle.Render(truncate("管理员: "+admin+"（"+state+"）", width)))
	b.blank()

	items := m.toolbarItems()
	startAll := toolbarIndexAction(items, actionStartAll)
	stopAll := toolbarIndexAction(items, actionStopAll)
	web := toolbarIndexSpecial(items, specialWebUI)
	b.buttonFlow("", startAll, stopAll, web)
	b.blank()

	if len(m.status.Accounts) == 0 {
		b.add(dimStyle.Render("还没有账号。请到「账号与房间」页登录或导入。"))
		return
	}
	b.add(sectionTitle("运行中的房间"))
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
		roomText = truncate(roomText, width)
		line := len(b.lines)
		text := phaseStyle(room.Phase).Render(roomText)
		if b.hovered(line, 0, lipgloss.Width(roomText)) {
			text = rowHoverStyle.Render(roomText)
		}
		b.add(text)
		b.hit(line, 0, lipgloss.Width(roomText), hitJumpRoom, index)
	}
	if active == 0 {
		b.add(dimStyle.Render("当前没有运行中的房间。"))
	}
	b.blank()
	b.add(sectionTitle("最近事件"))
	events := m.status.Events
	if len(events) > 6 {
		events = events[len(events)-6:]
	}
	if len(events) == 0 {
		b.add(dimStyle.Render("暂无事件。"))
	}
	for _, event := range events {
		b.add(textStyle.Render(truncate(
			fmt.Sprintf("%s [%s] %s", event.Time, event.Source, event.Message), width)))
	}
}

func (m Model) profilesInto(b *pageBuilder) {
	width := b.width
	profiles := m.status.Config.Profiles
	b.add(dimStyle.Render(truncate(fmt.Sprintf("共 %d 个方案 · 每个账号选择一个 · 房间运行中不能编辑", len(profiles)), width)))
	b.blank()
	if len(profiles) == 0 {
		b.add(dimStyle.Render("还没有配置方案，点击下方「新增」创建。"))
		return
	}
	for index, profile := range profiles {
		count := m.accountsUsingProfile(profile.ID)
		marker := "  "
		if index == m.profileIndex {
			marker = "▌ "
		}
		lineText := fmt.Sprintf("%s● %s（%s）· %s · %d 个账号",
			marker, profile.Name, profile.ID, connectionLabel(profile.ConnectionMode), count)
		lineText = padRightPlain(truncate(lineText, width), width)
		line := len(b.lines)
		text := textStyle.Render(lineText)
		switch {
		case index == m.profileIndex:
			text = selectedRowStyle.Render(lineText)
		case b.hovered(line, 0, width):
			text = rowHoverStyle.Render(lineText)
		case index%2 == 1:
			text = rowAltStyle.Render(lineText)
		}
		b.add(text)
		b.hit(line, 0, width, hitProfileSelect, index)
		b.add(dimStyle.Render(truncate("  "+profile.Upstream, width)))
		b.hit(line+1, 0, width, hitProfileSelect, index)
	}
	b.blank()
	items := m.toolbarItems()
	b.buttonFlow("",
		toolbarIndexSpecial(items, specialProfileEdit),
		toolbarIndexSpecial(items, specialProfileNew),
		toolbarIndexSpecial(items, specialProfileDup),
		toolbarIndexSpecial(items, specialProfileDel),
		toolbarIndexSpecial(items, specialPacks),
		toolbarIndexSpecial(items, specialWebUI),
	)
	b.add(dimStyle.Render("Enter 编辑当前方案；点击方案行选择；资源包仅对转服方案生效。"))
}

func (m Model) diagnosticsInto(b *pageBuilder) {
	width := b.width
	items := m.toolbarItems()
	b.buttonFlow("",
		toolbarIndexSpecial(items, specialDiagnostics),
		toolbarIndexSpecial(items, specialWebUI),
	)
	b.blank()
	if m.diagnosticsBusy {
		b.add(m.busyLabel("正在运行诊断"))
		b.add(dimStyle.Render("诊断不会登录网易账号。"))
		return
	}
	if len(m.diagnostics) == 0 {
		b.add(panelTitleStyle.Render("运行诊断"))
		b.add(dimStyle.Render("按 Enter 或点击「运行诊断」检查配置、服务器、中转房间和资源包。"))
		b.add(dimStyle.Render("诊断只读，不会登录网易账号。"))
		return
	}
	counts := map[string]int{}
	for _, check := range m.diagnostics {
		counts[check.Status]++
	}
	b.add(panelTitleStyle.Render(fmt.Sprintf("诊断 · 通过 %d · 注意 %d · 失败 %d · 跳过 %d",
		counts[core.DiagnosticOK], counts[core.DiagnosticWarn],
		counts[core.DiagnosticFail], counts[core.DiagnosticSkip])))
	if !m.diagnosticsAt.IsZero() {
		b.add(dimStyle.Render("运行时间: " + m.diagnosticsAt.Local().Format("15:04:05")))
	}
	b.blank()
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
		b.add(style.Render(truncate(symbol+" "+check.Label+" · "+check.Category, width)))
		if check.Detail != "" {
			b.add(dimStyle.Render(truncate("  "+check.Detail, width)))
		}
		if check.Hint != "" {
			b.add(warnStyle.Render(truncate("  "+check.Hint, width)))
		}
	}
}

func (m Model) settingsInto(b *pageBuilder) {
	width := b.width
	cfg := m.status.Config
	dataDir := "—"
	if m.manager != nil {
		dataDir = m.manager.Dir()
	}
	items := m.toolbarItems()
	b.buttonFlow("",
		toolbarIndexSpecial(items, specialSettingsEdit),
		toolbarIndexSpecial(items, specialBackup),
		toolbarIndexSpecial(items, specialRestore),
		toolbarIndexSpecial(items, specialAdminPassword),
		toolbarIndexSpecial(items, specialWebUI),
	)
	b.blank()
	b.add(dimStyle.Render("点击「编辑」修改 limbo 路径、自动启动和保活；Debug 用 d 切换。"))
	b.blank()
	b.add(textStyle.Render(truncate(fmt.Sprintf("控制台端口: %d", cfg.WebPort), width)))
	b.add(textStyle.Render(truncate("数据目录: "+dataDir, width)))
	b.add(textStyle.Render(truncate("自动启动全部房间: "+boolLabel(cfg.AutoStart), width)))
	b.add(textStyle.Render(truncate(fmt.Sprintf("空闲账号保活: %s（%d 分钟）", boolLabel(cfg.KeepAlive), cfg.KeepAliveMinutes), width)))
	b.add(textStyle.Render(truncate("Debug 日志: "+boolLabel(cfg.Debug), width)))
	if cfg.LimboPath != "" {
		b.add(textStyle.Render(truncate("limbo 路径: "+cfg.LimboPath, width)))
	} else {
		b.add(dimStyle.Render("limbo 路径: 自动查找主程序同目录"))
	}
	admin := m.options.AdminName
	if admin == "" {
		admin = "未设置"
	}
	b.add(textStyle.Render(truncate("管理员: "+admin, width)))
	if m.options.SetupRequired {
		b.add(warnStyle.Render("管理员尚未初始化：点击下方按钮或按 a 在 TUI 创建，也可打开 WebUI 创建。"))
		b.customButtonFlow("", customButton{label: "初始化管理员", kind: hitCreateAdmin})
	}
	if m.lastBackup != "" {
		b.add(dimStyle.Render(truncate("最近备份: "+m.lastBackup, width)))
	}
	b.add(dimStyle.Render("配置备份为 JSON；恢复只应用配置，不恢复网易登录凭证。"))
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
