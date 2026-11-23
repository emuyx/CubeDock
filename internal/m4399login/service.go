// Package m4399login turns a 4399 account into a NetEase mobile (4399com/ad)
// sauth through the 4399 open-platform OAuth login.
//
// The chain was verified on 2026-11-23 with a real account:
// oauth-callback -> authorize page -> loginAndAuthorize (CryptoJS AES password)
// -> 302 -> callback JSON (code 100, uid/state) -> x19 sauth (4399com/ad) ->
// uni_sauth access_token -> PE authentication -> CreateRoom credentials.
// It does not use the 4399 Game Center xcrypt layer or the PC kid_wdsj chain.
package m4399login

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

const (
	defaultCallbackURL = "https://m.4399api.com/openapi/oauth-callback.html?gamekey=44770&game_key=115716"
	defaultLoginURL    = "https://ptlogin.4399.com/oauth2/loginAndAuthorize.do?channel=&sdk=op&sdk_version=3.18.0.682"
	defaultUniSauthURL = "https://mgbsdk.matrix.netease.com/x19/sdk/uni_sauth"
	defaultCaptchaURL  = "https://ptlogin.4399.com/ptlogin/captcha.do?captchaId="
	defaultUserAgent   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:134.0) Gecko/20100101 Firefox/134.0"
	sauthUserAgent     = "WPFLauncher/0.0.0.0"

	oauthPassphrase = "lzYW5qaXVqa"
	sauthChannel    = "4399com"
	sauthPlatform   = "ad"
	sauthSDKVersion = "3.18.0"
	deviceSize      = 32
)

var hiddenInputPattern = regexp.MustCompile(`(?is)<input[^>]*type=["']hidden["'][^>]*>`)
var nameAttributePattern = regexp.MustCompile(`(?i)name=["']([^"']*)`)
var valueAttributePattern = regexp.MustCompile(`(?i)value=["']([^"']*)`)
var errorMessagePattern = regexp.MustCompile(`(?is)id="login_err_msg"[^>]*>(.*?)</p>`)
var tagPattern = regexp.MustCompile(`(?s)<[^>]+>`)

// CaptchaRequired is returned when the OAuth login needs a graphical captcha.
type CaptchaRequired struct {
	CaptchaID string
	Image     []byte
	ImageType string
	Message   string
}

func (e *CaptchaRequired) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "需要输入图形验证码"
}

// Service performs the 4399 mobile OAuth login. Fields are optional; New fills
// the production endpoints.
type Service struct {
	HTTP        *http.Client
	CallbackURL string
	LoginURL    string
	UniSauthURL string
	CaptchaURL  string
	UserAgent   string
	Now         func() time.Time
}

func New() *Service {
	return &Service{
		CallbackURL: defaultCallbackURL,
		LoginURL:    defaultLoginURL,
		UniSauthURL: defaultUniSauthURL,
		CaptchaURL:  defaultCaptchaURL,
		UserAgent:   defaultUserAgent,
		Now:         time.Now,
	}
}

func (s *Service) callbackURL() string {
	if strings.TrimSpace(s.CallbackURL) != "" {
		return s.CallbackURL
	}
	return defaultCallbackURL
}

func (s *Service) loginURL() string {
	if strings.TrimSpace(s.LoginURL) != "" {
		return s.LoginURL
	}
	return defaultLoginURL
}

func (s *Service) uniSauthURL() string {
	if strings.TrimSpace(s.UniSauthURL) != "" {
		return s.UniSauthURL
	}
	return defaultUniSauthURL
}

func (s *Service) captchaURL() string {
	if strings.TrimSpace(s.CaptchaURL) != "" {
		return s.CaptchaURL
	}
	return defaultCaptchaURL
}

