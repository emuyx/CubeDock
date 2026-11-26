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
	"github.com/emuyx/cubedock/internal/friends"
	"github.com/emuyx/cubedock/internal/interoplogin"
	"github.com/emuyx/cubedock/internal/m4399login"
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
	index       int
	name        string
	needName    bool
	token       string
	method      string
	verifyURL   string
	verifyNote  string
	captchaID   string
	captchaPath string
	captchaNote string
	err         error
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
type accountActionResultMsg struct {
	notice string
	err    error
}

type consoleLine struct {
	time  string
	level string
	text  string
}

type consoleResultMsg struct {
	output []string
	err    error
}

type playersMsg struct {
	players []core.PlayerStatus
	notice  string
	err     error
}

type membersMsg struct {
	members []core.RoomMemberStatus
	notice  string
	err     error
}

type friendsMsg struct {
	requests []friends.Request
	notice   string
	err      error
}

type packsMsg struct {
	list   core.ResourcePackList
	notice string
	err    error
}

type packsPullMsg struct {
	result core.ResourcePackRefresh
	err    error
}

type adminPasswordMsg struct {
	err error
}

type bulkLineResult struct {
	status  string // ok / skipped
	message string
}

type bulkLineMsg struct {
	index      int
	savedIndex int
	name       string
	needName   bool
	err        error
}

