// Package auth turns a stored 互通版 session into the transfer credentials
// needed to host a room, and resolves the account's in-game profile.
package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/emuyx/cubedock/internal/netease"
)

// RoomInfo is the material returned by NetEase for hosting one room.
type RoomInfo struct {
	UserID           uint32
	PlayerName       string
	RaknetAddress    string
	RaknetRand       []byte
	RaknetAESRand    []byte
	SignalingAddress string
	SignalingSeed    []byte
	SignalingTicket  []byte
	EncryptKey       []byte
	DecryptKey       []byte
	ProtocolID       uint8
}

// Profile is the NetEase game identity behind a stored session.
type Profile struct {
	Name  string `json:"name"`
	UID   string `json:"uid,omitempty"`
	Level int    `json:"level,omitempty"`
}

// PrepareRoom performs PE authentication and obtains a fresh room transfer
// ticket. The method is inferred from the stored session.
func PrepareRoom(ctx context.Context, lobbyProtocolID uint8, token string) (RoomInfo, error) {
	return PrepareRoomFor(ctx, "", lobbyProtocolID, token)
}

// PrepareRoomFor performs PE authentication with an explicit provider. An
// empty method keeps the legacy inference path.
func PrepareRoomFor(ctx context.Context, method netease.AuthMethod, lobbyProtocolID uint8, token string) (RoomInfo, error) {
	var info RoomInfo
	session, err := unwrapToken(token)
	if err != nil {
		return info, err
	}
	client, err := netease.New(ctx)
	if err != nil {
		return info, fmt.Errorf("初始化网易认证客户端失败: %w", err)
	}
	if err = client.AuthenticateFor(ctx, method, session); err != nil {
		return info, fmt.Errorf("网易账号验证失败: %w", err)
	}
	result, err := client.CreateRoom(ctx)
	if err != nil {
		return info, fmt.Errorf("获取网易房间信息失败: %w", err)
	}
	if result.UserID == 0 || result.PlayerName == "" ||
		result.RaknetAddress == "" || result.SignalingAddress == "" {
		return info, fmt.Errorf("网易认证返回的房主信息不完整")
	}
	for _, address := range []string{result.RaknetAddress, result.SignalingAddress} {
		if _, err := net.ResolveUDPAddr("udp", address); err != nil {
			return info, fmt.Errorf("网易认证返回的转发地址无效")
		}
	}
	info = RoomInfo{
		UserID:           result.UserID,
		PlayerName:       result.PlayerName,
		RaknetAddress:    result.RaknetAddress,
		RaknetRand:       result.RaknetRand,
		RaknetAESRand:    result.RaknetAESRand,
		SignalingAddress: result.SignalingAddress,
		SignalingSeed:    result.SignalingSeed,
		SignalingTicket:  result.SignalingTicket,
		EncryptKey:       result.EncryptKey,
		DecryptKey:       result.DecryptKey,
		ProtocolID:       lobbyProtocolID,
	}
	if info.ProtocolID == 0 {
		info.ProtocolID = 43
	}
	if err := validateRoomInfo(info); err != nil {
		return info, err
	}
	return info, nil
}

// validateRoomInfo enforces the fixed sizes of the transfer-node key material.
// A short or empty key would only fail later, inside the encrypted room
// handshake, so reject it before the RakNet connection is opened.
func validateRoomInfo(info RoomInfo) error {
	if len(info.RaknetRand) != 16 {
		return fmt.Errorf("网易认证返回的 RakNet 随机数长度无效")
	}
	if len(info.RaknetAESRand) < 16 {
		return fmt.Errorf("网易认证返回的 RakNet AES 随机数长度无效")
	}
	if len(info.SignalingSeed) != 16 {
		return fmt.Errorf("网易认证返回的信令种子长度无效")
	}
	if len(info.SignalingTicket) < 16 {
		return fmt.Errorf("网易认证返回的信令票据长度无效")
	}
	if len(info.EncryptKey) != 32 || len(info.DecryptKey) != 32 {
		return fmt.Errorf("网易认证返回的房间加密密钥长度无效")
	}
	return nil
}

