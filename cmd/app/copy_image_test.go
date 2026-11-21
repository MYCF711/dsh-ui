package main

// copy_image_test.go —— `#18` ①「复制图片」的钉子（cmd/app/copy_image.go）。
//
// # 它守的是什么
//
// 这一层是"点了菜单之后真的发生什么"的唯一位置：
//
//	workspaceFiles.readBytes（真 HTTP 形状）→ imgdecode.ToPNG → clipboardSetImage → 状态栏
//
// `clipboardSetImage` 是替换点：真剪贴板是**全局状态**，测试写它就是打扰正在用
// 这台机器的人。所以这里按下替换点，验的是"字节有没有被真的交给写入口"、
// "交的是不是 PNG"、"失败有没有说出来"——三件都不是靠看代码能保证的。
//
// 三条变异（改一处，下面的断言必须红）：
//
//	M1 不调用 clipboardSetImage          → TestCopyWorkspaceImagePNGPassthrough 的 calls 断言红
//	M2 交出去的不是 PNG（原始字节直传）  → 同一用例的字节断言 + TestCopyWorkspaceImageConverts 的魔数断言红
//	M3 失败不写状态栏                    → TestCopyWorkspaceImageReportsClipboardFailure 的状态断言红

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/imgdecode"
)

// 官方 zh 字典里的两条文案（OFF:ui-file-actions/src/client/locales.ts:8,12）：
// 状态栏必须出现其中一条，否则失败会被误读成成功、成功会被误读成没发生。
const (
	wantCopied     = "已复制"
	wantCopyFailed = "无法复制，请检查内容是否可用后重试"
)

// clipboardSpy 是 `clipboardSetImage` 的替身：记下**收到的字节**与调用次数。
type clipboardSpy struct {
	calls   int
	png     []byte
	failure error
}

// stubClipboard 按下替换点，并在用例结束时还原。
func stubClipboard(t *testing.T, failure error) *clipboardSpy {
	t.Helper()
	spy := &clipboardSpy{failure: failure}
	previous := clipboardSetImage
	clipboardSetImage = func(png []byte) error {
		spy.calls++
		// 拷一份：被测代码之后改切片也不会污染证据。
		spy.png = append([]byte(nil), png...)
		return spy.failure
	}
	t.Cleanup(func() { clipboardSetImage = previous })
	return spy
}

// readBytesFailHost 起一个只认 `workspaceFiles.readBytes`、永远按官方四字段错误
// 答复的假宿主（`workspace-file/not-found` 的真实形状，见
// HOST:internal/transport/handlers_workspace_official.go 的错误码表）。
func readBytesFailHost(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/workspaceFiles.readBytes" {
			t.Errorf("路径 = %q，want /api/workspaceFiles.readBytes", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
			"code": "workspace-file/not-found", "message": "文件不存在",
			"hint": "路径相对会话工作区", "retryable": false,
		}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// imageBytes 编一张纯色 PNG（fill 用 RGBA 描述）。
func imageBytesPNG(t *testing.T, w, h int, fill color.RGBA) []byte {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			source.Set(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, source); err != nil {
		t.Fatalf("编 PNG 失败：%v", err)
	}
	return buf.Bytes()
}

// imageBytesJPEG 编一张纯色 JPEG（不是 PNG → 必须被转码）。
func imageBytesJPEG(t *testing.T, w, h int, fill color.RGBA) []byte {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			source.Set(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, source, nil); err != nil {
		t.Fatalf("编 JPEG 失败：%v", err)
	}
	return buf.Bytes()
}

// TestCopyWorkspaceImagePNGPassthrough 走完整链路：读字节 → （PNG 透传）→ 写入口 → 状态栏。
//
// 三条断言各自对应一件"不能靠读代码保证"的事：
//   - `sessionId` / `path` 原样发给宿主（带 options，缺它真宿主 400）；
//   - 交给写入口的字节与宿主给回来的**逐字节相同**（PNG 不重编码）；
//   - 状态栏是官方的 `已复制`，且没有混进失败文案。
func TestCopyWorkspaceImagePNGPassthrough(t *testing.T) {
	payload := imageBytesPNG(t, 4, 3, color.RGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xFF})
	var seen map[string]any
	srv := readBytesHost(t, payload, &seen)
	spy := stubClipboard(t, nil)

	m := &model{cli: apiclient.New(srv.URL)}
	m.copyWorkspaceImage("sess-1", "pics/shot.png")

	if seen["sessionId"] != "sess-1" {
		t.Errorf("sessionId = %v，want sess-1", seen["sessionId"])
	}
	if seen["path"] != "pics/shot.png" {
		t.Errorf("path = %v，want pics/shot.png（工作区相对路径原样转发）", seen["path"])
	}
	if _, ok := seen["options"]; !ok {
		t.Error("缺 options 参数：真宿主会 400（官方该参数必填）")
	}
	if spy.calls != 1 {
		t.Fatalf("剪贴板写入口被调用 %d 次，want 1（图片没交出去 = 点了复制什么都没发生）", spy.calls)
	}
	if !bytes.Equal(spy.png, payload) {
		t.Fatalf("交给写入口的是 %d 字节、开头 %x；宿主给的是 %d 字节、开头 %x——PNG 必须原样透传",
			len(spy.png), firstBytes(spy.png), len(payload), firstBytes(payload))
	}
	if !strings.Contains(m.status, wantCopied) {
		t.Errorf("状态栏 = %q，want 含 %q（官方的 menu.copied）", m.status, wantCopied)
	}
	if strings.Contains(m.status, wantCopyFailed) {
		t.Errorf("状态栏 = %q，成功却混进了失败文案", m.status)
	}
}