type dashboardDiagnosticsMsg struct {
	checks []core.DiagnosticCheck
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
	modalAccountActions
	modalAssignProfile
	modalPlayers
	modalRoomMembers
	modalFriends
	modalPlayerTransfer
	modalResourcePacks
	modalSettings
	modalAdminPassword
	modalBulkImport
	modalAccountDashboard
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

	consoleMode       bool
	consoleInput      []rune
	consoleCursor     int
	consoleHistory    []string
	consoleHistoryPos int
	consoleTranscript []consoleLine
	consoleBusy       bool

	logQuery       []rune
	logQueryCursor int
	logSearch      bool

	players       []core.PlayerStatus
	playersCursor int
	playersBusy   bool
	playersErr    string
	playersNotice string

	members       []core.RoomMemberStatus
	membersCursor int
	membersBusy   bool
	membersErr    string
	membersNotice string

	friendRequests []friends.Request
	friendsCursor  int
	friendsBusy    bool
	friendsErr     string
	friendsNotice  string

	transferTarget  string
	transferAddress []rune
	transferCursor  int

	packsProfileID   string
	packsProfileName string
	packs            core.ResourcePackList
	packsCursor      int
	packsBusy        bool
	packsErr         string
	packsNotice      string

	settingsDraft     config.Config
	settingsRevision  uint64
	settingsField     int
	settingsFieldEdit bool
	settingsBuffer    []rune
	settingsCursor    int
	settingsError     string

	pwdCurrent       []rune
	pwdCurrentCursor int
	pwdNext          []rune
	pwdNextCursor    int
	pwdConfirm       []rune
	pwdConfirmCursor int
	pwdField         int

	bulkLines   []string
	bulkResults []bulkLineResult
	bulkIndex   int
	bulkCancel  bool
	bulkNotice  string
	bulkProfile string

	accountQuery       []rune
	accountQueryCursor int
	accountSearch      bool
	accountFilter      string
	accountSort        string

	dashboardChecks []core.DiagnosticCheck
	dashboardBusy   bool
	dashboardErr    string
	dashboardAt     time.Time

	modal       modalKind
	modalReturn modalKind
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
	loginReplace        bool
	loginReplaceIndex   int
	loginRemember       bool
	loginCaptchaID      string
	loginCaptchaPath    string
	loginCaptchaNote    string
	loginCaptchaInput   []rune
	loginCaptchaCursor  int
	login4399           *m4399login.Service

	accountActionIndex  int
	accountActionCursor int
	accountFingerprint  string
	accountDeleteArmed  bool
	accountDeleteAt     time.Time
	assignProfileCursor int

	nameIndex         int
	nameToken         string
	nameMethod        string
	nameFingerprint   string
	nameInput         []rune
	nameCursor        int
	nameRename        bool
	nameSuggestionsCN []string
	nameSuggestionsEN []string

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
	profileRevision   uint64
	restoreRevision   uint64
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
	model := Model{
		options: options, manager: options.Manager,
		page: pageHome, focus: focusNav,
		login4399: m4399login.New(),
	}
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
		m.closeToReturn()
		m.setNotice(fmt.Sprintf("已添加账号 %s（房间 %d）", message.name, message.index+1))
		return m, m.snapshotCmd()
	case loginResultMsg:
		m.modalBusy = false
		if message.err != nil {
			m.modalErr = message.err.Error()
			return m, nil
		}
		if message.captchaID != "" {
			m.loginCaptchaID = message.captchaID
			m.loginCaptchaPath = message.captchaPath
			m.loginCaptchaNote = message.captchaNote
			m.loginCaptchaInput = nil
			m.loginCaptchaCursor = 0
			m.loginField = 4
			m.modalErr = ""
			return m, nil
		}
		if message.verifyURL != "" || message.verifyNote != "" {
			m.loginVerifyURL = message.verifyURL
			m.loginVerifyNote = message.verifyNote
			return m, nil
		}
		m.loginCaptchaID = ""
		m.loginCaptchaPath = ""
		m.loginCaptchaNote = ""
		m.loginCaptchaInput = nil
		m.loginCaptchaCursor = 0
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
		m.closeToReturn()
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
		if m.nameFingerprint != "" {
			live := m.manager.Accounts()
			if index < 0 || index >= len(live) || accountIdentity(live[index]) != m.nameFingerprint {
				m.modalErr = "账号列表已变化（可能由 WebUI 操作），请关闭后重新打开"
				return m, nil
			}
		}
		if err := m.manager.UpdateAccountProfile(index, message.profile.Name, message.profile.UID,
			message.profile.Level, message.profile.AvatarURL, ""); err != nil {
			m.modalErr = "保存账号资料失败：" + err.Error()
			return m, nil
		}
		m.closeToReturn()
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
		m.closeToReturn()
		m.setNotice("管理员账户已创建：" + message.username)
		return m, nil
	case restoreResultMsg:
		m.modalBusy = false
		if message.err != nil {
			m.modalErr = message.err.Error()
			return m, nil
		}
		m.closeToReturn()
		m.input = nil
		m.inputCursor = 0
		m.page = pageSettings
		if len(message.warnings) > 0 {
			m.setNotice("配置已恢复；" + strings.Join(message.warnings, "；"))
		} else {
			m.setNotice("配置已恢复：" + message.path)
		}
		return m, m.snapshotCmd()
	case accountActionResultMsg:
		m.modalBusy = false
		if message.err != nil {
			m.modalErr = message.err.Error()
			return m, nil
		}
		if m.modal == modalAccountDashboard {
			m.setNotice(message.notice)
			return m, m.snapshotCmd()
		}
		m.modal = modalNone
		m.accountDeleteArmed = false
		m.setNotice(message.notice)
		return m, m.snapshotCmd()
	case consoleResultMsg:
		m.consoleBusy = false
		now := time.Now().Format("15:04:05")
		if message.err != nil {
			m.consoleTranscript = append(m.consoleTranscript, consoleLine{time: now, level: "error", text: message.err.Error()})
		} else if len(message.output) == 0 {
			m.consoleTranscript = append(m.consoleTranscript, consoleLine{time: now, level: "info", text: "（无输出）"})
		} else {
			for _, line := range message.output {
				m.consoleTranscript = append(m.consoleTranscript, consoleLine{time: now, level: "info", text: line})
			}
		}
		m.trimConsoleTranscript()
		m.logOffset = 0
		return m, nil
	case playersMsg:
		m.playersBusy = false
		m.players = message.players
		m.playersCursor = clampInt(m.playersCursor, 0, maxInt(0, len(m.players)-1))
		m.playersErr = ""
		if message.err != nil {
			m.playersErr = message.err.Error()
			if m.modal == modalPlayerTransfer {
				m.modalErr = message.err.Error()
			}
		} else if m.modal == modalPlayerTransfer {
			m.modal = modalPlayers
		}
		m.playersNotice = message.notice
		return m, nil
	case membersMsg:
		m.membersBusy = false
		m.members = message.members
		m.membersCursor = clampInt(m.membersCursor, 0, maxInt(0, len(m.members)-1))
		m.membersErr = ""
		if message.err != nil {
			m.membersErr = message.err.Error()
		}
		m.membersNotice = message.notice
		return m, nil
	case friendsMsg:
		m.friendsBusy = false
		m.friendRequests = message.requests
		m.friendsCursor = clampInt(m.friendsCursor, 0, maxInt(0, len(m.friendRequests)-1))
		m.friendsErr = ""
		if message.err != nil {
			m.friendsErr = message.err.Error()
		}
		m.friendsNotice = message.notice
		return m, nil
	case packsMsg:
		m.packsBusy = false
		m.packs = message.list
		m.packsCursor = clampInt(m.packsCursor, 0, maxInt(0, len(m.packs.Packs)-1))
		m.packsErr = ""
		if message.err != nil {
			m.packsErr = message.err.Error()
		}
		m.packsNotice = message.notice
		return m, nil
	case packsPullMsg:
		m.packsBusy = false
		if message.err != nil {
			m.packsErr = message.err.Error()
			return m, nil
		}
		m.packs = message.result.ResourcePackList
		m.packsCursor = clampInt(m.packsCursor, 0, maxInt(0, len(m.packs.Packs)-1))
		m.packsErr = ""
		m.packsNotice = fmt.Sprintf("拉取完成：新增 %d，跳过 %d", message.result.Pulled, message.result.Skipped)
		if len(message.result.Warnings) > 0 {
			m.packsNotice += "；" + strings.Join(message.result.Warnings, "；")
		}
		return m, nil
	case adminPasswordMsg:
		m.modalBusy = false
		if message.err != nil {
			m.modalErr = message.err.Error()
			return m, nil
		}
		m.closeToReturn()
		m.setNotice("管理员密码已修改；WebUI 会话已失效")
		return m, nil
	case bulkLineMsg:
		if message.index >= 0 && message.index < len(m.bulkResults) {
			result := bulkLineResult{status: "ok"}
			switch {
			case message.err != nil:
				result = bulkLineResult{status: "error", message: message.err.Error()}
			case message.needName:
				result = bulkLineResult{status: "name", message: fmt.Sprintf("房间 %d · 需要设置游戏名", message.savedIndex+1)}
			default:
				result.message = fmt.Sprintf("房间 %d · %s", message.savedIndex+1, message.name)
			}
			m.bulkResults[message.index] = result
		}
		m.bulkIndex = message.index + 1
		if m.bulkCancel || m.bulkIndex >= len(m.bulkLines) {
			for index := m.bulkIndex; index < len(m.bulkResults); index++ {
				if m.bulkResults[index].status == "pending" || m.bulkResults[index].status == "running" {
					m.bulkResults[index] = bulkLineResult{status: "skipped", message: "已停止"}
				}
			}
			m.modalBusy = false
			m.bulkNotice = bulkSummary(m.bulkResults)
			return m, m.snapshotCmd()
		}
		m.bulkResults[m.bulkIndex].status = "running"
		return m, m.bulkNextCmd()
	case dashboardDiagnosticsMsg:
		m.dashboardBusy = false
		m.dashboardChecks = message.checks
		m.dashboardAt = time.Now()
		return m, nil
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

	if m.logSearch {
		return m.handleLogSearchKey(key)
	}
	if m.accountSearch {
		return m.handleAccountSearchKey(key)
	}
	if m.consoleMode {
		return m.handleConsoleKey(key)
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
		} else if m.page == pageAccounts {
			m.accountFilter = cycleAccountFilter(m.accountFilterMode())
			m.normalizeAccountSelection()
			m.setNotice("账号筛选：" + accountFilterLabel(m.accountFilterMode()))
		}
	case "v":
		if m.page == pageAccounts {
			m.accountSort = cycleAccountSort(m.accountSortMode())
			m.normalizeAccountSelection()
			m.setNotice("账号排序：" + accountSortLabel(m.accountSortMode()))
		}
	case "/":
		if m.page == pageAccounts {
			m.accountSearch = true
			m.accountQueryCursor = len(m.accountQuery)
			m.focus = focusContent
		} else if m.page == pageLogs {
			m.logSearch = true
			m.logQueryCursor = len(m.logQuery)
			m.focus = focusContent
		}
	case "e":
		if m.page == pageLogs && !m.consoleMode {
			m.exportLogs()
		}
	case "o":
		m.openWebUI()
	case "i", ":":
		if m.page == pageLogs {
			m.enterConsole()
		}
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
	case "m":
		if m.page == pageAccounts {
			m.openAccountActions()
		}
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
		switch {
		case event.Button == tea.MouseButtonWheelUp:
			m.moveContent(-3)
		case event.Button == tea.MouseButtonWheelDown:
			m.moveContent(3)
		case event.Action == tea.MouseActionPress && event.Button == tea.MouseButtonLeft:
			info := m.layoutInfo()
			if !m.modalBusy {
				for index, area := range info.navRects {
					if area.Contains(event.X, event.Y) {
						if m.profileEditing {
							m.setNotice("请先保存或取消配置方案")
							return m, nil
						}
						m.cancelModal()
						// Transfer cancellation returns to the player list; a
						// sidebar page click must leave every modal behind.
						m.modal = modalNone
						if index < len(pageOrder) {
							m.setPage(pageOrder[index])
							m.focus = focusNav
						}
						return m, nil
					}
				}
				for row, area := range info.accountRects {
					if area.Contains(event.X, event.Y) && row < len(info.accountRowIndex) {
						if m.profileEditing {
							m.setNotice("请先保存或取消配置方案")
							return m, nil
						}
						m.openDashboardFor(info.accountRowIndex[row])
						return m, nil
					}
				}
			}
			if hit, ok := hitAt(info.pageHits, event.X, event.Y); ok {
				return m.activatePageHit(hit)
			}
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
	for row, area := range info.accountRects {
		if area.Contains(m.mouseX, m.mouseY) && row < len(info.accountRowIndex) {
			if m.profileEditing {
				m.setNotice("请先保存或取消配置方案")
				return m, nil
			}
			m.openDashboardFor(info.accountRowIndex[row])
			return m, nil
		}
	}
	if hit, ok := hitAt(info.pageHits, m.mouseX, m.mouseY); ok {
		return m.activatePageHit(hit)
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

func hitAt(hits []pageHit, x, y int) (pageHit, bool) {
	for _, hit := range hits {
		if hit.Contains(x, y) {
			return hit, true
		}
	}
	return pageHit{}, false
}

// activatePageHit runs whatever is under the mouse: an in-page button, a
// selectable row, a form field or a modal control.
func (m Model) activatePageHit(hit pageHit) (tea.Model, tea.Cmd) {
	if m.modal != modalNone && m.modalBusy {
		// Keyboard input is ignored while a modal request is in flight; make
		// mouse clicks behave the same so a late result cannot act on a
		// modal the user just dismissed.
		return m, nil
	}
	switch hit.kind {
	case hitToolbar:
		if hit.index < 0 || hit.index >= len(m.toolbarItems()) {
			return m, nil
		}
		m.focus = focusActions
		m.actionIndex = hit.index
		return m.activateToolbar(hit.index)
	case hitProfileSelect:
		m.profileIndex = hit.index
		m.focus = focusContent
	case hitProfileField:
		m.clickProfileField(hit.index)
	case hitJumpRoom:
		m.setPage(pageAccounts)
		m.selected = hit.index
		m.focus = focusContent
	case hitCreateAdmin:
		m.openAdminModal()
	case hitOpenWebUI:
		m.openWebUI()
	case hitToggleLogFilter:
		if m.page == pageLogs {
			m.logWatch = !m.logWatch
			m.logOffset = 0
			m.setNotice("日志筛选：" + m.logFilterLabel())
		}
	case hitConsoleInput:
		if m.page == pageLogs {
			m.enterConsole()
		}
	case hitLogSearch:
		if m.page == pageLogs {
			m.logSearch = true
			m.logQueryCursor = len(m.logQuery)
			m.focus = focusContent
		}
	case hitLogExport:
		if m.page == pageLogs {
			m.exportLogs()
		}
	case hitLoginPlatform:
		switch hit.index {
		case 1:
			m.setLoginPlatform(string(netease.AuthMobileNetEase))
		case 2:
			m.setLoginPlatform(string(netease.AuthMobile4399))
		default:
			m.setLoginPlatform(string(netease.AuthInterop))
		}
		m.loginField = 2
	case hitLoginField:
		m.loginField = clampInt(hit.index, 0, 4)
	case hitNameRandomCN:
		m.nameInput = []rune(randomChineseName())
		m.nameCursor = len(m.nameInput)
		m.modalErr = ""
	case hitNameRandomEN:
		m.nameInput = []rune(randomEnglishName())
		m.nameCursor = len(m.nameInput)
		m.modalErr = ""
	case hitNameRefresh:
		m.nameSuggestionsCN, m.nameSuggestionsEN = makeNameSuggestions(3)
	case hitNameCandidateCN:
		if hit.index >= 0 && hit.index < len(m.nameSuggestionsCN) {
			m.nameInput = []rune(m.nameSuggestionsCN[hit.index])
			m.nameCursor = len(m.nameInput)
			m.modalErr = ""
		}
	case hitNameCandidateEN:
		if hit.index >= 0 && hit.index < len(m.nameSuggestionsEN) {
			m.nameInput = []rune(m.nameSuggestionsEN[hit.index])
			m.nameCursor = len(m.nameInput)
			m.modalErr = ""
		}
	case hitLoginRemember:
		m.loginField = 3
		m.loginRemember = !m.loginRemember
	case hitOpenLoginURL:
		switch {
		case m.loginVerifyURL != "":
			m.openWebUIURL(m.loginVerifyURL)
		case m.loginCaptchaPath != "":
			m.openWebUIURL(m.loginCaptchaPath)
		}
	case hitAdminField:
		m.adminField = clampInt(hit.index, 0, 2)
	case hitAccountSearch:
		m.accountSearch = true
		m.accountQueryCursor = len(m.accountQuery)
		m.focus = focusContent
	case hitAccountFilter:
		m.accountFilter = cycleAccountFilter(m.accountFilterMode())
		m.normalizeAccountSelection()
	case hitAccountSort:
		m.accountSort = cycleAccountSort(m.accountSortMode())
		m.normalizeAccountSelection()
	case hitAccountName:
		m.rememberReturn()
		return m.openAccountName()
	case hitBulkStart:
		return m.startBulkImport()
	case hitAccountDashboard:
		if m.modal == modalAccountActions || m.modal == modalAccountDashboard {
			m.openAccountDashboard()
		} else {
			m.openSelectedDashboard()
		}
	case hitDashboardStart:
		return m.dashboardRoomAction(actionStart)
	case hitDashboardStop:
		return m.dashboardRoomAction(actionStop)
	case hitDashboardRestart:
		return m.dashboardRoomAction(actionRestart)
	case hitDashboardDiagnostics:
		return m.runDashboardDiagnostics()
	case hitSettingsField:
		m.settingsField = hit.index
		m.settingsFieldEdit = false
		m.settingsError = ""
	case hitAdminPasswordField:
		m.pwdField = clampInt(hit.index, 0, 2)
	case hitAccountRefresh, hitAccountRelogin, hitAccountAssign, hitAccountDelete:
		m.accountActionCursor = hit.index
		return m.activateAccountAction(hit.kind)
	case hitAssignProfile:
		m.assignProfileCursor = hit.index
		return m.applyAssignProfile(hit.index)
	case hitPlayersOpen, hitMembersOpen, hitFriendsOpen:
		m.accountActionCursor = hit.index
		return m.activateAccountAction(hit.kind)
	case hitPlayerSelect:
		m.playersCursor = hit.index
	case hitMemberSelect:
		m.membersCursor = hit.index
	case hitFriendSelect:
		m.friendsCursor = hit.index
	case hitPackSelect:
		m.packsCursor = hit.index
	case hitPlayerKick:
		return m.kickSelectedPlayer()
	case hitPlayerTransfer:
		m.openPlayerTransfer()
	case hitPlayerRefresh:
		m.openPlayers()
	case hitMemberKick:
		return m.kickSelectedMember()
	case hitMemberRefresh:
		return m.refreshMembers()
	case hitFriendAccept:
		return m.acceptSelectedFriend()
	case hitFriendRefresh:
		return m.openFriends()
	case hitTransferSend:
		return m.submitTransfer()
	case hitPacksRefresh:
		return m.refreshPacks()
	case hitPacksPull:
		return m.pullPacks()
	case hitTagToggle:
		m.toggleTag(hit.index)
	case hitRoomNamePick:
		if hit.index >= 0 && hit.index < len(profileRoomNames) {
			m.profileDraft.RoomName = profileRoomNames[hit.index]
			m.profileDirty = true
			m.modal = modalNone
			m.profileScrollToField()
		}
	case hitModalSubmit:
		return m.submitModal()
	case hitModalCancel:
		m.cancelModal()
	}
	return m, nil
}

func (m *Model) clickProfileField(index int) {
	fields := m.profileFields()
	if index < 0 || index >= len(fields) {
		return
	}
	// Moving focus with the mouse commits the field being edited first, just
	// like pressing Enter would.
	if m.profileFieldEdit && m.profileField >= 0 && m.profileField < len(fields) {
		if err := m.setProfileFieldText(fields[m.profileField], string(m.profileBuffer)); err != nil {
			m.profileError = err.Error()
			return
		}
		m.profileDirty = true
	}
	m.profileField = index
	m.profileFieldEdit = false
	m.profileBuffer = nil
	m.profileError = ""
	m.focus = focusContent
	spec := fields[index]
	switch spec.Kind {
	case profileText, profileNumber:
		m.profileFieldEdit = true
		m.profileBuffer = []rune(m.profileFieldValue(spec))
		m.profileCursor = len(m.profileBuffer)
	case profileToggle:
		m.toggleProfileField(spec)
	case profileChoice:
		m.adjustProfileField(spec, 1)
	case profileTags:
		m.openTagsModal()
	}
}

func (m Model) submitModal() (tea.Model, tea.Cmd) {
	switch m.modal {
	case modalLogin:
		return m.submitLogin()
	case modalName:
		return m.submitName()
	case modalAdmin:
		return m.submitAdmin()
	case modalRestore:
		return m.submitRestore()
	case modalImportSauth:
		return m.submitImport()
	}
	return m, nil
}

// rememberReturn records dashboard/account-management as the parent of a child
// modal so closing the child comes back to where the user started.
func (m *Model) rememberReturn() {
	switch m.modal {
	case modalAccountDashboard, modalAccountActions:
		m.modalReturn = m.modal
	default:
		m.modalReturn = modalNone
	}
}

func (m *Model) closeToReturn() {
	if m.modalReturn != modalNone {
		m.modal = m.modalReturn
		m.modalReturn = modalNone
	} else {
		m.modal = modalNone
	}
	m.modalErr = ""
}

func (m *Model) cancelModal() {
	if m.modal == modalPlayerTransfer {
		// Transfer is a nested step of the player list; the player list keeps
		// the dashboard/account-management return target.
		m.modal = modalPlayers
		m.modalErr = ""
		return
	}
	returnTo := m.modalReturn
	m.modalReturn = modalNone
	switch m.modal {
	case modalImportSauth, modalRestore:
		m.modal = modalNone
		m.input = nil
		m.inputCursor = 0
		m.modalErr = ""
	case modalLogin:
		m.modal = modalNone
		m.modalErr = ""
		m.loginVerifyURL = ""
		m.loginVerifyNote = ""
	case modalName:
		m.modal = modalNone
		m.modalErr = ""
		m.nameInput = nil
		m.nameCursor = 0
		if m.nameRename {
			m.setNotice("已取消修改游戏名")
		} else {
			m.setNotice("账号已添加；可以稍后设置游戏名")
		}
	case modalAdmin:
		m.modal = modalNone
		m.modalErr = ""
	case modalProfileTags, modalProfileRoomName:
		m.modal = modalNone
		m.modalErr = ""
		m.profileScrollToField()
	case modalAccountActions, modalAssignProfile:
		m.modal = modalNone
		m.modalErr = ""
		m.accountDeleteArmed = false
	case modalPlayerTransfer:
		m.modal = modalPlayers
		m.modalErr = ""
	case modalPlayers, modalRoomMembers, modalFriends, modalResourcePacks:
		m.modal = modalNone
		m.modalErr = ""
	case modalSettings, modalAdminPassword, modalAccountDashboard, modalBulkImport:
		m.modal = modalNone
		m.modalErr = ""
	}
	m.modalErr = ""
	if returnTo != modalNone {
		m.modal = returnTo
		m.accountDeleteArmed = false
	}
}

var accountActionSpecs = []customButton{
	{label: "刷新资料", kind: hitAccountRefresh},
	{label: "重新登录", kind: hitAccountRelogin},
	{label: "设置游戏名", kind: hitAccountName},
	{label: "换方案", kind: hitAccountAssign},
	{label: "玩家", kind: hitPlayersOpen},
	{label: "成员", kind: hitMembersOpen},
	{label: "好友", kind: hitFriendsOpen},
	{label: "仪表盘", kind: hitAccountDashboard},
	{label: "删除账号", kind: hitAccountDelete},
	{label: "关闭", kind: hitModalCancel},
}

// accountIdentity is a stable fingerprint used to detect an account list that
// shifted under an open modal (for example because the WebUI removed or
// reordered an account). Index-only actions could otherwise hit the wrong room.
func accountIdentity(account core.AccountStatus) string {
	switch {
	case account.UID != "":
		return "uid:" + account.UID + "|" + account.AuthMethod
	case account.Email != "":
		return "email:" + account.Email + "|" + account.AuthMethod
	default:
		return "name:" + account.Name + "|" + account.AuthMethod
	}
}

func (m Model) accountActionStillValid() bool {
	if m.manager == nil || m.accountFingerprint == "" {
		return true
	}
	index := m.accountActionIndex
	live := m.manager.Accounts()
	if index < 0 || index >= len(live) {
		return false
	}
	return accountIdentity(live[index]) == m.accountFingerprint
}

func (m *Model) openAccountActions() {
	index := m.selectedIndex()
	if index < 0 {
		if len(m.status.Accounts) > 0 {
			m.setNotice("没有匹配的账号，请按 / 搜索或 f 筛选")
		} else {
			m.setNotice("还没有账号")
		}
		return
	}
	accounts := m.status.Accounts
	if m.manager != nil {
		accounts = m.manager.Accounts()
	}
	if index >= len(accounts) {
		m.setNotice("账号列表已变化，请重新选择")
		return
	}
	m.modalReturn = modalNone
	m.accountActionIndex = index
	m.accountFingerprint = accountIdentity(accounts[index])
	m.accountActionCursor = 0
	m.accountDeleteArmed = false
	m.modalBusy = false
	m.modalErr = ""
	m.modal = modalAccountActions
	m.contentOffset = 0
	m.focus = focusContent
}

func (m Model) handleAccountActionsKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.cancelModal()
	case "up", "k":
		m.accountActionCursor = (m.accountActionCursor + len(accountActionSpecs) - 1) % len(accountActionSpecs)
	case "down", "j":
		m.accountActionCursor = (m.accountActionCursor + 1) % len(accountActionSpecs)
	case "enter", " ":
		return m.activateAccountAction(accountActionSpecs[m.accountActionCursor].kind)
	}
	return m, nil
}

func (m Model) activateAccountAction(kind pageHitKind) (tea.Model, tea.Cmd) {
	if kind != hitAccountDelete {
		m.accountDeleteArmed = false
	}
	if kind != hitModalCancel && !m.accountActionStillValid() {
		m.modalErr = "账号列表已变化（可能由 WebUI 操作），请关闭后重新打开"
		return m, nil
	}
	switch kind {
	case hitAccountRefresh:
		return m.refreshAccount()
	case hitAccountRelogin:
		m.rememberReturn()
		m.openLoginModalFor(m.accountActionIndex)
	case hitAccountName:
		m.rememberReturn()
		return m.openAccountName()
	case hitAccountDashboard:
		m.openAccountDashboard()
	case hitAccountAssign:
		m.rememberReturn()
		m.openAssignProfile(m.accountActionIndex)
	case hitPlayersOpen:
		m.rememberReturn()
		m.openPlayers()
	case hitMembersOpen:
		m.rememberReturn()
		return m.openRoomMembers()
	case hitFriendsOpen:
		m.rememberReturn()
		return m.openFriends()
	case hitAccountDelete:
		if !m.accountDeleteArmed {
			m.accountDeleteArmed = true
			m.accountDeleteAt = time.Now()
			m.modalErr = ""
			return m, nil
		}
		return m.removeSelectedAccount()
	case hitModalCancel:
		m.cancelModal()
	}
	return m, nil
}

func (m Model) refreshAccount() (tea.Model, tea.Cmd) {
	manager := m.manager
	index := m.accountActionIndex
	if manager == nil {
		m.modalErr = "房间管理器不可用"
		return m, nil
	}
	if !m.accountActionStillValid() {
		m.modalErr = "账号列表已变化（可能由 WebUI 操作），请关闭后重新打开"
		return m, nil
	}
	if m.status.Running {
		m.modalErr = "请先停止房间，再刷新资料"
		return m, nil
	}
	accounts := manager.Accounts()
	if index < 0 || index >= len(accounts) {
		m.modalErr = "账号序号无效"
		return m, nil
	}
	token, err := manager.Token(index)
	if err != nil {
		m.modalErr = err.Error()
		return m, nil
	}
	method, err := netease.ParseAuthMethod(accounts[index].AuthMethod)
	if err != nil {
		m.modalErr = "账号登录方式无效"
		return m, nil
	}
	if m.modalBusy {
		return m, nil
	}
	m.modalBusy = true
	m.modalErr = ""
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		profile, err := auth.LookupProfileFor(ctx, method, token)
		if err != nil {
			message := sanitizeImportError(err, token)
			_ = manager.UpdateAccountProfile(index, "", "", 0, "", message)
			return accountActionResultMsg{err: fmt.Errorf("%s", message)}
		}
		if err := manager.UpdateAccountProfile(index, profile.Name, profile.UID, profile.Level, profile.AvatarURL, ""); err != nil {
			return accountActionResultMsg{err: err}
		}
		manager.Record("账号", fmt.Sprintf("房间 %d 游戏名解析成功：%s（UID %s）", index+1, profile.Name, profile.UID))
		return accountActionResultMsg{notice: "账号资料已刷新：" + profile.Name}
	}
}

