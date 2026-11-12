package netease

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/encoding/simplifiedchinese"

	"github.com/emuyx/cubedock/internal/secret"
)

// The mobile email/password login chain is documented in
// docs/internal/MOBILE_PASSWORD_LOGIN_HANDOVER.md. Every constant below was
// captured from the official NetEase mobile client 3.10.16.302017 or observed
// in the captured /devices form; do not change one without a new capture.
const (
	mobileServiceBaseURL = "https://service.mkey.163.com"
	mobileUniSauthURL    = "https://mgbsdk.matrix.netease.com/x19/sdk/uni_sauth"
	mobileGameID         = "aecfrxodyqaaaajp-g-x19"
	mobileJFGameID       = "x19"
	mobileDeviceFile     = "mobile-device.dat"

	mobilePasswordLvl = 3

	mobileOptFields = "nickname,avatar,realname_status,mobile_bind_status,exit_popup_info," +
		"mask_related_mobile,related_login_status,detect_is_new_user"
	mobileMcountAppKey = "EEkEEXLymcNjM42yLY3Bn6AO15aGy4yq"
	mobileGV           = "840301327"
	mobileGVN          = "3.10.5.301327"
	mobileCV           = "a5.16.0"
	mobileSV           = "35"
	mobileAppChannel   = "netease.wyzymnqsd_cps_dev"
	mobilePkgChannel   = "netease"

	// Device registration values observed in the first-login capture. The
	// AccountLogin flow uses the same emulator-class device profile as the
	// captured client; only the identifiers are regenerated per data directory.
	mobileBrand       = "HONOR"
	mobileDeviceName  = "MuMu"
	mobileDeviceType  = "tablet"
	mobileDeviceModel = "SDY-AN00"
	mobileResolution  = "1440*2560"
	mobileSystemName  = "Android"
	mobileSystemVer   = "15"
	// request.d.b in the official client hardcodes 0 here; the URS identifiers
	// come from the SDK's stored device data, not from this flag.
	mobileInitURSDevice = "0"
	mobileCICode        = "2"
	mobileCloudExtra    = "e30="
	mobileSC            = "1"
)

// MobileVerifyRequired is returned when NetEase asks for an interactive
// security check during a mobile password login.
type MobileVerifyRequired struct {
	Message string
	URL     string
}

func (e *MobileVerifyRequired) Error() string {
	if e.URL != "" {
		return e.Message + "（" + e.URL + "）"
	}
	return e.Message
}

type mobileDevice struct {
	ID         string `json:"id"`
	Key        string `json:"key,omitempty"`
	UDID       string `json:"udid"`
	UniqueID   string `json:"unique_id"`
	MAC        string `json:"mac,omitempty"`
	URSUDID    string `json:"urs_udid,omitempty"`
	ExtCI      string `json:"ext_ci,omitempty"`
	DeviceUDID string `json:"device_udid,omitempty"`
}

// MobileLoginService performs the email/password login used by the NetEase
// mobile client. It caches one device identity per data directory so repeated
// logins do not register a new device every time.
type MobileLoginService struct {
	dir string

	HTTP        *http.Client
	BaseURL     string
	UniSauthURL string
	IPLookup    func(context.Context) string

	mu sync.Mutex
}

// NewMobileLoginService returns a service that persists its device identity in
// dir. An empty dir keeps the identity in memory only.
func NewMobileLoginService(dir string) *MobileLoginService {
	return &MobileLoginService{
		dir:         dir,
		HTTP:        &http.Client{Timeout: 30 * time.Second},
		BaseURL:     mobileServiceBaseURL,
		UniSauthURL: mobileUniSauthURL,
		IPLookup:    detectMobilePublicIP,
	}
}

// Login returns the inner mobile_netease sauth object (without the outer
// Cookie wrapper). The password is only used for this request.
func (s *MobileLoginService) Login(ctx context.Context, email, password string) (map[string]any, error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return nil, fmt.Errorf("请填写网易邮箱和密码")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	device, cached, err := s.loadDevice()
	if err != nil {
		return nil, fmt.Errorf("读取手游设备身份失败: %w", err)
	}
	if cached {
		if err := s.preflight(ctx, device, true); err != nil {
			return nil, fmt.Errorf("手游设备初始化失败: %w", err)
		}
	} else {
		device = newMobileDevice()
		if err := s.preflight(ctx, device, false); err != nil {
			return nil, fmt.Errorf("手游设备初始化失败: %w", err)
		}
		device, err = s.registerDevice(ctx, device)
		if err != nil {
			return nil, err
		}
		if err := s.uploadDevice(ctx, device); err != nil {
			return nil, fmt.Errorf("手游设备上传失败: %w", err)
		}
		if err := s.saveDevice(device); err != nil {
			return nil, fmt.Errorf("保存手游设备身份失败: %w", err)
		}
	}
	userToken, userID, err := s.passwordLogin(ctx, device, email, password)
	if err != nil {
		return nil, err
	}
	return s.uniSauth(ctx, device, userID, userToken)
}

