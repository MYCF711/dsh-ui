package dsstatusbar

// connection_test.go —— 连接指示的**官方座位**、三态、几何与图标来源。
//
// 为什么这一组必须存在：连接指示原来是 `shell.bottom` 上一整行「就绪 / 已断开」
// 加一颗 6px 的点，两个毛病都在真机上被点过名——
//
//	① **错层**：官方默认 bottom 为空，连接指示的家是 `sidebar.settings`
//	   （结构树 §4 S8：「连接指示由 settings 的连接状态驱动，不能移到底部自创 statusbar」）；
//	② **编事实**：`!Disconnected` 被当成「连着」写「就绪」，而 `Disconnected == false`
//	   只说明"没有断线证据"，不说明"连着"。缺状态必须写「未知」。
//
// 所以这里的断言分四组：座位（官方/兼容桥，不双画、不占 bottom）、
// 三态文案、官方几何与色调、图标来自官方 sprite。

import (
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// connFrame 画一帧，只 dispatch 给定的槽（模拟骨架在这一帧的派发行为）。
//
// bridged=true 表示"官方骨架这一帧走了旧部件兼容桥"（`ShellTier.LegacyBridge`），
// 那时 `sidebar.settings` 不会被 dispatch，只有 `sidebar.footer` 会。
func connFrame(t *testing.T, bridged bool, link pluginapi.LinkState, slot pluginapi.Slot) (*ui.Canvas, *pluginapi.Theme) {
	t.Helper()
	tier := uikernel.NewShellTier()
	tier.Set(true)
	tier.SetLegacyBridge(bridged)
	kernel := uikernel.New("conn-seat")
	kernel.Root().Provide(uikernel.KeyShellTier, tier)
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &Plugin{}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &ui.Tree{}
	canvas := &ui.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: theme, Data: pluginapi.Snapshot{Cur: "s", Link: link},
		Tree: tree, Feat: tree.Feature("conn"), Canvas: canvas,
		Bounds:   ui.Rect{Width: 280, Height: 200},
		Geometry: pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
		Has:      kernel.Has,
	}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(theme), env.Bounds, func(f *flex.Frame) {
		f.Column().FillWidth().Width(280).Children(func() {
			for _, painter := range kernel.Painters(slot) {
				if painter.Draw != nil {
					painter.Draw(f, env)
				}
			}
		})
	})
	return canvas, theme
}

