package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/core"
	"github.com/emuyx/cubedock/internal/netease"
)

func (m Model) openAccountName() (tea.Model, tea.Cmd) {
	if m.manager == nil {
		m.modalErr = "房间管理器不可用"
		return m, nil
	}
	if !m.accountActionStillValid() {
		m.modalErr = "账号列表已变化（可能由 WebUI 操作），请关闭后重新打开"
		return m, nil
	}
	index := m.accountActionIndex
	accounts := m.manager.Accounts()
	if index < 0 || index >= len(accounts) {
		m.modalErr = "账号序号无效"
		return m, nil
	}
	token, err := m.manager.Token(index)
	if err != nil {
		m.modalErr = err.Error()
		return m, nil
	}
	method, err := netease.ParseAuthMethod(accounts[index].AuthMethod)
	if err != nil {
		m.modalErr = "账号登录方式无效"
		return m, nil
	}
	m.openNameModal(index, token, string(method))
	if name := strings.TrimSpace(accounts[index].Name); name != "" {
		m.nameRename = true
		m.nameInput = []rune(name)
		m.nameCursor = len(m.nameInput)
	}
	return m, nil
}

func (m *Model) openAccountDashboard() {
	if m.accountActionIndex < 0 {
		return
	}
	m.modalReturn = modalNone
	m.modal = modalAccountDashboard
	m.dashboardChecks = nil
	m.dashboardBusy = false
	m.dashboardErr = ""
	m.dashboardAt = time.Time{}
	m.modalErr = ""
	m.contentOffset = 0
	m.focus = focusContent
}

func (m *Model) openSelectedDashboard() {
	index := m.selectedIndex()
	if index < 0 {
		m.setNotice("还没有账号")
		return
	}
	m.openDashboardFor(index)
}

func (m *Model) openDashboardFor(index int) {
	accounts := m.status.Accounts
	if m.manager != nil {
		accounts = m.manager.Accounts()
	}
	if index < 0 || index >= len(accounts) {
		m.setNotice("账号列表已变化，请重新选择")
		return
	}
	m.accountActionIndex = index
	m.accountFingerprint = accountIdentity(accounts[index])
	m.selected = index
	m.openAccountDashboard()
}

func (m Model) handleDashboardKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.cancelModal()
	case "s":
		return m.dashboardRoomAction(actionStart)
	case "x":
		return m.dashboardRoomAction(actionStop)
	case "r":
		return m.dashboardRoomAction(actionRestart)
	case "d":
		return m.runDashboardDiagnostics()
	case "u":
		return m.refreshAccount()
	case "n":
		return m.openAccountName()
	case "a":
		m.openAssignProfile(m.accountActionIndex)
	case "p":
		m.openPlayers()
	case "m":
		return m.openRoomMembers()
	case "f":
		return m.openFriends()
	}
	return m, nil
}

func (m Model) dashboardRoomAction(kind actionKind) (tea.Model, tea.Cmd) {
	if !m.accountActionStillValid() {
		m.modalErr = "账号列表已变化（可能由 WebUI 操作），请关闭后重新打开"
		return m, nil
	}
	m.selected = m.accountActionIndex
	return m.action(kind)
}

func (m Model) runDashboardDiagnostics() (tea.Model, tea.Cmd) {
	if m.dashboardBusy || m.manager == nil {
		return m, nil
	}
	index := m.accountActionIndex
	accounts := m.manager.Accounts()
	if index < 0 || index >= len(accounts) {
		m.dashboardErr = "账号序号无效"
		return m, nil
	}
	profile := m.profileFor(accounts[index])
	if profile == nil {
		m.dashboardErr = "配置方案不存在"
		return m, nil
	}
	manager := m.manager
	profileID := profile.ID
	m.dashboardBusy = true
	m.dashboardErr = ""
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		return dashboardDiagnosticsMsg{checks: manager.Diagnose(ctx, profileID)}
	}
}

func accountNameActionLabel(account core.AccountStatus) string {
	if strings.TrimSpace(account.Name) != "" {
		return "修改游戏名"
	}
	return "设置游戏名"
}

