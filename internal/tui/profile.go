package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/core"
)

type profileFieldKind int

const (
	profileText profileFieldKind = iota
	profileNumber
	profileChoice
	profileToggle
	profileSlider
	profileTags
)

type profileFieldSpec struct {
	Key     string
	Label   string
	Group   string
	Kind    profileFieldKind
	Options []string
	Min     int
	Max     int
	Hint    string
}

var profileRoomTagNames = []string{"生存", "创造", "冒险", "模拟", "动作", "建筑", "竞技", "休闲"}

var profileRoomNames = []string{
	"来和我一起玩吧！", "一起玩生存", "一起做建筑", "自由创造",
	"探索冒险", "动作游戏爱好者", "精美建筑欣赏", "交个新朋友",
	"寻找通关搭档", "寻找跑酷达人", "来和我PK吧", "有人来挑战我吗？",
	"来一场速度比拼！", "轻松休闲，快乐游戏", "一起聊聊天吧",
}

func (m Model) profileFields() []profileFieldSpec {
	transfer := m.profileDraft.ConnectionMode == config.ConnectionTransfer
	fields := []profileFieldSpec{
		{Key: "name", Label: "方案名称", Group: "连接", Kind: profileText, Hint: "用于识别这个方案"},
		{Key: "connection_mode", Label: "连接方式", Group: "连接", Kind: profileChoice, Options: []string{"代理", "转服"}},
		{Key: "upstream", Label: "服务器地址", Group: "连接", Kind: profileText, Hint: "IP:端口"},
	}
	if transfer {
		fields = append(fields,
			profileFieldSpec{Key: "limbo_upstream", Label: "limbo 监听地址", Group: "连接", Kind: profileText, Hint: "IP:端口"},
		)
	}
	fields = append(fields,
		profileFieldSpec{Key: "lobby_protocol_id", Label: "房间协议", Group: "连接", Kind: profileNumber, Hint: "默认 43"},
		profileFieldSpec{Key: "room_name", Label: "房间名称", Group: "房间", Kind: profileText, Hint: "Enter 编辑；Ctrl+L 选择内置名"},
		profileFieldSpec{Key: "room_password", Label: "房间密码", Group: "房间", Kind: profileText, Hint: "6 位数字，留空无密码"},
		profileFieldSpec{Key: "max_players", Label: "房间总人数", Group: "房间", Kind: profileSlider, Min: 2, Max: 10, Hint: "含 CubeDock 房主"},
		profileFieldSpec{Key: "min_level", Label: "准入等级", Group: "房间", Kind: profileSlider, Min: 0, Max: 50, Hint: "0 表示不限制"},
		profileFieldSpec{Key: "friends_only", Label: "好友可见", Group: "房间", Kind: profileToggle},
		profileFieldSpec{Key: "auto_accept_friends", Label: "自动同意好友", Group: "房间", Kind: profileToggle, Hint: "好友可见时自动跳过"},
		profileFieldSpec{Key: "allow_pvp", Label: "PvP 大厅标记", Group: "房间", Kind: profileToggle},
		profileFieldSpec{Key: "room_tags", Label: "大厅标签", Group: "房间", Kind: profileTags, Hint: "最多 2 个"},
		profileFieldSpec{Key: "room_game_version", Label: "房间版本", Group: "高级", Kind: profileText, Hint: "一般保持 1.21.130.0"},
		profileFieldSpec{Key: "retry_seconds", Label: "重连间隔", Group: "高级", Kind: profileNumber, Hint: "5–300 秒"},
	)
	if transfer {
		fields = append(fields,
			profileFieldSpec{Key: "limbo_form_title", Label: "表单标题", Group: "转服", Kind: profileText},
			profileFieldSpec{Key: "limbo_form_content", Label: "表单内容", Group: "转服", Kind: profileText, Hint: "可粘贴多行"},
			profileFieldSpec{Key: "limbo_form_button", Label: "按钮文字", Group: "转服", Kind: profileText},
			profileFieldSpec{Key: "limbo_packs_dir", Label: "资源包目录", Group: "转服", Kind: profileText, Hint: "留空自动创建"},
			profileFieldSpec{Key: "limbo_auto_pull_packs", Label: "自动拉取资源包", Group: "转服", Kind: profileToggle},
		)
	}
	return fields
}