func (s *Service) userAgent() string {
	if strings.TrimSpace(s.UserAgent) != "" {
		return s.UserAgent
	}
	return defaultUserAgent
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Login returns the inner mobile_4399 sauth object. captchaID/captcha are the
// values from a previous *CaptchaRequired error; pass empty strings first.
func (s *Service) Login(ctx context.Context, username, password, captchaID, captcha string) (map[string]any, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("请填写 4399 账号")
	}
	if password == "" {
		return nil, fmt.Errorf("请填写 4399 密码")
	}
	client, err := s.client()
	if err != nil {
		return nil, err
	}
	callback, err := s.fetchCallback(ctx, client)
	if err != nil {
		return nil, err
	}
	authURL := textValue(callback, "result")
	if authURL == "" || !strings.Contains(authURL, "?") {
		return nil, fmt.Errorf("4399 OAuth 回调地址无效")
	}
	hidden, err := s.authorizePage(ctx, client, authURL)
	if err != nil {
		return nil, err
	}
	encrypted, err := encryptPassword(password)
	if err != nil {
		return nil, err
	}
	hidden["username"] = username
	hidden["password"] = encrypted
	hidden["sec"] = "1"
	hidden["auth_action"] = "ORILOGIN"
	hidden["captcha_id"] = strings.TrimSpace(captchaID)
	hidden["captcha"] = strings.TrimSpace(captcha)

	location, err := s.submitLogin(ctx, client, authURL, hidden)
	if err != nil {
		return nil, err
	}
	result, err := s.followLogin(ctx, client, authURL, location)
	if err != nil {
		return nil, err
	}
	uid := textValue(result.Result, "uid")
	state := textValue(result.Result, "state")
	if uid == "" || state == "" {
		return nil, fmt.Errorf("4399 OAuth 登录没有返回 uid/state")
	}
	sauth, err := s.buildSauth(uid, state)
	if err != nil {
		return nil, err
	}
	if err := s.enrichWithUniSauth(ctx, client, sauth); err != nil {
		return nil, err
	}
	return sauth, nil
}

func (s *Service) client() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("初始化 4399 登录会话失败")
	}
	base := &http.Client{Timeout: 30 * time.Second}
	if s.HTTP != nil {
		base = s.HTTP
	}
	working := *base
	working.Jar = jar
	working.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &working, nil
}

func (s *Service) fetchCallback(ctx context.Context, client *http.Client) (map[string]any, error) {
	body, err := s.get(ctx, client, s.callbackURL(), nil)
	if err != nil {
		return nil, fmt.Errorf("4399 OAuth 初始化失败: %w", err)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("4399 OAuth 初始化响应格式无效")
	}
	return result, nil
}

func (s *Service) authorizePage(ctx context.Context, client *http.Client, authURL string) (map[string]string, error) {
	body, err := s.get(ctx, client, authURL, nil)
	if err != nil {
		return nil, fmt.Errorf("读取 4399 登录页失败: %w", err)
	}
	hidden := map[string]string{}
	for _, tag := range hiddenInputPattern.FindAllString(string(body), -1) {
		name := attribute(nameAttributePattern, tag)
		if name == "" {
			continue
		}
		if _, exists := hidden[name]; exists {
			continue
		}
		hidden[name] = attribute(valueAttributePattern, tag)
	}
	if _, ok := hidden["client_id"]; !ok {
		return nil, fmt.Errorf("4399 登录页缺少 OAuth 参数")
	}
	return hidden, nil
}

