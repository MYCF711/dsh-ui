// geometry_test.go —— 几何/样式的**官方值**断言。
//
// 这一组测试的 want 全部直接来自官方原件（`OFF:pr/src/client/Card.module.css`
// 与 `dsh-client-ui-theme/src/styles/base.css` 的圆角阶梯），不是从实现反推的
// golden：改实现改不动它们，只有改回官方的数值才过。
//
// 与 plugin_test.go 的分工：那边测**行为**（查询/动作/状态机），这边测**长相**
// （外扩、圆角、骨架条、令牌），都用同一套 `fixture` / `drawFrame` 夹具真画一帧。
package dspluginreco

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
)

// TestOfficialGeometryConstants 钉住 `Card.module.css` 里每个 px 值。
// src 是官方文件与行号；数值改了这里必须一起改，且只能改回官方值。
func TestOfficialGeometryConstants(t *testing.T) {
	cases := []struct {
		name string
		got  float32
		want float32
		src  string
	}{
		{"header 纵向 padding", recoHeaderPadY, 10, "Card.module.css:1 padding:10px 20px"},
		{"header 横向 padding", recoHeaderPadX, 20, "Card.module.css:1 padding:10px 20px"},
		{"header 行列间距", recoHeaderGap, 12, "Card.module.css:1 gap:12px"},
		{"header 字号", recoHeaderFont, 12, "Card.module.css:1 font-size:12px"},
		{"header 行高", recoHeaderLineH, 18, "Card.module.css:1 line-height:1.5 × 12px"},
		{"badge 字号", recoHeaderFont, 12, "Card.module.css:2 font-size:12px"},
		{"body padding", recoBodyPad, 20, "Card.module.css:3 padding:20px"},
		{"details 外扩", recoDetailsBleed, 8, "Card.module.css:4 margin:-8px"},
		{"details padding", recoDetailsPad, 8, "Card.module.css:4 padding:8px"},
		{"title 字号", recoTitleFont, 16, "Card.module.css:7 font-size:16px"},
		{"title 字重", recoTitleWeight, 500, "Card.module.css:7 font-weight:500"},
		{"title 行高", recoTitleLineH, 24, "Card.module.css:7 line-height:1.5 × 16px"},
		{"description margin-top", recoDescMarginTop, 8, "Card.module.css:8 margin:8px 0 0"},
		{"description 字号", recoDescFont, 13, "Card.module.css:8 font-size:13px"},
		{"description 行高", recoDescLineH, 22.1, "Card.module.css:8 line-height:1.7 × 13px"},
		{"placeholder 宽", recoPlaceholderPercent, 50, "Card.module.css:9 width:50%"},
		{"placeholder 高", recoPlaceholderHeight, 10.4, "Card.module.css:9 height:0.8em × 13px"},
		{"status margin-top", recoStatusMarginTop, 16, "Card.module.css:13 margin:16px 0 0"},
		{"status padding-top", recoStatusPadTop, 12, "Card.module.css:13 padding-top:12px"},
		{"status 描边", recoStatusBorder, 0.5, "Card.module.css:13 border-top:0.5px"},
		{"status 字号", recoStatusFont, 12, "Card.module.css:13 font-size:12px"},
		{"status 行高", recoStatusLineH, 19.2, "Card.module.css:13 line-height:1.6 × 12px"},
		{"footer margin-top", recoFooterMarginTop, 16, "Card.module.css:10 margin-top:16px"},
		{"footer padding-top", recoFooterPadTop, 16, "Card.module.css:10 padding-top:16px"},
		{"footer 描边", recoFooterBorder, 0.5, "Card.module.css:10 border-top:0.5px"},
		{"footer gap", recoFooterGap, 12, "Card.module.css:10 gap:12px"},
		{"scope 字号", recoScopeFont, 12, "Card.module.css:11 font-size:12px"},
		{"scope 行高", recoScopeLineH, 18, "Card.module.css:11 line-height:1.5 × 12px"},
		{"actions gap", recoActionGap, 8, "Card.module.css:12 gap:8px"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %v，官方 %v（%s）", c.name, c.got, c.want, c.src)
		}
	}
	// 圆角取自官方阶梯 base.css:18-19（xs 4 / sm 8），不是自己挑的。
	radii := dscore.Dark().Radii
	if radii.SM != 8 {
		t.Errorf("--dsw-radius-sm = %v，官方 base.css:19 = 8", radii.SM)
	}
	if radii.XS != 4 {
		t.Errorf("--dsw-radius-xs = %v，官方 base.css:18 = 4", radii.XS)
	}
}