func (m Model) profileFieldValue(spec profileFieldSpec) string {
	switch spec.Key {
	case "name":
		return m.profileDraft.Name
	case "connection_mode":
		if m.profileDraft.ConnectionMode == config.ConnectionTransfer {
			return "转服"
		}
		return "代理"
	case "upstream":
		return m.profileDraft.Upstream
	case "limbo_upstream":
		return m.profileDraft.LimboUpstream
	case "lobby_protocol_id":
		return strconv.Itoa(int(m.profileDraft.LobbyProtocolID))
	case "room_name":
		return m.profileDraft.RoomName
	case "room_password":
		return m.profileDraft.RoomPassword
	case "max_players":
		return strconv.Itoa(m.profileDraft.MaxPlayers)
	case "min_level":
		return strconv.Itoa(int(m.profileDraft.MinLevel))
	case "friends_only":
		return boolLabel(m.profileDraft.FriendsOnly)
	case "auto_accept_friends":
		return boolLabel(m.profileDraft.AutoAcceptFriends)
	case "allow_pvp":
		return boolLabel(m.profileDraft.AllowPvP)
	case "room_tags":
		if len(m.profileDraft.RoomTags) == 0 {
			return "未选择"
		}
		names := make([]string, 0, len(m.profileDraft.RoomTags))
		for _, tag := range m.profileDraft.RoomTags {
			if int(tag) < len(profileRoomTagNames) {
				names = append(names, profileRoomTagNames[tag])
			}
		}
		return strings.Join(names, "、")
	case "room_game_version":
		return m.profileDraft.RoomGameVersion
	case "retry_seconds":
		return strconv.Itoa(m.profileDraft.RetrySeconds)
	case "limbo_form_title":
		return m.profileDraft.LimboFormTitle
	case "limbo_form_content":
		return m.profileDraft.LimboFormContent
	case "limbo_form_button":
		return m.profileDraft.LimboFormButton
	case "limbo_packs_dir":
		return m.profileDraft.LimboPacksDir
	case "limbo_auto_pull_packs":
		return boolLabel(m.profileDraft.LimboAutoPullPacks)
	default:
		return ""
	}
}

func (m *Model) setProfileFieldText(spec profileFieldSpec, text string) error {
	switch spec.Key {
	case "name":
		m.profileDraft.Name = strings.TrimSpace(text)
	case "upstream":
		m.profileDraft.Upstream = strings.TrimSpace(text)
	case "limbo_upstream":
		m.profileDraft.LimboUpstream = strings.TrimSpace(text)
	case "lobby_protocol_id":
		value, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || value < 1 || value > 255 {
			return fmt.Errorf("房间协议必须是 1–255")
		}
		m.profileDraft.LobbyProtocolID = uint8(value)
	case "room_name":
		m.profileDraft.RoomName = strings.TrimSpace(text)
	case "room_password":
		value := strings.TrimSpace(text)
		if len(value) > 6 {
			return fmt.Errorf("房间密码最多 6 位")
		}
		for _, r := range value {
			if r < '0' || r > '9' {
				return fmt.Errorf("房间密码只能包含数字")
			}
		}
		m.profileDraft.RoomPassword = value
	case "max_players":
		value, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || value < 2 || value > 10 {
			return fmt.Errorf("房间总人数必须为 2–10")
		}
		m.profileDraft.MaxPlayers = value
	case "min_level":
		value, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || value < 0 || value > 1000 {
			return fmt.Errorf("准入等级必须为 0–1000")
		}
		m.profileDraft.MinLevel = uint32(value)
	case "room_game_version":
		m.profileDraft.RoomGameVersion = strings.TrimSpace(text)
	case "retry_seconds":
		value, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || value < 5 || value > 300 {
			return fmt.Errorf("重连间隔必须为 5–300 秒")
		}
		m.profileDraft.RetrySeconds = value
	case "limbo_form_title":
		m.profileDraft.LimboFormTitle = text
	case "limbo_form_content":
		m.profileDraft.LimboFormContent = text
	case "limbo_form_button":
		m.profileDraft.LimboFormButton = text
	case "limbo_packs_dir":
		m.profileDraft.LimboPacksDir = strings.TrimSpace(text)
	default:
		return fmt.Errorf("该字段不支持文本编辑")
	}
	return nil
}