// connTexts 收画布上的文字。
func connTexts(canvas *ui.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

// connPill 在画布里找连接胶囊的**两件套**，返回是否两件都找到。
//
// viva 的 `Border(w, color)` 画成「外圈 1px 描边 + 内缩一圈的填充」，所以官方
// `._indicator{border:1px solid …}` 在这里是：
//
//	外圈：高度 28、圆角 SM(8)、颜色 = 同色 20% 透明（color-mix 的等价）
//	内层：高度 26、圆角 SM-1(7)、颜色 = tertiary 底
//
// 两件都要在：只断言其一的话，"只有描边没有底"或"只有底没有描边"都会漏过。
func connPill(canvas *ui.Canvas, fill, border ui.Color) bool {
	outer, inner := false, false
	for _, rect := range canvas.Rectangles() {
		if rect.Bounds.Height == connIndicatorHeight && rect.Radius == 8 && rect.Color == border {
			outer = true
		}
		if rect.Bounds.Height == connIndicatorHeight-2 && rect.Color == fill {
			inner = true
		}
	}
	return outer && inner
}

// ① 座位：官方 `sidebar.settings` 上有一份，`shell.bottom` 上一份都没有。
func TestConnectionIndicatorSeatsAreOfficial(t *testing.T) {
	p := &Plugin{}
	slots := map[string]pluginapi.Slot{}
	orders := map[string]int{}
	for _, panel := range p.Panels() {
		slots[panel.ID] = panel.Slot
		orders[panel.ID] = panel.Order
	}
	if slots["connection"] != uikernel.OffSidebarSettings {
		t.Fatalf("connection 座位 = %q，want sidebar.settings（官方 SettingsRoot.triggerRow）", slots["connection"])
	}
	if slots["connection-bridge"] != pluginapi.SlotSidebarFooter {
		t.Fatalf("connection-bridge 座位 = %q，want sidebar.footer（整列兼容桥时的过渡位）", slots["connection-bridge"])
	}
	// 官方这一级里，账号启动器（dsnotifications）占 60；指示器紧跟其后。
	if orders["connection"] <= 60 {
		t.Fatalf("connection order = %d，应排在 settings.launcher(60) 之后", orders["connection"])
	}
	// footer 区最后一行（官方 footArea：footerActions 在上、settings 在下）。
	if orders["connection-bridge"] <= orders["connection"] {
		t.Fatalf("footer 桥 order = %d，应排在其它 footer 条目之后", orders["connection-bridge"])
	}
	for _, panel := range p.Panels() {
		if panel.Slot == pluginapi.SlotShellBottom {
			t.Fatalf("连接指示不许再占 shell.bottom：%+v", panel)
		}
	}
}

// ② 官方档位：`sidebar.settings` 真画，且 `sidebar.footer` 桥一声不响（不双画）。
func TestConnectionIndicatorDrawsOnOfficialSettingsSeat(t *testing.T) {
	canvas, theme := connFrame(t, false, pluginapi.LinkConnected, uikernel.OffSidebarSettings)
	if !connPill(canvas, theme.SuccessTertiary, withAlpha(theme.SuccessPrimary, connIndicatorBorderTint)) {
		t.Fatalf("官方座位没画出 success 胶囊（官方 ._success 底色）：rects=%+v", canvas.Rectangles())
	}
	if got := connTexts(canvas); len(got) == 0 || got[0] != zhConnectionConnected {
		t.Fatalf("官方座位文案 = %v，want %q", got, zhConnectionConnected)
	}
	// 官方座位这一帧活着时，桥不许再画一次（同帧只有一个活）。
	bridge, _ := connFrame(t, false, pluginapi.LinkConnected, pluginapi.SlotSidebarFooter)
	if rects := bridge.Rectangles(); len(rects) != 0 {
		t.Fatalf("官方座位活着时桥还画了 %d 个矩形（双画）：%+v", len(rects), rects)
	}
	if texts := connTexts(bridge); len(texts) != 0 {
		t.Fatalf("官方座位活着时桥还画了文字（双画）：%v", texts)
	}
}

// ③ 兼容桥：整列还是旧三栏时，官方槽不 dispatch，`sidebar.footer` 顶上（否则界面空白）。
func TestConnectionIndicatorFallsBackToFooterBridge(t *testing.T) {
	canvas, theme := connFrame(t, true, pluginapi.LinkDown, pluginapi.SlotSidebarFooter)
	if !connPill(canvas, theme.WarnTertiary, withAlpha(theme.WarnLabel, connIndicatorBorderTint)) {
		t.Fatalf("桥座位没画出 warning 胶囊（官方 ._warning 底色）：rects=%+v", canvas.Rectangles())
	}
	if got := connTexts(canvas); len(got) == 0 || got[0] != zhConnectionError {
		t.Fatalf("桥座位文案 = %v，want %q（官方 connection.error 原句）", got, zhConnectionError)
	}
	// 反向：桥开着时官方座位不许画（它本来也不会被 dispatch，这里钉住代码不再多一份）。
	official, _ := connFrame(t, true, pluginapi.LinkDown, uikernel.OffSidebarSettings)
	if rects := official.Rectangles(); len(rects) != 0 {
		t.Fatalf("整列走兼容桥时官方座位还画了 %d 个矩形：%+v", len(rects), rects)
	}
}

// ④ 三态文案与色调：connected / down / unknown 各一条，且**未知不写「已连接」**。
func TestConnectionIndicatorThreeStates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		link   pluginapi.LinkState
		want   string
		fill   func(*pluginapi.Theme) ui.Color // nil = 中性档（不许借用成功/异常底色）
		border func(*pluginapi.Theme) ui.Color
		icons  int // 官方 glyph 的段数（0 = 这一档没有官方图标）
	}{
		{
			name: "已连接", link: pluginapi.LinkConnected, want: zhConnectionConnected,
			fill:   func(t *pluginapi.Theme) ui.Color { return t.SuccessTertiary },
			border: func(t *pluginapi.Theme) ui.Color { return withAlpha(t.SuccessPrimary, connIndicatorBorderTint) },
			icons:  1,
		},
		{
			name: "连接异常", link: pluginapi.LinkDown, want: zhConnectionError,
			fill:   func(t *pluginapi.Theme) ui.Color { return t.WarnTertiary },
			border: func(t *pluginapi.Theme) ui.Color { return withAlpha(t.WarnLabel, connIndicatorBorderTint) },
			icons:  2,
		},
		{
			// 壳还没拿到任何连接事实（首帧 / 还没订阅 / 老壳）：写「未知」，
			// 中性描边、不借用 success/warning 的色，也不编「已连接」。
			name: "未知", link: pluginapi.LinkUnknown, want: zhConnectionUnknown, icons: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			canvas, theme := connFrame(t, false, tc.link, uikernel.OffSidebarSettings)
			if got := connTexts(canvas); len(got) == 0 || got[0] != tc.want {
				t.Fatalf("文案 = %v，want %q", got, tc.want)
			}
			if tc.fill != nil {
				if !connPill(canvas, tc.fill(theme), tc.border(theme)) {
					t.Fatalf("缺官方底色胶囊：want %v；rects=%+v", tc.fill(theme), canvas.Rectangles())
				}
			} else {
				// 中性档：不许出现 success/warning 底色（那是"有结论"的配色）。
				if connPill(canvas, theme.SuccessTertiary, withAlpha(theme.SuccessPrimary, connIndicatorBorderTint)) ||
					connPill(canvas, theme.WarnTertiary, withAlpha(theme.WarnLabel, connIndicatorBorderTint)) {
					t.Fatalf("未知态借用了成功/异常底色：rects=%+v", canvas.Rectangles())
				}
			}
			// 图标：官方 glyph 是描边（三角化），段数与 view 见 icons.go。
			if tc.icons == 0 {
				if n := len(canvas.Triangles()); n != 0 {
					t.Fatalf("未知态不该画成功/异常图标，却画了 %d 个三角形", n)
				}
			} else if len(canvas.Triangles()) == 0 {
				t.Fatalf("%s 态缺官方 glyph（没有任何描边三角形）", tc.name)
			}
			if tc.link == pluginapi.LinkUnknown {
				for _, banned := range []string{"就绪", "已连接", zhConnectionConnected} {
					if session := connTexts(canvas); len(session) > 0 && session[0] == banned {
						t.Fatalf("未知态写了 %q——没有证据的断言", banned)
					}
				}
			}
		})
	}
}

