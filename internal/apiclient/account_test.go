// account_test.go —— `account.getState` 的客户端契约测试（本波新增的账号数据面）。
//
// 三条判据：
//   - 路径是 `/api/account.getState`，**入参是空对象**（宿主 p.finish() 只做完结校验）；
//   - 结果是**平铺的 AccountView**（不是 `{state:…}`）——写错这一层，
//     界面会永远读不到 status，于是通知入口永远隐藏（一种不报错的"好了但没显示"）；
//   - 只有 `credential-stored` 算已登录：空串/未知值不许被当成登录。
package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAccountStateWireShape(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"status":"credential-stored","source":"local","links":{},"attempt":null}`))
	}))
	defer srv.Close()

	view, err := New(srv.URL).GetAccountState(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/account.getState" {
		t.Fatalf("路径应为 /api/account.getState，得到 %q", gotPath)
	}
	if len(body) != 0 {
		t.Fatalf("入参应是空对象（宿主不读字段），实际 %v", body)
	}
	if view == nil || view.Status != "credential-stored" {
		t.Fatalf("结果应是平铺 AccountView{status:…}，得到 %+v", view)
	}
	if view.Source != "local" {
		t.Fatalf("source 应原样透出，得到 %q", view.Source)
	}
	if !view.SignedIn() {
		t.Fatal("credential-stored 必须判为已登录")
	}
}

func TestAccountViewOnlyCredentialStoredIsSignedIn(t *testing.T) {
	for _, status := range []string{"signed-out", "", "unknown", "CREDENTIAL-STORED"} {
		view := AccountView{Status: status}
		if view.SignedIn() {
			t.Fatalf("status=%q 不该判为已登录（只有 credential-stored 算）", status)
		}
	}
}

func TestGetAccountStateNotImplementedIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"宿主没有账号状态机","hint":"","retryable":false}}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL).GetAccountState(context.Background()); err == nil {
		t.Fatal("宿主没实现时必须报错（壳据此按“未知”处理，而不是“未登录”）")
	}
}
