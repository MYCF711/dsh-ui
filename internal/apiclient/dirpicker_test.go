// dirpicker_test.go —— directoryPicker **组合二选一** 的客户端判据。
//
// 官方两个后端互斥，客户端靠"打只读动词问出组合"来选界面；判据由宿主 seam 定
// （`HOST:internal/transport/handlers_workspace_official.go` 的
// `directoryPickerUnavailable`）：
//
//	directoryPicker.list → 200                                ⇒ browse
//	                     → 501 + error.code=not_implemented
//	                       + message 前缀 directory-picker/unavailable:
//	                       + 信封顶层 details.capability=native ⇒ native
//	                     → 其它                                  ⇒ 失败（不选任何一套）
//
// 这里钉四件事：
//  1. 200 ⇒ browse（且载荷得真是目录列表，坏宿主不算 browse）；
//  2. 官方那条 501 ⇒ native（三条线缺一条都不认）；
//  3. 其它一切 ⇒ **错误**，不静默退化成某一套；
//  4. `pick` 的 null 是"取消"（不是错误），且这条请求**不带 8s 整体超时**。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// seamHost 是一个只回答 directoryPicker 三个动词的假宿主。
type seamHost struct {
	// kind 是本次组合：browse 时 list/createDirectory 200、pick 501；
	// native 时反过来。
	kind string
	// listing 是 browse 组合下 list 的响应体（默认给一份像样的）。
	listing string
	// pickBody 是 native 组合下 pick 的响应体。
	pickBody string
	// hits 记录收到的方法名（按到达顺序），用于断言"探针只打 list"。
	hits []string
}

// seamRefusalBody 是宿主 seam 拒绝的信封，逐字照
// `HOST:internal/transport/handlers_workspace_official.go` 的 `directoryPickerUnavailable`
// 造（HTTP 501 + code=not_implemented + message 前缀 + 顶层 details.capability）。
func seamRefusalBody(method, need, served string) string {
	msg := "directory-picker/unavailable: directoryPicker." + method +
		" needs the " + need + " capability; the composed picker serves \"" + served + "\""
	body := map[string]any{
		"error": map[string]any{
			"code": "not_implemented", "message": msg, "hint": "", "retryable": false,
		},
		"details": map[string]any{"capability": served},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func (h *seamHost) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, "/api/")
		h.hits = append(h.hits, method)
		w.Header().Set("Content-Type", "application/json")
		browse := h.kind == DirectoryPickerKindBrowse.String()
		switch method {
		case "directoryPicker.list":
			if browse {
				body := h.listing
				if body == "" {
					body = `{"result":{"path":"C:\\Users\\u","home":"C:\\Users\\u","crumbs":[],"entries":[],"truncated":false}}`
				}
				_, _ = w.Write([]byte(body))
				return
			}
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte(seamRefusalBody("list", "browse", h.kind)))
		case "directoryPicker.createDirectory":
			if browse {
				_, _ = w.Write([]byte(`{"result":"C:\\Users\\u\\new"}`))
				return
			}
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte(seamRefusalBody("createDirectory", "browse", h.kind)))
		case "directoryPicker.pick":
			if browse {
				w.WriteHeader(http.StatusNotImplemented)
				_, _ = w.Write([]byte(seamRefusalBody("pick", "native", h.kind)))
				return
			}
			body := h.pickBody
			if body == "" {
				body = `{"result":"C:\\picked"}`
			}
			_, _ = w.Write([]byte(body))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"unsupported","message":"未知 method"}}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestProbeDirectoryPickerBrowseOn200：list 200 ⇒ browse，且**只打了 list 一条**
// （探针不许用 pick —— 那会真弹原生框）。
func TestProbeDirectoryPickerBrowseOn200(t *testing.T) {
	host := &seamHost{kind: DirectoryPickerKindBrowse.String()}
	srv := host.serve(t)

	kind, err := New(srv.URL).ProbeDirectoryPicker(context.Background())
	if err != nil {
		t.Fatalf("browse 组合应当探得出来：%v", err)
	}
	if kind != DirectoryPickerKindBrowse {
		t.Fatalf("探到 %q，期望 browse", kind)
	}
	if len(host.hits) != 1 || host.hits[0] != "directoryPicker.list" {
		t.Fatalf("探针只该打只读的 directoryPicker.list，实际打了 %v", host.hits)
	}
}