// deferred 把宿主调用挂起，用来观察"目录文案还没到"的那一帧
// （产品路径是 goroutine，测试里必须能停在 loading 态）。
type deferred struct{ fns []func() }

func (d *deferred) run(fn func()) { d.fns = append(d.fns, fn) }
func (d *deferred) flush() {
	fns := d.fns
	d.fns = nil
	for _, fn := range fns {
		fn()
	}
}

// fixtureWith 与 plugin_test.go 的 fixture 同构，只多给主题与执行器。
func fixtureWith(t *testing.T, theme *pluginapi.Theme, src RecoSource, run func(func())) (*Plugin, *pluginapi.Env, *gfx.Canvas) {
	t.Helper()
	tree := &gfx.Tree{}
	feat := tree.Feature("pluginreco-geometry-test")
	p := &Plugin{theme: theme, run: run}
	env := &pluginapi.Env{Theme: theme, Tree: tree, Feat: feat, Data: pluginapi.Snapshot{Cur: "s1"}}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	SetSource(src)
	t.Cleanup(func() { SetSource(nil); SetOnOpenDetails(nil) })
	return p, env, canvas
}

// detailsHitBounds 返回「查看详情」那块命中区的矩形。
func detailsHitBounds(t *testing.T, canvas *gfx.Canvas, want string) gfx.Rect {
	t.Helper()
	for _, hit := range canvas.Hits() {
		if hit.Label == want {
			return hit.Bounds
		}
	}
	t.Fatalf("没有 %q 的命中区；hits=%v", want, canvas.Hits())
	return gfx.Rect{}
}

// TestDetailsBlockBleedsLikeOfficial 钉 `.details{margin:-8px; padding:8px;
// width:calc(100% + 16px)}`（Card.module.css:4）落到画布上的结果。
//
// 画布 640 宽、`.body{padding:20px}` → 内容宽 600。于是：
//
//	按钮左缘 = 20 − 8 = 12，宽 = 600 + 16 = 616（官方 `calc(100% + 16px)`）
//	标题文字左缘 = 12 + 8 = 20（与 body 内容框左缘对齐，不额外内缩）
//	标题文字宽 = 616 − 16 = 600（官方文字区宽度）
func TestDetailsBlockBleedsLikeOfficial(t *testing.T) {
	src := &fakeSource{details: availableDetails("安全审计", "扫描依赖并报告风险。", false)}
	p, env, canvas := fixture(t, src)
	SetOnOpenDetails(func(string) {})
	props := propsFor("", nil)
	drawFrame(t, p, env, canvas, props, "")

	if got, want := detailsHitBounds(t, canvas, detailsLabel("安全审计")), (gfx.Rect{X: 12, Y: 50, Width: 616, Height: 70.1}); got != want {
		t.Errorf("`.details` 命中区 = %+v，官方几何 = %+v（margin:-8px / width:calc(100%%+16px)）", got, want)
	}
	var title gfx.Text
	for _, text := range canvas.Texts() {
		if text.Value == "安全审计" {
			title = text
		}
	}
	if title.Value == "" {
		t.Fatalf("没有画出目录标题；texts=%v", texts(canvas))
	}
	if title.Bounds.X != 20 {
		t.Errorf("标题左缘 = %v，官方 = body 内容框左缘 20（`margin:-8px` + `padding:8px` 相抵）", title.Bounds.X)
	}
	if title.Bounds.Width != 600 {
		t.Errorf("标题文字宽 = %v，官方 = 600（`.details` 的 616 减去左右各 8）", title.Bounds.Width)
	}
	if title.Bounds.Height != recoTitleLineH {
		t.Errorf("标题行盒高 = %v，官方 line-height:1.5 × 16px = %v", title.Bounds.Height, recoTitleLineH)
	}
}

