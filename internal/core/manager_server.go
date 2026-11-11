// Package core owns CubeDock's room lifecycle, account list and event log. It
// has no HTTP dependencies so the console is only one possible front end.
package core

import (
	"context"
	"errors"
	"fmt"
	"github.com/Happy2018new/nemc-tan-lobby-solver/core/nethernet"
	"github.com/emuyx/cubedock/internal/auth"
	"github.com/emuyx/cubedock/internal/bridge"
	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/friends"
	"github.com/emuyx/cubedock/internal/relay"
	"io"
	"net"
	"sync"
	"time"
)

// shouldAutoAcceptFriends reports whether pending friend requests may be
// accepted. Accepting them while the room is friends-only would let any
// stranger join the supposedly private room, so that combination is skipped.
func shouldAutoAcceptFriends(profile config.Profile) bool {
	return profile.AutoAcceptFriends && !profile.FriendsOnly
}

// friendCredentials let the console list and accept pending friend requests
// while the room is online.
type friendCredentials struct {
	uid uint32
	key []byte
}

func (m *Manager) serveRoom(ctx context.Context, index int, profile config.Profile, host roomHost, info auth.RoomInfo, rl roomLog) error {
	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	m.mu.Lock()
	m.hosts[index] = host
	if info.UserID != 0 && len(info.EncryptKey) >= 16 {
		m.friendSessions[index] = friendCredentials{
			uid: info.UserID,
			key: append([]byte(nil), info.EncryptKey[:16]...),
		}
	}
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		if current, ok := m.hosts[index]; ok && current == host {
			delete(m.hosts, index)
		}
		delete(m.friendSessions, index)
		m.mu.Unlock()
	}()

	friendsDone := make(chan struct{})
	go func() {
		defer close(friendsDone)
		if shouldAutoAcceptFriends(profile) {
			friends.Watch(sessionCtx, info.UserID, info.EncryptKey, rl.event, rl.debug)
			return
		}
		if profile.AutoAcceptFriends && profile.FriendsOnly {
			rl.event("已跳过好友申请自动同意：房间设置为仅好友可进")
		}
	}()
	defer func() { cancel(); <-friendsDone }()

	var bridges sync.WaitGroup
	acceptDone := make(chan struct{})
	defer func() {
		cancel()
		host.Close()
		<-acceptDone
		bridges.Wait()
	}()

	acceptErrors := make(chan error, 1)
	go func() {
		defer close(acceptDone)
		for {
			conn, err := host.Accept()
			if err != nil {
				acceptErrors <- err
				return
			}
			if conn == nil {
				acceptErrors <- net.ErrClosed
				return
			}
			// NetEase counts the host bot, so a room admits MaxPlayers-1 guests.
			full := true
			m.updateRoom(index, func(status *RoomStatus) {
				full = status.Players >= profile.MaxPlayers-1
				if !full {
					status.Players++
				}
			})
			if full {
				conn.Close()
				rl.event(fmt.Sprintf("房间已满（最多 %d 位玩家），拒绝了一个新连接", profile.MaxPlayers-1))
				continue
			}
			client, ok := conn.(*nethernet.Conn)
			if !ok {
				conn.Close()
				m.updateRoom(index, func(status *RoomStatus) {
					if status.Players > 0 {
						status.Players--
					}
				})
				rl.event("收到非预期的连接类型，已关闭")
				continue
			}
			bridges.Add(1)
			go func(client *nethernet.Conn) {
				startedAt := time.Now()
				playerCtx, playerCancel := context.WithCancel(sessionCtx)
				guest := m.players.register(index, host.RoomID(), info.PlayerName, playerCancel, host.PlayerIDFor(client))
				defer playerCancel()
				defer bridges.Done()
				defer client.Close()
				defer m.updateRoom(index, func(status *RoomStatus) {
					if status.Players > 0 {
						status.Players--
					}
				})
				defer m.players.unregister(guest)
				rl.debug("玩家连接已建立，正在转发到本地服务器")
				dialCtx, stop := context.WithTimeout(playerCtx, 8*time.Second)
				server, err := dialUpstream(dialCtx, profile)
				stop()
				if err != nil {
					if playerCtx.Err() == nil {
						rl.event("有玩家尝试进入，但本地服务器连接失败")
					}
					return
				}
				defer server.Close()
				rl.debug("本地服务器 RakNet 连接成功")
				encrypted, err := bridge.NewCryptoRelay(server, rl.debug)
				if err != nil {
					rl.event("无法生成后端连接密钥")
					return
				}
				encrypted.SetPlayerHooks(
					func(info bridge.PlayerInfo) {
						m.players.noteLogin(guest, info.Name)
						rl.debug(fmt.Sprintf("已读取玩家登录身份：%s（UID %s）",
							playerLabel(info.Name), firstNonEmpty(info.UID, "—")))
					},
					func(info bridge.PlayerInfo, status int32) {
						m.players.notePlayStatus(guest, status)
						if status == 0 {
							rl.event(fmt.Sprintf("玩家 %s 已加入服务器%s",
								playerLabel(info.Name), m.roomOccupancy(index, profile.MaxPlayers)))
							return
						}
						rl.event(fmt.Sprintf("玩家 %s 登录失败：%s",
							playerLabel(info.Name), bridge.PlayStatusName(status)))
					},
				)
				rl.debug("网易上游：已代客户端完成登录重签与加密握手，游戏包直通")
				clientSync := &relay.SyncStream{Stream: client}
				clientStream := &relay.Observer{Stream: clientSync, Label: "玩家 → 本地服务器", Trace: rl.debug}
				serverStream := &relay.Observer{Stream: encrypted, Label: "本地服务器 → 玩家", Trace: rl.debug}
				m.players.attachStreams(guest, &clientStream.Packets, &serverStream.Packets)
				m.players.attachClient(guest, clientSync)
				err = relay.Pipe(playerCtx, clientStream, serverStream)
				if err != nil {
					rl.debug("转发结束：" + err.Error())
				}
				player := encrypted.Player()
				switch {
				case encrypted.Joined():
					rl.event(fmt.Sprintf("玩家 %s 已离开服务器（在线 %s，上行 %d 包，下行 %d 包）",
						playerLabel(player.Name), shortDuration(time.Since(startedAt)),
						clientStream.PacketCount(), serverStream.PacketCount()))
				case player.Name != "":
					rl.event(fmt.Sprintf("玩家 %s 未完成登录，连接已断开（上行 %d 包，下行 %d 包）",
						playerLabel(player.Name), clientStream.PacketCount(), serverStream.PacketCount()))
				default:
					rl.event(fmt.Sprintf("玩家连接已断开（未完成登录，上行 %d 包，下行 %d 包）",
						clientStream.PacketCount(), serverStream.PacketCount()))
				}
				if err != nil && sessionCtx.Err() == nil &&
					!errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
					rl.event(fmt.Sprintf("玩家连接异常：%v", err))
				}
			}(client)
		}
	}()

	upstreamOnline := true
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			cancel()
			return ctx.Err()
		case <-host.Done():
			cancel()
			return host.Err()
		case err := <-acceptErrors:
			cancel()
			return fmt.Errorf("WebRTC 信令断开: %w", err)
		case <-ticker.C:
			version, pingErr := pingUpstream(sessionCtx, profile)
			online := pingErr == nil
			if online != upstreamOnline {
				if online {
					rl.event("上游服务器已恢复，房间继续在线")
				} else {
					rl.event("上游服务器不可达，等待恢复")
				}
				upstreamOnline = online
			}
			m.updateRoom(index, func(status *RoomStatus) {
				status.UpstreamOnline = online
				if online {
					status.UpstreamVersion = version
					status.Phase = "online"
				} else {
					status.UpstreamVersion = ""
					status.Phase = "upstream_offline"
				}
			})
		}
	}
}
