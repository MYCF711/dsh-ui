// card_visual_test.go —— NoticeCard 的**几何证据**（官方 CSS 值逐条对表）。
//
// 判据不是"图里有东西"，而是结构树 §4 的那几个数字真的落到矩形上：
//
//	.stack  max-width 280px          → 卡片宽 = 280
//	.card   padding 20px             → 标题文字左边缘 = 卡左 + 20
//	.close  18×18、top/right 6px     → 关闭命中区右/上各距卡边 6，尺寸 18
//	.action min-height 28px          → 动作命中区高 ≥ 28
//	.title  font-size 14 / weight 500
//	叠层    (252/260)^depth 收窄     → 第二层可见边宽 = 280 × 252/260
//
// 两处降级也在这里钉住：非前景卡**没有**命中区（官方 pointer-events:none），
// 第二层只画可见的 4px 边。
package dsnotifications

import (
	"math"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/vivaui/viva/gfx"
)

// geometryNotice 造一条动作标签不撞"关闭"的通知（避免命中区标签歧义）。
func geometryNotice() apiclient.Notification {
	n := sampleNotice("n-geo")
	n.Actions = []apiclient.NotificationAction{
		{Label: "查看", Style: "primary", Target: apiclient.NotificationTarget{Type: "app_route", Name: "account"}},
		{Label: "忽略", Style: "secondary", Target: apiclient.NotificationTarget{Type: "dismiss"}},
	}
	return n
}

// activePlugin 造一个已经拿到一张卡的插件。
func activePlugin(t *testing.T, notices ...apiclient.Notification) *Plugin {
	t.Helper()
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications:        notices,
	}}
	p := newTestPlugin(remote)
	p.ctrl.Update(AccountState{Active: true, SignedIn: true})
	waitUntil(t, "取到通知", func() bool { return len(p.ctrl.Notices()) == len(notices) })
	return p
}

// cardRect 从画布上找那张卡：半径 16 的矩形里最宽的那个（正文区/背景层都更小）。
func cardRect(canvas *gfx.Canvas) (gfx.Rect, bool) {
	var best gfx.Rect
	found := false
	for _, r := range canvas.Rectangles() {
		if r.Radius != CardRadius {
			continue
		}
		if !found || r.Bounds.Width > best.Width {
			best = r.Bounds
			found = true
		}
	}
	return best, found
}

func hitByLabel(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if hit.Label == label {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

func textByValue(canvas *gfx.Canvas, value string) (gfx.Text, bool) {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text, true
		}
	}
	return gfx.Text{}, false
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 0.51 }

// TestCardMaxWidthIs280：官方 `.stack{max-width:280px}`。
func TestCardMaxWidthIs280(t *testing.T) {
	canvas := render(t, activePlugin(t, geometryNotice()))
	card, ok := cardRect(canvas)
	if !ok {
		t.Fatal("找不到卡片（半径 16 的矩形）")
	}
	if !near(card.Width, StackMaxWidth) {
		t.Fatalf("卡片宽应为 280（.stack max-width），得到 %.1f", card.Width)
	}
}

// TestCardPaddingIs20：官方 `.card{padding:20px}` + `.title{gap:2px}` + `.icon{14px}`。
//
// 标题文字前还有 icon（14px）与 gap（2px），所以文字左边缘 = 卡左 + 20 + 14 + 2；
// 这个偏移本身就是结构（标题行是 [icon][title]），不是误差。
func TestCardPaddingIs20(t *testing.T) {
	canvas := render(t, activePlugin(t, geometryNotice()))
	card, ok := cardRect(canvas)
	if !ok {
		t.Fatal("找不到卡片")
	}
	title, ok := textByValue(canvas, "赠金到账")
	if !ok {
		t.Fatal("找不到标题文字")
	}
	wantTitleX := CardPadding + IconSize + CopyGap
	if !near(title.Bounds.X-card.X, wantTitleX) {
		t.Fatalf("标题左边缘应距卡边 padding20+icon14+gap2=%.0fpx，得到 %.1f", wantTitleX, title.Bounds.X-card.X)
	}
	if !near(title.Style.Size, CardFontSize) {
		t.Fatalf("标题字号应为 14，得到 %.1f", title.Style.Size)
	}
	if !near(title.Style.Weight, 500) {
		t.Fatalf("标题字重应为 500（.title font-weight:500），得到 %.0f", title.Style.Weight)
	}
}

