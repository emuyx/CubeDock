// Package bunker is CubeDock's clean-room client for the Phoenix TanLobby
// authentication HTTP endpoints. It is licensed under MIT; see ../LICENSE.
package bunker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client calls the Phoenix TanLobby endpoints with a NetEase session.
type Client struct {
	AuthServer         string
	FBToken            string
	ProvidedPEAuthData string
	ProvidedSaAuthData string
}

// NewClient returns a client for the Phoenix auth server base URL.
func NewClient(authServer, token, peAuthData, saAuthData string) *Client {
	return &Client{
		AuthServer:         strings.TrimRight(authServer, "/"),
		FBToken:            token,
		ProvidedPEAuthData: peAuthData,
		ProvidedSaAuthData: saAuthData,
	}
}

func (c *Client) post(path string, request, response any) error {
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	resp, err := http.Post(c.AuthServer+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("post %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("post %s: unexpected HTTP status %d", path, resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("read %s response: %w", path, err)
	}
	if err := json.Unmarshal(raw, response); err != nil {
		return fmt.Errorf("decode %s response: %w", path, err)
	}
	return nil
}
