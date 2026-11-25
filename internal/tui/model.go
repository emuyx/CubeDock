package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/emuyx/cubedock/internal/auth"
	"github.com/emuyx/cubedock/internal/core"
	"github.com/emuyx/cubedock/internal/netease"
)

type actionKind int

const (
	actionStart actionKind = iota
	actionStop
	actionRestart
	actionStartAll
	actionStopAll
)

const (
	noticeLifetime = 6 * time.Second
)

type tickMsg time.Time
type statusMsg struct{ status core.Status }
type actionMsg struct {
	label string
	err   error
}
type diagnosticsMsg struct {
	checks []core.DiagnosticCheck
}
type importResultMsg struct {
	index    int
	name     string
	needName bool
	err      error
}

type modalKind int

const (
	modalNone modalKind = iota
	modalImportSauth
)

// Model is the Bubble Tea state. It never owns a second copy of the room data:
// every refresh replaces status with core.Manager.Snapshot().
type Model struct {
	options Options
	manager *core.Manager

	status core.Status
	width  int
	height int

	page          pageKind
	focus         focusArea
	selected      int
	actionIndex   int
	logWatch      bool // only show the selected room's events
	logOffset     int  // lines above the tail
	contentOffset int

	diagnostics     []core.DiagnosticCheck
	diagnosticsAt   time.Time
	diagnosticsBusy bool
	diagnosticsErr  string

	modal       modalKind
	input       []rune
	inputCursor int
	modalBusy   bool
	modalErr    string

	confirmQuit bool
	busy        bool
	notice      string
	noticeAt    time.Time

	mouseX      int
	mouseY      int
	mouseActive bool
}

func newModel(options Options) Model {
	return Model{options: options, manager: options.Manager, page: pageHome, focus: focusNav}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.snapshotCmd(), m.tickCmd())
}

func (m Model) snapshotCmd() tea.Cmd {
	manager := m.manager
	return func() tea.Msg { return statusMsg{status: manager.Snapshot()} }
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(at time.Time) tea.Msg { return tickMsg(at) })
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
		m.clamp()
	case statusMsg:
		m.status = message.status
		m.clamp()
	case tickMsg:
		if m.notice != "" && time.Since(m.noticeAt) >= noticeLifetime {
			m.notice = ""
		}
		return m, tea.Batch(m.snapshotCmd(), m.tickCmd())
	case actionMsg:
		m.busy = false
		if message.err != nil {
			m.setNotice(message.label + "失败：" + message.err.Error())
		} else {
			m.setNotice(message.label + "已发送")
		}
		return m, m.snapshotCmd()
	case diagnosticsMsg:
		m.diagnosticsBusy = false
		m.diagnostics = message.checks
		m.diagnosticsAt = time.Now()
		m.contentOffset = 0
		return m, nil
	case importResultMsg:
		m.modalBusy = false
		if message.err != nil {
			m.modalErr = message.err.Error()
			return m, nil
		}
		m.modal = modalNone
		m.input = nil
		m.inputCursor = 0
		m.page = pageAccounts
		m.focus = focusContent
		m.selected = message.index
		m.setNotice(fmt.Sprintf("已添加账号 %s（房间 %d）", message.name, message.index+1))
		if message.needName {
			m.setNotice(fmt.Sprintf("已添加账号（房间 %d），但还没有游戏名；请用 WebUI 起名", message.index+1))
		}
		return m, m.snapshotCmd()
	case tea.MouseMsg:
		return m.handleMouse(message)
	case tea.KeyMsg:
		return m.handleKey(message)
	}
	return m, nil
}

func (m Model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	m.mouseActive = false
	if m.modal != modalNone {
		return m.handleModalKey(key)
	}

	if m.confirmQuit {
		switch name {
		case "y", "Y", "enter":
			return m, tea.Quit
		case "n", "N", "esc", "q", "ctrl+c":
			m.confirmQuit = false
		}
		return m, nil
	}

	if name >= "1" && name <= "7" && len(name) == 1 {
		index := int(name[0] - '1')
		if index >= 0 && index < len(pageOrder) {
			m.setPage(pageOrder[index])
			m.focus = focusContent
		}
		return m, nil
	}

	switch name {
	case "ctrl+c":
		return m, tea.Quit
	case "q":
		m.confirmQuit = true
	case "?":
		m.setPage(pageFAQ)
		m.focus = focusContent
	case "tab":
		m.cycleFocus(false)
	case "shift+tab":
		m.cycleFocus(true)
	case "left":
		m.moveHorizontal(-1)
	case "right":
		m.moveHorizontal(1)
	case "enter":
		return m.activateFocused()
	case "esc":
		if m.page != pageHome {
			m.setPage(pageHome)
			m.focus = focusNav
		} else {
			m.focus = focusNav
		}
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "g", "home":
		m.goTop()
	case "G", "end":
		m.goBottom()
	case "pgup":
		m.move(-10)
	case "pgdown":
		m.move(10)
	case "f":
		if m.page == pageLogs {
			m.logWatch = !m.logWatch
			m.logOffset = 0
			m.setNotice("日志筛选：" + m.logFilterLabel())
		}
	case "o":
		m.openWebUI()
	case "s":
		m.focus = focusActions
		return m.action(actionStart)
	case "x":
		m.focus = focusActions
		return m.action(actionStop)
	case "r":
		m.focus = focusActions
		return m.action(actionRestart)
	case "S":
		m.focus = focusActions
		return m.action(actionStartAll)
	case "X":
		m.focus = focusActions
		return m.action(actionStopAll)
	case "d":
		m.focus = focusActions
		return m.toggleDebug()
	case "R":
		if m.page == pageDiagnostics {
			return m.runDiagnostics()
		}
	}
	return m, nil
}

