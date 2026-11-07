// Package session guards a NetEase account against being logged in twice by
// this machine. A process-scoped file lock is held for the whole lifetime of a
// room, including its reconnect loop. The token itself never touches the disk.
package session

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ErrTokenBusy means another CubeDock process is already using the account.
var ErrTokenBusy = errors.New("该网易账号已有本机房间在运行，请先停止对应房间；同一账号重复登录可能使现有房间掉线")

// Lease is an exclusive claim on one account token.
type Lease struct {
	handle  *os.File
	release sync.Once
}

// Claim takes the machine-wide lease for token.
func Claim(token string) (*Lease, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, errors.New("无法确定会话锁目录")
	}
	return ClaimIn(filepath.Join(cache, "CubeDock", "sessions"), token)
}

// ClaimIn takes a lease in dir. Tests and portable deployments may choose the
// directory, but the lock is always keyed by the token hash.
func ClaimIn(dir, token string) (*Lease, error) {
	normalized := strings.TrimSpace(token)
	if normalized == "" {
		return nil, errors.New("请先设置房间令牌")
	}
	return acquire(filepath.Join(dir, leaseName(normalized)))
}

func leaseName(token string) string {
	digest := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x.lock", digest)
}

func acquire(path string) (*Lease, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, errors.New("无法创建会话锁目录")
	}
	handle, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, errors.New("无法打开会话锁")
	}
	// The lock file is intentionally never deleted: removing it would allow a
	// second process to lock a different inode at the same path on Unix.
	if err := lockFile(handle); err != nil {
		_ = handle.Close()
		return nil, err
	}
	return &Lease{handle: handle}, nil
}

// Release gives up the lease. It is safe to call more than once.
func (l *Lease) Release() {
	if l == nil {
		return
	}
	l.release.Do(func() { _ = l.handle.Close() })
}

func lockFile(file *os.File) error {
	err := lockPlatform(file.Fd())
	if err == nil || err == ErrTokenBusy {
		return err
	}
	return fmt.Errorf("无法锁定会话: %w", err)
}
