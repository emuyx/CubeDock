package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/emuyx/cubedock/internal/core"
)

type settingsFieldSpec struct {
	key    string
	label  string
	hint   string
	toggle bool
}

func settingsFields() []settingsFieldSpec {
	return []settingsFieldSpec{
		{key: "auto_start", label: "自动启动", toggle: true, hint: "打开程序时自动启动全部房间"},
		{key: "keep_alive", label: "空闲保活", toggle: true, hint: "未开房账号定期刷新资料/头像"},
		{key: "keep_alive_minutes", label: "保活间隔", hint: "5–1440 分钟，仅在保活开启时使用"},
		{key: "limbo_path", label: "limbo 路径", hint: "留空时自动查找主程序同目录"},
	}
}

func (m Model) settingsValue(spec settingsFieldSpec) string {
	switch spec.key {
	case "auto_start":
		return boolLabel(m.settingsDraft.AutoStart)
	case "keep_alive":
		return boolLabel(m.settingsDraft.KeepAlive)
	case "keep_alive_minutes":
		return strconv.Itoa(m.settingsDraft.KeepAliveMinutes)
	case "limbo_path":
		if m.settingsDraft.LimboPath == "" {
			return "自动查找"
		}
		return m.settingsDraft.LimboPath
	default:
		return ""
	}
}

func (m *Model) openSettingsEditor() {
	if m.manager == nil {
		m.setNotice("房间管理器不可用")
		return
	}
	m.settingsDraft = m.status.Config.Clone()
	m.settingsRevision = m.status.ConfigRevision
	m.settingsField = 0
	m.settingsFieldEdit = false
	m.settingsBuffer = nil
	m.settingsCursor = 0
	m.settingsError = ""
	m.modalReturn = modalNone
	m.modal = modalSettings
	m.modalBusy = false
	m.modalErr = ""
	m.contentOffset = 0
	m.focus = focusContent
}

func (m Model) handleSettingsKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	fields := settingsFields()
	if len(fields) == 0 {
		return m, nil
	}
	m.settingsField = clampInt(m.settingsField, 0, len(fields)-1)
	spec := fields[m.settingsField]
	if m.settingsFieldEdit {
		switch key.String() {
		case "esc":
			m.settingsFieldEdit = false
			m.settingsBuffer = nil
			m.settingsError = ""
		case "enter":
			if err := m.setSettingsFieldText(spec, string(m.settingsBuffer)); err != nil {
				m.settingsError = err.Error()
				return m, nil
			}
			m.settingsFieldEdit = false
			m.settingsBuffer = nil
			m.settingsError = ""
		default:
			applyRuneKey(&m.settingsBuffer, &m.settingsCursor, key, 4096)
		}
		return m, nil
	}
	switch key.String() {
	case "esc":
		m.modal = modalNone
		m.modalErr = ""
	case "ctrl+s":
		return m.saveSettings()
	case "up", "k":
		m.settingsField = (m.settingsField + len(fields) - 1) % len(fields)
	case "down", "j":
		m.settingsField = (m.settingsField + 1) % len(fields)
	case "enter", " ":
		if spec.toggle {
			m.toggleSettingsField(spec)
			return m, nil
		}
		value := m.settingsValue(spec)
		if spec.key == "limbo_path" && value == "自动查找" {
			value = ""
		}
		if spec.key == "keep_alive_minutes" {
			value = strconv.Itoa(m.settingsDraft.KeepAliveMinutes)
		}
		m.settingsFieldEdit = true
		m.settingsBuffer = []rune(value)
		m.settingsCursor = len(m.settingsBuffer)
		m.settingsError = ""
	case "left":
		m.adjustSettingsField(spec, -1)
	case "right":
		m.adjustSettingsField(spec, 1)
	}
	return m, nil
}

func (m *Model) toggleSettingsField(spec settingsFieldSpec) {
	switch spec.key {
	case "auto_start":
		m.settingsDraft.AutoStart = !m.settingsDraft.AutoStart
	case "keep_alive":
		m.settingsDraft.KeepAlive = !m.settingsDraft.KeepAlive
	}
}

