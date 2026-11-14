// references_test.go —— `@` 候选与消息反馈客户端的 wire 契约。
//
// 每条只钉三样：路径、入参形状、回包怎么解。写错路径的后果是静默 404；
// 写错 union 形状的后果更隐蔽——业务失败被当成成功（界面显示"已记录"，
// 而宿主根本没写）。
package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFileReferencesWireShape(t *testing.T) {
	var gotPath string
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`[{"path":"plugins/dsopen","kind":"directory"},{"path":"plugins/dsopen/open.go","kind":"file"}]`))
	}))
	defer srv.Close()

	list, err := New(srv.URL).FileReferences(context.Background(), "s-1", "plugins/")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/fileReferences.list" {
		t.Fatalf("路径 = %q", gotPath)
	}
	if got["agentId"] != "s-1" || got["query"] != "plugins/" {
		t.Fatalf("入参 = %v，want {agentId,query}", got)
	}
	if len(list) != 2 || list[0].Kind != "directory" || list[1].Path != "plugins/dsopen/open.go" {
		t.Fatalf("回包没解全：%+v", list)
	}
}

func TestSessionReferenceCandidatesWireShape(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/sessionReferenceResolver.candidates" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`[{"mention":"@[上一次对话](dsh-session:s-2)","sessionId":"s-2","label":"上一次对话","sameWorkspace":true,"createdAt":1762000000000}]`))
	}))
	defer srv.Close()

	list, err := New(srv.URL).SessionReferenceCandidates(context.Background(), "s-1", "上")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if got["agentId"] != "s-1" || got["query"] != "上" {
		t.Fatalf("入参 = %v", got)
	}
	if len(list) != 1 || list[0].SessionID != "s-2" || list[0].Mention != "@[上一次对话](dsh-session:s-2)" {
		t.Fatalf("回包没解全：%+v", list)
	}
	if !list[0].SameWorkspace {
		t.Fatal("sameWorkspace 应原样透出")
	}
}

func TestMessageFeedbackListParsesUnion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/messageFeedback.list" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true,"value":{"items":[{"messageId":"v4:7","rating":"positive","category":"task-result","version":"v1","createdAt":1,"updatedAt":2}]}}`))
	}))
	defer srv.Close()

	res, err := New(srv.URL).MessageFeedbackList(context.Background(), "s-1")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if !res.OK || res.Value == nil || len(res.Value.Items) != 1 {
		t.Fatalf("union 没解出来：%+v", res)
	}
	if item := res.Value.Items[0]; item.MessageID != "v4:7" || item.Rating != "positive" || item.Version != "v1" {
		t.Fatalf("item = %+v", item)
	}
}

func TestMessageFeedbackPutConflictCarriesCurrent(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/messageFeedback.put" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"version-conflict","current":{"messageId":"v4:7","rating":"negative","version":"v2","createdAt":1,"updatedAt":3}}}`))
	}))
	defer srv.Close()

	version := "v1"
	res, err := New(srv.URL).MessageFeedbackPut(context.Background(), "s-1", "v4:7", "positive", "note", "task-result", &version)
	if err != nil {
		t.Fatalf("业务失败不是传输错误：%v", err)
	}
	if res.OK || res.Error == nil || res.Error.Code != "version-conflict" {
		t.Fatalf("union 解错：%+v", res)
	}
	req, ok := got["request"].(map[string]any)
	if !ok {
		t.Fatalf("request 参数形状不对：%v", got)
	}
	for _, key := range []string{"sessionId", "messageId", "rating", "note", "category", "ifVersion"} {
		if _, present := req[key]; !present {
			t.Errorf("request 缺 %q：%v", key, req)
		}
	}
	if req["ifVersion"] != "v1" {
		t.Errorf("ifVersion = %v，want v1", req["ifVersion"])
	}
	current, present := res.Error.FeedbackCurrent()
	if !present || current == nil || current.Version != "v2" {
		t.Fatalf("current 没解出来：present=%v current=%+v", present, current)
	}
}

func TestMessageFeedbackPutNilVersionIsExplicitNull(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"ok":true,"value":{"messageId":"v4:1","rating":"positive","version":"v1","createdAt":1,"updatedAt":1}}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL).MessageFeedbackPut(context.Background(), "s-1", "v4:1", "positive", "", "", nil); err != nil {
		t.Fatal(err)
	}
	req := got["request"].(map[string]any)
	if v, present := req["ifVersion"]; !present || v != nil {
		t.Fatalf("ifVersion 必须显式发 null（缺键会被宿主 400），实际 %v（present=%v）", v, present)
	}
	if _, present := req["note"]; present {
		t.Error("空 note 不该发（官方空草稿省略 note）")
	}
}

func TestMessageFeedbackDeleteWireShape(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/messageFeedback.delete" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"ok":true,"value":{"absent":true}}`))
	}))
	defer srv.Close()

	res, err := New(srv.URL).MessageFeedbackDelete(context.Background(), "s-1", "v4:7", "v3")
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || res.Value == nil || !res.Value.Absent {
		t.Fatalf("delete 成功态解错：%+v", res)
	}
	req := got["request"].(map[string]any)
	if req["sessionId"] != "s-1" || req["messageId"] != "v4:7" || req["ifVersion"] != "v3" {
		t.Fatalf("request = %v", req)
	}
}