// ⑤ 几何：官方 CSS 逐条（triggerRow 42/4/8、胶囊 28/8/4/12/18、描边 1 + 20%）。
func TestConnectionIndicatorGeometryMatchesOfficial(t *testing.T) {
	checks := []struct {
		name string
		got  float32
		want float32
		why  string
	}{
		{"triggerRow height", connRowHeight, 42, "SettingsRoot.module.css .trigger{height:42px}"},
		{"triggerRow margin-y", connRowMarginY, 4, ".triggerRow{margin:4px 0}"},
		{"triggerRow gap", connRowGap, 8, ".triggerRow{gap:8px}"},
		{"indicator height", connIndicatorHeight, 28, "._indicator_16cyo_1{height:28px}"},
		{"indicator pad-x", connIndicatorPadX, 8, "._indicator{padding:0 8px}"},
		{"indicator gap", connIndicatorGap, 4, "._indicator{column-gap:4px}"},
		{"indicator font", connIndicatorFont, 12, "._indicator{font-size:12px}"},
		{"indicator line", connIndicatorLine, 18, "._indicator{line-height:18px}"},
		{"indicator border", connIndicatorBorder, 1, "._indicator{border:1px solid transparent}"},
		{"indicator border tint", connIndicatorBorderTint, 0.2, "color-mix(… 20%, transparent)"},
		{"icon size", connectionIconSize, 14, "._icon_16cyo_71{width:14px;height:14px}"},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %v，官方 %v（%s）", check.name, check.got, check.want, check.why)
		}
	}
}

// ⑥ 图标来源：官方 sprite `check-outline` / `refresh-outline`（viewBox 16、regular=1）。
func TestConnectionIconsAreOfficialSprites(t *testing.T) {
	for _, tc := range []struct {
		name    string
		icon    ui.Icon
		paths   int // 官方 path 段数
		firstPt ui.Point
	}{
		{"check-outline", checkOutlineIcon(), 1, ui.Point{X: 2.25, Y: 8.5}},
		{"refresh-outline", refreshOutlineIcon(), 2, ui.Point{X: 14.5001, Y: 8}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.icon.ViewBox != 16 {
				t.Fatalf("viewBox = %v，want 16（官方 sprite）", tc.icon.ViewBox)
			}
			if tc.icon.Stroke != 1 {
				t.Fatalf("stroke = %v，want 1（regular 权重）", tc.icon.Stroke)
			}
			moves := 0
			for _, command := range tc.icon.Commands {
				if command.Kind == ui.PathCommandMove {
					moves++
				}
			}
			if moves != tc.paths {
				t.Fatalf("path 段数 = %d，want %d（官方 sprite）", moves, tc.paths)
			}
			first := tc.icon.Commands[0]
			if first.Kind != ui.PathCommandMove || first.Point != tc.firstPt {
				t.Fatalf("首命令 = %+v，want MoveTo(%v, %v)（官方 d 的起点）",
					first, tc.firstPt.X, tc.firstPt.Y)
			}
		})
	}
}
