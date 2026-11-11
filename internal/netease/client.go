package netease

import "context"

// Authenticate performs PE authentication for one normalized session.
func (c *Client) Authenticate(ctx context.Context, session string) error {
	parsed, err := ParseSession(session)
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
