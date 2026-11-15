// market_detail_test.go —— 壳侧 `pluginMarket.readEntryDetail` 数据面的**真 HTTP**测试。
//
// 判据跟着官方与宿主两份原件走：
//
//	入参四个位置参数   HOST:internal/transport/handlers_plugin_market_official.go:53-63
//	结果判别联合       OFF:../dsh-plugin-market/lib/typert.host.js:30-38
//	失败档六值         OFF:../dsh-plugin-market/src/types.ts:26-30
//
// 这里验的是**壳这一层的行为**：缓存（同一次读只发一次请求）、失败可重试、失败档
// 是官方六档而不是自由文本、连接重置后作废重来。
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// marketDetailRequest 是一次测试里的标准读身份。
const (
	marketTestURL     = "https://m.example/index.json"
	marketTestName    = "demo"
	marketTestVersion = "1.0.0"
	marketTestLang    = "zh-CN"
)

func TestMarketEntryDetailCachesOneRead(t *testing.T) {
	var mu sync.Mutex
	hits := 0
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pluginMarket.readEntryDetail" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		mu.Lock()
		hits++
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Unlock()
		_, _ = w.Write([]byte(`{"result":{"status":"found","origin":"readme","markdown":"# 标题","document":"demo@1.0.0"}}`))
	}))
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL)}
	// 首帧：缓存没有 → loaded=false（插件据此画"正在读取详细介绍"）。
	if _, loaded, failure := m.marketEntryDetail(marketTestURL, marketTestName, marketTestVersion, marketTestLang); loaded || failure != "" {
		t.Fatalf("首帧不该直接给出结果（loaded=%v failure=%q）", loaded, failure)
	}
	waitFor(t, "详情到位", func() bool {
		_, loaded, _ := m.marketEntryDetail(marketTestURL, marketTestName, marketTestVersion, marketTestLang)
		return loaded
	})
	detail, loaded, failure := m.marketEntryDetail(marketTestURL, marketTestName, marketTestVersion, marketTestLang)
	if !loaded || failure != "" || !detail.Found() || detail.Markdown != "# 标题" || detail.Document != "demo@1.0.0" {
		t.Fatalf("详情解错了：%+v（loaded=%v failure=%q）", detail, loaded, failure)
	}
	// 入参必须是平铺的四个键（宿主 handler 就解这四个）。
	mu.Lock()
	defer mu.Unlock()
	want := map[string]any{"url": marketTestURL, "name": marketTestName, "version": marketTestVersion, "language": marketTestLang}
	for key, value := range want {
		if body[key] != value {
			t.Fatalf("入参 %q = %v，想要 %v（全量 %v）", key, body[key], value, body)
		}
	}
	if hits != 1 {
		t.Fatalf("同一次读只该发一次请求，实际 %d 次", hits)
	}
}

// TestMarketEntryDetailFailureIsOfficialKind 钉住"失败给的是官方六档，不是错误串"。
func TestMarketEntryDetailFailureIsOfficialKind(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"code":"detail_unavailable","message":"读不到"}}`))
	}))
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL)}
	m.marketEntryDetail(marketTestURL, marketTestName, marketTestVersion, marketTestLang)
	waitFor(t, "失败档", func() bool {
		_, _, failure := m.marketEntryDetail(marketTestURL, marketTestName, marketTestVersion, marketTestLang)
		return failure != ""
	})
	_, loaded, failure := m.marketEntryDetail(marketTestURL, marketTestName, marketTestVersion, marketTestLang)
	if loaded {
		t.Fatal("失败时 loaded 必须为 false")
	}
	// 本仓宿主的 detail_unavailable 没有结构化 reason，官方档就是 other
	// （与官方客户端 market-store.ts:142 同一判据）。
	if failure != apiclient.PluginMarketDetailOther {
		t.Fatalf("失败档 = %q，想要 %q", failure, apiclient.PluginMarketDetailOther)
	}
}

// TestMarketEntryDetailRetryRereads 钉住重试真的再发一次请求，且成功后失败档清空。
func TestMarketEntryDetailRetryRereads(t *testing.T) {
	var mu sync.Mutex
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		first := hits == 1
		mu.Unlock()
		if first {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"code":"index_unavailable","message":"索引挂了"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":{"status":"found","origin":"readme","markdown":"第二次读到了"}}`))
	}))
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL)}
	m.marketEntryDetail(marketTestURL, marketTestName, marketTestVersion, marketTestLang)
	waitFor(t, "第一次失败", func() bool {
		_, _, failure := m.marketEntryDetail(marketTestURL, marketTestName, marketTestVersion, marketTestLang)
		return failure != ""
	})
	m.marketEntryDetailRetry(marketTestURL, marketTestName, marketTestVersion, marketTestLang)
	waitFor(t, "重试成功", func() bool {
		detail, loaded, failure := m.marketEntryDetail(marketTestURL, marketTestName, marketTestVersion, marketTestLang)
		return loaded && failure == "" && detail.Markdown == "第二次读到了"
	})
	mu.Lock()
	defer mu.Unlock()
	if hits != 2 {
		t.Fatalf("重试应当再发一次请求（共 2 次），实际 %d 次", hits)
	}
}