func (m Model) handleMouse(event tea.MouseMsg) (tea.Model, tea.Cmd) {
	m.mouseX, m.mouseY = event.X, event.Y
	m.mouseActive = true
	if m.modal != modalNone {
		return m, nil
	}
	if m.width < minWidth || m.height < minHeight {
		return m, nil
	}
	if m.confirmQuit {
		if event.Action == tea.MouseActionPress && event.Button == tea.MouseButtonLeft {
			m.confirmQuit = false
		}
		return m, nil
	}
	if event.Action == tea.MouseActionMotion {
		return m, nil
	}
	if event.Button == tea.MouseButtonWheelUp {
		m.wheel(-1)
		return m, nil
	}
	if event.Button == tea.MouseButtonWheelDown {
		m.wheel(1)
		return m, nil
	}
	if event.Action != tea.MouseActionPress || event.Button != tea.MouseButtonLeft {
		return m, nil
	}

	info := m.layoutInfo()
	for index, area := range info.navRects {
		if area.Contains(m.mouseX, m.mouseY) {
			if index < len(pageOrder) {
				m.setPage(pageOrder[index])
				m.focus = focusNav
			}
			return m, nil
		}
	}
	for index, area := range info.toolbarRects {
		if area.Contains(m.mouseX, m.mouseY) {
			m.focus = focusActions
			m.actionIndex = index
			return m.activateToolbar(index)
		}
	}
	for row, area := range info.roomRows {
		if area.Contains(m.mouseX, m.mouseY) {
			if row < len(info.roomRowIndex) {
				m.selected = info.roomRowIndex[row]
			}
			m.focus = focusContent
			return m, nil
		}
	}
	if info.contentArea.Contains(m.mouseX, m.mouseY) {
		m.focus = focusContent
	}
	return m, nil
}

func (m *Model) wheel(delta int) {
	info := m.layoutInfo()
	if info.navArea.Contains(m.mouseX, m.mouseY) {
		m.focus = focusNav
		m.cyclePage(delta)
		return
	}
	if !info.contentArea.Contains(m.mouseX, m.mouseY) {
		return
	}
	m.focus = focusContent
	switch m.page {
	case pageLogs:
		m.scrollLogs(-delta * 3)
	case pageAccounts:
		m.move(delta)
	default:
		m.moveContent(delta * 3)
	}
}

func (m *Model) moveHorizontal(step int) {
	switch m.focus {
	case focusNav:
		m.cyclePage(step)
	case focusActions:
		m.actionIndex += step
		m.clampActionIndex()
	}
}

func (m Model) activateFocused() (tea.Model, tea.Cmd) {
	switch m.focus {
	case focusNav:
		m.focus = focusContent
	case focusActions:
		return m.activateToolbar(m.actionIndex)
	case focusContent:
		switch m.page {
		case pageAccounts:
			if len(m.layoutInfo().toolbarRects) > 0 {
				m.focus = focusActions
				m.clampActionIndex()
			}
		case pageDiagnostics:
			return m.runDiagnostics()
		}
	}
	return m, nil
}

func (m Model) activateToolbar(index int) (tea.Model, tea.Cmd) {
	items := toolbarItemsFor(m.page)
	if index < 0 || index >= len(items) {
		return m, nil
	}
	item := items[index]
	m.focus = focusActions
	m.actionIndex = index
	switch item.Special {
	case specialWebUI:
		m.openWebUI()
		return m, nil
	case specialDebug:
		return m.toggleDebug()
	case specialDiagnostics:
		return m.runDiagnostics()
	case specialImport:
		m.openImportModal()
		return m, nil
	default:
		return m.action(item.Action)
	}
}