func (m Model) removeSelectedAccount() (tea.Model, tea.Cmd) {
	manager := m.manager
	index := m.accountActionIndex
	if manager == nil {
		m.modalErr = "房间管理器不可用"
		return m, nil
	}
	if !m.accountActionStillValid() {
		m.modalErr = "账号列表已变化（可能由 WebUI 操作），请关闭后重新打开"
		return m, nil
	}
	if m.status.Running {
		m.modalErr = "请先停止房间，再删除账号"
		return m, nil
	}
	if m.modalBusy {
		return m, nil
	}
	m.modalBusy = true
	m.modalErr = ""
	return m, func() tea.Msg {
		if err := manager.RemoveAccount(index); err != nil {
			return accountActionResultMsg{err: err}
		}
		manager.Record("账号", fmt.Sprintf("已移除房间 %d 的网易账号", index+1))
		return accountActionResultMsg{notice: fmt.Sprintf("已移除房间 %d 的账号", index+1)}
	}
}

func (m *Model) openAssignProfile(index int) {
	profiles := m.status.Config.Profiles
	if len(profiles) == 0 {
		m.setNotice("还没有配置方案，请先在「配置方案」页新增")
		return
	}
	m.accountActionIndex = index
	if !m.accountActionStillValid() {
		m.modalErr = "账号列表已变化（可能由 WebUI 操作），请关闭后重新打开"
		m.modal = modalAccountActions
		return
	}
	m.accountDeleteArmed = false
	m.assignProfileCursor = 0
	current := ""
	if index >= 0 && index < len(m.status.Accounts) {
		current = m.status.Accounts[index].ProfileID
	}
	if current == "" {
		current = m.status.Config.DefaultProfileID()
	}
	for position, profile := range profiles {
		if profile.ID == current {
			m.assignProfileCursor = position
			break
		}
	}
	m.modal = modalAssignProfile
	m.modalBusy = false
	m.modalErr = ""
	m.contentOffset = 0
	m.focus = focusContent
}

