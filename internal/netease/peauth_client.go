package netease

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
)

type peAuthRequest struct {
	EngineVersion string         `json:"engine_version"`
	ExtraParam    string         `json:"extra_param"`
	Message       string         `json:"message"`
	PatchVersion  string         `json:"patch_version"`
	PayChannel    string         `json:"pay_channel"`
	SaData        string         `json:"sa_data"`
	SauthJSON     map[string]any `json:"sauth_json"`
	Seed          string         `json:"seed"`
	Sign          string         `json:"sign"`
}

type peAuthResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Entity  struct {
		EntityID string `json:"entity_id"`
		Token    string `json:"token"`
		Seed     string `json:"seed"`
		Sead     string `json:"sead"`
	} `json:"entity"`
}

func (c *Client) performPEAuth(ctx context.Context, session Session) (Credentials, error) {
	profile, err := profileFor(session.Method)
	if err != nil {
		return Credentials{}, err
	}
	seed := uuid.NewString()
	loginSN := strings.TrimSpace(session.ClientLoginSN)
	if loginSN == "" {
		loginSN, err = randomUpperHex(32)
		if err != nil {
			return Credentials{}, err
		}
	}
	aimInfo := textField(session.Raw, "aim_info")
	if aimInfo == "" {
		aimInfo = `{"aim":"127.0.0.1","country":"CN","tz":"+0800","tzid":""}`
	}
	ip := textField(session.Raw, "ip")
	if ip == "" {
		ip = "127.0.0.1"
	}
	gameID := firstNonEmpty(session.GameID, textField(session.Raw, "gameid"), "x19")
	sauth := buildSauth(session, profile, loginSN, aimInfo, ip, gameID)
	saData, err := json.Marshal(buildSaData(session, profile))
	if err != nil {
		return Credentials{}, fmt.Errorf("序列化设备信息失败: %w", err)
	}
	message := profile.EngineVersion + profile.LibraryHash + profile.PatchVersion + profile.PatchHash + profile.SignHash + seed
	sign, err := peAuthSign(message, profile.SignSP, profile.SignTR)
	if err != nil {
		return Credentials{}, err
	}
	payload, err := json.Marshal(peAuthRequest{
		EngineVersion: profile.EngineVersion,
		ExtraParam:    "extra",
		Message:       message,
		PatchVersion:  profile.PatchVersion,
		PayChannel:    profile.PayChannel,
		SaData:        string(saData),
		SauthJSON:     sauth,
		Seed:          seed,
		Sign:          sign,
	})
	if err != nil {
		return Credentials{}, fmt.Errorf("序列化认证数据失败: %w", err)
	}
	encrypted, err := encryptEnvelope(payload, rand.Reader)
	if err != nil {
		return Credentials{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.g79.CoreServerURL+"/pe-authentication", strings.NewReader(encrypted))
	if err != nil {
		return Credentials{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "libhttpclient/1.0.0.0")
	request.Header.Set("Accept-Encoding", "gzip")
	response, err := c.HTTP.Do(request)
	if err != nil {
		return Credentials{}, fmt.Errorf("PE 认证请求失败: %w", err)
	}
	defer response.Body.Close()
	body, err := readHTTPBody(response)
	if err != nil {
		return Credentials{}, err
	}
	if response.StatusCode != http.StatusOK {
		return Credentials{}, fmt.Errorf("PE 认证返回 HTTP %d", response.StatusCode)
	}
	plain, err := decryptEnvelope(strings.TrimSpace(string(body)))
	if err != nil {
		return Credentials{}, err
	}
	valid := extractJSONObject(plain)
	var result peAuthResponse
	if err := json.Unmarshal(valid, &result); err != nil {
		return Credentials{}, fmt.Errorf("解析 PE 认证响应失败: %w", err)
	}
	if result.Code != 0 {
		return Credentials{}, fmt.Errorf("PE 认证失败 (code %d): %s", result.Code, result.Message)
	}
	if result.Entity.EntityID == "" || result.Entity.Token == "" {
		return Credentials{}, fmt.Errorf("PE 认证返回值不完整")
	}
	authSeed := result.Entity.Seed
	if authSeed == "" {
		authSeed = result.Entity.Sead
	}
	return Credentials{
		UserID:  result.Entity.EntityID,
		Token:   result.Entity.Token,
		Seed:    authSeed,
		Session: session,
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func buildSauth(session Session, profile PEAuthProfile, loginSN, aimInfo, ip, gameID string) map[string]any {
	if profile.Method == AuthInterop {
		// Keep the verified interop shape byte-for-byte; adding mobile-only
		// fields here would change the wire payload for existing accounts.
		return map[string]any{
			"access_token":     session.AccessToken,
			"aim_info":         aimInfo,
			"app_channel":      profile.AppChannel,
			"client_login_sn":  loginSN,
			"deviceid":         session.DeviceID,
			"gameid":           gameID,
			"gas_token":        textField(session.Raw, "gas_token"),
			"get_access_token": "1",
			"ip":               ip,
			"login_channel":    profile.LoginChannel,
			"platform":         profile.Platform,
			"sdk_version":      profile.SDKVersion,
			"sdkuid":           session.SDKUID,
			"sessionid":        session.SessionID,
			"source_platform":  profile.SourcePlatform,
			"tdid":             textField(session.Raw, "tdid"),
			"udid":             session.UDID,
		}
	}
	return map[string]any{
		"access_token":       session.AccessToken,
		"aim_info":           aimInfo,
		"app_channel":        firstNonEmpty(session.AppChannel, profile.AppChannel),
		"client_login_sn":    loginSN,
		"deviceid":           session.DeviceID,
		"gameid":             firstNonEmpty(session.GameID, gameID),
		"gas_token":          textField(session.Raw, "gas_token"),
		"get_access_token":   "1",
		"ip":                 ip,
		"is_unisdk_guest":    0,
		"login_channel":      firstNonEmpty(session.LoginChannel, profile.LoginChannel),
		"platform":           firstNonEmpty(session.Platform, profile.Platform),
		"sdk_version":        firstNonEmpty(session.SDKVersion, profile.SDKVersion),
		"sdkuid":             session.SDKUID,
		"sessionid":          session.SessionID,
		"source_app_channel": firstNonEmpty(session.SourceAppChannel, profile.SourceAppChannel),
		"source_platform":    firstNonEmpty(session.SourcePlatform, profile.SourcePlatform),
		"step":               "0",
		"step2":              "0",
		"tdid":               textField(session.Raw, "tdid"),
		"udid":               session.UDID,
	}
}

func buildSaData(session Session, profile PEAuthProfile) map[string]any {
	d := profile.Device
	if profile.Method == AuthInterop {
		return map[string]any{
			"app_channel":   profile.AppChannel,
			"app_ver":       profile.PatchVersion,
			"core_num":      d.CoreNum,
			"cpu_digit":     d.CPUDigit,
			"cpu_hz":        d.CPUHz,
			"cpu_name":      d.CPUName,
			"device_height": d.DeviceHeight,
			"device_model":  d.DeviceModel,
			"device_width":  d.DeviceWidth,
			"disk":          d.Disk,
			"emulator":      d.Emulator,
			"first_udid":    session.UDID,
			"is_guest":      d.IsGuest,
			"launcher_type": d.LauncherType,
			"mac_addr":      d.MACAddr,
			"network":       d.Network,
			"os_name":       d.OSName,
			"os_ver":        systemVersion(),
			"ram":           d.RAM,
			"rom":           d.ROM,
			"root":          d.Root,
			"sdk_ver":       profile.SDKVersion,
			"start_type":    d.StartType,
			"tdid":          "",
			"udid":          session.UDID,
		}
	}
	return map[string]any{
		"app_channel":   firstNonEmpty(session.AppChannel, profile.AppChannel),
		"app_ver":       d.AppVer,
		"core_num":      d.CoreNum,
		"cpu_digit":     d.CPUDigit,
		"cpu_hz":        d.CPUHz,
		"cpu_name":      d.CPUName,
		"device_height": d.DeviceHeight,
		"device_model":  d.DeviceModel,
		"device_width":  d.DeviceWidth,
		"disk":          d.Disk,
		"emulator":      0,
		"first_udid":    session.UDID,
		"is_guest":      0,
		"launcher_type": d.LauncherType,
		"mac_addr":      d.MACAddr,
		"network":       d.Network,
		"os_name":       d.OSName,
		"os_ver":        d.OSVer,
		"ram":           d.RAM,
		"rom":           d.ROM,
		"root":          d.Root,
		"sdk_ver":       firstNonEmpty(session.SDKVersion, profile.SDKVersion),
		"start_type":    d.StartType,
		"tdid":          textField(session.Raw, "tdid"),
		"udid":          session.UDID,
	}
}

func readHTTPBody(response *http.Response) ([]byte, error) {
	var reader io.Reader = response.Body
	if strings.EqualFold(response.Header.Get("Content-Encoding"), "gzip") {
		inflater, err := gzip.NewReader(response.Body)
		if err != nil {
			return nil, fmt.Errorf("解压响应失败: %w", err)
		}
		defer inflater.Close()
		reader = inflater
	}
	body, err := io.ReadAll(io.LimitReader(reader, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	return body, nil
}

func extractJSONObject(data []byte) []byte {
	start := -1
	end := -1
	for index, value := range data {
		if value == '{' && start < 0 {
			start = index
		}
		if value == '}' {
			end = index + 1
		}
	}
	if start < 0 || end <= start {
		return data
	}
	return data[start:end]
}

func randomUpperHex(size int) (string, error) {
	raw := make([]byte, (size+1)/2)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("生成随机串失败: %w", err)
	}
	return strings.ToUpper(hex.EncodeToString(raw))[:size], nil
}

func systemVersion() string {
	if value := strings.TrimSpace(os.Getenv("CUBEDOCK_INTEROP_SYSTEM_VERSION")); value != "" {
		return value
	}
	if value := hostSystemVersion(); value != "" {
		return value
	}
	return "10.0.26200"
}
