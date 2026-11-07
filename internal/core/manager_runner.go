// Package core owns CubeDock's room lifecycle, account list and event log. It
// has no HTTP dependencies so the console is only one possible front end.
package core

import (
	"context"
	"fmt"
	"github.com/emuyx/cubedock/internal/auth"
	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/redact"
	"github.com/emuyx/cubedock/internal/room"
	"net"
	"time"
)

func (m *Manager) run(ctx context.Context, index int, cfg config.Config, token string) {
	rl := roomLog{manager: m, source: fmt.Sprintf("房间 %d", index+1)}
	rl.event("启动本地联机房主机器人")
	rl.event("直通模式 + 网易 RakNet v8：原始游戏批次直接转发，代客户端完成登录重签与加密握手")

	failures := 0
	for ctx.Err() == nil {
		m.updateRoom(index, func(status *RoomStatus) {
			status.Phase = "checking_upstream"
			status.RoomID = 0
			status.Players = 0
			status.RetryAt = ""
			status.Attempts++
		})

		version, err := pingUpstream(ctx, cfg)
		if err == nil {
			m.updateStatus(func(status *Status) {
				status.UpstreamOnline = true
				status.UpstreamVersion = version
			})
			m.updateRoom(index, func(status *RoomStatus) { status.Phase = "authenticating" })
			rl.debug("本地 Bedrock 服务器在线，版本 " + version + "；开始鉴权")

			var info auth.RoomInfo
			authCtx, cancel := context.WithTimeout(ctx, 75*time.Second)
			info, err = auth.PrepareRoom(authCtx, cfg, token)
			cancel()
			if err == nil {
				m.updateRoom(index, func(status *RoomStatus) {
					status.Account = info.PlayerName
					status.ProtocolID = info.ProtocolID
					status.Phase = "creating"
				})
				rl.debug(fmt.Sprintf("鉴权通过，房间协议 %d；连接开房节点", info.ProtocolID))
				var host *room.Host
				createCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
				host, err = room.Open(createCtx, cfg, info, rl.debug)
				cancel()
				if err == nil {
					failures = 0
					m.updateRoom(index, func(status *RoomStatus) {
						status.Phase = "online"
						status.RoomID = host.ID
						status.ConnectedAt = time.Now().Format(time.RFC3339)
						status.LastError = ""
					})
					rl.event(fmt.Sprintf("房间 %d 已上线（协议 %d，上游 %s），房主机器人持续保活",
						host.ID, info.ProtocolID, upstreamDescription(cfg)))
					err = m.serveRoom(ctx, index, cfg, host, info, rl)
					host.Close()
				}
			}
		} else {
			m.updateStatus(func(status *Status) { status.UpstreamOnline = false })
			err = fmt.Errorf("本地服务器 %s 不可达，请先启动 Bedrock 服务器", upstreamAddress(cfg))
		}
		if ctx.Err() != nil {
			return
		}

		failures++
		delay := time.Duration(cfg.RetrySeconds) * time.Second
		for i := 1; i < failures && delay < 90*time.Second; i++ {
			delay *= 2
		}
		if delay > 90*time.Second {
			delay = 90 * time.Second
		}
		if err == nil {
			err = net.ErrClosed
		}
		message := redact.Sanitize(err.Error(), token)
		m.updateRoom(index, func(status *RoomStatus) {
			status.Phase = "retrying"
			status.LastError = message
			status.RoomID = 0
			status.Players = 0
			status.RetryAt = time.Now().Add(delay).Format(time.RFC3339)
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
