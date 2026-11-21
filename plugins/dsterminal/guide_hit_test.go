//go:build darwin || windows || linux

// guide_hit_test.go —— 向导卡**点击面**的可达性判据：点 trigger 开菜单、点卡片空白开终端。
//
// # 这一条闭合的是什么
//
// 卡片的点击面原来画成一枚绝对层：`ui.Row(f).Absolute().Inset(0).Hit(cardID, …)`，
// 指望它"垫在卡片下面、让 trigger 拿自己的点击"。但 Viva 的绝对元素
// 「is painted after the in-flow children in the order declared, and so **wins pointer
// hits over them**」（`viva@v0.6.12-ow/base/absolute.go:5-11` 原文）——绝对层恒赢
// 流内子元素，**与声明先后无关**，所以那枚 20×20 的 trigger 在卡片上永远拿不到指针：
// 指针停上去 `Snapshot.Hovered` 是**卡片**身份，点下去只开终端、shell 菜单打不开。
//
// 官方的形状不是"绝对层盖住内容"：`.main{position:absolute;inset:0}` 是垫在**内容下面**
// 的那层 ghost 按钮（`TerminalGuide.tsx:32-83`），`.trigger` 排在它上面。Viva 里与它
// 同构的是**卡片本体**的命中区：`base/paint.go:88,150` 先登记本体命中区、再画子元素，
// 所以本体在最底层、子元素（trigger）在它前面。几何仍按官方 `inset:0` 落 **padding box**
// ——`solveAbsolute` 从 `origin+border` 起、宽高各减 2×border，所以卡片命中区用
// `HitOutset` 的负值把边框那一圈削掉，**逐像素等同**原来的绝对层（本条 ⑥ 钉住）。
//
// # 判据（每条都能被一个改动单独弄红）
//
//	① trigger 仍在卡片之内、仍是官方 20×20 几何；
//	② 指针停在 trigger 中心时 `Snapshot.Hovered` 与 `Driver().HitTest` 都是 **trigger**；
//	③ 点 trigger **打开 shell 菜单**（`guideMenuOpen` 翻转 + 菜单行真的画出来），
//	   **不打开终端**；
//	④ 菜单开着时点卡片空白：只关菜单，不开终端（原来那条"点外只关菜单"的行为不变）；
//	⑤ 菜单关着时点卡片空白：**打开终端**（一次、不带 shellPath），菜单不动；
//	⑥ 卡片命中区几何没变：仍是官方 `inset:0` 的 padding box（边框那 0.5px 点击不到）。
package dsterminal

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// 夹具窗口与内边距：卡片宽度 = 窗口宽 - 2×内边距，⑥ 的期望几何由这两条算出来。
const (
	guideHitWindowWidth          = 420
	guideHitWindowHeight         = 200
	guideHitPad          float32 = 16
)

// guideHitHarness 是一张真窗口里的向导卡：指针走 `driver.Press/Move/Release` →
// 壳的 `cfg.OnPointer` → `ui.Page`，与产品里那条链一样（`cmd/app/main.go` 的 cfg.OnPointer）。
type guideHitHarness struct {
	t           *testing.T
	session     *headless.Session
	p           *Plugin
	tree        *gfx.Tree
	feat        gfx.Scope
	invalidator *app.Invalidator

	// opened 记 `Actions.OpenRightTab` / `Actions.OpenRightTabParams` 收到的 kind；
	// params 记后者的参数（压在 opened 同一序上）。
	opened []string
	params []pluginapi.RightTabParams
}

// openGuideHitHarness 打开一页只有这张向导卡的真窗口（headless）。
func openGuideHitHarness(t *testing.T) *guideHitHarness {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("terminal-guide-hit")
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	h := &guideHitHarness{t: t, p: &Plugin{}, tree: tree, feat: feat}

	var invalidator app.Invalidator
	config := app.Config{
		Title: "terminal-guide-hit", Width: guideHitWindowWidth, Height: guideHitWindowHeight,
		Tree: tree, WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: bounds.Width,
			Data: pluginapi.Snapshot{Cur: "s1"},
			GuideEntry: &pluginapi.GuideEntryProps{
				EntryID: "new", Kind: RightTabTerminal,
				Title: zhGuideTitle, Description: zhGuideDescription,
			},
		}
		env.Act.OpenRightTab = func(kind string) { h.opened = append(h.opened, kind) }
		env.Act.OpenRightTabParams = func(kind string, params pluginapi.RightTabParams) {
			h.opened = append(h.opened, kind)
			h.params = append(h.params, params)
		}
		env.Act.TerminalLaunchShells = func() pluginapi.TerminalShellChoices {
			return pluginapi.TerminalShellChoices{
				Phase: pluginapi.TerminalShellsReady, Shells: guideMenuShells, Selected: "/bin/zsh",
			}
		}
		env.Act.TerminalSelectShell = func(string) {}
		page.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).FillWidth().Pad(guideHitPad).Children(func() {
				h.p.drawGuideEntry(f, env)
			})
		})
	}
	config.OnPointer = func(ev gfx.PointerEvent) gfx.EventResult { return page.Pointer(ev) }
	h.invalidator = &invalidator
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	h.session = session
	h.pump()
	h.pump()
	return h
}

