package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/emuyx/cubedock/internal/auth"
	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/core"
	"github.com/emuyx/cubedock/internal/fileutil"
	"github.com/emuyx/cubedock/internal/interoplogin"
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
	token    string
	method   string
	err      error
}
type loginResultMsg struct {
	index      int
	name       string
	needName   bool
	token      string
	method     string
	verifyURL  string
	verifyNote string
	err        error
}
type nameResultMsg struct {
	profile auth.Profile
	err     error
}
type adminResultMsg struct {
	username string
	err      error
}
type restoreResultMsg struct {
	path     string
	warnings []string
	err      error
}

type modalKind int

const (
	modalNone modalKind = iota
	modalImportSauth
	modalLogin
	modalName
	modalAdmin
	modalRestore
	modalProfileTags
	modalProfileRoomName
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

	loginEmail          []rune
	loginEmailCursor    int
	loginPassword       []rune
	loginPasswordCursor int
	loginPlatform       string
	loginField          int
	loginVerifyURL      string
	loginVerifyNote     string

	nameIndex  int
	nameToken  string
	nameMethod string
	nameInput  []rune
	nameCursor int

	adminUser           []rune
	adminUserCursor     int
	adminPassword       []rune
	adminPasswordCursor int
	adminConfirm        []rune
	adminConfirmCursor  int
	adminField          int
	lastBackup          string

	profileEditing    bool
	profileNew        bool
	profileIndex      int
	profileField      int
	profileFieldEdit  bool
	profileBuffer     []rune
	profileCursor     int
	profileError      string
	profileDirty      bool
	profileDraft      config.Profile
	profileOriginalID string
	profileDeleteID   string
	profileDeleteAt   time.Time
	tagCursor         int
	roomNameCursor    int

	confirmQuit bool
	busy        bool
	frame       int
	notice      string
	noticeAt    time.Time

	mouseX      int
	mouseY      int
	mouseActive bool
}