func (m *Model) toggleProfileField(spec profileFieldSpec) {
	switch spec.Key {
	case "friends_only":
		m.profileDraft.FriendsOnly = !m.profileDraft.FriendsOnly
	case "auto_accept_friends":
		m.profileDraft.AutoAcceptFriends = !m.profileDraft.AutoAcceptFriends
	case "allow_pvp":
		m.profileDraft.AllowPvP = !m.profileDraft.AllowPvP
	case "limbo_auto_pull_packs":
		m.profileDraft.LimboAutoPullPacks = !m.profileDraft.LimboAutoPullPacks
	}
	m.profileDirty = true
}

func (m *Model) adjustProfileField(spec profileFieldSpec, step int) {
	switch spec.Kind {
	case profileChoice:
		if m.profileDraft.ConnectionMode == config.ConnectionTransfer {
			m.profileDraft.ConnectionMode = config.ConnectionProxy
		} else {
			m.profileDraft.ConnectionMode = config.ConnectionTransfer
		}
	case profileSlider:
		switch spec.Key {
		case "max_players":
			m.profileDraft.MaxPlayers = clampInt(m.profileDraft.MaxPlayers+step, spec.Min, spec.Max)
		case "min_level":
			m.profileDraft.MinLevel = uint32(clampInt(int(m.profileDraft.MinLevel)+step, spec.Min, spec.Max))
		}
	case profileToggle:
		m.toggleProfileField(spec)
		return
	default:
		return
	}
	m.profileDirty = true
}

func (m Model) profileFieldLine(spec profileFieldSpec, index, width int) string {
	focused := index == m.profileField
	marker := "  "
	if focused {
		marker = "▌ "
	}
	label := spec.Label + ": "
	value := m.profileFieldValue(spec)
	if focused && m.profileFieldEdit {
		cursor := clampInt(m.profileCursor, 0, len(m.profileBuffer))
		value = string(m.profileBuffer[:cursor]) + "▌" + string(m.profileBuffer[cursor:])
	}
	var body string
	switch spec.Kind {
	case profileToggle:
		state := "[ 关 ]"
		style := stoppedStyle
		if strings.EqualFold(value, "开启") {
			state = "[ 开 ]"
			style = onlineStyle
		}
		body = marker + label + style.Render(state)
	case profileChoice:
		body = marker + label + "‹ " + value + " ›"
	case profileSlider:
		number, _ := strconv.Atoi(value)
		body = marker + label + sliderText(number, spec.Min, spec.Max, 10)
	case profileTags:
		body = marker + label + value
	default:
		body = marker + label + value
	}
	plain := truncate(body, width)
	if focused && m.profileFieldEdit {
		return fieldActiveStyle.Render(padRightPlain(plain, width))
	}
	return textStyle.Render(plain)
}

func sliderText(value, min, max, width int) string {
	if max <= min {
		return "[" + strconv.Itoa(value) + "]"
	}
	if width < 3 {
		width = 3
	}
	filled := (value - min) * width / (max - min)
	filled = clampInt(filled, 0, width)
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "] " + strconv.Itoa(value)
}

func (m Model) profileFormLines(width int) ([]string, []int) {
	fields := m.profileFields()
	lines := make([]string, 0, len(fields)+8)
	indices := make([]int, len(fields))
	group := ""
	for index, spec := range fields {
		if spec.Group != group {
			if group != "" {
				lines = append(lines, "")
			}
			lines = append(lines, sectionTitle(spec.Group))
			group = spec.Group
		}
		indices[index] = len(lines)
		lines = append(lines, m.profileFieldLine(spec, index, width))
	}
	if m.profileField >= 0 && m.profileField < len(fields) && fields[m.profileField].Hint != "" {
		lines = append(lines, dimStyle.Render("提示: "+fields[m.profileField].Hint))
	}
	if m.profileError != "" {
		lines = append(lines, errorStyle.Render(truncate(m.profileError, width)))
	}
	if m.profileRevision != m.status.ConfigRevision {
		lines = append(lines, warnStyle.Render(truncate(
			"配置已在 WebUI/其他界面修改；保存会被拒绝，请取消后重新打开", width)))
	}
	return lines, indices
}

