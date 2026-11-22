// account_watch_test.go —— 官方 `account.watch` 客户端（`mode:'stream'`）的证据。
//
// 三件事各有断言：
//
//	① 帧形状    每行一个 `{type,state}`；type 只有官方 enum 那两个值
//	② 非法 wire 非法 type / 非 ndjson / 流内错误帧 —— 都要**报错**，不许当成"没变化"
//	③ 回落素材  not_implemented 要原样交回四字段码（壳据此判"宿主没这条流"）
package apiclient

import (
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// accountWatchServer 起一个只服务 `/api/account.watch` 的假宿主。
func accountWatchServer(t *testing.T, handle func(w http.ResponseWriter, r *http.Request)) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/account.watch" {
			t.Errorf("路径 = %q，want /api/account.watch", r.URL.Path)
		}
		handle(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// ndjsonStream 写好流式响应头，把帧逐行写出并 flush。
func ndjsonStream(w http.ResponseWriter, frames ...string) {
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	for _, frame := range frames {
		_, _ = w.Write([]byte(frame + "\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}
}

// TestAccountWatchStreamsAccountUpdates ①：`{type,state}` 两帧都要逐字段解出来。
//
// 这是一条"形状一旦回退就红"的判据：rc.1 的 watch 帧是**扁平 AccountView**
// （status/links/attempt 直接在顶上），rc.2 起包了一层 type+state
// （OFF:dsh-deepseek-account/lib/typert.host.js:640-657）。解错层就只会拿到空结构。
func TestAccountWatchStreamsAccountUpdates(t *testing.T) {
	var body map[string]any
	base := accountWatchServer(t, func(w http.ResponseWriter, r *http.Request) {
		body = decodeBody(t, r)
		ndjsonStream(w,
			// 第一帧：已登录、没有尝试、links 齐。
			`{"type":"snapshot","state":{"status":"credential-stored",`+
				`"links":{"usageUrl":"https://platform.test/usage","topUpUrl":"https://platform.test/topup"},`+
				`"attempt":null,"source":"local"}}`,
			// 第二帧：凭据变更（登出）。
			`{"type":"credentials-changed","state":{"status":"signed-out",`+
				`"links":{"usageUrl":"https://platform.test/usage","topUpUrl":"https://platform.test/topup"},`+
				`"attempt":null}}`,
		)
		<-r.Context().Done()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var got []AccountUpdate
	err := New(base).AccountWatch(ctx, func(update AccountUpdate) error {
		got = append(got, update)
		if len(got) == 2 {
			cancel()
			return context.Canceled
		}
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("watch 不该报错：%v", err)
	}
	if len(body) != 0 {
		t.Fatalf("官方 account/watch 没有参数，实得 %v", body)
	}
	if len(got) != 2 {
		t.Fatalf("应当收到 2 帧，实得 %d", len(got))
	}
	first := got[0]
	if first.Type != AccountUpdateSnapshot || first.CredentialsChanged() {
		t.Fatalf("第一帧 type 应是 snapshot，实得 %q", first.Type)
	}
	if !first.State.SignedIn() || first.State.Status != AccountCredentialStored {
		t.Fatalf("第一帧 state 解错：%+v", first.State)
	}
	if first.State.Links.UsageURL != "https://platform.test/usage" ||
		first.State.Links.TopUpURL != "https://platform.test/topup" {
		t.Fatalf("第一帧 links 解错：%+v", first.State.Links)
	}
	if first.State.Attempt != nil {
		t.Fatalf("第一帧没有尝试，attempt 必须是 nil：%+v", first.State.Attempt)
	}
	second := got[1]
	if !second.CredentialsChanged() || second.Type != AccountUpdateCredentialsChanged {
		t.Fatalf("第二帧 type 应是 credentials-changed，实得 %q", second.Type)
	}
	if second.State.SignedIn() {
		t.Fatalf("第二帧是登出，SignIn 必须为假：%+v", second.State)
	}
}

// TestAccountWatchKeepsAttemptProgress 首帧之后是登录进度（snapshot）——
// `attempt.phase` 必须原样留下，官方 AccountSection 的四相位全靠它。
func TestAccountWatchKeepsAttemptProgress(t *testing.T) {
	base := accountWatchServer(t, func(w http.ResponseWriter, r *http.Request) {
		ndjsonStream(w,
			`{"type":"snapshot","state":{"status":"signed-out",`+
				`"links":{"usageUrl":"u","topUpUrl":"t"},`+
				`"attempt":{"id":"a-1","phase":"waiting-browser","authorizeUrl":"https://auth.test/x","expiresAt":1763000000000}}}`,
		)
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var got AccountUpdate
	err := New(base).AccountWatch(ctx, func(update AccountUpdate) error {
		got = update
		cancel()
		return context.Canceled
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("watch 不该报错：%v", err)
	}
	if got.State.Attempt == nil {
		t.Fatal("attempt 丢了：登录进度卡会画不出来")
	}
	if got.State.Attempt.ID != "a-1" || got.State.Attempt.Phase != "waiting-browser" {
		t.Fatalf("attempt 解错：%+v", got.State.Attempt)
	}
	if !got.State.ActiveSignIn() {
		t.Fatal("waiting-browser 是官方四相位之一，ActiveSignIn 必须为真")
	}
	if got.State.Attempt.AuthorizeURL != "https://auth.test/x" || got.State.Attempt.ExpiresAt != 1763000000000 {
		t.Fatalf("attempt 的 authorizeUrl/expiresAt 解错：%+v", got.State.Attempt)
	}
}

// TestAccountWatchRejectsUnknownType ②：type 不在官方 enum 里 = 这次订阅坏了。
//
// 为什么不悄悄当 snapshot：宿主 `serveAccountWatch` 遇到非法 type 是**直接断流**
// （HOST:internal/transport/handlers_account_official.go:992-997），客户端要是
// 忍下来就会演成"宿主坏了但界面一直显示旧状态"。
func TestAccountWatchRejectsUnknownType(t *testing.T) {
	base := accountWatchServer(t, func(w http.ResponseWriter, r *http.Request) {
		ndjsonStream(w,
			`{"type":"snapshot","state":{"status":"credential-stored","links":{"usageUrl":"u","topUpUrl":"t"},"attempt":null}}`,
			`{"type":"mystery","state":{"status":"signed-out","links":{"usageUrl":"u","topUpUrl":"t"},"attempt":null}}`,
		)
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	applied := 0
	err := New(base).AccountWatch(ctx, func(AccountUpdate) error {
		applied++
		return nil
	})
	if err == nil {
		t.Fatal("非法 type 必须报错，不许静默跳过")
	}
	if applied != 1 {
		t.Fatalf("合法帧应当被应用，非法帧不该：applied=%d", applied)
	}
	if !strings.Contains(err.Error(), `"mystery"`) {
		t.Fatalf("错误里要点出非法值，实得 %v", err)
	}
}

// TestAccountWatchRejectsNonNdjson ②b：响应不是 ndjson 就不是这条流，别硬解。
func TestAccountWatchRejectsNonNdjson(t *testing.T) {
	base := accountWatchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"status":"signed-out"}}`))
	})
	err := New(base).AccountWatch(context.Background(), func(AccountUpdate) error { return nil })
	if err == nil {
		t.Fatal("非 ndjson 必须报错")
	}
	if !strings.Contains(err.Error(), "application/x-ndjson") {
		t.Fatalf("错误要说清要的是 ndjson，实得 %v", err)
	}
}

// TestAccountWatchReportsStreamErrorFrame ②c：流里夹着错误帧要解成四字段错误。
func TestAccountWatchReportsStreamErrorFrame(t *testing.T) {
	base := accountWatchServer(t, func(w http.ResponseWriter, r *http.Request) {
		ndjsonStream(w,
			`{"error":{"code":"internal","message":"账号栈挂了","hint":"看宿主日志","retryable":true}}`,
		)
	})
	err := New(base).AccountWatch(context.Background(), func(AccountUpdate) error { return nil })
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("流内错误帧要解成 *APIError，实得 %T %v", err, err)
	}
	if apiErr.Code != "internal" || apiErr.Message != "账号栈挂了" || !apiErr.Retryable {
		t.Fatalf("四字段解错：%+v", apiErr)
	}
}

// TestAccountWatchCapabilityMissing ③：宿主没有这条流时回 not_implemented，
// 客户端必须原样交回那个码（壳据此判"不可用、不再重连、纯轮询"）。
func TestAccountWatchCapabilityMissing(t *testing.T) {
	base := accountWatchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"account.watch 未接线：宿主没有账号状态流","hint":"老宿主","retryable":false}}`))
	})
	err := New(base).AccountWatch(context.Background(), func(AccountUpdate) error { return nil })
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("501 要解成 *APIError，实得 %T %v", err, err)
	}
	if apiErr.Code != "not_implemented" {
		t.Fatalf("码要原样保留，实得 %q", apiErr.Code)
	}
}

// TestAccountWatchStopsOnCancel：取消即结束，不许把 goroutine 挂在长流上。
func TestAccountWatchStopsOnCancel(t *testing.T) {
	opened := make(chan struct{})
	base := accountWatchServer(t, func(w http.ResponseWriter, r *http.Request) {
		ndjsonStream(w,
			`{"type":"snapshot","state":{"status":"signed-out","links":{"usageUrl":"u","topUpUrl":"t"},"attempt":null}}`,
		)
		close(opened)
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- New(base).AccountWatch(ctx, func(AccountUpdate) error { return nil })
	}()
	<-opened
	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("取消应当干净结束，实得 %v", err)
	}
}

// TestAccountWatchAsksForIdentityEncoding 钉住"这条流请求**不要**压缩"。
//
// 为什么值得一条测试：2026-11-22 用真宿主（dshd serve + 运行期 gzip 中间件）实测，
// 带默认 `Accept-Encoding: gzip` 时线上收到的是**没有 Content-Encoding 头的 gzip 字节**
// （`\x1f\x8b\x08…`）—— 因为 `serveAccountWatch` 先 flush 响应头，压缩决定发生在那之后
// （HOST:internal/transport/compression.go:166-173 的 Flush 先于 decide）。
// 客户端拿不到帧，账号状态会**静默退回**「未知」，比不接推送更糟。
// 修法是请求时明说不压缩（流式帧压了也没收益），这条测试钉住那个头。
func TestAccountWatchAsksForIdentityEncoding(t *testing.T) {
	var got string
	base := accountWatchServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Accept-Encoding")
		if r.Header.Get("Accept") != "application/x-ndjson" {
			t.Errorf("Accept = %q，want application/x-ndjson", r.Header.Get("Accept"))
		}
		ndjsonStream(w,
			`{"type":"snapshot","state":{"status":"signed-out","links":{"usageUrl":"u","topUpUrl":"t"},"attempt":null}}`,
		)
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = New(base).AccountWatch(ctx, func(AccountUpdate) error {
		cancel()
		return context.Canceled
	})
	if got != "identity" {
		t.Fatalf("请求头 Accept-Encoding = %q，必须是 identity —— 否则本仓宿主的流会被压成"+
			"没有 Content-Encoding 的乱码（见本条注释）", got)
	}
}

// TestAccountWatchDecodesGzipWhenHostCompresses ②d：中间跳真的压了（**正确**带
// `Content-Encoding: gzip`）时仍要解出帧 —— identity 是"别压"，不是"不会压"。
func TestAccountWatchDecodesGzipWhenHostCompresses(t *testing.T) {
	base := accountWatchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		gz := gzip.NewWriter(w)
		_, _ = gz.Write([]byte(`{"type":"snapshot","state":{"status":"credential-stored",` +
			`"links":{"usageUrl":"u","topUpUrl":"t"},"attempt":null}}` + "\n"))
		_ = gz.Flush()
		// gzip 写完还要把 HTTP 层的缓冲推出去，否则客户端一直等（真宿主的
		// `compressedResponse.Flush` 就是这两步一起做，见 compression.go:166-173）。
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done()
		_ = gz.Close()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var got AccountUpdate
	err := New(base).AccountWatch(ctx, func(update AccountUpdate) error {
		got = update
		cancel()
		return context.Canceled
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("watch 不该报错：%v", err)
	}
	if got.Type != AccountUpdateSnapshot || !got.State.SignedIn() {
		t.Fatalf("gzip 流没解出来：%+v", got)
	}
}
