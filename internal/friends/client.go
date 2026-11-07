// Package friends polls and accepts the bot account's pending NetEase friend
// requests while a room is online.
package friends

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/bits"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	releaseManifest = "https://g79.update.netease.com/serverlist/adr_release.0.17.json"
	signSalt        = "0eGsBkhl"

	// gatewayRefresh bounds how long one discovery result is trusted. The
	// gateway can move or stop serving an old address while a room stays open.
	gatewayRefresh = 30 * time.Minute
	// maxFriendRetryDelay keeps a transient failure from looking like the
	// watcher has stopped for too long.
	maxFriendRetryDelay = 2 * time.Minute
)

// Watch accepts friend requests until ctx is cancelled. It is a no-op if the
// account identity or encryption key is incomplete. Errors back off without
// affecting the game relay.
func Watch(ctx context.Context, uid uint32, encryptionKey []byte, event, debug func(string)) {
	if uid == 0 || len(encryptionKey) != 32 {
		event("无法开启自动同意好友申请：鉴权材料不完整")
		return
	}
	client := &gatewayClient{
		http: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
		manifest: releaseManifest,
		uid:      strconv.FormatUint(uint64(uid), 10),
		key:      bytes.Clone(encryptionKey[:16]),
	}
	if debug == nil {
		debug = func(string) {}
	}
	watcher := &watcher{
		client:   client,
		accepted: make(map[uint64]string),
		pace:     time.Second,
		event:    event,
		debug:    debug,
	}
	watch(ctx, watcher.poll, event, debug)
}

func watch(ctx context.Context, poll func(context.Context) error, event, debug func(string)) {
	event("自动同意好友申请已开启（每 30 秒检查）")
	delay := 30 * time.Second
	failures := 0
	for ctx.Err() == nil {
		err := poll(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			failures++
			debug("好友申请检查失败：" + err.Error())
			delay = min(delay*2, maxFriendRetryDelay)
			if failures == 1 || failures%5 == 0 {
				event(fmt.Sprintf("自动同意好友申请失败（连续 %d 次），%.0f 秒后重试：%s",
					failures, delay.Seconds(), err))
			}
		} else {
			if failures > 0 {
				event("自动同意好友申请已恢复")
			}
			failures = 0
			delay = 30 * time.Second
		}
		if !sleep(ctx, delay) {
			return
		}
	}
}

type application struct {
	UID       json.RawMessage `json:"uid"`
	Nickname  string          `json:"nickname"`
	ApplyTime json.RawMessage `json:"apply_time"`
}

type gatewayClient struct {
	http          *http.Client
	manifest      string
	gateway       string
	gatewayAt     time.Time
	uid           string
	key           []byte
	coreSignature bool
}

// requestSignature signs the exact body with a value derived from the room's
// encryption key. The raw account token is never needed.
func requestSignature(key []byte, path string, body []byte, core bool) string {
	sum := md5.Sum([]byte(hex.EncodeToString(key) + string(body) + signSalt + strings.TrimSuffix(path, "?")))
	text := []byte(hex.EncodeToString(sum[:]))
	encoded := make([]byte, len(text))
	for i, b := range text {
		rotated := b<<6 | text[(i+1)%len(text)]>>2
		if core {
			rotated = bits.Reverse8(rotated)
		}
		encoded[i] = b ^ rotated
	}
	text64 := base64.StdEncoding.EncodeToString(encoded)[:16]
	return strings.NewReplacer("+", "m", "/", "o").Replace(text64) + "1"
}

func (c *gatewayClient) request(ctx context.Context, path string, body []byte, result any) error {
	method, address := http.MethodPost, c.gateway+path
	if path == "" {
		method, address = http.MethodGet, c.manifest
	}
	req, err := http.NewRequestWithContext(ctx, method, address, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("好友接口地址无效")
	}
	if path != "" {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		req.Header.Set("User-Agent", "libhttpclient/1.0.0.0")
		req.Header.Set("user-id", c.uid)
		req.Header.Set("user-token", requestSignature(c.key, path, body, c.coreSignature))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("好友接口连接失败")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("好友接口返回 HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 || json.Unmarshal(raw, result) != nil {
		return fmt.Errorf("好友接口响应格式无效")
	}
	return nil
}

func (c *gatewayClient) discover(ctx context.Context) error {
	var release struct {
		Gateway string `json:"ApiGatewayUrl"`
	}
	if err := c.request(ctx, "", nil, &release); err != nil {
		return err
	}
	parsed, err := url.Parse(release.Gateway)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("网易好友接口配置无效")
	}
	c.gateway = strings.TrimRight(release.Gateway, "/")
	c.gatewayAt = time.Now()
	return nil
}

type responseError struct {
	action string
	code   int
}

func (e responseError) Error() string {
	return fmt.Sprintf("%s失败，错误码 %d", e.action, e.code)
}

