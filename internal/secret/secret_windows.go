//go:build windows

// Package secret stores credentials protected by the operating system.
package secret

import (
	"errors"
	"os"
	"syscall"
	"unsafe"

	"github.com/emuyx/cubedock/internal/fileutil"
)

type blob struct {
	size uint32
	data *byte
}

var (
	crypt32       = syscall.NewLazyDLL("crypt32.dll")
	kernel32      = syscall.NewLazyDLL("kernel32.dll")
	procProtect   = crypt32.NewProc("CryptProtectData")
	procUnprotect = crypt32.NewProc("CryptUnprotectData")
	procFree      = kernel32.NewProc("LocalFree")

	errEmptySecret = errors.New("凭据内容为空")
	errProtected   = errors.New("Windows 凭据保护失败")
)

// Description explains where the secret is kept.
func Description() string { return "Windows DPAPI（仅当前用户可解密）" }

func transform(input []byte, protect bool) ([]byte, error) {
	if len(input) == 0 {
		return nil, errEmptySecret
	}
	in := blob{size: uint32(len(input)), data: &input[0]}
	var out blob
	call := procUnprotect
	if protect {
		call = procProtect
	}
	ok, _, _ := call.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0, 1,
		uintptr(unsafe.Pointer(&out)),
	)
	if ok == 0 {
		return nil, errProtected
	}
	defer procFree.Call(uintptr(unsafe.Pointer(out.data)))
	return append([]byte(nil), unsafe.Slice(out.data, int(out.size))...), nil
}

// Save seals value with the current user's DPAPI key.
func Save(path, value string) error {
	sealed, err := transform([]byte(value), true)
	if err != nil {
		return err
	}
	return fileutil.Replace(path, sealed, 0600)
}

// Load opens a value written by Save. A missing file yields an empty string.
func Load(path string) (string, error) {
	sealed, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	plain, err := transform(sealed, false)
	return string(plain), err
}
