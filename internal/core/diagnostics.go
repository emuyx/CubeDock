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

// HasProfile reports whether a profile ID exists.
func (m *Manager) HasProfile(profileID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.cfg.Profile(profileID)
	return ok
}

// Diagnose runs the console's read-only preflight checks. It never
// authenticates against NetEase and never returns session tokens. An empty
// profileID checks every profile; otherwise only that profile's checks are
// included, while account, lock and environment checks stay global.
func (m *Manager) Diagnose(ctx context.Context, profileID string) []DiagnosticCheck {
	m.mu.Lock()
	cfg := m.cfg.Clone()
	tokens := append([]string(nil), m.tokens...)
	running := len(m.runtimes) > 0
	accounts := m.accountStatusesLocked()
	activeProfiles := make(map[string]bool, len(m.runtimes))
	for _, runtime := range m.runtimes {
		activeProfiles[runtime.profile.ID] = true
	}
	m.mu.Unlock()

	profiles := cfg.Profiles
	if strings.TrimSpace(profileID) != "" {
		profile, ok := cfg.Profile(profileID)
		if !ok {
			return []DiagnosticCheck{diagnostic("config", "配置", "配置方案",
				DiagnosticFail, "配置方案不存在："+profileID, "在「配置方案」页重新选择")}
		}
		profiles = []config.Profile{profile}
	}

	return []DiagnosticCheck{
		diagnosticConfig(cfg, profiles, profileID),
		diagnosticDataDir(m.dir),
		m.diagnosticUpstream(ctx, profiles, activeProfiles),
		diagnosticLimbo(cfg, profiles),
		diagnosticPacks(m.dir, profiles),
		diagnosticAccounts(accounts),
		diagnosticMode(profiles),
		m.diagnosticLocks(running, tokens),
	}
}

func diagnosticConfig(cfg config.Config, profiles []config.Profile, profileID string) DiagnosticCheck {
	if err := cfg.Validate(); err != nil {
		return diagnostic("config", "配置", "配置有效", DiagnosticFail, err.Error(),
			"检查「配置方案」页")
	}
	if profileID != "" && len(profiles) == 1 {
		return diagnostic("config", "配置", "配置方案", DiagnosticOK,
			profileDetail(profiles[0]), "")
	}
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		names = append(names, profile.Name)
	}
	return diagnostic("config", "配置", "配置有效", DiagnosticOK,
		fmt.Sprintf("%d 个配置方案：%s", len(profiles), strings.Join(names, "、")), "")
}

