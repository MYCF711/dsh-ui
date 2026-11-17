package apiclient

// session_create_test.go —— 官方 `session.create` 客户端的夹具。
//
// 两类断言，缺一不可：
//
//  1. **发出的裸参数**：官方组合规则（`manager.ts:498-504`）是"workspaceId 给了就
//     连 cwd 都不发"，这条只能在 params() 上钉，不能靠"宿主没报错"倒推。
//  2. **读回的结果/错误**：结果只有 `{sessionId, agentPreset?}`（`types.ts:279-284`），
//     错误里 `session/workspace-attach-failed` 的 `details.sessionId` 是"已发布未挂上"
//     那份会话的唯一线索（`manager.ts:855-857`）。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
)

// TestSessionCreateParamsWorkspaceIDWinsOverCwd 钉官方互斥组合：
// workspaceId 与 cwd 同时给时**只发 workspaceId**（客户端就不发 cwd，
// 宿主那边的 gateway/bad-request 永远不会被触发）。
func TestSessionCreateParamsWorkspaceIDWinsOverCwd(t *testing.T) {
	got := SessionCreateRequest{
		WorkspaceID: "ws-a",
		Cwd:         `D:\work`,
		SessionID:   "blank-1",
		AgentPreset: "read-only",
	}.params()
	want := map[string]any{
		"workspaceId": "ws-a",
		"sessionId":   "blank-1",
		"agentPreset": "read-only",
		// 旧名字：与本仓已发布宿主的 `mode` 对齐，见 session_create.go 文件头。
		"mode": "read-only",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("params = %#v，want %#v", got, want)
	}
	if _, exists := got["cwd"]; exists {
		t.Fatal("workspaceId 给了就不该发 cwd")
	}
}

// TestSessionCreateParamsCwdAlone：没有工作区时发官方 cwd（会话自己拥有该目录）。
func TestSessionCreateParamsCwdAlone(t *testing.T) {
	got := SessionCreateRequest{Cwd: `D:\work`}.params()
	if !reflect.DeepEqual(got, map[string]any{"cwd": `D:\work`}) {
		t.Fatalf("params = %#v", got)
	}
}

// TestSessionCreateParamsEmptySendsNothing：零值 = 一个字段都不发。
//
// 官方判据是 `!== undefined`（给了空串与没给是两件事），所以零值必须是**空 map**，
// 而不是 `{"cwd":""}` 或 `{"agentPreset":""}`。
func TestSessionCreateParamsEmptySendsNothing(t *testing.T) {
	if got := (SessionCreateRequest{}).params(); len(got) != 0 {
		t.Fatalf("零值不该发出任何字段，实际 %#v", got)
	}
}

