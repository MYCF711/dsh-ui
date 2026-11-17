package main

// file_actions_test.go —— #18 壳侧动作的钉子（cmd/app/file_actions.go）。
//
// 这一层是"点了菜单之后真的发生什么"的唯一位置：
//
//	workspaceFiles.readBytes（真 HTTP 形状）→ 另存对话框 / 下载目录 → 写盘 → 状态栏
//
// 三个替换点（fileSaveDialog / fileWriteBytes / fileHomeDir）在测试里按下，
// 所以这里验的是**完整决策链**，不是某个分支的返回值。

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/app"
)

// 官方 zh 字典里的三条文案（OFF:ui-file-actions/src/client/locales.ts:5-16）：
// 状态栏里必须出现其中一条，否则失败会被误读成成功、成功会被误读成没发生。
const (
	wantSaved      = "已保存"
	wantDownloaded = "下载完成"
)

// stubFileHooks 把三个替换点换成测试实现，并在用例结束时还原。
func stubFileHooks(t *testing.T, dialog func(string) (string, error), write func(string, []byte, os.FileMode) error, home func() (string, error)) {
	t.Helper()
	prevDialog, prevWrite, prevHome := fileSaveDialog, fileWriteBytes, fileHomeDir
	if dialog != nil {
		fileSaveDialog = dialog
	}
	if write != nil {
		fileWriteBytes = write
	}
	if home != nil {
		fileHomeDir = home
	}
	t.Cleanup(func() { fileSaveDialog, fileWriteBytes, fileHomeDir = prevDialog, prevWrite, prevHome })
}

