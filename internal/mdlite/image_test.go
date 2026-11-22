package mdlite

// image_test.go —— `#18` (b) markdown 正文图片的三件纯事实：
// 目的地判定（官方 Gb/qW/Fp 等价物）、显示尺寸（官方两条 max-* 规则）、三态缓存。
//
// 全是**无绘制**断言：像素级判据在 `plugins/dsconversation` 的 headless 用例里，
// 这边只钉判据本身会不会红（"该拒的拒、该取的取"两向都钉）。

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
)

// TestParseMarkdownImage 钉官方那三步判定。
//
// 每一行都对着官方实现的一个分支写的：
//
//	gb 拒绝表       javascript/vbscript/data/blob
//	qw 白名单       http/https
//	fp 路径识别     /、./、../、盘符、裸相对名
func TestParseMarkdownImage(t *testing.T) {
	cases := []struct {
		destination string
		want        MarkdownImageKind
	}{
		// ── 官方 qW：只放 http/https ────────────────────────────────
		{"https://example.com/a.png", MarkdownImageURL},
		{"http://example.com/a.png", MarkdownImageURL},
		{"HTTPS://EXAMPLE.COM/a.png", MarkdownImageURL},
		// ── 官方 Gb：显式拒绝的四个 scheme ─────────────────────────
		{"data:image/png;base64,iVBORw0KGgo=", MarkdownImageRefused},
		{"javascript:alert(1)", MarkdownImageRefused},
		{"vbscript:msgbox", MarkdownImageRefused},
		{"blob:http://x/y", MarkdownImageRefused},
		// ── 其它 scheme：官方一个都不取（qW 只放两个）──────────────
		{"file:///C:/x.png", MarkdownImageRefused},
		{"ftp://example.com/a.png", MarkdownImageRefused},
		{"mailto:a@b.c", MarkdownImageRefused},
		// ── 官方 Fp：路径（含裸相对名）──────────────────────────────
		{"/abs/x.png", MarkdownImagePath},
		{"//server/share/x.png", MarkdownImagePath},
		{`\\server\share\x.png`, MarkdownImagePath},
		{"./x.png", MarkdownImagePath},
		{"../x.png", MarkdownImagePath},
		{`C:\shots\x.png`, MarkdownImagePath},
		{"C:/shots/x.png", MarkdownImagePath},
		{"x.png", MarkdownImagePath},
		{"shots/x.png", MarkdownImagePath},
		// ── 官方 Fp：空串与控制字符 ─────────────────────────────────
		{"", MarkdownImageRefused},
		{"x\x00.png", MarkdownImageRefused},
		{"x\x1f.png", MarkdownImageRefused},
		// ── 罕见的边界：scheme 后面没东西 ───────────────────────────
		{"http:", MarkdownImageRefused},
	}
	for _, c := range cases {
		if got := ParseMarkdownImage(c.destination).Kind; got != c.want {
			t.Errorf("ParseMarkdownImage(%q).Kind = %v，要 %v", c.destination, got, c.want)
		}
	}
}

// TestParseMarkdownImageKeepsDestination 钉"不改写"：目的地原文原样进结构体。
//
// 为什么重要：路径里带空格/中文/百分号时，任何"顺手 decode/encode"都会让
// 宿主读不到文件；官方把 `destination` 原样交给 `fileImages.resolve`，我们照办。
func TestParseMarkdownImageKeepsDestination(t *testing.T) {
	const destination = "./截图 1.png?x=1"
	ref := ParseMarkdownImage(destination)
	if ref.Kind != MarkdownImagePath {
		t.Fatalf("Kind = %v，要 Path", ref.Kind)
	}
	if ref.Destination != destination {
		t.Fatalf("Destination = %q，要原样 %q", ref.Destination, destination)
	}
}

