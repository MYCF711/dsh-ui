// recommendations_test.go —— 官方插件推荐卡目录通道的客户端契约测试。
//
// 判据对着宿主 `internal/recommendations/service.go:120-185` 与
// `internal/transport/handlers_plugin_recommendations_official.go:53-84`，不是猜的：
//
//	① 入参**平铺**：sessionId + callId，两个都非空才发；空串本地就拒（宿主会回 bad_request，
//	   但界面更不该发出一个"不知道问哪张卡"的请求）；
//	② `available` 与 `refused` 是**判别联合**：refused 只有 reason，不许被解成"标题为空的 available"；
//	③ 未知 status **报错**，不猜（官方 Card.tsx 对未知分支同样不画可用态）。
package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPluginRecommendationDescribeSendsFlatScopedParams(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pluginRecommendations.describe" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{"status":"available","title":"Audit","description":"安全审计","enabled":false}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginRecommendationDescribe(context.Background(), "s-1", "call-1")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if body["sessionId"] != "s-1" || body["callId"] != "call-1" {
		t.Fatalf("入参必须平铺 sessionId/callId：%v", body)
	}
	if _, wrapped := body["params"]; wrapped {
		t.Fatalf("不许套 params 信封：%v", body)
	}
	if !got.Available() || got.Title != "Audit" || got.Description != "安全审计" || got.Enabled {
		t.Fatalf("available 详情解错了：%+v", got)
	}
}

func TestPluginRecommendationDescribeKeepsRefusedReason(t *testing.T) {
	for _, reason := range []string{"unknown", "unavailable", "access-denied", "not-live-root", "closing"} {
		reason := reason
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"result":{"status":"refused","reason":"` + reason + `"}}`))
		}))
		got, err := New(srv.URL).PluginRecommendationDescribe(context.Background(), "s-1", "call-1")
		srv.Close()
		if err != nil {
			t.Fatalf("%s: 应当成功返回 refused，而不是错误：%v", reason, err)
		}
		if got.Available() {
			t.Fatalf("%s: refused 被当成了 available", reason)
		}
		if got.Reason != reason {
			t.Fatalf("reason 丢了：想要 %q，得到 %q", reason, got.Reason)
		}
		if got.Title != "" {
			t.Fatalf("refused 不该有标题：%+v", got)
		}
	}
}

// TestPluginRecommendationDescribeRejectsUnknownStatus 钉住"未知 status 不猜"。
//
// 官方对未知分支不画可用态；本仓把它当**解码错误**冒上来——静默当成 refused
// 会让人以为"这个插件不可用"，而真相是宿主和客户端对契约的理解已经分叉了。
func TestPluginRecommendationDescribeRejectsUnknownStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"status":"maybe","title":"x"}}`))
	}))
	defer srv.Close()
	if _, err := New(srv.URL).PluginRecommendationDescribe(context.Background(), "s-1", "call-1"); err == nil {
		t.Fatal("未知 status 必须报错")
	}
}

// TestPluginRecommendationDescribeRefusesEmptyScope 钉住"空 sessionId/callId 不发请求"。
func TestPluginRecommendationDescribeRefusesEmptyScope(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		_, _ = w.Write([]byte(`{"result":{"status":"refused","reason":"unknown"}}`))
	}))
	defer srv.Close()
	cli := New(srv.URL)
	if _, err := cli.PluginRecommendationDescribe(context.Background(), "", "call-1"); err == nil {
		t.Fatal("空 sessionId 必须本地拒绝")
	}
	if _, err := cli.PluginRecommendationDescribe(context.Background(), "s-1", ""); err == nil {
		t.Fatal("空 callId 必须本地拒绝")
	}
	if hit {
		t.Fatal("本地拒绝时不该发请求")
	}
}
