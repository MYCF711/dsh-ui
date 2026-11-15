// plugin_market_test.go —— 官方 `pluginMarket.readEntryDetail` 的客户端契约测试。
//
// 判据对着宿主与官方两份原件，不是猜的：
//
//	① 入参**平铺**四个键（url/name/version/language），不套 params 信封、不带 options
//	   —— HOST:internal/transport/handlers_plugin_market_official.go:53-63 只解这四个；
//	② `found` / `none` 是**判别联合**：none 不许被解成"读到了空文档"，
//	   也不许被当成错误（官方 types.ts:157-166 明写 none 是一个合法回答）；
//	③ 失败档映射与官方客户端逐条一致：只有 `plugin-market/detail-unavailable` 带
//	   结构化 reason，其余（含本仓自己的 `detail_unavailable`）一律 other
//	   （OFF:../dsh-client-ui-plugin-market/src/client/market-store.ts:142）。
package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPluginMarketReadEntryDetailSendsFlatParams(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pluginMarket.readEntryDetail" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{"status":"found","origin":"readme","markdown":"# 标题\n\n正文","document":"demo@1.0.0"}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginMarketReadEntryDetail(context.Background(), "https://m.example/index.json", "demo", "1.0.0", "zh-CN")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	want := map[string]any{"url": "https://m.example/index.json", "name": "demo", "version": "1.0.0", "language": "zh-CN"}
	for key, value := range want {
		if body[key] != value {
			t.Fatalf("入参 %q = %v，想要 %v（全量：%v）", key, body[key], value, body)
		}
	}
	if _, wrapped := body["params"]; wrapped {
		t.Fatalf("不许套 params 信封：%v", body)
	}
	// 宿主 handler 不读 options：发了也白发，所以客户端不该发（文件头第 2 条差值）。
	for _, ignored := range []string{"refresh", "npmFileSources"} {
		if _, sent := body[ignored]; sent {
			t.Fatalf("宿主不读 %q，客户端不许发（会让界面以为重试能绕过缓存）：%v", ignored, body)
		}
	}
	if !got.Found() || got.Origin != PluginDetailOriginReadme || got.Markdown != "# 标题\n\n正文" || got.Document != "demo@1.0.0" {
		t.Fatalf("found 详情解错了：%+v", got)
	}
}

// TestPluginMarketReadEntryDetailNoneIsNotAnError 钉住"none 是回答不是错误"。
//
// 官方 typert 的联合里有 `{status:'none'}`（types.ts:166）。把它当错误，
// 界面就会给一个"存心没有文档"的版本画一行红色失败。
func TestPluginMarketReadEntryDetailNoneIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"status":"none"}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginMarketReadEntryDetail(context.Background(), "https://m.example/index.json", "demo", "1.0.0", "zh-CN")
	if err != nil {
		t.Fatalf("none 是合法回答，不该报错：%v", err)
	}
	if got.Status != PluginDetailNone || got.Found() {
		t.Fatalf("none 解错了：%+v", got)
	}
}

// TestPluginMarketDetailFailureOf 逐个钉住六个失败档与两条本仓码。
func TestPluginMarketDetailFailureOf(t *testing.T) {
	cases := []struct {
		name string
		body string
		want PluginMarketDetailFailure
	}{
		{"too-large", `{"error":{"code":"plugin-market/detail-unavailable","message":"大"},"details":{"reason":"too-large"}}`, PluginMarketDetailTooLarge},
		{"network", `{"error":{"code":"plugin-market/detail-unavailable","message":"网"},"details":{"reason":"network"}}`, PluginMarketDetailNetwork},
		{"denied", `{"error":{"code":"plugin-market/detail-unavailable","message":"凭据"},"details":{"reason":"denied"}}`, PluginMarketDetailDenied},
		{"git-unavailable", `{"error":{"code":"plugin-market/detail-unavailable","message":"git"},"details":{"reason":"git-unavailable"}}`, PluginMarketDetailGitUnavailable},
		{"invalid", `{"error":{"code":"plugin-market/detail-unavailable","message":"坏"},"details":{"reason":"invalid"}}`, PluginMarketDetailInvalid},
		// 官方六档之外（或 reason 缺席）一律 other：官方客户端同一个判据。
		{"unknown-reason", `{"error":{"code":"plugin-market/detail-unavailable","message":"?"},"details":{"reason":"weird"}}`, PluginMarketDetailOther},
		{"no-details", `{"error":{"code":"plugin-market/detail-unavailable","message":"?"}}`, PluginMarketDetailOther},
		// 本仓宿主三种错误码都没有结构化 reason：官方档就是 other。
		{"host-detail-unavailable", `{"error":{"code":"detail_unavailable","message":"读不到"}}`, PluginMarketDetailOther},
		{"host-registry-unavailable", `{"error":{"code":"registry_unavailable","message":"没配索引源"}}`, PluginMarketDetailOther},
		// 官方另外两个错误码（index.ts:283）同样不是六个失败档。
		{"unknown-plugin", `{"error":{"code":"plugin-market/unknown-plugin","message":"没有这一版"}}`, PluginMarketDetailOther},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			_, err := New(srv.URL).PluginMarketReadEntryDetail(context.Background(), "https://m.example/index.json", "demo", "1.0.0", "zh-CN")
			if err == nil {
				t.Fatal("错误响应必须变成 error")
			}
			if got := PluginMarketDetailFailureOf(err); got != tc.want {
				t.Fatalf("失败档 = %q，想要 %q（err=%v）", got, tc.want, err)
			}
		})
	}
}

// TestPluginMarketDetailFailureOfNil 钉住"没有错误就没有失败档"。
func TestPluginMarketDetailFailureOfNil(t *testing.T) {
	if got := PluginMarketDetailFailureOf(nil); got != "" {
		t.Fatalf("nil 错误应给空档，得到 %q", got)
	}
}