func (m *Model) adjustSettingsField(spec settingsFieldSpec, step int) {
	if spec.key == "keep_alive_minutes" {
		m.settingsDraft.KeepAliveMinutes = clampInt(m.settingsDraft.KeepAliveMinutes+step, 5, 1440)
	}
}

func (m *Model) setSettingsFieldText(spec settingsFieldSpec, text string) error {
	switch spec.key {
	case "keep_alive_minutes":
		value, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || value < 5 || value > 1440 {
			return fmt.Errorf("保活间隔必须是 5–1440 分钟")
		}
		m.settingsDraft.KeepAliveMinutes = value
	case "limbo_path":
		m.settingsDraft.LimboPath = strings.TrimSpace(text)
	default:
		return fmt.Errorf("该设置不支持文本编辑")
	}
	return nil
}

func (m Model) saveSettings() (tea.Model, tea.Cmd) {
	if m.manager == nil {
		m.settingsError = "房间管理器不可用"
		return m, nil
	}
	fields := settingsFields()
	if m.settingsFieldEdit && m.settingsField >= 0 && m.settingsField < len(fields) {
		if err := m.setSettingsFieldText(fields[m.settingsField], string(m.settingsBuffer)); err != nil {
			m.settingsError = err.Error()
			return m, nil
		}
		m.settingsFieldEdit = false
		m.settingsBuffer = nil
	}
	if m.settingsDraft.KeepAliveMinutes < 5 || m.settingsDraft.KeepAliveMinutes > 1440 {
		m.settingsError = "保活间隔必须是 5–1440 分钟"
		return m, nil
	}
	next := m.settingsDraft.Clone()
	// Debug is a live toggle; keep the current value instead of reverting it
	// from a draft opened earlier.
	next.Debug = m.manager.Snapshot().Config.Debug
	if err := m.manager.ConfigureIfRevision(next, m.settingsRevision); err != nil {
		switch {
		case errors.Is(err, core.ErrConfigChanged):
			m.settingsError = "配置已在 WebUI 或其他界面修改；请取消后重新打开"
		default:
			m.settingsError = err.Error()
		}
		m.setNotice(m.settingsError)
		return m, nil
	}
	m.modal = modalNone
	m.setNotice("全局设置已保存")
	return m, m.snapshotCmd()
}

func (m Model) settingsEditorInto(b *pageBuilder) {
	width := b.width
	b.add(dimStyle.Render("房间运行中不能保存。"))
	fields := settingsFields()
	rows := make([]fieldBoxRow, 0, len(fields))
	for index, spec := range fields {
		value := m.settingsValue(spec)
		if index == m.settingsField && m.settingsFieldEdit {
			cursor := clampInt(m.settingsCursor, 0, len(m.settingsBuffer))
			value = string(m.settingsBuffer[:cursor]) + "▌" + string(m.settingsBuffer[cursor:])
		}
		rows = append(rows, fieldBoxRow{
			label: spec.label, value: value, active: index == m.settingsField,
			clickable: true, kind: hitSettingsField, index: index,
		})
	}
	m.addFieldBox(b, "选项", rows, width)
	if m.settingsField >= 0 && m.settingsField < len(fields) && fields[m.settingsField].hint != "" {
		b.add(dimStyle.Render(truncate("提示: "+fields[m.settingsField].hint, width)))
	}
	b.customButtonFlow("",
		customButton{label: "保存", kind: hitModalSubmit},
		customButton{label: "取消", kind: hitModalCancel},
	)
	if m.settingsRevision != m.status.ConfigRevision {
		b.add(warnStyle.Render("配置已在 WebUI/其他界面修改；保存会被拒绝，请取消后重新打开。"))
	}
	if m.settingsError != "" {
		b.add(errorStyle.Render(truncate(m.settingsError, width)))
	}
}

