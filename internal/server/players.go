package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// players returns the live guest list so the console can show per-room player
// actions without scraping the text console output.
func (h *consoleHandler) players(w http.ResponseWriter, _ *http.Request) {
	respond(w, h.manager.ListPlayers())
}

func (h *consoleHandler) kickPlayer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Target string `json:"target"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || body.Target == "" {
		http.Error(w, "请指定要断开的玩家", http.StatusBadRequest)
		return
	}
	status, err := h.manager.KickPlayer(body.Target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respond(w, status)
}

func (h *consoleHandler) roomMembers(w http.ResponseWriter, r *http.Request) {
	index, err := strconv.Atoi(r.URL.Query().Get("index"))
	if err != nil || index < 0 {
		http.Error(w, "账号序号无效", http.StatusBadRequest)
		return
	}
	respond(w, map[string]any{"index": index, "members": h.manager.RoomMembers(index)})
}

func (h *consoleHandler) kickRoomMember(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Index    int    `json:"index"`
		PlayerID uint32 `json:"player_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || body.Index < 0 || body.PlayerID == 0 {
		http.Error(w, "请指定账号序号和房间玩家 ID", http.StatusBadRequest)
		return
	}
	if err := h.manager.KickRoomMember(body.Index, body.PlayerID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respond(w, map[string]any{"ok": true, "index": body.Index, "player_id": body.PlayerID})
}

func (h *consoleHandler) roomKickPlayer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Target string `json:"target"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || body.Target == "" {
		http.Error(w, "请指定要踢出的玩家", http.StatusBadRequest)
		return
	}
	status, err := h.manager.RoomKickPlayer(body.Target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respond(w, status)
}

func (h *consoleHandler) friendRequests(w http.ResponseWriter, r *http.Request) {
	index, err := strconv.Atoi(r.URL.Query().Get("index"))
	if err != nil || index < 0 {
		http.Error(w, "账号序号无效", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	requests, err := h.manager.FriendRequests(ctx, index)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respond(w, map[string]any{"index": index, "requests": requests})
}

func (h *consoleHandler) acceptFriendRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Index int    `json:"index"`
		UID   uint64 `json:"uid,string"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || body.Index < 0 || body.UID == 0 {
		http.Error(w, "请指定账号序号和好友申请 UID", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := h.manager.AcceptFriendRequest(ctx, body.Index, body.UID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respond(w, map[string]any{"ok": true, "index": body.Index, "uid": body.UID})
}

func (h *consoleHandler) transferPlayer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Target  string `json:"target"`
		Address string `json:"address"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || body.Target == "" || body.Address == "" {
		http.Error(w, "请指定玩家和目标地址", http.StatusBadRequest)
		return
	}
	status, err := h.manager.TransferPlayer(body.Target, body.Address)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respond(w, status)
}