func (m Model) profileFormInto(b *pageBuilder) {
	lines, indices := m.profileFormLines(b.width)
	for _, line := range lines {
		b.add(line)
	}
	for field, lineIndex := range indices {
		b.hit(lineIndex, 0, b.width, hitProfileField, field)
	}
	b.blank()
	items := m.toolbarItems()
	b.buttonFlow("",
		toolbarIndexSpecial(items, specialProfileSave),
		toolbarIndexSpecial(items, specialProfileCancel),
		toolbarIndexSpecial(items, specialWebUI),
	)
}

func (m *Model) openProfileEditor() {
	if m.status.Running {
		m.setNotice("请先停止房间，再编辑配置方案")
		return
	}
	profiles := m.status.Config.Profiles
	if len(profiles) == 0 {
		m.setNotice("还没有配置方案")
		return
	}
	m.profileIndex = clampInt(m.profileIndex, 0, len(profiles)-1)
	m.profileDraft = profiles[m.profileIndex]
	m.profileOriginalID = m.profileDraft.ID
	m.profileRevision = m.status.ConfigRevision
	m.profileNew = false
	m.profileEditing = true
	m.profileField = 0
	m.profileFieldEdit = false
	m.profileError = ""
	m.profileDirty = false
	m.profileBuffer = nil
	m.contentOffset = 0
	m.focus = focusContent
}

func (m *Model) newProfileDraft() {
	base := config.DefaultProfile()
	base.ID = m.uniqueProfileID()
	base.Name = m.uniqueProfileName()
	m.profileDraft = base
	m.profileOriginalID = ""
	m.profileRevision = m.status.ConfigRevision
	m.profileNew = true
	m.profileEditing = true
	m.profileField = 0
	m.profileFieldEdit = false
	m.profileError = ""
	m.profileDirty = true
	m.profileBuffer = nil
	m.contentOffset = 0
	m.focus = focusContent
}

func (m *Model) duplicateProfileDraft() {
	profiles := m.status.Config.Profiles
	if len(profiles) == 0 {
		return
	}
	m.profileIndex = clampInt(m.profileIndex, 0, len(profiles)-1)
	m.profileDraft = profiles[m.profileIndex]
	m.profileDraft.ID = m.uniqueProfileID()
	m.profileDraft.Name = m.uniqueProfileName()
	m.profileOriginalID = ""
	m.profileRevision = m.status.ConfigRevision
	m.profileNew = true
	m.profileEditing = true
	m.profileField = 0
	m.profileFieldEdit = false
	m.profileError = ""
	m.profileDirty = true
	m.profileBuffer = nil
	m.contentOffset = 0
	m.focus = focusContent
}

func (m Model) uniqueProfileID() string {
	existing := map[string]bool{}
	for _, profile := range m.status.Config.Profiles {
		existing[profile.ID] = true
	}
	for index := 1; index < 1000; index++ {
		id := fmt.Sprintf("profile-%d", index)
		if !existing[id] {
			return id
		}
	}
	return fmt.Sprintf("profile-%d", time.Now().Unix())
}

func (m Model) uniqueProfileName() string {
	existing := map[string]bool{}
	for _, profile := range m.status.Config.Profiles {
		existing[strings.ToLower(profile.Name)] = true
	}
	for index := 1; index < 1000; index++ {
		name := fmt.Sprintf("配置方案 %d", index)
		if !existing[strings.ToLower(name)] {
			return name
		}
	}
	return fmt.Sprintf("配置方案 %d", time.Now().Unix())
}

func (m *Model) cancelProfileEdit() {
	if m.profileDirty {
		m.setNotice("已放弃配置方案的修改")
	}
	m.profileEditing = false
	m.profileFieldEdit = false
	m.profileError = ""
	m.profileBuffer = nil
	m.profileDirty = false
	m.contentOffset = 0
	m.focus = focusContent
}

