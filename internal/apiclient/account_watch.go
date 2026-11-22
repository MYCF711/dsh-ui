// account_watch.go —— 官方 `account.watch`（`mode:'stream'`）的客户端（GUI 账本 #38 (a)）。
//
// 契约唯一来源是官方 rc.5 源码，不是我们的宿主，也不是推理：
//
//	OFF:dsh-deepseek-account/lib/typert.host.js:640-657
//	    'account/watch': { service:'deepseekAccount', implementation:'watchState',
//	                       mode:'stream', invocation:{kind:'direct'},
//	                       parameters: [], cancellation:{parameter:'signal'} }
//	    result: AccountUpdate（同一块 651 行的 jsonSchema）
//	        { type: 'snapshot'|'credentials-changed', state: AccountView }
//	OFF:dsh-client-ui-settings-account/src/client/index.ts:99-149
//	    客户端消费口径：每帧取 frame.value.type / frame.value.state，
//	    credentials-changed 与换代（:128 frame.generation）都要让下游 reset，
//	    应用完必须 frame.accept()（:138）。
//
// 宿主侧（我们）**早就能服务**这条流，不是新造：
//
//	HOST:internal/transport/handlers_account_official.go:968-1010  serveAccountWatch
//	    开流即 flush 响应头，之后 NDJSON 一行一帧；帧 type 不是上面那两个值就直接断流
//	HOST:internal/transport/handlers_account_official.go:1018-1050 accountWatchUpdates
//	    优先 AccountWatchUpdates（rc.2 的 AccountUpdate），rc.1 的扁平流包成 snapshot
//	HOST:cmd/dshd/serve_account_api.go:225-256 绑到 provider.WatchUpdates
//	HOST:internal/account/account.go:951-991  订阅时 pending 先放当前状态 ⇒ **首帧必有值**
//
// 为什么不能走 `Call`：它把整个 body 读完才返回，而这条流没有自然结束
// （直到客户端取消）。所以照 `forge.go:435-503` 那条 NDJSON 模板自己起请求、逐行解。
package apiclient

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// MethodAccountWatch 是官方 `account.watch` 的方法名（`mode:'stream'`）。
const MethodAccountWatch = "account.watch"

// 官方 `AccountUpdate.type` 的两个值（`lib/typert.host.js:651` 的 enum）。
//
// 为什么必须区分：官方客户端在 `credentials-changed` 时要清掉通知态并重启轮询，
// 而登录进度（snapshot）要**保留**通知态（官方迁移指南第 1 条）。
const (
	// AccountUpdateSnapshot 是初始状态与登录进度。
	AccountUpdateSnapshot = "snapshot"
	// AccountUpdateCredentialsChanged 是凭据被写入/移除。
	AccountUpdateCredentialsChanged = "credentials-changed"
)

// AccountUpdate 是官方 `AccountUpdate`：`account.watch` 的每一帧。
//
// 与 rc.1 的扁平 `AccountView` 流的**唯一**区别就是这层 {type,state} 包装
// （`HOST:internal/transport/handlers_account_official.go:452-460` 同一份判断）。
type AccountUpdate struct {
	Type  string      `json:"type"`
	State AccountView `json:"state"`
}

// CredentialsChanged 报告这一帧是不是凭据变更（官方先判这个再判换代）。
func (u AccountUpdate) CredentialsChanged() bool {
	return u.Type == AccountUpdateCredentialsChanged
}

// Valid 报告帧 type 是否在官方 enum 里。
//
// 不在的帧**不能**当成 snapshot 用：宿主 `serveAccountWatch` 遇到非法 type 是
// 直接断流的（`HOST:…:992-997`），客户端这边一样要当"这次订阅坏了"处理。
func (u AccountUpdate) Valid() bool {
	return u.Type == AccountUpdateSnapshot || u.Type == AccountUpdateCredentialsChanged
}

