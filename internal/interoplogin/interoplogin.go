// Package interoplogin implements the NetEase FeverGames (基岩互通版) email
// login chain: a50 device registration + mpay password login + create_ticket +
// x19 ticket login + uni_sauth. The resulting sauth is the platform=pc /
// a50_sdk_cn session consumed by the PE auth path.
package interoplogin

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/emuyx/cubedock/internal/secret"
)

const (
	baseURL       = "https://service.mkey.163.com"
	uniSauthURL   = "https://mgbsdk.matrix.netease.com/x19/sdk/uni_sauth"
	a50GameID     = "aecglf6ee4aaaarz-g-a50"
	x19GameID     = "aecfrxodyqaaaajp-g-x19"
	a50AppChannel = "netease.allysdk3rd"
	x19AppChannel = "a50_sdk_cn"
	a50CV         = "c4.21.0"
	a50GV         = "1.18.45.8"
	x19CV         = "c4.16.0"
	// x19GV follows the current NetEase game version. 3.10 = 1.21.130.0.
	x19GV         = "1.21.130.0"
	x19SDKVersion = "4.16.0"
	mcountAppKey  = "EEkEEXLymcNjM42yLY3Bn6AO15aGy4yq"
	optFields     = "nickname,avatar,realname_status,mobile_bind_status,mask_related_mobile,related_login_status"
	deviceFile    = "interop-device.dat"

	defaultSystemVersion = "10.0.26200"
)

// verifyURLPattern stops at whitespace, quotes and common CJK punctuation so a
// URL embedded in a Chinese error sentence does not swallow the prose that
// follows it. ASCII trailing punctuation is trimmed afterwards.
var verifyURLPattern = regexp.MustCompile(`https?://[^\s"'<>，。；：！？）】]+`)

// systemVersion is sent as sv/system_version in the a50/x19 login form. It uses
// the host build on Windows (matching the official client), a generic fallback
// elsewhere, and an optional environment override. The value is never stored in
// source or documentation.
func systemVersion() string {
	if value := strings.TrimSpace(os.Getenv("CUBEDOCK_INTEROP_SYSTEM_VERSION")); value != "" {
		return value
	}
	if value := hostSystemVersion(); value != "" {
		return value
	}
	return defaultSystemVersion
}

// VerifyRequired is returned when NetEase asks for an interactive security
// check. The console shows URL so the operator can complete it and retry.
type VerifyRequired struct {
	Message string
	URL     string
}

func (e *VerifyRequired) Error() string {
	if e.URL != "" {
		return e.Message + "（" + e.URL + "）"
	}
	return e.Message
}

type device struct {
	ID       string `json:"id"`
	Key      string `json:"key,omitempty"`
	UDID     string `json:"udid"`
	UniqueID string `json:"unique_id,omitempty"`
	MAC      string `json:"mac,omitempty"`
}

type state struct {
	A50 device `json:"a50"`
	X19 device `json:"x19"`
}

type Service struct {
	dir    string
	client *http.Client
	mu     sync.Mutex
}