func (m Model) runDiagnostics() (tea.Model, tea.Cmd) {
	if m.diagnosticsBusy {
		return m, nil
	}
	m.diagnosticsBusy = true
	m.diagnosticsErr = ""
	manager := m.manager
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		return diagnosticsMsg{checks: manager.Diagnose(ctx, "")}
	}
}

func (m *Model) openImportModal() {
	m.modal = modalImportSauth
	m.input = nil
	m.inputCursor = 0
	m.modalBusy = false
	m.modalErr = ""
	m.contentOffset = 0
	m.focus = focusContent
}

func (m Model) handleModalKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	if name == "ctrl+c" {
		return m, tea.Quit
	}
	if m.modalBusy {
		return m, nil
	}
	switch name {
	case "esc":
		m.modal = modalNone
		m.input = nil
		m.inputCursor = 0
		m.modalErr = ""
	case "enter":
		return m.submitImport()
	case "backspace", "ctrl+h":
		m.modalErr = ""
		if m.inputCursor > 0 {
			m.input = append(m.input[:m.inputCursor-1], m.input[m.inputCursor:]...)
			m.inputCursor--
		}
	case "delete":
		m.modalErr = ""
		if m.inputCursor < len(m.input) {
			m.input = append(m.input[:m.inputCursor], m.input[m.inputCursor+1:]...)
		}
	case "left":
		if m.inputCursor > 0 {
			m.inputCursor--
		}
	case "right":
		if m.inputCursor < len(m.input) {
			m.inputCursor++
		}
	case "home", "ctrl+a":
		m.inputCursor = 0
	case "end", "ctrl+e":
		m.inputCursor = len(m.input)
	case "ctrl+u":
		m.input = nil
		m.inputCursor = 0
		m.modalErr = ""
	default:
		if key.Paste || key.Type == tea.KeyRunes {
			m.modalErr = ""
			runes := key.Runes
			if len(m.input)+len(runes) > 64*1024 {
				m.modalErr = "粘贴内容过大，请分批导入"
				return m, nil
			}
			next := make([]rune, 0, len(m.input)+len(runes))
			next = append(next, m.input[:m.inputCursor]...)
			next = append(next, runes...)
			next = append(next, m.input[m.inputCursor:]...)
			m.input = next
			m.inputCursor += len(runes)
		}
	}
	return m, nil
}

func (m Model) submitImport() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(string(m.input))
	if text == "" {
		m.modalErr = "请先粘贴 sauth/session JSON"
		return m, nil
	}
	if m.manager == nil {
		m.modalErr = "房间管理器不可用"
		return m, nil
	}
	if m.modalBusy {
		return m, nil
	}
	m.modalBusy = true
	m.modalErr = ""
	manager := m.manager
	profileID := m.status.Config.DefaultProfileID()
	return m, func() tea.Msg {
		token, method, expiresAt, err := core.NormalizeSession(text, "")
		if err != nil {
			return importResultMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		profile, profileErr := auth.LookupProfileFor(ctx, netease.AuthMethod(method), token)
		needName := false
		profileError := ""
		if profileErr != nil {
			if errors.Is(profileErr, netease.ErrProfileNameMissing) {
				needName = true
				profileError = netease.ErrProfileNameMissing.Error()
			} else {
				return importResultMsg{err: fmt.Errorf("%s", sanitizeImportError(profileErr, token))}
			}
		}
		meta := core.AccountMeta{
			Name: profile.Name, UID: profile.UID, Level: profile.Level,
			AvatarURL: profile.AvatarURL, Platform: method, AuthMethod: method,
			ExpiresAt: expiresAt, ProfileError: profileError, ProfileID: profileID,
		}
		index, err := manager.AddAccount(token, meta)
		if err != nil {
			return importResultMsg{err: fmt.Errorf("%s", sanitizeImportError(err, token))}
		}
		name := profile.Name
		if name == "" {
			name = "未命名账号"
		}
		return importResultMsg{index: index, name: name, needName: needName}
	}
}

func sanitizeImportError(err error, token string) string {
	message := err.Error()
	if token != "" {
		message = strings.ReplaceAll(message, token, "[会话]")
	}
	return message
}

func (m *Model) openWebUI() {
	if m.options.OpenURL == nil {
		return
	}
	m.options.OpenURL(m.options.LocalURL)
	m.setNotice("已尝试打开 WebUI 地址")
}

func (m *Model) setNotice(message string) {
	m.notice = strings.TrimSpace(message)
	m.noticeAt = time.Now()
}

func (m *Model) move(delta int) {
	switch m.focus {
	case focusNav:
		m.cyclePage(delta)
		return
	case focusContent:
	default:
		return
	}
	switch m.page {
	case pageLogs:
		m.scrollLogs(-delta)
	case pageAccounts:
		count := len(m.status.Accounts)
		if count == 0 {
			m.selected = 0
			return
		}
		m.selected += delta
		m.clamp()
	default:
		m.moveContent(delta)
	}
}