func (s *MobileLoginService) loadDevice() (mobileDevice, bool, error) {
	if s.dir == "" {
		return mobileDevice{}, false, nil
	}
	raw, err := secret.Load(filepath.Join(s.dir, mobileDeviceFile))
	if err != nil {
		return mobileDevice{}, false, err
	}
	if raw == "" {
		return mobileDevice{}, false, nil
	}
	var device mobileDevice
	if json.Unmarshal([]byte(raw), &device) != nil ||
		device.ID == "" || device.UDID == "" || device.UniqueID == "" || device.DeviceUDID == "" {
		// Older state from before the preflight/upload fix is not reusable.
		return mobileDevice{}, false, nil
	}
	return device, true, nil
}

func (s *MobileLoginService) saveDevice(device mobileDevice) error {
	if s.dir == "" {
		return nil
	}
	encoded, err := json.Marshal(device)
	if err != nil {
		return fmt.Errorf("序列化手游设备身份失败: %w", err)
	}
	return secret.Save(filepath.Join(s.dir, mobileDeviceFile), string(encoded))
}

func newMobileDevice() mobileDevice {
	// android_id is the registration udid, the transaction prefix, and the
	// source of urs_udid/mac/device_udid. Keep all of them derived from one
	// value so the device identity is internally consistent.
	androidID := randomLowerHex(16)
	return mobileDevice{
		UDID:       androidID,
		UniqueID:   uuid.NewString() + strconv.FormatInt(time.Now().UnixMilli(), 10),
		MAC:        mobileFingerprintMAC(androidID),
		URSUDID:    mobileURSUDID(androidID, mobileDeviceModel),
		ExtCI:      mobileFingerprintSHA256(),
		DeviceUDID: mobileDeviceUDID(androidID),
	}
}

// mobileURSUDID reproduces widget.L.a(Context): SHA-1 of android_id + model,
// printed as a positive BigInteger in lower-case hex (leading zeros omitted).
func mobileURSUDID(androidID, model string) string {
	digest := sha1.Sum([]byte(androidID + model))
	return new(big.Int).SetBytes(digest[:]).Text(16)
}

// mobileFingerprintMAC reproduces K.c(Context) for a valid ANDROID_ID.
func mobileFingerprintMAC(androidID string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte("ANDROID_ID_4_LOGIN:"+androidID)))
}

// mobileDeviceUDID reproduces K.a(K.b(Context)): base64(android_id:unknown:
// unknown), then the client's even/odd +/-1 character shift and trailing tab.
func mobileDeviceUDID(androidID string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(androidID + ":unknown:unknown"))
	chars := []byte(encoded)
	for index := range chars {
		if index%2 == 0 {
			chars[index]--
		} else {
			chars[index]++
		}
	}
	return string(chars) + "\t"
}

// mobileFingerprintSHA256 approximates restore.a.a(Context): a SHA-256 over the
// device characteristics used by the captured emulator profile.
func mobileFingerprintSHA256() string {
	fingerprint := mobileResolution + "|6|8954|" + mobileDeviceModel + "|" +
		mobileBrand + "||arm64-v8a|qcom"
	sum := sha256.Sum256([]byte(fingerprint))
	return hex.EncodeToString(sum[:])
}

