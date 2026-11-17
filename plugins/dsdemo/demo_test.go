package dsdemo

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// dsdemo 是**第三方插件的样例**——它示范的五段接口（ID / Requires / Register /
// Panels / draw）是别人照着抄的模板。样例错了，抄的人一起错。
//
// 所以这里测的不是"演示条好看不好看"，是那几条**会被抄走的行为**：
//   - F12 切换显隐，且别的键不抢；
//   - 收起之后**真的什么都不画**（不是画个透明的）；
//   - 「×」点了会收起。

func TestF12TogglesAndNothingElse(t *testing.T) {
	p := &Plugin{}
	p.shown = true

	// 别的键一律不认，必须让回去（抢了别人的键，别的插件就收不到）。
	for _, k := range []gfx.Key{gfx.KeyF1, gfx.KeyEnter, gfx.KeyEscape, gfx.KeyDown} {
		if p.HandleKey(gfx.KeyEvent{Key: k}) {
			t.Errorf("%v 被 dsdemo 吃掉了——它只该认 F12", k)
		}
	}
	if !p.shown {
		t.Fatal("按了别的键却把显隐切了")
	}

	// 带修饰键的 F12 不算：Ctrl+F12 是别的意思（而且 viva 的组合键路由不同）。
	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyF12, Modifiers: gfx.ModifierControl}) {
		t.Error("Ctrl+F12 不该被当成 F12")
	}
	if !p.shown {
		t.Fatal("带修饰键的 F12 不该切换显隐")
	}

	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyF12}) {
		t.Fatal("F12 应当被处理（返回 true），否则键盘路由会继续往下传")
	}
	if p.shown {
		t.Fatal("F12 之後应当收起")
	}
	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyF12}) || !p.shown {
		t.Fatal("再按一次 F12 应当展开")
	}
}

// drawOnce 画一帧，返回这一帧画出来的矩形数（浮层有背景，所以"画了"= 至少一个矩形）。
func drawOnce(t *testing.T, p *Plugin, click string) int {
	t.Helper()
	theme := dscore.Dark()
	p.theme = theme
	if p.prims == nil {
		p.prims = &dscore.Primitives{Theme: theme}
	}
	tree := &gfx.Tree{}
	feat := tree.Feature("demo-test")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	if click != "" {
		canvas.BeginEventPass(tree.ID(feat, click), gfx.ClickInfo{})
		defer canvas.EndEventPass()
	}
	env := &pluginapi.Env{
		Theme: theme, Data: pluginapi.Snapshot{Cur: "s"}, Act: pluginapi.Actions{},
		Tree: tree, Feat: feat, Canvas: canvas,
		Bounds: gfx.Rect{Width: 1182, Height: 832}, MainWidth: 902,
	}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		p.draw(f, env)
	})
	return len(canvas.Rectangles())
}

// TestHiddenDrawsNothing 钉住"收起 = 真的不画"。
//
// 画一个透明的/零高的浮层，看着一样，但它仍然占着 overlay 槽位、仍然参与命中——
// 用户会点到一片看不见的区域。
func TestHiddenDrawsNothing(t *testing.T) {
	p := &Plugin{}
	p.shown = false
	if n := drawOnce(t, p, ""); n != 0 {
		t.Fatalf("收起状态下画出了 %d 个矩形", n)
	}
}

// TestCloseButtonHides：点「×」收起。
func TestCloseButtonHides(t *testing.T) {
	p := &Plugin{}
	p.shown = true
	if n := drawOnce(t, p, ""); n == 0 {
		t.Fatal("展开状态下应当画出来")
	}
	drawOnce(t, p, "demo-close")
	if p.shown {
		t.Fatal("点了「×」之后应当收起")
	}
}