// TestPlaceholderSkeletonHoldsTheDescriptionLine 钉 `.placeholder`
// （Card.module.css:9）：描述到达前画一条 50% 宽、0.8em 高、4px 圆角、
// 底色 `--dsw-alias-bg-skeleton` 的骨架条，并且它占的就是描述那一行
// （描述到达前后 `.details` 高度一致 → 卡片不跳高）。
func TestPlaceholderSkeletonHoldsTheDescriptionLine(t *testing.T) {
	theme := dscore.Dark()
	jobs := &deferred{}
	src := &fakeSource{details: availableDetails("安全审计", "扫描依赖并报告风险。", false)}
	p, env, canvas := fixtureWith(t, theme, src, jobs.run)
	SetOnOpenDetails(func(string) {})
	props := propsFor("", nil)

	// ── 加载中：只有骨架条，没有描述文字 ──
	drawFrame(t, p, env, canvas, props, "")
	if hasText(canvas, "扫描依赖并报告风险。") {
		t.Fatalf("目录文案还没到就画了描述；texts=%v", texts(canvas))
	}
	var skeleton *gfx.Rectangle
	for i, rect := range canvas.Rectangles() {
		if rect.Color == theme.BgSkeleton {
			skeleton = &canvas.Rectangles()[i]
		}
	}
	if skeleton == nil {
		t.Fatalf("加载态没有画骨架条（`--dsw-alias-bg-skeleton` = %+v）；rects=%v", theme.BgSkeleton, canvas.Rectangles())
	}
	if skeleton.Bounds.Width != 300 {
		t.Errorf("骨架条宽 = %v，官方 `width:50%%` = 300（内容的 50%%）", skeleton.Bounds.Width)
	}
	if skeleton.Bounds.Height != recoPlaceholderHeight {
		t.Errorf("骨架条高 = %v，官方 `height:0.8em` = %v（0.8 × 13px）", skeleton.Bounds.Height, recoPlaceholderHeight)
	}
	if skeleton.Radius != theme.Radii.XS {
		t.Errorf("骨架条圆角 = %v，官方 `--dsw-radius-xs` = %v", skeleton.Radius, theme.Radii.XS)
	}
	if skeleton.Bounds.X != 20 {
		t.Errorf("骨架条左缘 = %v，官方 = 描述行左缘 20", skeleton.Bounds.X)
	}
	// 垂直居中在 22.1px 的行盒里：行盒 90..112.1（标题底 82 + margin-top 8）。
	if want := float32(90) + (recoDescLineH-skeleton.Bounds.Height)/2; skeleton.Bounds.Y != want {
		t.Errorf("骨架条上缘 = %v，官方 `vertical-align:middle` 于行盒内 = %v", skeleton.Bounds.Y, want)
	}
	loadingHeight := detailsHitBounds(t, canvas, detailsLabel("审计插件")).Height

	// ── 文案到达：换成描述，高度不变 ──
	jobs.flush()
	drawFrame(t, p, env, canvas, props, "")
	if !hasText(canvas, "扫描依赖并报告风险。") {
		t.Fatalf("文案到了却没画描述；texts=%v", texts(canvas))
	}
	for _, rect := range canvas.Rectangles() {
		if rect.Color == theme.BgSkeleton {
			t.Fatalf("文案到了还留着骨架条：%+v", rect)
		}
	}
	if got := detailsHitBounds(t, canvas, detailsLabel("安全审计")).Height; got != loadingHeight {
		t.Errorf("描述到达前后 `.details` 高度变了（%v → %v），官方靠骨架条占位避免跳高", loadingHeight, got)
	}
}

