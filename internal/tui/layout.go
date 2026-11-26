package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// rect is a terminal cell rectangle. X/Y are zero-based; mouse coordinates from
// Bubble Tea are zero-based too.
type rect struct {
	X, Y, W, H int
}

func (r rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

type pageKind int

const (
	pageHome pageKind = iota
	pageAccounts
	pageProfiles
	pageLogs
	pageDiagnostics
	pageSettings
	pageFAQ
)

var pageOrder = []pageKind{
	pageHome, pageAccounts, pageProfiles, pageLogs,
	pageDiagnostics, pageSettings, pageFAQ,
}

func pageLabel(page pageKind) string {
	switch page {
	case pageHome:
		return "主页"
	case pageAccounts:
		return "账号与房间"
	case pageProfiles:
		return "配置方案"
	case pageLogs:
		return "控制台"
	case pageDiagnostics:
		return "诊断"
	case pageSettings:
		return "全局设置"
	case pageFAQ:
		return "常见问题"
	default:
		return "CubeDock"
	}
}

func pageShortLabel(page pageKind) string {
	switch page {
	case pageHome:
		return "主页"
	case pageAccounts:
		return "账号"
	case pageProfiles:
		return "配置"
	case pageLogs:
		return "日志"
	case pageDiagnostics:
		return "诊断"
	case pageSettings:
		return "设置"
	case pageFAQ:
		return "帮助"
	default:
		return "页面"
	}
}

type focusArea int

const (
	focusNav focusArea = iota
	focusActions
	focusContent
)

type toolbarItem struct {
	Label   string
	Action  actionKind
	Special string // webui / debug / diagnostics
}

const (
	specialWebUI         = "webui"
	specialDebug         = "debug"
	specialDiagnostics   = "diagnostics"
	specialImport        = "import"
	specialLogin         = "login"
	specialBackup        = "backup"
	specialRestore       = "restore"
	specialProfileEdit   = "profile-edit"
	specialProfileNew    = "profile-new"
	specialProfileDup    = "profile-duplicate"
	specialProfileDel    = "profile-delete"
	specialProfileSave   = "profile-save"
	specialProfileCancel = "profile-cancel"
)

var accountToolbar = []toolbarItem{
	{Label: "登录", Special: specialLogin},
	{Label: "导入", Special: specialImport},
	{Label: "启动", Action: actionStart},
	{Label: "停止", Action: actionStop},
	{Label: "重连", Action: actionRestart},
	{Label: "全部启动", Action: actionStartAll},
	{Label: "全部停止", Action: actionStopAll},
	{Label: "WebUI", Special: specialWebUI},
	{Label: "调试", Special: specialDebug},
}

var homeToolbar = []toolbarItem{
	{Label: "启动全部", Action: actionStartAll},
	{Label: "停止全部", Action: actionStopAll},
	{Label: "WebUI", Special: specialWebUI},
}

var diagnosticsToolbar = []toolbarItem{
	{Label: "运行诊断", Special: specialDiagnostics},
	{Label: "WebUI", Special: specialWebUI},
}

var profileToolbar = []toolbarItem{
	{Label: "编辑", Special: specialProfileEdit},
	{Label: "新增", Special: specialProfileNew},
	{Label: "复制", Special: specialProfileDup},
	{Label: "删除", Special: specialProfileDel},
	{Label: "WebUI", Special: specialWebUI},
}

var profileEditToolbar = []toolbarItem{
	{Label: "保存", Special: specialProfileSave},
	{Label: "取消", Special: specialProfileCancel},
	{Label: "WebUI", Special: specialWebUI},
}

var settingsToolbar = []toolbarItem{
	{Label: "备份", Special: specialBackup},
	{Label: "恢复", Special: specialRestore},
	{Label: "WebUI", Special: specialWebUI},
}

// screenLayout is computed from the same state as View(), so mouse hitboxes
// stay aligned with what the user sees.
type screenLayout struct {
	width, height    int
	header, footer   string
	headerH, footerH int
	bodyY, bodyH     int

	sidebar     bool
	navWidth    int
	navLabels   []string
	navRects    []rect
	navArea     rect
	contentArea rect

	toolbarY     int
	toolbar      []toolbarItem
	toolbarRects []rect
	toolbarRows  int

	frameY            int
	framed            bool
	frame             rect
	content           rect
	roomRows          []rect
	roomRowIndex      []int
	profileFields     []rect
	profileFieldIndex []int
	listArea          rect
	logArea           rect
}

func (m Model) layoutInfo() screenLayout {
	width := maxInt(1, m.width)
	height := maxInt(1, m.height)
	info := screenLayout{width: width, height: height}
	info.header = m.headerView()
	info.footer = m.footerView()
	info.headerH = lipgloss.Height(info.header)
	info.footerH = lipgloss.Height(info.footer)
	info.bodyY = info.headerH
	info.bodyH = height - info.headerH - info.footerH
	if info.bodyH < 1 {
		info.bodyH = 1
	}

	info.sidebar = width >= 100 && info.bodyH >= 10
	if info.sidebar {
		info.navWidth = clampInt(width/5, 18, 26)
		info.navArea = rect{X: 0, Y: info.bodyY, W: info.navWidth, H: info.bodyH}
		info.contentArea = rect{X: info.navWidth + 1, Y: info.bodyY, W: width - info.navWidth - 1, H: info.bodyH}
		for index, page := range pageOrder {
			info.navLabels = append(info.navLabels, pageLabel(page))
			info.navRects = append(info.navRects, rect{
				X: 0, Y: info.bodyY + index, W: info.navWidth, H: 1,
			})
		}
	} else {
		info.navLabels, info.navRects, info.navArea = m.topNavLayout(width, info.bodyY)
		info.contentArea = rect{X: 0, Y: info.bodyY + info.navArea.H, W: width, H: info.bodyH - info.navArea.H}
	}

	info.toolbar = m.toolbarItems()
	if m.modal != modalNone {
		info.toolbar = nil
	}
	if len(info.toolbar) > 0 && info.contentArea.H >= 3 {
		info.toolbarY = info.contentArea.Y
		info.toolbarRects, info.toolbarRows = layoutToolbar(info.toolbar, info.contentArea.X, info.toolbarY, info.contentArea.W)
	}

	info.frameY = info.contentArea.Y
	if len(info.toolbarRects) > 0 {
		info.frameY += maxInt(1, info.toolbarRows)
	}
	frameH := info.contentArea.Y + info.contentArea.H - info.frameY
	if frameH < 1 {
		frameH = 1
	}
	info.framed = info.contentArea.W >= 40 && frameH >= 4
	info.frame = rect{X: info.contentArea.X, Y: info.frameY, W: info.contentArea.W, H: frameH}
	if info.framed {
		info.content = rect{X: info.frame.X + 1, Y: info.frame.Y + 1, W: maxInt(1, info.frame.W-2), H: maxInt(0, info.frame.H-2)}
	} else {
		info.content = info.frame
	}
	info.listArea = info.content

	if m.page == pageAccounts && info.content.W > 0 && info.content.H > 0 {
		if roomsWide(info.content.W) {
			leftWidth := clampInt(info.content.W/3, 24, 38)
			rightWidth := info.content.W - leftWidth - 3
			if rightWidth >= 24 {
				info.listArea = rect{X: info.content.X, Y: info.content.Y, W: leftWidth, H: info.content.H}
				info.roomRows, info.roomRowIndex = m.roomRowHitboxes(info.content.X, info.content.Y, leftWidth, info.content.H)
			}
		} else {
			listHeight := compactListHeight(len(m.status.Accounts), info.content.H)
			info.listArea = rect{X: info.content.X, Y: info.content.Y, W: info.content.W, H: listHeight}
			info.roomRows, info.roomRowIndex = m.roomRowHitboxes(info.content.X, info.content.Y, info.content.W, listHeight)
		}
	}
	if m.page == pageLogs {
		info.logArea = info.content
	}
	if m.page == pageProfiles && m.profileEditing && info.content.W > 0 && info.content.H > 0 {
		_, indices := m.profileFormLines(info.content.W)
		offset := m.contentOffset
		maxOffset := len(indices) - info.content.H
		if maxOffset < 0 {
			maxOffset = 0
		}
		offset = clampInt(offset, 0, maxOffset)
		for field, lineIndex := range indices {
			if lineIndex < offset || lineIndex >= offset+info.content.H {
				continue
			}
			info.profileFields = append(info.profileFields, rect{
				X: info.content.X, Y: info.content.Y + lineIndex - offset, W: info.content.W, H: 1,
			})
			info.profileFieldIndex = append(info.profileFieldIndex, field)
		}
	}
	return info
}

func (m Model) topNavLayout(width, y int) ([]string, []rect, rect) {
	full := make([]string, 0, len(pageOrder))
	short := make([]string, 0, len(pageOrder))
	for _, page := range pageOrder {
		full = append(full, pageLabel(page))
		short = append(short, pageShortLabel(page))
	}
	if labels, rects, ok := layoutNavRows(full, width, y, 2); ok {
		return labels, rects, rect{X: 0, Y: y, W: width, H: navRows(rects, y)}
	}
	if labels, rects, ok := layoutNavRows(short, width, y, 2); ok {
		return labels, rects, rect{X: 0, Y: y, W: width, H: navRows(rects, y)}
	}
	labels, rects, _ := layoutNavRows(short, width, y, 3)
	return labels, rects, rect{X: 0, Y: y, W: width, H: navRows(rects, y)}
}

func layoutNavRows(labels []string, width, y, maxRows int) ([]string, []rect, bool) {
	count := len(labels)
	for rows := 1; rows <= maxRows; rows++ {
		perRow := (count + rows - 1) / rows
		rects := make([]rect, 0, count)
		row, x := 0, 0
		ok := true
		for index, label := range labels {
			if index == perRow {
				row++
				x = 0
			}
			textWidth := lipgloss.Width(" " + label + " ")
			if x+textWidth > width {
				ok = false
				break
			}
			rects = append(rects, rect{X: x, Y: y + row, W: textWidth, H: 1})
			x += textWidth + 1
		}
		if ok {
			return labels, rects, true
		}
	}
	return labels, nil, false
}

func navRows(rects []rect, y int) int {
	if len(rects) == 0 {
		return 1
	}
	rows := rects[len(rects)-1].Y - y + 1
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m Model) toolbarItems() []toolbarItem {
	switch m.page {
	case pageAccounts:
		return accountToolbar
	case pageHome:
		return homeToolbar
	case pageDiagnostics:
		return diagnosticsToolbar
	case pageSettings:
		return settingsToolbar
	case pageProfiles:
		if m.profileEditing {
			return profileEditToolbar
		}
		return profileToolbar
	default:
		return nil
	}
}

func layoutToolbar(items []toolbarItem, startX, startY, width int) ([]rect, int) {
	rects := make([]rect, 0, len(items))
	x, y := startX, startY
	rows := 1
	for index, item := range items {
		buttonWidth := lipgloss.Width(" " + item.Label + " ")
		if x > startX && x+buttonWidth > startX+width {
			y++
			x = startX
			rows++
		}
		rects = append(rects, rect{X: x, Y: y, W: buttonWidth, H: 1})
		x += buttonWidth
		if index < len(items)-1 {
			x++
		}
	}
	return rects, rows
}

func (m *Model) clampActionIndex() {
	items := m.toolbarItems()
	if len(items) == 0 {
		m.actionIndex = 0
		return
	}
	if m.actionIndex < 0 {
		m.actionIndex = 0
	}
	if m.actionIndex >= len(items) {
		m.actionIndex = len(items) - 1
	}
}

func (m Model) focusAreas() []focusArea {
	if m.modal != modalNone {
		return []focusArea{focusContent}
	}
	areas := []focusArea{focusNav}
	if len(m.toolbarItems()) > 0 {
		areas = append(areas, focusActions)
	}
	areas = append(areas, focusContent)
	return areas
}

func (m *Model) cycleFocus(reverse bool) {
	areas := m.focusAreas()
	if len(areas) == 0 {
		return
	}
	index := 0
	for position, area := range areas {
		if area == m.focus {
			index = position
			break
		}
	}
	if reverse {
		index = (index - 1 + len(areas)) % len(areas)
	} else {
		index = (index + 1) % len(areas)
	}
	m.focus = areas[index]
}

func (m *Model) normalizeFocus() {
	areas := m.focusAreas()
	for _, area := range areas {
		if area == m.focus {
			return
		}
	}
	if len(areas) > 0 {
		m.focus = areas[0]
	}
}

func (m Model) pageIndex() int {
	for index, page := range pageOrder {
		if page == m.page {
			return index
		}
	}
	return 0
}

func (m *Model) cyclePage(step int) {
	index := m.pageIndex()
	count := len(pageOrder)
	index = (index + step%count + count) % count
	m.setPage(pageOrder[index])
}

func (m *Model) setPage(page pageKind) {
	if m.profileEditing {
		m.cancelProfileEdit()
	}
	m.page = page
	m.focus = focusNav
	m.actionIndex = 0
	m.logOffset = 0
	m.contentOffset = 0
	m.clampActionIndex()
}

func roomsWide(contentWidth int) bool {
	return contentWidth >= 86
}

// compactListHeight keeps the room list close to its actual row count, leaving
// the rest of a narrow screen for the detail pane that a mouse/keyboard user is
// inspecting.
func compactListHeight(accounts, contentHeight int) int {
	height := accounts + 1
	if height < 3 {
		height = 3
	}
	if max := contentHeight / 2; height > max {
		height = max
	}
	if height < 1 {
		height = 1
	}
	return height
}

// roomRowHitboxes returns the clickable rows and their account indexes. The
// list follows the selection when there are more accounts than visible rows.
func (m Model) roomRowHitboxes(x, y, width, visible int) ([]rect, []int) {
	accounts := len(m.status.Accounts)
	if visible <= 0 || accounts == 0 {
		return nil, nil
	}
	selected := m.selectedIndex()
	offset := 0
	if selected >= visible {
		offset = selected - visible + 1
	}
	if maxOffset := accounts - visible; offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	rows := make([]rect, 0, visible)
	indexes := make([]int, 0, visible)
	for row := 0; row < visible && offset+row < accounts; row++ {
		rows = append(rows, rect{X: x, Y: y + row, W: width, H: 1})
		indexes = append(indexes, offset+row)
	}
	return rows, indexes
}