func (s *MobileLoginService) preflight(ctx context.Context, device mobileDevice, withID bool) error {
	common := mobileCommonFields(device.UDID)
	configFields := []formField{{Key: "device_udid", Value: device.DeviceUDID}}
	if withID {
		configFields = append(configFields, formField{Key: "device_id", Value: device.ID})
	}
	configFields = append(configFields, common...)
	if err := s.getOrderedForm(ctx, "/mpay/games/"+mobileGameID+"/config", configFields); err != nil {
		return fmt.Errorf("读取手游配置失败: %w", err)
	}
	methodFields := []formField{{Key: "oversea_user", Value: "0"}, {Key: "device_udid", Value: device.DeviceUDID}}
	if withID {
		methodFields = append(methodFields, formField{Key: "device_id", Value: device.ID})
	}
	methodFields = append(methodFields, formField{Key: "country_codes_hash", Value: ""})
	methodFields = append(methodFields, common...)
	if err := s.getOrderedForm(ctx, "/mpay/games/"+mobileGameID+"/login_methods", methodFields); err != nil {
		return fmt.Errorf("读取手游登录方式失败: %w", err)
	}
	if err := s.getOrderedForm(ctx, "/mpay/config/common.json", common); err != nil {
		return fmt.Errorf("读取手游公共配置失败: %w", err)
	}
	return nil
}

func (s *MobileLoginService) registerDevice(ctx context.Context, device mobileDevice) (mobileDevice, error) {
	fields := mobileDeviceFields(device, true)
	fields = append(fields, mobileCommonFields(device.UDID)...)
	var response struct {
		Device struct {
			ID  string `json:"id"`
			Key string `json:"key"`
		} `json:"device"`
	}
	if err := s.postOrderedForm(ctx, "/mpay/games/"+mobileGameID+"/devices", fields, &response); err != nil {
		return mobileDevice{}, fmt.Errorf("手游设备注册失败: %w", err)
	}
	if response.Device.ID == "" {
		return mobileDevice{}, fmt.Errorf("手游设备注册响应缺少 id")
	}
	device.ID = response.Device.ID
	device.Key = response.Device.Key
	return device, nil
}

func (s *MobileLoginService) uploadDevice(ctx context.Context, device mobileDevice) error {
	fields := []formField{{Key: "device_id", Value: device.ID}, {Key: "version", Value: mobileGV}}
	fields = append(fields, mobileDeviceFields(device, false)...)
	fields = append(fields, mobileCommonFields(device.UDID)...)
	if err := s.postOrderedForm(ctx, "/mpay/api/devices/upload", fields, nil); err != nil {
		return err
	}
	return nil
}

// mobileDeviceFields follows the field order captured from the official client.
// The upload endpoint rejects a re-ordered form with HTTP 400/code 1001.
func mobileDeviceFields(device mobileDevice, includeInit bool) []formField {
	fields := []formField{
		{Key: "mac", Value: device.MAC},
		{Key: "urs_udid", Value: device.URSUDID},
	}
	if includeInit {
		fields = append(fields, formField{Key: "init_urs_device", Value: mobileInitURSDevice})
	}
	fields = append(fields,
		formField{Key: "unique_id", Value: device.UniqueID},
		formField{Key: "brand", Value: mobileBrand},
		formField{Key: "device_name", Value: mobileDeviceName},
		formField{Key: "device_type", Value: mobileDeviceType},
		formField{Key: "device_model", Value: mobileDeviceModel},
		formField{Key: "resolution", Value: mobileResolution},
		formField{Key: "system_name", Value: mobileSystemName},
		formField{Key: "system_version", Value: mobileSystemVer},
		formField{Key: "udid", Value: device.UDID},
		formField{Key: "app_channel", Value: mobileAppChannel},
		formField{Key: "oaid", Value: ""},
		formField{Key: "ext_ci", Value: device.ExtCI},
		formField{Key: "ci_code", Value: mobileCICode},
	)
	return fields
}

func mobileCommonFields(udid string) []formField {
	return []formField{
		{Key: "game_id", Value: mobileGameID},
		{Key: "gv", Value: mobileGV},
		{Key: "gvn", Value: mobileGVN},
		{Key: "cv", Value: mobileCV},
		{Key: "sv", Value: mobileSV},
		{Key: "app_type", Value: "games"},
		{Key: "app_mode", Value: "2"},
		{Key: "jf_game_id", Value: mobileJFGameID},
		{Key: "pkg_channel", Value: mobilePkgChannel},
		{Key: "app_channel", Value: mobileAppChannel},
		{Key: "transid", Value: mobileTransactionID(udid)},
		{Key: "mcount_app_key", Value: mobileMcountAppKey},
		{Key: "mcount_transaction_id", Value: mobileTransactionID(udid)},
		{Key: "_cloud_extra_base64", Value: mobileCloudExtra},
		{Key: "sc", Value: mobileSC},
	}
}