// pump 推进一帧。
func (h *guideHitHarness) pump() {
	h.t.Helper()
	h.invalidator.Invalidate()
	if err := h.session.Driver().Pump(); err != nil {
		h.t.Fatal(err)
	}
}

// pumpTwice 在指针事件之后推进两帧（事件那一帧与重画那一帧）。
func (h *guideHitHarness) pumpTwice() {
	h.t.Helper()
	h.pump()
	h.pump()
}

// hitRect 读一份命中的矩形；没有就判失败（读不到说明画都没画出来）。
func (h *guideHitHarness) hitRect(name string) gfx.Rect {
	h.t.Helper()
	id := h.tree.ID(h.feat, name)
	if id.IsZero() {
		h.t.Fatalf("向导卡上没有 %q 这个身份（Hit 没接上）", name)
	}
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == id && hit.Bounds.Width > 0 && hit.Bounds.Height > 0 {
			return hit.Bounds
		}
	}
	h.t.Fatalf("这一帧里没有 %q 的命中区（Hits=%v）", name, h.session.Snapshot().Hits)
	return gfx.Rect{}
}

// moveTo 把指针移到某点并重画。
func (h *guideHitHarness) moveTo(x, y float32) {
	h.t.Helper()
	if err := h.session.Move(x, y); err != nil {
		h.t.Fatal(err)
	}
	h.pumpTwice()
}

// clickAt 在某个坐标上真点一下（按下 + 抬起）并重画。
func (h *guideHitHarness) clickAt(x, y float32) {
	h.t.Helper()
	if err := h.session.Click(x, y); err != nil {
		h.t.Fatal(err)
	}
	h.pumpTwice()
}

// hitAt 读某个坐标会打到谁。
func (h *guideHitHarness) hitAt(x, y float32) (gfx.ID, bool) {
	h.t.Helper()
	return h.session.Driver().HitTest(x, y)
}

// texts 收集这一帧画过的所有文字（菜单行也在里面）。
func (h *guideHitHarness) texts() map[string]bool {
	h.t.Helper()
	out := map[string]bool{}
	for _, text := range h.session.Snapshot().Texts {
		out[text.Value] = true
	}
	return out
}

// inside 判断 rect 是否整块落在外层里（含边界）。
func inside(rect, outer gfx.Rect) bool {
	return rect.X >= outer.X && rect.Y >= outer.Y &&
		rect.X+rect.Width <= outer.X+outer.Width &&
		rect.Y+rect.Height <= outer.Y+outer.Height
}

// mid 取矩形中心。
func mid(rect gfx.Rect) (float32, float32) { return rect.X + rect.Width/2, rect.Y + rect.Height/2 }