func New(dir string) *Service {
	return &Service{
		dir:    dir,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Login returns the inner sauth_json object (without the outer cookie wrapper).
func (s *Service) Login(ctx context.Context, email, password string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.loadState()
	if st.A50.ID == "" || st.A50.Key == "" || st.X19.ID == "" || st.X19.UDID == "" {
		var err error
		st, err = s.createState(ctx)
		if err != nil {
			return nil, err
		}
	}

	sauth, err := s.loginWithState(ctx, st, email, password)
	if err != nil {
		return nil, err
	}
	return sauth, nil
}

func (s *Service) loginWithState(ctx context.Context, st state, email, password string) (map[string]any, error) {
	passwordMD5 := fmt.Sprintf("%x", md5.Sum([]byte(password)))
	params, err := encryptParams(st.A50.Key, st.A50.UniqueID, email, passwordMD5)
	if err != nil {
		return nil, fmt.Errorf("加密登录参数失败: %w", err)
	}
	transID, uniTx, mcountTx, processID := transactionIDs()

	loginForm := map[string]string{
		"app_channel":           a50AppChannel,
		"app_mode":              "2",
		"app_type":              "games",
		"arch":                  "win_x64",
		"cv":                    "c4.21.0",
		"game_id":               a50GameID,
		"gv":                    "1.18.45.8",
		"is_outer_browser_open": "1",
		"jf_game_id":            "a50",
		"mcount_app_key":        mcountAppKey,
		"mcount_transaction_id": mcountTx,
		"opt_fields":            optFields,
		"params":                params,
		"process_id":            processID,
		"sv":                    systemVersion(),
		"transid":               transID,
		"un":                    base64.StdEncoding.EncodeToString([]byte(email)),
		"uni_transaction_id":    uniTx,
		"updater_cv":            "c1.0.0",
	}
	var loginResp struct {
		User      json.RawMessage `json:"user"`
		Code      int             `json:"code"`
		Message   string          `json:"message"`
		Reason    string          `json:"reason"`
		VerifyURL string          `json:"verify_url"`
		Verify    string          `json:"verify"`
	}
	if err := s.postForm(ctx, fmt.Sprintf("/mpay/games/%s/devices/%s/users", a50GameID, st.A50.ID), loginForm, &loginResp); err != nil {
		return nil, err
	}
	user, err := decodeUser(loginResp.User)
	if err != nil || user["token"] == "" || user["id"] == "" {
		verifyURL := firstNonEmpty(
			loginResp.VerifyURL,
			loginResp.Verify,
			extractVerifyURL(loginResp.Message),
			extractVerifyURL(loginResp.Reason),
		)
		if loginResp.Code == 1351 || verifyURL != "" {
			msg := firstNonEmpty(loginResp.Message, loginResp.Reason, "网易要求安全验证")
			return nil, &VerifyRequired{Message: msg, URL: verifyURL}
		}
		msg := firstNonEmpty(loginResp.Message, loginResp.Reason, fmt.Sprintf("登录失败 (code: %d)", loginResp.Code))
		return nil, fmt.Errorf("a50 登录失败: %s", msg)
	}

	createForm := map[string]string{}
	for key, value := range loginForm {
		createForm[key] = value
	}
	delete(createForm, "un")
	delete(createForm, "params")
	delete(createForm, "opt_fields")
	createForm["device_id"] = st.A50.ID
	createForm["token"] = user["token"]
	createForm["user_id"] = user["id"]
	var ticketResp struct {
		Ticket string `json:"ticket"`
	}
	if err := s.postForm(ctx, "/mpay/api/users/create_ticket", createForm, &ticketResp); err != nil {
		return nil, err
	}
	if ticketResp.Ticket == "" {
		return nil, fmt.Errorf("创建登录票据失败")
	}

	x19TransID, x19UniTx, x19Mcount, x19ProcessID := transactionIDs()
	x19Form := map[string]string{
		"app_channel":           x19AppChannel,
		"app_mode":              "2",
		"app_type":              "games",
		"arch":                  "win_x64",
		"cv":                    "c4.16.0",
		"device_id":             st.X19.ID,
		"game_id":               x19GameID,
		"gv":                    "1.21.130.0",
		"is_remember_account":   "0",
		"jf_game_id":            "x19",
		"mcount_app_key":        mcountAppKey,
		"mcount_transaction_id": x19Mcount,
		"opt_fields":            optFields,
		"process_id":            x19ProcessID,
		"sv":                    systemVersion(),
		"ticket":                ticketResp.Ticket,
		"transid":               x19TransID,
		"uni_transaction_id":    x19UniTx,
		"updater_cv":            "c1.0.0",
	}
	var x19Resp struct {
		User json.RawMessage `json:"user"`
		Code int             `json:"code"`
	}
	if err := s.postForm(ctx, "/mpay/api/users/login/ticket", x19Form, &x19Resp); err != nil {
		return nil, err
	}
	x19User, err := decodeUser(x19Resp.User)
	if err != nil || x19User["token"] == "" || x19User["id"] == "" {
		return nil, fmt.Errorf("x19 票据登录失败 (code: %d)", x19Resp.Code)
	}

	ip := detectPublicIP(ctx)
	aimInfo := fmt.Sprintf(`{"aim":"%s","tz":"+0800","tzid":"","country":"CN"}`, ip)
	clientLoginSN := randomUpper(32)
	sauthReq := map[string]any{
		"gameid":           "x19",
		"login_channel":    "netease",
		"app_channel":      x19AppChannel,
		"platform":         "pc",
		"sdkuid":           x19User["id"],
		"sessionid":        x19User["token"],
		"sdk_version":      "4.16.0",
		"udid":             st.X19.UDID,
		"deviceid":         st.X19.ID,
		"client_login_sn":  clientLoginSN,
		"gas_token":        "",
		"source_platform":  "pc",
		"ip":               ip,
		"get_access_token": "1",
		"aim_info":         aimInfo,
		"lang":             "zh",
	}
	var uniResp struct {
		Status          string `json:"status"`
		Code            int    `json:"code"`
		Subcode         int    `json:"subcode"`
		UnisdkLoginJSON string `json:"unisdk_login_json"`
	}
	if err := s.postJSON(ctx, uniSauthURL, sauthReq, &uniResp); err != nil {
		return nil, err
	}
	if uniResp.Status != "ok" {
		return nil, fmt.Errorf("uni_sauth 失败 (code: %d/%d)", uniResp.Code, uniResp.Subcode)
	}
	var loginJSON struct {
		OAuth2 struct {
			AccessToken string `json:"access_token"`
		} `json:"oauth2"`
	}
	decoded, err := base64.StdEncoding.DecodeString(uniResp.UnisdkLoginJSON)
	if err != nil {
		return nil, fmt.Errorf("解析 uni_sauth 响应失败: %w", err)
	}
	if err := json.Unmarshal(decoded, &loginJSON); err != nil {
		return nil, fmt.Errorf("解析 uni_sauth 响应失败: %w", err)
	}
	if loginJSON.OAuth2.AccessToken == "" {
		return nil, fmt.Errorf("uni_sauth 没有返回 access_token")
	}
	return map[string]any{
		"access_token":     loginJSON.OAuth2.AccessToken,
		"aim_info":         aimInfo,
		"app_channel":      x19AppChannel,
		"client_login_sn":  clientLoginSN,
		"deviceid":         st.X19.ID,
		"gameid":           "x19",
		"gas_token":        "",
		"get_access_token": "1",
		"ip":               ip,
		"login_channel":    "netease",
		"platform":         "pc",
		"sdk_version":      "4.16.0",
		"sdkuid":           x19User["id"],
		"sessionid":        x19User["token"],
		"source_platform":  "pc",
		"tdid":             "",
		"udid":             st.X19.UDID,
	}, nil
}

func (s *Service) loadState() state {
	path := filepath.Join(s.dir, deviceFile)
	raw, err := secret.Load(path)
	if err != nil || raw == "" {
		return state{}
	}
	var st state
	if json.Unmarshal([]byte(raw), &st) != nil {
		return state{}
	}
	return st
}

func (s *Service) saveState(st state) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return secret.Save(filepath.Join(s.dir, deviceFile), string(raw))
}

func (s *Service) createState(ctx context.Context) (state, error) {
	st := state{}
	var err error
	st.A50, err = s.createDevice(ctx, a50GameID, "a50", a50AppChannel, "c4.21.0", "1.18.45.8", 10)
	if err != nil {
		return state{}, fmt.Errorf("创建 a50 设备失败: %w", err)
	}
	st.X19, err = s.createDevice(ctx, x19GameID, "x19", x19AppChannel, "c4.16.0", "1.21.130.0", 32)
	if err != nil {
		return state{}, fmt.Errorf("创建 x19 设备失败: %w", err)
	}
	if err := s.saveState(st); err != nil {
		return state{}, fmt.Errorf("保存互通版设备失败: %w", err)
	}
	return st, nil
}

func (s *Service) createDevice(ctx context.Context, gameID, jfGameID, appChannel, cv, gv string, udidLen int) (device, error) {
	transID, uniTx, _, processID := transactionIDs()
	form := map[string]string{
		"app_channel":           appChannel,
		"app_mode":              "2",
		"app_type":              "games",
		"arch":                  "win_x64",
		"brand":                 "Microsoft",
		"cv":                    cv,
		"device_model":          "pc_mode",
		"device_name":           "COMPUTER",
		"device_type":           "Computer",
		"game_id":               gameID,
		"gv":                    gv,
		"init_urs_device":       "0",
		"is_outer_browser_open": "1",
		"jf_game_id":            jfGameID,
		"mac":                   randomMac(),
		"mcount_app_key":        mcountAppKey,
		"mcount_transaction_id": "1",
		"process_id":            processID,
		"resolution":            "3072*1920",
		"sv":                    systemVersion(),
		"system_name":           "windows",
		"system_version":        systemVersion(),
		"transid":               transID,
		"udid":                  randomUpper(udidLen),
		"uni_transaction_id":    uniTx,
		"unique_id":             randomLower(32),
		"updater_cv":            "c1.0.0",
	}
	var resp struct {
		Device struct {
			ID  string `json:"id"`
			Key string `json:"key"`
		} `json:"device"`
	}
	if err := s.postForm(ctx, "/mpay/games/"+gameID+"/devices", form, &resp); err != nil {
		return device{}, err
	}
	if resp.Device.ID == "" || resp.Device.Key == "" {
		return device{}, fmt.Errorf("设备注册响应缺少 id/key")
	}
	upload := map[string]string{}
	for key, value := range form {
		upload[key] = value
	}
	upload["device_id"] = resp.Device.ID
	delete(upload, "init_urs_device")
	var uploadResp map[string]any
	_ = s.postForm(ctx, "/mpay/api/devices/upload", upload, &uploadResp)
	return device{
		ID:       resp.Device.ID,
		Key:      resp.Device.Key,
		UDID:     form["udid"],
		UniqueID: form["unique_id"],
		MAC:      form["mac"],
	}, nil
}

func (s *Service) postForm(ctx context.Context, path string, form map[string]string, out any) error {
	values := url.Values{}
	for key, value := range form {
		values.Set(key, value)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "*/*")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("网易接口返回 HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("网易接口响应格式无效: %w", err)
		}
	}
	return nil
}