func (s *MobileLoginService) passwordLogin(ctx context.Context, device mobileDevice, email, password string) (userToken, userID string, err error) {
	params, err := encryptMobileParams(device.Key, device.UniqueID, email, password)
	if err != nil {
		return "", "", fmt.Errorf("加密手游登录参数失败: %w", err)
	}
	fields := []formField{
		{Key: "opt_fields", Value: mobileOptFields},
		{Key: "params", Value: params},
	}
	fields = append(fields, mobileCommonFields(device.UDID)...)
	query := url.Values{}
	query.Set("un", base64.StdEncoding.EncodeToString([]byte(email)))
	path := "/mpay/games/" + mobileGameID + "/devices/" + url.PathEscape(device.ID) + "/users?" + query.Encode()

	var response struct {
		User      json.RawMessage `json:"user"`
		Code      int             `json:"code"`
		Message   string          `json:"message"`
		Reason    string          `json:"reason"`
		VerifyURL string          `json:"verify_url"`
		Verify    string          `json:"verify"`
	}
	if err := s.postOrderedForm(ctx, path, fields, &response); err != nil {
		return "", "", fmt.Errorf("手游密码登录失败: %w", err)
	}
	user, err := decodeMobileUser(response.User)
	if err != nil || user["token"] == "" || user["id"] == "" {
		verifyURL := firstNonEmpty(
			response.VerifyURL,
			response.Verify,
			extractMobileVerifyURL(response.Message),
			extractMobileVerifyURL(response.Reason),
		)
		if response.Code == 1351 || verifyURL != "" {
			message := firstNonEmpty(response.Message, response.Reason, "网易要求安全验证")
			return "", "", &MobileVerifyRequired{Message: message, URL: verifyURL}
		}
		message := firstNonEmpty(response.Message, response.Reason,
			fmt.Sprintf("登录失败 (code: %d)", response.Code))
		return "", "", fmt.Errorf("手游密码登录失败: %s", message)
	}
	return user["token"], user["id"], nil
}

func (s *MobileLoginService) uniSauth(ctx context.Context, device mobileDevice, userID, userToken string) (map[string]any, error) {
	profile, err := profileFor(AuthMobileNetEase)
	if err != nil {
		return nil, err
	}
	ip := "127.0.0.1"
	if s.IPLookup != nil {
		if value := strings.TrimSpace(s.IPLookup(ctx)); value != "" {
			ip = value
		}
	}
	aimInfo := mobileAimInfo(ip)
	sdkLog := mobileSDKLog(device)
	clientLoginSN, err := randomUpperHex(32)
	if err != nil {
		return nil, err
	}
	request := map[string]any{
		"gameid":             "x19",
		"login_channel":      profile.LoginChannel,
		"app_channel":        profile.AppChannel,
		"platform":           profile.Platform,
		"sdkuid":             userID,
		"udid":               device.UDID,
		"sessionid":          userToken,
		"sdk_version":        profile.SDKVersion,
		"is_unisdk_guest":    0,
		"ip":                 ip,
		"aim_info":           aimInfo,
		"source_app_channel": profile.SourceAppChannel,
		"source_platform":    profile.SourcePlatform,
		"get_access_token":   "1",
		"deviceid":           device.ID,
		"client_login_sn":    clientLoginSN,
		"step":               "0",
		"step2":              "0",
		"hostid":             0,
		"sdklog":             sdkLog,
	}
	var response struct {
		Status          string `json:"status"`
		Code            int    `json:"code"`
		UnisdkLoginJSON string `json:"unisdk_login_json"`
	}
	if err := s.postJSON(ctx, s.UniSauthURL, request, &response); err != nil {
		return nil, fmt.Errorf("手游 uni_sauth 请求失败: %w", err)
	}
	if !strings.EqualFold(response.Status, "ok") && response.Code != 200 {
		return nil, fmt.Errorf("uni_sauth 失败 (code: %d)", response.Code)
	}
	decoded, err := decodeBase64JSON(response.UnisdkLoginJSON)
	if err != nil {
		return nil, fmt.Errorf("解析 uni_sauth 响应失败: %w", err)
	}
	var loginJSON struct {
		OAuth2 struct {
			AccessToken string `json:"access_token"`
		} `json:"oauth2"`
	}
	if err := json.Unmarshal(decoded, &loginJSON); err != nil {
		return nil, fmt.Errorf("解析 uni_sauth 响应失败: %w", err)
	}
	if loginJSON.OAuth2.AccessToken == "" {
		return nil, fmt.Errorf("uni_sauth 没有返回 access_token")
	}
	return map[string]any{
		"access_token":       loginJSON.OAuth2.AccessToken,
		"aim_info":           aimInfo,
		"app_channel":        profile.AppChannel,
		"client_login_sn":    clientLoginSN,
		"deviceid":           device.ID,
		"gameid":             "x19",
		"get_access_token":   "1",
		"ip":                 ip,
		"login_channel":      profile.LoginChannel,
		"platform":           profile.Platform,
		"sdk_version":        profile.SDKVersion,
		"sdkuid":             userID,
		"sessionid":          userToken,
		"source_app_channel": profile.SourceAppChannel,
		"source_platform":    profile.SourcePlatform,
		"udid":               device.UDID,
	}, nil
}

