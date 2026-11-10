package server

import (
	"encoding/json"
	"net/http"
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
