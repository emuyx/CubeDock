package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// pageHitKind identifies a clickable region inside a page. Toolbar actions use
// hitToolbar with an index into toolbarItems(); the other kinds map to content
// such as a room row, a profile row, a form field or a modal button.
type pageHitKind int

const (
	hitToolbar pageHitKind = iota
	hitProfileField
	hitProfileSelect
	hitJumpRoom
	hitCreateAdmin
	hitOpenWebUI
	hitToggleLogFilter
	hitConsoleInput
	hitLogSearch
	hitLogExport
	hitLoginPlatform
	hitLoginField
	hitLoginRemember
	hitOpenLoginURL
	hitAdminField
	hitTagToggle
	hitRoomNamePick
	hitAccountRefresh
	hitAccountRelogin
	hitAccountAssign
	hitAccountDelete
	hitAssignProfile
	hitPlayersOpen
	hitMembersOpen
	hitFriendsOpen
	hitPlayerSelect
	hitMemberSelect
	hitFriendSelect
	hitPlayerKick
	hitPlayerTransfer
	hitPlayerRefresh
	hitMemberKick
	hitMemberRefresh
	hitFriendAccept
	hitFriendRefresh
	hitTransferSend
	hitPackSelect
	hitPacksRefresh
	hitPacksPull
	hitSettingsField
	hitAdminPasswordField
	hitBulkStart
	hitAccountSearch
	hitAccountFilter
	hitAccountSort
	hitNameRandomCN
	hitNameRandomEN
	hitNameRefresh
	hitNameCandidateCN
	hitNameCandidateEN
	hitAccountDashboard
	hitDashboardStart
	hitDashboardStop
	hitDashboardRestart
	hitDashboardDiagnostics
	hitAccountName
	hitModalSubmit
	hitModalCancel
)

type pageHit struct {
	rect
	kind  pageHitKind
	index int
}

const buttonPad = 2

// buttonText keeps every button the same size; the focus variant uses block
// markers so the focused state does not change the layout width.
func buttonText(label string) string {
	pad := strings.Repeat(" ", buttonPad)
	return pad + label + pad
}

func buttonFocusText(label string) string { return "▐ " + label + " ▌" }

func buttonWidth(label string) int { return lipgloss.Width(buttonText(label)) }

func sectionTitle(text string) string {
	return sectionStyle.Render("▌ ") + panelTitleStyle.Render(text)
}

// applyRuneKey applies simple single-line text editing keys to a rune buffer.
// Callers handle their own Esc/Enter semantics first.
func applyRuneKey(value *[]rune, cursor *int, key tea.KeyMsg, limit int) {
	switch key.String() {
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
			if limit > 0 {
				room := limit - len(*value)
				if room < 0 {
					room = 0
				}
				if len(runes) > room {
					runes = runes[:room]
				}
			}
			next := make([]rune, 0, len(*value)+len(runes))
			next = append(next, (*value)[:*cursor]...)
			next = append(next, runes...)
			next = append(next, (*value)[*cursor:]...)
			*value = next
			*cursor += len(runes)
		}
	}
}

type customButton struct {
	label   string
	kind    pageHitKind
	index   int
	active  bool
	focused bool
}

// pageBuilder renders one page and records the clickable regions it emitted.
// Coordinates are page-local: X is a column in the combined page line and Y is
// the line index before the frame windows/scrolled it.
type pageBuilder struct {
	m       Model
	width   int
	originX int
	originY int
	offset  int
	colBase int
	lines   []string
	hits    []pageHit
}

func newPageBuilder(m Model, width, originX, originY int) *pageBuilder {
	return &pageBuilder{
		m: m, width: maxInt(1, width),
		originX: originX, originY: originY, offset: m.contentOffset,
	}
}

func (b *pageBuilder) add(line string) { b.lines = append(b.lines, line) }
func (b *pageBuilder) blank()          { b.add("") }

func (b *pageBuilder) hit(line, col, width int, kind pageHitKind, index int) {
	if width < 1 {
		width = 1
	}
	b.hits = append(b.hits, pageHit{
		rect:  rect{X: b.colBase + col, Y: line, W: width, H: 1},
		kind:  kind,
		index: index,
	})
}

func (b *pageBuilder) hovered(line, col, width int) bool {
	if !b.m.mouseActive {
		return false
	}
	x := b.originX + b.colBase + col
	y := b.originY + line - b.offset
	return b.m.mouseX >= x && b.m.mouseX < x+width && b.m.mouseY == y
}

// buttonFlow renders toolbar buttons in order, wrapping at the page width. The
// indices refer to Model.toolbarItems(); every item for the page should be
// rendered exactly once so keyboard focus and mouse hitboxes agree.
func (b *pageBuilder) buttonFlow(prefix string, indices ...int) {
	items := b.m.toolbarItems()
	buttons := make([]customButton, 0, len(indices))
	for _, index := range indices {
		if index < 0 || index >= len(items) {
			continue
		}
		buttons = append(buttons, customButton{
			label: items[index].Label, kind: hitToolbar, index: index,
		})
	}
	b.customButtonFlow(prefix, buttons...)
}

func (b *pageBuilder) customButtonFlow(prefix string, buttons ...customButton) {
	if len(buttons) == 0 {
		if prefix != "" {
			b.add(dimStyle.Render(prefix))
		}
		return
	}
	line := len(b.lines)
	col := 0
	first := true
	var sb strings.Builder
	if prefix != "" {
		sb.WriteString(dimStyle.Render(prefix))
		col = lipgloss.Width(prefix)
	}
	for _, button := range buttons {
		focused := button.focused ||
			(button.kind == hitToolbar && b.m.focus == focusActions && b.m.actionIndex == button.index)
		text := buttonText(button.label)
		if focused || button.active {
			text = buttonFocusText(button.label)
		}
		width := lipgloss.Width(text)
		gap := 0
		if !first {
			gap = 1
		}
		if col > 0 && col+gap+width > b.width {
			b.lines = append(b.lines, sb.String())
			sb.Reset()
			line = len(b.lines)
			col = 0
			gap = 0
		}
		if gap > 0 {
			sb.WriteString(" ")
			col++
		}
		hovered := b.hovered(line, col, width)
		style := buttonStyle
		switch {
		case focused:
			style = buttonFocusStyle
		case button.active:
			style = tabActiveStyle
		case hovered:
			style = buttonHoverStyle
		}
		sb.WriteString(style.Render(text))
		b.hit(line, col, width, button.kind, button.index)
		col += width
		first = false
	}
	b.lines = append(b.lines, sb.String())
}

func toolbarIndexAction(items []toolbarItem, action actionKind) int {
	for index, item := range items {
		if item.Special == "" && item.Action == action {
			return index
		}
	}
	return -1
}

func toolbarIndexSpecial(items []toolbarItem, special string) int {
	for index, item := range items {
		if item.Special == special {
			return index
		}
	}
	return -1
}