func newModel(options Options) Model {
	model := Model{options: options, manager: options.Manager, page: pageHome, focus: focusNav}
	if options.SetupRequired {
		model.openAdminModal()
	}
	return model
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
		m.frame++
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
		m.input = nil
		m.inputCursor = 0
		m.page = pageAccounts
		m.focus = focusContent
		m.selected = message.index
		if message.needName {
			m.openNameModal(message.index, message.token, message.method)
			m.setNotice(fmt.Sprintf("已添加账号（房间 %d），请设置游戏名", message.index+1))
			return m, m.snapshotCmd()
		}
		m.modal = modalNone
		m.setNotice(fmt.Sprintf("已添加账号 %s（房间 %d）", message.name, message.index+1))
		return m, m.snapshotCmd()
	case loginResultMsg:
		m.modalBusy = false
		if message.err != nil {
			m.modalErr = message.err.Error()
			return m, nil
		}
		if message.verifyURL != "" || message.verifyNote != "" {
			m.loginVerifyURL = message.verifyURL
			m.loginVerifyNote = message.verifyNote
			return m, nil
		}
		m.loginPassword = nil
		m.loginPasswordCursor = 0
		m.page = pageAccounts
		m.focus = focusContent
		m.selected = message.index
		if message.needName {
			m.openNameModal(message.index, message.token, message.method)
			m.setNotice(fmt.Sprintf("已添加账号（房间 %d），请设置游戏名", message.index+1))
			return m, m.snapshotCmd()
		}
		m.modal = modalNone
		m.setNotice(fmt.Sprintf("已添加账号 %s（房间 %d）", message.name, message.index+1))
		return m, m.snapshotCmd()
	case nameResultMsg:
		m.modalBusy = false
		if message.err != nil {
			m.modalErr = message.err.Error()
			return m, nil
		}
		if m.manager == nil {
			m.modalErr = "房间管理器不可用"
			return m, nil
		}
		index := m.nameIndex
		if err := m.manager.UpdateAccountProfile(index, message.profile.Name, message.profile.UID,
			message.profile.Level, message.profile.AvatarURL, ""); err != nil {
			m.modalErr = "保存账号资料失败：" + err.Error()
			return m, nil
		}
		m.modal = modalNone
		m.nameInput = nil
		m.nameCursor = 0
		m.setNotice(fmt.Sprintf("已设置游戏名：%s", message.profile.Name))
		return m, m.snapshotCmd()
	case adminResultMsg:
		m.modalBusy = false
		if message.err != nil {
			m.modalErr = message.err.Error()
			return m, nil
		}
		m.options.SetupRequired = false
		m.options.AdminName = message.username
		m.options.CreateAdmin = nil
		m.modal = modalNone
		m.setNotice("管理员账户已创建：" + message.username)
		return m, nil
	case restoreResultMsg:
		m.modalBusy = false
		if message.err != nil {
			m.modalErr = message.err.Error()
			return m, nil
		}
		m.modal = modalNone
		m.input = nil
		m.inputCursor = 0
		m.page = pageSettings
		if len(message.warnings) > 0 {
			m.setNotice("配置已恢复；" + strings.Join(message.warnings, "；"))
		} else {
			m.setNotice("配置已恢复：" + message.path)
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
	if m.profileEditing {
		return m.handleProfileKey(key)
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
		m.scrollPage(-10)
	case "pgdown":
		m.scrollPage(10)
	case "f":
		if m.page == pageLogs {
			m.logWatch = !m.logWatch
			m.logOffset = 0
			m.setNotice("日志筛选：" + m.logFilterLabel())
		}
	case "o":
		m.openWebUI()
	case "a":
		if m.page == pageSettings && m.options.SetupRequired {
			m.openAdminModal()
		}
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
		if event.Button == tea.MouseButtonWheelUp {
			m.moveContent(-3)
		} else if event.Button == tea.MouseButtonWheelDown {
			m.moveContent(3)
		}
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
			if m.profileEditing {
				m.setNotice("请先保存或取消配置方案")
				return m, nil
			}
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
	if m.profileEditing {
		for index, area := range info.profileFields {
			if area.Contains(m.mouseX, m.mouseY) {
				if index < len(info.profileFieldIndex) {
					m.profileField = info.profileFieldIndex[index]
					m.profileFieldEdit = false
					m.profileError = ""
					m.focus = focusContent
				}
				return m, nil
			}
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
		case pageProfiles:
			m.openProfileEditor()
		}
	}
	return m, nil
}

func (m Model) activateToolbar(index int) (tea.Model, tea.Cmd) {
	items := m.toolbarItems()
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
	case specialLogin:
		m.openLoginModal()
		return m, nil
	case specialBackup:
		m.backupConfig()
		return m, nil
	case specialRestore:
		m.openRestoreModal()
		return m, nil
	case specialProfileEdit:
		m.openProfileEditor()
		return m, nil
	case specialProfileNew:
		if m.status.Running {
			m.setNotice("请先停止房间，再新增配置方案")
			return m, nil
		}
		m.newProfileDraft()
		return m, nil
	case specialProfileDup:
		if m.status.Running {
			m.setNotice("请先停止房间，再复制配置方案")
			return m, nil
		}
		m.duplicateProfileDraft()
		return m, nil
	case specialProfileDel:
		return m.deleteProfile()
	case specialProfileSave:
		return m.saveProfile()
	case specialProfileCancel:
		m.cancelProfileEdit()
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
	if m.status.Running {
		m.setNotice("请先停止房间，再添加账号")
		return
	}
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
	if name == "pgup" {
		m.moveContent(-10)
		return m, nil
	}
	if name == "pgdown" {
		m.moveContent(10)
		return m, nil
	}
	switch m.modal {
	case modalImportSauth:
		return m.handleSauthKey(key)
	case modalLogin:
		return m.handleLoginKey(key)
	case modalName:
		return m.handleNameKey(key)
	case modalAdmin:
		return m.handleAdminKey(key)
	case modalRestore:
		return m.handleRestoreKey(key)
	case modalProfileTags:
		return m.handleTagsKey(key)
	case modalProfileRoomName:
		return m.handleRoomNameKey(key)
	}
	return m, nil
}

func (m Model) handleSauthKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	name := key.String()
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

func (m Model) handleLoginKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	switch name {
	case "esc":
		m.modal = modalNone
		m.modalErr = ""
		m.loginVerifyURL = ""
		m.loginVerifyNote = ""
		return m, nil
	case "enter":
		return m.submitLogin()
	case "ctrl+o":
		if m.loginVerifyURL != "" {
			m.openWebUIURL(m.loginVerifyURL)
		}
		return m, nil
	case "tab":
		m.loginField = (m.loginField + 1) % 3
		return m, nil
	case "shift+tab":
		m.loginField = (m.loginField + 2) % 3
		return m, nil
	case "up":
		m.loginField = (m.loginField + 2) % 3
		return m, nil
	case "down":
		m.loginField = (m.loginField + 1) % 3
		return m, nil
	}
	if m.loginField == 2 {
		switch name {
		case "1":
			m.loginPlatform = string(netease.AuthInterop)
		case "2":
			m.loginPlatform = string(netease.AuthMobileNetEase)
		case "left", "right", " ":
			if m.loginPlatform == string(netease.AuthMobileNetEase) {
				m.loginPlatform = string(netease.AuthInterop)
			} else {
				m.loginPlatform = string(netease.AuthMobileNetEase)
			}
		}
		return m, nil
	}
	m.modalErr = ""
	var value *[]rune
	var cursor *int
	if m.loginField == 0 {
		value, cursor = &m.loginEmail, &m.loginEmailCursor
	} else {
		value, cursor = &m.loginPassword, &m.loginPasswordCursor
	}
	switch name {
	case "backspace", "ctrl+h":
		if *cursor > 0 {
			*value = append((*value)[:*cursor-1], (*value)[*cursor:]...)
			*cursor--
		}
	case "delete":
		if *cursor < len(*value) {
			*value = append((*value)[:*cursor], (*value)[*cursor+1:]...)
		}
	case "left":
		if *cursor > 0 {
			*cursor--
		}
	case "right":
		if *cursor < len(*value) {
			*cursor++
		}
	case "home", "ctrl+a":
		*cursor = 0
	case "end", "ctrl+e":
		*cursor = len(*value)
	case "ctrl+u":
		*value = nil
		*cursor = 0
	default:
		if key.Paste || key.Type == tea.KeyRunes {
			runes := key.Runes
			next := make([]rune, 0, len(*value)+len(runes))
			next = append(next, (*value)[:*cursor]...)
			next = append(next, runes...)
			next = append(next, (*value)[*cursor:]...)
			*value = next
			*cursor += len(runes)
		}
	}
	return m, nil
}

func (m Model) handleNameKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	switch name {
	case "esc":
		m.modal = modalNone
		m.modalErr = ""
		m.nameInput = nil
		m.nameCursor = 0
		m.setNotice("账号已添加；可以稍后用 WebUI 设置游戏名")
	case "enter":
		return m.submitName()
	case "backspace", "ctrl+h":
		m.modalErr = ""
		if m.nameCursor > 0 {
			m.nameInput = append(m.nameInput[:m.nameCursor-1], m.nameInput[m.nameCursor:]...)
			m.nameCursor--
		}
	case "delete":
		m.modalErr = ""
		if m.nameCursor < len(m.nameInput) {
			m.nameInput = append(m.nameInput[:m.nameCursor], m.nameInput[m.nameCursor+1:]...)
		}
	case "left":
		if m.nameCursor > 0 {
			m.nameCursor--
		}
	case "right":
		if m.nameCursor < len(m.nameInput) {
			m.nameCursor++
		}
	case "home", "ctrl+a":
		m.nameCursor = 0
	case "end", "ctrl+e":
		m.nameCursor = len(m.nameInput)
	case "ctrl+u":
		m.nameInput = nil
		m.nameCursor = 0
		m.modalErr = ""
	default:
		if key.Paste || key.Type == tea.KeyRunes {
			runes := key.Runes
			next := make([]rune, 0, len(m.nameInput)+len(runes))
			next = append(next, m.nameInput[:m.nameCursor]...)
			next = append(next, runes...)
			next = append(next, m.nameInput[m.nameCursor:]...)
			m.nameInput = next
			m.nameCursor += len(runes)
		}
	}
	return m, nil
}