func profileDetail(profile config.Profile) string {
	if profile.ConnectionMode == config.ConnectionTransfer {
		return fmt.Sprintf("「%s」转服模式，中转房间 %s → 服务器 %s",
			profile.Name, profile.LimboUpstream, profile.Upstream)
	}
	return fmt.Sprintf("「%s」代理模式，服务器 %s", profile.Name, profile.Upstream)
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

func (m *Manager) diagnosticUpstream(ctx context.Context, profiles []config.Profile, activeProfiles map[string]bool) DiagnosticCheck {
	originals := append([]config.Profile(nil), profiles...)
	var checked []config.Profile
	for _, profile := range profiles {
		if profile.ConnectionMode == config.ConnectionTransfer && !activeProfiles[profile.ID] {
			continue
		}
		checked = append(checked, profile)
	}
	label := "服务器可达"
	if len(originals) == 1 && originals[0].ConnectionMode == config.ConnectionTransfer {
		label = "中转房间可达"
	}
	if len(checked) == 0 {
		return diagnostic("upstream", "网络", label, DiagnosticSkip,
			"所有转服配置方案均未运行；中转房间只在开房时启动",
			"停止状态不检测中转房间连通性，开房后会自动启动")
	}
	if m.pingFn == nil {
		return diagnostic("upstream", "网络", label, DiagnosticSkip, "检查函数未初始化", "")
	}

	status := DiagnosticOK
	details := make([]string, 0, len(checked))
	for _, profile := range checked {
		pingCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		version, err := m.pingFn(pingCtx, profile)
		cancel()
		if err != nil {
			if status == DiagnosticOK {
				status = DiagnosticFail
			}
			details = append(details, fmt.Sprintf("「%s」%s：%s", profile.Name, upstreamAddress(profile), err.Error()))
			continue
		}
		detail := fmt.Sprintf("「%s」%s", profile.Name, upstreamAddress(profile))
		if strings.TrimSpace(version) != "" {
			detail += " · " + version
		}
		details = append(details, detail)
	}
	hint := ""
	if status == DiagnosticFail {
		hint = "确认地址正确且服务已启动"
	}
	if len(checked) > 1 {
		label = fmt.Sprintf("服务器可达（%d 个方案）", len(checked))
	}
	return diagnostic("upstream", "网络", label, status, strings.Join(details, "；"), hint)
}

func diagnosticLimbo(cfg config.Config, profiles []config.Profile) DiagnosticCheck {
	transfer := make([]config.Profile, 0, len(profiles))
	for _, profile := range profiles {
		if profile.ConnectionMode == config.ConnectionTransfer {
			transfer = append(transfer, profile)
		}
	}
	if len(transfer) == 0 {
		return diagnostic("limbo", "资源", "中转程序", DiagnosticSkip,
			"所选配置方案均为代理模式，不需要中转程序", "")
	}
	path, err := limboExecutable(cfg.LimboPath)
	if err != nil {
		return diagnostic("limbo", "资源", "中转程序", DiagnosticFail, err.Error(),
			"安装中转程序，或在设置中指定 limbo_path")
	}
	addresses := make([]string, 0, len(transfer))
	for _, profile := range transfer {
		addresses = append(addresses, fmt.Sprintf("「%s」%s", profile.Name, profile.LimboUpstream))
	}
	return diagnostic("limbo", "资源", "中转程序", DiagnosticOK,
		path+"；"+strings.Join(addresses, "、"), "")
}

func diagnosticPacks(dataDir string, profiles []config.Profile) DiagnosticCheck {
	transfer := make([]config.Profile, 0, len(profiles))
	for _, profile := range profiles {
		if profile.ConnectionMode == config.ConnectionTransfer {
			transfer = append(transfer, profile)
		}
	}
	if len(transfer) == 0 {
		return diagnostic("packs", "资源", "资源包目录", DiagnosticSkip,
			"所选配置方案均为代理模式，不使用本地资源包", "")
	}
	status := DiagnosticOK
	details := make([]string, 0, len(transfer))
	for _, profile := range transfer {
		dir := ProfilePacksDir(dataDir, profile)
		list, err := scanResourcePacks(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if status == DiagnosticOK {
					status = DiagnosticWarn
				}
				details = append(details, fmt.Sprintf("「%s」%s（目录尚未创建）", profile.Name, dir))
				continue
			}
			status = DiagnosticFail
			details = append(details, fmt.Sprintf("「%s」%s：%s", profile.Name, dir, err.Error()))
			continue
		}
		details = append(details, fmt.Sprintf("「%s」%s（%d 个包）", profile.Name, dir, len(list.Packs)))
	}
	hint := ""
	if status == DiagnosticFail {
		hint = "检查资源包目录权限"
	} else if status == DiagnosticWarn {
		hint = "目录尚未创建；开房时会自动创建，可在此放入 .mcpack"
	}
	return diagnostic("packs", "资源", "资源包目录", status, strings.Join(details, "；"), hint)
}

func diagnosticAccounts(accounts []AccountStatus) DiagnosticCheck {
	if len(accounts) == 0 {
		return diagnostic("accounts", "账号", "网易账号", DiagnosticWarn, "尚未添加账号",
			"在「账号与房间」登录或导入账号")
	}
	expired, profileErrors, missingProfiles := 0, 0, 0
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
		if account.ProfileName == "" {
			missingProfiles++
		}
	}
	detail := fmt.Sprintf("%d 个账号", len(accounts))
	if expired > 0 {
		detail += fmt.Sprintf("，%d 个登录已过期", expired)
	}
	if profileErrors > 0 {
		detail += fmt.Sprintf("，%d 个游戏名读取失败", profileErrors)
	}
	if missingProfiles > 0 {
		detail += fmt.Sprintf("，%d 个账号引用了不存在的配置方案", missingProfiles)
	}
	status := DiagnosticOK
	hint := ""
	if expired > 0 || profileErrors > 0 || missingProfiles > 0 {
		status = DiagnosticWarn
		hint = "重新登录过期账号，或为账号重新选择配置方案"
	}
	return diagnostic("accounts", "账号", "网易账号", status, detail, hint)
}

func diagnosticMode(profiles []config.Profile) DiagnosticCheck {
	var notes []string
	for _, profile := range profiles {
		if profile.FriendsOnly && profile.AutoAcceptFriends {
			notes = append(notes, fmt.Sprintf("「%s」仅好友可见与自动同意好友申请同时开启；为避免陌生人进入，自动同意会被跳过",
				profile.Name))
		}
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
			"有房间运行中，跳过账号占用检查", "")
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
