package netease

import (
	"context"
	"fmt"
)

// Authenticate performs PE authentication for one normalized session.
//
// The HTTP envelope, key table and PeAuthSign tables still require capture
// evidence; until they are recorded in the internal specification this
// deliberately fails instead of guessing protocol details.
func (c *Client) Authenticate(ctx context.Context, session string) error {
	parsed, err := ParseSession(session)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.creds = Credentials{Session: parsed}
	c.mu.Unlock()
	return ErrAwaitingCaptures
}

// Profile returns the game identity behind the authenticated session.
func (c *Client) Profile(ctx context.Context) (Profile, error) {
	c.mu.RLock()
	creds := c.creds
	discovered := c.discovered
	c.mu.RUnlock()
	if !discovered {
		return Profile{}, fmt.Errorf("网易服务配置尚未加载")
	}
	if creds.Token == "" {
		return Profile{}, fmt.Errorf("账号尚未完成 PE 认证")
	}
	return Profile{}, ErrAwaitingCaptures
}

// CreateRoom returns the transfer credentials for one hosted room.
func (c *Client) CreateRoom(ctx context.Context) (RoomAuth, error) {
	c.mu.RLock()
	creds := c.creds
	discovered := c.discovered
	c.mu.RUnlock()
	if !discovered {
		return RoomAuth{}, fmt.Errorf("网易服务配置尚未加载")
	}
	if creds.Token == "" {
		return RoomAuth{}, fmt.Errorf("账号尚未完成 PE 认证")
	}
	return RoomAuth{}, ErrAwaitingCaptures
}

// Credentials returns a copy of the current account identity.
func (c *Client) Credentials() Credentials {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.creds
}
