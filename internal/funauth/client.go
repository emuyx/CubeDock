// Package funauth contains the minimal FunAuth/G79 room-authentication logic
// used by CubeDock. It derives from the AGPL-3.0 FunAuth project; see NOTICE.
// CubeDock only uses the 互通版 (platform=pc) session path.
package funauth

import (
	"context"
	"strings"
	"sync"
	"time"

	g79 "github.com/Yeah114/g79client"
)

// FallbackEngineVersion is only used to keep g79client's mobile code path
// initialised. CubeDock never opens rooms with phone (platform=ad) sessions;
// the 互通版 path overwrites the version with its fixed official values.
const FallbackEngineVersion = "3.9.15.297907"

var (
	clientOnce sync.Once
	refresh    sync.Once
)

// NewClient prepares the G79 HTTP client. Metadata is refreshed every ten
// minutes after the first successful authentication.
func NewClient(ctx context.Context) (*g79.Client, error) {
	_ = ctx
	clientOnce.Do(func() {
		if strings.TrimSpace(g79.EngineVersion) == "" {
			g79.EngineVersion = FallbackEngineVersion
		}
	})
	refresh.Do(func() {
		go func() {
			ticker := time.NewTicker(10 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				refreshMetadata()
			}
		}()
	})
	return g79.NewClient()
}

func refreshMetadata() {
	_, _ = g79.RefreshG79PatchMetadata()
	_, _ = g79.RefreshG79ReleaseJSON()
	_, _ = g79.RefreshX19ReleaseJSON()
	_, _ = g79.RefreshG79ChatServers()
	_, _ = g79.RefreshG79LinkServers()
	_, _ = g79.RefreshG79TransferServers()
}