func (m Model) accountDashboardInto(b *pageBuilder) {
	width := b.width
	index := m.accountActionIndex
	accounts := m.status.Accounts
	if index < 0 || index >= len(accounts) {
		b.add(errorStyle.Render("没有选中账号。"))
		return
	}
	account := accounts[index]
	room := core.RoomStatus{Phase: "stopped"}
	if index < len(m.status.Rooms) {
		room = m.status.Rooms[index]
	}
	phase := room.Phase
	if phase == "" {
		phase = "stopped"
	}
	profile := m.profileFor(account)

	b.add(sectionTitle(fmt.Sprintf("房间 %d · %s", index+1, accountLabel(account))))
	left := []string{
		phaseStyle(phase).Render(truncate("状态: "+phaseLabel(phase), width)),
	}
	if room.RoomID != 0 {
		left = append(left, textStyle.Render(truncate(fmt.Sprintf("房间号: %d", room.RoomID), width)))
	}
	if room.ConnectedAt != "" {
		left = append(left, textStyle.Render(truncate("在线时间: "+durationText(room.ConnectedAt), width)))
	}
	left = append(left, textStyle.Render(truncate(fmt.Sprintf("玩家: %d", room.Players), width)))
	right := make([]string, 0, 6)
	if profile != nil {
		right = append(right, textStyle.Render(truncate(fmt.Sprintf("配置方案: %s（%s）", profile.Name, connectionLabel(profile.ConnectionMode)), width)))
		if profile.ConnectionMode == config.ConnectionTransfer {
			right = append(right, dimStyle.Render(truncate(fmt.Sprintf("中转: %s → %s", profile.LimboUpstream, profile.Upstream), width)))
		} else {
			right = append(right, dimStyle.Render(truncate("服务器: "+profile.Upstream, width)))
		}
	}
	if account.Email != "" {
		right = append(right, dimStyle.Render(truncate("账号: "+account.Email, width)))
	}
	if account.UID != "" {
		right = append(right, dimStyle.Render(truncate("UID: "+account.UID, width)))
	}
	if account.Level > 0 {
		right = append(right, dimStyle.Render(truncate(fmt.Sprintf("等级: %d", account.Level), width)))
	}
	if account.AuthMethod != "" {
		right = append(right, dimStyle.Render(truncate("登录方式: "+authMethodLabel(account.AuthMethod), width)))
	}
	if account.ExpiresAt != "" {
		right = append(right, dimStyle.Render(truncate("会话到期: "+localTime(account.ExpiresAt), width)))
	}
	if width >= 70 {
		columns := (width - 3) / 2
		rightColumns := width - columns - 3
		rows := maxInt(len(left), len(right))
		for row := 0; row < rows; row++ {
			leftText, rightText := "", ""
			if row < len(left) {
				leftText = left[row]
			}
			if row < len(right) {
				rightText = right[row]
			}
			b.add(padRightPlain(truncate(leftText, columns), columns) +
				borderStyle.Render(" │ ") + truncate(rightText, rightColumns))
		}
	} else {
		for _, line := range left {
			b.add(line)
		}
		for _, line := range right {
			b.add(line)
		}
	}
	if !m.accountActionStillValid() {
		b.add(warnStyle.Render("账号列表已在 WebUI/其他界面变化；管理操作会被拒绝。"))
	}
	b.blank()
	b.customButtonFlow("房间: ",
		customButton{label: "启动", kind: hitDashboardStart},
		customButton{label: "停止", kind: hitDashboardStop},
		customButton{label: "重连", kind: hitDashboardRestart},
	)
	b.customButtonFlow("账号: ",
		customButton{label: "刷新资料", kind: hitAccountRefresh},
		customButton{label: "重新登录", kind: hitAccountRelogin},
		customButton{label: accountNameActionLabel(account), kind: hitAccountName},
		customButton{label: "换方案", kind: hitAccountAssign},
	)
	b.customButtonFlow("互动: ",
		customButton{label: "玩家", kind: hitPlayersOpen},
		customButton{label: "成员", kind: hitMembersOpen},
		customButton{label: "好友", kind: hitFriendsOpen},
	)
	b.customButtonFlow("",
		customButton{label: "诊断", kind: hitDashboardDiagnostics},
		customButton{label: "WebUI", kind: hitOpenWebUI},
		customButton{label: "关闭", kind: hitModalCancel},
	)
	b.blank()
	if len(m.dashboardChecks) > 0 {
		b.add(sectionTitle("诊断"))
		counts := map[string]int{}
		for _, check := range m.dashboardChecks {
			counts[check.Status]++
		}
		b.add(dimStyle.Render(truncate(fmt.Sprintf("通过 %d · 注意 %d · 失败 %d · 跳过 %d",
			counts[core.DiagnosticOK], counts[core.DiagnosticWarn],
			counts[core.DiagnosticFail], counts[core.DiagnosticSkip]), width)))
		for _, check := range m.dashboardChecks {
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
		b.blank()
	}
	if m.dashboardBusy {
		b.add(m.busyLabel("正在运行诊断"))
	}
	if m.dashboardErr != "" {
		b.add(errorStyle.Render(truncate(m.dashboardErr, width)))
	}
	if m.modalErr != "" {
		b.add(errorStyle.Render(truncate(m.modalErr, width)))
	}
	events := make([]core.Event, 0, 8)
	for _, event := range m.status.Events {
		if event.Room == index+1 {
			events = append(events, event)
		}
	}
	if len(events) > 8 {
		events = events[len(events)-8:]
	}
	b.add(sectionTitle("最近事件"))
	if len(events) == 0 {
		b.add(dimStyle.Render("暂无该房间事件。"))
	}
	for _, event := range events {
		line := truncate(fmt.Sprintf("%s %s", event.Time, event.Message), width)
		switch event.Level {
		case "error":
			line = errorStyle.Render(line)
		case "warning":
			line = warnStyle.Render(line)
		case "debug":
			line = dimStyle.Render(line)
		default:
			line = textStyle.Render(line)
		}
		b.add(line)
	}
}
