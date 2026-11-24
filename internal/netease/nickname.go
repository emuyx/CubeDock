package netease

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"
)

// NetEase counts an ASCII character as 1 and a non-ASCII character (Chinese,
// etc.) as 1.5. The allowed scope is 3–12 characters, so use half-units to
// avoid floating point.
const (
	minNicknameWeight = 6  // 3 characters
	maxNicknameWeight = 24 // 12 characters
)

// ValidateNickname applies the console-side checks before calling NetEase.
// NetEase still owns uniqueness and any additional policy; its error message is
// returned unchanged so the operator can act on it.
func ValidateNickname(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("游戏名不能为空")
	}
	weight := nicknameWeight(name)
	if weight < minNicknameWeight || weight > maxNicknameWeight {
		return fmt.Errorf("游戏名长度需为 3–12 个字符（中文按 1.5 个字符计算）")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return errors.New("游戏名不能包含控制字符")
		}
	}
	return nil
}

// nicknameWeight returns half-character units: ASCII is 2, non-ASCII is 3.
func nicknameWeight(name string) int {
	weight := 0
	for _, r := range name {
		if r < 0x80 {
			weight += 2
		} else {
			weight += 3
		}
	}
	return weight
}

// SetNickname sets the account nickname used as the in-game name. New accounts
// must set one before /pe-user-detail/get returns a usable profile.
func (c *Client) SetNickname(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if err := ValidateNickname(name); err != nil {
		return err
	}
	c.mu.RLock()
	creds := c.creds
	gateway := strings.TrimSpace(c.g79.APIGatewayURL)
	c.mu.RUnlock()
	if creds.Token == "" || creds.UserID == "" {
		return errors.New("账号尚未完成 PE 认证")
	}
	if gateway == "" {
		return errors.New("网易服务配置缺少 API 网关")
	}
	if err := validateServiceURL(gateway); err != nil {
		return fmt.Errorf("网易 API 网关无效: %w", err)
	}

	path := "/pe-nickname-setting/update"
	body, err := json.Marshal(map[string]string{"name": name})
	if err != nil {
		return fmt.Errorf("序列化游戏名失败: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(gateway, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("User-Agent", "libhttpclient/1.0.0.0")
	request.Header.Set("user-id", creds.UserID)
	request.Header.Set("user-token", DynamicToken(creds.Token, path, body, false))
	response, err := c.HTTP.Do(request)
	if err != nil {
		return fmt.Errorf("设置游戏名请求失败: %w", err)
	}
	defer response.Body.Close()
	raw, err := readHTTPBody(response)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("设置游戏名返回 HTTP %d", response.StatusCode)
	}
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal(extractJSONObject(raw), &result); err != nil {
		return fmt.Errorf("解析设置游戏名响应失败: %w", err)
	}
	if result.Code != 0 {
		message := firstNonEmpty(result.Message, result.Reason, fmt.Sprintf("code %d", result.Code))
		return fmt.Errorf("设置游戏名失败: %s", message)
	}
	return nil
}