// TestOfficialOverlayGeometry 钉住**官方浮层底板的几何**，不是"看着差不多"。
//
// 每一条都对应一处官方声明，数值写进断言就是为了以后有人手改常量时先红：
//
//	距右上角 12px      FontNotice.module.css:45 `top: max(12px, …)`、:48 `right: 12px`
//	宽 ≤ 340px         FontNotice.module.css:32 `width: min(340px, calc(100vw - 24px))`
//	圆角 16px          FontNotice.module.css:39 `--dsw-radius-lg` → base.css:21 `16px`
//	描边 0.5px         viewer-status-banner.module.css:5 `border: 0.5px solid …`
//	内边距 12/8        CordisRunRow.module.css:106 `padding: 8px 12px`
//	底色 bg-layer-2    FontNotice.module.css:37
func TestOfficialOverlayGeometry(t *testing.T) {
	rects, env, theme := drawRects(t, &Plugin{shown: true})

	// 按底色认底板：描边层（BorderL2 那条 rgba 白）会画出同位置、外扩 0.5px 的矩形，
	// 所以不能只按"最大"挑，只能按底色挑。
	var (
		panel  *gfx.Rectangle
		border *gfx.Rectangle
	)
	for i := range rects {
		switch rects[i].Color {
		case theme.BgLayer2:
			if panel == nil {
				panel = &rects[i]
			}
		case theme.BorderL2:
			if border == nil {
				border = &rects[i]
			}
		}
	}
	if panel == nil {
		t.Fatal("没画出浮层底板（bg-layer-2）")
	}
	if border == nil {
		t.Fatal("没画出 0.5px 描边（viewer-status-banner.module.css:5）")
	}

	// 定宽底板：外沿（描边层）就是 min(340px, 100vw-24px)。
	// 1182-24 远大于 340，所以走 min 的 340 那一支。
	// viva 把 0.5px 描边画成向内收 0.5px 的轮廓，所以内层底色是 339，
	// 这里两层都断，顺带钉住"描边确实是 0.5 而不是 1"。
	if border.Bounds.Width != 340 {
		t.Fatalf("浮层外沿宽=%v，want 340（FontNotice.module.css:32）", border.Bounds.Width)
	}
	if panel.Bounds.Width != 339 {
		t.Fatalf("浮层底色层宽=%v，want 339（340 − 2×0.5 描边）", panel.Bounds.Width)
	}
	if right := env.Bounds.Width - (border.Bounds.X + border.Bounds.Width); right != 12 {
		t.Fatalf("浮层右边距=%v，want 12（FontNotice.module.css:48）", right)
	}
	if border.Bounds.Y != 12 {
		t.Fatalf("浮层上边距=%v，want 12（FontNotice.module.css:45）", border.Bounds.Y)
	}
	if border.Radius != theme.Radii.LG || theme.Radii.LG != 16 {
		t.Fatalf("浮层圆角=%v（Radii.LG=%v），want 16（base.css:21）", border.Radius, theme.Radii.LG)
	}
	if panel.Color != theme.BgLayer2 || border.Color != theme.BorderL2 {
		t.Fatalf("浮层配色=底%+v/边%+v，want bg-layer-2 + border-l2（FontNotice.module.css:37）",
			panel.Color, border.Color)
	}
}

// TestNarrowWindowFollowsOfficialClamp：官方是 `min(340px, calc(100vw - 24px))`，
// 窄窗必须让路，不能画到窗口外。
func TestNarrowWindowFollowsOfficialClamp(t *testing.T) {
	for _, tc := range []struct{ window, want float32 }{
		{1182, 340}, {364, 340}, {300, 276},
	} {
		if got := panelWidth(tc.window); got != tc.want {
			t.Errorf("panelWidth(%v)=%v，want %v（FontNotice.module.css:32）", tc.window, got, tc.want)
		}
	}
}

