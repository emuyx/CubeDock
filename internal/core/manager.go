// Package core owns CubeDock's room lifecycle, account list and event log. It
// has no HTTP dependencies so the console is only one possible front end.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/emuyx/cubedock/internal/auth"
	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/fileutil"
	"github.com/emuyx/cubedock/internal/redact"
	"github.com/emuyx/cubedock/internal/room"
	"github.com/emuyx/cubedock/internal/session"
)

// maxEvents bounds the in-memory console history returned by /api/status.
const maxEvents = 300

const (
	tokenFile   = "room-tokens.dat"
	accountFile = "accounts.json"
	statusFile  = "logs/status.json"
)

// Event is one console/log line. Room is 1-based and zero for system-wide
// events; Profile names the room's configuration profile when known.
type Event struct {
	Time    string `json:"time"`
	Source  string `json:"source"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Room    int    `json:"room,omitempty"`
	Profile string `json:"profile,omitempty"`
}

// RoomStatus describes one account's room. The slice is always aligned with
// the account list; inactive accounts carry Phase "stopped".
type RoomStatus struct {
	Phase           string `json:"phase"`
	RoomID          uint32 `json:"room_id"`
	Players         int    `json:"players"`
	Account         string `json:"account"`
	ProfileID       string `json:"profile_id,omitempty"`
	ProfileName     string `json:"profile_name,omitempty"`
	ConnectionMode  string `json:"connection_mode,omitempty"`
	ProtocolID      uint8  `json:"protocol_id"`
	UpstreamOnline  bool   `json:"upstream_online"`
	UpstreamVersion string `json:"upstream_version,omitempty"`
	ConnectedAt     string `json:"connected_at"`
	RetryAt         string `json:"retry_at"`
	Attempts        int    `json:"attempts"`
	LastError       string `json:"last_error"`
}

// Status is the console snapshot. The top-level room fields mirror the first
// active room for older clients; Players is the total across active rooms.
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

// Manager coordinates every room owned by this process.
type Manager struct {
	dir    string
	logger *log.Logger

	mu             sync.Mutex
	cfg            config.Config
	tokens         []string
	accounts       []accountRecord
	status         Status
	runtimes       map[int]*roomRuntime
	hosts          map[int]roomHost
	friendSessions map[int]friendCredentials
	limbos         map[string]*limboGroup
	idle           chan struct{}
	preparedPacks  map[string]bool

	players playerRegistry

	// lifecycleMu serializes lease claims and runtime registration only. It is
	// never held across pull/limbo/network waits.
	lifecycleMu sync.Mutex
	limboMu     sync.Mutex
	actionsMu   sync.Mutex
	actions     map[int]*sync.Mutex

	// prepMu serializes per-profile pack preparation.
	prepMu    sync.Mutex
	prepLocks map[string]*sync.Mutex

	// Room operations are indirections so tests can exercise many rooms at
	// once without NetEase. New sets the production implementations.
	prepareRoom  func(context.Context, uint8, string) (auth.RoomInfo, error)
	openRoom     func(context.Context, config.Profile, auth.RoomInfo, func(string)) (roomHost, error)
	pingFn       func(context.Context, config.Profile) (string, error)
	claimSession func(string) (*session.Lease, error)
	startLimboFn func(string, config.Profile, string, *log.Logger, func(string)) (*limboProcess, error)
	pullFn       func(context.Context, config.Profile, string, string) (string, error)

	exit     chan struct{}
	exitOnce sync.Once
}

// New loads account metadata and returns a stopped manager.
func New(dir string, cfg config.Config, tokens []string, logger *log.Logger) (*Manager, error) {
	accounts, err := loadAccounts(filepath.Join(dir, accountFile), len(tokens), cfg.DefaultProfileID())
	if err != nil {
		return nil, fmt.Errorf("读取账号列表失败: %w", err)
	}
	m := &Manager{
		dir:            dir,
		logger:         logger,
		cfg:            cfg.Clone(),
		tokens:         append([]string(nil), tokens...),
		accounts:       accounts,
		status:         Status{Phase: "stopped", Events: []Event{}},
		runtimes:       make(map[int]*roomRuntime),
		hosts:          make(map[int]roomHost),
		friendSessions: make(map[int]friendCredentials),
		limbos:         make(map[string]*limboGroup),
		preparedPacks:  make(map[string]bool),
		actions:        make(map[int]*sync.Mutex),
		prepLocks:      make(map[string]*sync.Mutex),
		exit:           make(chan struct{}),

		prepareRoom: auth.PrepareRoom,
		openRoom: func(ctx context.Context, profile config.Profile, info auth.RoomInfo, trace func(string)) (roomHost, error) {
			return room.Open(ctx, profile, info, trace)
		},
		pingFn:       pingUpstream,
		claimSession: session.Claim,
		startLimboFn: startLimboProcess,
	}
	m.pullFn = m.pullResourcePacks
	m.idle = make(chan struct{})
	close(m.idle)
	m.mu.Lock()
	m.syncRoomsLocked()
	m.mu.Unlock()
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
	"starting": 3, "stopping": 4, "retrying": 5, "upstream_offline": 6,
	"stopped": 7,
}

// Snapshot returns a copy of the current status.
func (m *Manager) Snapshot() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncRoomsLocked()
	s := m.status
	s.Config = m.cfg.Clone()
	s.TokenSet = len(m.tokens) > 0
	s.TokenCount = len(m.tokens)
	s.Events = append([]Event{}, s.Events...)
	s.Rooms = append([]RoomStatus{}, s.Rooms...)
	s.Accounts = m.accountStatusesLocked()
	s.Running = len(m.runtimes) > 0
	s.Players = 0
	s.LastError = ""
	s.UpstreamOnline = false
	s.UpstreamVersion = ""
	first := -1
	best := len(phaseRank)
	for index := range s.Rooms {
		status := &s.Rooms[index]
		if status.Phase == "" || status.Phase == "stopped" {
			continue
		}
		if first < 0 {
			first = index
		}
		s.Players += status.Players
		if status.UpstreamOnline {
			s.UpstreamOnline = true
		}
		if s.UpstreamVersion == "" && status.UpstreamVersion != "" {
			s.UpstreamVersion = status.UpstreamVersion
		}
		if s.LastError == "" && status.LastError != "" {
			s.LastError = roomLabel(index, len(s.Rooms)) + status.LastError
		}
		if rank, ok := phaseRank[status.Phase]; ok && rank < best {
			best = rank
		}
	}
	if first < 0 {
		s.Phase = "stopped"
		s.RoomID, s.Account, s.ProtocolID = 0, "", 0
		s.ConnectedAt, s.RetryAt, s.Attempts = "", "", 0
		return s
	}
	primary := s.Rooms[first]
	s.RoomID, s.Account, s.ProtocolID = primary.RoomID, primary.Account, primary.ProtocolID
	s.ConnectedAt, s.RetryAt, s.Attempts = primary.ConnectedAt, primary.RetryAt, primary.Attempts
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

// anyRunningLocked reports whether any room runtime exists, including one in
// preflight or shutdown. Callers must hold m.mu.
func (m *Manager) anyRunningLocked() bool {
	return len(m.runtimes) > 0
}

// syncRoomsLocked keeps the room status slice aligned with the account list.
func (m *Manager) syncRoomsLocked() {
	count := len(m.accounts)
	if len(m.status.Rooms) < count {
		for len(m.status.Rooms) < count {
			m.status.Rooms = append(m.status.Rooms, RoomStatus{Phase: "stopped"})
		}
	} else if len(m.status.Rooms) > count {
		m.status.Rooms = m.status.Rooms[:count]
	}
	for index := range m.status.Rooms {
		status := &m.status.Rooms[index]
		if status.Phase == "" {
			status.Phase = "stopped"
		}
		record := m.accounts[index]
		id := record.ProfileID
		if id == "" {
			id = m.cfg.DefaultProfileID()
		}
		status.ProfileID = id
		if profile, ok := m.cfg.Profile(id); ok {
			status.ProfileName = profile.Name
			status.ConnectionMode = profile.ConnectionMode
		} else {
			status.ProfileName = ""
			status.ConnectionMode = ""
		}
		if status.Phase == "stopped" {
			status.Account = firstNonEmpty(record.Name, record.Label, record.Email)
		}
	}
}

func (m *Manager) emit(source, message string)  { m.record(source, message, false, 0, "") }
func (m *Manager) trace(source, message string) { m.record(source, message, true, 0, "") }

// Record writes a console event from outside the manager (for example an
// account-login adapter) using the same redaction and log file.
func (m *Manager) Record(source, message string) { m.emit(source, message) }

func (m *Manager) record(source, message string, debug bool, room int, profile string) {
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
	event := Event{
		Time: time.Now().Format("15:04:05"), Source: source, Level: level,
		Message: message, Room: room, Profile: profile,
	}
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
	m.syncRoomsLocked()
	if index >= 0 && index < len(m.status.Rooms) {
		change(&m.status.Rooms[index])
	}
	m.mu.Unlock()
	m.persist()
}

// persist writes a status snapshot without secrets. Failures are non-fatal.
func (m *Manager) persist() {
	status := m.Snapshot()
	status.Config = status.Config.WithoutRoomPasswords()
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
	if m.anyRunningLocked() {
		return fmt.Errorf("请先停止所有房间，再修改配置")
	}
	if next.WebPort != m.cfg.WebPort {
		return fmt.Errorf("控制台端口需关闭程序后在 config.json 修改")
	}
	if err := m.profilesAssignedLocked(next); err != nil {
		return err
	}
	if err := config.Save(filepath.Join(m.dir, "config.json"), next); err != nil {
		return err
	}
	m.cfg = next.Clone()
	m.preparedPacks = make(map[string]bool)
	m.syncRoomsLocked()
	return nil
}

// ConfigureForRestore applies a restored config and reassigns accounts whose
// profile no longer exists to the restored default profile.
func (m *Manager) ConfigureForRestore(next config.Config) ([]string, error) {
	var warnings []string
	next.Normalize()
	if err := next.Validate(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.anyRunningLocked() {
		return nil, fmt.Errorf("请先停止所有房间，再恢复配置")
	}
	if next.WebPort != m.cfg.WebPort {
		return nil, fmt.Errorf("控制台端口需关闭程序后在 config.json 修改")
	}
	fallback := next.DefaultProfileID()
	nextAccounts := append([]accountRecord(nil), m.accounts...)
	for index := range nextAccounts {
		id := nextAccounts[index].ProfileID
		if id == "" {
			nextAccounts[index].ProfileID = fallback
			continue
		}
		if _, ok := next.Profile(id); !ok {
			nextAccounts[index].ProfileID = fallback
			warnings = append(warnings, fmt.Sprintf("房间 %d 的配置方案 %q 已不存在，已改用「%s」",
				index+1, id, next.Profiles[0].Name))
		}
	}
	if err := saveAccounts(filepath.Join(m.dir, accountFile), nextAccounts); err != nil {
		return nil, err
	}
	if err := config.Save(filepath.Join(m.dir, "config.json"), next); err != nil {
		_ = saveAccounts(filepath.Join(m.dir, accountFile), m.accounts)
		return nil, err
	}
	m.cfg = next.Clone()
	m.accounts = nextAccounts
	m.preparedPacks = make(map[string]bool)
	m.syncRoomsLocked()
	return warnings, nil
}

// profilesAssignedLocked rejects configs that would leave an account pointing
// at a deleted profile. A blank profile ID falls back to the new default.
func (m *Manager) profilesAssignedLocked(next config.Config) error {
	fallback := next.DefaultProfileID()
	for index, record := range m.accounts {
		id := record.ProfileID
		if id == "" {
			id = fallback
		}
		if _, ok := next.Profile(id); ok {
			continue
		}
		return fmt.Errorf("配置方案 %q 仍被房间 %d 使用，请先为它选择其他方案", id, index+1)
	}
	return nil
}

// SetDebug toggles detailed logging and persists the choice.
func (m *Manager) SetDebug(enabled bool) error {
	m.mu.Lock()
	next := m.cfg.Clone()
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