func (s *Service) postJSON(ctx context.Context, endpoint string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("网易接口返回 HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	return json.Unmarshal(raw, out)
}

func decodeUser(raw json.RawMessage) (map[string]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("登录响应缺少 user")
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, err
		}
		raw = []byte(text)
	}
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for key, value := range values {
		switch v := value.(type) {
		case string:
			out[key] = v
		case float64:
			out[key] = fmt.Sprint(v)
		}
	}
	return out, nil
}

func encryptParams(keyHex, uniqueID, username, passwordMD5 string) (string, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil || len(key) != 16 {
		return "", fmt.Errorf("设备密钥无效")
	}
	payload, err := json.Marshal(map[string]string{
		"password":  passwordMD5,
		"unique_id": uniqueID,
		"username":  username,
	})
	if err != nil {
		return "", err
	}
	padding := aes.BlockSize - len(payload)%aes.BlockSize
	payload = append(payload, bytes.Repeat([]byte{byte(padding)}, padding)...)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	out := make([]byte, len(payload))
	for start := 0; start < len(payload); start += aes.BlockSize {
		block.Encrypt(out[start:start+aes.BlockSize], payload[start:start+aes.BlockSize])
	}
	return hex.EncodeToString(out), nil
}

func transactionIDs() (transID, uniTx, mcountTx, processID string) {
	now := time.Now().UnixMilli()
	counter := randInt(100000000, 999999999)
	transID = fmt.Sprintf("%s_%d_%d", randomUpper(32), now, counter)
	uniTx = fmt.Sprintf("%s_%d_%d", randomUpper(32), now, counter+1)
	mcountTx = uuid.NewString() + "-2"
	processID = fmt.Sprint(os.Getpid())
	return
}

