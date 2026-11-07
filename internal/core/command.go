package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maxConsoleCommandRunes = 512

// CommandResult is the response of one console command. Output is rendered by
// the browser as a session-local transcript; only side effects such as a kick
// are written to the persistent event log.
type CommandResult struct {
	OK     bool     `json:"ok"`
	Output []string `json:"output"`
}

// RunCommand parses and executes one console line.
func (m *Manager) RunCommand(line string) (CommandResult, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return CommandResult{}, errors.New("请输入命令；输入 help 查看可用命令")
	}
	if utf8.RuneCountInString(line) > maxConsoleCommandRunes {
		return CommandResult{}, fmt.Errorf("命令过长（最多 %d 个字符）", maxConsoleCommandRunes)
	}
	command, arg, err := parseConsoleLine(line)
	if err != nil {
		return CommandResult{}, err
	}

	switch command {
	case "help":
		if arg != "" {
			return CommandResult{}, errors.New("用法：help")
		}
		return CommandResult{OK: true, Output: helpLines()}, nil
	case "list":
		if arg != "" {
			return CommandResult{}, errors.New("用法：list")
		}
		return CommandResult{OK: true, Output: m.listPlayerLines()}, nil
	case "kick":
		if arg == "" {
			return CommandResult{}, errors.New("用法：kick <玩家ID|名称>")
		}
		target, err := unquoteConsoleArg(arg)
		if err != nil {
			return CommandResult{}, err
		}
		if target == "" {
			return CommandResult{}, errors.New("请指定玩家 ID 或名称")
		}
		status, err := m.KickPlayer(target)
		if err != nil {
			return CommandResult{}, err
		}
		return CommandResult{OK: true, Output: []string{
			fmt.Sprintf("已请求断开玩家 %s（%s，房间 %d）。", playerLabel(status.Name), status.ID, status.Room),
		}}, nil
	case "transfer":
		target, address, err := splitTransferArg(arg)
		if err != nil {
			return CommandResult{}, err
		}
		status, err := m.TransferPlayer(target, address)
		if err != nil {
			return CommandResult{}, err
		}
		return CommandResult{OK: true, Output: []string{
			fmt.Sprintf("已向玩家 %s（%s）发送转服包：%s。", playerLabel(status.Name), status.ID, address),
		}}, nil
	default:
		return CommandResult{}, fmt.Errorf("未知命令 %q；可用命令：help、list、kick、transfer", command)
	}
}

// splitTransferArg splits "player target host:port" from the right so player
// names may contain spaces. The address itself must not contain spaces.
func splitTransferArg(arg string) (target, address string, err error) {
	arg = strings.TrimSpace(arg)
	const usage = "用法：transfer <玩家ID|名称> <host:port>"
	if arg == "" {
		return "", "", errors.New(usage)
	}
	index := strings.LastIndexFunc(arg, unicode.IsSpace)
	if index < 0 {
		return "", "", errors.New(usage)
	}
	targetPart := strings.TrimSpace(arg[:index])
	address = strings.TrimSpace(arg[index:])
	if targetPart == "" || address == "" {
		return "", "", errors.New(usage)
	}
	target, err = unquoteConsoleArg(targetPart)
	if err != nil {
		return "", "", err
	}
	if target == "" {
		return "", "", errors.New("请指定玩家 ID 或名称")
	}
	return target, address, nil
}

func parseConsoleLine(line string) (command, arg string, err error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", "", errors.New("请输入命令；输入 help 查看可用命令")
	}
	index := strings.IndexFunc(line, unicode.IsSpace)
	if index < 0 {
		return strings.ToLower(line), "", nil
	}
	return strings.ToLower(line[:index]), strings.TrimSpace(line[index:]), nil
}

// unquoteConsoleArg accepts a bare remainder (which may contain spaces) or one
// quoted argument. Backslash escapes the next character in either form.
func unquoteConsoleArg(arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", errors.New("请指定玩家 ID 或名称")
	}

	var quote rune
	first, size := utf8.DecodeRuneInString(arg)
	if first == '\'' || first == '"' {
		quote = first
		arg = arg[size:]
	}

	var out strings.Builder
	escaped := false
	for index, r := range arg {
		if escaped {
			out.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 && r == quote {
			rest := strings.TrimSpace(arg[index+utf8.RuneLen(r):])
			if rest != "" {
				return "", fmt.Errorf("引号后还有多余内容：%s", rest)
			}
			return out.String(), nil
		}
		out.WriteRune(r)
	}
	if escaped {
		return "", errors.New("参数不能以反斜杠结尾")
	}
	if quote != 0 {
		return "", errors.New("参数缺少结束引号")
	}
	return strings.TrimSpace(out.String()), nil
}

func helpLines() []string {
	return []string{
		"可用命令：",
		"  help                 显示本帮助",
		"  list                 列出当前通过 CubeDock 连接的玩家",
		"  kick <ID|名称>       断开指定玩家；名称含空格时请加引号",
		"  transfer <ID|名称> <host:port>  发送 Transfer 包让玩家直连目标服务器（POC）",
		"示例：",
		"  list",
		"  kick p3",
		`  kick "Alice Smith"`,
		"  transfer p3 192.0.2.10:19132",
	}
}

func (m *Manager) listPlayerLines() []string {
	players := m.ListPlayers()
	if len(players) == 0 {
		return []string{"当前没有玩家连接。"}
	}
	lines := []string{fmt.Sprintf("当前共有 %d 位玩家：", len(players))}
	for _, player := range players {
		line := fmt.Sprintf("%s  %s  房间 %d", player.ID, playerLabel(player.Name), player.Room)
		if player.RoomID != 0 {
			line += fmt.Sprintf("（%d）", player.RoomID)
		}
		line += fmt.Sprintf("  %s  已连接 %s  上行 %d 包 / 下行 %d 包",
			playerStateLabel(player.State),
			shortDuration(time.Duration(player.UptimeSeconds)*time.Second),
			player.UpPackets, player.DownPackets)
		lines = append(lines, line)
	}
	return lines
}

func playerStateLabel(state PlayerState) string {
	switch state {
	case PlayerConnecting:
		return "连接中"
	case PlayerAuthenticating:
		return "登录中"
	case PlayerJoined:
		return "在线"
	case PlayerLoginFailed:
		return "登录失败"
	case PlayerKicked:
		return "断开中"
	default:
		return string(state)
	}
}
