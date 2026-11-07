// Package core owns CubeDock's room lifecycle, account list and event log. It
// has no HTTP dependencies so the console is only one possible front end.
package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Happy2018new/nemc-tan-lobby-solver/core/nethernet"
	"github.com/emuyx/cubedock/internal/auth"
	"github.com/emuyx/cubedock/internal/bridge"
	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/fileutil"
	"github.com/emuyx/cubedock/internal/friends"
	"github.com/emuyx/cubedock/internal/redact"
	"github.com/emuyx/cubedock/internal/relay"
	"github.com/emuyx/cubedock/internal/room"
	"github.com/emuyx/cubedock/internal/secret"
	"github.com/emuyx/cubedock/internal/session"
)

// MaxRooms bounds how many accounts (one room each) may run at once.
const MaxRooms = 8

// maxEvents bounds the in-memory console history returned by /api/status.
const maxEvents = 300

const (
	tokenFile   = "room-tokens.dat"
	accountFile = "accounts.json"
	statusFile  = "logs/status.json"
)

// Event is one console/log line.
type Event struct {
	Time    string `json:"time"`
	Source  string `json:"source"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// RoomStatus describes one hosted room.
type RoomStatus struct {
	Phase       string `json:"phase"`
	RoomID      uint32 `json:"room_id"`
	Players     int    `json:"players"`
	Account     string `json:"account"`
	ProtocolID  uint8  `json:"protocol_id"`
	ConnectedAt string `json:"connected_at"`
	RetryAt     string `json:"retry_at"`
	Attempts    int    `json:"attempts"`
	LastError   string `json:"last_error"`
}

// Status is the console snapshot. The top-level room fields mirror the first
// room for older clients; Players is the total across rooms.
type Status struct {
	Phase           string          `json:"phase"`
	Running         bool            `json:"running"`
	RoomID          uint32          `json:"room_id"`
	Players         int             `json:"players"`
	Account         string          `json:"account"`
	ProtocolID      uint8           `json:"protocol_id"`
	ConnectedAt     string          `json:"connected_at"`
	RetryAt         string          `json:"retry_at"`
	Attempts        int             `json:"attempts"`
	LastError       string          `json:"last_error"`
	UpstreamOnline  bool            `json:"upstream_online"`
	UpstreamVersion string          `json:"upstream_version"`
	TokenSet        bool            `json:"token_set"`
	TokenCount      int             `json:"token_count"`
	Rooms           []RoomStatus    `json:"rooms"`
	Accounts        []AccountStatus `json:"accounts"`
	Config          config.Config   `json:"config"`
	Events          []Event         `json:"events"`
}

// Manager coordinates all rooms owned by this process.
type Manager struct {
	dir    string
	logger *log.Logger

	mu       sync.Mutex
	cfg      config.Config
	tokens   []string
	accounts []accountRecord
	status   Status
	cancel   context.CancelFunc
	finished chan struct{}

	players playerRegistry
	limbo   *limboProcess

	exit     chan struct{}
	exitOnce sync.Once
}

// New loads account metadata and returns a stopped manager.
func New(dir string, cfg config.Config, tokens []string, logger *log.Logger) (*Manager, error) {
	accounts, err := loadAccounts(filepath.Join(dir, accountFile), len(tokens))
	if err != nil {
		return nil, fmt.Errorf("读取账号列表失败: %w", err)
	}
	m := &Manager{
		dir:      dir,
		logger:   logger,
		cfg:      cfg,
		tokens:   append([]string(nil), tokens...),
		accounts: accounts,
		status:   Status{Phase: "stopped", Events: []Event{}},
		exit:     make(chan struct{}),
	}
	m.persist()
	return m, nil
}

// Dir returns the data directory.
func (m *Manager) Dir() string { return m.dir }

// Exit closes when the console requests process shutdown.
func (m *Manager) Exit() <-chan struct{} { return m.exit }

// RequestExit asks the main loop to shut down.
func (m *Manager) RequestExit() {
	m.exitOnce.Do(func() { close(m.exit) })
}

// phaseRank orders room phases; a joinable room outranks one still connecting.
var phaseRank = map[string]int{
	"online": 0, "creating": 1, "authenticating": 2, "checking_upstream": 3,
	"retrying": 4, "upstream_offline": 5, "stopped": 6,
}

// Snapshot returns a copy of the current status.
func (m *Manager) Snapshot() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.status
	s.Config = m.cfg
	s.TokenSet = len(m.tokens) > 0
	s.TokenCount = len(m.tokens)
	s.Events = append([]Event{}, s.Events...)
	s.Rooms = append([]RoomStatus{}, s.Rooms...)
	s.Accounts = m.accountStatusesLocked()
	if len(s.Rooms) == 0 {
		return s
	}
	first := s.Rooms[0]
	s.RoomID, s.Account, s.ProtocolID = first.RoomID, first.Account, first.ProtocolID
	s.ConnectedAt, s.RetryAt, s.Attempts = first.ConnectedAt, first.RetryAt, first.Attempts
	s.Players, s.LastError = 0, ""
	best := len(phaseRank)
	for index, status := range s.Rooms {
		s.Players += status.Players
		if s.LastError == "" && status.LastError != "" {
			s.LastError = roomLabel(index, len(s.Rooms)) + status.LastError
		}
		if rank, ok := phaseRank[status.Phase]; ok && rank < best {
			best = rank
		}
	}
	for phase, rank := range phaseRank {
		if rank == best {
			s.Phase = phase
		}
	}
	return s
}

func roomLabel(index, count int) string {
	if count <= 1 {
		return ""
	}
	return fmt.Sprintf("[房间 %d] ", index+1)
}

func (m *Manager) emit(source, message string)  { m.record(source, message, false) }
func (m *Manager) trace(source, message string) { m.record(source, message, true) }

// Record writes a console event from outside the manager (for example an
// account-login adapter) using the same redaction and log file.
func (m *Manager) Record(source, message string) { m.emit(source, message) }

func (m *Manager) record(source, message string, debug bool) {
	m.mu.Lock()
	if debug && !m.cfg.Debug {
		m.mu.Unlock()
		return
	}
	if source == "" {
		source = "系统"
	}
	level := "info"
	if debug {
		level = "debug"
		message = "[DEBUG] " + message
	}
	for _, token := range m.tokens {
		message = redact.Sanitize(message, token)
	}
	event := Event{Time: time.Now().Format("15:04:05"), Source: source, Level: level, Message: message}
	m.status.Events = append(m.status.Events, event)
	if len(m.status.Events) > maxEvents {
		m.status.Events = m.status.Events[len(m.status.Events)-maxEvents:]
	}
	m.mu.Unlock()

	if m.logger != nil {
		m.logger.Println("[" + source + "] " + message)
	}
}

func (m *Manager) updateStatus(change func(*Status)) {
	m.mu.Lock()
	change(&m.status)
	m.mu.Unlock()
	m.persist()
}

func (m *Manager) updateRoom(index int, change func(*RoomStatus)) {
	m.mu.Lock()
	if index >= 0 && index < len(m.status.Rooms) {
		change(&m.status.Rooms[index])
	}
	m.mu.Unlock()
	m.persist()
}

// persist writes a status snapshot without secrets. Failures are non-fatal.
func (m *Manager) persist() {
	status := m.Snapshot()
	status.Config.RoomPassword = ""
	status.Events = nil
	raw, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return
	}
	_ = fileutil.Replace(filepath.Join(m.dir, statusFile), raw, 0600)
}

// Configure saves settings. The web port is intentionally fixed until restart.
func (m *Manager) Configure(next config.Config) error {
	next.Normalize()
	if err := next.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return fmt.Errorf("请先停止房间，再修改配置")
	}
	if next.WebPort != m.cfg.WebPort {
		return fmt.Errorf("控制台端口需关闭程序后在 config.json 修改")
	}
	if err := config.Save(filepath.Join(m.dir, "config.json"), next); err != nil {
		return err
	}
	m.cfg = next
	return nil
}

// SetDebug toggles detailed logging and persists the choice.
func (m *Manager) SetDebug(enabled bool) error {
	m.mu.Lock()
	next := m.cfg
	next.Debug = enabled
	if err := config.Save(filepath.Join(m.dir, "config.json"), next); err != nil {
		m.mu.Unlock()
		return err
	}
	m.cfg = next
	m.mu.Unlock()
	if enabled {
		m.emit("系统", "Debug 模式已开启")
	} else {
		m.emit("系统", "Debug 模式已关闭")
	}
	return nil
}

// Token returns the stored session for one room.
func (m *Manager) Token(index int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < 0 || index >= len(m.tokens) {
		return "", fmt.Errorf("账号序号无效")
	}
	return m.tokens[index], nil
}

// AddAccount appends a session and its metadata, returning the room index.
func (m *Manager) AddAccount(token string, meta AccountMeta) (int, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return -1, fmt.Errorf("账号会话为空")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return -1, fmt.Errorf("请先停止房间，再添加账号")
	}
	if len(m.tokens) >= MaxRooms {
		return -1, fmt.Errorf("最多同时开 %d 个房间", MaxRooms)
	}
	for _, existing := range m.tokens {
		if existing == token {
			// One account cannot keep two rooms alive; the second login drops the first.
			return -1, fmt.Errorf("这个网易账号已经添加过了，请先移除或更换账号")
		}
	}
	nextTokens := append(append([]string(nil), m.tokens...), token)
	nextAccounts := append(append([]accountRecord(nil), m.accounts...), recordFromMeta(meta))
	if err := m.persistIdentityLocked(nextTokens, nextAccounts); err != nil {
		return -1, err
	}
	m.tokens, m.accounts = nextTokens, nextAccounts
	return len(nextTokens) - 1, nil
}

// ReplaceAccount swaps the session and metadata of an existing room.
func (m *Manager) ReplaceAccount(index int, token string, meta AccountMeta) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("账号会话为空")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return fmt.Errorf("请先停止房间，再更新账号")
	}
	if index < 0 || index >= len(m.tokens) {
		return fmt.Errorf("账号序号无效")
	}
	for i, existing := range m.tokens {
		if i != index && existing == token {
			return fmt.Errorf("这个网易账号已经用于房间 %d，不能重复使用", i+1)
		}
	}
	nextTokens := append([]string(nil), m.tokens...)
	nextTokens[index] = token
	nextAccounts := append([]accountRecord(nil), m.accounts...)
	nextAccounts[index] = recordFromMeta(meta)
	if err := m.persistIdentityLocked(nextTokens, nextAccounts); err != nil {
		return err
	}
	m.tokens, m.accounts = nextTokens, nextAccounts
	return nil
}

// RemoveAccount removes a room's session and metadata.
func (m *Manager) RemoveAccount(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return fmt.Errorf("请先停止房间，再移除账号")
	}
	if index < 0 || index >= len(m.tokens) {
		return fmt.Errorf("账号序号无效")
	}
	nextTokens := append([]string(nil), m.tokens[:index]...)
	nextTokens = append(nextTokens, m.tokens[index+1:]...)
	nextAccounts := append([]accountRecord(nil), m.accounts[:index]...)
	nextAccounts = append(nextAccounts, m.accounts[index+1:]...)
	if err := m.persistIdentityLocked(nextTokens, nextAccounts); err != nil {
		return err
	}
	m.tokens, m.accounts = nextTokens, nextAccounts
	return nil
}

// UpdateAccountProfile updates the resolved game identity or its error.
func (m *Manager) UpdateAccountProfile(index int, name, uid string, level int, profileErr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < 0 || index >= len(m.accounts) {
		return fmt.Errorf("账号序号无效")
	}
	next := append([]accountRecord(nil), m.accounts...)
	next[index].Name = name
	next[index].UID = uid
	next[index].Level = level
	next[index].ProfileError = profileErr
	if profileErr == "" && name != "" {
		next[index].VerifiedAt = time.Now().Format(time.RFC3339)
	}
	if err := saveAccounts(filepath.Join(m.dir, accountFile), next); err != nil {
		return err
	}
	m.accounts = next
	return nil
}

func recordFromMeta(meta AccountMeta) accountRecord {
	record := accountRecord{
		Email: meta.Email, Label: meta.Label, Name: meta.Name, UID: meta.UID,
		Level: meta.Level, Platform: meta.Platform, ExpiresAt: meta.ExpiresAt,
		ProfileError: meta.ProfileError, VerifiedAt: meta.Verified,
	}
	if record.VerifiedAt == "" && record.Name != "" {
		record.VerifiedAt = time.Now().Format(time.RFC3339)
	}
	return record
}

// persistIdentityLocked writes both the protected token list and the account
// metadata, restoring the previous token file if the second write fails.
func (m *Manager) persistIdentityLocked(tokens []string, accounts []accountRecord) error {
	tokenPath := filepath.Join(m.dir, tokenFile)
	previous := strings.Join(m.tokens, "\n")
	if err := saveTokens(tokenPath, tokens); err != nil {
		return err
	}
	if err := saveAccounts(filepath.Join(m.dir, accountFile), accounts); err != nil {
		_ = saveTokens(tokenPath, splitTokens(previous))
		return fmt.Errorf("保存账号信息失败: %w", err)
	}
	return nil
}

func saveTokens(path string, tokens []string) error {
	if len(tokens) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return secret.Save(path, strings.Join(tokens, "\n"))
}

func splitTokens(raw string) []string { return strings.Fields(raw) }

// Start launches one room goroutine per account. Calling Start while running is
// a no-op.
func (m *Manager) Start() error {
	if m.cfg.ConnectionMode == config.ConnectionTransfer {
		m.mu.Lock()
		running := m.cancel != nil
		hasTokens := len(m.tokens) > 0
		m.mu.Unlock()
		if m.cfg.LimboAutoPullPacks && !running && hasTokens {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			output, pullErr := m.pullResourcePacks(ctx)
			cancel()
			dir, dirErr := EnsureLimboPacksDir(m.cfg)
			switch {
			case pullErr != nil:
				m.Record("资源包", "自动拉取后端资源包失败："+pullErr.Error())
			default:
				m.Record("资源包", "已从后端服拉取资源包")
				if output != "" && m.logger != nil {
					m.logger.Printf("[packs] %s", output)
				}
			}
			if dirErr == nil {
				removed, warnings, dedupeErr := dedupeResourcePacks(dir)
				if dedupeErr != nil {
					m.Record("资源包", "清理重复资源包失败："+dedupeErr.Error())
				} else if len(removed) > 0 {
					m.Record("资源包", fmt.Sprintf("已删除 %d 个重复资源包：%s",
						len(removed), strings.Join(removed, ", ")))
				}
				for _, warning := range warnings {
					m.Record("资源包", warning)
				}
			}
		}
		if err := m.ensureLimbo(); err != nil {
			return err
		}
	}
	m.mu.Lock()
	success := false
	defer func() {
		if !success {
			m.stopLimbo()
		}
	}()
	defer m.mu.Unlock()
	if m.cancel != nil {
		success = true
		return nil
	}
	if len(m.tokens) == 0 {
		return fmt.Errorf("请先添加网易账号")
	}
	// Acquire every account lease before starting anything, so a busy account
	// cannot leave the remaining rooms half-started.
	leases := make([]*session.Lease, 0, len(m.tokens))
	for index, token := range m.tokens {
		lease, err := session.Claim(token)
		if err != nil {
			for _, held := range leases {
				held.Release()
			}
			if len(m.tokens) > 1 {
				return fmt.Errorf("%s%w", roomLabel(index, len(m.tokens)), err)
			}
			return err
		}
		leases = append(leases, lease)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.finished = make(chan struct{})
	m.status.Running = true
	m.status.LastError = ""
	m.status.Attempts = 0
	m.status.Rooms = make([]RoomStatus, len(m.tokens))
	for index := range m.status.Rooms {
		m.status.Rooms[index] = RoomStatus{Phase: "checking_upstream"}
		if index < len(m.accounts) {
			m.status.Rooms[index].Account = firstNonEmpty(m.accounts[index].Name, m.accounts[index].Label, m.accounts[index].Email)
		}
	}
	cfg := m.cfg
	tokens := append([]string(nil), m.tokens...)
	finished := m.finished

	var rooms sync.WaitGroup
	for index, token := range tokens {
		rooms.Add(1)
		go func(index int, token string, lease *session.Lease) {
			defer rooms.Done()
			defer lease.Release()
			m.run(ctx, index, cfg, token)
		}(index, token, leases[index])
	}
	go func() {
		rooms.Wait()
		m.updateStatus(func(status *Status) {
			status.Running = false
			status.Phase = "stopped"
			status.RoomID = 0
			status.Players = 0
			status.Account = ""
			status.ProtocolID = 0
			status.ConnectedAt = ""
			status.RetryAt = ""
			status.LastError = ""
			status.Rooms = nil
		})
		m.mu.Lock()
		m.cancel = nil
		close(finished)
		m.mu.Unlock()
		m.stopLimbo()
		m.emit("系统", "房间已停止")
	}()
	success = true
	return nil
}

// Stop asks every room to shut down.
func (m *Manager) Stop() {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Unlock()
	m.stopLimbo()
}

// Wait blocks until all rooms have stopped.
func (m *Manager) Wait() {
	m.mu.Lock()
	done := m.finished
	m.mu.Unlock()
	if done != nil {
		<-done
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func playerLabel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "未知玩家"
	}
	return name
}

func shortDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d 秒", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d 分 %d 秒", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%d 时 %d 分", int(d.Hours()), int(d.Minutes())%60)
}

// roomOccupancy returns the current guest count for console messages. The host
// bot occupies one slot, so the guest capacity is maxPlayers-1.
func (m *Manager) roomOccupancy(index, maxPlayers int) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < 0 || index >= len(m.status.Rooms) {
		return ""
	}
	return fmt.Sprintf("（房间在线 %d/%d）", m.status.Rooms[index].Players, maxPlayers-1)
}

type roomLog struct {
	manager *Manager
	source  string
}

func (l roomLog) event(message string) { l.manager.emit(l.source, message) }
func (l roomLog) debug(message string) { l.manager.trace(l.source, message) }

func (m *Manager) run(ctx context.Context, index int, cfg config.Config, token string) {
	rl := roomLog{manager: m, source: fmt.Sprintf("房间 %d", index+1)}
	rl.event("启动本地联机房主机器人")
	rl.event("直通模式 + 网易 RakNet v8：原始游戏批次直接转发，代客户端完成登录重签与加密握手")

	failures := 0
	for ctx.Err() == nil {
		m.updateRoom(index, func(status *RoomStatus) {
			status.Phase = "checking_upstream"
			status.RoomID = 0
			status.Players = 0
			status.RetryAt = ""
			status.Attempts++
		})

		version, err := pingUpstream(ctx, cfg)
		if err == nil {
			m.updateStatus(func(status *Status) {
				status.UpstreamOnline = true
				status.UpstreamVersion = version
			})
			m.updateRoom(index, func(status *RoomStatus) { status.Phase = "authenticating" })
			rl.debug("本地 Bedrock 服务器在线，版本 " + version + "；开始鉴权")

			var info auth.RoomInfo
			authCtx, cancel := context.WithTimeout(ctx, 75*time.Second)
			info, err = auth.PrepareRoom(authCtx, cfg, token)
			cancel()
			if err == nil {
				m.updateRoom(index, func(status *RoomStatus) {
					status.Account = info.PlayerName
					status.ProtocolID = info.ProtocolID
					status.Phase = "creating"
				})
				rl.debug(fmt.Sprintf("鉴权通过，房间协议 %d；连接开房节点", info.ProtocolID))
				var host *room.Host
				createCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
				host, err = room.Open(createCtx, cfg, info, rl.debug)
				cancel()
				if err == nil {
					failures = 0
					m.updateRoom(index, func(status *RoomStatus) {
						status.Phase = "online"
						status.RoomID = host.ID
						status.ConnectedAt = time.Now().Format(time.RFC3339)
						status.LastError = ""
					})
					rl.event(fmt.Sprintf("房间 %d 已上线（协议 %d，上游 %s），房主机器人持续保活",
						host.ID, info.ProtocolID, upstreamDescription(cfg)))
					err = m.serveRoom(ctx, index, cfg, host, info, rl)
					host.Close()
				}
			}
		} else {
			m.updateStatus(func(status *Status) { status.UpstreamOnline = false })
			err = fmt.Errorf("本地服务器 %s 不可达，请先启动 Bedrock 服务器", upstreamAddress(cfg))
		}
		if ctx.Err() != nil {
			return
		}

		failures++
		delay := time.Duration(cfg.RetrySeconds) * time.Second
		for i := 1; i < failures && delay < 90*time.Second; i++ {
			delay *= 2
		}
		if delay > 90*time.Second {
			delay = 90 * time.Second
		}
		if err == nil {
			err = net.ErrClosed
		}
		message := redact.Sanitize(err.Error(), token)
		m.updateRoom(index, func(status *RoomStatus) {
			status.Phase = "retrying"
			status.LastError = message
			status.RoomID = 0
			status.Players = 0
			status.RetryAt = time.Now().Add(delay).Format(time.RFC3339)
		})
		rl.event(fmt.Sprintf("%s；%.0f 秒后重试", message, delay.Seconds()))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (m *Manager) serveRoom(ctx context.Context, index int, cfg config.Config, host *room.Host, info auth.RoomInfo, rl roomLog) error {
	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	friendsDone := make(chan struct{})
	go func() {
		defer close(friendsDone)
		if cfg.AutoAcceptFriends {
			friends.Watch(sessionCtx, info.UserID, info.EncryptKey, rl.event, rl.debug)
		}
	}()
	defer func() { cancel(); <-friendsDone }()

	var bridges sync.WaitGroup
	acceptDone := make(chan struct{})
	defer func() {
		cancel()
		host.Close()
		<-acceptDone
		bridges.Wait()
	}()

	acceptErrors := make(chan error, 1)
	go func() {
		defer close(acceptDone)
		for {
			conn, err := host.Accept()
			if err != nil {
				acceptErrors <- err
				return
			}
			if conn == nil {
				acceptErrors <- net.ErrClosed
				return
			}
			// NetEase counts the host bot, so a room admits MaxPlayers-1 guests.
			full := true
			m.updateRoom(index, func(status *RoomStatus) {
				full = status.Players >= cfg.MaxPlayers-1
				if !full {
					status.Players++
				}
			})
			if full {
				conn.Close()
				rl.event(fmt.Sprintf("房间已满（最多 %d 位玩家），拒绝了一个新连接", cfg.MaxPlayers-1))
				continue
			}
			bridges.Add(1)
			go func(client *nethernet.Conn) {
				startedAt := time.Now()
				playerCtx, playerCancel := context.WithCancel(sessionCtx)
				guest := m.players.register(index, host.ID, info.PlayerName, playerCancel)
				defer playerCancel()
				defer bridges.Done()
				defer client.Close()
				defer m.updateRoom(index, func(status *RoomStatus) {
					if status.Players > 0 {
						status.Players--
					}
				})
				defer m.players.unregister(guest)
				rl.debug("玩家连接已建立，正在转发到本地服务器")
				dialCtx, stop := context.WithTimeout(playerCtx, 8*time.Second)
				server, err := dialUpstream(dialCtx, cfg)
				stop()
				if err != nil {
					if playerCtx.Err() == nil {
						rl.event("有玩家尝试进入，但本地服务器连接失败")
					}
					return
				}
				defer server.Close()
				rl.debug("本地服务器 RakNet 连接成功")
				encrypted, err := bridge.NewCryptoRelay(server, rl.debug)
				if err != nil {
					rl.event("无法生成后端连接密钥")
					return
				}
				encrypted.SetPlayerHooks(
					func(info bridge.PlayerInfo) {
						m.players.noteLogin(guest, info.Name)
						rl.debug(fmt.Sprintf("已读取玩家登录身份：%s（UID %s）",
							playerLabel(info.Name), firstNonEmpty(info.UID, "—")))
					},
					func(info bridge.PlayerInfo, status int32) {
						m.players.notePlayStatus(guest, status)
						if status == 0 {
							rl.event(fmt.Sprintf("玩家 %s 已加入服务器%s",
								playerLabel(info.Name), m.roomOccupancy(index, cfg.MaxPlayers)))
							return
						}
						rl.event(fmt.Sprintf("玩家 %s 登录失败：%s",
							playerLabel(info.Name), bridge.PlayStatusName(status)))
					},
				)
				rl.debug("网易上游：已代客户端完成登录重签与加密握手，游戏包直通")
				clientSync := &relay.SyncStream{Stream: client}
				clientStream := &relay.Observer{Stream: clientSync, Label: "玩家 → 本地服务器", Trace: rl.debug}
				serverStream := &relay.Observer{Stream: encrypted, Label: "本地服务器 → 玩家", Trace: rl.debug}
				m.players.attachStreams(guest, &clientStream.Packets, &serverStream.Packets)
				m.players.attachClient(guest, clientSync)
				err = relay.Pipe(playerCtx, clientStream, serverStream)
				if err != nil {
					rl.debug("转发结束：" + err.Error())
				}
				player := encrypted.Player()
				switch {
				case encrypted.Joined():
					rl.event(fmt.Sprintf("玩家 %s 已离开服务器（在线 %s，上行 %d 包，下行 %d 包）",
						playerLabel(player.Name), shortDuration(time.Since(startedAt)),
						clientStream.PacketCount(), serverStream.PacketCount()))
				case player.Name != "":
					rl.event(fmt.Sprintf("玩家 %s 未完成登录，连接已断开（上行 %d 包，下行 %d 包）",
						playerLabel(player.Name), clientStream.PacketCount(), serverStream.PacketCount()))
				default:
					rl.event(fmt.Sprintf("玩家连接已断开（未完成登录，上行 %d 包，下行 %d 包）",
						clientStream.PacketCount(), serverStream.PacketCount()))
				}
				if err != nil && sessionCtx.Err() == nil &&
					!errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
					rl.event(fmt.Sprintf("玩家连接异常：%v", err))
				}
			}(conn.(*nethernet.Conn))
		}
	}()

	upstreamOnline := true
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			cancel()
			return ctx.Err()
		case <-host.Done():
			cancel()
			return host.Err()
		case err := <-acceptErrors:
			cancel()
			return fmt.Errorf("WebRTC 信令断开: %w", err)
		case <-ticker.C:
			version, pingErr := pingUpstream(sessionCtx, cfg)
			online := pingErr == nil
			if online != upstreamOnline {
				if online {
					rl.event("上游服务器已恢复，房间继续在线")
				} else {
					rl.event("上游服务器不可达，等待恢复")
				}
				upstreamOnline = online
			}
			m.updateStatus(func(status *Status) {
				status.UpstreamOnline = online
				if online {
					status.UpstreamVersion = version
				}
			})
			m.updateRoom(index, func(status *RoomStatus) {
				if online {
					status.Phase = "online"
				} else {
					status.Phase = "upstream_offline"
				}
			})
		}
	}
}
