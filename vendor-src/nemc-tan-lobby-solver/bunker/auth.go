package bunker

import "fmt"

// PhoenixSkinInfo is the bot skin returned by the TanLobby API.
type PhoenixSkinInfo struct {
	ItemID          string `json:"entity_id"`
	SkinDownloadURL string `json:"res_url"`
	SkinIsSlim      bool   `json:"is_slim"`
}

// TanLobbyLoginRequest joins an existing room.
type TanLobbyLoginRequest struct {
	FBToken            string `json:"login_token"`
	ProvidedPEAuthData string `json:"provided_pe_auth_data"`
	ProvidedSaAuthData string `json:"provided_sa_auth_data"`
	RoomID             string `json:"room_id"`
}

// TanLobbyLoginResponse is returned when joining a room.
type TanLobbyLoginResponse struct {
	Success   bool   `json:"success"`
	ErrorInfo string `json:"error_info"`

	UserUniqueID   uint32          `json:"user_unique_id"`
	UserPlayerName string          `json:"user_player_name"`
	BotLevel       int             `json:"growth_level"`
	BotSkin        PhoenixSkinInfo `json:"skin_info"`
	BotComponent   map[string]*int `json:"outfit_info,omitempty"`

	RoomOwnerID        uint32   `json:"room_owner_id"`
	RoomModDisplayName []string `json:"room_mod_display_name"`
	RoomModDownloadURL []string `json:"room_mod_download_url"`
	RoomModEncryptKey  [][]byte `json:"room_mod_encrypt_key"`

	RaknetServerAddress string `json:"raknet_server_address"`
	RaknetRand          []byte `json:"raknet_rand"`
	RaknetAESRand       []byte `json:"raknet_aes_rand"`
	EncryptKeyBytes     []byte `json:"encrypt_key_bytes"`
	DecryptKeyBytes     []byte `json:"decrypt_key_bytes"`

	SignalingServerAddress string `json:"signaling_server_address"`
	SignalingSeed          []byte `json:"signaling_seed"`
	SignalingTicket        []byte `json:"signaling_ticket"`
}

// Auth joins the room with the client's session.
func (c *Client) Auth(roomID string) (TanLobbyLoginResponse, error) {
	var response TanLobbyLoginResponse
	err := c.post("/api/phoenix/tan_lobby_login", TanLobbyLoginRequest{
		FBToken:            c.FBToken,
		ProvidedPEAuthData: c.ProvidedPEAuthData,
		ProvidedSaAuthData: c.ProvidedSaAuthData,
		RoomID:             roomID,
	}, &response)
	if err != nil {
		return TanLobbyLoginResponse{}, fmt.Errorf("Auth: %w", err)
	}
	return response, nil
}

// TanLobbyCreateRequest creates a new room.
type TanLobbyCreateRequest struct {
	FBToken            string `json:"login_token"`
	ProvidedPEAuthData string `json:"provided_pe_auth_data"`
	ProvidedSaAuthData string `json:"provided_sa_auth_data"`
}

// TanLobbyCreateResponse is returned when creating a room.
type TanLobbyCreateResponse struct {
	Success   bool   `json:"success"`
	ErrorInfo string `json:"error_info"`

	UserUniqueID   uint32 `json:"user_unique_id"`
	UserPlayerName string `json:"user_player_name"`

	RaknetServerAddress string `json:"raknet_server_address"`
	RaknetRand          []byte `json:"raknet_rand"`
	RaknetAESRand       []byte `json:"raknet_aes_rand"`
	EncryptKeyBytes     []byte `json:"encrypt_key_bytes"`
	DecryptKeyBytes     []byte `json:"decrypt_key_bytes"`

	SignalingServerAddress string `json:"signaling_server_address"`
	SignalingSeed          []byte `json:"signaling_seed"`
	SignalingTicket        []byte `json:"signaling_ticket"`
}

// TanLobbyCreate creates a room with the client's session.
func (c *Client) TanLobbyCreate() (TanLobbyCreateResponse, error) {
	var response TanLobbyCreateResponse
	err := c.post("/api/phoenix/tan_lobby_create", TanLobbyCreateRequest{
		FBToken:            c.FBToken,
		ProvidedPEAuthData: c.ProvidedPEAuthData,
		ProvidedSaAuthData: c.ProvidedSaAuthData,
	}, &response)
	if err != nil {
		return TanLobbyCreateResponse{}, fmt.Errorf("TanLobbyCreate: %w", err)
	}
	return response, nil
}

// TanLobbyDebugRequest asks the auth service to debug a room login.
type TanLobbyDebugRequest struct {
	FBToken       string `json:"login_token"`
	LoginResponse string `json:"login_response"`
	RaknetRand    []byte `json:"raknet_rand"`
}

// TanLobbyDebugResponse contains refreshed encryption material.
type TanLobbyDebugResponse struct {
	Success         bool   `json:"success"`
	ErrorInfo       string `json:"error_info"`
	EncryptKeyBytes []byte `json:"encrypt_key_bytes"`
	DecryptKeyBytes []byte `json:"decrypt_key_bytes"`
}

// TanLobbyDebug requests refreshed keys for a room login.
func (c *Client) TanLobbyDebug(loginResponse string, raknetRand []byte) (TanLobbyDebugResponse, error) {
	var response TanLobbyDebugResponse
	err := c.post("/api/phoenix/tan_lobby_debug", TanLobbyDebugRequest{
		FBToken:       c.FBToken,
		LoginResponse: loginResponse,
		RaknetRand:    raknetRand,
	}, &response)
	if err != nil {
		return TanLobbyDebugResponse{}, fmt.Errorf("TanLobbyDebug: %w", err)
	}
	return response, nil
}
