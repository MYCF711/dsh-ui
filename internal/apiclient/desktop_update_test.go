// desktop_update_test.go —— 「桌面更新」两个壳桥客户端的线上形状测试。
//
// 这一族是官方 `DesktopUpdateBridge`（`ui-settings-general/src/types.ts:30-35`）
// 的 `status()` / `open()`，宿主 `desktopUpdate.status` / `desktopUpdate.open`。
// 渠道族（`channel.get` / `channel.set`）是它的兄弟，本文件只覆盖状态族。
//
// 五条判据：
//   - 路径是 `/api/desktopUpdate.status` 与 `/api/desktopUpdate.open`，
//     **入参是空对象**（宿主这两个方法不收参数）；
//   - 结果是**平铺的** presentation / open result（不是 `{presentation:…}` 套一层）；
//   - `percent` 是**可选的**：宿主没有下载器，这个键不出现时不能解成"0%"；
//   - `configurationError` 必须原样透出（官方渲染层先判它，丢了它整条提示就错位）；
//   - `open` 的 `url` 原样透出——界面靠它去唤起浏览器，任何裁剪都会打开错的页。
package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDesktopUpdateStatusWireShape(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		// 真宿主 `desktopUpdate.status` 的形状（internal/updater/status.go 的
		// Presentation：phase 必有，version/percent/failure/configurationError 可选）。
		_, _ = w.Write([]byte(`{"phase":"available","version":"0.2.2-go.3"}`))
	}))
	defer srv.Close()

	pres, err := New(srv.URL).DesktopUpdateStatus(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/desktopUpdate.status" {
		t.Fatalf("路径应为 /api/desktopUpdate.status，得到 %q", gotPath)
	}
	if len(body) != 0 {
		t.Fatalf("入参应是空对象（宿主不读字段），实际 %v", body)
	}
	if pres.Phase != "available" || pres.Version != "0.2.2-go.3" {
		t.Fatalf("结果应是平铺 presentation，得到 %+v", pres)
	}
	// 没有下载器 ⇒ 宿主永远不填 percent。**缺失 ≠ 0**：解成 0 会让界面
	// 把一个不存在的下载进度画成 "0%"。
	if pres.Percent != nil {
		t.Fatalf("percent 缺失时应是 nil，实得 %v（宿主没有下载器，不该有这个键）", *pres.Percent)
	}
}

func TestDesktopUpdateStatusCarriesConfigurationError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// 官方启动时渠道文件读不出来的形状：configurationError 与
		// phase=error+failure=configuration **同时**为真（宿主报告 §1.3）。
		_, _ = w.Write([]byte(`{"phase":"error","failure":"configuration","configurationError":"read"}`))
	}))
	defer srv.Close()

	pres, err := New(srv.URL).DesktopUpdateStatus(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if pres.ConfigurationError != "read" {
		t.Fatalf("configurationError = %q，应为 %q（官方渲染层先判它，丢了整条提示就错位）",
			pres.ConfigurationError, "read")
	}
	if pres.Failure != "configuration" {
		t.Fatalf("failure = %q，应为 configuration", pres.Failure)
	}
}

func TestDesktopUpdateStatusNotImplementedIsError(t *testing.T) {
	// 宿主没接线 -> 501。调用方据此判"这条能力不存在"（= 官方根本没有 updates 桥），
	// **不能**当成 phase=error。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"未接线","hint":"","retryable":false}}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL).DesktopUpdateStatus(context.Background()); err == nil {
		t.Fatal("501 必须返回错误，不能当成一个空 presentation")
	}
}

func TestDesktopUpdateOpenWireShape(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"url":"https://github.com/MYCF711/dsh-go/releases/tag/v0.2.2-go.3",` +
			`"channel":"stable","presentation":{"phase":"available","version":"0.2.2-go.3"}}`))
	}))
	defer srv.Close()

	result, err := New(srv.URL).DesktopUpdateOpen(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/desktopUpdate.open" {
		t.Fatalf("路径应为 /api/desktopUpdate.open，得到 %q", gotPath)
	}
	if len(body) != 0 {
		t.Fatalf("入参应是空对象（宿主不读字段），实际 %v", body)
	}
	want := "https://github.com/MYCF711/dsh-go/releases/tag/v0.2.2-go.3"
	if result.URL != want {
		t.Fatalf("url = %q，want %q（界面靠它唤起浏览器，裁剪就会打开错的页）", result.URL, want)
	}
	if result.Channel != "stable" {
		t.Fatalf("channel = %q，want stable", result.Channel)
	}
	if result.Presentation.Phase != "available" || result.Presentation.Version != "0.2.2-go.3" {
		t.Fatalf("presentation 应随 open 一起回来（界面不用再问一次），得到 %+v", result.Presentation)
	}
}

func TestDesktopUpdateOpenNotImplementedIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"未接线","hint":"","retryable":false}}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL).DesktopUpdateOpen(context.Background()); err == nil {
		t.Fatal("501 必须返回错误，不能当成一个空 URL")
	}
}