// TestHeaderUsesModulePlatformBackground 钉 `.header{background:var(--dsw-alias-bg-module-platform)}`
// （Card.module.css:1）。
//
// 为什么值得单独测：深色下 `bg-layer-3` 与 `bg-module-platform` 恰好同值
// （都指到 neutral-bluish-800），只有浅色才分得开——浅色 layer-3 是纯白 #ffffff，
// module-platform 是 neutral-bluish-60 #f9fafb（design-platform.css:189/341）。
func TestHeaderUsesModulePlatformBackground(t *testing.T) {
	for _, tc := range []struct {
		name  string
		theme *pluginapi.Theme
	}{{"dark", dscore.Dark()}, {"light", dscore.Light()}} {
		t.Run(tc.name, func(t *testing.T) {
			src := &fakeSource{details: availableDetails("安全审计", "扫描依赖并报告风险。", false)}
			p, env, canvas := fixtureWith(t, tc.theme, src, func(fn func()) { fn() })
			props := propsFor("", nil)
			drawFrame(t, p, env, canvas, props, "")

			var header *gfx.Rectangle
			for i, rect := range canvas.Rectangles() {
				if rect.Bounds.Width == 640 && rect.Color == tc.theme.BgModulePlatform {
					header = &canvas.Rectangles()[i]
				}
			}
			if header == nil {
				t.Fatalf("header 底色不是 `--dsw-alias-bg-module-platform`（%+v）；rects=%v", tc.theme.BgModulePlatform, canvas.Rectangles())
			}
			// `.header{padding:10px 20px; line-height:1.5; font-size:12px}` → 18 + 20 = 38。
			if got, want := header.Bounds.Height, float32(recoHeaderLineH+2*recoHeaderPadY); got != want {
				t.Errorf("header 高 = %v，官方 = padding 10×2 + 行高 18 = %v", got, want)
			}
			if got, want := header.Bounds.Width, float32(640); got != want {
				t.Errorf("header 宽 = %v，官方 display:flex 通栏 = %v", got, want)
			}
		})
	}
}

// TestDetailsHitCoversTitleAndDescription 钉"整块是按钮"（Card.module.css:4 的
// `.details` 是 `<button>`，Card.tsx:131-137）：命中区必须同时盖住描述那一行，
// 不能只有标题行可点。
func TestDetailsHitCoversTitleAndDescription(t *testing.T) {
	src := &fakeSource{details: availableDetails("安全审计", "扫描依赖并报告风险。", false)}
	p, env, canvas := fixtureWith(t, dscore.Dark(), src, func(fn func()) { fn() })
	var opened []string
	SetOnOpenDetails(func(name string) { opened = append(opened, name) })
	props := propsFor("", nil)
	drawFrame(t, p, env, canvas, props, "")

	var desc gfx.Text
	for _, text := range canvas.Texts() {
		if text.Value == "扫描依赖并报告风险。" {
			desc = text
		}
	}
	if desc.Value == "" {
		t.Fatalf("没有画描述；texts=%v", texts(canvas))
	}
	block := detailsHitBounds(t, canvas, detailsLabel("安全审计"))
	if desc.Bounds.Y < block.Y || desc.Bounds.Y+desc.Bounds.Height > block.Y+block.Height {
		t.Fatalf("命中区 %+v 没盖住描述行 %+v：官方整块 `.details` 都是按钮", block, desc.Bounds)
	}
	// 点在描述上也算「查看详情」（官方整块按钮）。
	drawFrame(t, p, env, canvas, props, "reco-details-call-1")
	if len(opened) != 1 || opened[0] != "acme.audit" {
		t.Fatalf("点详情块打开了 %v，应为 [acme.audit]", opened)
	}
}

// TestCardUsesNoIcons 钉一个**否证**：官方这张卡零图标（`src/client/` 下没有
// 任何 SVG / Icon 引用），所以本包也不许出现图标素材，更不许手画一枚顶替。
func TestCardUsesNoIcons(t *testing.T) {
	src := &fakeSource{details: availableDetails("安全审计", "扫描依赖并报告风险。", false)}
	p, env, canvas := fixtureWith(t, dscore.Dark(), src, func(fn func()) { fn() })
	SetOnOpenDetails(func(string) {})
	props := propsFor("", nil)
	drawFrame(t, p, env, canvas, props, "")
	// 图标在画布上是三角网格；这张卡只有文字、矩形与命中区。
	if got := canvas.Triangles(); len(got) != 0 {
		t.Fatalf("卡片画出了 %d 块矢量（官方此卡无图标）：%v", len(got), got)
	}
}