// readBytesHost 起一个只认 `workspaceFiles.readBytes` 的假宿主，并把它收到的参数记下来。
//
// 参数形状是**契约**（宿主 DisallowUnknownFields）：sessionId + path + options 三者
// 都必须到，缺 options 真宿主会 400（HOST:handlers_workspace_official.go:1130-1135）。
func readBytesHost(t *testing.T, payload []byte, seen *map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/workspaceFiles.readBytes" {
			t.Errorf("路径 = %q，want /api/workspaceFiles.readBytes", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("请求体不是 JSON：%v", err)
		}
		if seen != nil {
			*seen = body
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"offset": 0, "data": base64.StdEncoding.EncodeToString(payload),
				"encoding": "base64", "eof": true,
				"absolutePath": `D:\ws\a.txt`, "version": "v1",
			},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSuggestedFileName(t *testing.T) {
	cases := []struct{ name, path, want string }{
		{"", `docs\a.txt`, "a.txt"},
		{"", "docs/sub/b.png", "b.png"},
		{"截图.png", "docs/a.png", "截图.png"}, // name 优先（官方 FileActionSource.name）
		{"  ", "docs/a.png", "a.png"},      // 纯空白等于没给
		{"", "", "download"},               // 两样都没有时的兜底
		{"", "docs/", "docs"},              // 尾随分隔符剥掉后，最后一段就是它
	}
	for _, tc := range cases {
		if got := suggestedFileName(tc.name, tc.path); got != tc.want {
			t.Errorf("suggestedFileName(%q, %q) = %q，want %q", tc.name, tc.path, got, tc.want)
		}
	}
}

func TestUniquePathNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(first, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := uniquePath(dir, "a.txt"); got == first {
		t.Fatalf("同名文件已存在时仍返回 %q：会覆盖用户的文件", got)
	} else if filepath.Base(got) != "a (2).txt" {
		t.Fatalf("第二个落点 = %q，want a (2).txt", filepath.Base(got))
	}
	if got := uniquePath(dir, "b.txt"); filepath.Base(got) != "b.txt" {
		t.Fatalf("不存在的名字被改了：%q", got)
	}
}

// TestSaveWorkspaceFileSaveAs 走完整链路：读字节 → 原生对话框 → 写盘 → 状态栏。
func TestSaveWorkspaceFileSaveAs(t *testing.T) {
	payload := []byte("hello bytes")
	var seen map[string]any
	srv := readBytesHost(t, payload, &seen)

	chosen := filepath.Join(t.TempDir(), "a.txt")
	var wrotePath string
	var wroteData []byte
	stubFileHooks(t,
		func(name string) (string, error) {
			if name != "a.txt" {
				t.Errorf("建议文件名 = %q，want a.txt", name)
			}
			return chosen, nil
		},
		func(p string, data []byte, _ os.FileMode) error {
			wrotePath, wroteData = p, data
			return nil
		}, nil)

	m := &model{cli: apiclient.New(srv.URL)}
	m.saveWorkspaceFile("sess-1", "docs/a.txt", "", pluginapi.FileSaveAs)

	if seen["sessionId"] != "sess-1" {
		t.Errorf("sessionId = %v，want sess-1", seen["sessionId"])
	}
	if seen["path"] != "docs/a.txt" {
		t.Errorf("path = %v，want docs/a.txt（工作区相对路径原样转发）", seen["path"])
	}
	if _, ok := seen["options"]; !ok {
		t.Error("缺 options 参数：真宿主会 400（官方该参数必填）")
	}
	if wrotePath != chosen {
		t.Errorf("写盘路径 = %q，want 对话框选的 %q", wrotePath, chosen)
	}
	if string(wroteData) != string(payload) {
		t.Errorf("写盘字节 = %q，want %q（必须是**原始字节**，不是文本预览）", wroteData, payload)
	}
	if !strings.Contains(m.status, wantSaved) {
		t.Errorf("状态栏 = %q，want 含 %q", m.status, wantSaved)
	}
}

// TestSaveWorkspaceFileDownload 走 download 模式：不问位置，落到 <home>/Downloads。
func TestSaveWorkspaceFileDownload(t *testing.T) {
	payload := []byte{0x89, 'P', 'N', 'G'} // 二进制：证明没走文本通道
	srv := readBytesHost(t, payload, nil)
	home := t.TempDir()
	stubFileHooks(t, func(string) (string, error) {
		t.Error("download 模式不该开对话框")
		return "", errors.New("不该被调用")
	}, nil, func() (string, error) { return home, nil })

	m := &model{cli: apiclient.New(srv.URL)}
	m.saveWorkspaceFile("sess-1", "pics/shot.png", "shot.png", pluginapi.FileDownload)

	written, err := os.ReadFile(filepath.Join(home, "Downloads", "shot.png"))
	if err != nil {
		t.Fatalf("下载目录里没有文件：%v", err)
	}
	if string(written) != string(payload) {
		t.Fatalf("下载内容 = %v，want %v", written, payload)
	}
	if !strings.Contains(m.status, wantDownloaded) {
		t.Errorf("状态栏 = %q，want 含 %q", m.status, wantDownloaded)
	}
}

// TestSaveWorkspaceFileCancelIsSilent 钉住"用户取消不是错误"：
// 不写盘、不写状态栏（官方 `status:'cancelled'` 也不弹 toast）。
func TestSaveWorkspaceFileCancelIsSilent(t *testing.T) {
	srv := readBytesHost(t, []byte("x"), nil)
	var wrote bool
	stubFileHooks(t, func(string) (string, error) { return "", app.ErrCanceled },
		func(string, []byte, os.FileMode) error { wrote = true; return nil }, nil)

	m := &model{cli: apiclient.New(srv.URL)}
	m.saveWorkspaceFile("sess-1", "a.txt", "a.txt", pluginapi.FileSaveAs)
	if wrote {
		t.Fatal("取消之后仍然写了盘")
	}
	if m.status != "" {
		t.Fatalf("取消写进了状态栏：%q（官方取消是静默的）", m.status)
	}
}

// TestSaveWorkspaceFileRejectsBadArguments 钉住"参数不全就别发请求、别静默成功"。
func TestSaveWorkspaceFileRejectsBadArguments(t *testing.T) {
	srv := readBytesHost(t, []byte("x"), nil)
	m := &model{cli: apiclient.New(srv.URL)}
	for _, tc := range []struct{ session, path, mode string }{
		{"", "a.txt", pluginapi.FileSaveAs},
		{"s1", "", pluginapi.FileSaveAs},
		{"s1", "a.txt", "移到别处"},
	} {
		m.status = ""
		m.saveWorkspaceFile(tc.session, tc.path, "a.txt", tc.mode)
		if !strings.Contains(m.status, zhFileSaveFailed) {
			t.Errorf("%+v 的状态 = %q，want 含 %q", tc, m.status, zhFileSaveFailed)
		}
	}
}

// statHost 起一个只认 `workspaceFiles.stat` 的假宿主；fail=true 时按官方
// `workspace-file/not-found` 四字段错误答复（宿主 handler 的真形状）。
//
// 参数形状同样是契约：**只有 sessionId + path**，官方 typert 签名是
// `stat(sessionId, path, signal)`，宿主 DisallowUnknownFields —— 多塞 options 就 400。
func statHost(t *testing.T, fail bool, seen *map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/workspaceFiles.stat" {
			t.Errorf("路径 = %q，want /api/workspaceFiles.stat", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("请求体不是 JSON：%v", err)
		}
		if seen != nil {
			*seen = body
		}
		w.Header().Set("Content-Type", "application/json")
		if fail {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
				"code": "workspace-file/not-found", "message": "文件不存在",
				"hint": "路径相对会话工作区", "retryable": false,
			}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{"absolutePath": `D:\ws\docs\a.png`, "version": "v1"},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestWorkspaceFileStatProbe 钉住 #18 的 stat 预取：成功 = 可用、失败 = 不可用，
// 两个参数原样转发、且**不带 options**（宿主 DisallowUnknownFields）。
func TestWorkspaceFileStatProbe(t *testing.T) {
	var seen map[string]any
	m := &model{cli: apiclient.New(statHost(t, false, &seen).URL)}
	if !m.workspaceFileStat("sess-1", "docs/a.png") {
		t.Fatal("stat 成功却报告不可用：动作会被无故置灰")
	}
	if seen["sessionId"] != "sess-1" || seen["path"] != "docs/a.png" {
		t.Fatalf("stat 参数 = %v，want sessionId=sess-1 path=docs/a.png（按会话解析工作区根）", seen)
	}
	if _, ok := seen["options"]; ok {
		t.Error("多发了 options：官方 stat 没有这个参数，宿主会 400")
	}

	failed := &model{cli: apiclient.New(statHost(t, true, nil).URL)}
	if failed.workspaceFileStat("sess-1", "docs/a.png") {
		t.Fatal("stat 失败却报告可用：官方 `available = absolutePath !== undefined` 应为 false")
	}
}

// TestWorkspaceFileStatWithoutClientIsUnavailable 钉住"没有 apiclient 就别假装可用"：
// 没壳 = 探测不出来 = 不可用（动作置灰），不是"默认可用"。
func TestWorkspaceFileStatWithoutClientIsUnavailable(t *testing.T) {
	m := &model{}
	if m.workspaceFileStat("sess-1", "a.txt") {
		t.Fatal("没有 apiclient 却报告可用")
	}
	if m.workspaceFileStat("", "a.txt") || m.workspaceFileStat("sess-1", "") {
		t.Fatal("缺身份却报告可用：没有 (sessionId, path) 就不知道 stat 什么")
	}
}
