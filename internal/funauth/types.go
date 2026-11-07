package funauth

type TanLobbyLoginParams struct {
	RoomID string
}

type TanLobbyLoginResult struct {
	RoomOwnerID            uint32
	UserUniqueID           uint32
	UserPlayerName         string
	BotLevel               int
	BotComponent           map[string]*int
	RaknetServerAddress    string
	RoomModDisplayName     []string
	RoomModDownloadURL     []string
	RoomModEncryptKey      [][]byte
	SignalingServerAddress string

	RaknetRand      []byte
	RaknetAESRand   []byte
	EncryptKeyBytes []byte
	DecryptKeyBytes []byte

	SignalingSeed   []byte
	SignalingTicket []byte
}

type TanLobbyCreateResult struct {
	UserUniqueID           uint32
	UserPlayerName         string
	RaknetServerAddress    string
	RaknetRand             []byte
	RaknetAESRand          []byte
	EncryptKeyBytes        []byte
	DecryptKeyBytes        []byte
	SignalingServerAddress string
	SignalingSeed          []byte
	SignalingTicket        []byte
}