func (m Model) saveProfile() (tea.Model, tea.Cmd) {
	if m.manager == nil {
		m.profileError = "房间管理器不可用"
		return m, nil
	}
	if m.profileFieldEdit {
		fields := m.profileFields()
		if m.profileField >= 0 && m.profileField < len(fields) {
			if err := m.setProfileFieldText(fields[m.profileField], string(m.profileBuffer)); err != nil {
				m.profileError = err.Error()
				return m, nil
			}
		}
		m.profileFieldEdit = false
		m.profileBuffer = nil
	}
	next := m.status.Config.Clone()
	if m.profileNew {
		next.Profiles = append(next.Profiles, m.profileDraft)
	} else {
		found := false
		for index := range next.Profiles {
			if next.Profiles[index].ID == m.profileOriginalID {
				next.Profiles[index] = m.profileDraft
				found = true
				break
			}
		}
		if !found {
			m.profileError = "原配置方案已不存在，请取消后重新打开"
			return m, nil
		}
	}
	// Debug is toggled live; keep its current value instead of reverting it
	// from a draft opened earlier.
	next.Debug = m.manager.Snapshot().Config.Debug
	if err := m.manager.ConfigureIfRevision(next, m.profileRevision); err != nil {
		if errors.Is(err, core.ErrConfigChanged) {
			m.profileError = "配置已在 WebUI 或其他界面修改；请取消后重新打开"
			m.setNotice(m.profileError)
			return m, nil
		}
		m.profileError = err.Error()
		m.setNotice("保存配置方案失败：" + err.Error())
		m.contentOffset = 1 << 20
		return m, nil
	}
	id := m.profileDraft.ID
	m.profileEditing = false
	m.profileFieldEdit = false
	m.profileDirty = false
	m.profileError = ""
	m.profileBuffer = nil
	m.page = pageProfiles
	m.focus = focusContent
	m.profileIndex = 0
	for index, profile := range next.Profiles {
		if profile.ID == id {
			m.profileIndex = index
			break
		}
	}
	m.setNotice("配置方案已保存：" + m.profileDraft.Name)
	return m, m.snapshotCmd()
}

func (m Model) deleteProfile() (tea.Model, tea.Cmd) {
	if m.status.Running {
		m.setNotice("请先停止房间，再删除配置方案")
		return m, nil
	}
	if m.manager == nil {
		return m, nil
	}
	profiles := m.status.Config.Profiles
	if len(profiles) == 0 {
		return m, nil
	}
	m.profileIndex = clampInt(m.profileIndex, 0, len(profiles)-1)
	profile := profiles[m.profileIndex]
	now := time.Now()
	if m.profileDeleteID != profile.ID || now.Sub(m.profileDeleteAt) > 5*time.Second {
		m.profileDeleteID = profile.ID
		m.profileDeleteAt = now
		m.setNotice("再次点击「删除」确认删除：" + profile.Name)
		return m, nil
	}
	if m.accountsUsingProfile(profile.ID) > 0 {
		m.setNotice("仍有账号使用该配置方案，不能删除")
		return m, nil
	}
	// Delete from a fresh snapshot so a WebUI edit is not silently undone by
	// a stale TUI copy.
	snapshot := m.manager.Snapshot()
	next := snapshot.Config.Clone()
	removeAt := -1
	for index, candidate := range next.Profiles {
		if candidate.ID == profile.ID {
			removeAt = index
			break
		}
	}
	if removeAt < 0 {
		m.setNotice("配置方案已在其他界面变化，请重新选择")
		return m, m.snapshotCmd()
	}
	next.Profiles = append(next.Profiles[:removeAt], next.Profiles[removeAt+1:]...)
	if err := m.manager.ConfigureIfRevision(next, snapshot.ConfigRevision); err != nil {
		if errors.Is(err, core.ErrConfigChanged) {
			m.setNotice(core.ErrConfigChanged.Error())
		} else {
			m.setNotice("删除配置方案失败：" + err.Error())
		}
		return m, m.snapshotCmd()
	}
	m.profileDeleteID = ""
	m.profileIndex = clampInt(m.profileIndex, 0, len(next.Profiles)-1)
	m.setNotice("已删除配置方案：" + profile.Name)
	return m, m.snapshotCmd()
}

func (m *Model) moveProfileField(step int) {
	fields := m.profileFields()
	if len(fields) == 0 {
		return
	}
	m.profileField = clampInt(m.profileField+step, 0, len(fields)-1)
	m.profileScrollToField()
}

func (m *Model) profileScrollToField() {
	info := m.layoutInfo()
	_, indices := m.profileFormLines(info.content.W)
	if m.profileField < 0 || m.profileField >= len(indices) {
		return
	}
	index := indices[m.profileField]
	height := info.content.H
	if height < 1 {
		height = 1
	}
	if index < m.contentOffset {
		m.contentOffset = index
	}
	if index >= m.contentOffset+height {
		m.contentOffset = index - height + 1
	}
	if m.contentOffset < 0 {
		m.contentOffset = 0
	}
}

