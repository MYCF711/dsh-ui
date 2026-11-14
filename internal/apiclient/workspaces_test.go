package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWorkspaceRegistryViewDecodesRegistry(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/workspace.registry" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{"version":1,"defaultWorkspaceId":"w1",
			"order":["w1","w2"],
			"workspaces":[{"workspaceId":"w1","path":"D:\\work\\a","title":"A组",
			  "sessionIds":["s1","s2"],"createdAt":"2026-11-01T10:00:00+08:00",
			  "updatedAt":"2026-11-14T10:00:00+08:00"},
			 {"workspaceId":"w2","path":"D:\\work\\b","title":"B组","sessionIds":[],"createdAt":"","updatedAt":""}],
			"archivedSessionIds":["s9"],"pinnedSessionIds":["s1"]}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).WorkspaceRegistryView(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if len(body) != 0 {
		t.Fatalf("workspace.registry 必须发空对象，实际 %v", body)
	}
	if got.DefaultWorkspaceID != "w1" || len(got.Order) != 2 {
		t.Fatalf("registry 头丢了：%+v", got)
	}
	if len(got.Workspaces) != 2 || got.Workspaces[0].Title != "A组" ||
		len(got.Workspaces[0].SessionIDs) != 2 {
		t.Fatalf("workspaces 解码错：%+v", got.Workspaces)
	}
	if len(got.ArchivedSessionIDs) != 1 || got.ArchivedSessionIDs[0] != "s9" {
		t.Fatalf("archivedSessionIds 丢了：%+v", got.ArchivedSessionIDs)
	}
}

func TestWorkspaceRenameSendsIDAndTitle(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/workspace.rename" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{"workspace":{"workspaceId":"w1","path":"D:\\a","title":"新名字",
			"sessionIds":[],"createdAt":"","updatedAt":""}}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).WorkspaceRename(context.Background(), "w1", "新名字")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if body["workspaceId"] != "w1" || body["title"] != "新名字" {
		t.Fatalf("参数 = %v", body)
	}
	if got.Title != "新名字" {
		t.Fatalf("返回的 workspace = %+v", got)
	}
}

func TestWorkspaceDeleteRequiresLiteralTrue(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		ok   bool
	}{
		{"deleted", `{"result":{"deleted":true}}`, true},
		{"not-confirmed", `{"result":{"deleted":false}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&body)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			err := New(srv.URL).WorkspaceDelete(context.Background(), "w1")
			if tc.ok && err != nil {
				t.Fatalf("deleted=true 应当成功：%v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("deleted=false 是失败，不能当成功")
			}
			if body["workspaceId"] != "w1" {
				t.Fatalf("参数 = %v", body)
			}
		})
	}
}
