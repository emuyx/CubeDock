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

const (
	bulkMaxLines = 50
	bulkMaxChars = 64 * 1024
)

func (m *Model) openBulkImport() {
	if m.status.Running {
		m.setNotice("请先停止房间，再批量导入账号")
		return
	}
	if m.manager == nil {
		m.setNotice("房间管理器不可用")
		return
	}
	m.modalReturn = modalNone
	m.modal = modalBulkImport
	m.input = nil
	m.inputCursor = 0
	m.bulkLines = nil
	m.bulkResults = nil
	m.bulkIndex = 0
	m.bulkCancel = false
	m.bulkNotice = ""
	m.bulkProfile = m.status.Config.DefaultProfileID()
	m.modalBusy = false
	m.modalErr = ""
	m.contentOffset = 0
	m.focus = focusContent
}

func (m Model) handleBulkKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	switch name {
	case "esc":
		if m.modalBusy {
			m.bulkCancel = true
			return m, nil
		}
		m.modal = modalNone
		m.input = nil
		m.inputCursor = 0
		m.modalErr = ""
	case "enter", "ctrl+s":
		return m.startBulkImport()
	case "pgup":
		m.moveContent(-8)
	case "pgdown":
		m.moveContent(8)
	default:
		m.applyBulkTextKey(key)
	}
	return m, nil
}

func (m *Model) applyBulkTextKey(key tea.KeyMsg) {
	applyRuneKey(&m.input, &m.inputCursor, key, bulkMaxChars)
}

func (m Model) startBulkImport() (tea.Model, tea.Cmd) {
	if m.modalBusy {
		return m, nil
	}
	text := strings.TrimSpace(string(m.input))
	if text == "" {
		m.modalErr = "请粘贴至少一个登录凭证"
		return m, nil
	}
	if len(text) > bulkMaxChars {
		m.modalErr = "粘贴内容过大，请分批导入"
		return m, nil
	}
	lines := make([]string, 0, 8)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		m.modalErr = "请粘贴至少一个登录凭证"
		return m, nil
	}
	if len(lines) > bulkMaxLines {
		m.modalErr = fmt.Sprintf("一次最多导入 %d 条", bulkMaxLines)
		return m, nil
	}
	if m.status.Running {
		m.modalErr = "请先停止房间，再批量导入账号"
		return m, nil
	}
	if m.manager == nil {
		m.modalErr = "房间管理器不可用"
		return m, nil
	}
	m.bulkLines = lines
	m.bulkResults = make([]bulkLineResult, len(lines))
	for index := range m.bulkResults {
		m.bulkResults[index] = bulkLineResult{status: "pending"}
	}
	m.bulkIndex = 0
	m.bulkCancel = false
	m.bulkNotice = ""
	m.modalBusy = true
	m.modalErr = ""
	m.bulkResults[0].status = "running"
	return m, m.bulkNextCmd()
}

func (m Model) bulkNextCmd() tea.Cmd {
	manager := m.manager
	index := m.bulkIndex
	if index < 0 || index >= len(m.bulkLines) {
		return nil
	}
	line := m.bulkLines[index]
	profileID := m.bulkProfile
	return func() tea.Msg {
		token, method, expiresAt, err := core.NormalizeSession(line, "")
		if err != nil {
			return bulkLineMsg{index: index, savedIndex: -1, err: err}
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
				return bulkLineMsg{index: index, savedIndex: -1,
					err: fmt.Errorf("%s", sanitizeImportError(profileErr, token))}
			}
		}
		saved, err := saveSessionAccount(manager, -1, token, method, expiresAt, "", "", profileID, profile, profileError)
		if err != nil {
			return bulkLineMsg{index: index, savedIndex: -1,
				err: fmt.Errorf("%s", sanitizeImportError(err, token))}
		}
		name := profile.Name
		if name == "" {
			name = "未命名账号"
		}
		return bulkLineMsg{index: index, savedIndex: saved, name: name, needName: needName}
	}
}

func (m Model) bulkInto(b *pageBuilder, height int) {
	width := b.width
	b.add(sectionTitle("批量导入账号"))
	b.add(dimStyle.Render("每行粘贴一个登录凭证；空行和 # 开头的行会被忽略。"))
	profileLabel := m.bulkProfile
	for _, profile := range m.status.Config.Profiles {
		if profile.ID == m.bulkProfile {
			profileLabel = profile.Name
			break
		}
	}
	b.add(dimStyle.Render(truncate("配置方案: "+profileLabel, width)))
	b.blank()
	if m.modalBusy {
		done := 0
		for _, result := range m.bulkResults {
			if result.status != "pending" && result.status != "running" {
				done++
			}
		}
		b.add(m.busyLabel(fmt.Sprintf("正在导入 %d/%d（Esc 停止）", done, len(m.bulkResults))))
	}
	if m.modalErr != "" {
		b.add(errorStyle.Render(truncate(m.modalErr, width)))
	}
	if m.bulkNotice != "" {
		b.add(accentStyle.Render(truncate(m.bulkNotice, width)))
	}
	if len(m.bulkResults) > 0 {
		b.add(dimStyle.Render(fmt.Sprintf("结果：%d 条", len(m.bulkResults))))
		for index, result := range m.bulkResults {
			symbol := "·"
			style := dimStyle
			switch result.status {
			case "ok":
				symbol = "●"
				style = onlineStyle
			case "name":
				symbol = "!"
				style = warnStyle
			case "error":
				symbol = "×"
				style = errorStyle
			case "running":
				symbol = "…"
				style = infoStyle
			case "skipped":
				symbol = "-"
				style = dimStyle
			}
			text := fmt.Sprintf("%2d %s %s", index+1, symbol, result.message)
			b.add(style.Render(truncate(text, width)))
		}
		b.blank()
	}

	inputHeight := height - len(b.lines) - 4
	if inputHeight < 1 {
		inputHeight = 1
	}
	text := ""
	if len(m.input) > 0 {
		cursor := clampInt(m.inputCursor, 0, len(m.input))
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
	m.addTextAreaBox(b, "登录凭证（每行一个）", wrapped, width, true)
	b.add(dimStyle.Render("Enter 开始导入；Esc 关闭；导入中 Esc 停止。"))
	b.customButtonFlow("",
		customButton{label: "开始导入", kind: hitBulkStart},
		customButton{label: "关闭", kind: hitModalCancel},
	)
}

func bulkSummary(results []bulkLineResult) string {
	ok, needName, failed, skipped := 0, 0, 0, 0
	for _, result := range results {
		switch result.status {
		case "ok":
			ok++
		case "name":
			needName++
		case "error":
			failed++
		case "skipped":
			skipped++
		}
	}
	parts := []string{fmt.Sprintf("成功 %d", ok)}
	if needName > 0 {
		parts = append(parts, fmt.Sprintf("需要起名 %d", needName))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("失败 %d", failed))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("跳过 %d", skipped))
	}
	return "批量导入完成：" + strings.Join(parts, "，")
}
