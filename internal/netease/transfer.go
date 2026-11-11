package netease

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type flexibleInt int

func (value *flexibleInt) UnmarshalJSON(data []byte) error {
	text := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if text == "" || text == "null" {
		*value = 0
		return nil
	}
	parsed, err := strconv.Atoi(text)
	if err != nil {
		return err
	}
	*value = flexibleInt(parsed)
	return nil
}

type transferEntry struct {
	Status        flexibleInt `json:"status"`
	IP            string      `json:"ip"`
	ID            flexibleInt `json:"id"`
	ServerType    string      `json:"ServerType"`
	SignalWebPort flexibleInt `json:"SignalWebPort"`
	Ports         []int       `json:"ports"`
}

func (c *Client) fetchRoomAuth(ctx context.Context) (RoomAuth, error) {
	c.mu.RLock()
	creds := c.creds
	discovered := c.discovered
	c.mu.RUnlock()
	if !discovered {
		return RoomAuth{}, fmt.Errorf("网易服务配置尚未加载")
	}
	if creds.Token == "" || creds.UserID == "" {
		return RoomAuth{}, fmt.Errorf("账号尚未完成 PE 认证")
	}
	userID, err := strconv.ParseUint(creds.UserID, 10, 32)
	if err != nil {
		return RoomAuth{}, fmt.Errorf("网易账号 UID 格式无效")
	}
	profile, err := c.fetchProfile(ctx)
	if err != nil {
		return RoomAuth{}, err
	}
	servers, err := c.fetchTransferServers(ctx)
	if err != nil {
		return RoomAuth{}, err
	}
	server, err := SelectTransferServer(servers, rand.Reader)
	if err != nil {
		return RoomAuth{}, err
	}
	return DeriveRoomAuth(uint32(userID), profile.Name, creds.Token, server, rand.Reader)
}

func (c *Client) fetchTransferServers(ctx context.Context) ([]TransferServer, error) {
	if strings.TrimSpace(c.g79.TransferServerURL) == "" {
		return nil, fmt.Errorf("网易服务配置缺少转发节点列表")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.g79.TransferServerURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "libhttpclient/1.0.0.0")
	request.Header.Set("Accept-Encoding", "gzip")
	request.Header.Set("Content-Type", "text/plain")
	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, fmt.Errorf("读取网易转发节点失败: %w", err)
	}
	defer response.Body.Close()
	raw, err := readHTTPBody(response)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取转发节点返回 HTTP %d", response.StatusCode)
	}
	var entries []transferEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("解析转发节点失败: %w", err)
	}
	servers := make([]TransferServer, 0, len(entries))
	for _, entry := range entries {
		servers = append(servers, TransferServer{
			ID: int(entry.ID), IP: entry.IP, ServerType: entry.ServerType,
			SignalWebPort: int(entry.SignalWebPort), Ports: entry.Ports, Status: int(entry.Status),
		})
	}
	return servers, nil
}

// CreateRoom returns the transfer credentials for one hosted room.
func (c *Client) CreateRoom(ctx context.Context) (RoomAuth, error) {
	return c.fetchRoomAuth(ctx)
}