func (m Model) handleAssignProfileKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	profiles := m.status.Config.Profiles
	if len(profiles) == 0 {
		m.cancelModal()
		return m, nil
	}
	switch key.String() {
	case "esc":
		m.cancelModal()
	case "up", "k":
		m.assignProfileCursor = (m.assignProfileCursor + len(profiles) - 1) % len(profiles)
	case "down", "j":
		m.assignProfileCursor = (m.assignProfileCursor + 1) % len(profiles)
	case "enter", " ":
		return m.applyAssignProfile(m.assignProfileCursor)
	}
	return m, nil
}

func (m Model) applyAssignProfile(index int) (tea.Model, tea.Cmd) {
	profiles := m.status.Config.Profiles
	if index < 0 || index >= len(profiles) {
		return m, nil
	}
	if m.manager == nil {
		m.modalErr = "房间管理器不可用"
		return m, nil
	}
	if !m.accountActionStillValid() {
		m.modalErr = "账号列表已变化（可能由 WebUI 操作），请关闭后重新打开"
		return m, nil
	}
	if m.status.Running {
		m.modalErr = "请先停止房间，再切换配置方案"
		return m, nil
	}
	if err := m.manager.AssignAccountProfile(m.accountActionIndex, profiles[index].ID); err != nil {
		m.modalErr = err.Error()
		return m, nil
	}
	m.closeToReturn()
	m.setNotice("已切换配置方案：" + profiles[index].Name)
	return m, m.snapshotCmd()
}

