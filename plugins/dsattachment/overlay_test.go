package dsattachment

// 悬停压层（官方 `DropOverlay`）的无头测试：真的画一帧（`ui.NewPage` + `gfx.Canvas`），
// 断言的是**官方能证明的东西**——底色逐值来自官方 `--dsw-alias-bg-mask-drop` 的两个取值，
// 文案逐字来自 conversation zh 字典，几何取「整窗铺满 + 内容居中」。
//
// 与 rail_test.go 同一套夹具（`fixture` / `textValues` / `hasText` / `HandleFileDrop`），
// 区别只在画的是 `drawDropOverlay` 而不是 `draw`——它是 `shell.overlay` 那个槽位的 painter。
//
// 两条本包的硬约束照旧：
//
//   - `ui.Box` 只在**本次 paint** 内有效，断言只对数值做（这里抄的是 `canvas.Rectangles()`）。
//   - 断言前先 `Reset`：canvas 的图元是累加的，不 Reset 会把上一帧的图画进这一帧。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// drawOverlayOnce 只画悬停压层那一帧（壳把 `shell.overlay` 的全屏图层交给它）。
func drawOverlayOnce(t *testing.T, p *Plugin, env *pluginapi.Env, canvas *gfx.Canvas, w, h float32) {
	t.Helper()
	canvas.Reset()
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: w, Height: h}, func(f ui.C) { p.drawDropOverlay(f, env) })
}

// over 造一个 over 相位的系统拖入事件（viva 在 Windows 的 OLE `DragEnter`/`DragOver`
// 上报 `FileDropOver`：`drop_windows.go:174-193`）。
func over() gfx.FileDropEvent {
	return gfx.FileDropEvent{Phase: gfx.FileDropOver, X: 600, Y: 300}
}

// leave 造一个 leave 相位（`drop_windows.go:195-202`）。
func leave() gfx.FileDropEvent {
	return gfx.FileDropEvent{Phase: gfx.FileDropLeave}
}

// maskRect 在这一帧里找那块铺满窗口的压层（官方 `.mask` 是 `inset:0`）。
func maskRect(canvas *gfx.Canvas, w, h float32) (gfx.Rectangle, bool) {
	for _, r := range canvas.Rectangles() {
		if r.Bounds.X == 0 && r.Bounds.Y == 0 && r.Bounds.Width == w && r.Bounds.Height == h {
			return r, true
		}
	}
	return gfx.Rectangle{}, false
}

// ── 1. 悬停中 → 整窗压层（官方暗色原值）+ 官方邀请文案 ─────────────────────

func TestDropOverlayDrawsMaskAndOfficialTitle(t *testing.T) {
	p, env, _, canvas := fixture(t, &fakeSink{})
	const w, h = 1180, 700

	// 没有悬停 → 零节点（官方 `{dragActive && <DropOverlay/>}`）。
	drawOverlayOnce(t, p, env, canvas, w, h)
	if p.overlayBox.Valid() {
		t.Fatal("没有悬停态时不该画压层")
	}
	if len(canvas.Rectangles()) != 0 || len(canvas.Texts()) != 0 {
		t.Fatalf("没有悬停态时画面该是空的：rects=%d texts=%v",
			len(canvas.Rectangles()), textValues(canvas))
	}

	// 真的走一次系统拖入：over 必须被消费（官方 `drop-events.ts:50-62`
	// 的 preventDefault + dropEffect='copy' 就是这里的 Handled）。
	if result := HandleFileDrop(over()); !result.Handled {
		t.Fatal("有 sink 时 FileDropOver 必须被消费（系统才会显示收文件的指示）")
	}
	drawOverlayOnce(t, p, env, canvas, w, h)

	if !p.overlayBox.Valid() {
		t.Fatal("悬停中必须画压层")
	}
	// 官方暗色档：`--dsw-alias-bg-mask-drop: rgba(39, 39, 48, 0.7)`
	// （`OFF:dsh-client-ui-theme/src/styles/design-platform.css:340`）。
	want := gfx.RGBA(39, 39, 48, dropMaskAlpha)
	rect, ok := maskRect(canvas, w, h)
	if !ok {
		t.Fatalf("压层必须铺满整窗 %d×%d，实际矩形：%v", w, h, canvas.Rectangles())
	}
	if rect.Color != want {
		t.Fatalf("压层底色必须是官方暗色原值 %v，实际 %v", want, rect.Color)
	}
	// 官方 `attachment.dropTitle`（`OFF:dsh-client-ui-conversation/src/client/locales.ts:37`）。
	if !hasText(canvas, zhDropTitle) {
		t.Fatalf("画面上必须有官方标题 %q，实际：%v", zhDropTitle, textValues(canvas))
	}
	// `--dsw-font-l-20` = 500 20px（`gradient-shadow-text.css:192-196`）+ `--dsw-alias-label-primary`。
	for _, text := range canvas.Texts() {
		if text.Value != zhDropTitle {
			continue
		}
		if text.Style.Size != dropTitleFontSize || text.Style.Weight != dropTitleWeight {
			t.Fatalf("标题字体必须取 --dsw-font-l-20：size=%.0f weight=%.0f，实际 size=%.0f weight=%.0f",
				dropTitleFontSize, dropTitleWeight, text.Style.Size, text.Style.Weight)
		}
		if text.Style.Color != p.theme.LabelPrimary {
			t.Fatalf("标题颜色必须取 --dsw-alias-label-primary")
		}
	}
	// 压层不吃指针（官方 `pointer-events:none`）：它不许有命中区，
	// 否则整窗交互会被一层装饰挡住。canvas 的命中区列表就是这条判据。
	if hits := canvas.Hits(); len(hits) != 0 {
		t.Fatalf("压层是纯装饰，不该声明命中区，实际 %d 个：%v", len(hits), hits)
	}
}

