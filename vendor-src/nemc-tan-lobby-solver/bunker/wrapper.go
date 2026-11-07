package bunker

import "fmt"

// Authenticator is the room-auth surface used by the signaling/service layer.
type Authenticator interface {
	GetAccess(roomID string) (TanLobbyLoginResponse, error)
	GetCreate() (TanLobbyCreateResponse, error)
	GetDebug(loginResponse string, raknetRand []byte) (TanLobbyDebugResponse, error)
}

// AccessWrapper adapts Client to Authenticator.
type AccessWrapper struct {
	client *Client
}

// NewAccessWrapper builds an Authenticator backed by the Phoenix endpoints.
func NewAccessWrapper(authServer, token, peAuthData, saAuthData string) *AccessWrapper {
	return &AccessWrapper{client: NewClient(authServer, token, peAuthData, saAuthData)}
}

func (w *AccessWrapper) GetAccess(roomID string) (TanLobbyLoginResponse, error) {
	response, err := w.client.Auth(roomID)
	if err != nil {
		return TanLobbyLoginResponse{}, fmt.Errorf("GetAccess: %w", err)
	}
	return response, nil
}

func (w *AccessWrapper) GetCreate() (TanLobbyCreateResponse, error) {
	response, err := w.client.TanLobbyCreate()
	if err != nil {
		return TanLobbyCreateResponse{}, fmt.Errorf("GetCreate: %w", err)
	}
	return response, nil
}

func (w *AccessWrapper) GetDebug(loginResponse string, raknetRand []byte) (TanLobbyDebugResponse, error) {
	response, err := w.client.TanLobbyDebug(loginResponse, raknetRand)
	if err != nil {
		return TanLobbyDebugResponse{}, fmt.Errorf("GetDebug: %w", err)
	}
	return response, nil
}