// LookupProfile verifies a stored session and returns its game name. The
// method is inferred from the stored session.
func LookupProfile(ctx context.Context, token string) (Profile, error) {
	return LookupProfileFor(ctx, "", token)
}

// LookupProfileFor verifies a stored session with an explicit provider. It is
// used before starting a room so the console can show who the account belongs to.
func LookupProfileFor(ctx context.Context, method netease.AuthMethod, token string) (Profile, error) {
	var profile Profile
	session, err := unwrapToken(token)
	if err != nil {
		return profile, err
	}
	client, err := netease.New(ctx)
	if err != nil {
		return profile, fmt.Errorf("初始化网易认证客户端失败: %w", err)
	}
	if err = client.AuthenticateFor(ctx, method, session); err != nil {
		return profile, fmt.Errorf("网易账号验证失败: %w", err)
	}
	identity, err := client.Profile(ctx)
	profile = Profile{Name: identity.Name, UID: identity.UID, Level: identity.Level}
	if err != nil {
		// A new account has no game name yet. Return the partial identity so
		// the console can save the session and offer the naming flow.
		return profile, err
	}
	if profile.Name == "" {
		return profile, netease.ErrProfileNameMissing
	}
	return profile, nil
}

// SetPlayerName sets the account's in-game name and returns the refreshed
// profile. Authentication happens once; the nickname update and the profile
// read share that PE session, avoiding the PE 2100 rate limit that a second
// immediate authentication would trigger.
func SetPlayerName(ctx context.Context, method netease.AuthMethod, token, name string) (Profile, error) {
	var profile Profile
	name = strings.TrimSpace(name)
	session, err := unwrapToken(token)
	if err != nil {
		return profile, err
	}
	client, err := netease.New(ctx)
	if err != nil {
		return profile, fmt.Errorf("初始化网易认证客户端失败: %w", err)
	}
	if err = authenticateWithRetry(ctx, client, method, session); err != nil {
		return profile, fmt.Errorf("网易账号验证失败: %w", err)
	}
	if err = client.SetNickname(ctx, name); err != nil {
		return profile, err
	}
	// The nickname endpoint may be visible to the profile service a moment
	// later; retry the read without repeating PE authentication.
	for attempt := 0; ; attempt++ {
		identity, profileErr := client.Profile(ctx)
		profileName := strings.TrimSpace(identity.Name)
		profile = Profile{Name: profileName, UID: identity.UID, Level: identity.Level}
		if profileErr == nil && profileName == name {
			return profile, nil
		}
		if profileErr != nil && !errors.Is(profileErr, netease.ErrProfileNameMissing) {
			return profile, profileErr
		}
		if attempt >= 2 {
			if profileErr != nil {
				return profile, profileErr
			}
			return profile, fmt.Errorf("游戏名设置后资料尚未更新，请稍后刷新")
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return profile, ctx.Err()
		case <-timer.C:
		}
	}
}

// authenticateWithRetry waits out the temporary PE 2100 rate limit. It is
// used by the naming flow because the operator usually triggers it right after
// adding a new account, which has just authenticated.
func authenticateWithRetry(ctx context.Context, client *netease.Client, method netease.AuthMethod, session string) error {
	delays := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}
	var err error
	for attempt := 0; ; attempt++ {
		err = client.AuthenticateFor(ctx, method, session)
		if err == nil {
			return nil
		}
		if !netease.IsPEAuthRateLimited(err) || attempt >= len(delays) {
			return err
		}
		timer := time.NewTimer(delays[attempt])
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// unwrapToken accepts a raw session object or the legacy b64: transport form.
func unwrapToken(token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("房间会话为空")
	}
	if !strings.HasPrefix(token, "b64:") {
		return token, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(token, "b64:"))
	if err != nil {
		return "", fmt.Errorf("房间会话编码无效")
	}
	return string(raw), nil
}
