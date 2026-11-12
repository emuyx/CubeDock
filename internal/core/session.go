package core

import (
	"fmt"
	"strings"

	"github.com/emuyx/cubedock/internal/netease"
)

// NormalizeInteropSession accepts a 基岩互通版 session copied out of the Fever
// Games launcher in any of the shapes the console may receive. It is kept for
// the existing interop login path; mobile sessions use NormalizeSession.
func NormalizeInteropSession(input string) (token, platform, expiresAt string, err error) {
	token, resolved, expiresAt, err := NormalizeSession(input, string(netease.AuthInterop))
	if err != nil {
		return "", "", "", err
	}
	return token, string(resolved), expiresAt, nil
}

// NormalizeSession validates a session for one auth method and returns the
// stored token form, the resolved method and the access-token expiry. An empty
// method infers the provider from the session itself.
func NormalizeSession(input, method string) (token, resolved string, expiresAt string, err error) {
	var parsed netease.AuthMethod
	if strings.TrimSpace(method) != "" {
		parsed, err = netease.ParseAuthMethod(method)
		if err != nil {
			return "", "", "", fmt.Errorf("登录方式无效：%w", err)
		}
	}
	token, resolvedMethod, expiresAt, err := netease.NormalizeSession(input, parsed)
	if err != nil {
		return "", "", "", err
	}
	return token, string(resolvedMethod), expiresAt, nil
}