// TestFitMarkdownImage 钉官方两条 max-* 规则与「从不放大」。
func TestFitMarkdownImage(t *testing.T) {
	cases := []struct {
		name           string
		width, height  int
		column         float32
		wantW, wantH   float32
		descriptionCN  string
		allowRounding  bool
		tolerancePixel float32
	}{
		{name: "小图原样", width: 200, height: 120, column: 800, wantW: 200, wantH: 120,
			descriptionCN: "200×120 既没超 640 也没超 360，也不许放大到列宽"},
		{name: "宽图按 640 压", width: 1280, height: 200, column: 800, wantW: 640, wantH: 100,
			descriptionCN: "宽 1280 压到 640，比例 6.4 → 高 200/6.4=31.25？不对：高也要按同比例"},
		{name: "列宽小于 640 时按列宽", width: 2000, height: 1000, column: 500, wantW: 500, wantH: 250,
			descriptionCN: "max-width:min(100%,640px) 里的 100% 就是列宽"},
		{name: "高图按 360 压", width: 100, height: 1000, column: 800, wantW: 36, wantH: 360,
			descriptionCN: "max-height:360px 先压高，宽按比例跟着缩"},
		{name: "两边都超", width: 4000, height: 4000, column: 400, wantW: 360, wantH: 360,
			descriptionCN: "400 列宽先压到 400×400，再被 360 高度封顶"},
		{name: "列宽未知只按 640/360", width: 2000, height: 200, column: 0, wantW: 640, wantH: 64,
			descriptionCN: "column<=0 时只按官方常量封顶"},
		{name: "零宽位图", width: 0, height: 0, column: 800, wantW: 0, wantH: 0,
			descriptionCN: "坏位图给 0，让调用方按失败态画"},
	}
	for _, c := range cases {
		bitmap := gfx.Bitmap{Width: c.width, Height: c.height}
		gotW, gotH := FitMarkdownImage(bitmap, c.column)
		if gotW != c.wantW || gotH != c.wantH {
			t.Errorf("%s: Fit(%d×%d, 列宽 %v) = %v×%v，要 %v×%v（%s）",
				c.name, c.width, c.height, c.column, gotW, gotH, c.wantW, c.wantH, c.descriptionCN)
		}
		if gotW > float32(c.width) || gotH > float32(c.height) {
			if c.width > 0 && c.height > 0 {
				t.Errorf("%s: 放大了（%v×%v > %d×%d），官方 width:auto/height:auto 从不放大",
					c.name, gotW, gotH, c.width, c.height)
			}
		}
	}
}

// TestMarkdownImageFailureText 钉官方失败态文案（逐字）。
//
// `image.failed` 的 zh 是「图片无法预览」，后面是官方的 ` · ` 分隔符再接
// `alt||destination`（`OFF:lK` 的模板）。一个字都不许改。
func TestMarkdownImageFailureText(t *testing.T) {
	if got := MarkdownImageFailedText("示意图", "./a.png"); got != "图片无法预览 · 示意图" {
		t.Fatalf("有 alt 时 = %q", got)
	}
	if got := MarkdownImageFailedText("", "./a.png"); got != "图片无法预览 · ./a.png" {
		t.Fatalf("无 alt 时 = %q（官方 alt||destination）", got)
	}
	if got := MarkdownImageAltText("", "./a.png"); got != "./a.png" {
		t.Fatalf("AltText 兜底 = %q", got)
	}
}

// solidPNG 编一张纯色 PNG（宽度大于 1 时右半列换色，方便数像素）。
func solidPNG(t *testing.T, width, height int, fill color.RGBA) []byte {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			source.Set(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, source); err != nil {
		t.Fatalf("编 PNG 失败：%v", err)
	}
	return buf.Bytes()
}

// waitFor 等到条件成立（取数是后台 goroutine，测试要等它落库）。
func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("%s：2 秒内没等到", what)
}

