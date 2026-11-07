package g79client

import (
	"net/http"
	"strconv"
)

// EngineVersion is the NetEase PC/Minecraft engine version used when signing
// the legacy mobile PE auth payload. CubeDock keeps a fallback value; the
// 互通版 path replaces it with the fixed official launcher versions.
var EngineVersion = "3.9.42.301055"

func Refetch() {
	packList, _ := RefreshG79PackList()
	if neteasePack, ok := packList["netease"]; ok && neteasePack.Version != "" {
		EngineVersion = neteasePack.Version
	}
	_, _ = RefreshG79LatestVersion()
	_, _ = RefreshG79ReleaseJSON()
	_, _ = RefreshX19ReleaseJSON()
	_, _ = RefreshG79ChatServers()
	_, _ = RefreshG79LinkServers()
	_, _ = RefreshG79TransferServers()
}

// Automatic refetching is disabled in the CubeDock build. NewClient
// fetches the metadata it needs, so the library no longer performs background
// NetEase requests when the host program starts or while it is idle.

// Client 结构体
type Client struct {
	UserID             string
	UserToken          string
	Seed               string
	ReleaseJSON        G79ReleaseJSON
	X19ReleaseJSON     X19ReleaseJSON
	EngineVersion      string
	G79LatestVersion   string
	patchResourcesHash string
	UserDetail         *UserDetailEntity
	peUserLoginAfter   *PeUserLoginAfterResponse
	httpClient         *http.Client
	Cookie             string
}

// 创建新的客户端
func NewClient() (*Client, error) {
	c := &Client{
		EngineVersion: EngineVersion,
		httpClient:    &http.Client{},
	}
	var err error
	// PC launcher sessions use their own fixed patch metadata and do not need
	// the Android patch list. Keep the fields empty when that list is
	// unavailable; the Android auth path reports the missing patch itself.
	if patchMeta, metaErr := GetGlobalG79PatchMetadata(); metaErr == nil {
		c.G79LatestVersion = patchMeta.Version
		c.patchResourcesHash = patchMeta.ResourcesHash
	}
	ReleaseJSON, err := GetGlobalG79ReleaseJSON()
	if err != nil {
		return nil, err
	}
	c.ReleaseJSON = *ReleaseJSON
	X19ReleaseJSON, err := GetGlobalX19ReleaseJSON()
	if err != nil {
		return nil, err
	}
	c.X19ReleaseJSON = *X19ReleaseJSON
	return c, nil
}

// 设置用户凭证
func (c *Client) SetCredentials(userID, userToken string) {
	c.UserID = userID
	c.UserToken = userToken
}

// PatchResourcesHash returns the RN patch resources hash selected for this
// client. It is exposed for diagnostics when the live patch list lags behind
// a client build.
func (c *Client) PatchResourcesHash() string {
	return c.patchResourcesHash
}

// SetPatchVersion overrides the RN patch version and resources hash used in
// PE auth. Callers can pin a patch that is not yet present in
// the public patch list; the empty string keeps the current value.
func (c *Client) SetPatchVersion(version, resourcesHash string) {
	if version != "" {
		c.G79LatestVersion = version
	}
	if resourcesHash != "" {
		c.patchResourcesHash = resourcesHash
	}
}

// 获取用户ID的整数形式
func (c *Client) GetUserIDInt() (int64, error) {
	return strconv.ParseInt(c.UserID, 10, 64)
}
