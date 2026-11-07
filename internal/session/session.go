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
	file *os.File
	once sync.Once
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
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("请先设置房间令牌")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, errors.New("无法创建会话锁目录")
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	path := filepath.Join(dir, fmt.Sprintf("%x.lock", sum))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, errors.New("无法打开会话锁")
	}
	// The lock file is intentionally never deleted: removing it would allow a
	// second process to lock a different inode at the same path on Unix.
	if err := lockFile(file); err != nil {
		file.Close()
		return nil, err
	}
	return &Lease{file: file}, nil
}

// Release gives up the lease. It is safe to call more than once.
func (l *Lease) Release() {
	if l == nil {
		return
	}
	l.once.Do(func() { l.file.Close() })
}