// TestCloseButtonGeometry：官方 `.close{18×18;top:6px;right:6px}`。
func TestCloseButtonGeometry(t *testing.T) {
	canvas := render(t, activePlugin(t, geometryNotice()))
	card, ok := cardRect(canvas)
	if !ok {
		t.Fatal("找不到卡片")
	}
	closeHit, ok := hitByLabel(canvas, labelClose)
	if !ok {
		t.Fatalf("找不到关闭按钮命中区（label=%q）", labelClose)
	}
	if !near(closeHit.Bounds.Width, 18) || !near(closeHit.Bounds.Height, 18) {
		t.Fatalf("关闭按钮应为 18×18，得到 %.1f×%.1f", closeHit.Bounds.Width, closeHit.Bounds.Height)
	}
	if right := card.X + card.Width - (closeHit.Bounds.X + closeHit.Bounds.Width); !near(right, CloseInset) {
		t.Fatalf("关闭按钮应距卡右边 6px，得到 %.1f", right)
	}
	if top := closeHit.Bounds.Y - card.Y; !near(top, CloseInset) {
		t.Fatalf("关闭按钮应距卡上边 6px，得到 %.1f", top)
	}
}

// TestActionMinHeightIs28：官方 `.action{min-height:28px}`。
func TestActionMinHeightIs28(t *testing.T) {
	canvas := render(t, activePlugin(t, geometryNotice()))
	for _, label := range []string{"查看", "忽略"} {
		hit, ok := hitByLabel(canvas, label)
		if !ok {
			t.Fatalf("找不到动作按钮 %q 的命中区", label)
		}
		if hit.Bounds.Height < ActionMinHeight-0.5 {
			t.Fatalf("动作按钮高度应 ≥28px（.action min-height），%q 得到 %.1f", label, hit.Bounds.Height)
		}
	}
}

// TestStackDepthScale：官方叠层比例 252/260 + 每层上移 4px。
//
// 本实现把非前景卡画成"可见边"：4px 高、宽度 280×(252/260)^depth。
func TestStackDepthScale(t *testing.T) {
	canvas := render(t, activePlugin(t, geometryNotice(), sampleNotice("n-back")))
	card, ok := cardRect(canvas)
	if !ok {
		t.Fatal("找不到前景卡")
	}
	wantPeek := float32(float64(StackMaxWidth) * depthScale(1))
	peek, ok := findByRadius(canvas.Rectangles(), DepthOffset/2)
	if !ok {
		t.Fatal("找不到第二层的可见边（半径 2 的矩形）")
	}
	if !near(peek.Bounds.Width, wantPeek) {
		t.Fatalf("第二层宽应为 280×252/260=%.1f，得到 %.1f", wantPeek, peek.Bounds.Width)
	}
	if !near(peek.Bounds.Height, DepthOffset) {
		t.Fatalf("第二层可见边高应为 4px（translateY(-4px)），得到 %.1f", peek.Bounds.Height)
	}
	// 前景卡在可见边之下：卡顶 = 边底。
	if !near(card.Y, peek.Bounds.Y+DepthOffset) {
		t.Fatalf("前景卡应紧接在可见边之下：卡顶 %.1f，边底 %.1f", card.Y, peek.Bounds.Y+DepthOffset)
	}
}

func findByRadius(rects []gfx.Rectangle, radius float32) (gfx.Rectangle, bool) {
	for _, r := range rects {
		if near(r.Radius, radius) {
			return r, true
		}
	}
	return gfx.Rectangle{}, false
}

// TestOnlyForegroundIsInteractive：官方非前景卡 pointer-events:none。
//
// 判据：两张卡时，命中区仍然只有"前景卡的动作 + 关闭"这 3 个，
// **没有**第二张卡的任何命中区。
func TestOnlyForegroundIsInteractive(t *testing.T) {
	canvas := render(t, activePlugin(t, geometryNotice(), sampleNotice("n-back")))
	hits := canvas.Hits()
	if len(hits) != 3 {
		labels := make([]string, 0, len(hits))
		for _, h := range hits {
			labels = append(labels, h.Label)
		}
		t.Fatalf("应该只有前景卡的 3 个命中区（2 动作 + 关闭），得到 %d 个：%v", len(hits), labels)
	}
}

// TestBodyTextWrapped：正文宽度不应超过卡片内容宽度（280-40=240）。
func TestBodyTextWrapped(t *testing.T) {
	canvas := render(t, activePlugin(t, geometryNotice()))
	card, ok := cardRect(canvas)
	if !ok {
		t.Fatal("找不到卡片")
	}
	body, ok := textByValue(canvas, "1000 tokens 已加入你的账户")
	if !ok {
		t.Fatal("找不到正文")
	}
	if body.Bounds.Width > card.Width-CardPadding*2+0.5 {
		t.Fatalf("正文宽 %.1f 超过内容宽度 %.1f", body.Bounds.Width, card.Width-CardPadding*2)
	}
}
