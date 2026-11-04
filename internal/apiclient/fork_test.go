// fork_test.go —— `session.fork` 的客户端自测。
//
// 这条容易错在两个地方：
//  1. 它是**整份复制**，不是"从某条消息往后"——契约里没有切点参数，
//     谁在界面上画成"从这里分支"，用户就会以为后半段被丢了；
//  2. 200 但没有新 id 时**不能当成功**——调用方会去开一个空 id 的会话，
//     界面上看起来像"分支出来了但打不开"。
package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestForkSendsBothKeysAndReturnsNewID(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session.fork" {
			t.Errorf("打错了端点：%s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"result":{"sessionId":"session-new"}}`))
	}))
	defer srv.Close()

	id, err := New(srv.URL).Fork(context.Background(), "s-1", "")
	if err != nil {
		t.Fatalf("分支不该报错：%v", err)
	}
	if id != "session-new" {
		t.Fatalf("新会话 id = %q，want session-new", id)
	}
	if got["sessionId"] != "s-1" || got["id"] != "s-1" {
		t.Fatalf("两个键都要发：%v", got)
	}
	// 没给标题就不发 title：宿主 `serveFork` 目前**不消费**它，
	// 发一个没人看的字段只会让下一个人以为"改标题走这里"。
	if _, ok := got["title"]; ok {
		t.Errorf("title 为空时不该发出去，实际 %v", got["title"])
	}
}

func TestForkSendsTitleWhenGiven(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"result":{"id":"n2"}}`))
	}))
	defer srv.Close()

	id, err := New(srv.URL).Fork(context.Background(), "s-1", "实验分支")
	if err != nil {
		t.Fatalf("不该报错：%v", err)
	}
	if id != "n2" {
		t.Fatalf("宿主只回 id 时也应当认：%q", id)
	}
	if got["title"] != "实验分支" {
		t.Fatalf("给了标题就要发：%v", got["title"])
	}
}

func TestForkRejectsSuccessWithoutID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL).Fork(context.Background(), "s-1", ""); err == nil {
		t.Fatal("200 但没有新 id 不能当成功——调用方会去开一个空 id 的会话")
	}
}

func TestForkErrorsPropagate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
	}{
		{"会话不存在", http.StatusNotFound},
		{"分支失败", http.StatusInternalServerError},
		{"宿主没实现", http.StatusNotImplemented},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"error":{"code":"x","message":"x"}}`))
			}))
			defer srv.Close()
			if _, err := New(srv.URL).Fork(context.Background(), "s-1", ""); err == nil {
				t.Fatalf("应当报错（status=%d）", tc.status)
			}
		})
	}
}

func TestForkRejectsEmptyID(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{"result":{"sessionId":"x"}}`))
	}))
	defer srv.Close()
	if _, err := New(srv.URL).Fork(context.Background(), "", ""); err == nil {
		t.Fatal("空 id 应当报错")
	}
	if called {
		t.Error("空 id 不该真的发出请求")
	}
}
