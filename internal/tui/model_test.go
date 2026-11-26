package tui

import (
	"errors"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/core"
)

func sampleModel() Model {
	profile := config.DefaultProfile()
	profile.ID = "default"
	profile.Name = "默认配置"
	return Model{
		options: Options{
			Version:   "v0.4.3-dev",
			AdminName: "admin",
			LocalURL:  "http://127.0.0.1:18321/",
			LANURLs:   []string{"http://192.168.1.10:18321/"},
		},
		page:  pageHome,
		focus: focusNav,
		status: core.Status{
			Phase:   "online",
			Running: true,
			Players: 1,
			Accounts: []core.AccountStatus{
				{Index: 0, Name: "Alice", Email: "alice@example.com", ProfileID: "default", AuthMethod: "interop"},
				{Index: 1, Name: "小林", ProfileID: "default", AuthMethod: "mobile_netease"},
			},
			Rooms: []core.RoomStatus{
				{Phase: "online", RoomID: 123456, Players: 1, ConnectedAt: "2026-11-25T12:00:00+08:00", UpstreamOnline: true, UpstreamVersion: "1.21.130", ProfileName: "默认配置", ConnectionMode: "proxy"},
				{Phase: "retrying", LastError: "服务器不可达", RetryAt: "2026-11-25T12:01:00+08:00", ProfileName: "默认配置", ConnectionMode: "proxy"},
			},
			Config: config.Config{Profiles: []config.Profile{profile}, WebPort: 18321},
			Events: []core.Event{
				{Time: "12:00:00", Source: "房间 1", Level: "info", Message: "房间已上线", Room: 1},
				{Time: "12:00:01", Source: "房间 2", Level: "warning", Message: "服务器不可达，等待重连", Room: 2},
				{Time: "12:00:02", Source: "系统", Level: "info", Message: "一条比较长的日志，用来确认窄终端不会横向溢出", Room: 0},
			},
		},
	}
}

func TestViewFitsTerminalSizes(t *testing.T) {
	sizes := []struct {
		name   string
		width  int
		height int
	}{
		{name: "tiny", width: 30, height: 8},
		{name: "minimum", width: 34, height: 10},
		{name: "termux-portrait", width: 45, height: 20},
		{name: "termux-tall", width: 45, height: 34},
		{name: "termux-landscape", width: 80, height: 24},
		{name: "narrow-tall", width: 60, height: 50},
		{name: "short-wide", width: 100, height: 18},
		{name: "wide-short", width: 200, height: 18},
		{name: "desktop-narrow", width: 100, height: 30},
		{name: "desktop-wide", width: 120, height: 40},
		{name: "ultrawide", width: 200, height: 60},
		{name: "tall", width: 80, height: 50},
	}
	for _, size := range sizes {
		for _, page := range pageOrder {
			name := size.name + "/page" + string(rune('0'+int(page)))
			t.Run(name, func(t *testing.T) {
				model := sampleModel()
				model.width = size.width
				model.height = size.height
				model.page = page
				view := model.View()
				lines := strings.Split(view, "\n")
				if len(lines) != size.height {
					t.Fatalf("height = %d, want %d", len(lines), size.height)
				}
				for index, line := range lines {
					if width := lipgloss.Width(line); width > size.width {
						t.Fatalf("line %d width = %d > %d: %q", index, width, size.width, line)
					}
				}
			})
		}
	}
}

func TestViewWithoutAccountsOrManager(t *testing.T) {
	model := Model{
		options: Options{LocalURL: "http://127.0.0.1:18321/", SetupRequired: true},
		page:    pageHome,
		focus:   focusNav,
		width:   60,
		height:  20,
	}
	view := model.View()
	if !strings.Contains(view, "还没有账号") {
		t.Fatalf("empty account view missing hint: %q", view)
	}
	if len(strings.Split(view, "\n")) != 20 {
		t.Fatalf("view should fill the terminal height")
	}
}

