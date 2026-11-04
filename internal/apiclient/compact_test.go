// compact_test.go —— `session.compact` 的客户端自测。
//
// 这一条和删除不一样：它不是破坏性的（v4 日志不改写），但**会调一次模型**。
// 所以两种错都很贵：把失败当成功（用户以为压了）、或者把"没压"当失败
// （用户以为坏了，再点一次 → 再调一次模型）。
package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompactSendsBothKeysAndParsesResult(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session.compact" {
			t.Errorf("打错了端点：%s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"result":{"compacted":true,"from":3,"to":40,"shadowed":37,"summaryChars":812}}`))
	}))
	defer srv.Close()

	res, err := New(srv.URL).Compact(context.Background(), "s-7", 0)
	if err != nil {
		t.Fatalf("压缩不该报错：%v", err)
	}
	if got["sessionId"] != "s-7" || got["id"] != "s-7" {
		t.Fatalf("两个键都要发：%v", got)
	}
	// keep 传 0 时**不发**这个键：宿主自己会填默认 6。
	// 两边各写一个默认值，分叉的表现是"界面以为保留 6 条，实际保留了别的数"。
	if _, ok := got["keep"]; ok {
		t.Errorf("keep=0 不该发出去（让宿主用它的默认值），实际 %v", got["keep"])
	}
	if !res.Compacted || res.Shadowed != 37 || res.SummaryChars != 812 || res.From != 3 || res.To != 40 {
		t.Fatalf("结果没解全：%+v", res)
	}
}

func TestCompactSendsKeepWhenGiven(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"result":{"compacted":false}}`))
	}))
	defer srv.Close()

	res, err := New(srv.URL).Compact(context.Background(), "s-7", 12)
	if err != nil {
		t.Fatalf("压缩不该报错：%v", err)
	}
	if int(got["keep"].(float64)) != 12 {
		t.Fatalf("keep 应当原样发出去，实际 %v", got["keep"])
	}
	// **没压也是一个正常结局**：内容太少时宿主回 compacted=false。
	// 客户端不许把它变成 error —— 那会让界面报"失败"，用户再点一次（又调一次模型）。
	if res.Compacted {
		t.Error("结果是 compacted=false，不该解成 true")
	}
}

func TestCompactErrorsPropagate(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"会话不存在", http.StatusNotFound, `{"error":{"code":"no_such_session","message":"会话不存在"}}`},
		{"压缩失败", http.StatusInternalServerError, `{"error":{"code":"internal","message":"压缩失败: 生成摘要时模型超时"}}`},
		{"宿主没实现", http.StatusNotImplemented, `{"error":{"code":"not_implemented","message":"这个宿主还没实现 session.compact"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			if _, err := New(srv.URL).Compact(context.Background(), "s-1", 0); err == nil {
				t.Fatalf("应当报错（status=%d）", tc.status)
			}
		})
	}
}

func TestCompactRejectsEmptyID(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	defer srv.Close()
	if _, err := New(srv.URL).Compact(context.Background(), "", 0); err == nil {
		t.Fatal("空 id 应当报错")
	}
	if called {
		t.Error("空 id 不该真的发出请求")
	}
}