// TestCopyWorkspaceImageConvertsNonPNG 钉住"不是 PNG 的图也要能复制"。
//
// 工作区里的图可以是 JPEG / GIF，而 viva 的写入口只收 PNG（空 / 非 PNG / 超
// 32 MiB 都报错且不动剪贴板）。官方在渲染层用 `<canvas>` 转码（files.ts:70-86），
// 本仓用 imgdecode.ToPNG。少了这一步，JPEG 会**必然失败**——那不是"官方行为"。
func TestCopyWorkspaceImageConvertsNonPNG(t *testing.T) {
	payload := imageBytesJPEG(t, 5, 2, color.RGBA{R: 0xC0, G: 0x40, B: 0x10, A: 0xFF})
	srv := readBytesHost(t, payload, nil)
	spy := stubClipboard(t, nil)

	m := &model{cli: apiclient.New(srv.URL)}
	m.copyWorkspaceImage("sess-1", "pics/photo.jpg")

	if spy.calls != 1 {
		t.Fatalf("剪贴板写入口被调用 %d 次，want 1", spy.calls)
	}
	if imgdecode.IsPNG(payload) {
		t.Fatal("夹具本身是 PNG，测不到转码")
	}
	if !imgdecode.IsPNG(spy.png) {
		t.Fatalf("交给写入口的不是 PNG（开头 %x）：viva 的写入口只收 PNG，原样直传必然被拒", firstBytes(spy.png))
	}
	decoded, err := png.Decode(bytes.NewReader(spy.png))
	if err != nil {
		t.Fatalf("交给写入口的 PNG 解不开：%v", err)
	}
	if bounds := decoded.Bounds(); bounds.Dx() != 5 || bounds.Dy() != 2 {
		t.Fatalf("转码后尺寸 = %d×%d，want 5×2（转码不许缩放）", bounds.Dx(), bounds.Dy())
	}
	if !strings.Contains(m.status, wantCopied) {
		t.Errorf("状态栏 = %q，want 含 %q", m.status, wantCopied)
	}
}

// TestCopyWorkspaceImageReportsClipboardFailure 钉住"失败必须说出来"：
// 写入口报错时状态栏要出现官方的 `menu.copyFailed` 与**原因**，不许静默当成功。
//
// 同时钉住"调用发生过"：字节**已经**交给写入口了，失败的是写入口自己
// （真实场景：剪贴板被别的程序占着、图超过 32 MiB、平台不支持）。
func TestCopyWorkspaceImageReportsClipboardFailure(t *testing.T) {
	payload := imageBytesPNG(t, 2, 2, color.RGBA{R: 0x01, G: 0x02, B: 0x03, A: 0xFF})
	srv := readBytesHost(t, payload, nil)
	spy := stubClipboard(t, errors.New("剪贴板被别的程序占着"))

	m := &model{cli: apiclient.New(srv.URL)}
	m.copyWorkspaceImage("sess-1", "pics/a.png")

	if spy.calls != 1 {
		t.Fatalf("剪贴板写入口被调用 %d 次，want 1（失败点必须在写入口，不是没调用）", spy.calls)
	}
	if !strings.Contains(m.status, wantCopyFailed) {
		t.Fatalf("状态栏 = %q，want 含官方的 %q", m.status, wantCopyFailed)
	}
	if !strings.Contains(m.status, "剪贴板被别的程序占着") {
		t.Errorf("状态栏 = %q，want 带上失败原因（否则用户不知道为什么失败）", m.status)
	}
	if strings.Contains(m.status, wantCopied) {
		t.Errorf("状态栏 = %q，失败却写着已复制", m.status)
	}
}