func TestLogFilterAndScroll(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 80, 24
	model.page = pageLogs
	model.focus = focusContent
	model.logWatch = true
	if got := len(model.visibleEvents()); got != 1 {
		t.Fatalf("filtered events = %d, want 1", got)
	}
	model.move(1)
	if model.logOffset < 0 {
		t.Fatalf("log offset must not be negative: %d", model.logOffset)
	}
	model.move(-1)
	if model.logOffset > len(model.visibleEvents()) {
		t.Fatalf("log offset out of range: %d", model.logOffset)
	}
}

func TestQuitConfirmationAndPageNavigation(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	model = next.(Model)
	if !model.confirmQuit {
		t.Fatal("q should ask for confirmation")
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model = next.(Model)
	if model.confirmQuit {
		t.Fatal("n should cancel quit confirmation")
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(Model)
	if model.focus != focusActions {
		t.Fatalf("focus = %v, want actions", model.focus)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(Model)
	if model.focus != focusContent {
		t.Fatalf("focus = %v, want content", model.focus)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(Model)
	if model.focus != focusNav {
		t.Fatalf("focus = %v, want nav", model.focus)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	model = next.(Model)
	if model.page != pageAccounts || model.focus != focusContent {
		t.Fatalf("page/focus = %v/%v, want accounts/content", model.page, model.focus)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	model = next.(Model)
	if model.page != pageProfiles {
		t.Fatalf("page = %v, want profiles", model.page)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	model = next.(Model)
	if model.page != pageFAQ {
		t.Fatalf("page = %v, want FAQ", model.page)
	}
}

func TestMouseClicksNavigationAndRoomRows(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	info := model.layoutInfo()

	logsIndex := -1
	for index, page := range pageOrder {
		if page == pageLogs {
			logsIndex = index
			break
		}
	}
	logsRect := info.navRects[logsIndex]
	next, _ := model.Update(tea.MouseMsg{
		X: logsRect.X + 1, Y: logsRect.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if model.page != pageLogs || model.focus != focusNav {
		t.Fatalf("page/focus = %v/%v after clicking logs nav", model.page, model.focus)
	}

	accountsIndex := -1
	for index, page := range pageOrder {
		if page == pageAccounts {
			accountsIndex = index
			break
		}
	}
	accountsRect := model.layoutInfo().navRects[accountsIndex]
	next, _ = model.Update(tea.MouseMsg{
		X: accountsRect.X + 1, Y: accountsRect.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if model.page != pageAccounts {
		t.Fatalf("page = %v, want accounts", model.page)
	}
	info = model.layoutInfo()
	if len(info.roomRows) < 2 {
		t.Fatalf("expected two clickable room rows, got %d", len(info.roomRows))
	}
	row := info.roomRows[1]
	next, _ = model.Update(tea.MouseMsg{
		X: row.X + 1, Y: row.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if model.selected != 1 || model.focus != focusContent {
		t.Fatalf("selected/focus = %d/%v after clicking row 2", model.selected, model.focus)
	}
}

func TestMouseClicksToolbarWebUI(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageAccounts
	opened := ""
	model.options.OpenURL = func(url string) { opened = url }
	info := model.layoutInfo()
	webIndex := -1
	for index, item := range info.toolbar {
		if item.Special == specialWebUI {
			webIndex = index
			break
		}
	}
	if webIndex < 0 || webIndex >= len(info.toolbarRects) {
		t.Fatal("WebUI toolbar button not laid out")
	}
	button := info.toolbarRects[webIndex]
	next, _ := model.Update(tea.MouseMsg{
		X: button.X + 1, Y: button.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if opened != model.options.LocalURL {
		t.Fatalf("opened %q, want %q", opened, model.options.LocalURL)
	}
	if model.focus != focusActions || model.actionIndex != webIndex {
		t.Fatalf("focus/action = %v/%d, want actions/%d", model.focus, model.actionIndex, webIndex)
	}
}

func TestLayoutHitboxesStayInsideScreen(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{45, 20}, {80, 24}, {100, 18}, {120, 40}, {200, 60},
	} {
		model := sampleModel()
		model.width, model.height = size.width, size.height
		model.page = pageAccounts
		info := model.layoutInfo()
		rects := append([]rect{}, info.navRects...)
		rects = append(rects, info.toolbarRects...)
		rects = append(rects, info.roomRows...)
		for _, area := range rects {
			if area.X < 0 || area.Y < 0 || area.X+area.W > size.width || area.Y+area.H > size.height {
				t.Fatalf("hitbox %+v outside %dx%d", area, size.width, size.height)
			}
		}
	}
}

func TestManyAccountsListFollowsSelection(t *testing.T) {
	model := sampleModel()
	for index := 2; index < 12; index++ {
		model.status.Accounts = append(model.status.Accounts, core.AccountStatus{
			Index: index, Name: "账号" + string(rune('A'+index)), ProfileID: "default",
		})
		model.status.Rooms = append(model.status.Rooms, core.RoomStatus{Phase: "stopped"})
	}
	model.width, model.height = 45, 20
	model.page = pageAccounts
	model.selected = 11
	info := model.layoutInfo()
	if len(info.roomRowIndex) == 0 {
		t.Fatal("no room hitboxes")
	}
	if info.roomRowIndex[len(info.roomRowIndex)-1] != 11 {
		t.Fatalf("last visible account = %d, want 11 (offset should follow selection)", info.roomRowIndex[len(info.roomRowIndex)-1])
	}
	row := info.roomRows[len(info.roomRows)-1]
	next, _ := model.Update(tea.MouseMsg{
		X: row.X + 1, Y: row.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if model.selected != 11 {
		t.Fatalf("selected = %d, want 11", model.selected)
	}
}

func TestTruncateKeepsDisplayWidth(t *testing.T) {
	cases := []struct {
		text  string
		width int
	}{
		{text: "普通中文日志", width: 8},
		{text: "Alice 已进入世界", width: 10},
		{text: "short", width: 40},
	}
	for _, item := range cases {
		got := truncate(item.text, item.width)
		if width := lipgloss.Width(got); width > item.width {
			t.Fatalf("truncate(%q, %d) width = %d (%q)", item.text, item.width, width, got)
		}
	}
}

func TestPhaseLabels(t *testing.T) {
	for phase, want := range map[string]string{
		"online": "在线", "retrying": "等待重连", "stopped": "已停止",
	} {
		if got := phaseLabel(phase); got != want {
			t.Fatalf("phaseLabel(%q) = %q, want %q", phase, got, want)
		}
	}
}

func TestSidebarToolbarHitboxAlignment(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageHome
	info := model.layoutInfo()
	if len(info.navRects) == 0 || len(info.toolbarRects) == 0 {
		t.Fatal("missing nav/toolbar hitboxes")
	}
	if info.navRects[0].X != 0 || info.navRects[0].W != info.navWidth {
		t.Fatalf("sidebar nav rect = %+v, want x=0 w=%d", info.navRects[0], info.navWidth)
	}
	if info.toolbarRects[0].X != info.contentArea.X {
		t.Fatalf("toolbar starts at x=%d, want content x=%d", info.toolbarRects[0].X, info.contentArea.X)
	}
	for _, nav := range info.navRects {
		for _, button := range info.toolbarRects {
			if rectsOverlap(nav, button) {
				t.Fatalf("nav %+v overlaps toolbar %+v", nav, button)
			}
		}
	}
}

func TestMouseUsesVisualToolbarX(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageAccounts
	opened := ""
	model.options.OpenURL = func(url string) { opened = url }
	info := model.layoutInfo()
	webIndex := -1
	for index, item := range info.toolbar {
		if item.Special == specialWebUI {
			webIndex = index
			break
		}
	}
	if webIndex < 0 {
		t.Fatal("WebUI toolbar item missing")
	}
	row := info.toolbarRects[webIndex].Y
	visualX := info.contentArea.X
	for index := 0; index < webIndex; index++ {
		if info.toolbarRects[index].Y != row {
			visualX = info.contentArea.X
			continue
		}
		visualX += lipgloss.Width(" "+info.toolbar[index].Label+" ") + 1
	}
	next, _ := model.Update(tea.MouseMsg{
		X: visualX + 1, Y: row,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if opened != model.options.LocalURL {
		t.Fatalf("click at visual WebUI x=%d did not open WebUI (opened=%q)", visualX, opened)
	}
}

func TestNarrowScreenExpandsNavigationAndToolbar(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 45, 24
	model.page = pageAccounts
	info := model.layoutInfo()
	if info.sidebar {
		t.Fatal("45 columns should not use the wide sidebar")
	}
	if len(info.navLabels) != len(pageOrder) || info.navLabels[int(pageAccounts)] != "账号与房间" {
		t.Fatalf("narrow nav labels = %#v, want full labels", info.navLabels)
	}
	if info.navArea.H < 2 {
		t.Fatalf("narrow nav height = %d, want expanded multi-row nav", info.navArea.H)
	}
	if len(info.toolbar) != len(accountToolbar) {
		t.Fatalf("narrow toolbar has %d buttons, want all %d", len(info.toolbar), len(accountToolbar))
	}
	if info.toolbarRows < 2 {
		t.Fatalf("narrow toolbar rows = %d, want wrapped rows", info.toolbarRows)
	}
}

func rectsOverlap(left, right rect) bool {
	return left.X < right.X+right.W && right.X < left.X+left.W &&
		left.Y < right.Y+right.H && right.Y < left.Y+left.H
}

func TestImportModalOpensAndEdits(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageAccounts
	model.status.Running = false
	info := model.layoutInfo()
	importIndex := -1
	for index, item := range info.toolbar {
		if item.Special == specialImport {
			importIndex = index
			break
		}
	}
	if importIndex < 0 {
		t.Fatal("import toolbar button missing")
	}
	button := info.toolbarRects[importIndex]
	next, _ := model.Update(tea.MouseMsg{
		X: button.X + 1, Y: button.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if model.modal != modalImportSauth {
		t.Fatalf("modal = %v, want import", model.modal)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(`{"sauth_json":"x"}`), Paste: true})
	model = next.(Model)
	if got := string(model.input); got != `{"sauth_json":"x"}` {
		t.Fatalf("input = %q", got)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	model = next.(Model)
	if got := string(model.input); got != `{"sauth_json":"x"` {
		t.Fatalf("input after backspace = %q", got)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(Model)
	if model.modal != modalNone {
		t.Fatalf("modal = %v, want none", model.modal)
	}
}

func TestImportModalRequiresInput(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageAccounts
	model.status.Running = false
	model.openImportModal()
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(Model)
	if model.modalErr == "" {
		t.Fatal("empty import should show an error")
	}
	if model.modal != modalImportSauth {
		t.Fatal("modal should remain open on validation error")
	}
}

func TestImportModalViewFitsSizes(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{34, 10}, {45, 20}, {80, 24}, {100, 30}, {200, 60},
	} {
		model := sampleModel()
		model.width, model.height = size.width, size.height
		model.page = pageAccounts
		model.status.Running = false
		model.openImportModal()
		model.input = []rune(`{"sauth_json":"` + strings.Repeat("x", 400) + `"}`)
		model.inputCursor = len(model.input)
		view := model.View()
		lines := strings.Split(view, "\n")
		if len(lines) != size.height {
			t.Fatalf("%dx%d: height = %d", size.width, size.height, len(lines))
		}
		for _, line := range lines {
			if width := lipgloss.Width(line); width > size.width {
				t.Fatalf("%dx%d: line width = %d", size.width, size.height, width)
			}
		}
	}
}

func TestLoginModalOpensAndEdits(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageAccounts
	model.status.Running = false
	info := model.layoutInfo()
	loginIndex := -1
	for index, item := range info.toolbar {
		if item.Special == specialLogin {
			loginIndex = index
			break
		}
	}
	if loginIndex < 0 {
		t.Fatal("login toolbar button missing")
	}
	button := info.toolbarRects[loginIndex]
	next, _ := model.Update(tea.MouseMsg{
		X: button.X + 1, Y: button.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if model.modal != modalLogin || model.loginField != 0 {
		t.Fatalf("modal/field = %v/%d", model.modal, model.loginField)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("user@example.com")})
	model = next.(Model)
	if got := string(model.loginEmail); got != "user@example.com" {
		t.Fatalf("email = %q", got)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(Model)
	if model.loginField != 1 {
		t.Fatalf("field = %d, want password", model.loginField)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("secret")})
	model = next.(Model)
	if got := string(model.loginPassword); got != "secret" {
		t.Fatalf("password = %q", got)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(Model)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRight})
	model = next.(Model)
	if model.loginPlatform != "mobile_netease" {
		t.Fatalf("platform = %q", model.loginPlatform)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(Model)
	if model.modal != modalNone {
		t.Fatal("Esc should close login modal")
	}
}

func TestLoginModalRequiresCredentials(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageAccounts
	model.status.Running = false
	model.openLoginModal()
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(Model)
	if model.modalErr == "" || model.modal != modalLogin {
		t.Fatalf("empty login should show error and keep modal (err=%q modal=%v)", model.modalErr, model.modal)
	}
}

func TestLoginAndNameModalViewFits(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{34, 10}, {45, 20}, {80, 24}, {100, 30}, {200, 60},
	} {
		model := sampleModel()
		model.width, model.height = size.width, size.height
		model.page = pageAccounts
		model.status.Running = false
		model.openLoginModal()
		model.loginEmail = []rune("user@example.com")
		model.loginPassword = []rune(strings.Repeat("p", 20))
		model.loginField = 1
		model.loginVerifyURL = "https://example.com/verify?token=abcdefghijklmnopqrstuvwxyz"
		model.loginVerifyNote = "需要完成安全验证"
		model.modalErr = "测试错误信息"
		view := model.View()
		lines := strings.Split(view, "\n")
		if len(lines) != size.height {
			t.Fatalf("%dx%d login height = %d", size.width, size.height, len(lines))
		}
		for _, line := range lines {
			if width := lipgloss.Width(line); width > size.width {
				t.Fatalf("%dx%d login width = %d", size.width, size.height, width)
			}
		}

		model.modal = modalName
		model.nameInput = []rune("星海旅人")
		model.nameCursor = len(model.nameInput)
		model.modalErr = "测试错误"
		view = model.View()
		lines = strings.Split(view, "\n")
		if len(lines) != size.height {
			t.Fatalf("%dx%d name height = %d", size.width, size.height, len(lines))
		}
		for _, line := range lines {
			if width := lipgloss.Width(line); width > size.width {
				t.Fatalf("%dx%d name width = %d", size.width, size.height, width)
			}
		}
	}
}

func TestLoginResultVerifyAndNameFlow(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageAccounts
	model.status.Running = false
	model.openLoginModal()

	next, _ := model.Update(loginResultMsg{verifyURL: "https://example.com/verify", verifyNote: "需要验证"})
	model = next.(Model)
	if model.loginVerifyURL == "" || model.loginVerifyNote == "" || model.modal != modalLogin {
		t.Fatalf("verify result not shown (modal=%v url=%q note=%q)", model.modal, model.loginVerifyURL, model.loginVerifyNote)
	}

	next, _ = model.Update(loginResultMsg{
		index: 0, name: "Alice", needName: true, token: "token", method: "interop",
	})
	model = next.(Model)
	if model.modal != modalName || model.nameIndex != 0 || model.nameToken != "token" {
		t.Fatalf("need_name did not open name modal (modal=%v index=%d token=%q)", model.modal, model.nameIndex, model.nameToken)
	}

	next, _ = model.Update(nameResultMsg{err: errors.New("网络错误")})
	model = next.(Model)
	if model.modalErr == "" || model.modal != modalName {
		t.Fatalf("name error should stay in modal (err=%q modal=%v)", model.modalErr, model.modal)
	}
}

func TestAdminSetupModal(t *testing.T) {
	createdUser, createdPass := "", ""
	model := newModel(Options{
		Version:       "v0.4.3-dev",
		LocalURL:      "http://127.0.0.1:18321/",
		SetupRequired: true,
		CreateAdmin: func(username, password string) error {
			createdUser, createdPass = username, password
			return nil
		},
	})
	model.width, model.height = 100, 30
	if model.modal != modalAdmin {
		t.Fatalf("modal = %v, want admin", model.modal)
	}
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(Model)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("password123")})
	model = next.(Model)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = next.(Model)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("mismatch")})
	model = next.(Model)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(Model)
	if model.modalErr == "" {
		t.Fatal("password mismatch should be rejected")
	}
	// Clear confirm and type the correct password.
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	model = next.(Model)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("password123")})
	model = next.(Model)
	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(Model)
	if cmd == nil {
		t.Fatal("admin submit returned no command")
	}
	model = applyMsg(model, cmd())
	if createdUser != "admin" || createdPass != "password123" {
		t.Fatalf("created %q/%q", createdUser, createdPass)
	}
	if model.options.SetupRequired || model.options.AdminName != "admin" || model.modal != modalNone {
		t.Fatalf("admin state not applied: setup=%v name=%q modal=%v", model.options.SetupRequired, model.options.AdminName, model.modal)
	}
}

func TestBackupAndRestore(t *testing.T) {
	manager, err := core.New(t.TempDir(), config.Defaults(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.RequestExit)
	model := sampleModel()
	model.manager = manager
	model.page = pageSettings
	model.width, model.height = 100, 30
	model.status.Running = false

	info := model.layoutInfo()
	if len(info.toolbar) == 0 {
		t.Fatal("settings toolbar missing")
	}
	model.backupConfig()
	if model.lastBackup == "" {
		t.Fatal("backup path not recorded")
	}
	raw, err := os.ReadFile(model.lastBackup)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"format": 2`) {
		t.Fatalf("backup missing format: %s", raw)
	}

	model.openRestoreModal()
	if len(model.input) == 0 {
		t.Fatal("restore should prefill the latest backup")
	}
	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(Model)
	if cmd == nil {
		t.Fatal("restore returned no command")
	}
	model = applyMsg(model, cmd())
	if model.modalErr != "" {
		t.Fatalf("restore error: %s", model.modalErr)
	}
	if model.modal != modalNone || model.page != pageSettings {
		t.Fatalf("restore state modal=%v page=%v", model.modal, model.page)
	}
}

func applyMsg(model Model, message tea.Msg) Model {
	next, _ := model.Update(message)
	return next.(Model)
}

func TestAdminRestoreViewFitsSizes(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{34, 10}, {45, 20}, {80, 24}, {100, 30}, {200, 60},
	} {
		admin := newModel(Options{
			Version: "v0.4.3-dev", LocalURL: "http://127.0.0.1:18321/",
			SetupRequired: true,
			CreateAdmin:   func(string, string) error { return nil },
		})
		admin.width, admin.height = size.width, size.height
		admin.modalErr = "测试错误"
		for _, model := range []Model{admin} {
			view := model.View()
			lines := strings.Split(view, "\n")
			if len(lines) != size.height {
				t.Fatalf("%dx%d admin height = %d", size.width, size.height, len(lines))
			}
			for _, line := range lines {
				if width := lipgloss.Width(line); width > size.width {
					t.Fatalf("%dx%d admin width = %d", size.width, size.height, width)
				}
			}
		}
		restore := sampleModel()
		restore.width, restore.height = size.width, size.height
		restore.page = pageSettings
		restore.status.Running = false
		restore.openRestoreModal()
		restore.input = []rune(strings.Repeat("x", 120) + ".json")
		restore.inputCursor = len(restore.input)
		view := restore.View()
		lines := strings.Split(view, "\n")
		if len(lines) != size.height {
			t.Fatalf("%dx%d restore height = %d", size.width, size.height, len(lines))
		}
		for _, line := range lines {
			if width := lipgloss.Width(line); width > size.width {
				t.Fatalf("%dx%d restore width = %d", size.width, size.height, width)
			}
		}
	}
}

func profileTestManager(t *testing.T) *core.Manager {
	t.Helper()
	manager, err := core.New(t.TempDir(), config.Defaults(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.RequestExit)
	return manager
}

func TestProfileEditorSave(t *testing.T) {
	manager := profileTestManager(t)
	model := sampleModel()
	model.manager = manager
	model.page = pageProfiles
	model.status.Running = false
	model.width, model.height = 100, 30
	model.openProfileEditor()
	if !model.profileEditing {
		t.Fatal("editor did not open")
	}
	model.profileDraft.Name = "测试方案"
	model.profileDraft.Upstream = "127.0.0.1:20000"
	model.profileDirty = true
	next, cmd := model.saveProfile()
	model = next.(Model)
	if model.profileError != "" {
		t.Fatalf("save error: %s", model.profileError)
	}
	if model.profileEditing {
		t.Fatal("editor should close after save")
	}
	if cmd == nil {
		t.Fatal("save should refresh status")
	}
	found := false
	for _, profile := range manager.Snapshot().Config.Profiles {
		if profile.Name == "测试方案" && profile.Upstream == "127.0.0.1:20000" {
			found = true
		}
	}
	if !found {
		t.Fatal("saved profile not found in manager config")
	}
}

func TestProfileToggleSliderAndTags(t *testing.T) {
	manager := profileTestManager(t)
	model := sampleModel()
	model.manager = manager
	model.page = pageProfiles
	model.status.Running = false
	model.width, model.height = 100, 30
	model.openProfileEditor()

	// Navigate to "好友可见" and toggle it.
	for index, spec := range model.profileFields() {
		if spec.Key == "friends_only" {
			model.profileField = index
			break
		}
	}
	before := model.profileDraft.FriendsOnly
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(Model)
	if model.profileDraft.FriendsOnly == before {
		t.Fatal("toggle did not change")
	}

	for index, spec := range model.profileFields() {
		if spec.Key == "max_players" {
			model.profileField = index
			break
		}
	}
	beforePlayers := model.profileDraft.MaxPlayers
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	model = next.(Model)
	if model.profileDraft.MaxPlayers == beforePlayers {
		t.Fatal("slider did not change")
	}

	for index, spec := range model.profileFields() {
		if spec.Key == "room_tags" {
			model.profileField = index
			break
		}
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(Model)
	if model.modal != modalProfileTags {
		t.Fatalf("tags modal = %v", model.modal)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	model = next.(Model)
	if len(model.profileDraft.RoomTags) != 1 {
		t.Fatalf("tags = %v", model.profileDraft.RoomTags)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(Model)
	if model.modal != modalNone {
		t.Fatal("tags modal should close")
	}
}

func TestProfileFormFitsSizes(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{34, 10}, {45, 20}, {80, 24}, {100, 30}, {200, 60},
	} {
		model := sampleModel()
		model.width, model.height = size.width, size.height
		model.page = pageProfiles
		model.status.Running = false
		model.openProfileEditor()
		model.profileDraft.ConnectionMode = "transfer"
		view := model.View()
		lines := strings.Split(view, "\n")
		if len(lines) != size.height {
			t.Fatalf("%dx%d profile height = %d", size.width, size.height, len(lines))
		}
		for _, line := range lines {
			if width := lipgloss.Width(line); width > size.width {
				t.Fatalf("%dx%d profile width = %d", size.width, size.height, width)
			}
		}
	}
}

func TestProfileNewSaveAndDelete(t *testing.T) {
	manager := profileTestManager(t)
	model := sampleModel()
	model.manager = manager
	model.page = pageProfiles
	model.status.Running = false
	model.width, model.height = 100, 30
	model.newProfileDraft()
	model.profileDraft.Name = "新增方案"
	next, cmd := model.saveProfile()
	model = next.(Model)
	if model.profileError != "" || cmd == nil {
		t.Fatalf("new profile save error=%q cmd=%v", model.profileError, cmd)
	}
	if len(manager.Snapshot().Config.Profiles) != 2 {
		t.Fatalf("profiles = %d, want 2", len(manager.Snapshot().Config.Profiles))
	}
	// Find and delete the new profile (two-step confirmation).
	model.page = pageProfiles
	for index, profile := range model.status.Config.Profiles {
		if profile.Name == "新增方案" {
			model.profileIndex = index
			break
		}
	}
	// Use manager snapshot because the test model's status is stale.
	model.status = manager.Snapshot()
	model.profileIndex = 1
	next, _ = model.deleteProfile()
	model = next.(Model)
	next, cmd = model.deleteProfile()
	model = next.(Model)
	if model.profileError != "" {
		t.Fatal(model.profileError)
	}
	if len(manager.Snapshot().Config.Profiles) != 1 {
		t.Fatalf("profiles after delete = %d, want 1", len(manager.Snapshot().Config.Profiles))
	}
	_ = cmd
}

func TestProfileFieldMouseSelection(t *testing.T) {
	model := sampleModel()
	model.page = pageProfiles
	model.status.Running = false
	model.width, model.height = 100, 30
	model.openProfileEditor()
	info := model.layoutInfo()
	if len(info.profileFields) < 3 {
		t.Fatalf("profile field hitboxes = %d", len(info.profileFields))
	}
	area := info.profileFields[2]
	next, _ := model.Update(tea.MouseMsg{
		X: area.X + 2, Y: area.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if model.profileField != info.profileFieldIndex[2] {
		t.Fatalf("profile field = %d, want %d", model.profileField, info.profileFieldIndex[2])
	}
}

func TestProfileScrollbarAndScroll(t *testing.T) {
	model := sampleModel()
	model.page = pageProfiles
	model.status.Running = false
	model.width, model.height = 45, 20
	model.openProfileEditor()
	model.profileDraft.ConnectionMode = "transfer"
	view := model.View()
	if !strings.Contains(view, "┃") {
		t.Fatal("profile form should show a scrollbar thumb")
	}
	before := model.contentOffset
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = next.(Model)
	if model.contentOffset == before {
		t.Fatal("PgDn should scroll the profile form")
	}
}

func TestPageScrollbarAndPgScroll(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 45, 20
	model.page = pageFAQ
	model.focus = focusNav
	view := model.View()
	if !strings.Contains(view, "┃") {
		t.Fatal("long page should show a scrollbar thumb")
	}
	before := model.contentOffset
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = next.(Model)
	if model.contentOffset == before {
		t.Fatal("PgDn should scroll the page")
	}
	if model.focus != focusContent {
		t.Fatalf("focus = %v, want content", model.focus)
	}
}

func TestRoomNameSelector(t *testing.T) {
	model := sampleModel()
	model.page = pageProfiles
	model.status.Running = false
	model.width, model.height = 100, 30
	model.openProfileEditor()
	for index, spec := range model.profileFields() {
		if spec.Key == "room_name" {
			model.profileField = index
			break
		}
	}
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	model = next.(Model)
	if model.modal != modalProfileRoomName {
		t.Fatalf("modal = %v, want room name", model.modal)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = next.(Model)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(Model)
	if model.modal != modalNone || model.profileDraft.RoomName == "" {
		t.Fatalf("room name selection failed (modal=%v name=%q)", model.modal, model.profileDraft.RoomName)
	}
}