func (m Model) handleProfileKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	if name == "ctrl+c" {
		return m, tea.Quit
	}
	fields := m.profileFields()
	if len(fields) == 0 {
		return m, nil
	}
	m.profileField = clampInt(m.profileField, 0, len(fields)-1)
	spec := fields[m.profileField]
	// Ctrl+L opens the built-in room name chooser even while the text field is
	// being edited; otherwise the hint would only work before pressing Enter.
	if name == "ctrl+l" && spec.Key == "room_name" {
		m.profileFieldEdit = false
		m.profileBuffer = nil
		m.profileError = ""
		m.openRoomNameModal()
		return m, nil
	}
	if m.profileFieldEdit {
		switch name {
		case "esc":
			m.profileFieldEdit = false
			m.profileBuffer = nil
		case "enter":
			if err := m.setProfileFieldText(spec, string(m.profileBuffer)); err != nil {
				m.profileError = err.Error()
				return m, nil
			}
			m.profileError = ""
			m.profileDirty = true
			m.profileFieldEdit = false
			m.profileBuffer = nil
			m.profileScrollToField()
		case "backspace", "ctrl+h":
			if m.profileCursor > 0 {
				m.profileBuffer = append(m.profileBuffer[:m.profileCursor-1], m.profileBuffer[m.profileCursor:]...)
				m.profileCursor--
			}
		case "delete":
			if m.profileCursor < len(m.profileBuffer) {
				m.profileBuffer = append(m.profileBuffer[:m.profileCursor], m.profileBuffer[m.profileCursor+1:]...)
			}
		case "left":
			if m.profileCursor > 0 {
				m.profileCursor--
			}
		case "right":
			if m.profileCursor < len(m.profileBuffer) {
				m.profileCursor++
			}
		case "home", "ctrl+a":
			m.profileCursor = 0
		case "end", "ctrl+e":
			m.profileCursor = len(m.profileBuffer)
		case "ctrl+u":
			m.profileBuffer = nil
			m.profileCursor = 0
		default:
			if key.Paste || key.Type == tea.KeyRunes {
				runes := key.Runes
				if spec.Key == "room_password" {
					filtered := runes[:0]
					for _, r := range runes {
						if r >= '0' && r <= '9' && len(m.profileBuffer)+len(filtered) < 6 {
							filtered = append(filtered, r)
						}
					}
					runes = filtered
				}
				next := make([]rune, 0, len(m.profileBuffer)+len(runes))
				next = append(next, m.profileBuffer[:m.profileCursor]...)
				next = append(next, runes...)
				next = append(next, m.profileBuffer[m.profileCursor:]...)
				m.profileBuffer = next
				m.profileCursor += len(runes)
			}
		}
		return m, nil
	}

	switch name {
	case "esc":
		m.cancelProfileEdit()
	case "ctrl+s":
		return m.saveProfile()
	case "tab", "down", "j":
		m.moveProfileField(1)
	case "shift+tab", "up", "k":
		m.moveProfileField(-1)
	case "g", "home":
		m.profileField = 0
		m.profileScrollToField()
	case "G", "end":
		m.profileField = len(fields) - 1
		m.profileScrollToField()
	case "pgup":
		m.moveContent(-8)
	case "pgdown":
		m.moveContent(8)
	case "enter", " ":
		switch spec.Kind {
		case profileText, profileNumber:
			m.profileFieldEdit = true
			m.profileBuffer = []rune(m.profileFieldValue(spec))
			m.profileCursor = len(m.profileBuffer)
			m.profileError = ""
		case profileToggle:
			m.toggleProfileField(spec)
		case profileChoice:
			m.adjustProfileField(spec, 1)
		case profileTags:
			m.openTagsModal()
		}
	case "left":
		m.adjustProfileField(spec, -1)
	case "right":
		m.adjustProfileField(spec, 1)
	}
	return m, nil
}

func (m *Model) openTagsModal() {
	m.modalReturn = modalNone
	m.modal = modalProfileTags
	m.tagCursor = 0
	m.modalErr = ""
	m.contentOffset = 0
	m.focus = focusContent
}

func (m Model) handleTagsKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	switch name {
	case "esc", "enter":
		m.modal = modalNone
		m.profileScrollToField()
	case "up", "k":
		m.tagCursor = (m.tagCursor + len(profileRoomTagNames) - 1) % len(profileRoomTagNames)
	case "down", "j":
		m.tagCursor = (m.tagCursor + 1) % len(profileRoomTagNames)
	case " ", "space":
		m.toggleTag(m.tagCursor)
	}
	return m, nil
}

