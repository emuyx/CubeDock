// Package netease implements CubeDock's own NetEase account and TanLobby
// authentication path. It is intentionally independent from the historical
// AGPL authentication modules.
//
// It is derived from an internal protocol specification and the constants
// already verified in docs/rewrite/VERIFIED_STATUS.md. The PE envelope, user
// detail and transfer-node flows have been verified against a real account.
package netease

// ReleaseInfo is the G79 service discovery document.
type ReleaseInfo struct {
	CoreServerURL            string `json:"CoreServerUrl"`
	AuthServerURL            string `json:"AuthServerUrl"`
	APIGatewayURL            string `json:"ApiGatewayUrl"`
	TransferServerURL        string `json:"TransferServerUrl"`
	TransferServerNewHTTPURL string `json:"TransferServerNewHttpUrl"`
	ChatServerURL            string `json:"ChatServerUrl"`
	LinkServerURL            string `json:"LinkServerUrl"`
}

// X19Release is the subset of the x19 discovery document used by the interop
// client after PE authentication.
type X19Release struct {
	APIGatewayGrayURL string `json:"ApiGatewayGrayUrl"`
}

// AuthMethod identifies the client/channel whose session was imported.
// Several methods coexist in one installation; old accounts default to interop.
type AuthMethod string

const (
	AuthInterop       AuthMethod = "interop"        // 基岩互通版 / FeverGames PC
	AuthMobileNetEase AuthMethod = "mobile_netease" // 网易手游
	AuthMobile4399    AuthMethod = "mobile_4399"    // 4399 手游
)

// Session is the parsed inner sauth object. Method is inferred from the
// platform/channel fields unless the caller explicitly selects a provider.
type Session struct {
	Method           AuthMethod
	SDKUID           string
	SessionID        string
	UDID             string
	DeviceID         string
	Platform         string
	AppChannel       string
	LoginChannel     string
	SourcePlatform   string
	SourceAppChannel string
	GameID           string
	SDKVersion       string
	ClientLoginSN    string
	AccessToken      string
	Raw              map[string]any
}

// Credentials are the account identity returned by PE authentication.
type Credentials struct {
	UserID  string
	Token   string
	Seed    string
	Session Session
}

// Profile is the public game identity behind a session.
type Profile struct {
	Name  string
	UID   string
	Level int
}

// TransferServer is one entry in NetEase's transfer-node list.
type TransferServer struct {
	ID            int
	IP            string
	ServerType    string
	SignalWebPort int
	Ports         []int
	Status        int
}

// RoomAuth is the transfer credential bundle consumed by room.Open.
type RoomAuth struct {
	UserID           uint32
	PlayerName       string
	RaknetAddress    string
	SignalingAddress string
	RaknetRand       []byte
	RaknetAESRand    []byte
	SignalingSeed    []byte
	SignalingTicket  []byte
	EncryptKey       []byte
	DecryptKey       []byte
}
