// delete_test.go —— `session.delete` 的客户端自测。
//
// 为什么单独一个文件：这是**破坏性**动作，错的方向只有一个——"以为删了，其实没删"
// 或者"以为没删，其实删了"。所以这里既验发出去的形状，也验**返回值的每种走法**。
package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDeleteSendsBothKeys 钉住请求体：宿主同时认 `sessionId` 和 `id`
// （handlers_extra.go 自己写了"界面那边两种写法都有"）。只发一种就对不上，
// 而少发一个键的代价是 400 —— 多发一个没有任何代价。
func TestDeleteSendsBothKeys(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session.delete" {
			t.Errorf("打错了端点：%s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"result":{"deleted":true}}`))
	}))
	defer srv.Close()

	if err := New(srv.URL).Delete(context.Background(), "s-42"); err != nil {
		t.Fatalf("删成功不该报错：%v", err)
	}
	if got["sessionId"] != "s-42" || got["id"] != "s-42" {
		t.Fatalf("两个键都要发：%v", got)
	}
}

// TestDeleteRejectsEmptyID：不给 id 就别发请求。
// 让宿主去回 400 也算能删失败，但那是让用户等一个来回才看到"缺 id"。
func TestDeleteRejectsEmptyID(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{"result":{"deleted":true}}`))
	}))
	defer srv.Close()

	if err := New(srv.URL).Delete(context.Background(), ""); err == nil {
		t.Fatal("空 id 应当报错")
	}
	if called {
		t.Error("空 id 不该真的发出请求")
	}
}

// TestDeleteErrorsPropagate 是最要紧的一条：**失败必须报出来**。
//
// 宿主把三种情况分得很清楚（handlers_extra.go）：404 = 会话不存在（换个 id 重试），
// 500 = 这次没删掉（可以再试）。客户端不能把它们都吞掉——吞掉之后，
// 界面上"已删除"的提示就是假的。
func TestDeleteErrorsPropagate(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"会话不存在", http.StatusNotFound, `{"error":{"code":"no_such_session","message":"会话不存在"}}`},
		{"删失败", http.StatusInternalServerError, `{"error":{"code":"internal","message":"删除失败：磁盘只读"}}`},
		// 200 但没有 deleted:true —— 契约上不该出现，真出现时**不能当成功**。
		{"200 但没有 deleted", http.StatusOK, `{"result":{}}`},
		{"200 但 deleted=false", http.StatusOK, `{"result":{"deleted":false}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			if err := New(srv.URL).Delete(context.Background(), "s-1"); err == nil {
				t.Fatalf("应当报错（status=%d body=%s）", tc.status, tc.body)
			}
		})
	}
}