type formField struct {
	Key   string
	Value string
}

// encodeForm preserves the captured field order. NetEase's device upload
// endpoint rejects an alphabetically sorted form with code 1001.
func encodeForm(fields []formField) string {
	var builder strings.Builder
	for index, field := range fields {
		if index > 0 {
			builder.WriteByte('&')
		}
		builder.WriteString(url.QueryEscape(field.Key))
		builder.WriteByte('=')
		builder.WriteString(url.QueryEscape(field.Value))
	}
	return builder.String()
}

func (s *MobileLoginService) postOrderedForm(ctx context.Context, path string, fields []formField, out any) error {
	body := strings.NewReader(encodeForm(fields))
	return s.do(ctx, http.MethodPost, strings.TrimRight(s.BaseURL, "/")+path,
		"application/x-www-form-urlencoded", mobileMkeyUserAgent(), body, out)
}

func (s *MobileLoginService) postJSON(ctx context.Context, endpoint string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodPost, endpoint, "application/json", mobileDalvikUserAgent, bytes.NewReader(body), out)
}

func (s *MobileLoginService) getOrderedForm(ctx context.Context, path string, fields []formField) error {
	endpoint := strings.TrimRight(s.BaseURL, "/") + path
	if len(fields) > 0 {
		endpoint += "?" + encodeForm(fields)
	}
	return s.do(ctx, http.MethodGet, endpoint, "application/x-www-form-urlencoded", mobileMkeyUserAgent(), nil, nil)
}

func (s *MobileLoginService) do(ctx context.Context, method, endpoint, contentType, userAgent string, body io.Reader, out any) error {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if userAgent != "" {
		request.Header.Set("User-Agent", userAgent)
	}
	request.Header.Set("Accept", "*/*")
	request.Header.Set("Accept-Charset", "UTF-8")
	request.Header.Set("Accept-Language", "zh-cn")
	client := s.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("网易接口返回 HTTP %d: %s", response.StatusCode, truncateText(string(raw), 200))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("网易接口响应格式无效: %w", err)
	}
	return nil
}

// encryptMobileParams produces hex(AES/ECB/PKCS7(JSON)) for the mobile password
// login. Unlike the interop chain, the password is sent as plaintext inside the
// encrypted envelope, and password_level is fixed at 3.
type mobileParamsPayload struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	UniqueID      string `json:"unique_id"`
	PasswordLevel int    `json:"password_level"`
}

func encryptMobileParams(keyHex, uniqueID, email, password string) (string, error) {
	// The captured client sends hex(md5(password.getBytes("GBK"))) as 32
	// lowercase hex characters, not the plaintext password. The JSON key order
	// also matches the capture. The AES key is the device key returned by
	// /devices, not a universal constant.
	key, err := hex.DecodeString(keyHex)
	if err != nil || len(key) != 16 {
		return "", fmt.Errorf("设备密钥无效")
	}
	passwordHash, err := mobilePasswordHash(password)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(mobileParamsPayload{
		Username:      email,
		Password:      passwordHash,
		UniqueID:      uniqueID,
		PasswordLevel: mobilePasswordLvl,
	})
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	padding := aes.BlockSize - len(payload)%aes.BlockSize
	payload = append(payload, bytes.Repeat([]byte{byte(padding)}, padding)...)
	out := make([]byte, len(payload))
	for start := 0; start < len(payload); start += aes.BlockSize {
		block.Encrypt(out[start:start+aes.BlockSize], payload[start:start+aes.BlockSize])
	}
	return hex.EncodeToString(out), nil
}