func (c *gatewayClient) pending(ctx context.Context) ([]application, error) {
	applications, err := c.fetchPending(ctx)
	if err == nil {
		return applications, nil
	}
	// Negotiate the alternative signature only on a read-only call. Retrying a
	// write could accept the same request twice. Try again even if a format
	// worked before: the server can change or move between formats mid-session.
	code, ok := err.(responseError)
	if !ok || code.code != 22 || ctx.Err() != nil {
		return nil, err
	}
	previous := c.coreSignature
	c.coreSignature = !previous
	applications, fallbackErr := c.fetchPending(ctx)
	if fallbackErr != nil {
		c.coreSignature = previous
		return nil, fallbackErr
	}
	return applications, nil
}

func (c *gatewayClient) fetchPending(ctx context.Context) ([]application, error) {
	var result struct {
		Code     *int          `json:"code"`
		Entities []application `json:"entities"`
	}
	if err := c.request(ctx, "/user-addme-friends/", []byte("{}"), &result); err != nil {
		return nil, err
	}
	if result.Code == nil {
		return nil, fmt.Errorf("好友申请列表缺少状态码")
	}
	if *result.Code != 0 {
		return nil, responseError{"查询好友申请", *result.Code}
	}
	return result.Entities, nil
}

func (c *gatewayClient) accept(ctx context.Context, uid uint64) error {
	body, _ := json.Marshal(struct {
		FID      uint64 `json:"fid"`
		Accept   bool   `json:"accept"`
		AcceptUI string `json:"accept_ui"`
	}{uid, true, "联系人-好友申请"})
	var result struct {
		Code *int `json:"code"`
	}
	if err := c.request(ctx, "/user-reply-friend/", body, &result); err != nil {
		return err
	}
	if result.Code == nil {
		return fmt.Errorf("同意好友申请响应缺少状态码")
	}
	if *result.Code != 0 {
		return responseError{"同意好友申请", *result.Code}
	}
	return nil
}

type watcher struct {
	client   *gatewayClient
	accepted map[uint64]string
	pace     time.Duration
	event    func(string)
	debug    func(string)
}

func (w *watcher) ensureGateway(ctx context.Context) error {
	if w.client.gateway == "" {
		return w.client.discover(ctx)
	}
	if !w.client.gatewayAt.IsZero() && time.Since(w.client.gatewayAt) > gatewayRefresh {
		// A failed refresh is not fatal: the old address may still work. If it
		// does not, the pending call clears it and the next poll rediscovers.
		if err := w.client.discover(ctx); err != nil && w.debug != nil {
			w.debug("好友网关重新发现失败：" + err.Error())
		}
	}
	return nil
}

func (w *watcher) poll(ctx context.Context) error {
	if err := w.ensureGateway(ctx); err != nil {
		return err
	}
	applications, err := w.client.pending(ctx)
	if err != nil {
		// The gateway may have rotated while the room stayed open. Force a
		// fresh discovery before the next poll.
		w.client.gateway = ""
		return err
	}
	// Forget responders who disappeared so a later request is accepted again
	// and the cache stays bounded.
	present := make(map[uint64]bool, len(applications))
	for _, app := range applications {
		if id, ok := accountID(app.UID); ok {
			present[id] = true
		}
	}
	for id := range w.accepted {
		if !present[id] {
			delete(w.accepted, id)
		}
	}

	attempted := 0
	tried := make(map[uint64]bool)
	var failures []string
	for _, app := range applications {
		id, ok := accountID(app.UID)
		if !ok || strconv.FormatUint(id, 10) == w.client.uid {
			continue
		}
		stamp := string(app.ApplyTime)
		if seen, exists := w.accepted[id]; exists && seen == stamp {
			continue
		}
		if tried[id] {
			continue
		}
		if ctx.Err() != nil || attempted == 20 {
			break
		}
		tried[id] = true
		if attempted > 0 && !sleep(ctx, w.pace) {
			return ctx.Err()
		}
		attempted++
		if err := w.client.accept(ctx, id); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// One bad request must not block every later request in the list.
			failure := fmt.Sprintf("%s（%d）：%v", displayName(app.Nickname), id, err)
			failures = append(failures, failure)
			if w.debug != nil {
				w.debug("同意好友申请失败：" + failure)
			}
			continue
		}
		w.accepted[id] = stamp
		w.event(fmt.Sprintf("已自动同意好友申请：%s（%d）", displayName(app.Nickname), id))
	}
	if len(failures) > 0 {
		shown := failures
		if len(shown) > 3 {
			shown = shown[:3]
		}
		message := strings.Join(shown, "；")
		if len(failures) > len(shown) {
			message += fmt.Sprintf("；另有 %d 个失败", len(failures)-len(shown))
		}
		return fmt.Errorf("同意 %d 个好友申请失败：%s", len(failures), message)
	}
	return ctx.Err()
}

func accountID(raw json.RawMessage) (uint64, bool) {
	text := string(raw)
	if len(text) > 0 && text[0] == '"' {
		if json.Unmarshal(raw, &text) != nil {
			return 0, false
		}
	}
	id, err := strconv.ParseUint(text, 10, 64)
	return id, err == nil && id != 0
}

func displayName(name string) string {
	printable := []rune(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name))
	if len(printable) > 40 {
		printable = printable[:40]
	}
	return string(printable)
}

func sleep(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