func (m *Model) backupConfig() {
	if m.manager == nil {
		m.setNotice("房间管理器不可用")
		return
	}
	snapshot := m.manager.Snapshot()
	payload := struct {
		Format     int                  `json:"format"`
		App        string               `json:"app"`
		Version    string               `json:"version"`
		ExportedAt string               `json:"exported_at"`
		Config     config.Config        `json:"config"`
		Accounts   []core.AccountStatus `json:"accounts"`
		Note       string               `json:"note"`
	}{
		Format: 2, App: "CubeDock", Version: m.options.Version,
		ExportedAt: time.Now().Format(time.RFC3339),
		Config:     snapshot.Config.WithoutRoomPasswords(),
		Accounts:   snapshot.Accounts,
		Note:       "备份包含配置方案、全局设置和账号信息，不包含网易登录凭证；恢复后需要重新登录或导入账号。",
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		m.setNotice("导出备份失败：" + err.Error())
		return
	}
	path := filepath.Join(m.manager.Dir(),
		fmt.Sprintf("cubedock-backup-%s.json", time.Now().Format("20060102-150405")))
	if err := fileutil.Replace(path, append(raw, '\n'), 0600); err != nil {
		m.setNotice("保存备份失败：" + err.Error())
		return
	}
	m.lastBackup = path
	m.setNotice("备份已保存：" + path)
}

