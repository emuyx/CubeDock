package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/core"
	"github.com/emuyx/cubedock/internal/webui"
)

// RuntimeInfo identifies the running binary in diagnostics and backups.
type RuntimeInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"go"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

func (r RuntimeInfo) withDefaults() RuntimeInfo {
	if r.Version == "" {
		r.Version = "dev"
	}
	if r.Commit == "" {
		r.Commit = "unknown"
	}
	if r.GoVersion == "" {
		r.GoVersion = runtime.Version()
	}
	if r.OS == "" {
		r.OS = runtime.GOOS
	}
	if r.Arch == "" {
		r.Arch = runtime.GOARCH
	}
	return r
}

type runtimeView struct {
	RuntimeInfo
	DataDir string `json:"data_dir"`
	WebPort int    `json:"web_port"`
}

func (h *consoleHandler) diagnostics(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	checks := h.manager.Diagnose(ctx)
	checks = append(checks, h.consoleChecks()...)
	summary := map[string]int{}
	for _, check := range checks {
		summary[check.Status]++
	}
	snapshot := h.manager.Snapshot()
	respond(w, map[string]any{
		"generated_at": time.Now().Format(time.RFC3339),
		"runtime": runtimeView{
			RuntimeInfo: h.runtime.withDefaults(),
			DataDir:     h.manager.Dir(),
			WebPort:     snapshot.Config.WebPort,
		},
		"summary": summary,
		"checks":  checks,
	})
}

// consoleChecks covers the parts of the deployment only the HTTP layer knows
// about. It never reads credentials.
func (h *consoleHandler) consoleChecks() []core.DiagnosticCheck {
	checks := []core.DiagnosticCheck{}
	if _, err := fs.Stat(webui.Root(), "index.html"); err != nil {
		checks = append(checks, core.DiagnosticCheck{
			ID: "webui", Category: "控制台", Label: "WebUI 资源",
			Status: core.DiagnosticFail, Detail: err.Error(), Hint: "重新安装或重新构建程序",
		})
	} else {
		checks = append(checks, core.DiagnosticCheck{
			ID: "webui", Category: "控制台", Label: "WebUI 资源",
			Status: core.DiagnosticOK, Detail: "控制台页面可用",
		})
	}
	if h.admin != nil {
		check := core.DiagnosticCheck{ID: "admin", Category: "控制台", Label: "管理员密码"}
		if info, err := os.Stat(h.admin.filePath); err != nil || info.Size() == 0 {
			check.Status = core.DiagnosticFail
			check.Detail = "管理员密码文件缺失或为空"
			check.Hint = "使用 -reset-admin-password 重置"
		} else {
			check.Status = core.DiagnosticOK
			check.Detail = "管理员密码文件可用"
		}
		checks = append(checks, check)
	}
	logDir := filepath.Join(h.manager.Dir(), "logs")
	check := core.DiagnosticCheck{ID: "logs", Category: "控制台", Label: "日志目录"}
	if info, err := os.Stat(logDir); err != nil || !info.IsDir() {
		check.Status = core.DiagnosticWarn
		check.Detail = "日志目录尚不可用"
		check.Hint = "确认数据目录可写"
	} else {
		check.Status = core.DiagnosticOK
		check.Detail = logDir
	}
	return append(checks, check)
}

type backupFile struct {
	Format     int                  `json:"format"`
	App        string               `json:"app"`
	Version    string               `json:"version"`
	ExportedAt string               `json:"exported_at"`
	Config     config.Config        `json:"config"`
	Accounts   []core.AccountStatus `json:"accounts"`
	Note       string               `json:"note"`
}

func (h *consoleHandler) backup(w http.ResponseWriter, r *http.Request) {
	snapshot := h.manager.Snapshot()
	cfg := snapshot.Config
	if r.URL.Query().Get("include_password") != "true" {
		cfg.RoomPassword = ""
	}
	payload := backupFile{
		Format: 1, App: "CubeDock", Version: h.runtime.withDefaults().Version,
		ExportedAt: time.Now().Format(time.RFC3339),
		Config:     cfg, Accounts: snapshot.Accounts,
		Note: "备份包含配置和账号信息，不包含网易登录凭证；恢复后需要重新登录或导入账号。",
	}
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="cubedock-backup-%s.json"`, time.Now().Format("20060102-150405")))
	respond(w, payload)
}

func (h *consoleHandler) restore(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Config config.Config `json:"config"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "备份 JSON 无效", http.StatusBadRequest)
		return
	}
	snapshot := h.manager.Snapshot()
	if snapshot.Running {
		http.Error(w, "请先停止房间，再恢复配置", http.StatusBadRequest)
		return
	}
	next := body.Config
	var warnings []string
	switch {
	case next.WebPort == 0:
		warnings = append(warnings, "备份未包含控制台端口，已保留当前端口")
		next.WebPort = snapshot.Config.WebPort
	case next.WebPort != snapshot.Config.WebPort:
		warnings = append(warnings, fmt.Sprintf(
			"控制台端口未恢复，当前仍为 %d；如需修改端口，请关闭程序后修改配置文件",
			snapshot.Config.WebPort))
		next.WebPort = snapshot.Config.WebPort
	}
	if err := h.manager.Configure(next); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.manager.Record("设置", "已从备份恢复配置")
	respond(w, map[string]any{"ok": true, "warnings": warnings})
}
