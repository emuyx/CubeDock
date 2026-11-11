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

// Session is the parsed inner sauth object.
type Session struct {
	SDKUID      string
	SessionID   string
	UDID        string
	DeviceID    string
	Platform    string
	AppChannel  string
	AccessToken string
	Raw         map[string]any
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
