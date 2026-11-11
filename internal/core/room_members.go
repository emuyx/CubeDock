package core

import (
	"fmt"
	"sort"
	"time"
)

// RoomMemberStatus is one NetEase room member. InWorld distinguishes guests
// that only joined the room from those that completed the upstream login.
type RoomMemberStatus struct {
	ID           string `json:"id"`
	Room         int    `json:"room"`
	RoomPlayerID uint32 `json:"room_player_id"`
	Name         string `json:"name,omitempty"`
	Connected    bool   `json:"connected"`
	InWorld      bool   `json:"in_world"`
	JoinedAt     string `json:"joined_at,omitempty"`
}

// RoomMembers returns the NetEase room roster for one room, merged with the
// world player registry so names and world state are visible.
func (m *Manager) RoomMembers(index int) []RoomMemberStatus {
	m.mu.Lock()
	host := m.hosts[index]
	m.mu.Unlock()
	if host == nil {
		return nil
	}
	members := host.RoomMembers()
	if len(members) == 0 {
		return nil
	}
	world := make(map[uint32]PlayerStatus, len(members))
	for _, player := range m.players.list() {
		if player.RoomPlayerID != 0 {
			world[player.RoomPlayerID] = player
		}
	}
	out := make([]RoomMemberStatus, 0, len(members))
	for _, member := range members {
		status := RoomMemberStatus{
			ID:           fmt.Sprintf("r%d", member.PlayerID),
			Room:         index + 1,
			RoomPlayerID: member.PlayerID,
			Connected:    member.Connected,
			JoinedAt:     member.JoinedAt.Format(time.RFC3339),
		}
		if player, ok := world[member.PlayerID]; ok {
			status.Name = player.Name
			status.InWorld = player.State == PlayerJoined
			status.Connected = status.Connected || status.InWorld
		}
		out = append(out, status)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RoomPlayerID < out[j].RoomPlayerID })
	return out
}

// KickRoomMember kicks a member by the TanLobby player ID. It works for guests
// that have not entered the world and have no local player-registry entry.
func (m *Manager) KickRoomMember(index int, playerID uint32) error {
	if playerID == 0 {
		return fmt.Errorf("房间玩家 ID 无效")
	}
	m.mu.Lock()
	host := m.hosts[index]
	m.mu.Unlock()
	if host == nil {
		return fmt.Errorf("房间控制连接不可用，请确认房间在线")
	}
	if err := host.KickPlayer(playerID); err != nil {
		return err
	}
	if cancel := m.players.cancelByTanID(playerID); cancel != nil {
		cancel()
	}
	m.emit("控制台", fmt.Sprintf("管理员已请求房间踢出玩家 %d（房间 %d）", playerID, index+1))
	return nil
}