// TestSessionCreateNeverSendsTitle 钉"title 删掉了"：官方 SessionCreateRequest 只有
// 四个字段，宿主也从来没解过 title（它一直是空转字段）。
func TestSessionCreateNeverSendsTitle(t *testing.T) {
	var mu sync.Mutex
	var body map[string]any
	srv := newRecordingServer(t, &mu, &body, http.StatusOK,
		`{"result":{"sessionId":"s1"}}`)
	_, err := New(srv.URL).SessionCreate(context.Background(), SessionCreateRequest{Cwd: `D:\work`})
	if err != nil {
		t.Fatalf("SessionCreate: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, exists := body["title"]; exists {
		t.Fatalf("官方没有 title，不许再发：%#v", body)
	}
	if body["cwd"] != `D:\work` {
		t.Fatalf("cwd 没发出去：%#v", body)
	}
}

// TestSessionCreateResultShape：成功结果只认官方两个键。
func TestSessionCreateResultShape(t *testing.T) {
	srv := serveRaw(t, http.StatusOK, `{"result":{"sessionId":"s-9","agentPreset":"cordis"}}`)
	got, err := New(srv.URL).SessionCreate(context.Background(), SessionCreateRequest{AgentPreset: "cordis"})
	if err != nil {
		t.Fatalf("SessionCreate: %v", err)
	}
	if got.SessionID != "s-9" || got.AgentPreset != "cordis" {
		t.Fatalf("结果 = %#v", got)
	}
}

// TestSessionCreateOmitsAbsentAgentPreset：宿主不知道 preset 时**不出现**这个键，
// 客户端读成空串（不是"叫空串的 preset"），界面据此整段不显示。
func TestSessionCreateOmitsAbsentAgentPreset(t *testing.T) {
	srv := serveRaw(t, http.StatusOK, `{"result":{"sessionId":"s-9"}}`)
	got, err := New(srv.URL).SessionCreate(context.Background(), SessionCreateRequest{})
	if err != nil {
		t.Fatalf("SessionCreate: %v", err)
	}
	if got.AgentPreset != "" {
		t.Fatalf("没有 agentPreset 键时应是空串，实际 %q", got.AgentPreset)
	}
}

// TestSessionCreateEmptySessionIDIsLoud：成功却没有 id 不能当成功。
func TestSessionCreateEmptySessionIDIsLoud(t *testing.T) {
	srv := serveRaw(t, http.StatusOK, `{"result":{}}`)
	if _, err := New(srv.URL).SessionCreate(context.Background(), SessionCreateRequest{}); err == nil {
		t.Fatal("空 sessionId 必须报错（拿它去 load 只会变成 no_such_session）")
	}
}

// TestAttachFailedSessionIDBothWireShapes 钉两套错误形状都能取到那份会话 id：
//
//   - 官方线：`error.code` 本身就是 `session/workspace-attach-failed`；
//   - 本仓宿主：四字段 code 是闭合集合，官方码在 message 前缀里
//     （`session/workspace-attach-failed: …`，`HOST:docs/transport-contract.md` §18）。
func TestAttachFailedSessionIDBothWireShapes(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "官方 code 字段",
			body: `{"error":{"code":"session/workspace-attach-failed","message":"attach failed"},` +
				`"details":{"sessionId":"published-1","workspaceId":"ws-a"}}`,
			want: "published-1",
		},
		{
			name: "本仓宿主：code=internal + message 前缀",
			body: `{"error":{"code":"internal","message":"session/workspace-attach-failed: 挂不上"},` +
				`"details":{"sessionId":"published-2","workspaceId":"ws-a"}}`,
			want: "published-2",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := serveRaw(t, http.StatusInternalServerError, c.body)
			_, err := New(srv.URL).SessionCreate(context.Background(), SessionCreateRequest{WorkspaceID: "ws-a"})
			if err == nil {
				t.Fatal("错误体应作为失败返回")
			}
			id, ok := AttachFailedSessionID(err)
			if !ok || id != c.want {
				t.Fatalf("AttachFailedSessionID = %q,%v，want %q", id, ok, c.want)
			}
		})
	}
}

// TestAttachFailedSessionIDIgnoresOtherCodes：别的错误即使带了 details.sessionId
// 也不算——那是另一件事的字段（官方只认 `code === 'session/workspace-attach-failed'`）。
func TestAttachFailedSessionIDIgnoresOtherCodes(t *testing.T) {
	srv := serveRaw(t, http.StatusBadRequest, `{"error":{"code":"bad_request","message":"nope"},`+
		`"details":{"sessionId":"other-1"}}`)
	_, err := New(srv.URL).SessionCreate(context.Background(), SessionCreateRequest{})
	if err == nil {
		t.Fatal("400 应作为失败返回")
	}
	if id, ok := AttachFailedSessionID(err); ok {
		t.Fatalf("非 attach-failed 不该取出会话 id，实际 %q", id)
	}
}

// TestWorkspaceRegistryUnsupported：只有"注册表没接线"这一类才算，其余失败不算。
func TestWorkspaceRegistryUnsupported(t *testing.T) {
	unwired := serveRaw(t, http.StatusNotImplemented,
		`{"error":{"code":"not_implemented","message":"workspace.create 未接线"}}`)
	_, err := New(unwired.URL).WorkspaceCreate(context.Background(), `D:\p`)
	if !WorkspaceRegistryUnsupported(err) {
		t.Fatalf("501 not_implemented 应判为注册表不可用，实际 %v", err)
	}
	badPath := serveRaw(t, http.StatusBadRequest, `{"error":{"code":"bad_request","message":"path 不是绝对路径"}}`)
	_, err = New(badPath.URL).WorkspaceCreate(context.Background(), `D:\p`)
	if err == nil || WorkspaceRegistryUnsupported(err) {
		t.Fatalf("bad_request 是这次路径的问题，不能判成注册表不可用：%v", err)
	}
}

// newRecordingServer 起一个记下请求体的宿主替身（只回答一次固定响应）。
func newRecordingServer(t *testing.T, mu *sync.Mutex, body *map[string]any, status int, resp string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		mu.Lock()
		*body = in
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if status >= 400 {
			w.WriteHeader(status)
		}
		_, _ = w.Write([]byte(resp))
	}))
	t.Cleanup(srv.Close)
	return srv
}
