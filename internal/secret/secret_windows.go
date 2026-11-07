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

type dataBlob struct {
	length  uint32
	pointer *byte
}

var (
	crypt32DLL    = syscall.NewLazyDLL("crypt32.dll")
	kernelDLL     = syscall.NewLazyDLL("kernel32.dll")
	protectData   = crypt32DLL.NewProc("CryptProtectData")
	unprotectData = crypt32DLL.NewProc("CryptUnprotectData")
	freeMemory    = kernelDLL.NewProc("LocalFree")
)

// Description explains where the secret is kept.
func Description() string { return "Windows DPAPI（仅当前用户可解密）" }

// Save seals value with the current user's DPAPI key.
func Save(path, value string) error {
	sealed, err := dpapi([]byte(value), true)
	if err != nil {
		return err
	}
	return fileutil.Replace(path, sealed, 0o600)
}

// Load opens a value written by Save. A missing file yields an empty string.
func Load(path string) (string, error) {
	sealed, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	plain, err := dpapi(sealed, false)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func dpapi(input []byte, encrypt bool) ([]byte, error) {
	if len(input) == 0 {
		return nil, errors.New("凭据内容为空")
	}
	source := dataBlob{length: uint32(len(input)), pointer: &input[0]}
	var result dataBlob
	procedure := unprotectData
	if encrypt {
		procedure = protectData
	}
	ok, _, _ := procedure.Call(
		uintptr(unsafe.Pointer(&source)),
		0, 0, 0, 0, 1,
		uintptr(unsafe.Pointer(&result)),
	)
	if ok == 0 {
		return nil, errors.New("Windows 凭据保护失败")
	}
	defer freeMemory.Call(uintptr(unsafe.Pointer(result.pointer)))
	return append([]byte(nil), unsafe.Slice(result.pointer, int(result.length))...), nil
}