func (m *Model) toggleTag(index int) {
	if index < 0 || index >= len(profileRoomTagNames) {
		return
	}
	tag := uint8(index)
	found := -1
	for existingIndex, existing := range m.profileDraft.RoomTags {
		if existing == tag {
			found = existingIndex
			break
		}
	}
	if found >= 0 {
		m.profileDraft.RoomTags = append(m.profileDraft.RoomTags[:found], m.profileDraft.RoomTags[found+1:]...)
		m.profileDirty = true
		m.modalErr = ""
		return
	}
	if len(m.profileDraft.RoomTags) >= 2 {
		m.modalErr = "大厅标签最多选择 2 个"
		return
	}
	m.profileDraft.RoomTags = append(m.profileDraft.RoomTags, tag)
	m.profileDirty = true
	m.modalErr = ""
}

func (m Model) tagsInto(b *pageBuilder) {
	width := b.width
	b.add(panelTitleStyle.Render("大厅标签（最多 2 个）"))
	b.add(dimStyle.Render("↑↓ 选择，Space 切换，Enter/Esc 完成；点击标签可直接切换。"))
	b.blank()
	for index, name := range profileRoomTagNames {
		checked := false
		for _, tag := range m.profileDraft.RoomTags {
			if int(tag) == index {
				checked = true
				break
			}
		}
		marker := "  "
		if index == m.tagCursor {
			marker = "▌ "
		}
		box := "[ ]"
		lineText := marker + box + " " + name
		line := len(b.lines)
		style := textStyle
		switch {
		case checked:
			box = "[x]"
			lineText = marker + box + " " + name
			style = onlineStyle
		case b.hovered(line, 0, width):
			style = rowHoverStyle
		}
		b.add(style.Render(truncate(lineText, width)))
		b.hit(line, 0, width, hitTagToggle, index)
	}
	if m.modalErr != "" {
		b.add(errorStyle.Render(truncate(m.modalErr, width)))
	}
	b.blank()
	b.customButtonFlow("", customButton{label: "完成", kind: hitModalCancel})
}

func (m *Model) openRoomNameModal() {
	m.modalReturn = modalNone
	m.modal = modalProfileRoomName
	m.roomNameCursor = 0
	for index, name := range profileRoomNames {
		if name == m.profileDraft.RoomName {
			m.roomNameCursor = index
			break
		}
	}
	m.modalErr = ""
	m.contentOffset = 0
	m.focus = focusContent
}

func (m Model) handleRoomNameKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.modal = modalNone
		m.profileScrollToField()
	case "enter":
		m.profileDraft.RoomName = profileRoomNames[m.roomNameCursor]
		m.profileDirty = true
		m.modal = modalNone
		m.profileScrollToField()
	case "up", "k":
		m.roomNameCursor = (m.roomNameCursor + len(profileRoomNames) - 1) % len(profileRoomNames)
	case "down", "j":
		m.roomNameCursor = (m.roomNameCursor + 1) % len(profileRoomNames)
	}
	return m, nil
}

func (m Model) roomNameInto(b *pageBuilder) {
	width := b.width
	b.add(panelTitleStyle.Render("内置房间名称"))
	b.add(dimStyle.Render("普通账号建议使用内置名；自定义名称可能被网易拒绝（需要 VIP）。"))
	b.add(dimStyle.Render("点击名称即可使用；↑↓ 选择，Enter 确认。"))
	for index, name := range profileRoomNames {
		marker := "  "
		if index == m.roomNameCursor {
			marker = "▌ "
		}
		lineText := marker + name
		if name == m.profileDraft.RoomName {
			lineText += "  ●"
		}
		line := len(b.lines)
		lineText = padRightPlain(truncate(lineText, width), width)
		style := textStyle
		switch {
		case index == m.roomNameCursor:
			style = selectedRowStyle
		case b.hovered(line, 0, width):
			style = rowHoverStyle
		}
		b.add(style.Render(lineText))
		b.hit(line, 0, width, hitRoomNamePick, index)
	}
	b.blank()
	b.customButtonFlow("", customButton{label: "取消", kind: hitModalCancel})
}
