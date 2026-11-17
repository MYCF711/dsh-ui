package dsnotifications

import (
	"testing"

	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
)

// exactNear 是比 near（容差 0.51）更严的比较：这里要钉的是 0.5px 那一档。
func exactNear(a, b float32) bool {
	if a > b {
		return a-b < 1e-6
	}
	return b-a < 1e-6
}

// TestCardElevationIsTheSharedPanelToken 钉通知卡片的阴影来自**共用令牌**，
// 而不是本包自己攒的数。
//
// 官方 `AccountNotifications.module.css:4` 的 `.card{box-shadow:var(--dsw-elevation-panel)}`，
// 描边色没重绑 → 走默认的 `--dsw-elevation-stroke-color: var(--dsw-alias-border-l4)`
// （gradient-shadow-text.css:17，暗色档 design-platform.css:351）。
//
// 这一只在系统化（task-77）里改的是**来源**不是数值：本包原来自己写了一份
// PanelStroke/PanelShadowY/…/PanelGlowAlpha 加一个 drawPanelElevation，数值本来就
// 是对的，但 20+ 个浮层各抄一份的结果是——只有抄得最认真的那几个是对的。
// 现在数值来自 dscore 的官方令牌，本包只负责传描边色。
func TestCardElevationIsTheSharedPanelToken(t *testing.T) {
	theme := dscore.Dark()
	// 两张卡：前景卡 + 一张叠在后面的（`translateY(-4px)` 那一摞）。
	// 两条绘制路径（drawCard / drawStack 的 depth 层）都要走同一套令牌，
	// 只测一张会漏掉另一条。
	back := geometryNotice()
	back.ID = "n-geo-back"
	p := activePlugin(t, geometryNotice(), back)
	canvas := renderWithTheme(t, p, theme)

	byBounds := map[gfx.Rect][]gfx.Shadow{}
	for _, shadow := range canvas.Shadows() {
		if near(shadow.Radius, CardRadius) {
			byBounds[shadow.Bounds] = append(byBounds[shadow.Bounds], shadow)
		}
	}
	if len(byBounds) < 2 {
		t.Fatalf("两张叠放的通知卡应有 2 组柔光（前景 + 后层），实际 %d 组：%+v", len(byBounds), canvas.Shadows())
	}

	want := theme.Shadows.Panel.Glows[:]
	for card, glows := range byBounds {
		if len(glows) != len(want) {
			t.Fatalf("卡片 %+v 应有 %d 层官方柔光，实际 %d 层：%+v", card, len(want), len(glows), glows)
		}
		for i, layer := range glows {
			if !exactNear(layer.Style.OffsetY, want[i].OffsetY) || !exactNear(layer.Style.Blur, want[i].Blur) ||
				!exactNear(layer.Color.Alpha, want[i].Alpha) {
				t.Errorf("卡片 %+v 第 %d 层柔光 ≠ theme.Shadows.Panel：得到 %+v alpha=%v，令牌 %+v alpha=%v",
					card, i, layer.Style, layer.Color.Alpha, want[i], want[i].Alpha)
			}
		}
	}

	// 描边只核**前景卡**：官方那一摞里只有 `notices[0]` 是可交互的前景卡，
	// 后层卡是 `scaleX` 缩放后的示意条。drawStack 先画后层、最后画前景，
	// 所以画布上最后一组（也就是最大的一组）就是前景。
	var card gfx.Rect
	for bounds := range byBounds {
		if bounds.Width > card.Width {
			card = bounds
		}
	}
	if card.Width == 0 {
		t.Fatal("找不到前景通知卡")
	}

	// 描边：0.5px，颜色是 theme.BorderL4，画在外扩 0.5 的矩形上。
	minX, minY := float32(1e9), float32(1e9)
	maxX, maxY := float32(-1e9), float32(-1e9)
	count := 0
	for _, tri := range canvas.Triangles() {
		if tri.Color != theme.BorderL4 {
			continue
		}
		inRing := true
		for _, point := range tri.Points {
			if point.X < card.X-theme.Shadows.Panel.Stroke-1e-3 ||
				point.Y < card.Y-theme.Shadows.Panel.Stroke-1e-3 ||
				point.X > card.X+card.Width+theme.Shadows.Panel.Stroke+1e-3 ||
				point.Y > card.Y+card.Height+theme.Shadows.Panel.Stroke+1e-3 {
				inRing = false
			}
		}
		if !inRing {
			continue // 后层卡的描边：颜色相同，但不在前景卡这一圈里
		}
		count++
		for _, point := range tri.Points {
			minX, minY = min(minX, point.X), min(minY, point.Y)
			maxX, maxY = max(maxX, point.X), max(maxY, point.Y)
		}
	}
	if count == 0 {
		t.Fatal("卡片没有 0.5px 的 border-l4 发丝描边：官方 elevation 三件套的第一层就是它（gradient-shadow-text.css:33）")
	}
	for _, want := range []struct {
		name string
		got  float32
		want float32
	}{
		{"左边界", minX, card.X - theme.Shadows.Panel.Stroke},
		{"上边界", minY, card.Y - theme.Shadows.Panel.Stroke},
		{"右边界", maxX, card.X + card.Width + theme.Shadows.Panel.Stroke},
		{"下边界", maxY, card.Y + card.Height + theme.Shadows.Panel.Stroke},
	} {
		if !exactNear(want.got, want.want) {
			t.Errorf("描边%s应为 %v（外扩 %vpx），实际 %v",
				want.name, want.want, theme.Shadows.Panel.Stroke, want.got)
		}
	}
}