// TestMarkdownImageResolverThreeStates 钉四态流转。
//
// 四个判据（每一档都是一个官方分支）：
//
//	取到了    Pending → Ready（位图有像素）
//	取不到    Pending → Failed（官方 onError）
//	不取      Unresolved，且**一次都不许调 loader**（官方 src===undefined）
//	没能力    Unresolved（壳没接这条能力 → 官方画 alt）
func TestMarkdownImageResolverThreeStates(t *testing.T) {
	body := solidPNG(t, 12, 8, color.RGBA{R: 0x11, G: 0xEE, B: 0x33, A: 0xFF})
	cases := []struct {
		name        string
		destination string
		allowPath   bool
		load        MarkdownImageLoader
		want        MarkdownImageState
		wantCalls   int32
	}{
		{
			name:        "URL 取到",
			destination: "https://example.com/a.png",
			allowPath:   true,
			load: func(string, string, string) ([]byte, bool) {
				return body, true
			},
			want:      MarkdownImageReady,
			wantCalls: 1,
		},
		{
			name:        "路径取到",
			destination: "./shot.png",
			allowPath:   true,
			load: func(string, string, string) ([]byte, bool) {
				return body, true
			},
			want:      MarkdownImageReady,
			wantCalls: 1,
		},
		{
			name:        "取不到字节",
			destination: "./missing.png",
			allowPath:   true,
			load: func(string, string, string) ([]byte, bool) {
				return nil, false
			},
			want:      MarkdownImageFailed,
			wantCalls: 1,
		},
		{
			name:        "字节不是图",
			destination: "./broken.png",
			allowPath:   true,
			load: func(string, string, string) ([]byte, bool) {
				return []byte("not a png"), true
			},
			want:      MarkdownImageFailed,
			wantCalls: 1,
		},
		{
			name:        "空字节算失败",
			destination: "./empty.png",
			allowPath:   true,
			load: func(string, string, string) ([]byte, bool) {
				return nil, true
			},
			want:      MarkdownImageFailed,
			wantCalls: 1,
		},
		{
			name:        "拒绝表里的 scheme 一次都不取",
			destination: "data:image/png;base64,iVBORw0KGgo=",
			allowPath:   true,
			load: func(string, string, string) ([]byte, bool) {
				return body, true
			},
			want:      MarkdownImageUnresolved,
			wantCalls: 0,
		},
		{
			name:        "流式期间不取路径（官方 streaming 时跳过 fileImages.resolve）",
			destination: "./shot.png",
			allowPath:   false,
			load: func(string, string, string) ([]byte, bool) {
				return body, true
			},
			want:      MarkdownImageUnresolved,
			wantCalls: 0,
		},
		{
			name:        "流式期间 URL 照取",
			destination: "https://example.com/a.png",
			allowPath:   false,
			load: func(string, string, string) ([]byte, bool) {
				return body, true
			},
			want:      MarkdownImageReady,
			wantCalls: 1,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var calls int32
			loader := func(sessionID, cwd, destination string) ([]byte, bool) {
				atomic.AddInt32(&calls, 1)
				return c.load(sessionID, cwd, destination)
			}
			resolver := NewMarkdownImageResolver()
			resolver.Use(loader, nil)
			first := resolver.Lookup("s1", "D:/w", c.destination, c.allowPath)
			if c.want == MarkdownImageUnresolved {
				if first.State != MarkdownImageUnresolved {
					t.Fatalf("首帧 State = %v，要 Unresolved", first.State)
				}
				if got := atomic.LoadInt32(&calls); got != 0 {
					t.Fatalf("Unresolved 档调了 %d 次 loader，要 0 次", got)
				}
				// 再问一次也不许起请求。
				_ = resolver.Lookup("s1", "D:/w", c.destination, c.allowPath)
				time.Sleep(2 * time.Millisecond)
				if got := atomic.LoadInt32(&calls); got != 0 {
					t.Fatalf("第二次问 Unresolved 档又调了 loader（共 %d 次）", got)
				}
				return
			}
			waitFor(t, "后台取数落库", func() bool {
				return resolver.Lookup("s1", "D:/w", c.destination, c.allowPath).State == c.want
			})
			got := resolver.Lookup("s1", "D:/w", c.destination, c.allowPath)
			if got.State != c.want {
				t.Fatalf("State = %v，要 %v", got.State, c.want)
			}
			if got.State == MarkdownImageReady {
				if got.Bitmap.Width != 12 || got.Bitmap.Height != 8 {
					t.Fatalf("位图 = %d×%d，要 12×8", got.Bitmap.Width, got.Bitmap.Height)
				}
				if got.Bitmap.ID == 0 {
					t.Fatal("位图没有稳定 ID（viva 的纹理缓存每帧都会重新上传）")
				}
			}
			if got := atomic.LoadInt32(&calls); got != c.wantCalls {
				t.Fatalf("loader 调了 %d 次，要 %d 次", got, c.wantCalls)
			}
		})
	}
}

// TestMarkdownImageResolverWithoutLoader 钉"壳没接这条能力"这一档。
func TestMarkdownImageResolverWithoutLoader(t *testing.T) {
	resolver := NewMarkdownImageResolver()
	resolver.Use(nil, nil)
	got := resolver.Lookup("s1", "D:/w", "https://example.com/a.png", true)
	if got.State != MarkdownImageUnresolved {
		t.Fatalf("State = %v，要 Unresolved（没有取数入口时不假装加载中）", got.State)
	}
	var nilResolver *MarkdownImageResolver
	if got := nilResolver.Lookup("s1", "D:/w", "https://example.com/a.png", true); got.State != MarkdownImageUnresolved {
		t.Fatalf("nil resolver State = %v，要 Unresolved", got.State)
	}
	if got := nilResolver.Generation(); got != 0 {
		t.Fatalf("nil resolver Generation = %d，要 0", got)
	}
}

