// Command cubedock runs the CubeDock room bridge and its web console.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/emuyx/cubedock/internal/auth"
	"github.com/emuyx/cubedock/internal/bridge"
	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/console"
	"github.com/emuyx/cubedock/internal/core"
	"github.com/emuyx/cubedock/internal/fileutil"
	"github.com/emuyx/cubedock/internal/secret"
	"github.com/emuyx/cubedock/internal/server"
	"github.com/emuyx/cubedock/internal/session"
)

// Set by the release workflow; local builds keep the dev defaults.
var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	showVersion := flag.Bool("version", false, "显示版本和源代码提交")
	dataDir := flag.String("data-dir", "", "配置和日志目录（默认程序所在目录）")
	headless := flag.Bool("headless", false, "不自动打开浏览器")
	webPort := flag.Int("web-port", 0, "临时覆盖控制台端口")
	runNow := flag.Bool("run", false, "启动后自动开房")
	debug := flag.Bool("debug", false, "开启详细诊断日志（也可在控制台即时切换）")
	check := flag.Bool("check", false, "检查本地服务器和账号鉴权，然后退出")
	runSeconds := flag.Int("run-seconds", 0, "运行指定秒数后关房退出（测试用）")
	flag.Parse()

	if *showVersion {
		fmt.Printf("CubeDock %s (%s/%s, commit %s)\n", version, runtime.GOOS, runtime.GOARCH, commit)
		return nil
	}
	if *dataDir == "" {
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		*dataDir = filepath.Dir(executable)
	}
	dir, err := filepath.Abs(*dataDir)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	// A crash during an atomic state write can leave a plaintext temp file.
	fileutil.CleanupStaleState(dir, 24*time.Hour)

	cfg, err := config.Load(filepath.Join(dir, "config.json"))
	if err != nil {
		return fmt.Errorf("配置读取失败: %w", err)
	}
	if *webPort != 0 {
		cfg.WebPort = *webPort
		if err = cfg.Validate(); err != nil {
			return err
		}
	}
	if *debug {
		cfg.Debug = true
	}

	tokens, err := loadTokens(dir)
	if err != nil {
		return err
	}
	logger, logFile, err := openLog(dir)
	if err != nil {
		return err
	}
	defer logFile.Close()

	if *check {
		return checkRooms(cfg, tokens)
	}

	packsDir, err := core.EnsureLimboPacksDir(cfg)
	if err != nil {
		logger.Printf("资源包目录创建失败: %v", err)
		packsDir = ""
	}
	manager, err := core.New(dir, cfg, tokens, logger)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", cfg.WebPort))
	if err != nil {
		return fmt.Errorf("控制台端口被占用: %w", err)
	}
	defer listener.Close()

	admin, err := server.LoadAdmin(dir)
	if err != nil {
		return err
	}
	// CUBEDOCK_ADMIN_PASSWORD is an initial-setup helper for headless runs.
	// It is only used when admin.json does not exist; it never resets an
	// existing password.
	adminPassword := os.Getenv("CUBEDOCK_ADMIN_PASSWORD")
	adminUsername := strings.TrimSpace(os.Getenv("CUBEDOCK_ADMIN_USERNAME"))
	_ = os.Unsetenv("CUBEDOCK_ADMIN_PASSWORD")
	_ = os.Unsetenv("CUBEDOCK_ADMIN_USERNAME")
	if adminPassword != "" {
		if admin.NeedsSetup() {
			if adminUsername == "" {
				adminUsername = "admin"
			}
			if err := admin.CompleteSetup(adminUsername, adminPassword); err != nil {
				return fmt.Errorf("初始化管理员账户失败: %w", err)
			}
		} else {
			fmt.Println("管理员账户已存在，已忽略 CUBEDOCK_ADMIN_PASSWORD（不会重置密码）。")
		}
	}
	httpServer := &http.Server{
		Handler:           server.NewHandler(manager, admin, server.RuntimeInfo{Version: version, Commit: commit}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      4 * time.Minute, // resource pack pulls can take a while
		IdleTimeout:       45 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() { serverErr <- httpServer.Serve(listener) }()

	address := fmt.Sprintf("http://127.0.0.1:%d", cfg.WebPort)
	console.Banner(os.Stdout, version, commit)
	console.URLs(os.Stdout, cfg.WebPort, console.IPv4Addresses())
	if packsDir != "" {
		fmt.Println("Limbo 资源包目录：", packsDir)
	}
	fmt.Println("管理员账号：", admin.Username())
	if admin.NeedsSetup() {
		fmt.Println("管理员账户尚未创建：请打开控制台，按页面提示设置账号和密码。")
	} else {
		fmt.Println("管理员密码：已设置。忘记密码需停止程序后备份并删除 admin.json，重启后在控制台重新初始化。")
	}
	if !*headless {
		openBrowser(address)
	}

	if *runNow || cfg.AutoStart {
		if err = manager.Start(); err != nil {
			manager.Record("系统", err.Error())
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var timed <-chan time.Time
	var timer *time.Timer
	if *runSeconds > 0 {
		timer = time.NewTimer(time.Duration(*runSeconds) * time.Second)
		timed = timer.C
		defer timer.Stop()
	}
	select {
	case <-ctx.Done():
	case <-timed:
	case <-manager.Exit():
	case err = <-serverErr:
		if err != http.ErrServerClosed {
			return err
		}
	}

	manager.Stop()
	manager.Wait()
	shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdown)
}

// loadTokens reads the protected token file. CUBEDOCK_TOKEN is only a
// bootstrap fallback when that file is empty; the bootstrap value is written to
// the protected file so account metadata and tokens stay in sync.
func loadTokens(dir string) ([]string, error) {
	path := filepath.Join(dir, "room-tokens.dat")
	stored, err := secret.Load(path)
	if err != nil {
		return nil, fmt.Errorf("无法读取房间会话，请检查当前用户权限或重新登录: %w", err)
	}
	tokens := core.ParseTokens(stored)
	if len(tokens) == 0 {
		tokens = core.ParseTokens(os.Getenv("CUBEDOCK_TOKEN"))
		if len(tokens) > 0 {
			if err := secret.Save(path, core.FormatTokens(tokens)); err != nil {
				return nil, fmt.Errorf("保存引导会话失败: %w", err)
			}
		}
	}
	// The token is persisted in the protected file; do not keep a second copy
	// in the environment for the lifetime of the process.
	_ = os.Unsetenv("CUBEDOCK_TOKEN")
	return tokens, nil
}

// checkRooms is a one-shot diagnostic for scripts and first-time setup.
func checkRooms(cfg config.Config, tokens []string) error {
	if len(tokens) == 0 {
		return fmt.Errorf("请先在控制台添加网易账号")
	}
	leases := make([]*session.Lease, 0, len(tokens))
	defer func() {
		for _, lease := range leases {
			lease.Release()
		}
	}()
	for _, token := range tokens {
		lease, err := session.Claim(token)
		if err != nil {
			return err
		}
		leases = append(leases, lease)
	}
	version, err := bridge.PingServer(context.Background(), cfg.Upstream)
	if err != nil {
		return fmt.Errorf("本地 Bedrock 服务器不可达")
	}
	fmt.Println("本地服务器在线，版本:", version)

	players := map[string]int{}
	for index, token := range tokens {
		ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
		info, err := auth.PrepareRoom(ctx, cfg, token)
		cancel()
		if err != nil {
			return fmt.Errorf("%s%w", roomPrefix(index, len(tokens)), err)
		}
		fmt.Printf("%s账号 %s；房间协议 %d；转发凭据校验通过\n",
			roomPrefix(index, len(tokens)), info.PlayerName, info.ProtocolID)
		if previous, duplicate := players[info.PlayerName]; duplicate {
			return fmt.Errorf("房间 %d 与房间 %d 使用了同一个网易账号，请为每个房间使用不同账号",
				previous+1, index+1)
		}
		players[info.PlayerName] = index
	}
	return nil
}

func roomPrefix(index, count int) string {
	if count <= 1 {
		return ""
	}
	return fmt.Sprintf("[房间 %d] ", index+1)
}

func openLog(dir string) (*log.Logger, *os.File, error) {
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil, nil, err
	}
	file, err := os.OpenFile(filepath.Join(logDir, "cubedock.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, nil, err
	}
	// Correct the mode of a log file created by an older build or a restore.
	_ = file.Chmod(0600)
	return log.New(file, "", log.LstdFlags), file, nil
}

func openBrowser(address string) {
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", address).Start()
	case "darwin":
		_ = exec.Command("open", address).Start()
	default:
		_ = exec.Command("xdg-open", address).Start()
	}
}