func (m *Model) openRestoreModal() {
	if m.status.Running {
		m.setNotice("请先停止房间，再恢复配置")
		return
	}
	m.modal = modalRestore
	m.input = nil
	m.inputCursor = 0
	if m.manager != nil {
		if entries, err := os.ReadDir(m.manager.Dir()); err == nil {
			latest := ""
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasPrefix(entry.Name(), "cubedock-backup-") ||
					!strings.HasSuffix(entry.Name(), ".json") {
					continue
				}
				if entry.Name() > latest {
					latest = entry.Name()
				}
			}
			if latest != "" {
				m.input = []rune(latest)
				m.inputCursor = len(m.input)
			}
		}
	}
	m.modalBusy = false
	m.modalErr = ""
	m.contentOffset = 0
	m.focus = focusContent
}

func (m Model) handleRestoreKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	switch name {
	case "esc":
		m.modal = modalNone
		m.input = nil
		m.inputCursor = 0
		m.modalErr = ""
	case "enter":
		return m.submitRestore()
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

func (m Model) submitRestore() (tea.Model, tea.Cmd) {
	path := strings.TrimSpace(string(m.input))
	if path == "" {
		m.modalErr = "请输入备份 JSON 路径"
		return m, nil
	}
	if m.manager == nil {
		m.modalErr = "房间管理器不可用"
		return m, nil
	}
	if m.status.Running {
		m.modalErr = "请先停止房间，再恢复配置"
		return m, nil
	}
	if m.modalBusy {
		return m, nil
	}
	m.modalBusy = true
	m.modalErr = ""
	manager := m.manager
	resolved := path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(manager.Dir(), resolved)
	}
	return m, func() tea.Msg {
		raw, err := os.ReadFile(resolved)
		if err != nil {
			return restoreResultMsg{err: fmt.Errorf("读取备份失败: %w", err)}
		}
		var envelope struct {
			Config json.RawMessage `json:"config"`
		}
		if json.Unmarshal(raw, &envelope) != nil || len(envelope.Config) == 0 {
			return restoreResultMsg{err: errors.New("备份 JSON 无效")}
		}
		next, legacy, err := config.Decode(envelope.Config)
		if err != nil {
			return restoreResultMsg{err: errors.New("备份配置格式无效")}
		}
		snapshot := manager.Snapshot()
		warnings := []string{}
		if legacy {
			warnings = append(warnings, "这是旧版单配置备份，已迁移为「默认配置」")
		}
		switch {
		case next.WebPort == 0:
			warnings = append(warnings, "备份未包含控制台端口，已保留当前端口")
			next.WebPort = snapshot.Config.WebPort
		case next.WebPort != snapshot.Config.WebPort:
			warnings = append(warnings, fmt.Sprintf(
				"控制台端口未恢复，当前仍为 %d；如需修改端口，请关闭程序后修改配置文件",
				snapshot.Config.WebPort))
			next.WebPort = snapshot.Config.WebPort
		}
		restoreWarnings, err := manager.ConfigureForRestore(next)
		if err != nil {
			return restoreResultMsg{err: err}
		}
		warnings = append(warnings, restoreWarnings...)
		manager.Record("设置", "已从备份恢复配置")
		return restoreResultMsg{path: resolved, warnings: warnings}
	}
}