func detectPublicIP(ctx context.Context) string {
	for _, endpoint := range []string{"https://api.ipify.org", "https://ifconfig.me/ip"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}
		resp, err := (&http.Client{Timeout: 6 * time.Second}).Do(req)
		if err != nil {
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
		resp.Body.Close()
		ip := strings.TrimSpace(string(raw))
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	return "127.0.0.1"
}

func randomMac() string {
	raw := make([]byte, 6)
	_, _ = rand.Read(raw)
	raw[0] = (raw[0] | 0x02) & 0xfe
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", raw[0], raw[1], raw[2], raw[3], raw[4], raw[5])
}

func randomHex(n int) string {
	raw := make([]byte, (n+1)/2)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)[:n]
}

func randomUpper(n int) string {
	return strings.ToUpper(randomHex(n))
}

func randomLower(n int) string {
	return strings.ToLower(randomHex(n))
}

func randInt(min, max int) int {
	raw := make([]byte, 4)
	_, _ = rand.Read(raw)
	value := int(raw[0])<<24 | int(raw[1])<<16 | int(raw[2])<<8 | int(raw[3])
	if value < 0 {
		value = -value
	}
	return min + value%(max-min+1)
}

// extractVerifyURL pulls the verification link out of a NetEase error message.
// Some upstream errors put the URL only inside message/reason, which is why the
// console used to show an incomplete link without a usable button.
func extractVerifyURL(text string) string {
	match := verifyURLPattern.FindString(text)
	return strings.TrimRight(match, `).,;:!?，。；：！？）】"'`)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
