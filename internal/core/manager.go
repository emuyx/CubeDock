// Package core owns CubeDock's room lifecycle, account list and event log. It
// has no HTTP dependencies so the console is only one possible front end.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/fileutil"
	"github.com/emuyx/cubedock/internal/redact"
	"log"
	"path/filepath"
	"sync"
	"time"
)

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