func (m *Model) openAdminModal() {
	if !m.options.SetupRequired {
		return
	}
	m.modal = modalAdmin
	m.adminUser = []rune("admin")
	m.adminUserCursor = len(m.adminUser)
	m.adminPassword = nil
	m.adminPasswordCursor = 0
	m.adminConfirm = nil
	m.adminConfirmCursor = 0
	m.adminField = 0
	m.modalBusy = false
	m.modalErr = ""
	m.contentOffset = 0
	m.focus = focusContent
}

func (m Model) handleAdminKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	switch name {
	case "esc":
		m.modal = modalNone
		m.modalErr = ""
		return m, nil
	case "enter":
		return m.submitAdmin()
	case "tab", "down":
		m.adminField = (m.adminField + 1) % 3
		return m, nil
	case "shift+tab", "up":
		m.adminField = (m.adminField + 2) % 3
		return m, nil
	}
	m.modalErr = ""
	var value *[]rune
	var cursor *int
	switch m.adminField {
	case 0:
		value, cursor = &m.adminUser, &m.adminUserCursor
	case 1:
		value, cursor = &m.adminPassword, &m.adminPasswordCursor
	default:
		value, cursor = &m.adminConfirm, &m.adminConfirmCursor
	}
	switch name {
	case "backspace", "ctrl+h":
		if *cursor > 0 {
			*value = append((*value)[:*cursor-1], (*value)[*cursor:]...)
			*cursor--
		}
	case "delete":
		if *cursor < len(*value) {
			*value = append((*value)[:*cursor], (*value)[*cursor+1:]...)
		}
	case "left":
		if *cursor > 0 {
			*cursor--
		}
	case "right":
		if *cursor < len(*value) {
			*cursor++
		}
	case "home", "ctrl+a":
		*cursor = 0
	case "end", "ctrl+e":
		*cursor = len(*value)
	case "ctrl+u":
		*value = nil
		*cursor = 0
	default:
		if key.Paste || key.Type == tea.KeyRunes {
			runes := key.Runes
			next := make([]rune, 0, len(*value)+len(runes))
			next = append(next, (*value)[:*cursor]...)
			next = append(next, runes...)
			next = append(next, (*value)[*cursor:]...)
			*value = next
			*cursor += len(runes)
		}
	}
	return m, nil
}

func (m Model) submitAdmin() (tea.Model, tea.Cmd) {
	username := strings.TrimSpace(string(m.adminUser))
	password := string(m.adminPassword)
	confirm := string(m.adminConfirm)
	if username == "" || password == "" {
		m.modalErr = "请填写管理员账号和密码"
		return m, nil
	}
	if password != confirm {
		m.modalErr = "两次输入的密码不一致"
		return m, nil
	}
	if m.options.CreateAdmin == nil {
		m.modalErr = "管理员初始化不可用"
		return m, nil
	}
	if m.modalBusy {
		return m, nil
	}
	m.modalBusy = true
	m.modalErr = ""
	create := m.options.CreateAdmin
	return m, func() tea.Msg {
		if err := create(username, password); err != nil {
			return adminResultMsg{err: err}
		}
		return adminResultMsg{username: username}
	}
}