// TestProbeDirectoryPickerNativeOnSeamRefusal：官方那条 501 ⇒ native。
func TestProbeDirectoryPickerNativeOnSeamRefusal(t *testing.T) {
	host := &seamHost{kind: DirectoryPickerKindNative.String()}
	srv := host.serve(t)

	kind, err := New(srv.URL).ProbeDirectoryPicker(context.Background())
	if err != nil {
		t.Fatalf("native 组合应当探得出来：%v", err)
	}
	if kind != DirectoryPickerKindNative {
		t.Fatalf("探到 %q，期望 native", kind)
	}
	// 探针全程不许碰 pick：那会真的弹出宿主屏幕上的原生目录框。
	for _, m := range host.hits {
		if m == "directoryPicker.pick" {
			t.Fatalf("探针调了 pick（会真弹框）：%v", host.hits)
		}
	}
}

// TestProbeDirectoryPickerRejectsIncompleteRefusal 钉住"三条线缺一条就不算"。
func TestProbeDirectoryPickerRejectsIncompleteRefusal(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{
			name:   "501 但没有 details.capability",
			status: http.StatusNotImplemented,
			body:   `{"error":{"code":"not_implemented","message":"directory-picker/unavailable: directoryPicker.list needs the browse capability","hint":"","retryable":false}}`,
		},
		{
			name:   "501 但 code 不是 not_implemented",
			status: http.StatusNotImplemented,
			body:   `{"error":{"code":"unsupported","message":"directory-picker/unavailable: x"},"details":{"capability":"native"}}`,
		},
		{
			name:   "501 但 message 没有官方前缀",
			status: http.StatusNotImplemented,
			body:   `{"error":{"code":"not_implemented","message":"宿主没接线"},"details":{"capability":"native"}}`,
		},
		{
			name:   "501 但 capability 是拼错的值",
			status: http.StatusNotImplemented,
			body:   `{"error":{"code":"not_implemented","message":"directory-picker/unavailable: x"},"details":{"capability":"auto"}}`,
		},
		{
			name:   "501 且自称服务 browse（与 200=browse 矛盾）",
			status: http.StatusNotImplemented,
			body:   seamRefusalBody("list", "native", DirectoryPickerKindBrowse.String()),
		},
		{
			// mockd / 老宿主：`unknown method` 的 404 unsupported。这里必须失败，
			// 不许当 browse（它根本不会列目录）。
			name:   "404 unsupported（mockd 那种）",
			status: http.StatusNotFound,
			body:   `{"error":{"code":"unsupported","message":"未知 method: directoryPicker.list"}}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := serveRaw(t, c.status, c.body)
			kind, err := New(srv.URL).ProbeDirectoryPicker(context.Background())
			if err == nil {
				t.Fatalf("应当失败，实际探到 %q", kind)
			}
			if kind != "" {
				t.Fatalf("失败时 kind 必须是空串（不许退化到某一套），实际 %q", kind)
			}
		})
	}
}

// TestProbeDirectoryPickerRejectsEmptyListing：200 但载荷不是目录列表 = 坏宿主，
// 不许当成 browse（否则用户点开会看到一个永远空的对话框）。
func TestProbeDirectoryPickerRejectsEmptyListing(t *testing.T) {
	for _, body := range []string{`{"result":null}`, `{"result":{}}`} {
		srv := serveRaw(t, http.StatusOK, body)
		kind, err := New(srv.URL).ProbeDirectoryPicker(context.Background())
		if err == nil {
			t.Fatalf("200 但载荷不是目录列表应当失败（body=%s），实际探到 %q", body, kind)
		}
	}
}

// TestWorkspaceCreateReturnsOfficialSelection 钉住官方 adoptDirectory 的输入：
// onPick 收的是 createWorkspace 返回的 workspaceId，不是客户端回显的请求 path。
func TestWorkspaceCreateReturnsOfficialSelection(t *testing.T) {
	srv := serveRaw(t, http.StatusOK, `{"result":{"workspace":{"workspaceId":"ws-17","path":"C:\\canonical","title":"canonical","sessionIds":["blank-1"],"createdAt":"1","updatedAt":"2"},"created":false}}`)
	workspace, err := New(srv.URL).WorkspaceCreate(context.Background(), `C:\picked`)
	if err != nil {
		t.Fatalf("合法 workspace.create 不该失败：%v", err)
	}
	if workspace.WorkspaceID != "ws-17" || workspace.Path != `C:\canonical` || len(workspace.SessionIDs) != 1 || workspace.SessionIDs[0] != "blank-1" {
		t.Fatalf("官方 workspace 返回值被丢了：%+v", workspace)
	}
}

func TestWorkspaceCreateRejectsMissingSelection(t *testing.T) {
	for _, body := range []string{
		`{"result":{"workspace":{"path":"C:\\picked"},"created":true}}`,
		`{"result":{"workspace":{"workspaceId":"ws-17"},"created":true}}`,
		`{"result":{}}`,
	} {
		srv := serveRaw(t, http.StatusOK, body)
		if workspace, err := New(srv.URL).WorkspaceCreate(context.Background(), `C:\picked`); err == nil {
			t.Fatalf("缺 workspaceId/path 不能静默成功：body=%s workspace=%+v", body, workspace)
		}
	}
}

// TestDirectoryPickPathAndCancel：官方结果类型是 `string | null`，
// null = 操作员取消（**不是错误**），空串 = 宿主坏了（是错误）。
func TestDirectoryPickPathAndCancel(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		path    string
		picked  bool
		wantErr bool
	}{
		{name: "选中一个目录", body: `{"result":"C:\\work"}`, path: `C:\work`, picked: true},
		{name: "官方 null = 取消", body: `{"result":null}`, picked: false},
		{name: "空串是坏宿主，不是取消", body: `{"result":""}`, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := serveRaw(t, http.StatusOK, c.body)
			path, picked, err := New(srv.URL).DirectoryPick(context.Background())
			if c.wantErr {
				if err == nil {
					t.Fatalf("应当报错，实际 path=%q picked=%v", path, picked)
				}
				return
			}
			if err != nil {
				t.Fatalf("不该报错：%v", err)
			}
			if path != c.path || picked != c.picked {
				t.Fatalf("拿到 (%q, %v)，期望 (%q, %v)", path, picked, c.path, c.picked)
			}
		})
	}
}

// TestDirectoryPickHasNoOverallTimeout 钉住 pick 用的是**无整体超时**的客户端副本。
//
// 为什么必须有这一条：`pick` 是人工交互，原生框开着三分钟是正常用法；`New` 给的
// 8s 整体超时会把正常操作变成一次失败（而且失败时宿主那边框还开着）。这条断言
// 直接看副本的 Timeout，不靠"睡 9 秒"那种又慢又会抖的测法。
func TestDirectoryPickHasNoOverallTimeout(t *testing.T) {
	c := New("http://127.0.0.1:1")
	if c.HTTP == nil || c.HTTP.Timeout == 0 {
		t.Fatal("前提不成立：New 应当给一个带超时的客户端")
	}
	before := c.HTTP.Timeout
	pick := c.untimed()
	if pick == c {
		t.Fatal("untimed 必须返回副本——直接改 c.HTTP.Timeout 会污染所有其它调用（含并发中的）")
	}
	if pick.HTTP.Timeout != 0 {
		t.Fatalf("pick 用的客户端 Timeout = %v，必须为 0（人工交互不能有整体超时）", pick.HTTP.Timeout)
	}
	if c.HTTP.Timeout != before {
		t.Fatalf("原客户端的 Timeout 被改了：%v → %v", before, c.HTTP.Timeout)
	}
}

// TestDirectoryPickWaitsLongerThanTransportTimeout 是上一条的行为侧证据：
// 一个回得比 8s 慢的 pick 仍然成功。用 8.2s 实测太慢，所以这里把原客户端的
// Timeout 临时调成 20ms、请求睡 60ms——同一条代码路径，验证"pick 不受整体超时约束"。
func TestDirectoryPickWaitsLongerThanTransportTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(60 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":"C:\\slow"}`))
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL)
	c.HTTP.Timeout = 20 * time.Millisecond

	if _, _, err := c.DirectoryPick(context.Background()); err != nil {
		t.Fatalf("pick 被整体超时打断了（人工交互不该有整体超时）：%v", err)
	}
	// 反向对照：同一个客户端上的普通调用**应当**被那个超时打断——否则上一条
	// 就成了"我把超时删了"的假证。
	var out DirectoryListing
	if err := c.Call(context.Background(), "directoryPicker.list", struct{}{}, &out); err == nil {
		t.Fatal("普通调用应当被 20ms 的整体超时打断（说明 untimed 只作用于 pick）")
	} else if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "Timeout") &&
		!strings.Contains(err.Error(), "超时") && !strings.Contains(err.Error(), "连不上") {
		t.Logf("普通调用的失败形态：%v（形态不固定，这里只要求它确实失败）", err)
	}
}