// TestGuideCardClickSurfaceKeepsTheTriggerReachable 是本单主判据（见文件头 ①–⑥）：
// **点 trigger** 与 **点卡片空白** 必须走两条不同的路，而且两条都正确。
func TestGuideCardClickSurfaceKeepsTheTriggerReachable(t *testing.T) {
	h := openGuideHitHarness(t)
	cardID := h.tree.ID(h.feat, "terminal-guide-entry")
	triggerID := h.tree.ID(h.feat, "terminal-guide-shell")

	card := h.hitRect("terminal-guide-entry")
	trigger := h.hitRect("terminal-guide-shell")

	// ① trigger 仍在卡片之内、仍是官方 20×20（`.trigger{height:20px;width:20px}`）。
	if trigger.Width != guideTriggerSize || trigger.Height != guideTriggerSize {
		t.Fatalf("trigger 命中区 = %+v，官方 `.trigger` 是 %v×%v",
			trigger, guideTriggerSize, guideTriggerSize)
	}
	if !inside(trigger, card) {
		t.Fatalf("trigger %+v 不在卡片命中区 %+v 之内", trigger, card)
	}

	// ② 指针停在 trigger 中心：悬停身份必须是 **trigger**，不是卡片。
	tx, ty := mid(trigger)
	h.moveTo(tx, ty)
	hovered := h.session.Snapshot().Hovered
	if hovered != triggerID {
		t.Fatalf("指针停在 trigger 中心 (%v,%v) 时悬停身份 = %v，want %v（卡片是 %v）",
			tx, ty, hovered, triggerID, cardID)
	}
	if id, ok := h.hitAt(tx, ty); !ok || id != triggerID {
		t.Fatalf("`HitTest` 在 trigger 中心 (%v,%v) 回 %v（ok=%v），want %v",
			tx, ty, id, ok, triggerID)
	}

	// ③ 点 trigger：打开 shell 菜单，**不**打开终端。
	h.clickAt(tx, ty)
	if !h.p.guideMenuOpen {
		t.Fatal("点 trigger 没有打开 shell 菜单（guideMenuOpen 仍为 false）")
	}
	if len(h.opened) != 0 {
		t.Fatalf("点 trigger 不该打开终端，实际打开了 %v", h.opened)
	}
	texts := h.texts()
	for _, shell := range guideMenuShells {
		if !texts[shell.Name] {
			t.Fatalf("菜单开着但没画出 shell 行 %q：%v", shell.Name, h.texts())
		}
	}

	// ③' 菜单开着时 trigger 仍然可达：再点一下只关菜单（旧实现这一档也是在的，
	// 因为绝对层在菜单展开时不登记；修完不能把它弄丢）。
	h.clickAt(tx, ty)
	if h.p.guideMenuOpen {
		t.Fatal("菜单开着时再点 trigger 没有关掉菜单")
	}
	if len(h.opened) != 0 {
		t.Fatalf("菜单开着时点 trigger 不该打开终端，实际 %v", h.opened)
	}
	h.clickAt(tx, ty)
	if !h.p.guideMenuOpen {
		t.Fatal("关掉之后再点 trigger 没有重新打开菜单")
	}

	// ④ 菜单开着时点卡片空白：只关菜单，不开终端（既有行为不变）。
	bx, by := card.X+guideEntryBorder+2, card.Y+card.Height/2
	h.clickAt(bx, by)
	if h.p.guideMenuOpen {
		t.Fatal("菜单开着时点卡片空白没有关掉菜单")
	}
	if len(h.opened) != 0 {
		t.Fatalf("菜单开着时点卡片空白不该打开终端，实际 %v", h.opened)
	}

	// ⑤ 菜单关着时点卡片空白：打开终端一次、不带 shellPath，菜单不动。
	h.clickAt(bx, by)
	if len(h.opened) != 1 || h.opened[0] != RightTabTerminal {
		t.Fatalf("点卡片空白该打开一次终端，实际 %v", h.opened)
	}
	if len(h.params) != 0 {
		t.Fatalf("点卡片空白不该带 shellPath，实际 %+v", h.params)
	}
	if h.p.guideMenuOpen {
		t.Fatal("点卡片空白不该打开菜单")
	}

	// ⑥ 卡片命中区几何没变：官方 `inset:0` 落的是 padding box，
	// 所以左缘 / 宽度都比卡片矩形各少一个 0.5 的边框（`guideEntryBorder`）。
	wantX := guideHitPad + guideEntryBorder
	wantWidth := float32(guideHitWindowWidth) - 2*guideHitPad - 2*guideEntryBorder
	if card.X != wantX || card.Width != wantWidth {
		t.Fatalf("卡片命中区 X/Width = %v/%v，want %v/%v（官方 `inset:0` 的 padding box）",
			card.X, card.Width, wantX, wantWidth)
	}
	// 边框那一圈（0.5px）在旧实现里点不到卡片，现在也必须一样。
	if id, ok := h.hitAt(guideHitPad+guideEntryBorder/2, card.Y+card.Height/2); ok && id == cardID {
		t.Fatalf("卡片边框那一圈（x=%v）现在能点中卡片了：命中区被放大到了边框盒",
			guideHitPad+guideEntryBorder/2)
	}
}
