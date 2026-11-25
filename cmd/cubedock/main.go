// Command cubedock runs the CubeDock room bridge and its web console.
package main

import (
	"context"
	"errors"
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
	"sync"
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
	"github.com/emuyx/cubedock/internal/tui"
)

// Set by the release workflow; local builds keep the dev defaults.
var (
	version = "v0.4.3-dev"
	commit  = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	console.EnableUTF8()
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
		return checkRooms(dir, cfg, tokens)
	}

	for _, profile := range cfg.Profiles {
		if profile.ConnectionMode != config.ConnectionTransfer {
			continue
		}
		packsDir, packsErr := core.EnsureLimboPacksDir(dir, profile)
		if packsErr != nil {
			logger.Printf("配置方案「%s」的资源包目录创建失败: %v", profile.Name, packsErr)
			continue
		}
		fmt.Printf("Limbo 资源包目录（%s）：%s\n", profile.Name, packsDir)
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
	localURL := address + "/"
	addresses := console.IPv4Addresses()
	lanURLs := make([]string, 0, len(addresses))
	for _, ip := range addresses {
		lanURLs = append(lanURLs, fmt.Sprintf("http://%s:%d/", ip.String(), cfg.WebPort))
	}

	// The TUI is the default view on an interactive terminal. It coexists with
	// the web console: both drive the same Manager. Redirected output, CI and
	// dumb terminals keep the plain banner and log stream.
	useTUI := tui.Enabled()
	printPlainStatus := func() {
		console.Banner(os.Stdout, version, commit)
		console.URLs(os.Stdout, cfg.WebPort, addresses)
		fmt.Println("管理员账号：", admin.Username())
		if admin.NeedsSetup() {
			fmt.Println("管理员账户尚未创建：请打开控制台，按页面提示设置账号和密码。")
		} else {
			fmt.Println("管理员密码：已设置。忘记密码需停止程序后备份并删除 admin.json，重启后在控制台重新初始化。")
		}
	}
	if !useTUI {
		printPlainStatus()
	}
	if !*headless {
		openBrowser(localURL)
	}

	if *runNow || cfg.AutoStart {
		go func() {
			if err := manager.Start(); err != nil {
				manager.Record("系统", err.Error())
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	if *runSeconds > 0 {
		timer := time.AfterFunc(time.Duration(*runSeconds)*time.Second, cancelRun)
		defer timer.Stop()
	}
	go func() {
		select {
		case <-manager.Exit():
			cancelRun()
		case <-runCtx.Done():
		}
	}()

	var serverFailureMu sync.Mutex
	var serverFailure error
	go func() {
		err := <-serverErr
		if err == nil {
			cancelRun()
			return
		}
		if !errors.Is(err, http.ErrServerClosed) {
			serverFailureMu.Lock()
			serverFailure = err
			serverFailureMu.Unlock()
			cancelRun()
		}
	}()

	if useTUI {
		err := tui.Run(runCtx, tui.Options{
			Manager:       manager,
			Version:       version,
			Commit:        commit,
			AdminName:     admin.Username(),
			SetupRequired: admin.NeedsSetup(),
			LocalURL:      localURL,
			LANURLs:       lanURLs,
			OpenURL:       openBrowser,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "终端控制台不可用：", err)
			if runCtx.Err() == nil {
				printPlainStatus()
				<-runCtx.Done()
			}
		}
	} else {
		<-runCtx.Done()
	}

	stopManager(manager, 10*time.Second)
	shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	shutdownErr := httpServer.Shutdown(shutdown)
	serverFailureMu.Lock()
	failure := serverFailure
	serverFailureMu.Unlock()
	if failure != nil {
		return failure
	}
	return shutdownErr
}

// stopManager asks every room to stop and waits for the runtime map to drain.
// It keeps calling Stop while waiting because an auto-start/auth flow that was
// already in progress can register a runtime just after the first Stop.
func stopManager(manager *core.Manager, timeout time.Duration) {
	manager.Stop()
	done := make(chan struct{})
	go func() {
		manager.Wait()
		close(done)
	}()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			manager.Stop()
		case <-timer.C:
			return
		}
	}
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
func checkRooms(dataDir string, cfg config.Config, tokens []string) error {
	if len(tokens) == 0 {
		return fmt.Errorf("请先在控制台添加网易账号")
	}
	profiles, err := core.AccountProfiles(dataDir, cfg, len(tokens))
	if err != nil {
		return err
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
	pinged := make(map[string]bool)
	for _, profile := range profiles {
		if profile.ConnectionMode == config.ConnectionTransfer || pinged[profile.Upstream] {
			continue
		}
		pinged[profile.Upstream] = true
		version, pingErr := bridge.PingServer(context.Background(), profile.Upstream)
		if pingErr != nil {
			return fmt.Errorf("配置方案「%s」的服务器 %s 不可达", profile.Name, profile.Upstream)
		}
		fmt.Printf("配置方案「%s」服务器在线，版本：%s\n", profile.Name, version)
	}

	players := map[string]int{}
	for index, token := range tokens {
		ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
		info, err := auth.PrepareRoom(ctx, profiles[index].LobbyProtocolID, token)
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