func playerText(player core.PlayerStatus) string {
	label := player.Name
	if label == "" {
		label = player.ID
	}
	text := fmt.Sprintf("房间 %d · %s · %s", player.Room, label, tuiPlayerStateLabel(player.State))
	if player.RoomPlayerID != 0 {
		text += fmt.Sprintf(" · 成员 %d", player.RoomPlayerID)
	}
	if player.Account != "" {
		text += " · " + player.Account
	}
	return text
}

func tuiPlayerStateLabel(state core.PlayerState) string {
	switch state {
	case core.PlayerConnecting:
		return "连接中"
	case core.PlayerAuthenticating:
		return "认证中"
	case core.PlayerJoined:
		return "已进服"
	case core.PlayerLoginFailed:
		return "登录失败"
	case core.PlayerKicked:
		return "已断开"
	default:
		return string(state)
	}
}

func (m *Model) openPlayers() {
	if m.manager == nil {
		m.setNotice("房间管理器不可用")
		return
	}
	m.players = m.manager.ListPlayers()
	m.playersCursor = 0
	m.playersBusy = false
	m.playersErr = ""
	m.playersNotice = ""
	m.modal = modalPlayers
	m.contentOffset = 0
	m.focus = focusContent
}

func (m Model) openRoomMembers() (tea.Model, tea.Cmd) {
	if m.manager == nil {
		m.setNotice("房间管理器不可用")
		return m, nil
	}
	if !m.accountActionStillValid() {
		m.modalErr = "账号列表已变化（可能由 WebUI 操作），请关闭后重新打开"
		return m, nil
	}
	m.members = m.manager.RoomMembers(m.accountActionIndex)
	m.membersCursor = 0
	m.membersBusy = false
	m.membersErr = ""
	m.membersNotice = ""
	m.modal = modalRoomMembers
	m.contentOffset = 0
	m.focus = focusContent
	return m, nil
}

func (m Model) openFriends() (tea.Model, tea.Cmd) {
	if m.manager == nil {
		m.setNotice("房间管理器不可用")
		return m, nil
	}
	if !m.accountActionStillValid() {
		m.modalErr = "账号列表已变化（可能由 WebUI 操作），请关闭后重新打开"
		return m, nil
	}
	manager := m.manager
	index := m.accountActionIndex
	m.friendRequests = nil
	m.friendsCursor = 0
	m.friendsBusy = true
	m.friendsErr = ""
	m.friendsNotice = ""
	m.modal = modalFriends
	m.contentOffset = 0
	m.focus = focusContent
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		requests, err := manager.FriendRequests(ctx, index)
		return friendsMsg{requests: requests, err: err}
	}
}

func (m Model) handlePlayersKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.cancelModal()
	case "up":
		m.playersCursor = clampInt(m.playersCursor-1, 0, maxInt(0, len(m.players)-1))
	case "down":
		m.playersCursor = clampInt(m.playersCursor+1, 0, maxInt(0, len(m.players)-1))
	case "r":
		m.openPlayers()
	case "x":
		return m.kickSelectedPlayer()
	case "t":
		m.openPlayerTransfer()
	}
	return m, nil
}

func (m Model) kickSelectedPlayer() (tea.Model, tea.Cmd) {
	if m.playersBusy || len(m.players) == 0 {
		return m, nil
	}
	player := m.players[clampInt(m.playersCursor, 0, len(m.players)-1)]
	manager := m.manager
	if manager == nil {
		return m, nil
	}
	m.playersBusy = true
	m.playersErr = ""
	m.playersNotice = ""
	return m, func() tea.Msg {
		_, err := manager.KickPlayer(player.ID)
		notice := ""
		if err == nil {
			label := player.Name
			if label == "" {
				label = player.ID
			}
			notice = "已请求断开玩家：" + label
		}
		return playersMsg{players: manager.ListPlayers(), notice: notice, err: err}
	}
}

func (m *Model) openPlayerTransfer() {
	if len(m.players) == 0 {
		return
	}
	player := m.players[clampInt(m.playersCursor, 0, len(m.players)-1)]
	m.transferTarget = player.ID
	m.transferAddress = nil
	m.transferCursor = 0
	m.modal = modalPlayerTransfer
	m.modalErr = ""
	m.focus = focusContent
}