// AccountWatch 订阅 `account.watch`：每来一帧 `AccountUpdate` 调一次 fn。
//
// 首帧一定是**当前状态**（宿主 `internal/account/account.go:951-991` 在订阅时就把
// 当前状态放进 pending），所以 fn 至少被调用一次才可能正常返回。`ctx` 取消即结束。
//
// 流里的错误帧（宿主在响应头已发出之后只能这样报错）解成四字段错误返回，
// **不静默**当成"没有更多状态"——否则"宿主没有这条流"会被伪装成"账号状态没变化"。
func (c *Client) AccountWatch(ctx context.Context, fn func(AccountUpdate) error) error {
	if c == nil {
		return fmt.Errorf("account.watch 未接线：没有 Transport 客户端")
	}
	body, err := json.Marshal(struct{}{})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Base+"/api/"+MethodAccountWatch, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/x-ndjson")
	// **明说不压缩**（实测必须）。
	//
	// 本仓宿主的 `/api/*` 外面套着运行期 gzip（`HOST:internal/transport/compression.go:45-71`），
	// 而 `serveAccountWatch` 是"先发头再写首帧"的流（`HOST:…:978-983` 的
	// `writeOfficialStreamHeaders` + `flusher.Flush()`）：`compressedResponse.Flush()`
	// 在还没决定压不压的时候就把头刷出去，紧接着的第一次 `Write` 才 `decide(true)`
	// 并 `Header().Set("Content-Encoding","gzip")` —— 那时头已经上路了。
	// 于是线路上收到的是**没有 Content-Encoding 的 gzip 字节**（2026-11-22 用真宿主
	// dshd 实测复现：`\x1f\x8b\x08…`，Go 的 transport 不会解，因为它连"服务端压过"都不知道）。
	//
	// 解法不是猜字节，是**别让它压**：流式帧压了也没有收益（每帧几百字节，还要
	// 等 gzip 缓冲），identity 让线路上就是官方那份 NDJSON。
	req.Header.Set("Accept-Encoding", "identity")
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}
	// 长连接不能继承一元调用的整体超时；取消只由 ctx 决定（与 forgePanel.follow 同口径）。
	httpClient := *c.HTTP
	httpClient.Timeout = 0
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("连不上 %s：%w", c.Base, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var env envelope
		if json.Unmarshal(raw, &env) == nil && len(env.Error) > 0 {
			if apiErr := parseError(env.Error); apiErr != nil {
				return apiErr
			}
		}
		return fmt.Errorf("%s 返回 HTTP %d: %.200s", MethodAccountWatch, resp.StatusCode, raw)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/x-ndjson") {
		return fmt.Errorf("%s 必须返回 application/x-ndjson，拿到 %q", MethodAccountWatch, got)
	}
	// 上面的 identity 拦住了本仓宿主；但中间有别的跳（代理/网关）时仍可能被压，
	// 那种情况下头是**正确**带出来的，自己解即可 —— 两类情况都不该让帧变成乱码。
	var stream io.Reader = resp.Body
	if strings.HasPrefix(resp.Header.Get("Content-Encoding"), "gzip") {
		gzr, gzErr := gzip.NewReader(resp.Body)
		if gzErr != nil {
			return fmt.Errorf("%s 声明了 gzip 但解不开：%w", MethodAccountWatch, gzErr)
		}
		defer gzr.Close()
		stream = gzr
	}
	sc := bufio.NewScanner(stream)
	// 一帧是 AccountView（status/links/attempt），几百字节；给足余量。
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// 错误帧：宿主在流里写 `{"error":{code,message}}`（响应头已经发出）。
		var errFrame struct {
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &errFrame); err != nil {
			return fmt.Errorf("account.watch 帧不是合法 JSON: %.200s", line)
		}
		if len(errFrame.Error) > 0 {
			if apiErr := parseError(errFrame.Error); apiErr != nil {
				return apiErr
			}
			return fmt.Errorf("account.watch 流里报告了错误: %.200s", line)
		}
		var update AccountUpdate
		if err := json.Unmarshal([]byte(line), &update); err != nil {
			return fmt.Errorf("account.watch 帧不是 AccountUpdate: %.200s", line)
		}
		if !update.Valid() {
			return fmt.Errorf("account.watch 帧 type 非法：%q（官方 enum 只有 %q / %q）",
				update.Type, AccountUpdateSnapshot, AccountUpdateCredentialsChanged)
		}
		if err := fn(update); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}
