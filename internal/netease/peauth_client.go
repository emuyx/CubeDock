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
	seed := uuid.NewString()
	loginSN, err := randomUpperHex(32)
	if err != nil {
		return Credentials{}, err
	}
	aimInfo := textField(session.Raw, "aim_info")
	if aimInfo == "" {
		aimInfo = `{"aim":"127.0.0.1","country":"CN","tz":"+0800","tzid":""}`
	}
	ip := textField(session.Raw, "ip")
	if ip == "" {
		ip = "127.0.0.1"
	}
	gameID := textField(session.Raw, "gameid")
	if gameID == "" {
		gameID = "x19"
	}
	sauth := map[string]any{
		"access_token":     session.AccessToken,
		"aim_info":         aimInfo,
		"app_channel":      "a50_sdk_cn",
		"client_login_sn":  loginSN,
		"deviceid":         session.DeviceID,
		"gameid":           gameID,
		"gas_token":        textField(session.Raw, "gas_token"),
		"get_access_token": "1",
		"ip":               ip,
		"login_channel":    "netease",
		"platform":         "pc",
		"sdk_version":      "4.16.0",
		"sdkuid":           session.SDKUID,
		"sessionid":        session.SessionID,
		"source_platform":  "pc",
		"tdid":             textField(session.Raw, "tdid"),
		"udid":             session.UDID,
	}
	saData, err := json.Marshal(map[string]any{
		"app_channel":   "a50_sdk_cn",
		"app_ver":       interopPatchVersion,
		"core_num":      "8",
		"cpu_digit":     "0",
		"cpu_hz":        "",
		"cpu_name":      "Intel(R) Core(TM) i7-8700 CPU @ 3.20GHz",
		"device_height": "1080",
		"device_model":  "Win32",
		"device_width":  "1920",
		"disk":          `Data_disk_name: SCSI\Disk&Ven_NVMe&Prod_Generic`,
		"emulator":      "0",
		"first_udid":    session.UDID,
		"is_guest":      "0",
		"launcher_type": "PE_C++",
		"mac_addr":      "02:00:00:00:00:01",
		"network":       "",
		"os_name":       "windows",
		"os_ver":        systemVersion(),
		"ram":           "8589934592",
		"rom":           "",
		"root":          false,
		"sdk_ver":       "4.16.0",
		"start_type":    "default",
		"tdid":          "",
		"udid":          session.UDID,
	})
	if err != nil {
		return Credentials{}, fmt.Errorf("序列化设备信息失败: %w", err)
	}
	message := interopEngineVersion + interopLibraryHash + interopPatchVersion + interopPatchHash + interopSignHash + seed
	sign, err := peAuthSign(message, interopSignSP, interopSignTR)
	if err != nil {
		return Credentials{}, err
	}
	payload, err := json.Marshal(peAuthRequest{
		EngineVersion: interopEngineVersion,
		ExtraParam:    "extra",
		Message:       message,
		PatchVersion:  interopPatchVersion,
		PayChannel:    "netease",
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
