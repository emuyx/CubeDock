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
	specialAccountManage = "account-manage"
	specialBackup        = "backup"
	specialRestore       = "restore"
	specialProfileEdit   = "profile-edit"
	specialProfileNew    = "profile-new"
	specialProfileDup    = "profile-duplicate"
	specialProfileDel    = "profile-delete"
	specialProfileSave   = "profile-save"
	specialProfileCancel = "profile-cancel"
	specialPacks         = "packs"
	specialSettingsEdit  = "settings-edit"
	specialAdminPassword = "admin-password"
	specialBulk          = "bulk"
)

var accountToolbar = []toolbarItem{
	{Label: "登录", Special: specialLogin},
	{Label: "导入", Special: specialImport},
	{Label: "批量", Special: specialBulk},
	{Label: "管理", Special: specialAccountManage},
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
	{Label: "资源包", Special: specialPacks},
	{Label: "WebUI", Special: specialWebUI},
}

var profileEditToolbar = []toolbarItem{
	{Label: "保存", Special: specialProfileSave},
	{Label: "取消", Special: specialProfileCancel},
	{Label: "WebUI", Special: specialWebUI},
}

var settingsToolbar = []toolbarItem{
	{Label: "编辑", Special: specialSettingsEdit},
	{Label: "备份", Special: specialBackup},
	{Label: "恢复", Special: specialRestore},
	{Label: "密码", Special: specialAdminPassword},
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

	// Per-account shortcuts under the page list in the wide sidebar.
	accountRects    []rect
	accountRowIndex []int

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

	// pageLines/pageHits are rendered once here so View() and hit testing use
	// exactly the same content.
	pageLines []string
	pageHits  []pageHit
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
		info.accountRects, info.accountRowIndex = m.sidebarAccountRects(info)
	} else {
		info.navLabels, info.navRects, info.navArea = m.topNavLayout(width, info.bodyY)
		info.contentArea = rect{X: 0, Y: info.bodyY + info.navArea.H, W: width, H: info.bodyH - info.navArea.H}
	}

	info.frameY = info.contentArea.Y
	frameH := info.contentArea.H
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
		top := info.content.Y + accountControlsRows
		avail := info.content.H - accountControlsRows - m.accountActionBarRows(info.content.W)
		if avail < 1 {
			avail = 1
		}
		visibleCount := len(m.visibleAccountIndexes())
		if roomsWide(info.content.W) {
			leftWidth := clampInt(info.content.W/3, 24, 38)
			rightWidth := info.content.W - leftWidth - 3
			if rightWidth >= 24 {
				info.listArea = rect{X: info.content.X, Y: top, W: leftWidth, H: avail}
				info.roomRows, info.roomRowIndex = m.roomRowHitboxes(info.content.X, top, leftWidth, avail)
			}
		} else {
			listHeight := compactListHeight(visibleCount, avail)
			info.listArea = rect{X: info.content.X, Y: top, W: info.content.W, H: listHeight}
			info.roomRows, info.roomRowIndex = m.roomRowHitboxes(info.content.X, top, info.content.W, listHeight)
		}
	}
	if m.page == pageLogs {
		info.logArea = info.content
	}

	// Render the page once, then map its local hitboxes into screen coordinates.
	info.pageLines, info.pageHits = m.buildPage(info)
	offset := 0
	if info.content.H > 0 && len(info.pageLines) > info.content.H {
		_, offset, _ = windowLines(info.pageLines, info.content.H, m.contentOffset)
	}
	info.pageHits = screenHits(info.pageHits, info.content, offset)

	// Compatibility views derived from the same hitboxes.
	info.toolbar = m.toolbarItems()
	if m.modal != modalNone {
		info.toolbar = nil
	}
	if len(info.toolbar) > 0 {
		info.toolbarRects = make([]rect, len(info.toolbar))
		rows := make(map[int]struct{})
		for _, hit := range info.pageHits {
			if hit.kind == hitToolbar && hit.index >= 0 && hit.index < len(info.toolbarRects) {
				info.toolbarRects[hit.index] = hit.rect
				rows[hit.rect.Y] = struct{}{}
			}
		}
		info.toolbarRows = len(rows)
		if info.toolbarRows < 1 {
			info.toolbarRows = 1
		}
	}
	if m.page == pageProfiles && m.profileEditing {
		for _, hit := range info.pageHits {
			if hit.kind == hitProfileField {
				info.profileFields = append(info.profileFields, hit.rect)
				info.profileFieldIndex = append(info.profileFieldIndex, hit.index)
			}
		}
	}
	return info
}

// screenHits translates page-local hitboxes to screen coordinates and drops
// the ones scrolled out of the content view.
func screenHits(hits []pageHit, content rect, offset int) []pageHit {
	out := make([]pageHit, 0, len(hits))
	for _, hit := range hits {
		y := content.Y + hit.Y - offset
		if y < content.Y || y >= content.Y+content.H {
			continue
		}
		hit.Y = y
		hit.X = content.X + hit.X
		out = append(out, hit)
	}
	return out
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
	if page != pageLogs {
		m.consoleMode = false
		m.logSearch = false
	}
	if page != pageAccounts {
		m.accountSearch = false
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

const sidebarAccountHeaderRows = 2 // blank line + "我的房间" heading

// sidebarAccountRects returns the per-account dashboard shortcuts below the
// page list. The list shows as many accounts as fit and reserves one row for an
// overflow counter when there are more.
func (m Model) sidebarAccountRects(info screenLayout) ([]rect, []int) {
	if len(m.status.Accounts) == 0 {
		return nil, nil
	}
	start := info.bodyY + len(pageOrder) + sidebarAccountHeaderRows
	available := info.bodyY + info.bodyH - start
	if available <= 0 {
		return nil, nil
	}
	rows := len(m.status.Accounts)
	if rows > available {
		rows = available - 1
	}
	if rows < 0 {
		rows = 0
	}
	rects := make([]rect, 0, rows)
	indexes := make([]int, 0, rows)
	for row := 0; row < rows; row++ {
		rects = append(rects, rect{X: 0, Y: start + row, W: info.navWidth, H: 1})
		indexes = append(indexes, row)
	}
	return rects, indexes
}

// roomRowHitboxes returns the clickable rows and their original account
// indexes. The list follows the selection when there are more visible accounts
// than rows, and respects the current search/filter/sort order.
func (m Model) roomRowHitboxes(x, y, width, visible int) ([]rect, []int) {
	visibleAccounts := m.visibleAccountIndexes()
	if visible <= 0 || len(visibleAccounts) == 0 {
		return nil, nil
	}
	selected := m.selectedIndex()
	selectedPos := 0
	for position, index := range visibleAccounts {
		if index == selected {
			selectedPos = position
			break
		}
	}
	offset := 0
	if selectedPos >= visible {
		offset = selectedPos - visible + 1
	}
	if maxOffset := len(visibleAccounts) - visible; offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	rows := make([]rect, 0, visible)
	indexes := make([]int, 0, visible)
	for row := 0; row < visible && offset+row < len(visibleAccounts); row++ {
		rows = append(rows, rect{X: x, Y: y + row, W: width, H: 1})
		indexes = append(indexes, visibleAccounts[offset+row])
	}
	return rows, indexes
}
