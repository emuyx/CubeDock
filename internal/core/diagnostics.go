package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/redact"
	"github.com/emuyx/cubedock/internal/session"
)

// Diagnostic status values exposed to the console.
const (
	DiagnosticOK   = "ok"
	DiagnosticWarn = "warn"
	DiagnosticFail = "fail"
	DiagnosticSkip = "skip"
)

// DiagnosticCheck is one read-only preflight result.
type DiagnosticCheck struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Label    string `json:"label"`
	Status   string `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Hint     string `json:"hint,omitempty"`
}

func diagnostic(id, category, label, status, detail, hint string) DiagnosticCheck {
	return DiagnosticCheck{
		ID: id, Category: category, Label: label, Status: status,
		Detail: redact.Sanitize(detail, ""), Hint: hint,
	}
}

// Diagnose runs the console's read-only preflight checks. It never
// authenticates against NetEase and never returns session tokens.
func (m *Manager) Diagnose(ctx context.Context) []DiagnosticCheck {
	m.mu.Lock()
	cfg := m.cfg
	tokens := append([]string(nil), m.tokens...)
	running := m.cancel != nil
	accounts := m.accountStatusesLocked()
	m.mu.Unlock()

	return []DiagnosticCheck{
		diagnosticConfig(cfg),
		diagnosticDataDir(m.dir),
		m.diagnosticUpstream(ctx, cfg, running),
		diagnosticLimbo(cfg),
		diagnosticPacks(cfg),
		diagnosticAccounts(accounts),
		diagnosticMode(cfg),
		m.diagnosticLocks(running, tokens),
	}
}

func diagnosticConfig(cfg config.Config) DiagnosticCheck {
	if err := cfg.Validate(); err != nil {
		return diagnostic("config", "配置", "配置有效", DiagnosticFail, err.Error(),
			"检查设置页")
	}
	detail := fmt.Sprintf("代理模式，服务器 %s", cfg.Upstream)
	if cfg.ConnectionMode == config.ConnectionTransfer {
		detail = fmt.Sprintf("转服模式，中转房间 %s → 服务器 %s", cfg.LimboUpstream, cfg.Upstream)
	}
	return diagnostic("config", "配置", "配置有效", DiagnosticOK, detail, "")
}

func diagnosticDataDir(dir string) DiagnosticCheck {
	info, err := os.Stat(dir)
	if err != nil {
		return diagnostic("data_dir", "环境", "数据目录可读写", DiagnosticFail, err.Error(),
			"确认数据目录存在且可写")
	}
	if !info.IsDir() {
		return diagnostic("data_dir", "环境", "数据目录可读写", DiagnosticFail, dir+" 不是目录", "")
	}
	handle, err := os.CreateTemp(dir, ".diag-*")
	if err != nil {
		return diagnostic("data_dir", "环境", "数据目录可读写", DiagnosticFail, err.Error(),
			"检查目录权限或磁盘空间")
	}
	name := handle.Name()
	_ = handle.Close()
	_ = os.Remove(name)
	return diagnostic("data_dir", "环境", "数据目录可读写", DiagnosticOK, dir, "")
}

func (m *Manager) diagnosticUpstream(ctx context.Context, cfg config.Config, running bool) DiagnosticCheck {
	label := "服务器可达"
	if cfg.ConnectionMode == config.ConnectionTransfer {
		if !running {
			// limbo only listens while a room is running; pinging it from a
			// stopped self-check would always fail and look like an error.
			return diagnostic("upstream", "网络", "中转房间", DiagnosticSkip,
				"房间未运行；中转房间只在开房时启动",
				"停止状态不检测中转房间连通性，开房后会自动启动")
		}
		label = "中转房间可达"
	}
	if m.pingFn == nil {
		return diagnostic("upstream", "网络", label, DiagnosticSkip, "检查函数未初始化", "")
	}
	pingCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	version, err := m.pingFn(pingCtx, cfg)
	if err != nil {
		return diagnostic("upstream", "网络", label, DiagnosticFail, err.Error(),
			"确认地址正确且服务已启动")
	}
	detail := upstreamAddress(cfg)
	if strings.TrimSpace(version) != "" {
		detail += " · " + version
	}
	return diagnostic("upstream", "网络", label, DiagnosticOK, detail, "")
}

func diagnosticLimbo(cfg config.Config) DiagnosticCheck {
	if cfg.ConnectionMode != config.ConnectionTransfer {
		return diagnostic("limbo", "资源", "中转程序", DiagnosticSkip,
			"代理模式不需要中转程序", "")
	}
	path, err := limboExecutable(cfg)
	if err != nil {
		return diagnostic("limbo", "资源", "中转程序", DiagnosticFail, err.Error(),
			"安装中转程序，或在连接设置中指定 limbo_path")
	}
	return diagnostic("limbo", "资源", "中转程序", DiagnosticOK, path, "")
}

func diagnosticPacks(cfg config.Config) DiagnosticCheck {
	dir := DefaultLimboPacksDir(cfg)
	list, err := scanResourcePacks(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return diagnostic("packs", "资源", "资源包目录", DiagnosticWarn, dir,
				"目录尚未创建；开房时会自动创建，可在此放入 .mcpack")
		}
		return diagnostic("packs", "资源", "资源包目录", DiagnosticFail, err.Error(),
			"检查资源包目录权限")
	}
	return diagnostic("packs", "资源", "资源包目录", DiagnosticOK,
		fmt.Sprintf("%s（%d 个包）", dir, len(list.Packs)), "")
}

func diagnosticAccounts(accounts []AccountStatus) DiagnosticCheck {
	if len(accounts) == 0 {
		return diagnostic("accounts", "账号", "网易账号", DiagnosticWarn, "尚未添加账号",
			"在「账号与房间」登录或导入账号")
	}
	expired, profileErrors := 0, 0
	for _, account := range accounts {
		if account.ExpiresAt != "" {
			if expiry, err := time.Parse(time.RFC3339, account.ExpiresAt); err == nil &&
				time.Now().After(expiry) {
				expired++
			}
		}
		if account.ProfileError != "" {
			profileErrors++
		}
	}
	detail := fmt.Sprintf("%d 个账号", len(accounts))
	if expired > 0 {
		detail += fmt.Sprintf("，%d 个登录已过期", expired)
	}
	if profileErrors > 0 {
		detail += fmt.Sprintf("，%d 个游戏名读取失败", profileErrors)
	}
	status := DiagnosticOK
	hint := ""
	if expired > 0 || profileErrors > 0 {
		status = DiagnosticWarn
		hint = "重新登录过期账号，或点验证按钮刷新信息"
	}
	return diagnostic("accounts", "账号", "网易账号", status, detail, hint)
}

func diagnosticMode(cfg config.Config) DiagnosticCheck {
	var notes []string
	if cfg.FriendsOnly && cfg.AutoAcceptFriends {
		notes = append(notes,
			"仅好友可见与自动同意好友申请同时开启；为避免陌生人进入，自动同意会被跳过")
	}
	if len(notes) == 0 {
		return diagnostic("mode", "配置", "房间策略", DiagnosticOK, "未发现冲突", "")
	}
	return diagnostic("mode", "配置", "房间策略", DiagnosticWarn,
		strings.Join(notes, "；"), "如需要陌生人加入，请关闭「仅好友可见」")
}

func (m *Manager) diagnosticLocks(running bool, tokens []string) DiagnosticCheck {
	if running {
		return diagnostic("locks", "账号", "账号占用", DiagnosticSkip,
			"房间运行中，跳过账号占用检查", "")
	}
	if len(tokens) == 0 {
		return diagnostic("locks", "账号", "账号占用", DiagnosticSkip, "没有账号需要检查", "")
	}
	if m.claimSession == nil {
		return diagnostic("locks", "账号", "账号占用", DiagnosticSkip, "检查函数未初始化", "")
	}
	var busy, failures []string
	for index, token := range tokens {
		lease, err := m.claimSession(token)
		if err != nil {
			if errors.Is(err, session.ErrTokenBusy) {
				busy = append(busy, fmt.Sprintf("房间 %d", index+1))
			} else {
				failures = append(failures, fmt.Sprintf("房间 %d：%s", index+1, err.Error()))
			}
			continue
		}
		if lease != nil {
			lease.Release()
		}
	}
	if len(failures) > 0 {
		return diagnostic("locks", "账号", "账号占用", DiagnosticFail,
			strings.Join(failures, "；"), "检查本机缓存目录权限")
	}
	if len(busy) > 0 {
		return diagnostic("locks", "账号", "账号占用", DiagnosticWarn,
			strings.Join(busy, "、")+" 的账号正被另一个 CubeDock 进程使用",
			"关闭另一个实例，或为这些房间更换账号")
	}
	return diagnostic("locks", "账号", "账号占用", DiagnosticOK,
		fmt.Sprintf("%d 个账号的占用检查通过", len(tokens)), "")
}