func (m *Model) openLoginModal() {
	if m.status.Running {
		m.setNotice("请先停止房间，再登录账号")
		return
	}
	m.modal = modalLogin
	m.loginEmail = nil
	m.loginEmailCursor = 0
	m.loginPassword = nil
	m.loginPasswordCursor = 0
	m.loginPlatform = string(netease.AuthInterop)
	m.loginField = 0
	m.loginVerifyURL = ""
	m.loginVerifyNote = ""
	m.modalBusy = false
	m.modalErr = ""
	m.contentOffset = 0
	m.focus = focusContent
}

func (m *Model) openNameModal(index int, token, method string) {
	m.modal = modalName
	m.nameIndex = index
	m.nameToken = token
	m.nameMethod = method
	m.nameInput = nil
	m.nameCursor = 0
	m.modalBusy = false
	m.modalErr = ""
	m.contentOffset = 0
	m.focus = focusContent
}

func (m Model) submitLogin() (tea.Model, tea.Cmd) {
	email := strings.TrimSpace(string(m.loginEmail))
	password := string(m.loginPassword)
	if email == "" || password == "" {
		m.modalErr = "请填写账号和密码"
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
	m.loginVerifyURL = ""
	m.loginVerifyNote = ""
	manager := m.manager
	profileID := m.status.Config.DefaultProfileID()
	platform := m.loginPlatform
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		var (
			sauth  map[string]any
			err    error
			method = netease.AuthInterop
		)
		if platform == string(netease.AuthMobileNetEase) {
			service := netease.NewMobileLoginService(manager.Dir())
			sauth, err = service.Login(ctx, email, password)
			method = netease.AuthMobileNetEase
			if err != nil {
				var verify *netease.MobileVerifyRequired
				if errors.As(err, &verify) {
					return loginResultMsg{verifyURL: verify.URL, verifyNote: verify.Message}
				}
				return loginResultMsg{err: fmt.Errorf("%s", sanitizeLoginError(err, email, password))}
			}
		} else {
			service := interoplogin.New(manager.Dir())
			sauth, err = service.Login(ctx, email, password)
			if err != nil {
				var verify *interoplogin.VerifyRequired
				if errors.As(err, &verify) {
					return loginResultMsg{verifyURL: verify.URL, verifyNote: verify.Message}
				}
				return loginResultMsg{err: fmt.Errorf("%s", sanitizeLoginError(err, email, password))}
			}
		}
		inner, err := json.Marshal(sauth)
		if err != nil {
			return loginResultMsg{err: errors.New("序列化登录凭证失败")}
		}
		token, resolved, expiresAt, err := core.NormalizeSession(string(inner), string(method))
		if err != nil {
			return loginResultMsg{err: err}
		}
		profile, profileErr := auth.LookupProfileFor(ctx, netease.AuthMethod(resolved), token)
		needName := false
		profileError := ""
		if profileErr != nil {
			if errors.Is(profileErr, netease.ErrProfileNameMissing) {
				needName = true
				profileError = netease.ErrProfileNameMissing.Error()
			} else {
				return loginResultMsg{err: fmt.Errorf("%s", sanitizeImportError(profileErr, token))}
			}
		}
		index, err := addSessionAccount(manager, token, resolved, expiresAt, profileID, profile, profileError)
		if err != nil {
			return loginResultMsg{err: fmt.Errorf("%s", sanitizeImportError(err, token))}
		}
		name := profile.Name
		if name == "" {
			name = "未命名账号"
		}
		return loginResultMsg{
			index: index, name: name, needName: needName,
			token: token, method: resolved,
		}
	}
}

