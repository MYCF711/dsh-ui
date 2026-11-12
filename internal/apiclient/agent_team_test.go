// agent_team_test.go —— 官方 rc.2 `agentTeams/setEnabled` 的客户端契约测试。
//
// 三个判据（都不是猜的）：
//   - 参数 wire 名是 `agentId`（官方 source=lookup 的 wire 名），不是 sessionId；
//   - 结果是空对象 `{}`（官方 z.void()，本仓出口不接受 result:null）；
//   - 非根会话回 403 `team_lead_required`——这是**稳定业务码**，不能被界面
//     塌成"服务器错误"（否则用户不知道该去根会话改）。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetAgentTeamEnabledSendsWireNames(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	if err := New(srv.URL).SetAgentTeamEnabled(context.Background(), "root-session", true); err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/agentTeams.setEnabled" {
		t.Fatalf("路径应为 /api/agentTeams.setEnabled，得到 %q", gotPath)
	}
	if body["agentId"] != "root-session" {
		t.Fatalf("wire 名必须是 agentId（官方 source=lookup），得到 %v", body)
	}
	if enabled, ok := body["enabled"].(bool); !ok || !enabled {
		t.Fatalf("enabled 必须是 boolean true，得到 %v", body["enabled"])
	}
}

func TestSetAgentTeamEnabledLeadRequiredIsStableCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"team_lead_required","message":"change Team mode from the root Session","hint":"去根会话改","retryable":false}}`))
	}))
	defer srv.Close()

	err := New(srv.URL).SetAgentTeamEnabled(context.Background(), "child-session", true)
	if err == nil {
		t.Fatal("非根会话必须报错")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("应该是 *APIError，得到 %T: %v", err, err)
	}
	if apiErr.Code != TeamLeadRequiredCode {
		t.Fatalf("code 应为 %q，得到 %q", TeamLeadRequiredCode, apiErr.Code)
	}
}

func TestSetAgentTeamEnabledOffIsNotOmitted(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	if err := New(srv.URL).SetAgentTeamEnabled(context.Background(), "root", false); err != nil {
		t.Fatalf("关闭也必须能发：%v", err)
	}
	if enabled, ok := body["enabled"].(bool); !ok || enabled {
		t.Fatalf("enabled=false 必须显式发出（宿主缺 enabled 会 invalid_arg），得到 %v", body["enabled"])
	}
}
