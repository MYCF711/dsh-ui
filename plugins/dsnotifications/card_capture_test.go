//go:build darwin || windows || linux

// card_capture_test.go —— 通知卡的**真实渲染截图**（给截图比对用）。
//
// 与 `card_visual_test.go` 的分工：那边把官方 CSS 的数字逐条钉在矩形上
// （280 宽 / 20 padding / 18×18 关闭按钮 / 叠层 252/260）；这边把同一帧经真
// Viva headless 渲染成 PNG，专门看**只有像素才暴露**的东西：
//
//	· 卡片阴影（`var(--dsw-elevation-panel)`）有没有被父级 Clip 裁掉；
//	· 第二层的 4px 可见边是不是从卡片顶端"露出来"而不是叠在卡里；
//	· 圆角 16 的裁切有没有切到标题/按钮。
//
// 截图落 `$VR_NOTICE_CAPTURE_DIR`（默认 t.TempDir()）。
package dsnotifications

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// TestNoticeCardRenderCapture 画一摞两层通知卡并落 PNG。
func TestNoticeCardRenderCapture(t *testing.T) {
	dir := os.Getenv("VR_NOTICE_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	second := sampleNotice("n-geo-2")
	second.Title = "用量提醒"
	second.Body = "本周已用掉 80% 的额度"
	p := activePlugin(t, geometryNotice(), second)
	theme := dscore.Dark()

	const width, height = 420, 360
	tree := &gfx.Tree{}
	feat := tree.Feature("notice-capture")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{
		Title: "notice-capture", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless,
	}
	var lastCanvas *gfx.Canvas
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		lastCanvas = canvas
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width,
			Act: pluginapi.Actions{Notify: func(string) {}},
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			// 官方座位在账号菜单右侧（`settings.launcher.notifications`）；
			// 这里按同一位置放：右贴边 24、顶 56，容器比卡宽 40 留出阴影。
			ui.Row(f).Absolute().Right(24).Top(56).Width(StackMaxWidth + 40).Children(func() {
				p.draw(f, env)
			})
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for range 2 {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}

	if lastCanvas == nil {
		t.Fatal("Paint 没被调用")
	}

	// ① 官方 elevation-panel 的两层柔光**真的画出来了、且没有带裁剪**。
	//
	//    这条专门守一类只有渲染才暴露的回归：元素自己 `.Clip()` 时 viva 先裁再跑
	//    Draw 钩子（`viva@0.6.4/ui/paint.go:50-52` 推 Clip、`:132` 才调钩子），
	//    阴影会整层消失；`gfx.Shadow.HasClip` 是这件事的机读证据。
	var card gfx.Rect
	var glows int
	for _, shadow := range lastCanvas.Shadows() {
		if shadow.Bounds.Width != StackMaxWidth || shadow.Radius != CardRadius {
			continue
		}
		if glows == 0 {
			card = shadow.Bounds
		}
		if shadow.HasClip {
			t.Fatalf("卡片柔光带上了裁剪 %+v：`.card{box-shadow:elevation-panel}` 会被整层裁掉", shadow.ClipOf)
		}
		want := p.theme.Shadows.Panel.Glows[glows]
		if !near(shadow.Style.OffsetY, want.OffsetY) || !near(shadow.Style.Blur, want.Blur) ||
			!near(shadow.Color.Alpha, want.Alpha) {
			t.Fatalf("第 %d 层柔光 = {OffsetY:%v Blur:%v Alpha:%v}，官方 elevation-panel 是 "+
				"{OffsetY:%v Blur:%v Alpha:%v}（gradient-shadow-text.css:35）",
				glows, shadow.Style.OffsetY, shadow.Style.Blur, shadow.Color.Alpha,
				want.OffsetY, want.Blur, want.Alpha)
		}
		glows++
	}
	if glows != len(p.theme.Shadows.Panel.Glows) {
		t.Fatalf("卡片柔光层数 = %d，官方 elevation-panel 两层", glows)
	}

	// ② `.close` 是**纯图标**：官方 `.close` 没有任何文本（`AccountNotifications.tsx:173`
	//    只有一个 `<IconCloseOutlineRegular size={10}/>`）。viva 的 `Button` 不挂 Icon
	//    时会把 label 真画出来，18px 宽里溢出「关闭」二字——这条断言钉住它不再出现。
	joined := strings.Join(session.Texts(), "\u0000")
	if strings.Contains(joined, labelClose) {
		t.Fatalf("关闭按钮画出了文字 %q（官方 .close 只有 10px 图标）：%q", labelClose, session.Texts())
	}
	// 命中区仍是官方 18×18、top/right 6（即使没有 Button 的视觉）。
	closeHit, ok := hitByLabel(lastCanvas, labelClose)
	if !ok {
		t.Fatalf("找不到关闭按钮命中区（标签 %q）", labelClose)
	}
	wantX := card.X + card.Width - CloseInset - CloseSize
	if closeHit.Bounds.X != wantX || closeHit.Bounds.Y != card.Y+CloseInset ||
		closeHit.Bounds.Width != CloseSize || closeHit.Bounds.Height != CloseSize {
		t.Fatalf("关闭命中区 = %+v，官方 .close 应在外侧 6px、18×18（卡 %+v）", closeHit.Bounds, card)
	}

	// ③ 正文与动作都在；被前景卡盖住的第二层正文不许漏画。
	for _, want := range []string{"赠金到账", "1000 tokens 已加入你的账户", "查看", "忽略"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("通知卡缺 %q；文本=%q", want, session.Texts())
		}
	}
	if strings.Contains(joined, "本周已用掉 80%") {
		t.Fatalf("非前景卡的正文被画出来了（官方 pointer-events:none + 只露 4px 边）：%q", session.Texts())
	}

	t.Logf("VR_NOTICE_GEOM card=%+v closeHit=%+v glows=%d", card, closeHit.Bounds, glows)
	path := filepath.Join(dir, "notice-card.png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	t.Logf("VR_NOTICE_CAPTURE path=%s", path)
}