// TestCopyWorkspaceImageReportsReadFailure 钉住"读不到字节就别去碰剪贴板"。
//
// 宿主 404 时如果照样调写入口，就等于**先清空用户剪贴板、再报告失败**——
// 那是净损失。所以这里断言调用次数是 0。
func TestCopyWorkspaceImageReportsReadFailure(t *testing.T) {
	srv := readBytesFailHost(t)
	spy := stubClipboard(t, nil)

	m := &model{cli: apiclient.New(srv.URL)}
	m.copyWorkspaceImage("sess-1", "pics/gone.png")

	if spy.calls != 0 {
		t.Fatalf("读字节都失败了却调了剪贴板写入口 %d 次：用户的剪贴板被无故清空", spy.calls)
	}
	if !strings.Contains(m.status, wantCopyFailed) {
		t.Fatalf("状态栏 = %q，want 含 %q", m.status, wantCopyFailed)
	}
}

// TestCopyWorkspaceImageRejectsEmptyBytes 钉住"0 字节算失败"（与 attachment_bytes.go
// 的同一条判据）：宿主不会登记 0 字节文件，所以 0 字节只能是链上出了问题。
func TestCopyWorkspaceImageRejectsEmptyBytes(t *testing.T) {
	srv := readBytesHost(t, nil, nil) // 回 0 字节
	spy := stubClipboard(t, nil)

	m := &model{cli: apiclient.New(srv.URL)}
	m.copyWorkspaceImage("sess-1", "pics/empty.png")

	if spy.calls != 0 {
		t.Fatalf("0 字节也去写了剪贴板 %d 次", spy.calls)
	}
	if !strings.Contains(m.status, wantCopyFailed) {
		t.Fatalf("状态栏 = %q，want 含 %q", m.status, wantCopyFailed)
	}
	if !strings.Contains(m.status, "0 字节") {
		t.Errorf("状态栏 = %q，want 说明是 0 字节（而不是把 0 字节当成一张图）", m.status)
	}
}

// TestCopyWorkspaceImageRejectsBadArguments 钉住"参数不全就别发请求"：
// 没有 apiclient / 缺会话 / 缺路径，都不许猜一个工作区去读。
func TestCopyWorkspaceImageRejectsBadArguments(t *testing.T) {
	noClient := &model{}
	noClient.copyWorkspaceImage("sess-1", "pics/a.png")
	if !strings.Contains(noClient.status, wantCopyFailed) {
		t.Errorf("没有 apiclient 时状态栏 = %q，want 含 %q", noClient.status, wantCopyFailed)
	}

	srv := readBytesHost(t, imageBytesPNG(t, 1, 1, color.RGBA{A: 0xFF}), nil)
	for _, tc := range []struct{ session, path string }{
		{"", "pics/a.png"},
		{"sess-1", ""},
	} {
		spy := stubClipboard(t, nil)
		m := &model{cli: apiclient.New(srv.URL)}
		m.copyWorkspaceImage(tc.session, tc.path)
		if spy.calls != 0 {
			t.Errorf("%+v 的坏参数也去写了剪贴板 %d 次", tc, spy.calls)
		}
		if !strings.Contains(m.status, wantCopyFailed) {
			t.Errorf("%+v 的状态 = %q，want 含 %q", tc, m.status, wantCopyFailed)
		}
	}
}

// firstBytes 取开头最多 8 字节，用于把"交错了字节"的失败信息写清楚。
func firstBytes(body []byte) []byte {
	if len(body) > 8 {
		return body[:8]
	}
	return body
}