// mobilePasswordHash reproduces com.netease.mpay.widget.M.c: the client takes
// String.getBytes("GBK"), MD5, then lower-case hex. GBK matters for passwords
// containing non-ASCII characters.
func mobilePasswordHash(password string) (string, error) {
	encoded, err := encodeGBK(password)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", md5.Sum(encoded)), nil
}

func encodeGBK(value string) ([]byte, error) {
	encoder := simplifiedchinese.GBK.NewEncoder()
	if encoded, err := encoder.Bytes([]byte(value)); err == nil {
		return encoded, nil
	}
	// Java's String.getBytes("GBK") replaces unmappable characters with '?'.
	var out bytes.Buffer
	for _, r := range value {
		encoded, err := encoder.Bytes([]byte(string(r)))
		if err != nil {
			out.WriteByte('?')
			continue
		}
		out.Write(encoded)
	}
	return out.Bytes(), nil
}

func decodeMobileUser(raw json.RawMessage) (map[string]string, error) {
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
		switch typed := value.(type) {
		case string:
			out[key] = typed
		case float64:
			out[key] = fmt.Sprint(typed)
		}
	}
	return out, nil
}

func decodeBase64JSON(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("响应为空")
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err == nil && json.Valid(decoded) {
		return decoded, nil
	}
	if json.Valid([]byte(value)) {
		return []byte(value), nil
	}
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

var mobileVerifyURLPattern = regexp.MustCompile(`https?://[^\s"'<>，。；：！？）】]+`)

func extractMobileVerifyURL(text string) string {
	match := mobileVerifyURLPattern.FindString(text)
	return strings.TrimRight(match, `).,;:!?，。；：！？）】"'`)
}

func mobileAimInfo(ip string) string {
	raw, err := json.Marshal(map[string]any{
		"aim":            ip,
		"country":        "CN",
		"tz":             "+0800",
		"tzid":           "Asia/Shanghai",
		"celluar_ip":     "",
		"operator":       "460000",
		"is_vpn_enabled": false,
	})
	if err != nil {
		return `{"aim":"127.0.0.1","country":"CN","tz":"+0800","tzid":"Asia/Shanghai"}`
	}
	return string(raw)
}

func mobileSDKLog(device mobileDevice) string {
	raw, err := json.Marshal(map[string]any{
		"device_model": mobileDeviceModel,
		"os_name":      "android",
		"os_ver":       mobileSystemVer,
		"udid":         device.UDID,
		"app_ver":      mobileGV,
		"imei":         "",
		"area_code":    "CN",
		"is_emulator":  1,
		"is_root":      0,
		"oaid":         "",
	})
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func mobileTransactionID(udid string) string {
	// Captured transactions all share the device registration udid as their
	// 16-hex prefix, with a fresh timestamp/counter for each request.
	return udid + "_" + strconv.FormatInt(time.Now().UnixMilli(), 10) +
		"_" + strconv.Itoa(randomInt(100000000, 999999999))
}

const mobileDalvikUserAgent = "Dalvik/2.1.0 (Linux; U; Android 15; SDY-AN00 Build/ba29e06.0)"

func mobileMkeyUserAgent() string {
	return fmt.Sprintf("com.netease.x19/%s NeteaseMobileGame/%s (%s;%s)",
		mobileGV, mobileCV, mobileDeviceModel, mobileSV)
}

func randomLowerHex(size int) string {
	raw := make([]byte, (size+1)/2)
	if _, err := rand.Read(raw); err != nil {
		// crypto/rand should not fail; fall back to a zero-filled value so the
		// caller still gets a well-formed identifier.
		raw = make([]byte, (size+1)/2)
	}
	return strings.ToLower(hex.EncodeToString(raw))[:size]
}

func randomInt(min, max int) int {
	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		return min
	}
	value := int(raw[0])<<24 | int(raw[1])<<16 | int(raw[2])<<8 | int(raw[3])
	if value < 0 {
		value = -value
	}
	return min + value%(max-min+1)
}

func detectMobilePublicIP(ctx context.Context) string {
	for _, endpoint := range []string{"https://api.ipify.org", "https://ifconfig.me/ip"} {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}
		response, err := (&http.Client{Timeout: 6 * time.Second}).Do(request)
		if err != nil {
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 64))
		response.Body.Close()
		ip := strings.TrimSpace(string(raw))
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	return "127.0.0.1"
}

func truncateText(value string, size int) string {
	if len(value) <= size {
		return value
	}
	return value[:size]
}
