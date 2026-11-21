//go:build darwin || windows || linux

// guide_trigger_test.go —— `.trigger` 的**悬停行为**判据（不是"读一个常量"）。
//
// # 官方这两行到底说了什么
//
// `.trigger` 自己**没有**悬停规则。`TerminalGuide.module.css` 全文 12 行，与 trigger
// 有关的只有两行：
//
//	.trigger     { flex:none; height:20px; width:20px; padding:0; border-radius:4px }   ← :9
//	.trigger svg { width:14px; height:14px; color: var(--dsw-alias-label-tertiary) }    ← :10
//
// 悬停底来自它的组件身份：官方那个 anchor 是 primitives 的
// `<Button variant="ghost" className={css.trigger}>`（`TerminalGuide.tsx:73`），而
// primitives 的 ghost 变体（`index-CSAdOBx6.css` 同一份产物）写着：
//
//	._ghost_1ulci_46:hover:not(:disabled)  { background: var(--dsw-alias-interactive-bg-hover) }   ← byte 8644
//	._ghost_1ulci_46:active:not(:disabled) { background: var(--dsw-alias-interactive-bg-active) }  ← byte 8732
//
// 本仓 `pluginapi.Theme.Hover` 就是 `--dsw-alias-interactive-bg-hover`、
// `InteractiveActive` 就是 `--dsw-alias-interactive-bg-active`（`plugins/dscore/dscore.go`
// 亮暗两档都带令牌注释），所以这两个状态是**复用**，不是新造。
//
// # 为什么这一支画的是"孤立的一枚 trigger"而不是整张向导卡
//
// 向导卡里那枚 trigger **现在拿不到指针**：卡片自己的点击面是
// `ui.Row(f).Absolute().Inset(0)`，而 Viva 的绝对元素「is painted after the in-flow
// children in the order declared, and so **wins pointer hits over them**」
// （`viva@v0.6.12-ow/base/absolute.go:5-11` 原文）——绝对层恒赢流内子元素，与声明
// 先后无关。实测：指针停在 trigger 中心时 `Snapshot.Hovered` 是**卡片**的身份，
// 点下去也打不开菜单（`menu.go:225-227` 那句「先声明它才能让后面的 trigger 拿到
// 自己的点击」是错的）。这是既存的**可达性缺陷**，不属于本次 C 类零代价改动
// （修它要动卡片布局/锚点，见账本登记），所以这里把 trigger 单独画在一页上，
// 钉住**画笔本身**的悬停行为；可达性另有登记。
//
// # 判据（每条都能被一个改动单独弄红）
//
//	① 指针真的进了 trigger：`Snapshot.Hovered` 就是它的身份 id；
//	② 悬停时**外观变了**：这一帧多出一块 `--dsw-alias-interactive-bg-hover` 的矩形，
//	   且 bounds **正好是 trigger 的 20×20**；
//	③ 不悬停时**没变**：指针不在上面时连拍两帧，画面逐像素相同（不是"一直在动"混过去）；
//	   而且那两帧里**不许**有 trigger 那一块悬停底；
//	④ chevron 的字色**恒定**是 label-tertiary：两帧里 trigger 矩形内都不许出现
//	   label-primary 的墨迹 —— 官方 `.trigger svg` 没有状态，这条钉住"没再自己加换色"。
package dsterminal