// ── 2. leave / drop 之后 → 悬停态清掉，一个节点都不画 ─────────────────────

func TestDropOverlayClearsOnLeave(t *testing.T) {
	p, env, _, canvas := fixture(t, &fakeSink{})
	const w, h = 1180, 700
	HandleFileDrop(over())
	drawOverlayOnce(t, p, env, canvas, w, h)
	if !p.overlayBox.Valid() {
		t.Fatal("悬停中必须画压层")
	}

	if result := HandleFileDrop(leave()); result.Handled {
		t.Fatal("FileDropLeave 不是接受，不该被消费（只请求重画）")
	}
	drawOverlayOnce(t, p, env, canvas, w, h)
	if p.overlayBox.Valid() {
		t.Fatal("离开之后不该还画压层")
	}
	if len(canvas.Rectangles()) != 0 || len(canvas.Texts()) != 0 {
		t.Fatalf("离开之后画面该是空的：rects=%d texts=%v",
			len(canvas.Rectangles()), textValues(canvas))
	}
}

// ── 3. 没有 sink（宿主不可用）→ over 不消费，也没有悬停态可画 ───────────────

func TestDropOverlayNotDrawnWithoutSink(t *testing.T) {
	p, env, _, canvas := fixture(t, nil)
	const w, h = 1180, 700
	if result := HandleFileDrop(over()); result.Handled {
		t.Fatal("没有 sink 时 FileDropOver 不该被消费")
	}
	drawOverlayOnce(t, p, env, canvas, w, h)
	if p.overlayBox.Valid() || len(canvas.Rectangles()) != 0 || len(canvas.Texts()) != 0 {
		t.Fatalf("没有 sink 时不该画压层：valid=%v rects=%d texts=%v",
			p.overlayBox.Valid(), len(canvas.Rectangles()), textValues(canvas))
	}
}

// ── 4. 亮色主题 → 亮色档原值（官方 `rgba(255, 255, 255, 0.7)`）──────────────

func TestDropOverlayUsesLightTokenInLightTheme(t *testing.T) {
	p, env, _, canvas := fixture(t, &fakeSink{})
	p.theme = dscore.Light()
	env.Theme = p.theme
	const w, h = 1180, 700
	HandleFileDrop(over())
	drawOverlayOnce(t, p, env, canvas, w, h)

	want := gfx.RGBA(255, 255, 255, dropMaskAlpha)
	rect, ok := maskRect(canvas, w, h)
	if !ok {
		t.Fatalf("压层必须铺满整窗，实际矩形：%v", canvas.Rectangles())
	}
	if rect.Color != want {
		// 官方亮色档：`OFF:dsh-client-ui-theme/src/styles/design-platform.css:188`
		t.Fatalf("亮色主题必须用官方亮色档 %v，实际 %v", want, rect.Color)
	}
}