func (m *Model) moveContent(delta int) {
	m.contentOffset += delta
	if m.contentOffset < 0 {
		m.contentOffset = 0
	}
	if m.contentOffset > 500 {
		m.contentOffset = 500
	}
}

func (m *Model) goTop() {
	if m.focus == focusNav {
		m.setPage(pageOrder[0])
		return
	}
	switch m.page {
	case pageLogs:
		m.logOffset = 1 << 20
		m.clampLog()
	case pageAccounts:
		m.selected = 0
	default:
		m.contentOffset = 0
	}
}

func (m *Model) goBottom() {
	if m.focus == focusNav {
		m.setPage(pageOrder[len(pageOrder)-1])
		return
	}
	switch m.page {
	case pageLogs:
		m.logOffset = 0
	case pageAccounts:
		if count := len(m.status.Accounts); count > 0 {
			m.selected = count - 1
		}
	default:
		m.contentOffset = 1 << 20
	}
}

func (m *Model) scrollLogs(delta int) {
	m.logOffset += delta
	m.clampLog()
}

func (m *Model) clamp() {
	count := len(m.status.Accounts)
	switch {
	case count == 0:
		m.selected = 0
	case m.selected < 0:
		m.selected = 0
	case m.selected >= count:
		m.selected = count - 1
	}
	m.clampActionIndex()
	m.normalizeFocus()
	m.clampLog()
	if m.contentOffset < 0 {
		m.contentOffset = 0
	}
}

func (m *Model) clampLog() {
	maxOffset := len(m.visibleEvents()) - m.logVisibleLines()
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.logOffset < 0 {
		m.logOffset = 0
	}
	if m.logOffset > maxOffset {
		m.logOffset = maxOffset
	}
}

func (m Model) selectedIndex() int {
	if len(m.status.Accounts) == 0 {
		return -1
	}
	if m.selected < 0 {
		return 0
	}
	if m.selected >= len(m.status.Accounts) {
		return len(m.status.Accounts) - 1
	}
	return m.selected
}

func (m Model) selectedRoom() (core.RoomStatus, core.AccountStatus, bool) {
	index := m.selectedIndex()
	if index < 0 {
		return core.RoomStatus{Phase: "stopped"}, core.AccountStatus{}, false
	}
	room := core.RoomStatus{Phase: "stopped"}
	if index < len(m.status.Rooms) {
		room = m.status.Rooms[index]
	}
	return room, m.status.Accounts[index], true
}

func (m Model) visibleEvents() []core.Event {
	if !m.logWatch {
		return m.status.Events
	}
	index := m.selectedIndex()
	if index < 0 {
		return m.status.Events
	}
	room := index + 1
	out := make([]core.Event, 0, len(m.status.Events))
	for _, event := range m.status.Events {
		if event.Room == room {
			out = append(out, event)
		}
	}
	return out
}

func (m Model) logFilterLabel() string {
	if !m.logWatch {
		return "全部"
	}
	if index := m.selectedIndex(); index >= 0 {
		return fmt.Sprintf("房间 %d", index+1)
	}
	return "全部"
}

func (m Model) logVisibleLines() int {
	lines := m.height - 9
	if lines < 1 {
		return 1
	}
	return lines
}

func (m Model) action(kind actionKind) (tea.Model, tea.Cmd) {
	if m.busy {
		return m, nil
	}
	index := m.selectedIndex()
	if kind != actionStartAll && kind != actionStopAll && index < 0 {
		m.setNotice("还没有账号，请先在 WebUI 添加")
		return m, nil
	}
	m.busy = true
	manager := m.manager
	label := actionLabel(kind)
	return m, func() tea.Msg {
		var err error
		switch kind {
		case actionStart:
			err = manager.StartAccount(index)
		case actionStop:
			err = manager.StopAccount(index)
		case actionRestart:
			err = manager.RestartAccount(index)
		case actionStartAll:
			err = manager.Start()
		case actionStopAll:
			manager.Stop()
		}
		return actionMsg{label: label, err: err}
	}
}

func actionLabel(kind actionKind) string {
	switch kind {
	case actionStart:
		return "启动"
	case actionStop:
		return "停止"
	case actionRestart:
		return "重连"
	case actionStartAll:
		return "启动全部房间"
	case actionStopAll:
		return "停止全部房间"
	default:
		return "操作"
	}
}

func (m Model) toggleDebug() (tea.Model, tea.Cmd) {
	if m.busy {
		return m, nil
	}
	m.busy = true
	enabled := !m.status.Config.Debug
	manager := m.manager
	label := "开启 Debug"
	if !enabled {
		label = "关闭 Debug"
	}
	return m, func() tea.Msg {
		return actionMsg{label: label, err: manager.SetDebug(enabled)}
	}
}
