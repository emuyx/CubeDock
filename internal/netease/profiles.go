package netease

import (
	"fmt"
	"strings"
)

// AuthMethodLabel returns the user-facing label for an auth method.
func AuthMethodLabel(method AuthMethod) string {
	switch AuthMethod(strings.TrimSpace(string(method))) {
	case AuthMobileNetEase:
		return "网易手游"
	case AuthMobile4399:
		return "4399 手游"
	default:
		return "基岩互通版"
	}
}

// ParseAuthMethod normalises a console/API supplied method. Empty input keeps
// backwards compatibility and means interop.
func ParseAuthMethod(value string) (AuthMethod, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "interop", "基岩互通版":
		return AuthInterop, nil
	case "mobile", "mobile_netease", "网易手游":
		return AuthMobileNetEase, nil
	case "mobile_4399", "4399", "4399 手游":
		return AuthMobile4399, nil
	default:
		return "", fmt.Errorf("未知的登录方式 %q", value)
	}
}

// DeviceProfile is the device/sa_data block sent with PE authentication.
type DeviceProfile struct {
	AppVer       string
	CoreNum      string
	CPUDigit     string
	CPUHz        string
	CPUName      string
	DeviceHeight string
	DeviceModel  string
	DeviceWidth  string
	Disk         string
	Emulator     string
	IsGuest      string
	LauncherType string
	MACAddr      string
	Network      string
	OSName       string
	OSVer        string
	RAM          string
	ROM          string
	Root         bool
	StartType    string
}

// PEAuthProfile contains every platform/channel constant used by one PE
// authentication provider. Interop keeps the verified v3.10.5 constants.
type PEAuthProfile struct {
	Method           AuthMethod
	EngineVersion    string
	PatchVersion     string
	LibraryHash      string
	PatchHash        string
	SignHash         string
	SignSP           int
	SignTR           int
	Platform         string
	AppChannel       string
	LoginChannel     string
	SourcePlatform   string
	SourceAppChannel string
	SDKVersion       string
	PayChannel       string
	Device           DeviceProfile
}

var interopPEAuthProfile = PEAuthProfile{
	Method:           AuthInterop,
	EngineVersion:    interopEngineVersion,
	PatchVersion:     interopPatchVersion,
	LibraryHash:      interopLibraryHash,
	PatchHash:        interopPatchHash,
	SignHash:         interopSignHash,
	SignSP:           interopSignSP,
	SignTR:           interopSignTR,
	Platform:         "pc",
	AppChannel:       "a50_sdk_cn",
	LoginChannel:     "netease",
	SourcePlatform:   "pc",
	SourceAppChannel: "a50_sdk_cn",
	SDKVersion:       "4.16.0",
	PayChannel:       "netease",
	Device: DeviceProfile{
		AppVer:       interopPatchVersion,
		CoreNum:      "8",
		CPUDigit:     "0",
		CPUHz:        "",
		CPUName:      "Intel(R) Core(TM) i7-8700 CPU @ 3.20GHz",
		DeviceHeight: "1080",
		DeviceModel:  "Win32",
		DeviceWidth:  "1920",
		Disk:         `Data_disk_name: SCSI\Disk&Ven_NVMe&Prod_Generic`,
		Emulator:     "0",
		IsGuest:      "0",
		LauncherType: "PE_C++",
		MACAddr:      "02:00:00:00:00:01",
		Network:      "",
		OSName:       "windows",
		OSVer:        "", // filled with the host version
		RAM:          "8589934592",
		ROM:          "",
		Root:         false,
		StartType:    "default",
	},
}

// Captured from the official NetEase mobile client 3.10.16.302017. The sign
// parameters remain (2, 8); only the engine/patch/hash constants differ.
var mobileNetEasePEAuthProfile = PEAuthProfile{
	Method:           AuthMobileNetEase,
	EngineVersion:    "3.10.5.301327",
	PatchVersion:     "3.10.16.302017",
	LibraryHash:      "393dfdf6f78831f80a7e6aa305b21227",
	PatchHash:        "741daafcbdbee57ee1526cee9a8c4e14",
	SignHash:         interopSignHash,
	SignSP:           interopSignSP,
	SignTR:           interopSignTR,
	Platform:         "ad",
	AppChannel:       "netease.wyzymnqsd_cps_dev",
	LoginChannel:     "netease",
	SourcePlatform:   "ad",
	SourceAppChannel: "netease.wyzymnqsd_cps_dev",
	SDKVersion:       "5.16.0",
	PayChannel:       "dashen_cloudgame",
	Device: DeviceProfile{
		AppVer:       "3.10.16.302017",
		CoreNum:      "6",
		CPUDigit:     "64",
		CPUHz:        "3200000",
		CPUName:      "placeholder",
		DeviceHeight: "2560",
		DeviceModel:  "HONOR SDY-AN00",
		DeviceWidth:  "1440",
		Disk:         "",
		Emulator:     "0",
		IsGuest:      "0",
		LauncherType: "PE_C++",
		MACAddr:      "02:00:00:00:00:00",
		Network:      "CHANNEL_UNKNOW",
		OSName:       "android",
		OSVer:        "15",
		RAM:          "9389064192",
		ROM:          "135192317952",
		Root:         false,
		StartType:    "default",
	},
}

func profileFor(method AuthMethod) (PEAuthProfile, error) {
	switch method {
	case AuthInterop:
		return interopPEAuthProfile, nil
	case AuthMobileNetEase:
		return mobileNetEasePEAuthProfile, nil
	case AuthMobile4399:
		return PEAuthProfile{}, fmt.Errorf("4399 手游认证尚未配置：需要抓包确认 app_channel / PE 常量")
	default:
		return PEAuthProfile{}, fmt.Errorf("未知的登录方式 %q", method)
	}
}