func (m Model) submitName() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(string(m.nameInput))
	if name == "" {
		m.modalErr = "请填写游戏名"
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
	token := m.nameToken
	method := m.nameMethod
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
		defer cancel()
		profile, err := auth.SetPlayerName(ctx, netease.AuthMethod(method), token, name)
		if err != nil {
			return nameResultMsg{err: fmt.Errorf("%s", sanitizeImportError(err, token))}
		}
		return nameResultMsg{profile: profile}
	}
}

func addSessionAccount(manager *core.Manager, token, method, expiresAt, profileID string, profile auth.Profile, profileError string) (int, error) {
	meta := core.AccountMeta{
		Name: profile.Name, UID: profile.UID, Level: profile.Level,
		AvatarURL: profile.AvatarURL, Platform: method, AuthMethod: method,
		ExpiresAt: expiresAt, ProfileError: profileError, ProfileID: profileID,
	}
	return manager.AddAccount(token, meta)
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
		index, err := addSessionAccount(manager, token, method, expiresAt, profileID, profile, profileError)
		if err != nil {
			return importResultMsg{err: fmt.Errorf("%s", sanitizeImportError(err, token))}
		}
		name := profile.Name
		if name == "" {
			name = "未命名账号"
		}
		return importResultMsg{index: index, name: name, needName: needName, token: token, method: method}
	}
}

func sanitizeImportError(err error, token string) string {
	message := err.Error()
	if token != "" {
		message = strings.ReplaceAll(message, token, "[会话]")
	}
	return message
}

func sanitizeLoginError(err error, email, password string) string {
	message := err.Error()
	for _, secret := range []string{password, email} {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[账号]")
		}
	}
	return message
}

func (m *Model) openWebUI() {
	m.openWebUIURL(m.options.LocalURL)
	m.setNotice("已尝试打开 WebUI 地址")
}

func (m *Model) openWebUIURL(url string) {
	if m.options.OpenURL == nil || url == "" {
		return
	}
	m.options.OpenURL(url)
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
	case pageProfiles:
		count := len(m.status.Config.Profiles)
		if count > 0 {
			m.profileIndex = clampInt(m.profileIndex+delta, 0, count-1)
			m.scrollProfileList()
		}
	default:
		m.moveContent(delta)
	}
}

func (m *Model) moveContent(delta int) {
	m.contentOffset += delta
	if m.contentOffset < 0 {
		m.contentOffset = 0
	}
	if m.contentOffset > 5000 {
		m.contentOffset = 5000
	}
}

func (m *Model) scrollProfileList() {
	info := m.layoutInfo()
	height := info.content.H
	if height < 1 {
		return
	}
	line := m.profileIndex * 2
	if line < m.contentOffset {
		m.contentOffset = line
	}
	if line+2 > m.contentOffset+height {
		m.contentOffset = line - height + 2
	}
	if m.contentOffset < 0 {
		m.contentOffset = 0
	}
}

func (m *Model) scrollPage(delta int) {
	if m.modal != modalNone {
		m.moveContent(delta)
		return
	}
	switch m.page {
	case pageLogs:
		m.focus = focusContent
		m.scrollLogs(-delta)
	case pageAccounts:
		m.focus = focusContent
		count := len(m.status.Accounts)
		if count > 0 {
			m.selected = clampInt(m.selected+delta, 0, count-1)
			m.clamp()
		}
	default:
		m.focus = focusContent
		m.moveContent(delta)
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
	case pageProfiles:
		m.profileIndex = 0
		m.contentOffset = 0
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
	case pageProfiles:
		if count := len(m.status.Config.Profiles); count > 0 {
			m.profileIndex = count - 1
			m.scrollProfileList()
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
	profileCount := len(m.status.Config.Profiles)
	if profileCount == 0 {
		m.profileIndex = 0
	} else {
		m.profileIndex = clampInt(m.profileIndex, 0, profileCount-1)
	}
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