// TestCloseIconIsOfficialVector：关闭键画的是**官方矢量图标**，不是「×」字形。
//
// 官方界面里所有图标键都是 `<svg>`（MessageFeedbackActions.module.css:20-23
// `.action svg { width: 15px; height: 15px }`），逐个改用官方素材是本轮的目标；
// 这里钉住"关闭键落在 .action 几何里且确实画了矢量三角形"，字形退路不算过。
func TestCloseIconIsOfficialVector(t *testing.T) {
	theme := dscore.Dark()
	p := &Plugin{shown: true, theme: theme, prims: &dscore.Primitives{Theme: theme}}
	tree := &gfx.Tree{}
	feat := tree.Feature("demo-icon")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: theme, Data: pluginapi.Snapshot{Cur: "s"}, Act: pluginapi.Actions{},
		Tree: tree, Feat: feat, Canvas: canvas,
		Bounds: gfx.Rect{Width: 1182, Height: 832}, MainWidth: 902,
	}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		p.draw(f, env)
	})

	// 命中区就是 28×28 的 .action（QueueDock.module.css:222-223）。
	hit, ok := closeHit(canvas, tree.ID(feat, "demo-close"))
	if !ok {
		t.Fatal("关闭键没有命中区")
	}
	if hit.Bounds.Width != 28 || hit.Bounds.Height != 28 {
		t.Fatalf("关闭键 %vx%v，want 28x28（QueueDock.module.css:222-223）",
			hit.Bounds.Width, hit.Bounds.Height)
	}

	// 图标三角形必须落在命中区内：字形走的是 Text，这里能数出三角形就说明
	// 画的是矢量素材。颜色取 CircleIconButton 给的那个（label-tertiary，
	// 即官方 .action 静止态，QueueDock.module.css:231）。
	inside := 0
	for _, tri := range canvas.Triangles() {
		contained := true
		for _, pt := range tri.Points {
			if pt.X < hit.Bounds.X-0.01 || pt.Y < hit.Bounds.Y-0.01 ||
				pt.X > hit.Bounds.X+hit.Bounds.Width+0.01 ||
				pt.Y > hit.Bounds.Y+hit.Bounds.Height+0.01 {
				contained = false
				break
			}
		}
		if contained {
			inside++
		}
	}
	if inside == 0 {
		t.Fatal("关闭键里没有官方矢量图标（画成字形了？）")
	}

	// 图标本身 15×15：跨过它的三角形包围盒不能超出该边长。
	minX, minY := float32(1e9), float32(1e9)
	maxX, maxY := float32(-1e9), float32(-1e9)
	for _, tri := range canvas.Triangles() {
		if tri.Color != theme.LabelSecondary {
			continue
		}
		for _, pt := range tri.Points {
			if pt.X < hit.Bounds.X-0.01 || pt.X > hit.Bounds.X+hit.Bounds.Width+0.01 {
				continue
			}
			if pt.Y < hit.Bounds.Y-0.01 || pt.Y > hit.Bounds.Y+hit.Bounds.Height+0.01 {
				continue
			}
			if pt.X < minX {
				minX = pt.X
			}
			if pt.Y < minY {
				minY = pt.Y
			}
			if pt.X > maxX {
				maxX = pt.X
			}
			if pt.Y > maxY {
				maxY = pt.Y
			}
		}
	}
	if w := maxX - minX; w > 15.01 {
		t.Fatalf("图标宽=%v，超官方 .action svg 的 15px", w)
	}
	if h := maxY - minY; h > 15.01 {
		t.Fatalf("图标高=%v，超官方 .action svg 的 15px", h)
	}
}

// closeHit 找 id 对应的命中区（viva 把 Hit 记在 canvas.Hits()）。
func closeHit(canvas *gfx.Canvas, id gfx.ID) (gfx.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if hit.ID == id {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// drawRects 与 drawOnce 同一条装配路径，但把画出来的矩形和 env 一起返回，
// 供几何断言用（位置类断言要拿窗口宽高做参照）。
func drawRects(t *testing.T, p *Plugin) ([]gfx.Rectangle, *pluginapi.Env, *pluginapi.Theme) {
	t.Helper()
	theme := dscore.Dark()
	p.theme = theme
	if p.prims == nil {
		p.prims = &dscore.Primitives{Theme: theme}
	}
	tree := &gfx.Tree{}
	feat := tree.Feature("demo-geometry")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: theme, Data: pluginapi.Snapshot{Cur: "s"}, Act: pluginapi.Actions{},
		Tree: tree, Feat: feat, Canvas: canvas,
		Bounds: gfx.Rect{Width: 1182, Height: 832}, MainWidth: 902,
	}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		p.draw(f, env)
	})
	return canvas.Rectangles(), env, theme
}
