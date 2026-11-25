package netease

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrProfileNameMissing means the account exists but has not chosen its game
// name yet. The caller can offer the naming flow and retry the profile.
var ErrProfileNameMissing = errors.New("网易用户资料缺少游戏名")

// ProfileNameMissingError carries the partial identity so the console can save
// the session and show the account before a name is chosen.
type ProfileNameMissingError struct {
	Profile Profile
}

func (e *ProfileNameMissingError) Error() string { return ErrProfileNameMissing.Error() }

func (e *ProfileNameMissingError) Unwrap() error { return ErrProfileNameMissing }

type userDetailResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Entity  struct {
		EntityID       string `json:"entity_id"`
		Name           string `json:"name"`
		Level          int    `json:"level"`
		AvatarImageURL string `json:"avatar_image_url"`
		HeadImage      string `json:"head_image"`
	} `json:"entity"`
}

func (c *Client) fetchProfile(ctx context.Context) (Profile, error) {
	c.mu.RLock()
	creds := c.creds
	c.mu.RUnlock()
	if creds.Token == "" || creds.UserID == "" {
		return Profile{}, fmt.Errorf("账号尚未完成 PE 认证")
	}
	result, err := c.fetchUserDetail(ctx, creds, false)
	if err != nil {
		return Profile{}, err
	}
	if result.Code == 22 {
		result, err = c.fetchUserDetail(ctx, creds, true)
		if err != nil {
			return Profile{}, err
		}
	}
	if result.Code != 0 {
		return Profile{}, fmt.Errorf("读取网易用户资料失败 (code %d): %s", result.Code, result.Message)
	}
	avatar := strings.TrimSpace(result.Entity.AvatarImageURL)
	if avatar == "" {
		avatar = strings.TrimSpace(result.Entity.HeadImage)
	}
	identity := Profile{Name: result.Entity.Name, UID: creds.UserID, Level: result.Entity.Level, AvatarURL: avatar}
	if identity.Name == "" {
		return identity, &ProfileNameMissingError{Profile: identity}
	}
	return identity, nil
}

func (c *Client) fetchUserDetail(ctx context.Context, creds Credentials, core bool) (userDetailResponse, error) {
	var result userDetailResponse
	body := []byte("{}")
	path := "/pe-user-detail/get"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.g79.CoreServerURL+path, strings.NewReader(string(body)))
	if err != nil {
		return result, err
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("User-Agent", "WPFLauncher/0.0.0.0")
	request.Header.Set("user-id", creds.UserID)
	request.Header.Set("user-token", DynamicToken(creds.Token, path, body, core))
	response, err := c.HTTP.Do(request)
	if err != nil {
		return result, fmt.Errorf("读取用户资料失败: %w", err)
	}
	defer response.Body.Close()
	raw, err := readHTTPBody(response)
	if err != nil {
		return result, err
	}
	if response.StatusCode != http.StatusOK {
		return result, fmt.Errorf("读取用户资料返回 HTTP %d", response.StatusCode)
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, fmt.Errorf("解析用户资料失败: %w", err)
	}
	return result, nil
}

// Profile returns the public identity behind the authenticated session.
func (c *Client) Profile(ctx context.Context) (Profile, error) {
	return c.fetchProfile(ctx)
}