func attribute(pattern *regexp.Regexp, tag string) string {
	match := pattern.FindStringSubmatch(tag)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func (s *Service) submitLogin(ctx context.Context, client *http.Client, authURL string, hidden map[string]string) (string, error) {
	form := url.Values{}
	for key, value := range hidden {
		form.Set(key, value)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.loginURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("构建 4399 登录请求失败")
	}
	request.Header.Set("User-Agent", s.userAgent())
	request.Header.Set("Referer", authURL)
	request.Header.Set("Origin", "https://ptlogin.4399.com")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("4399 登录请求失败: %w", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		location := response.Header.Get("Location")
		if location == "" {
			return "", fmt.Errorf("4399 登录跳转缺少地址")
		}
		return location, nil
	}
	// A 2xx HTML response means the login did not redirect: either an error or
	// a captcha challenge.
	text := decodeText(body)
	if message := errorMessage(text); message != "" {
		if strings.Contains(message, "验证码") {
			return "", s.captchaError(ctx, client)
		}
		for _, known := range []string{"用户不存在", "密码错误", "请稍后", "频繁", "账号被冻结", "账号存在风险"} {
			if strings.Contains(message, known) {
				return "", fmt.Errorf("4399 登录失败：%s", message)
			}
		}
		return "", fmt.Errorf("4399 登录失败：%s", message)
	}
	if strings.Contains(text, "验证码") {
		return "", s.captchaError(ctx, client)
	}
	return "", fmt.Errorf("4399 登录响应异常（没有跳转地址）")
}

func (s *Service) captchaError(ctx context.Context, client *http.Client) error {
	id, err := randomLowerHex(96)
	if err != nil {
		return fmt.Errorf("生成 4399 验证码 ID 失败")
	}
	image, contentType, err := s.getWithType(ctx, client, s.captchaURL()+url.QueryEscape(id), nil)
	if err != nil {
		return fmt.Errorf("获取 4399 验证码图片失败: %w", err)
	}
	return &CaptchaRequired{CaptchaID: id, Image: image, ImageType: contentType}
}

func decodeText(raw []byte) string {
	if utf8.Valid(raw) {
		return string(raw)
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(raw)
	if err != nil {
		return string(raw)
	}
	return string(decoded)
}

func errorMessage(text string) string {
	match := errorMessagePattern.FindStringSubmatch(text)
	if len(match) != 2 {
		return ""
	}
	return strings.TrimSpace(tagPattern.ReplaceAllString(match[1], ""))
}

type oauthResult struct {
	Code    json.RawMessage `json:"code"`
	Message string          `json:"message"`
	Result  map[string]any  `json:"result"`
}

func (s *Service) followLogin(ctx context.Context, client *http.Client, referer, location string) (oauthResult, error) {
	var result oauthResult
	current := location
	for hop := 0; hop < 5; hop++ {
		response, err := s.getResponse(ctx, client, current, referer)
		if err != nil {
			return result, fmt.Errorf("读取 4399 OAuth 回调失败: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if readErr != nil {
			return result, fmt.Errorf("读取 4399 OAuth 回调失败")
		}
		if response.StatusCode >= 300 && response.StatusCode < 400 {
			next := response.Header.Get("Location")
			if next == "" {
				return result, fmt.Errorf("4399 OAuth 回调跳转缺少地址")
			}
			referer, current = current, next
			continue
		}
		if response.StatusCode != http.StatusOK {
			return result, fmt.Errorf("4399 OAuth 回调返回 HTTP %d", response.StatusCode)
		}
		if err := json.Unmarshal(body, &result); err != nil {
			// The callback may answer with an HTML page; treat it as a failure.
			return result, fmt.Errorf("4399 OAuth 回调响应格式无效")
		}
		if !codeIsSuccess(result.Code) {
			message := strings.TrimSpace(result.Message)
			if message == "" {
				message = "code " + strings.TrimSpace(string(result.Code))
			}
			return result, fmt.Errorf("4399 OAuth 登录失败：%s", message)
		}
		return result, nil
	}
	return result, fmt.Errorf("4399 OAuth 回调跳转过多")
}

func codeIsSuccess(raw json.RawMessage) bool {
	text := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if text == "" {
		return false
	}
	if value, err := strconv.Atoi(text); err == nil {
		return value == 100
	}
	return text == "100"
}

func (s *Service) buildSauth(uid, state string) (map[string]any, error) {
	device, err := randomUpperHex(deviceSize)
	if err != nil {
		return nil, err
	}
	clientSN, err := randomUpperHex(deviceSize)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"aim_info":           `{"aim":"127.0.0.1","country":"CN","tz":"+0800","tzid":""}`,
		"app_channel":        sauthChannel,
		"client_login_sn":    clientSN,
		"deviceid":           device,
		"gameid":             "x19",
		"gas_token":          "",
		"get_access_token":   "1",
		"ip":                 "127.0.0.1",
		"is_unisdk_guest":    0,
		"login_channel":      sauthChannel,
		"platform":           sauthPlatform,
		"realname":           `{"realname_type":"0"}`,
		"sdk_version":        sauthSDKVersion,
		"sdkuid":             uid,
		"sessionid":          state,
		"source_app_channel": sauthChannel,
		"source_platform":    sauthPlatform,
		"step":               "0",
		"step2":              "0",
		"udid":               device,
	}, nil
}

func (s *Service) enrichWithUniSauth(ctx context.Context, client *http.Client, sauth map[string]any) error {
	payload, err := json.Marshal(sauth)
	if err != nil {
		return fmt.Errorf("序列化 4399 会话失败")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.uniSauthURL(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("构建 uni_sauth 请求失败")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", sauthUserAgent)
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("uni_sauth 请求失败: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("读取 uni_sauth 响应失败")
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("uni_sauth 返回 HTTP %d", response.StatusCode)
	}
	var result struct {
		Status          string `json:"status"`
		Code            int    `json:"code"`
		Subcode         int    `json:"subcode"`
		Message         string `json:"message"`
		UnisdkLoginJSON string `json:"unisdk_login_json"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("uni_sauth 响应格式无效")
	}
	if !strings.EqualFold(result.Status, "ok") && result.Code != 200 {
		message := strings.TrimSpace(result.Message)
		if message == "" {
			message = fmt.Sprintf("code %d/%d", result.Code, result.Subcode)
		}
		return fmt.Errorf("uni_sauth 失败：%s", message)
	}
	if result.UnisdkLoginJSON == "" {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(result.UnisdkLoginJSON)
	if err != nil {
		return nil
	}
	var login struct {
		OAuth2 struct {
			AccessToken string `json:"access_token"`
		} `json:"oauth2"`
	}
	if json.Unmarshal(decoded, &login) == nil && login.OAuth2.AccessToken != "" {
		sauth["access_token"] = login.OAuth2.AccessToken
	}
	return nil
}

func (s *Service) get(ctx context.Context, client *http.Client, endpoint string, header http.Header) ([]byte, error) {
	body, _, err := s.getWithType(ctx, client, endpoint, header)
	return body, err
}

func (s *Service) getWithType(ctx context.Context, client *http.Client, endpoint string, header http.Header) ([]byte, string, error) {
	response, err := s.getResponse(ctx, client, endpoint, "")
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "", fmt.Errorf("HTTP %d", response.StatusCode)
	}
	return body, response.Header.Get("Content-Type"), nil
}

func (s *Service) getResponse(ctx context.Context, client *http.Client, endpoint, referer string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败")
	}
	request.Header.Set("User-Agent", s.userAgent())
	request.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	if referer != "" {
		request.Header.Set("Referer", referer)
	}
	return client.Do(request)
}

func textValue(values map[string]any, key string) string {
	value, exists := values[key]
	if !exists || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if math.Trunc(typed) == typed {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case json.Number:
		return typed.String()
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

// encryptPassword reproduces CryptoJS.AES.encrypt(password, oauthPassphrase)
// with the OpenSSL salted format: base64("Salted__" + salt + AES-256-CBC).
func encryptPassword(password string) (string, error) {
	salt := make([]byte, 8)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("生成 4399 密码盐失败")
	}
	key, iv := evpBytesToKey([]byte(oauthPassphrase), salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("初始化 4399 密码加密失败")
	}
	plaintext := []byte(password)
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for index := len(plaintext); index < len(padded); index++ {
		padded[index] = byte(padding)
	}
	encrypted := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, padded)
	out := append([]byte("Salted__"), salt...)
	out = append(out, encrypted...)
	return base64.StdEncoding.EncodeToString(out), nil
}

func evpBytesToKey(passphrase, salt []byte) (key, iv []byte) {
	var derived []byte
	var previous []byte
	for len(derived) < 48 {
		hash := md5.New()
		hash.Write(previous)
		hash.Write(passphrase)
		hash.Write(salt)
		previous = hash.Sum(nil)
		derived = append(derived, previous...)
	}
	return derived[:32], derived[32:48]
}

func randomLowerHex(size int) (string, error) {
	raw := make([]byte, (size+1)/2)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return strings.ToLower(hex.EncodeToString(raw))[:size], nil
}

func randomUpperHex(size int) (string, error) {
	value, err := randomLowerHex(size)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(value), nil
}
