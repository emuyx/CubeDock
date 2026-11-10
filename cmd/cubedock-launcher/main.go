// Command cubedock-launcher self-extracts CubeDockMain and limbo, then starts
// the main program. It is the single entry point shipped to users.
package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed payload/*
var payload embed.FS

func main() {
	installDir, args, err := parseArgs(os.Args[1:])
	if err != nil {
		fatal(err)
	}
	if installDir == "" {
		installDir = defaultInstallDir()
	}
	dir, err := ensureInstallDir(installDir)
	if err != nil {
		fatal(err)
	}
	if !hasDataDir(args) {
		dataDir := dir
		if filepath.Base(dir) == "bin" {
			dataDir = filepath.Dir(dir)
		}
		if dataDir != "" && dataDir != "." {
			args = append(args, "-data-dir", dataDir)
		}
	}
	mainName := "CubeDockMain"
	limboName := "limbo"
	if runtime.GOOS == "windows" {
		mainName += ".exe"
		limboName += ".exe"
	}
	if err := extract("payload/"+mainName, filepath.Join(dir, mainName)); err != nil {
		fatal(fmt.Errorf("释放主程序失败: %w", err))
	}
	if err := extract("payload/"+limboName, filepath.Join(dir, limboName)); err != nil {
		fatal(fmt.Errorf("释放 limbo 失败: %w", err))
	}

	cmd := exec.Command(filepath.Join(dir, mainName), args...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	configureLauncherChild(cmd)
	if err := cmd.Start(); err != nil {
		fatal(fmt.Errorf("启动主程序失败: %w", err))
	}
	cleanup, err := attachLauncherJob(cmd)
	if err != nil {
		_ = cmd.Process.Kill()
		fatal(fmt.Errorf("绑定主程序生命周期失败: %w", err))
	}
	if cleanup != nil {
		defer cleanup()
	}
	waitErr := cmd.Wait()
	if waitErr == nil {
		return
	}
	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		os.Exit(exitErr.ExitCode())
	}
	fatal(waitErr)
}

func hasDataDir(args []string) bool {
	for _, arg := range args {
		if arg == "-data-dir" || arg == "--data-dir" ||
			strings.HasPrefix(arg, "-data-dir=") || strings.HasPrefix(arg, "--data-dir=") {
			return true
		}
	}
	return false
}

func parseArgs(args []string) (string, []string, error) {
	var installDir string
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-install-dir" || arg == "--install-dir":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("-install-dir 需要参数")
			}
			i++
			installDir = args[i]
		case strings.HasPrefix(arg, "-install-dir="):
			installDir = strings.TrimPrefix(arg, "-install-dir=")
		case strings.HasPrefix(arg, "--install-dir="):
			installDir = strings.TrimPrefix(arg, "--install-dir=")
		default:
			out = append(out, arg)
		}
	}
	return installDir, out, nil
}

func defaultInstallDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "bin")
}

func ensureInstallDir(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("无法确定安装目录")
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0755); err == nil {
		test := filepath.Join(dir, ".cubedock-write-test")
		if os.WriteFile(test, []byte("ok"), 0600) == nil {
			_ = os.Remove(test)
			return dir, nil
		}
	}
	fallback, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("安装目录不可写，且无法确定回退目录: %w", err)
	}
	fallback = filepath.Join(fallback, "CubeDock", "bin")
	if err := os.MkdirAll(fallback, 0755); err != nil {
		return "", err
	}
	return fallback, nil
}

func extract(embeddedPath, target string) error {
	data, err := payload.ReadFile(embeddedPath)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	if existing, err := os.ReadFile(target); err == nil && bytes.Equal(hash[:], sha256Sum(existing)) {
		return nil
	}
	tempFile, err := os.CreateTemp(filepath.Dir(target), ".cubedock-*.tmp")
	if err != nil {
		return err
	}
	temp := tempFile.Name()
	if _, err = tempFile.Write(data); err == nil {
		err = tempFile.Chmod(0755)
	}
	if closeErr := tempFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(temp)
		return err
	}
	if err := os.Rename(temp, target); err != nil {
		// Windows cannot rename over a running executable. Remove the old
		// target first; if it is in use, return a clear error.
		if removeErr := os.Remove(target); removeErr != nil && !os.IsNotExist(removeErr) {
			_ = os.Remove(temp)
			return fmt.Errorf("目标文件正在使用或不可覆盖: %w", removeErr)
		}
		if err := os.Rename(temp, target); err != nil {
			_ = os.Remove(temp)
			return err
		}
	}
	_ = os.Chmod(target, 0755)
	return nil
}

func sha256Sum(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "CubeDock launcher:", err)
	os.Exit(1)
}