func (m Model) handleTransferKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	switch name {
	case "esc":
		m.modal = modalPlayers
		m.modalErr = ""
		return m, nil
	case "enter":
		return m.submitTransfer()
	case "backspace", "ctrl+h":
		if m.transferCursor > 0 {
			m.transferAddress = append(m.transferAddress[:m.transferCursor-1], m.transferAddress[m.transferCursor:]...)
			m.transferCursor--
		}
	case "delete":
		if m.transferCursor < len(m.transferAddress) {
			m.transferAddress = append(m.transferAddress[:m.transferCursor], m.transferAddress[m.transferCursor+1:]...)
		}
	case "left":
		if m.transferCursor > 0 {
			m.transferCursor--
		}
	case "right":
		if m.transferCursor < len(m.transferAddress) {
			m.transferCursor++
		}
	case "home", "ctrl+a":
		m.transferCursor = 0
	case "end", "ctrl+e":
		m.transferCursor = len(m.transferAddress)
	case "ctrl+u":
		m.transferAddress = nil
		m.transferCursor = 0
	default:
		if key.Paste || key.Type == tea.KeyRunes {
			runes := key.Runes
			next := make([]rune, 0, len(m.transferAddress)+len(runes))
			next = append(next, m.transferAddress[:m.transferCursor]...)
			next = append(next, runes...)
			next = append(next, m.transferAddress[m.transferCursor:]...)
			m.transferAddress = next
			m.transferCursor += len(runes)
		}
	}
	return m, nil
}

func (m Model) submitTransfer() (tea.Model, tea.Cmd) {
	address := strings.TrimSpace(string(m.transferAddress))
	if address == "" {
		m.modalErr = "请输入目标服务器地址，例如 192.168.1.2:19132"
		return m, nil
	}
	manager := m.manager
	if manager == nil {
		m.modalErr = "房间管理器不可用"
		return m, nil
	}
	if m.playersBusy {
		return m, nil
	}
	target := m.transferTarget
	m.playersBusy = true
	m.modalErr = ""
	return m, func() tea.Msg {
		_, err := manager.TransferPlayer(target, address)
		notice := ""
		if err == nil {
			notice = "已发送转服包：" + target + " → " + address
		}
		return playersMsg{players: manager.ListPlayers(), notice: notice, err: err}
	}
}

func (m Model) handleMembersKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.cancelModal()
	case "up":
		m.membersCursor = clampInt(m.membersCursor-1, 0, maxInt(0, len(m.members)-1))
	case "down":
		m.membersCursor = clampInt(m.membersCursor+1, 0, maxInt(0, len(m.members)-1))
	case "r":
		return m.refreshMembers()
	case "x":
		return m.kickSelectedMember()
	}
	return m, nil
}

func (m Model) refreshMembers() (tea.Model, tea.Cmd) {
	if m.manager == nil || !m.accountActionStillValid() {
		return m, nil
	}
	m.members = m.manager.RoomMembers(m.accountActionIndex)
	m.membersCursor = clampInt(m.membersCursor, 0, maxInt(0, len(m.members)-1))
	m.membersErr = ""
	return m, nil
}

func (m Model) kickSelectedMember() (tea.Model, tea.Cmd) {
	if m.membersBusy || len(m.members) == 0 {
		return m, nil
	}
	member := m.members[clampInt(m.membersCursor, 0, len(m.members)-1)]
	manager := m.manager
	if manager == nil {
		return m, nil
	}
	index := m.accountActionIndex
	m.membersBusy = true
	m.membersErr = ""
	m.membersNotice = ""
	return m, func() tea.Msg {
		err := manager.KickRoomMember(index, member.RoomPlayerID)
		notice := ""
		if err == nil {
			notice = "已请求断开房间成员：" + memberDisplayName(member)
		}
		return membersMsg{members: manager.RoomMembers(index), notice: notice, err: err}
	}
}

func memberDisplayName(member core.RoomMemberStatus) string {
	if member.Name != "" {
		return member.Name
	}
	return fmt.Sprintf("成员 %d", member.RoomPlayerID)
}

func (m Model) handleFriendsKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.cancelModal()
	case "up":
		m.friendsCursor = clampInt(m.friendsCursor-1, 0, maxInt(0, len(m.friendRequests)-1))
	case "down":
		m.friendsCursor = clampInt(m.friendsCursor+1, 0, maxInt(0, len(m.friendRequests)-1))
	case "r":
		return m.openFriends()
	case "a":
		return m.acceptSelectedFriend()
	}
	return m, nil
}

func (m Model) acceptSelectedFriend() (tea.Model, tea.Cmd) {
	if m.friendsBusy || len(m.friendRequests) == 0 {
		return m, nil
	}
	request := m.friendRequests[clampInt(m.friendsCursor, 0, len(m.friendRequests)-1)]
	manager := m.manager
	if manager == nil {
		return m, nil
	}
	index := m.accountActionIndex
	m.friendsBusy = true
	m.friendsErr = ""
	m.friendsNotice = ""
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		err := manager.AcceptFriendRequest(ctx, index, request.UID)
		requests, listErr := manager.FriendRequests(ctx, index)
		if err == nil {
			err = listErr
		}
		notice := ""
		if err == nil {
			notice = "已同意好友申请：" + request.Name
		}
		return friendsMsg{requests: requests, notice: notice, err: err}
	}
}

func (m *Model) openResourcePacks() {
	if m.manager == nil {
		m.setNotice("房间管理器不可用")
		return
	}
	profiles := m.status.Config.Profiles
	if len(profiles) == 0 {
		m.setNotice("还没有配置方案")
		return
	}
	profile := profiles[clampInt(m.profileIndex, 0, len(profiles)-1)]
	if profile.ConnectionMode != config.ConnectionTransfer {
		m.setNotice("只有转服模式的配置方案使用资源包")
		return
	}
	list, err := m.manager.ResourcePacks(profile.ID)
	if err != nil {
		m.setNotice("读取资源包失败：" + err.Error())
		return
	}
	m.packsProfileID = profile.ID
	m.packsProfileName = profile.Name
	m.packs = list
	m.packsCursor = 0
	m.packsBusy = false
	m.packsErr = ""
	m.packsNotice = ""
	m.modalReturn = modalNone
	m.modal = modalResourcePacks
	m.contentOffset = 0
	m.focus = focusContent
}

func (m Model) handlePacksKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.cancelModal()
	case "up":
		m.packsCursor = clampInt(m.packsCursor-1, 0, maxInt(0, len(m.packs.Packs)-1))
	case "down":
		m.packsCursor = clampInt(m.packsCursor+1, 0, maxInt(0, len(m.packs.Packs)-1))
	case "r":
		return m.refreshPacks()
	case "p":
		return m.pullPacks()
	}
	return m, nil
}

func (m Model) refreshPacks() (tea.Model, tea.Cmd) {
	if m.manager == nil {
		return m, nil
	}
	list, err := m.manager.ResourcePacks(m.packsProfileID)
	if err != nil {
		m.packsErr = err.Error()
		return m, nil
	}
	m.packs = list
	m.packsCursor = clampInt(m.packsCursor, 0, maxInt(0, len(m.packs.Packs)-1))
	m.packsErr = ""
	return m, nil
}

