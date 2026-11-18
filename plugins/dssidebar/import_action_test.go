package dssidebar

// 官方 rc.5 `sidebar.session.action` 座位的断言：
//   1. 壳没接这条能力时**不画**，旧几何一像素不变；
//   2. 接了就画在官方宽档几何上（38×38、与 New session 间距 6、右端对齐）；
//   3. 点一下**恰好**调用一次壳动作。
//
// 官方几何来源：OFF:dsh-deepseek-chat-import/src/client/ImportSessions.module.css:3-24
// （`.trigger.wide` 38×38）与 SidebarRoot.module.css:424-430（`.newSessionRow` gap:6px）。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// TestImportActionSeatAbsentWithoutShellCapability 钉住"没能力就不画"：
// 老壳 / 无头 / 测试环境里 `ImportDeepSeekSessions == nil`，体积必须与官方
// slot 空着时一致，不许留一个点不动的空位。
func TestImportActionSeatAbsentWithoutShellCapability(t *testing.T) {
	p, _, env, _, canvas, _ := searchFixture(t)
	env.Act.ImportDeepSeekSessions = nil
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: 256, Height: 420}, func(f ui.C) {
		p.drawHeader(f, env)
	})
	boxes := p.frameBoxes
	newRect := boxes.NewSession.Rect()
	// 没有触发器时新会话按钮吃掉整行（官方 `.newSession` 的 flex:1 1 auto）。
	if newRect.Width <= 0 {
		t.Fatalf("无导入能力时新会话宽=%v，应该占满整行", newRect.Width)
	}
	if boxes.ImportDrawn {
		t.Fatal("没有导入能力却画了触发器")
	}
}

// TestImportActionSeatMatchesOfficialGeometry 钉住宽档几何。
func TestImportActionSeatMatchesOfficialGeometry(t *testing.T) {
	p, _, env, _, canvas, _ := searchFixture(t)
	calls := 0
	env.Act.ImportDeepSeekSessions = func() { calls++ }
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: 256, Height: 420}, func(f ui.C) {
		p.drawHeader(f, env)
	})
	boxes := p.frameBoxes
	if !boxes.ImportDrawn {
		t.Fatal("壳接了导入能力却没画触发器")
	}
	actionRect := boxes.Import.Rect()
	newRect := boxes.NewSession.Rect()

	if actionRect.Width != importActionSize || actionRect.Height != importActionSize {
		t.Fatalf("触发器=%vx%v，官方宽档 38x38", actionRect.Width, actionRect.Height)
	}
	// 同一行：顶边与高都跟新会话按钮一致（官方 align-items:center + 同高）。
	if actionRect.Y != newRect.Y || actionRect.Height != newRect.Height {
		t.Fatalf("触发器 y/高=%v/%v，新会话=%v/%v（必须同一行）",
			actionRect.Y, actionRect.Height, newRect.Y, newRect.Height)
	}
	// gap 6（官方 `.newSessionRow { gap: 6px }`）。
	if gap := actionRect.X - (newRect.X + newRect.Width); gap != newSessionRowGap {
		t.Fatalf("间距=%v，官方 gap=6", gap)
	}
	// 新会话让位给触发器：整行宽度不变，按钮正好少 38+6（官方 `.newSession{flex:1}`）。
	// 对照：同一帧里把能力摘掉重画一次，量它本来占多宽。
	withAction := newRect.Width
	env.Act.ImportDeepSeekSessions = nil
	canvas.Reset()
	view.Draw(canvas, gfx.Rect{Width: 256, Height: 420}, func(f ui.C) {
		p.drawHeader(f, env)
	})
	withoutAction := p.frameBoxes.NewSession.Rect().Width
	if got, want := withoutAction-withAction, float32(importActionSize+newSessionRowGap); got != want {
		t.Fatalf("加触发器后新会话少了 %v，应少 38+6=%v", got, want)
	}
	// 点了立刻交给壳（壳自己起 goroutine，不在绘制线程里干活）。
	if calls != 0 {
		t.Fatalf("没有点击却调了 %d 次", calls)
	}
}

// TestImportActionSeatClickCallsShellOnce 钉住点击 → 恰好一次。
func TestImportActionSeatClickCallsShellOnce(t *testing.T) {
	p, _, env, tree, canvas, _ := searchFixture(t)
	calls := 0
	env.Act.ImportDeepSeekSessions = func() { calls++ }
	env.Act.NewSession = func() { t.Fatal("点导入按钮不许触发新建会话") }
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	draw := func() {
		view.Draw(canvas, gfx.Rect{Width: 256, Height: 420}, func(f ui.C) {
			p.drawHeader(f, env)
		})
	}
	draw()
	canvas.Reset()
	canvas.BeginEventPass(tree.ID(env.Feat, "import-sessions"), gfx.ClickInfo{})
	draw()
	canvas.EndEventPass()
	if calls != 1 {
		t.Fatalf("一次点击调用=%d，want 1", calls)
	}
}

// TestImportActionUsesOfficialIconAndLabel 钉住"用官方素材与官方无障碍名"。
func TestImportActionUsesOfficialIconAndLabel(t *testing.T) {
	// 图标必须是官方 sprite 里那三条 path（中等档 1.3）。
	icon, ok := dsassets.SidebarImportUpload(gfx.Color{Alpha: 1})
	if !ok {
		t.Fatal("导入触发器没有图标：SidebarImportUpload 编译失败")
	}
	if icon.ViewBox != 16 {
		t.Fatalf("图标 viewBox=%v，官方 sprite 是 16", icon.ViewBox)
	}
	if len(icon.Commands) == 0 {
		t.Fatal("图标没有任何路径命令")
	}
	// 无障碍名逐字取官方 zh 字典的 `action.label`。
	if got := importActionLabel; got != "导入 DeepSeek 会话" {
		t.Fatalf("无障碍名=%q，官方 action.label 是「导入 DeepSeek 会话」", got)
	}
}