func (m *Model) openAdminPasswordModal() {
	if m.options.SetupRequired {
		m.setNotice("请先初始化管理员，再修改密码")
		return
	}
	if m.options.ChangePassword == nil {
		m.setNotice("管理员密码修改不可用")
		return
	}
	m.pwdCurrent = nil
	m.pwdCurrentCursor = 0
	m.pwdNext = nil
	m.pwdNextCursor = 0
	m.pwdConfirm = nil
	m.pwdConfirmCursor = 0
	m.pwdField = 0
	m.modalReturn = modalNone
	m.modal = modalAdminPassword
	m.modalBusy = false
	m.modalErr = ""
	m.contentOffset = 0
	m.focus = focusContent
}

func (m Model) handleAdminPasswordKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	switch name {
	case "esc":
		m.modal = modalNone
		m.modalErr = ""
		return m, nil
	case "enter":
		return m.submitAdminPassword()
	case "tab", "down":
		m.pwdField = (m.pwdField + 1) % 3
		return m, nil
	case "shift+tab", "up":
		m.pwdField = (m.pwdField + 2) % 3
		return m, nil
	}
	m.modalErr = ""
	var value *[]rune
	var cursor *int
	switch m.pwdField {
	case 0:
		value, cursor = &m.pwdCurrent, &m.pwdCurrentCursor
	case 1:
		value, cursor = &m.pwdNext, &m.pwdNextCursor
	default:
		value, cursor = &m.pwdConfirm, &m.pwdConfirmCursor
	}
	applyRuneKey(value, cursor, key, 72)
	return m, nil
}

func (m Model) submitAdminPassword() (tea.Model, tea.Cmd) {
	current := string(m.pwdCurrent)
	next := string(m.pwdNext)
	confirm := string(m.pwdConfirm)
	if current == "" || next == "" {
		m.modalErr = "请填写当前密码和新密码"
		return m, nil
	}
	if next != confirm {
		m.modalErr = "两次输入的新密码不一致"
		return m, nil
	}
	if len(next) < 8 || len(next) > 72 {
		m.modalErr = "新密码需要 8–72 个字节"
		return m, nil
	}
	if m.options.ChangePassword == nil {
		m.modalErr = "管理员密码修改不可用"
		return m, nil
	}
	if m.modalBusy {
		return m, nil
	}
	m.modalBusy = true
	m.modalErr = ""
	change := m.options.ChangePassword
	return m, func() tea.Msg {
		return adminPasswordMsg{err: change(current, next)}
	}
}

func (m Model) adminPasswordInto(b *pageBuilder) {
	width := b.width
	b.add(dimStyle.Render("修改后所有 WebUI 登录会话会失效。"))
	m.addFieldBox(b, "修改密码", []fieldBoxRow{
		{label: "当前", value: displayFieldValue(m.pwdCurrent, m.pwdCurrentCursor, true, m.pwdField == 0),
			active: m.pwdField == 0, clickable: true, kind: hitAdminPasswordField, index: 0},
		{label: "新密码", value: displayFieldValue(m.pwdNext, m.pwdNextCursor, true, m.pwdField == 1),
			active: m.pwdField == 1, clickable: true, kind: hitAdminPasswordField, index: 1},
		{label: "确认", value: displayFieldValue(m.pwdConfirm, m.pwdConfirmCursor, true, m.pwdField == 2),
			active: m.pwdField == 2, clickable: true, kind: hitAdminPasswordField, index: 2},
	}, width)
	b.add(dimStyle.Render("密码 8–72 字节；Tab/↑↓ 切换字段。"))
	b.customButtonFlow("",
		customButton{label: "保存", kind: hitModalSubmit},
		customButton{label: "取消", kind: hitModalCancel},
	)
	if m.modalBusy {
		b.add(m.busyLabel("正在修改管理员密码"))
	}
	if m.modalErr != "" {
		b.add(errorStyle.Render(truncate(m.modalErr, width)))
	}
}
