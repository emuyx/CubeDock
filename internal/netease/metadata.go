package netease

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	defaultG79ReleaseURL = "https://g79.update.netease.com/serverlist/adr_release.0.17.json"
	defaultX19ReleaseURL = "https://x19.update.netease.com/serverlist/release.json"
)

// Client owns the release metadata and, after authentication, the account
// credentials used to request room transfer material.
type Client struct {
	HTTP   *http.Client
	G79URL string
	X19URL string

	mu         sync.RWMutex
	g79        ReleaseInfo
	x19        X19Release
	creds      Credentials
	discovered bool
}

// NewClient returns a client with defaults; call Discover before use.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		HTTP:   httpClient,
		G79URL: defaultG79ReleaseURL,
		X19URL: defaultX19ReleaseURL,
	}
}

// New discovers the current G79 and x19 service endpoints.
func New(ctx context.Context) (*Client, error) {
	client := NewClient(nil)
	if err := client.Discover(ctx); err != nil {
		return nil, err
	}
	return client, nil
}

// Discover fetches the service-discovery documents.
func (c *Client) Discover(ctx context.Context) error {
	var g79 ReleaseInfo
	if err := c.getJSON(ctx, c.G79URL, &g79); err != nil {
		return fmt.Errorf("读取网易 G79 服务配置失败: %w", err)
	}
	if err := validateServiceURL(g79.CoreServerURL); err != nil {
		return fmt.Errorf("网易 G79 服务地址无效: %w", err)
	}
	var x19 X19Release
	if err := c.getJSON(ctx, c.X19URL, &x19); err != nil {
		return fmt.Errorf("读取网易 x19 服务配置失败: %w", err)
	}
	// The gray gateway is only used after PE authentication; tolerate an empty
	// value but reject a malformed URL when one is present.
	if x19.APIGatewayGrayURL != "" {
		if err := validateServiceURL(x19.APIGatewayGrayURL); err != nil {
			return fmt.Errorf("网易 x19 网关地址无效: %w", err)
		}
	}
	c.mu.Lock()
	c.g79 = g79
	c.x19 = x19
	c.discovered = true
	c.mu.Unlock()
	return nil
}

func (c *Client) getJSON(ctx context.Context, address string, out any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func validateServiceURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return fmt.Errorf("不是有效的 HTTPS 服务地址")
	}
	return nil
}

// Release returns the discovered G79 document.
func (c *Client) Release() ReleaseInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.g79
}

// GrayGateway returns the x19 API gateway used after PE authentication.
func (c *Client) GrayGateway() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.x19.APIGatewayGrayURL
}