func (m Model) pullPacks() (tea.Model, tea.Cmd) {
	if m.manager == nil || m.packsBusy {
		return m, nil
	}
	manager := m.manager
	profileID := m.packsProfileID
	m.packsBusy = true
	m.packsErr = ""
	m.packsNotice = ""
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		result, err := manager.RefreshResourcePacks(ctx, profileID)
		return packsPullMsg{result: result, err: err}
	}
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
	case specialAccountManage:
		m.openAccountActions()
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
	case specialPacks:
		m.openResourcePacks()
		return m, nil
	case specialBulk:
		m.openBulkImport()
		return m, nil
	case specialSettingsEdit:
		m.openSettingsEditor()
		return m, nil
	case specialAdminPassword:
		m.openAdminPasswordModal()
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
	m.modalReturn = modalNone
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
		if m.modal == modalBulkImport && name == "esc" {
			m.bulkCancel = true
		}
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
	case modalAccountActions:
		return m.handleAccountActionsKey(key)
	case modalAssignProfile:
		return m.handleAssignProfileKey(key)
	case modalPlayers:
		return m.handlePlayersKey(key)
	case modalRoomMembers:
		return m.handleMembersKey(key)
	case modalFriends:
		return m.handleFriendsKey(key)
	case modalPlayerTransfer:
		return m.handleTransferKey(key)
	case modalResourcePacks:
		return m.handlePacksKey(key)
	case modalSettings:
		return m.handleSettingsKey(key)
	case modalAdminPassword:
		return m.handleAdminPasswordKey(key)
	case modalBulkImport:
		return m.handleBulkKey(key)
	case modalAccountDashboard:
		return m.handleDashboardKey(key)
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
		m.cancelModal()
		return m, nil
	case "enter":
		return m.submitLogin()
	case "ctrl+o":
		switch {
		case m.loginVerifyURL != "":
			m.openWebUIURL(m.loginVerifyURL)
		case m.loginCaptchaPath != "":
			m.openWebUIURL(m.loginCaptchaPath)
		}
		return m, nil
	case "tab":
		m.loginField = (m.loginField + 1) % m.loginFieldCount()
		return m, nil
	case "shift+tab":
		m.loginField = (m.loginField + m.loginFieldCount() - 1) % m.loginFieldCount()
		return m, nil
	case "up":
		m.loginField = (m.loginField + m.loginFieldCount() - 1) % m.loginFieldCount()
		return m, nil
	case "down":
		m.loginField = (m.loginField + 1) % m.loginFieldCount()
		return m, nil
	}
	if m.loginField == 4 {
		if name == "enter" {
			return m.submitLogin()
		}
		applyRuneKey(&m.loginCaptchaInput, &m.loginCaptchaCursor, key, 16)
		return m, nil
	}
	if m.loginField == 3 {
		switch name {
		case " ", "space":
			m.loginRemember = !m.loginRemember
			return m, nil
		}
		if name == "enter" {
			return m.submitLogin()
		}
		return m, nil
	}
	if m.loginField == 2 {
		switch name {
		case "1":
			m.setLoginPlatform(string(netease.AuthInterop))
		case "2":
			m.setLoginPlatform(string(netease.AuthMobileNetEase))
		case "3":
			m.setLoginPlatform(string(netease.AuthMobile4399))
		case "left", "right", " ":
			switch m.loginPlatform {
			case string(netease.AuthMobileNetEase):
				m.setLoginPlatform(string(netease.AuthMobile4399))
			case string(netease.AuthMobile4399):
				m.setLoginPlatform(string(netease.AuthInterop))
			default:
				m.setLoginPlatform(string(netease.AuthMobileNetEase))
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
		m.nameInput = nil
		m.nameCursor = 0
		if m.nameRename {
			m.setNotice("已取消修改游戏名")
		} else {
			m.setNotice("账号已添加；可以稍后设置游戏名")
		}
		m.closeToReturn()
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
	m.restoreRevision = m.status.ConfigRevision
	m.modalReturn = modalNone
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
	revision := m.restoreRevision
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
		restoreWarnings, err := manager.ConfigureForRestoreIfRevision(next, revision)
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
	m.modalReturn = modalNone
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
	m.modalReturn = modalNone
	m.openLoginModalFor(-1)
}

func (m *Model) openLoginModalFor(replace int) {
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
	m.loginRemember = false
	m.loginCaptchaID = ""
	m.loginCaptchaPath = ""
	m.loginCaptchaNote = ""
	m.loginCaptchaInput = nil
	m.loginCaptchaCursor = 0
	if m.login4399 == nil {
		m.login4399 = m4399login.New()
	}
	m.loginReplace = replace >= 0
	m.loginReplaceIndex = replace
	m.modalBusy = false
	m.modalErr = ""
	m.contentOffset = 0
	m.focus = focusContent
	if replace < 0 || m.manager == nil {
		return
	}
	accounts := m.manager.Accounts()
	if replace >= len(accounts) {
		m.modal = modalNone
		m.setNotice("账号序号无效")
		return
	}
	account := accounts[replace]
	m.loginEmail = []rune(account.Email)
	m.loginEmailCursor = len(m.loginEmail)
	switch account.AuthMethod {
	case string(netease.AuthMobileNetEase):
		m.loginPlatform = string(netease.AuthMobileNetEase)
	case string(netease.AuthMobile4399):
		m.loginPlatform = string(netease.AuthMobile4399)
	}
	if password, saved, err := m.manager.AccountPassword(replace); err == nil && saved {
		m.loginPassword = []rune(password)
		m.loginPasswordCursor = len(m.loginPassword)
		m.loginRemember = true
	}
}

func (m Model) loginFieldCount() int {
	if m.loginPlatform == string(netease.AuthMobile4399) && m.loginCaptchaID != "" {
		return 5 // email, password, platform, remember, captcha
	}
	return 4
}

func (m *Model) setLoginPlatform(platform string) {
	m.loginPlatform = platform
	if platform != string(netease.AuthMobile4399) {
		m.loginCaptchaID = ""
		m.loginCaptchaPath = ""
		m.loginCaptchaNote = ""
		m.loginCaptchaInput = nil
		m.loginCaptchaCursor = 0
		if m.loginField > 3 {
			m.loginField = 3
		}
	}
}

func (m *Model) openNameModal(index int, token, method string) {
	m.modal = modalName
	m.nameIndex = index
	m.nameToken = token
	m.nameMethod = method
	m.nameFingerprint = ""
	if m.manager != nil {
		accounts := m.manager.Accounts()
		if index >= 0 && index < len(accounts) {
			m.nameFingerprint = accountIdentity(accounts[index])
		}
	}
	m.nameRename = false
	m.nameInput = nil
	m.nameCursor = 0
	m.nameSuggestionsCN, m.nameSuggestionsEN = makeNameSuggestions(3)
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
	replace := -1
	if m.loginReplace {
		replace = m.loginReplaceIndex
		accounts := manager.Accounts()
		if replace >= 0 && replace < len(accounts) {
			if id := accounts[replace].ProfileID; id != "" {
				profileID = id
			}
		}
	}
	platform := m.loginPlatform
	remember := m.loginRemember
	captchaID := strings.TrimSpace(m.loginCaptchaID)
	captchaInput := strings.TrimSpace(string(m.loginCaptchaInput))
	login4399 := m.login4399
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		var (
			sauth  map[string]any
			err    error
			method = netease.AuthInterop
		)
		switch platform {
		case string(netease.AuthMobile4399):
			service := login4399
			if service == nil {
				service = m4399login.New()
			}
			sauth, err = service.Login(ctx, email, password, captchaID, captchaInput)
			method = netease.AuthMobile4399
			if err != nil {
				var captcha *m4399login.CaptchaRequired
				if errors.As(err, &captcha) {
					manager.Record("账号", "4399 登录需要图形验证码")
					path, note := saveCaptchaImage(manager, captcha)
					return loginResultMsg{
						captchaID: captcha.CaptchaID, captchaPath: path, captchaNote: note,
					}
				}
				return loginResultMsg{err: fmt.Errorf("%s", sanitizeLoginError(err, email, password))}
			}
		case string(netease.AuthMobileNetEase):
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
		default:
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
		index, err := saveSessionAccount(manager, replace, token, resolved, expiresAt, email, "", profileID, profile, profileError)
		if err != nil {
			return loginResultMsg{err: fmt.Errorf("%s", sanitizeImportError(err, token))}
		}
		if remember && password != "" {
			_ = manager.SaveAccountPassword(index, password)
		} else if replace >= 0 {
			_ = manager.SaveAccountPassword(index, "")
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
	if !validPlayerName(name) {
		m.modalErr = "游戏名长度需为 3–12 个字符（中文按 1.5 个字符计算）"
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

// saveSessionAccount mirrors the server's saveSession: replace < 0 adds a new
// account, otherwise it swaps the session of an existing room. Empty email and
// label fall back to the existing account, then to method-based defaults.
func saveSessionAccount(manager *core.Manager, replace int, token, method, expiresAt, email, label, profileID string, profile auth.Profile, profileError string) (int, error) {
	meta := core.AccountMeta{
		Email: strings.TrimSpace(email), Label: strings.TrimSpace(label),
		Name: profile.Name, UID: profile.UID, Level: profile.Level,
		AvatarURL: profile.AvatarURL, Platform: method, AuthMethod: method,
		ExpiresAt: expiresAt, ProfileError: profileError, ProfileID: profileID,
	}
	methodLabel := netease.AuthMethodLabel(netease.AuthMethod(method))
	if replace >= 0 {
		existing := manager.Accounts()
		if replace < len(existing) {
			if meta.Email == "" {
				meta.Email = existing[replace].Email
			}
			if meta.Label == "" {
				meta.Label = existing[replace].Label
			}
		}
		meta = defaultAccountMeta(meta, methodLabel)
		if err := manager.ReplaceAccount(replace, token, meta); err != nil {
			return -1, err
		}
		return replace, nil
	}
	meta = defaultAccountMeta(meta, methodLabel)
	return manager.AddAccount(token, meta)
}

func defaultAccountMeta(meta core.AccountMeta, methodLabel string) core.AccountMeta {
	if meta.Email == "" {
		meta.Email = meta.Label
	}
	if meta.Email == "" {
		meta.Email = meta.Name
	}
	if meta.Label == "" {
		meta.Label = methodLabel + "会话"
	}
	if meta.Email == "" {
		meta.Email = methodLabel + "会话"
	}
	return meta
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
		index, err := saveSessionAccount(manager, -1, token, method, expiresAt, "", "", profileID, profile, profileError)
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
		m.moveAccountSelection(delta)
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

const profileListHeaderLines = 3

func (m *Model) scrollProfileList() {
	info := m.layoutInfo()
	height := info.content.H
	if height < 1 {
		return
	}
	line := profileListHeaderLines + m.profileIndex*2
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
		m.moveAccountSelection(delta)
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
		if visible := m.visibleAccountIndexes(); len(visible) > 0 {
			m.selected = visible[0]
		}
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
		if visible := m.visibleAccountIndexes(); len(visible) > 0 {
			m.selected = visible[len(visible)-1]
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

func (m *Model) enterConsole() {
	m.consoleMode = true
	m.consoleHistoryPos = -1
	m.logOffset = 0
	m.focus = focusContent
}

func (m Model) handleConsoleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	switch name {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.consoleMode = false
		return m, nil
	case "enter":
		return m.submitConsole()
	case "backspace", "ctrl+h":
		if m.consoleCursor > 0 {
			m.consoleInput = append(m.consoleInput[:m.consoleCursor-1], m.consoleInput[m.consoleCursor:]...)
			m.consoleCursor--
		}
	case "delete":
		if m.consoleCursor < len(m.consoleInput) {
			m.consoleInput = append(m.consoleInput[:m.consoleCursor], m.consoleInput[m.consoleCursor+1:]...)
		}
	case "left":
		if m.consoleCursor > 0 {
			m.consoleCursor--
		}
	case "right":
		if m.consoleCursor < len(m.consoleInput) {
			m.consoleCursor++
		}
	case "home", "ctrl+a":
		m.consoleCursor = 0
	case "end", "ctrl+e":
		m.consoleCursor = len(m.consoleInput)
	case "ctrl+u":
		m.consoleInput = nil
		m.consoleCursor = 0
	case "up":
		m.consoleHistoryUp()
	case "down":
		m.consoleHistoryDown()
	default:
		if key.Paste || key.Type == tea.KeyRunes {
			runes := key.Runes
			room := 512 - len(m.consoleInput)
			if room < 0 {
				room = 0
			}
			if len(runes) > room {
				runes = runes[:room]
			}
			next := make([]rune, 0, len(m.consoleInput)+len(runes))
			next = append(next, m.consoleInput[:m.consoleCursor]...)
			next = append(next, runes...)
			next = append(next, m.consoleInput[m.consoleCursor:]...)
			m.consoleInput = next
			m.consoleCursor += len(runes)
		}
	}
	return m, nil
}

func (m *Model) consoleHistoryUp() {
	if len(m.consoleHistory) == 0 {
		return
	}
	if m.consoleHistoryPos < 0 {
		m.consoleHistoryPos = len(m.consoleHistory) - 1
	} else if m.consoleHistoryPos > 0 {
		m.consoleHistoryPos--
	}
	m.setConsoleInput(m.consoleHistory[m.consoleHistoryPos])
}

func (m *Model) consoleHistoryDown() {
	if m.consoleHistoryPos < 0 {
		return
	}
	if m.consoleHistoryPos < len(m.consoleHistory)-1 {
		m.consoleHistoryPos++
		m.setConsoleInput(m.consoleHistory[m.consoleHistoryPos])
		return
	}
	m.consoleHistoryPos = -1
	m.consoleInput = nil
	m.consoleCursor = 0
}

func (m *Model) setConsoleInput(text string) {
	m.consoleInput = []rune(text)
	m.consoleCursor = len(m.consoleInput)
}

func (m *Model) trimConsoleTranscript() {
	const limit = 200
	if len(m.consoleTranscript) > limit {
		m.consoleTranscript = m.consoleTranscript[len(m.consoleTranscript)-limit:]
	}
}

func (m Model) submitConsole() (tea.Model, tea.Cmd) {
	line := strings.TrimSpace(string(m.consoleInput))
	if line == "" {
		return m, nil
	}
	manager := m.manager
	if manager == nil {
		m.consoleTranscript = append(m.consoleTranscript, consoleLine{
			time: time.Now().Format("15:04:05"), level: "error", text: "房间管理器不可用",
		})
		m.trimConsoleTranscript()
		return m, nil
	}
	if m.consoleBusy {
		return m, nil
	}
	if len(m.consoleHistory) == 0 || m.consoleHistory[len(m.consoleHistory)-1] != line {
		m.consoleHistory = append(m.consoleHistory, line)
		if len(m.consoleHistory) > 100 {
			m.consoleHistory = m.consoleHistory[len(m.consoleHistory)-100:]
		}
	}
	m.consoleHistoryPos = -1
	m.consoleInput = nil
	m.consoleCursor = 0
	m.consoleTranscript = append(m.consoleTranscript, consoleLine{
		time: time.Now().Format("15:04:05"), level: "input", text: "❯ " + line,
	})
	m.trimConsoleTranscript()
	m.consoleBusy = true
	m.logOffset = 0
	return m, func() tea.Msg {
		result, err := manager.RunCommand(line)
		if err != nil {
			return consoleResultMsg{err: fmt.Errorf("%s", sanitizeImportError(err, ""))}
		}
		return consoleResultMsg{output: result.Output}
	}
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
	if m.page == pageAccounts {
		m.normalizeAccountSelection()
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
	maxOffset := len(m.visibleEvents()) + len(m.consoleTranscript) - m.logVisibleLines()
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
		return -1
	}
	if m.selected >= len(m.status.Accounts) {
		return len(m.status.Accounts) - 1
	}
	return m.selected
}

func (m Model) searchFilteredEmpty() bool {
	return len(m.status.Accounts) > 0 && len(m.visibleAccountIndexes()) == 0
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
	lines := m.height - 10
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
		if len(m.status.Accounts) > 0 {
			m.setNotice("没有匹配的账号，请按 / 搜索或 f 筛选")
		} else {
			m.setNotice("还没有账号，请先登录或导入账号")
		}
		return m, nil
	}
	manager := m.manager
	if manager != nil && kind != actionStartAll && kind != actionStopAll {
		live := manager.Accounts()
		if index >= len(m.status.Accounts) || index >= len(live) ||
			accountIdentity(m.status.Accounts[index]) != accountIdentity(live[index]) {
			m.setNotice("账号列表已变化（可能由 WebUI 操作），请重新选择")
			return m, m.snapshotCmd()
		}
	}
	m.busy = true
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
