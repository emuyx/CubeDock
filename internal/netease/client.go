package netease

import "context"

// Authenticate performs PE authentication for one normalized session. The
// method is inferred from the session; use AuthenticateFor to force one.
func (c *Client) Authenticate(ctx context.Context, session string) error {
	return c.AuthenticateFor(ctx, "", session)
}

// AuthenticateFor performs PE authentication using a specific provider.
func (c *Client) AuthenticateFor(ctx context.Context, method AuthMethod, session string) error {
	parsed, err := ParseSessionFor(method, session)
	if err != nil {
		return err
	}
	creds, err := c.performPEAuth(ctx, parsed)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.creds = creds
	c.mu.Unlock()
	return nil
}

// Credentials returns a copy of the current account identity.
func (c *Client) Credentials() Credentials {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.creds
}

// Discovered reports whether service discovery has completed.
func (c *Client) Discovered() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.discovered
}
