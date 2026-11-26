package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/core"
	"github.com/emuyx/cubedock/internal/friends"
	"github.com/emuyx/cubedock/internal/m4399login"
	"github.com/emuyx/cubedock/internal/netease"
)

func sampleModel() Model {
	profile := config.DefaultProfile()
	profile.ID = "default"
	profile.Name = "默认配置"
	return Model{
		options: Options{
			Version:   "v0.4.4-dev",
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
	if info.toolbarRects[0].X != info.content.X {
		t.Fatalf("toolbar starts at x=%d, want page content x=%d", info.toolbarRects[0].X, info.content.X)
	}
	for _, nav := range info.navRects {
		for _, button := range info.toolbarRects {
			if rectsOverlap(nav, button) {
				t.Fatalf("nav %+v overlaps toolbar %+v", nav, button)
			}
		}
	}
}

func TestInPageToolbarHitboxesMatchLabels(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageAccounts
	info := model.layoutInfo()
	seen := 0
	for index, item := range info.toolbar {
		area := info.toolbarRects[index]
		if area.W == 0 {
			continue
		}
		if !info.content.Contains(area.X, area.Y) || area.X+area.W > info.content.X+info.content.W {
			t.Fatalf("button %q outside content: %+v", item.Label, area)
		}
		want := buttonWidth(item.Label)
		if area.W != want {
			t.Fatalf("button %q width=%d, want %d", item.Label, area.W, want)
		}
		seen++
	}
	if seen != len(accountToolbar) {
		t.Fatalf("visible buttons = %d, want %d", seen, len(accountToolbar))
	}
}

func TestAccountsEmptyStateKeepsLoginAndImport(t *testing.T) {
	for _, size := range []struct{ width, height int }{{100, 30}, {45, 24}} {
		model := sampleModel()
		model.status.Accounts = nil
		model.status.Rooms = nil
		model.status.Running = false
		model.width, model.height = size.width, size.height
		model.page = pageAccounts
		info := model.layoutInfo()
		seen := map[string]bool{}
		for index, item := range info.toolbar {
			if item.Special == specialLogin || item.Special == specialImport {
				seen[item.Special] = info.toolbarRects[index].W > 0
			}
		}
		if !seen[specialLogin] || !seen[specialImport] {
			t.Fatalf("%dx%d: login/import buttons missing with no accounts: %v", size.width, size.height, seen)
		}

		loginIndex := -1
		for index, item := range info.toolbar {
			if item.Special == specialLogin {
				loginIndex = index
				break
			}
		}
		button := info.toolbarRects[loginIndex]
		next, _ := model.Update(tea.MouseMsg{
			X: button.X + 1, Y: button.Y,
			Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
		})
		model = next.(Model)
		if model.modal != modalLogin {
			t.Fatalf("%dx%d: clicking login with no accounts opened modal %v", size.width, size.height, model.modal)
		}
	}
}

func TestProfileRowClickSelects(t *testing.T) {
	model := sampleModel()
	model.status.Config.Profiles = append(model.status.Config.Profiles, config.Profile{
		ID: "second", Name: "第二套", ConnectionMode: config.ConnectionProxy,
	})
	model.width, model.height = 100, 30
	model.page = pageProfiles
	info := model.layoutInfo()
	var row pageHit
	found := false
	for _, hit := range info.pageHits {
		if hit.kind == hitProfileSelect && hit.index == 1 {
			row, found = hit, true
			break
		}
	}
	if !found {
		t.Fatal("second profile row is not clickable")
	}
	next, _ := model.Update(tea.MouseMsg{
		X: row.X + 1, Y: row.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if model.profileIndex != 1 || model.focus != focusContent {
		t.Fatalf("profile index/focus = %d/%v", model.profileIndex, model.focus)
	}
}

func TestHomeRoomClickJumpsToAccounts(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageHome
	info := model.layoutInfo()
	var row pageHit
	found := false
	for _, hit := range info.pageHits {
		if hit.kind == hitJumpRoom {
			row, found = hit, true
			break
		}
	}
	if !found {
		t.Fatal("running room row is not clickable")
	}
	next, _ := model.Update(tea.MouseMsg{
		X: row.X + 1, Y: row.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if model.page != pageAccounts || model.selected != row.index {
		t.Fatalf("page/selected = %v/%d, want accounts/%d", model.page, model.selected, row.index)
	}
}

func TestLogFilterClickable(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageLogs
	info := model.layoutInfo()
	var filter pageHit
	found := false
	for _, hit := range info.pageHits {
		if hit.kind == hitToggleLogFilter {
			filter, found = hit, true
			break
		}
	}
	if !found {
		t.Fatal("log filter is not clickable")
	}
	next, _ := model.Update(tea.MouseMsg{
		X: filter.X + 1, Y: filter.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if !model.logWatch {
		t.Fatal("clicking the filter should enable room-only logs")
	}
}

func TestModalCancelButtonClickable(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageAccounts
	model.status.Running = false
	model.openLoginModal()
	info := model.layoutInfo()
	var cancel pageHit
	found := false
	for _, hit := range info.pageHits {
		if hit.kind == hitModalCancel {
			cancel, found = hit, true
			break
		}
	}
	if !found {
		t.Fatal("login modal cancel button is not clickable")
	}
	next, _ := model.Update(tea.MouseMsg{
		X: cancel.X + 1, Y: cancel.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if model.modal != modalNone {
		t.Fatalf("modal = %v, want none", model.modal)
	}
}

func TestTagRowClickable(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageProfiles
	model.status.Running = false
	model.openProfileEditor()
	model.openTagsModal()
	info := model.layoutInfo()
	var tag pageHit
	found := false
	for _, hit := range info.pageHits {
		if hit.kind == hitTagToggle && hit.index == 0 {
			tag, found = hit, true
			break
		}
	}
	if !found {
		t.Fatal("tag row is not clickable")
	}
	next, _ := model.Update(tea.MouseMsg{
		X: tag.X + 1, Y: tag.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if len(model.profileDraft.RoomTags) != 1 || model.profileDraft.RoomTags[0] != 0 {
		t.Fatalf("tags = %v, want [0]", model.profileDraft.RoomTags)
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
		Version:       "v0.4.4-dev",
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
			Version: "v0.4.4-dev", LocalURL: "http://127.0.0.1:18321/",
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
	if !model.profileFieldEdit {
		t.Fatal("clicking a text field should start editing")
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

func accountTestManager(t *testing.T) *core.Manager {
	t.Helper()
	manager := profileTestManager(t)
	if _, err := manager.AddAccount("test-token-1", core.AccountMeta{
		Name: "Alice", Email: "alice@example.com",
		AuthMethod: "interop", Platform: "interop",
	}); err != nil {
		t.Fatal(err)
	}
	return manager
}

func assertViewFits(t *testing.T, model Model, width, height int) {
	t.Helper()
	lines := strings.Split(model.View(), "\n")
	if len(lines) != height {
		t.Fatalf("%dx%d: height = %d", width, height, len(lines))
	}
	for _, line := range lines {
		if lipgloss.Width(line) > width {
			t.Fatalf("%dx%d: line width = %d: %q", width, height, lipgloss.Width(line), line)
		}
	}
}

func TestAccountActionsReloginPrefill(t *testing.T) {
	manager := accountTestManager(t)
	if err := manager.SaveAccountPassword(0, "secret"); err != nil {
		t.Fatal(err)
	}
	model := sampleModel()
	model.manager = manager
	model.status = manager.Snapshot()
	model.status.Running = false
	model.width, model.height = 100, 30
	model.page = pageAccounts
	model.openAccountActions()
	if model.modal != modalAccountActions {
		t.Fatalf("modal = %v, want account actions", model.modal)
	}
	info := model.layoutInfo()
	var relogin pageHit
	found := false
	for _, hit := range info.pageHits {
		if hit.kind == hitAccountRelogin {
			relogin, found = hit, true
			break
		}
	}
	if !found {
		t.Fatal("re-login action is not clickable")
	}
	next, _ := model.Update(tea.MouseMsg{
		X: relogin.X + 1, Y: relogin.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if model.modal != modalLogin || !model.loginReplace || model.loginReplaceIndex != 0 {
		t.Fatalf("modal/replace = %v/%v/%d", model.modal, model.loginReplace, model.loginReplaceIndex)
	}
	if got := string(model.loginEmail); got != "alice@example.com" {
		t.Fatalf("email = %q", got)
	}
	if got := string(model.loginPassword); got != "secret" || !model.loginRemember {
		t.Fatalf("saved password not loaded: %q remember=%v", got, model.loginRemember)
	}
}

func TestAccountDeleteRequiresTwoClicks(t *testing.T) {
	manager := accountTestManager(t)
	model := sampleModel()
	model.manager = manager
	model.status = manager.Snapshot()
	model.status.Running = false
	model.width, model.height = 100, 30
	model.page = pageAccounts
	model.openAccountActions()

	next, cmd := model.activateAccountAction(hitAccountDelete)
	model = next.(Model)
	if cmd != nil || !model.accountDeleteArmed {
		t.Fatalf("first delete click should only arm (cmd=%v armed=%v)", cmd, model.accountDeleteArmed)
	}
	next, cmd = model.activateAccountAction(hitAccountDelete)
	model = next.(Model)
	if cmd == nil {
		t.Fatal("second delete click returned no command")
	}
	model = applyMsg(model, cmd())
	if model.modal != modalNone {
		t.Fatalf("modal = %v after delete", model.modal)
	}
	if got := len(manager.Accounts()); got != 0 {
		t.Fatalf("accounts = %d, want 0", got)
	}
}

func TestAssignProfileFromAccountActions(t *testing.T) {
	manager := accountTestManager(t)
	cfg := manager.Snapshot().Config.Clone()
	second := cfg.Profiles[0]
	second.ID = "second"
	second.Name = "第二套"
	cfg.Profiles = append(cfg.Profiles, second)
	if err := manager.Configure(cfg); err != nil {
		t.Fatal(err)
	}
	model := sampleModel()
	model.manager = manager
	model.status = manager.Snapshot()
	model.status.Running = false
	model.width, model.height = 100, 30
	model.page = pageAccounts
	model.selected = 0
	model.openAccountActions()
	model.openAssignProfile(0)
	if model.modal != modalAssignProfile {
		t.Fatalf("modal = %v, want assign profile", model.modal)
	}
	next, cmd := model.applyAssignProfile(1)
	model = next.(Model)
	if cmd == nil {
		t.Fatal("assign profile returned no refresh command")
	}
	model = applyMsg(model, cmd())
	if got := manager.Snapshot().Accounts[0].ProfileID; got != "second" {
		t.Fatalf("profile = %q, want second", got)
	}
}

func TestConsoleCommandInput(t *testing.T) {
	manager := profileTestManager(t)
	model := sampleModel()
	model.manager = manager
	model.width, model.height = 100, 30
	model.page = pageLogs
	model.focus = focusContent

	info := model.layoutInfo()
	var input pageHit
	found := false
	for _, hit := range info.pageHits {
		if hit.kind == hitConsoleInput {
			input, found = hit, true
			break
		}
	}
	if !found {
		t.Fatal("console input is not clickable")
	}
	next, _ := model.Update(tea.MouseMsg{
		X: input.X + 1, Y: input.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if !model.consoleMode {
		t.Fatal("clicking the input line should enter console mode")
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("help")})
	model = next.(Model)
	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(Model)
	if cmd == nil {
		t.Fatal("console submit returned no command")
	}
	model = applyMsg(model, cmd())
	if model.consoleBusy {
		t.Fatal("console should not stay busy after the result")
	}
	var transcript strings.Builder
	for _, line := range model.consoleTranscript {
		transcript.WriteString(line.text)
		transcript.WriteByte('\n')
	}
	if !strings.Contains(transcript.String(), "kick") {
		t.Fatalf("help output missing from transcript: %q", transcript.String())
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = next.(Model)
	if got := string(model.consoleInput); got != "help" {
		t.Fatalf("history input = %q, want help", got)
	}
}

func TestFocusedFrameUsesDoubleBorder(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageHome
	model.focus = focusNav
	if strings.Contains(model.View(), "╔") {
		t.Fatal("unfocused frame should use the light border")
	}
	model.focus = focusContent
	if !strings.Contains(model.View(), "╔") {
		t.Fatal("focused frame should use the beveled border")
	}
}

func TestNewModalsFitSizes(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{34, 10}, {45, 20}, {80, 24}, {100, 30}, {200, 60},
	} {
		manager := accountTestManager(t)
		model := sampleModel()
		model.manager = manager
		model.status = manager.Snapshot()
		model.status.Running = false
		model.width, model.height = size.width, size.height
		model.page = pageAccounts
		model.openAccountActions()
		assertViewFits(t, model, size.width, size.height)

		model.openAssignProfile(0)
		assertViewFits(t, model, size.width, size.height)

		model.modal = modalNone
		model.page = pageLogs
		model.consoleMode = true
		model.consoleInput = []rune("help")
		model.consoleCursor = len(model.consoleInput)
		assertViewFits(t, model, size.width, size.height)
	}
}

func TestProfileSaveRejectsExternalConfigChange(t *testing.T) {
	manager := profileTestManager(t)
	model := sampleModel()
	model.manager = manager
	model.page = pageProfiles
	model.status = manager.Snapshot()
	model.status.Running = false
	model.width, model.height = 100, 30
	model.openProfileEditor()

	external := manager.Snapshot().Config.Clone()
	external.Profiles[0].RoomName = "WebUI 修改"
	if err := manager.Configure(external); err != nil {
		t.Fatal(err)
	}

	model.profileDraft.Name = "TUI 修改"
	next, _ := model.saveProfile()
	model = next.(Model)
	if model.profileError == "" || !strings.Contains(model.profileError, "配置已在") {
		t.Fatalf("save conflict not reported: %q", model.profileError)
	}
	if !model.profileEditing {
		t.Fatal("editor should stay open after a conflict")
	}
	if got := manager.Snapshot().Config.Profiles[0].RoomName; got != "WebUI 修改" {
		t.Fatalf("external change was overwritten: %q", got)
	}
}

func TestAccountActionDetectsShiftedList(t *testing.T) {
	manager := profileTestManager(t)
	if _, err := manager.AddAccount("token-a", core.AccountMeta{
		Name: "Alice", Email: "a@example.com", UID: "1", AuthMethod: "interop", Platform: "interop",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.AddAccount("token-b", core.AccountMeta{
		Name: "Bob", Email: "b@example.com", UID: "2", AuthMethod: "interop", Platform: "interop",
	}); err != nil {
		t.Fatal(err)
	}
	model := sampleModel()
	model.manager = manager
	model.status = manager.Snapshot()
	model.status.Running = false
	model.selected = 0
	model.width, model.height = 100, 30
	model.openAccountActions()
	if model.accountFingerprint == "" {
		t.Fatal("missing account fingerprint")
	}
	if err := manager.RemoveAccount(0); err != nil {
		t.Fatal(err)
	}

	next, cmd := model.activateAccountAction(hitAccountRefresh)
	model = next.(Model)
	if cmd != nil || !strings.Contains(model.modalErr, "变化") {
		t.Fatalf("shifted account not rejected: err=%q cmd=%v", model.modalErr, cmd)
	}

	model.selected = 0
	next, _ = model.action(actionStart)
	model = next.(Model)
	if model.busy {
		t.Fatal("room action started on a shifted account list")
	}
	if !strings.Contains(model.notice, "变化") {
		t.Fatalf("room action notice = %q", model.notice)
	}
}

func TestInteractionModalsFit(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{45, 20}, {100, 30}, {200, 60},
	} {
		manager := accountTestManager(t)
		model := sampleModel()
		model.manager = manager
		model.status = manager.Snapshot()
		model.status.Running = false
		model.selected = 0
		model.width, model.height = size.width, size.height
		model.page = pageAccounts
		model.openAccountActions()

		model.openPlayers()
		assertViewFits(t, model, size.width, size.height)

		next, _ := model.openRoomMembers()
		model = next.(Model)
		assertViewFits(t, model, size.width, size.height)

		model.modal = modalFriends
		model.friendRequests = []friends.Request{{UID: 42, Name: "Bob", ApplyTime: "12:00:00"}}
		model.friendsBusy = false
		assertViewFits(t, model, size.width, size.height)

		model.modal = modalPlayerTransfer
		model.transferTarget = "player-1"
		model.transferAddress = []rune("127.0.0.1:19132")
		model.transferCursor = len(model.transferAddress)
		assertViewFits(t, model, size.width, size.height)

		model.page = pageProfiles
		model.profileIndex = 0
		model.status.Config.Profiles[0].ConnectionMode = config.ConnectionTransfer
		model.openResourcePacks()
		if model.modal != modalResourcePacks {
			t.Fatalf("%dx%d: packs modal = %v", size.width, size.height, model.modal)
		}
		model.packs.Packs = []core.ResourcePack{{File: "a.mcpack", Name: "A", Version: "1", Size: 2048}}
		assertViewFits(t, model, size.width, size.height)
	}
}

func TestSettingsEditorSave(t *testing.T) {
	manager := profileTestManager(t)
	model := sampleModel()
	model.manager = manager
	model.status = manager.Snapshot()
	model.status.Running = false
	model.page = pageSettings
	model.width, model.height = 100, 30
	model.openSettingsEditor()
	model.settingsDraft.AutoStart = true
	model.settingsDraft.KeepAliveMinutes = 45
	next, cmd := model.saveSettings()
	model = next.(Model)
	if model.settingsError != "" || cmd == nil {
		t.Fatalf("save settings error=%q cmd=%v", model.settingsError, cmd)
	}
	model = applyMsg(model, cmd())
	cfg := manager.Snapshot().Config
	if !cfg.AutoStart || cfg.KeepAliveMinutes != 45 {
		t.Fatalf("settings not saved: auto_start=%v minutes=%d", cfg.AutoStart, cfg.KeepAliveMinutes)
	}
}

func TestSettingsSaveRejectsExternalChange(t *testing.T) {
	manager := profileTestManager(t)
	model := sampleModel()
	model.manager = manager
	model.status = manager.Snapshot()
	model.status.Running = false
	model.page = pageSettings
	model.width, model.height = 100, 30
	model.openSettingsEditor()

	external := manager.Snapshot().Config.Clone()
	external.Profiles[0].RoomName = "WebUI 修改"
	if err := manager.Configure(external); err != nil {
		t.Fatal(err)
	}
	model.settingsDraft.AutoStart = true
	next, _ := model.saveSettings()
	model = next.(Model)
	if !strings.Contains(model.settingsError, "配置已在") {
		t.Fatalf("conflict not reported: %q", model.settingsError)
	}
	if manager.Snapshot().Config.AutoStart {
		t.Fatal("conflicting settings save was applied")
	}
}

func TestAdminPasswordChange(t *testing.T) {
	var got [2]string
	model := newModel(Options{
		Version:        "v0.4.4-dev",
		LocalURL:       "http://127.0.0.1:18321/",
		AdminName:      "admin",
		ChangePassword: func(current, next string) error { got = [2]string{current, next}; return nil },
	})
	model.width, model.height = 100, 30
	model.openAdminPasswordModal()
	if model.modal != modalAdminPassword {
		t.Fatalf("modal = %v", model.modal)
	}
	model.pwdCurrent = []rune("old-password")
	model.pwdNext = []rune("new-password")
	model.pwdConfirm = []rune("mismatch")
	next, _ := model.submitAdminPassword()
	model = next.(Model)
	if model.modalErr == "" || model.modal != modalAdminPassword {
		t.Fatal("password mismatch should be rejected")
	}
	model.pwdConfirm = []rune("new-password")
	next, cmd := model.submitAdminPassword()
	model = next.(Model)
	if cmd == nil {
		t.Fatal("password change returned no command")
	}
	model = applyMsg(model, cmd())
	if model.modal != modalNone {
		t.Fatalf("modal = %v after password change", model.modal)
	}
	if got != [2]string{"old-password", "new-password"} {
		t.Fatalf("callback = %#v", got)
	}
}

func TestBulkImportValidationAndFailure(t *testing.T) {
	manager := profileTestManager(t)
	model := sampleModel()
	model.manager = manager
	model.status = manager.Snapshot()
	model.status.Running = false
	model.page = pageAccounts
	model.width, model.height = 100, 30

	model.openBulkImport()
	next, _ := model.startBulkImport()
	model = next.(Model)
	if model.modalErr == "" {
		t.Fatal("empty bulk import should be rejected")
	}

	model.input = []rune("not a session")
	model.inputCursor = len(model.input)
	next, cmd := model.startBulkImport()
	model = next.(Model)
	if cmd == nil {
		t.Fatal("bulk import returned no command")
	}
	model = applyMsg(model, cmd())
	if model.modalBusy {
		t.Fatal("bulk import should finish after one invalid line")
	}
	if len(model.bulkResults) != 1 || model.bulkResults[0].status != "error" {
		t.Fatalf("bulk results = %#v", model.bulkResults)
	}
	if !strings.Contains(model.bulkNotice, "失败 1") {
		t.Fatalf("bulk notice = %q", model.bulkNotice)
	}
}

func TestAccountSearchFilterSort(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageAccounts
	model.selected = 0

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model = next.(Model)
	if !model.accountSearch {
		t.Fatal("/ should enter account search")
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("alice")})
	model = next.(Model)
	if visible := model.visibleAccountIndexes(); len(visible) != 1 || visible[0] != 0 {
		t.Fatalf("search visible = %v", visible)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(Model)
	if model.accountSearch {
		t.Fatal("Esc should leave account search")
	}

	model.accountQuery = []rune("zzz")
	model.normalizeAccountSelection()
	if !model.searchFilteredEmpty() || model.selected != -1 {
		t.Fatalf("empty search state: filtered=%v selected=%d", model.searchFilteredEmpty(), model.selected)
	}

	model.accountQuery = nil
	model.accountFilter = "online"
	model.normalizeAccountSelection()
	if visible := model.visibleAccountIndexes(); len(visible) != 1 || visible[0] != 0 {
		t.Fatalf("online visible = %v", visible)
	}

	model.accountFilter = "all"
	model.status.Accounts[0].Name = "Zoe"
	model.status.Accounts[1].Name = "Amy"
	model.accountSort = "name"
	if visible := model.visibleAccountIndexes(); len(visible) != 2 || visible[0] != 1 || visible[1] != 0 {
		t.Fatalf("name-sorted visible = %v", visible)
	}

	model.accountSort = ""
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	model = next.(Model)
	if model.accountFilterMode() != "online" {
		t.Fatalf("f filter = %q", model.accountFilterMode())
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	model = next.(Model)
	if model.accountSortMode() != "name" {
		t.Fatalf("v sort = %q", model.accountSortMode())
	}
}

func TestDashboardOpensAndFits(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{45, 20}, {100, 30}, {200, 60},
	} {
		manager := accountTestManager(t)
		model := sampleModel()
		model.manager = manager
		model.status = manager.Snapshot()
		model.status.Running = false
		model.selected = 0
		model.width, model.height = size.width, size.height
		model.page = pageAccounts
		model.status.Events = []core.Event{
			{Time: "12:00:00", Source: "房间 1", Level: "info", Message: "房间已上线", Room: 1},
			{Time: "12:00:01", Source: "房间 2", Level: "warning", Message: "其他房间", Room: 2},
		}
		model.openSelectedDashboard()
		if model.modal != modalAccountDashboard || model.accountActionIndex != 0 {
			t.Fatalf("%dx%d: dashboard modal=%v index=%d", size.width, size.height, model.modal, model.accountActionIndex)
		}
		assertViewFits(t, model, size.width, size.height)
	}
}

func TestSettingsAndPasswordModalsFit(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{34, 10}, {45, 20}, {100, 30}, {200, 60},
	} {
		manager := profileTestManager(t)
		model := sampleModel()
		model.manager = manager
		model.status = manager.Snapshot()
		model.status.Running = false
		model.width, model.height = size.width, size.height
		model.page = pageSettings
		model.openSettingsEditor()
		assertViewFits(t, model, size.width, size.height)

		model.options.ChangePassword = func(string, string) error { return nil }
		model.openAdminPasswordModal()
		assertViewFits(t, model, size.width, size.height)

		model.openBulkImport()
		model.input = []rune(`{"sauth_json":"x"}`)
		model.inputCursor = len(model.input)
		assertViewFits(t, model, size.width, size.height)
	}
}

func TestSidebarAccountEntries(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageHome
	info := model.layoutInfo()
	if len(info.accountRects) != 2 || len(info.accountRowIndex) != 2 {
		t.Fatalf("sidebar account entries = %d/%d", len(info.accountRects), len(info.accountRowIndex))
	}
	row := info.accountRects[1]
	next, _ := model.Update(tea.MouseMsg{
		X: row.X + 1, Y: row.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if model.modal != modalAccountDashboard || model.accountActionIndex != 1 {
		t.Fatalf("dashboard modal/index = %v/%d", model.modal, model.accountActionIndex)
	}

	profilesIndex := 0
	for index, page := range pageOrder {
		if page == pageProfiles {
			profilesIndex = index
			break
		}
	}
	nav := model.layoutInfo().navRects[profilesIndex]
	next, _ = model.Update(tea.MouseMsg{
		X: nav.X + 1, Y: nav.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	model = next.(Model)
	if model.modal != modalNone || model.page != pageProfiles {
		t.Fatalf("nav click while dashboard open: modal=%v page=%v", model.modal, model.page)
	}
}

func TestLogSearchAndExport(t *testing.T) {
	manager := profileTestManager(t)
	model := sampleModel()
	model.manager = manager
	model.width, model.height = 100, 30
	model.page = pageLogs
	model.consoleTranscript = []consoleLine{
		{time: "12:00:00", level: "info", text: "hello world"},
		{time: "12:00:01", level: "info", text: "other line"},
	}

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model = next.(Model)
	if !model.logSearch {
		t.Fatal("/ should enter log search")
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	model = next.(Model)
	lines := model.combinedLogs()
	if len(lines) != 1 || !strings.Contains(lines[0].text, "hello world") {
		t.Fatalf("filtered logs = %#v", lines)
	}
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(Model)
	if model.logSearch {
		t.Fatal("Esc should leave log search")
	}

	model.exportLogs()
	entries, err := os.ReadDir(manager.Dir())
	if err != nil {
		t.Fatal(err)
	}
	var exported string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "cubedock-logs-") && strings.HasSuffix(entry.Name(), ".txt") {
			exported = filepath.Join(manager.Dir(), entry.Name())
		}
	}
	if exported == "" {
		t.Fatal("log export file not found")
	}
	raw, err := os.ReadFile(exported)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "hello world") || strings.Contains(string(raw), "other line") {
		t.Fatalf("export content = %q", raw)
	}
}

func TestLogin4399CaptchaDisplay(t *testing.T) {
	manager := profileTestManager(t)
	model := newModel(Options{Version: "v0.4.4-dev", LocalURL: "http://127.0.0.1:18321/"})
	model.manager = manager
	model.status.Running = false
	model.width, model.height = 100, 30
	model.page = pageAccounts
	model.openLoginModal()
	model.setLoginPlatform(string(netease.AuthMobile4399))
	model.loginCaptchaID = "captcha-1"
	model.loginCaptchaPath = filepath.Join(manager.Dir(), "captcha.png")
	model.loginCaptchaNote = "请输入图形验证码；图片已保存，Ctrl+O 打开"
	model.loginCaptchaInput = []rune("abcd")
	model.loginCaptchaCursor = len(model.loginCaptchaInput)
	model.loginField = 4
	if model.loginFieldCount() != 5 {
		t.Fatalf("login field count = %d", model.loginFieldCount())
	}
	view := model.View()
	if !strings.Contains(view, "4399") || !strings.Contains(view, "验证码") {
		t.Fatalf("4399 captcha UI missing: %q", view)
	}
	assertViewFits(t, model, model.width, model.height)
}

func TestSaveCaptchaImage(t *testing.T) {
	manager := profileTestManager(t)
	captcha := &m4399login.CaptchaRequired{
		CaptchaID: "id-1", Image: []byte{1, 2, 3, 4}, ImageType: "image/png",
	}
	path, note := saveCaptchaImage(manager, captcha)
	if path == "" || !strings.Contains(note, "已保存") {
		t.Fatalf("captcha path/note = %q/%q", path, note)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 4 {
		t.Fatalf("captcha bytes = %d", len(raw))
	}
}

func TestLogsFitWithSearch(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{34, 10}, {45, 20}, {100, 30}, {200, 60},
	} {
		model := sampleModel()
		model.width, model.height = size.width, size.height
		model.page = pageLogs
		model.logSearch = true
		model.logQuery = []rune("一段比较长的日志搜索词")
		model.logQueryCursor = len(model.logQuery)
		model.consoleTranscript = []consoleLine{
			{time: "12:00:00", level: "info", text: "一段比较长的日志搜索词 命中"},
		}
		assertViewFits(t, model, size.width, size.height)
	}
}

func TestInputBoxesAndStatusChips(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageAccounts
	view := model.View()
	if !strings.Contains(view, "● 在线 1") || !strings.Contains(view, "× 异常 1") {
		t.Fatalf("account status chips missing: %q", view)
	}

	model.status.Running = false
	model.width, model.height = 45, 20
	model.openLoginModal()
	model.loginEmail = []rune("alice@example.com")
	model.loginEmailCursor = len(model.loginEmail)
	model.loginPassword = []rune("secret")
	model.loginPasswordCursor = len(model.loginPassword)
	model.loginField = 1
	view = model.View()
	if !strings.Contains(view, "╭─ 登录信息") {
		t.Fatalf("login input box missing: %q", view)
	}
	if !strings.Contains(view, "保存密码到本机") || !strings.Contains(view, "取消") {
		t.Fatalf("narrow login controls clipped: %q", view)
	}
}

func TestNarrowModalsKeepActionsVisible(t *testing.T) {
	manager := profileTestManager(t)
	model := sampleModel()
	model.manager = manager
	model.status = manager.Snapshot()
	model.status.Running = false
	model.width, model.height = 45, 20

	model.page = pageSettings
	model.openSettingsEditor()
	view := model.View()
	if !strings.Contains(view, "保存") || !strings.Contains(view, "取消") {
		t.Fatalf("settings actions clipped at 45x20: %q", view)
	}

	model.options.ChangePassword = func(string, string) error { return nil }
	model.openAdminPasswordModal()
	view = model.View()
	if !strings.Contains(view, "保存") || !strings.Contains(view, "取消") {
		t.Fatalf("password actions clipped at 45x20: %q", view)
	}

	model.page = pageAccounts
	model.openImportModal()
	model.input = []rune(`{"sauth_json":"x"}`)
	model.inputCursor = len(model.input)
	view = model.View()
	if !strings.Contains(view, "导入") || !strings.Contains(view, "取消") {
		t.Fatalf("import actions clipped at 45x20: %q", view)
	}

	model.modal = modalNone
	model.openBulkImport()
	model.input = []rune(`{"sauth_json":"x"}`)
	model.inputCursor = len(model.input)
	view = model.View()
	if !strings.Contains(view, "开始导入") || !strings.Contains(view, "关闭") {
		t.Fatalf("bulk actions clipped at 45x20: %q", view)
	}
}

func TestDashboardChildModalReturnsToDashboard(t *testing.T) {
	manager := accountTestManager(t)
	model := sampleModel()
	model.manager = manager
	model.status = manager.Snapshot()
	model.status.Running = false
	model.selected = 0
	model.width, model.height = 100, 30
	model.page = pageAccounts
	model.openSelectedDashboard()
	if model.modal != modalAccountDashboard {
		t.Fatalf("dashboard modal = %v", model.modal)
	}

	next, _ := model.activateAccountAction(hitPlayersOpen)
	model = next.(Model)
	if model.modal != modalPlayers {
		t.Fatalf("players modal = %v", model.modal)
	}
	model.cancelModal()
	if model.modal != modalAccountDashboard {
		t.Fatalf("players close returned to %v, want dashboard", model.modal)
	}

	next, _ = model.activateAccountAction(hitAccountName)
	model = next.(Model)
	if model.modal != modalName {
		t.Fatalf("name modal = %v", model.modal)
	}
	model.cancelModal()
	if model.modal != modalAccountDashboard {
		t.Fatalf("name close returned to %v, want dashboard", model.modal)
	}

	next, _ = model.activateAccountAction(hitAccountAssign)
	model = next.(Model)
	if model.modal != modalAssignProfile {
		t.Fatalf("assign modal = %v", model.modal)
	}
	model.cancelModal()
	if model.modal != modalAccountDashboard {
		t.Fatalf("assign close returned to %v, want dashboard", model.modal)
	}

	next, _ = model.activateAccountAction(hitAccountRelogin)
	model = next.(Model)
	if model.modal != modalLogin {
		t.Fatalf("login modal = %v", model.modal)
	}
	model.cancelModal()
	if model.modal != modalAccountDashboard {
		t.Fatalf("login close returned to %v, want dashboard", model.modal)
	}
}

func TestResourcePacksRequireTransfer(t *testing.T) {
	manager := profileTestManager(t)
	model := sampleModel()
	model.manager = manager
	model.status = manager.Snapshot()
	model.status.Running = false
	model.page = pageProfiles
	model.profileIndex = 0
	model.openResourcePacks()
	if model.modal == modalResourcePacks {
		t.Fatal("proxy profile should not open the resource pack modal")
	}
	if !strings.Contains(model.notice, "转服") {
		t.Fatalf("proxy resource pack notice = %q", model.notice)
	}

	model.status.Config.Profiles[0].ConnectionMode = config.ConnectionTransfer
	model.openResourcePacks()
	if model.modal != modalResourcePacks {
		t.Fatalf("transfer profile packs modal = %v", model.modal)
	}
}

func TestAccountActionBarAtBottom(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.page = pageAccounts
	info := model.layoutInfo()
	login := toolbarIndexSpecial(info.toolbar, specialLogin)
	start := toolbarIndexAction(info.toolbar, actionStart)
	if login < 0 || start < 0 {
		t.Fatalf("login/start index = %d/%d", login, start)
	}
	if info.toolbarRects[login].W == 0 || info.toolbarRects[start].W == 0 {
		t.Fatal("bottom action bar or room actions not laid out")
	}
	if info.toolbarRects[login].Y <= info.toolbarRects[start].Y {
		t.Fatalf("global account actions at y=%d should be below room actions at y=%d",
			info.toolbarRects[login].Y, info.toolbarRects[start].Y)
	}
	for _, special := range []string{specialLogin, specialImport, specialBulk, specialAccountManage} {
		index := toolbarIndexSpecial(info.toolbar, special)
		if index < 0 || info.toolbarRects[index].W == 0 {
			t.Fatalf("bottom action %q missing", special)
		}
	}
}

func TestNameModalRenameMode(t *testing.T) {
	manager := accountTestManager(t)
	model := sampleModel()
	model.manager = manager
	model.status = manager.Snapshot()
	model.status.Running = false
	model.selected = 0
	model.width, model.height = 100, 30
	model.page = pageAccounts
	model.openAccountActions()
	next, _ := model.activateAccountAction(hitAccountName)
	model = next.(Model)
	if model.modal != modalName || !model.nameRename {
		t.Fatalf("modal/rename = %v/%v", model.modal, model.nameRename)
	}
	if got := string(model.nameInput); got != "Alice" {
		t.Fatalf("rename prefill = %q", got)
	}
	view := model.View()
	if strings.Contains(view, "还没有返回游戏名") {
		t.Fatalf("rename view still says the account has no name: %q", view)
	}
	if !strings.Contains(view, "修改已有游戏名") || !strings.Contains(view, "保存新游戏名") || !strings.Contains(view, "取消") {
		t.Fatalf("rename controls missing: %q", view)
	}
	model.cancelModal()
	if model.modal != modalAccountActions {
		t.Fatalf("rename cancel returned to %v", model.modal)
	}
}

func TestRandomNamesValid(t *testing.T) {
	for attempt := 0; attempt < 200; attempt++ {
		if name := randomChineseName(); !validPlayerName(name) {
			t.Fatalf("invalid Chinese name %q", name)
		}
		if name := randomEnglishName(); !validPlayerName(name) {
			t.Fatalf("invalid English name %q", name)
		}
	}
	chinese, english := makeNameSuggestions(3)
	if len(chinese) != 3 || len(english) != 3 {
		t.Fatalf("suggestions = %d/%d", len(chinese), len(english))
	}
}

func TestNameSuggestionsClickable(t *testing.T) {
	model := sampleModel()
	model.width, model.height = 100, 30
	model.openNameModal(0, "token", "interop")
	model.nameSuggestionsCN = []string{"星海旅人"}
	model.nameSuggestionsEN = []string{"PixelFox42"}
	info := model.layoutInfo()
	var candidateCN, candidateEN pageHit
	hasCN, hasEN := false, false
	for _, hit := range info.pageHits {
		switch hit.kind {
		case hitNameCandidateCN:
			candidateCN, hasCN = hit, true
		case hitNameCandidateEN:
			candidateEN, hasEN = hit, true
		}
	}
	if !hasCN || !hasEN {
		t.Fatal("name candidates are not clickable")
	}
	next, _ := model.Update(tea.MouseMsg{X: candidateCN.X + 1, Y: candidateCN.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	model = next.(Model)
	if got := string(model.nameInput); got != "星海旅人" {
		t.Fatalf("Chinese candidate click = %q", got)
	}
	next, _ = model.Update(tea.MouseMsg{X: candidateEN.X + 1, Y: candidateEN.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	model = next.(Model)
	if got := string(model.nameInput); got != "PixelFox42" {
		t.Fatalf("English candidate click = %q", got)
	}
}

func TestCtrlLOpensRoomNameChooser(t *testing.T) {
	manager := profileTestManager(t)
	model := sampleModel()
	model.manager = manager
	model.status = manager.Snapshot()
	model.status.Running = false
	model.width, model.height = 100, 30
	model.page = pageProfiles
	model.openProfileEditor()
	roomNameField := -1
	for index, spec := range model.profileFields() {
		if spec.Key == "room_name" {
			roomNameField = index
			break
		}
	}
	if roomNameField < 0 {
		t.Fatal("room name field not found")
	}
	model.profileField = roomNameField

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	model = next.(Model)
	if model.modal != modalProfileRoomName {
		t.Fatalf("Ctrl+L modal = %v", model.modal)
	}
	model.cancelModal()

	// It must also work while the text field is being edited.
	model.profileField = roomNameField
	model.profileFieldEdit = true
	model.profileBuffer = []rune("custom name")
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	model = next.(Model)
	if model.modal != modalProfileRoomName {
		t.Fatalf("Ctrl+L while editing modal = %v", model.modal)
	}
}

func TestTerminalTextSanitized(t *testing.T) {
	styled := "\x1b[31mred\x1b[0m"
	if got := sanitizeTerminalText(styled); got != styled {
		t.Fatalf("SGR sequence was stripped: %q", got)
	}
	malicious := "name\x1b]52;c;Zm9v\x07\x1b[2J"
	if got := sanitizeTerminalText(malicious); got != "name" {
		t.Fatalf("terminal escape survived: %q", got)
	}
	if got := sanitizeTerminalText("a‮b"); got != "ab" {
		t.Fatalf("bidi override survived: %q", got)
	}
	lines := wrapDisplay("a\x1b[31mb\nc", 10)
	if len(lines) != 2 || strings.Contains(lines[0], "\x1b") {
		t.Fatalf("multiline sanitize = %#v", lines)
	}
	if got := truncate("bad\x1b]0;title\x07name", 20); strings.Contains(got, "\x1b]") {
		t.Fatalf("truncate kept escape: %q", got)
	}
}