// TestMarkdownImageResolverLoadsOnce 钉"同一张图只取一次"（并发问也不再取）。
//
// 为什么这条必须红得起来：官方文档明说 `Image` 会被问"每帧、每张图、可能不止一次"，
// 少了 inflight 去重就是每帧一个 HTTP。
func TestMarkdownImageResolverLoadsOnce(t *testing.T) {
	body := solidPNG(t, 4, 4, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	var calls int32
	resolver := NewMarkdownImageResolver()
	resolver.Use(func(string, string, string) ([]byte, bool) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(5 * time.Millisecond)
		return body, true
	}, nil)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = resolver.Lookup("s1", "D:/w", "./a.png", true)
		}()
	}
	wg.Wait()
	waitFor(t, "取数落库", func() bool {
		return resolver.Lookup("s1", "D:/w", "./a.png", true).State == MarkdownImageReady
	})
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("16 次并发 Lookup 调了 %d 次 loader，要 1 次", got)
	}
}

// TestMarkdownImageResolverGenerationBumps 钉代号：没有它，外层排版缓存永不过期。
func TestMarkdownImageResolverGenerationBumps(t *testing.T) {
	body := solidPNG(t, 4, 4, color.RGBA{R: 9, G: 9, B: 9, A: 255})
	resolver := NewMarkdownImageResolver()
	redraws := int32(0)
	resolver.Use(func(string, string, string) ([]byte, bool) { return body, true }, func() {
		atomic.AddInt32(&redraws, 1)
	})
	before := resolver.Generation()
	_ = resolver.Lookup("s1", "D:/w", "./a.png", true)
	waitFor(t, "取数落库", func() bool {
		return resolver.Generation() > before
	})
	// 完成时必须请求重画一次（否则图永远不会上屏）。
	waitFor(t, "重画回调", func() bool { return atomic.LoadInt32(&redraws) >= 1 })
}

// TestMarkdownImageResolverKeyIncludesSessionAndCwd 钉缓存键：换会话/换工作区就是另一张图。
func TestMarkdownImageResolverKeyIncludesSessionAndCwd(t *testing.T) {
	var mu sync.Mutex
	bodies := map[string][]byte{}
	resolver := NewMarkdownImageResolver()
	resolver.Use(func(sessionID, cwd, destination string) ([]byte, bool) {
		mu.Lock()
		defer mu.Unlock()
		body, ok := bodies[sessionID+"|"+cwd]
		return body, ok
	}, nil)
	bodies["s1|D:/a"] = solidPNG(t, 3, 3, color.RGBA{R: 1, G: 1, B: 1, A: 255})
	bodies["s2|D:/b"] = solidPNG(t, 5, 5, color.RGBA{R: 2, G: 2, B: 2, A: 255})
	_ = resolver.Lookup("s1", "D:/a", "./x.png", true)
	_ = resolver.Lookup("s2", "D:/b", "./x.png", true)
	waitFor(t, "两张图各自落库", func() bool {
		return resolver.Lookup("s1", "D:/a", "./x.png", true).State == MarkdownImageReady &&
			resolver.Lookup("s2", "D:/b", "./x.png", true).State == MarkdownImageReady
	})
	first := resolver.Lookup("s1", "D:/a", "./x.png", true)
	second := resolver.Lookup("s2", "D:/b", "./x.png", true)
	if first.Bitmap.Width != 3 || second.Bitmap.Width != 5 {
		t.Fatalf("缓存串了：s1=%dpx、s2=%dpx，要 3/5", first.Bitmap.Width, second.Bitmap.Width)
	}
}