import (
	"image"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// guideTriggerIsolated 画一页只有一枚 `.trigger` 的页面，返回会话与它的身份/矩形。
func guideTriggerIsolated(t *testing.T) (*headless.Session, gfx.ID, gfx.Rect) {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("terminal-guide-trigger")
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	p := &Plugin{}

	var invalidator app.Invalidator
	config := app.Config{
		Title: "terminal-guide-trigger", Width: 200, Height: 120, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: bounds.Width,
			Data: pluginapi.Snapshot{Cur: "s1"},
		}
		env.Act.OpenRightTab = func(string) {}
		env.Act.OpenRightTabParams = func(string, pluginapi.RightTabParams) {}
		env.Act.TerminalLaunchShells = func() pluginapi.TerminalShellChoices {
			return pluginapi.TerminalShellChoices{
				Phase: pluginapi.TerminalShellsReady, Shells: guideMenuShells, Selected: "/bin/zsh",
			}
		}
		env.Act.TerminalSelectShell = func(string) {}
		page.Draw(canvas, bounds, func(f ui.C) {
			ui.Row(f).FillWidth().Pad(16).Items(ui.Center).Children(func() {
				p.drawGuideTrigger(f, env, theme)
			})
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	invalidator.Invalidate()
	for i := 0; i < 5; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	id := tree.ID(feat, "terminal-guide-shell")
	if id.IsZero() {
		t.Fatalf("trigger 没有身份（drawGuideTrigger 的 Hit 没接上）")
	}
	var box gfx.Rect
	for _, hit := range session.Snapshot().Hits {
		if hit.ID == id {
			box = hit.Bounds
		}
	}
	if box.Width != guideTriggerSize || box.Height != guideTriggerSize {
		t.Fatalf("trigger 命中区 = %+v，官方 `.trigger{width:20px;height:20px}`", box)
	}
	return session, id, box
}

// guideTriggerPump 推进两帧（指针事件之后要看**新**帧）。
func guideTriggerPump(t *testing.T, session *headless.Session) {
	t.Helper()
	for i := 0; i < 2; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
}

// guideTriggerImage 取当前帧的像素。
func guideTriggerImage(t *testing.T, session *headless.Session) *image.RGBA {
	t.Helper()
	img, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// guidePixelDiffIn 数两块图在 rect 里有多少个像素不同。
func guidePixelDiffIn(a, b *image.RGBA, rect gfx.Rect) int {
	width, height := a.Bounds().Dx(), a.Bounds().Dy()
	x0 := max(int(rect.X), 0)
	y0 := max(int(rect.Y), 0)
	x1 := min(int(rect.X+rect.Width), width)
	y1 := min(int(rect.Y+rect.Height), height)
	diff := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				diff++
			}
		}
	}
	return diff
}

// guideTriggerHoverRect 在这一帧里找「bounds 正好是 trigger 那块」的 hover 底矩形。
func guideTriggerHoverRect(snapshot app.Snapshot, box gfx.Rect, hover ui.Color) (app.Primitive, bool) {
	for _, prim := range snapshot.Primitives {
		if prim.Kind != app.RectanglePrimitive || !sameRGB(prim.Color, hover) {
			continue
		}
		if prim.Bounds.X == box.X && prim.Bounds.Y == box.Y &&
			prim.Bounds.Width == box.Width && prim.Bounds.Height == box.Height {
			return prim, true
		}
	}
	return app.Primitive{}, false
}

// TestGuideTriggerHoverComesFromGhostButton 是这一支的行为判据（见文件头 ①②③④）。
func TestGuideTriggerHoverComesFromGhostButton(t *testing.T) {
	theme := dscore.Dark()
	session, id, box := guideTriggerIsolated(t)
	triggerRect := gfx.Rect{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height}

	// ③ 指针停在角落：连拍两帧画面必须一样，且不许有 trigger 的悬停底。
	if err := session.Move(1, 1); err != nil {
		t.Fatal(err)
	}
	guideTriggerPump(t, session)
	rest := session.Snapshot()
	if rest.Hovered == id {
		t.Fatalf("指针在 (1,1) 却报告悬停的是 trigger（id=%v）", rest.Hovered)
	}
	if prim, ok := guideTriggerHoverRect(rest, triggerRect, theme.Hover); ok {
		t.Fatalf("没悬停的帧里已铺了 trigger 的悬停底（%+v）", prim.Bounds)
	}
	restA, restB := guideTriggerImage(t, session), guideTriggerImage(t, session)
	if diff := guidePixelDiffIn(restA, restB, triggerRect); diff != 0 {
		t.Fatalf("没有指针事件时 trigger 那一块两帧之间变了 %d 个像素（画面不安定，"+
			"后面的悬停判据就不可信）", diff)
	}

	// ① 指针移到 trigger 中心，悬浮身份必须就是它。
	if err := session.Move(box.X+box.Width/2, box.Y+box.Height/2); err != nil {
		t.Fatal(err)
	}
	guideTriggerPump(t, session)
	hovered := session.Snapshot()
	if hovered.Hovered != id {
		t.Fatalf("指针在 trigger 中心 (%.1f,%.1f) 时悬停身份 = %v，want %v",
			box.X+box.Width/2, box.Y+box.Height/2, hovered.Hovered, id)
	}

	// ② 外观**变了**，而且变的就是 ghost Button 那档悬停底（20×20、圆角 4）。
	if diff := guidePixelDiffIn(restA, guideTriggerImage(t, session), triggerRect); diff == 0 {
		t.Fatalf("指针进入 trigger %+v 后那一块**一个像素都没变**：没有悬停底。"+
			"官方 primitives `._ghost_1ulci_46:hover{background:var(--dsw-alias-interactive-bg-hover)}`"+
			"（index-CSAdOBx6.css byte 8644）", triggerRect)
	}
	prim, ok := guideTriggerHoverRect(hovered, triggerRect, theme.Hover)
	if !ok {
		t.Fatalf("悬停帧没有 bounds 正好是 %+v 的 interactive-bg-hover 矩形"+
			"（trigger 的悬停底没走 ghost Button 那档）", triggerRect)
	}
	// 圆角 4 = `--dsw-radius-xs`（`.trigger{border-radius:4px}`，TerminalGuide.module.css:9）。
	if prim.Radius != 4 {
		t.Fatalf("悬停底圆角 = %v，官方 `.trigger{border-radius:4px}`", prim.Radius)
	}

	// ③' 按住不放时换成 ghost 的 `:active` 底：
	//     `._ghost_1ulci_46:active:not(:disabled){background:var(--dsw-alias-interactive-bg-active)}`
	//     （byte 8732），令牌是 `pluginapi.Theme.InteractiveActive`。
	hoverImage := guideTriggerImage(t, session)
	if err := session.Press(box.X+box.Width/2, box.Y+box.Height/2); err != nil {
		t.Fatal(err)
	}
	guideTriggerPump(t, session)
	pressed := session.Snapshot()
	if _, ok := guideTriggerHoverRect(pressed, triggerRect, theme.InteractiveActive); !ok {
		t.Fatalf("按住 trigger 时没有 interactive-bg-active 的底（官方 ghost 的 :active 档）")
	}
	// 外观确实又变了一次（不是"按下去没反应"）。Viva 把 pressed 档叠在 hover 档之上画，
	// 所以判据用像素差而不是"hover 那层消失了"——视觉结果与官方 `:active` 覆盖 `:hover` 相同。
	if diff := guidePixelDiffIn(hoverImage, guideTriggerImage(t, session), triggerRect); diff == 0 {
		t.Fatalf("按住 trigger 后那一块一个像素都没变：`:active` 档没生效")
	}
	if err := session.Release(box.X+box.Width/2, box.Y+box.Height/2); err != nil {
		t.Fatal(err)
	}
	guideTriggerPump(t, session)

	// ④ chevron 字色恒定 label-tertiary：两帧里都有 tertiary 墨迹、都不许有 primary 墨迹。
	for name, snapshot := range map[string]app.Snapshot{"不悬停": rest, "悬停": hovered} {
		if _, count := triInkRGB(snapshot, theme.LabelTertiary, triggerRect); count == 0 {
			t.Fatalf("%s帧里 trigger 矩形内没有 label-tertiary 的 chevron 墨迹：%+v", name, triggerRect)
		}
		if ink, count := triInkRGB(snapshot, theme.LabelPrimary, triggerRect); count != 0 {
			t.Fatalf("%s帧里 trigger 矩形内出现 label-primary 的墨迹（%d 条，%+v）："+
				"官方 `.trigger svg{color:label-tertiary}` 没有状态，不该换色", name, count, ink)
		}
	}
}
