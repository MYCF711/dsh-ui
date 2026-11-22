package main

// markdown_image_test.go —— `#18` (b) 正文图片的**壳侧取数**钉子（cmd/app/markdown_image.go）。
//
// # 它守的是什么
//
// 这一层是"正文里写了一张图之后，字节从哪儿来"的唯一位置：
//
//	URL 档   http(s) GET（带超时、只收 image/*、卡 50 MiB）
//	路径档   resolveWorkspacePath(cwd, path) → 宿主 `workspaceFiles.readBytes`
//	拒绝档   其它 scheme / 落不到绝对的路径 → 一次请求都不发
//
// 四条变异（改一处，下面的断言必须红）：
//
//	M1 URL 档不判状态码/类型          → TestMarkdownImageBytesURL 的 404/text 两档红
//	M2 路径档不拼 cwd                 → TestResolveMarkdownImagePath 与
//	                                   TestMarkdownImageBytesPath 的 seen["path"] 断言红
//	M3 拒绝档也去发请求               → TestMarkdownImageBytesRefused 的计数断言红
//	M4 0 字节当成功                   → TestMarkdownImageBytesPath 的 0 字节用例红

import (
	"image/color"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// TestResolveMarkdownImagePath 钉官方 `resolveWorkspacePath(cwd, path)` 的逐条语义。
//
// 参照物是 `OFF:dsh-util-workspace-path/lib/index.js` 的同名函数：
//
//	绝对路径（`/a`、`C:/a`、`\\unc`）  原样
//	cwd 为空                          原样（相对路径就此取不到）
//	其余                              cwd（去尾分隔符）+ 分隔符 + path（去前导 `./`）
//	`../a.png` 的前导点                **不清**（官方正则 `^(?:\.?[/\\])+` 匹配不到 `../`）
func TestResolveMarkdownImagePath(t *testing.T) {
	cases := []struct {
		name, cwd, path, want string
	}{
		{"Windows cwd + 相对名", `D:\ws`, "shot.png", `D:\ws\shot.png`},
		{"Windows cwd + ./ 前缀", `D:\ws`, "./shot.png", `D:\ws\shot.png`},
		{"Windows cwd + 多个 ./ 前缀", `D:\ws`, "././shot.png", `D:\ws\shot.png`},
		{"Windows cwd 尾分隔符", `D:\ws\`, "./shot.png", `D:\ws\shot.png`},
		{"POSIX cwd 用 / 拼", "/ws", "./shot.png", "/ws/shot.png"},
		{"../ 原样拼给宿主", `D:\ws`, "../shot.png", `D:\ws\../shot.png`},
		{"绝对 POSIX 路径原样", `D:\ws`, "/abs/shot.png", "/abs/shot.png"},
		{"盘符路径原样", `D:\ws`, `C:\other\shot.png`, `C:\other\shot.png`},
		{"UNC 路径原样", `D:\ws`, `\\server\share\shot.png`, `\\server\share\shot.png`},
		{"子目录 + ./ 前缀（path 里的 / 原样保留）", `D:\ws`, "./shots/a.png", `D:\ws\shots/a.png`},
		{"cwd 为空时原样", "", "./shot.png", "./shot.png"},
		{"空 path", `D:\ws`, "", ""},
	}
	for _, c := range cases {
		if got := resolveMarkdownImagePath(c.cwd, c.path); got != c.want {
			t.Errorf("%s: resolveMarkdownImagePath(%q, %q) = %q，want %q",
				c.name, c.cwd, c.path, got, c.want)
		}
	}
}

// TestMarkdownImageBytesURL 钉 URL 档的三道闸。
func TestMarkdownImageBytesURL(t *testing.T) {
	body := imageBytesPNG(t, 12, 8, color.RGBA{R: 0x11, G: 0xEE, B: 0x33, A: 0xFF})
	cases := []struct {
		name        string
		status      int
		contentType string
		payload     []byte
		wantOK      bool
	}{
		{"2xx + image/png", http.StatusOK, "image/png", body, true},
		{"2xx + image/png;charset", http.StatusOK, "image/png; charset=binary", body, true},
		{"404", http.StatusNotFound, "text/plain", []byte("nope"), false},
		{"200 但是 HTML", http.StatusOK, "text/html", []byte("<html></html>"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", c.contentType)
				w.WriteHeader(c.status)
				_, _ = w.Write(c.payload)
			}))
			defer srv.Close()
			m := &model{}
			got, ok := m.markdownImageBytes("s1", `D:\ws`, srv.URL+"/a.png")
			if ok != c.wantOK {
				t.Fatalf("ok = %v，期望 %v", ok, c.wantOK)
			}
			if !c.wantOK {
				if len(got.Bytes) != 0 {
					t.Fatalf("失败档不该带回字节，实际 %d 字节", len(got.Bytes))
				}
				return
			}
			if string(got.Bytes) != string(c.payload) {
				t.Fatalf("字节不是响应体原样（%d vs %d）", len(got.Bytes), len(c.payload))
			}
			if got.MIME != "image/png" {
				t.Fatalf("MIME = %q，期望 image/png", got.MIME)
			}
		})
	}
}

// TestFetchMarkdownImageURLTooLarge 钉字节上限（官方 `maxImageBytes` 默认值 52428800）。
//
// 上限是参数，所以夹具用 8 字节即可验越界逻辑——不必真造 50 MiB 响应。
func TestFetchMarkdownImageURLTooLarge(t *testing.T) {
	payload := make([]byte, 9)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()
	if body, _, err := fetchMarkdownImageURLWithin(srv.URL+"/big.png", 8); err == nil {
		t.Fatalf("超过上限的响应取成功了（%d 字节）", len(body))
	}
	// 反向：刚好等于上限必须成功（不许把 `>` 写成 `>=`）。
	body, mediaType, err := fetchMarkdownImageURLWithin(srv.URL+"/ok.png", 9)
	if err != nil {
		t.Fatalf("刚好等于上限应成功，实际 %v", err)
	}
	if len(body) != 9 || mediaType != "image/png" {
		t.Fatalf("刚好等于上限：%d 字节 / %q", len(body), mediaType)
	}
}

// TestMarkdownImageBytesRefused 钉拒绝档：一次请求都不发。
func TestMarkdownImageBytesRefused(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "image/png")
	}))
	defer srv.Close()
	m := &model{cli: apiclient.New(srv.URL)}
	refused := []string{
		"data:image/png;base64,iVBORw0KGgo=",
		"javascript:alert(1)",
		"blob:" + srv.URL + "/x",
		"file:///C:/x.png",
	}
	for _, destination := range refused {
		if _, ok := m.markdownImageBytes("s1", `D:\ws`, destination); ok {
			t.Fatalf("拒绝表里的 %q 竟然取成功", destination)
		}
	}
	if calls != 0 {
		t.Fatalf("拒绝档发了 %d 次请求，期望 0 次", calls)
	}
}

// TestMarkdownImageBytesPath 钉路径档：走宿主 `workspaceFiles.readBytes`，路径已拼成绝对。
func TestMarkdownImageBytesPath(t *testing.T) {
	payload := imageBytesPNG(t, 6, 4, color.RGBA{R: 1, G: 2, B: 3, A: 0xFF})
	var seen map[string]any
	srv := readBytesHost(t, payload, &seen)
	m := &model{cli: apiclient.New(srv.URL)}

	got, ok := m.markdownImageBytes("sess-1", `D:\ws`, "./shots/a.png")
	if !ok {
		t.Fatal("路径档应取到字节")
	}
	if string(got.Bytes) != string(payload) {
		t.Fatalf("字节不是宿主原样（%d vs %d）", len(got.Bytes), len(payload))
	}
	if seen == nil {
		t.Fatal("没有向宿主发 readBytes 请求")
	}
	if seen["sessionId"] != "sess-1" {
		t.Fatalf("sessionId = %v，期望 sess-1（缓存/权限都按会话）", seen["sessionId"])
	}
	if _, hasOptions := seen["options"]; !hasOptions {
		t.Fatal("缺 options：真宿主会 400（DisallowUnknownFields）")
	}
	// 注意拼写：官方只把 cwd 的尾分隔符换成分隔符，path 自己带的 `/` **不换**，
	// 于是 Windows 上会出现 `D:\ws\shots/a.png` 这种混合拼写——宿主两种都收，
	// 所以照抄，不做"看起来更整齐"的规范化（规范化会改 path 的字节，超出契约）。
	if path, _ := seen["path"].(string); path != `D:\ws\shots/a.png` {
		t.Fatalf("path = %q，期望 D:\\ws\\shots/a.png（cwd 已拼进绝对路径，path 里的 / 原样）", path)
	}

	// 0 字节必须是失败，不是"这张图本来就没内容"。
	emptyServer := readBytesHost(t, nil, nil)
	defer emptyServer.Close()
	empty := &model{cli: apiclient.New(emptyServer.URL)}
	if _, ok := empty.markdownImageBytes("sess-1", `D:\ws`, "./empty.png"); ok {
		t.Fatal("0 字节当成功了")
	}
}

// TestMarkdownImageBytesPathWithoutClient 钉"壳没有 apiclient"这一档。
func TestMarkdownImageBytesPathWithoutClient(t *testing.T) {
	var m *model
	if _, ok := m.markdownImageBytes("s1", `D:\ws`, "./a.png"); ok {
		t.Fatal("nil model 不该取到字节")
	}
	bare := &model{}
	if _, ok := bare.markdownImageBytes("s1", `D:\ws`, "./a.png"); ok {
		t.Fatal("没有 apiclient 不该取到字节")
	}
	// cwd 未知 + 相对路径 ⇒ 落不到绝对 ⇒ 不取（官方 fileMediaUrl 只认绝对路径）。
	withClient := &model{cli: apiclient.New("http://127.0.0.1:1")}
	if _, ok := withClient.markdownImageBytes("s1", "", "./a.png"); ok {
		t.Fatal("cwd 未知时相对路径不该取")
	}
}

// TestMarkdownImageHTTPClientHasDeadline 钉超时：没有总时限的客户端会把 goroutine 挂死。
func TestMarkdownImageHTTPClientHasDeadline(t *testing.T) {
	if markdownImageHTTPClient.Timeout <= 0 {
		t.Fatal("URL 取数客户端没有超时：一个不响应的地址会让取数 goroutine 永远挂着")
	}
	if markdownImageHTTPClient.Timeout < time.Second {
		t.Fatalf("超时 = %v，短到会误伤正常的大图", markdownImageHTTPClient.Timeout)
	}
	if markdownImageMaxBytes != 52_428_800 {
		t.Fatalf("字节上限 = %d，期望官方默认值 52428800", markdownImageMaxBytes)
	}
}