// TestSplitMarkdownInline 钉切分：没有图时与旧 `spansText` 逐字节相同，有图时保序。
func TestSplitMarkdownInline(t *testing.T) {
	var doc markdown.Document
	doc.Parse("前文 ![示意图](./a.png) 后文")
	var block markdown.Block
	found := false
	for _, candidate := range doc.Blocks {
		if candidate.Kind == markdown.Paragraph {
			block, found = candidate, true
			break
		}
	}
	if !found {
		t.Fatal("夹具没解析出段落")
	}
	runs := SplitMarkdownInline(doc.Spans(&block))
	if len(runs) != 3 {
		t.Fatalf("切出 %d 段，要 3 段（文字 / 图 / 文字）：%+v", len(runs), runs)
	}
	if runs[0].Text != "前文 " || runs[0].Image != "" {
		t.Fatalf("第 1 段 = %+v", runs[0])
	}
	if runs[1].Image != "./a.png" || runs[1].Text != "示意图" {
		t.Fatalf("第 2 段（图）= %+v，要 Destination=./a.png、alt=示意图", runs[1])
	}
	if runs[2].Text != " 后文" || runs[2].Image != "" {
		t.Fatalf("第 3 段 = %+v", runs[2])
	}

	// 反向：没有图时必须是一段，且与旧的 spansText 逐字节一致。
	var plain markdown.Document
	plain.Parse("**加粗** 与 `code`")
	var plainBlock markdown.Block
	for _, candidate := range plain.Blocks {
		if candidate.Kind == markdown.Paragraph {
			plainBlock, found = candidate, true
			break
		}
	}
	if !found {
		t.Fatal("夹具没解析出纯文字段落")
	}
	plainRuns := SplitMarkdownInline(plain.Spans(&plainBlock))
	if len(plainRuns) != 1 {
		t.Fatalf("纯文字段落切出 %d 段，要 1 段", len(plainRuns))
	}
	var joined bytes.Buffer
	for _, span := range plain.Spans(&plainBlock) {
		joined.WriteString(span.Text)
	}
	if plainRuns[0].Text != joined.String() {
		t.Fatalf("纯文字段落文本 = %q，要 %q（与旧 spansText 不同）", plainRuns[0].Text, joined.String())
	}
	if plainRuns[0].Mono {
		t.Fatal("混合了行内代码的段落不该判成整段等宽")
	}
}

// TestMarkdownImageResolverNeedsCwdForRelativePaths 钉"cwd 未知 + 相对路径"这一档。
//
// 官方 `fileMediaUrl` 只给**绝对**路径开 URL（`isAbsoluteWorkspacePath`），所以
// cwd 未知时相对路径根本取不到——这一档是 Unresolved（只画 alt），**不是** Failed
// （那会显示「图片无法预览」，等于告诉用户"图坏了"）。绝对路径照取不误。
func TestMarkdownImageResolverNeedsCwdForRelativePaths(t *testing.T) {
	body := solidPNG(t, 4, 4, color.RGBA{R: 5, G: 6, B: 7, A: 255})
	cases := []struct {
		name        string
		cwd         string
		destination string
		want        MarkdownImageState
		wantCalls   int32
	}{
		{"cwd 未知 + 相对路径 → 取不到地址", "", "./shot.png", MarkdownImageUnresolved, 0},
		{"cwd 未知 + 裸相对名 → 取不到地址", "", "shot.png", MarkdownImageUnresolved, 0},
		{"cwd 未知 + 绝对 POSIX 路径 → 照取", "", "/abs/shot.png", MarkdownImageReady, 1},
		{"cwd 未知 + 盘符路径 → 照取", "", `C:\shots\a.png`, MarkdownImageReady, 1},
		{"cwd 未知 + URL → 照取", "", "https://example.com/a.png", MarkdownImageReady, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var calls int32
			resolver := NewMarkdownImageResolver()
			resolver.Use(func(string, string, string) ([]byte, bool) {
				atomic.AddInt32(&calls, 1)
				return body, true
			}, nil)
			first := resolver.Lookup("s1", c.cwd, c.destination, true)
			if c.want == MarkdownImageUnresolved {
				if first.State != MarkdownImageUnresolved {
					t.Fatalf("State = %v，要 Unresolved", first.State)
				}
				time.Sleep(2 * time.Millisecond)
				if got := atomic.LoadInt32(&calls); got != 0 {
					t.Fatalf("Unresolved 档调了 %d 次 loader，要 0 次", got)
				}
				return
			}
			waitFor(t, "取数落库", func() bool {
				return resolver.Lookup("s1", c.cwd, c.destination, true).State == c.want
			})
			if got := atomic.LoadInt32(&calls); got != c.wantCalls {
				t.Fatalf("loader 调了 %d 次，要 %d 次", got, c.wantCalls)
			}
		})
	}
}
