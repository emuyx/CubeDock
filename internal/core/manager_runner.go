// Package core owns CubeDock's room lifecycle, account list and event log. It
// has no HTTP dependencies so the console is only one possible front end.
package core

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/emuyx/cubedock/internal/auth"
	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/redact"
	"github.com/emuyx/cubedock/internal/room"
)

// roomHost is the part of room.Host the manager supervises. Tests substitute a
// fake so many-room startup, retry and shutdown run without NetEase.
type roomHost interface {
	RoomID() uint32
	Accept() (net.Conn, error)
	Done() <-chan struct{}
	Err() error
	Close()
	PlayerIDFor(net.Conn) uint32
	KickPlayer(uint32) error
	RoomMembers() []room.RoomMember
}

func (m *Manager) run(ctx context.Context, index int, profile config.Profile, token string) {
	rl := roomLog{manager: m, index: index, profile: profile.Name}
	rl.event("启动本地联机房主机器人")
	rl.event(fmt.Sprintf("直通模式 + 网易 RakNet v8（配置方案「%s」）：原始游戏批次直接转发，代客户端完成登录重签与加密握手",
		profile.Name))

	failures := 0
	for ctx.Err() == nil {
		m.updateRoom(index, func(status *RoomStatus) {
			status.Phase = "checking_upstream"
			status.RoomID = 0
			status.Players = 0
			status.RetryAt = ""
			status.Attempts++
			status.ProfileID = profile.ID
			status.ProfileName = profile.Name
			status.ConnectionMode = profile.ConnectionMode
		})

		var err error
		if profile.ConnectionMode == config.ConnectionTransfer {
			// A limbo may have exited while the room was retrying. Revive it
			// without changing the profile's reference count.
			if limboErr := m.ensureLimboProcess(profile); limboErr != nil {
				err = limboErr
			}
		}
		var version string
		if err == nil {
			version, err = m.pingFn(ctx, profile)
		}
		if err != nil {
			m.updateStatus(func(status *Status) { status.UpstreamOnline = false })
			if profile.ConnectionMode == config.ConnectionTransfer {
				err = fmt.Errorf("中转房间 %s 不可达（配置方案「%s」）：%w",
					upstreamAddress(profile), profile.Name, err)
			} else {
				err = fmt.Errorf("本地服务器 %s 不可达，请先启动 Bedrock 服务器（配置方案「%s」）：%w",
					upstreamAddress(profile), profile.Name, err)
			}
		} else {
			m.updateStatus(func(status *Status) {
				status.UpstreamOnline = true
				status.UpstreamVersion = version
			})
			m.updateRoom(index, func(status *RoomStatus) {
				status.Phase = "authenticating"
				status.UpstreamOnline = true
				status.UpstreamVersion = version
			})
			rl.debug("上游服务器在线，版本 " + version + "；开始鉴权")

			var info auth.RoomInfo
			authCtx, cancel := context.WithTimeout(ctx, 75*time.Second)
			info, err = m.prepareRoom(authCtx, profile.LobbyProtocolID, token)
			cancel()
			if err == nil {
				m.updateRoom(index, func(status *RoomStatus) {
					status.Account = info.PlayerName
					status.ProtocolID = info.ProtocolID
					status.Phase = "creating"
				})
				rl.debug(fmt.Sprintf("鉴权通过，房间协议 %d；连接开房节点", info.ProtocolID))
				var host roomHost
				createCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
				host, err = m.openRoom(createCtx, profile, info, rl.debug)
				cancel()
				if err == nil {
					openedAt := time.Now()
					m.updateRoom(index, func(status *RoomStatus) {
						status.Phase = "online"
						status.RoomID = host.RoomID()
						status.ConnectedAt = openedAt.Format(time.RFC3339)
						status.LastError = ""
					})
					rl.event(fmt.Sprintf("房间 %d 已上线（配置方案「%s」，协议 %d，上游 %s），房主机器人持续保活",
						host.RoomID(), profile.Name, info.ProtocolID, upstreamDescription(profile)))
					err = m.serveRoom(ctx, index, profile, host, info, rl)
					host.Close()
					// Only a session that stayed up long enough resets the
					// backoff. A room that opens and immediately drops must not
					// re-authenticate and recreate itself every few seconds.
					if time.Since(openedAt) >= 30*time.Second {
						failures = 0
					}
				}
			}
		}
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			err = net.ErrClosed
		}

		failures++
		delay := time.Duration(profile.RetrySeconds) * time.Second
		for attempt := 1; attempt < failures && delay < 90*time.Second; attempt++ {
			delay *= 2
		}
		if delay > 90*time.Second {
			delay = 90 * time.Second
		}
		message := redact.Sanitize(err.Error(), token)
		m.updateRoom(index, func(status *RoomStatus) {
			status.Phase = "retrying"
			status.LastError = message
			status.RoomID = 0
			status.Players = 0
			status.RetryAt = time.Now().Add(delay).Format(time.RFC3339)
			status.UpstreamOnline = false
		})
		rl.event(fmt.Sprintf("%s；%.0f 秒后重试", message, delay.Seconds()))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
