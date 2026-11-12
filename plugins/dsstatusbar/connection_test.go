package dsstatusbar

// connection_test.go —— 连接指示的**官方座位**、四态、几何、图标来源、150ms 离场与重连按钮。
//
// 为什么这一组必须存在：连接指示原来是 `shell.bottom` 上一整行「就绪 / 已断开」
// 加一颗 6px 的点，两个毛病都在真机上被点过名——
//
//	① **错层**：官方默认 bottom 为空，连接指示的家是 `sidebar.settings`
//	   （结构树 §4 S8：「连接指示由 settings 的连接状态驱动，不能移到底部自创 statusbar」）；
//	② **编事实**：`!Disconnected` 被当成「连着」写「就绪」，而 `Disconnected == false`
//	   只说明"没有断线证据"，不说明"连着"。缺状态必须写「未知」。
//
// 所以这里的断言分六组：
//
//	① 座位：唯一座位是 `sidebar.settings`（官方 `SettingsRoot.triggerRow`），
//	   不再有 `shell.bottom`，也不再需要"兼容桥座位"（壳在**所有**绘制路径下
//	   都会画设置行那一行）；
//	② 四态文案与色调（含官方 `connecting` 与 derived 的「未知」）；
//	③ 官方几何（行 42 / 胶囊 28 / 图标 14 …）；
//	④ 图标来自官方 sprite / spinner；
//	⑤ **150ms 离场淡出**：目标态到了以后旧态再画 150ms（opacity 1→0），
//	   150ms 后才换成新态（`._leaving{opacity:0}` + `transition:opacity .15s`）；
//	⑥ **warning 态是按钮**：命中区 + 点击真的走 `Actions.Reconnect`，
//	   而 connected / unknown 两态**不可点**（官方也只把 warning 画成 button）。

import (
	"testing"
	"time"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// connHarness 是一帧连接指示的测试台：假时钟 + 真 flex 布局 + 真主题。
type connHarness struct {
	plugin *Plugin
	kernel *uikernel.Kernel
	tree   *gfx.Tree
	feat   gfx.Scope
	theme  *pluginapi.Theme
	// now 是注入给插件的假时钟：150ms 离场窗必须**确定**，不能靠 sleep。
	now time.Time
	// act 是这一帧给插件的动作表（重连按钮要读它）。
	act pluginapi.Actions
	// hit 是这一帧要模拟点击的交互 id（空 = 不点）。
	hit string
}

func newConnHarness(t *testing.T) *connHarness {
	t.Helper()
	tier := uikernel.NewShellTier()
	tier.Set(true)
	kernel := uikernel.New("conn-seat")
	kernel.Root().Provide(uikernel.KeyShellTier, tier)
	plugin := &Plugin{}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, plugin); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	tree := &gfx.Tree{}
	h := &connHarness{
		plugin: plugin,
		kernel: kernel,
		tree:   tree,
		feat:   tree.Feature("conn"),
		theme:  kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme),
		// 固定起点：假时钟从 0 开始走（每次 Draw 由调用方决定推进多少）。
		now: time.Unix(0, 0),
	}
	plugin.now = func() time.Time { return h.now }
	return h
}

// frame 画一帧「设置行」：与壳（uikernel / dsshell）同源的 Row 容器 +
// 一个占位的"设置按钮"子项（flex:1、42 高），再把 `sidebar.settings` 槽
// 上的占用方画进同一行。
//
// 为什么连容器一起画：连接指示**不再自画一行**（官方 `._indicator{flex:none}`），
// 只画行里的一个子项。只调 `drawConnectionIndicator` 而不给 Row 的话，
// 测的是"它自己会画一个胶囊"，测不到"它在设置行里"。
func (h *connHarness) frame(t *testing.T, link pluginapi.LinkState) *gfx.Canvas {
	t.Helper()
	canvas := &gfx.Canvas{}
	canvas.SetTree(h.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: h.theme, Data: pluginapi.Snapshot{Cur: "s", Link: link},
		Act:  h.act,
		Tree: h.tree, Feat: h.feat, Canvas: canvas,
		Bounds:   ui.Rect{Width: 280, Height: 200},
		Geometry: pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
		Has:      h.kernel.Has,
	}
	if h.hit != "" {
		canvas.BeginEventPass(h.tree.ID(h.feat, h.hit), gfx.ClickInfo{})
		defer canvas.EndEventPass()
	}
	pageTheme := dscore.VivaTheme(h.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		ui.Column(f).FillWidth().Width(280).Children(func() {
			row := ui.Row(f).FillWidth().
				MarginSides(connRowMarginY, -2, connRowMarginY, -2).
				Background(h.theme.SidebarFill).
				Items(ui.Center).Gap(connRowGap)
			row.Children(func() {
				// 官方这一行的第一个子项：设置启动器（`flex:1`、42 高）。
				// 测试里用一棵定尺寸的盒子代替，只为量"同一行"。
				ui.Row(f).Height(connRowHeight).Grow(1).Children(func() {})
				for _, painter := range h.kernel.Painters(uikernel.OffSidebarSettings) {
					if painter.Draw != nil {
						painter.Draw(f, env)
					}
				}
			})
		})
	})
	return canvas
}

// connTexts 收画布上的文字。
func connTexts(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

// countOpaqueTriangles 数**不透明**三角形：图标描边是 α=1，胶囊底色是 α=0.12。
//
// Viva v0.5.1 起圆角填充也走三角形 mesh，所以"有没有三角形"不能当
// "画没画图标"的判据——能区分的性质是不透明度。
func countOpaqueTriangles(canvas *gfx.Canvas) int {
	const opaque = 0.5
	n := 0
	for _, triangle := range canvas.Triangles() {
		if triangle.Color.Alpha >= opaque {
			n++
		}
	}
	return n
}

// connPill 在画布里找连接胶囊的**两件套**，返回是否两件都找到。
//
// viva 的 `Border(w, color)` 画成「外圈 1px 描边 + 内缩一圈的填充」，所以官方
// `._indicator{border:1px solid …}` 在这里是：
//
//	外圈：高度 28、圆角 SM(8)、颜色 = 同色 20% 透明（color-mix 的等价）
//	内层：高度 26、圆角 SM-1(7)、颜色 = tertiary 底
func connPill(canvas *gfx.Canvas, fill, border ui.Color) bool {
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

// connRowRect 找设置行矩形（测试台给行加了一枚侧栏底色，见 frame）。
func connRowRect(canvas *gfx.Canvas, fill ui.Color) (gfx.Rectangle, bool) {
	for _, rect := range canvas.Rectangles() {
		if rect.Bounds.Height == connRowHeight && rect.Color == fill {
			return rect, true
		}
	}
	return gfx.Rectangle{}, false
}

// connPillRect 找胶囊外层（28 高、圆角 8 的描边矩形）。
func connPillRect(canvas *gfx.Canvas, border ui.Color) (gfx.Rectangle, bool) {
	for _, rect := range canvas.Rectangles() {
		if rect.Bounds.Height == connIndicatorHeight && rect.Radius == 8 && rect.Color == border {
			return rect, true
		}
	}
	return gfx.Rectangle{}, false
}

// ① 座位：唯一座位是官方 `sidebar.settings`，不许再占 `shell.bottom`，
// 也不许再注册第二个"兼容桥座位"（壳在所有绘制路径都画设置行）。
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
	if _, ok := slots["connection-bridge"]; ok {
		t.Fatal("不该再有 connection-bridge：设置行在所有绘制路径下都会被 dispatch（HH 单第 1 件）")
	}
	// 官方这一级里，账号启动器（dsnotifications）占 60；指示器紧跟其后。
	if orders["connection"] <= 60 {
		t.Fatalf("connection order = %d，应排在 settings.launcher(60) 之后", orders["connection"])
	}
	for _, panel := range p.Panels() {
		if panel.Slot == pluginapi.SlotShellBottom {
			t.Fatalf("连接指示不许再占 shell.bottom：%+v", panel)
		}
	}
}

// ② 官方座位真画：胶囊 + 文案 + 与设置按钮**同一行**（同一 Y 中心）。
func TestConnectionIndicatorDrawsInSettingsRow(t *testing.T) {
	h := newConnHarness(t)
	canvas := h.frame(t, pluginapi.LinkConnected)
	if !connPill(canvas, h.theme.SuccessTertiary, withAlpha(h.theme.SuccessPrimary, connIndicatorBorderTint)) {
		t.Fatalf("没画出 success 胶囊（官方 ._success 底色）：rects=%+v", canvas.Rectangles())
	}
	if got := connTexts(canvas); len(got) == 0 || got[0] != zhConnectionConnected {
		t.Fatalf("文案 = %v，want %q", got, zhConnectionConnected)
	}
	// 同一行：胶囊（28 高）必须与设置按钮（42 高）**垂直居中在同一行**，
	// 而不是各画一行（各画一行时行高会是 42+28 = 70，胶囊 Y 会整体下移）。
	pill, ok := connPillRect(canvas, withAlpha(h.theme.SuccessPrimary, connIndicatorBorderTint))
	if !ok {
		t.Fatal("找不到胶囊外圈矩形")
	}
	row, ok := connRowRect(canvas, h.theme.SidebarFill)
	if !ok {
		t.Fatal("找不到设置行矩形")
	}
	if row.Bounds.Height != connRowHeight {
		t.Fatalf("设置行高 = %.2f，want %.2f（官方 .trigger{height:42px}；70 = 两行叠在一起）",
			row.Bounds.Height, connRowHeight)
	}
	// 垂直居中：两个子项的中心是同一个 Y（`align-items:center` 的等价断言）。
	if got, want := pill.Bounds.Y+pill.Bounds.Height/2, row.Bounds.Y+row.Bounds.Height/2; got != want {
		t.Fatalf("胶囊中心 Y = %.2f，设置行中心 Y = %.2f（不在同一行）", got, want)
	}
	// 右缘：指示器 `flex:none` 被 `flex:1` 的设置按钮推到行右半侧。
	if pill.Bounds.X <= row.Bounds.X+row.Bounds.Width/2 {
		t.Fatalf("胶囊应贴在行的右半侧（设置按钮 flex:1 在左），胶囊 X = %.2f、行 X = %.2f",
			pill.Bounds.X, row.Bounds.X)
	}
}

// ③ 四态文案与色调：connected / down / connecting / unknown 各一条，
// 且**未知不写「已连接」**。
func TestConnectionIndicatorFourStates(t *testing.T) {
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
			icons:  1,
		},
		{
			// 官方 `connecting` 态：warning 色 + 加载弧 + `connection.connecting`。
			name: "重新连接中", link: pluginapi.LinkConnecting, want: zhConnectionConnecting + zhConnectionDots,
			fill:   func(t *pluginapi.Theme) ui.Color { return t.WarnTertiary },
			border: func(t *pluginapi.Theme) ui.Color { return withAlpha(t.WarnLabel, connIndicatorBorderTint) },
			icons:  1,
		},
		{
			// 壳还没拿到任何连接事实（首帧 / 还没订阅 / 老壳）：写「未知」，
			// 中性描边、不借用 success/warning 的色，也不编「已连接」。
			name: "未知", link: pluginapi.LinkUnknown, want: zhConnectionUnknown, icons: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newConnHarness(t)
			canvas := h.frame(t, tc.link)
			if got := connTexts(canvas); len(got) == 0 || got[0] != tc.want {
				t.Fatalf("文案 = %v，want %q", got, tc.want)
			}
			if tc.fill != nil {
				if !connPill(canvas, tc.fill(h.theme), tc.border(h.theme)) {
					t.Fatalf("缺官方底色胶囊：want %v；rects=%+v", tc.fill(h.theme), canvas.Rectangles())
				}
			} else if connPill(canvas, h.theme.SuccessTertiary, withAlpha(h.theme.SuccessPrimary, connIndicatorBorderTint)) ||
				connPill(canvas, h.theme.WarnTertiary, withAlpha(h.theme.WarnLabel, connIndicatorBorderTint)) {
				// 中性档：不许出现 success/warning 底色（那是"有结论"的配色）。
				t.Fatalf("未知态借用了成功/异常底色：rects=%+v", canvas.Rectangles())
			}
			if n := countOpaqueTriangles(canvas); tc.icons == 0 {
				if n != 0 {
					t.Fatalf("未知态不该画成功/异常图标，却画了 %d 个不透明三角形", n)
				}
			} else if n == 0 {
				t.Fatalf("%s 态缺官方 glyph（没有不透明描边三角形）", tc.name)
			}
			if tc.link == pluginapi.LinkUnknown {
				for _, banned := range []string{"就绪", "已连接", zhConnectionConnected} {
					if texts := connTexts(canvas); len(texts) > 0 && texts[0] == banned {
						t.Fatalf("未知态写了 %q——没有证据的断言", banned)
					}
				}
			}
		})
	}
}

// ④ 150ms 离场淡出（官方 `._leaving{opacity:0}` + `transition:opacity .15s`）。
//
// 状态机语义（与官方 ConnectionIndicator 的 `shown`/`leaving`/`state` 同构）：
//
//	目标态到了       旧态这一帧还是全不透明（1.0）
//	+75ms            旧态画成 0.5（线性淡出）
//	+150ms           换成新态（新态不透明度 1）
//
// 这条断言同时钉住"150ms 之后新态在位"，以及"150ms 之前不许提前换"。
func TestConnectionLeaveWindowIs150ms(t *testing.T) {
	h := newConnHarness(t)
	if texts := connTexts(h.frame(t, pluginapi.LinkDown)); len(texts) == 0 || texts[0] != zhConnectionError {
		t.Fatalf("首帧应是断开态：%v", texts)
	}
	// 目标态到了：这一帧还是旧态（全不透明）。
	canvas := h.frame(t, pluginapi.LinkConnected)
	if texts := connTexts(canvas); len(texts) == 0 || texts[0] != zhConnectionError {
		t.Fatalf("150ms 离场窗的第一帧应仍画旧态（断开），得到 %v", texts)
	}
	// +75ms：仍在旧态。
	h.now = h.now.Add(75 * time.Millisecond)
	canvas = h.frame(t, pluginapi.LinkConnected)
	if texts := connTexts(canvas); len(texts) == 0 || texts[0] != zhConnectionError {
		t.Fatalf("离场窗中途（75ms）应仍画旧态，得到 %v", texts)
	}
	// +150ms：换成新态（连着）。
	h.now = h.now.Add(75 * time.Millisecond)
	canvas = h.frame(t, pluginapi.LinkConnected)
	if texts := connTexts(canvas); len(texts) == 0 || texts[0] != zhConnectionConnected {
		t.Fatalf("150ms 后应换成新态（连接成功），得到 %v", texts)
	}
}

// TestConnectionAnimOpacity 是离场窗的**数值**断言（直接测状态机，不经过 viva 绘制）。
//
// 为什么单测状态机：viva 的 `Opacity` 是预乘进颜色的，而"圆角矩形填充"在不同
// 不透明度下会走不同的图元路径（矩形 / 三角 mesh），拿画布矩形反算不透明度
// 既脆又测不到"150ms 这个时长本身"。时长与曲线由本函数钉住。
func TestConnectionAnimOpacity(t *testing.T) {
	var a connectionAnim
	t0 := time.Unix(0, 0)
	if phase, op := a.advance(pluginapi.LinkDown, t0); phase != connPhaseDown || op != 1 {
		t.Fatalf("首帧 = (%v, %v)，want (down, 1)", phase, op)
	}
	// 目标态到了：旧态整帧不透明（`_leaving` 的过渡起点）。
	if phase, op := a.advance(pluginapi.LinkConnected, t0); phase != connPhaseDown || op != 1 {
		t.Fatalf("离场窗第一帧 = (%v, %v)，want (down, 1)", phase, op)
	}
	// +75ms：旧态画成 0.5。
	if phase, op := a.advance(pluginapi.LinkConnected, t0.Add(75*time.Millisecond)); phase != connPhaseDown || op != 0.5 {
		t.Fatalf("+75ms = (%v, %v)，want (down, 0.5)", phase, op)
	}
	// +150ms：换成新态。
	if phase, op := a.advance(pluginapi.LinkConnected, t0.Add(connLeaveWindow)); phase != connPhaseConnected || op != 1 {
		t.Fatalf("+150ms = (%v, %v)，want (connected, 1)", phase, op)
	}
	// 稳定态不抖动。
	if phase, op := a.advance(pluginapi.LinkConnected, t0.Add(500*time.Millisecond)); phase != connPhaseConnected || op != 1 {
		t.Fatalf("稳定态 = (%v, %v)，want (connected, 1)", phase, op)
	}
	// 离场窗是官方时长：`._indicator{transition:opacity .15s}`。
	if connLeaveWindow != 150*time.Millisecond {
		t.Fatalf("离场窗 = %v，官方 150ms", connLeaveWindow)
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

// ⑥ 图标来源：官方 sprite `check-outline` / `refresh-outline`（viewBox 16、regular=1）
// 与 `connecting` 态的 spinner 弧（viewBox 24、stroke 2）。
func TestConnectionIconsAreOfficialSprites(t *testing.T) {
	for _, tc := range []struct {
		name     string
		icon     gfx.Icon
		viewBox  float32
		stroke   float32
		paths    int // 官方 path 段数
		firstPt  ui.Point
		iconName string
	}{
		{"check-outline", checkOutlineIcon(), 16, 1, 1, ui.Point{X: 2.25, Y: 8.5}, "check-outline"},
		{"refresh-outline", refreshOutlineIcon(), 16, 1, 2, ui.Point{X: 14.5001, Y: 8}, "refresh-outline"},
		// 官方 spinner：24 viewBox、stroke 2、只转写 72° 的 arc（见 icons.go 的 derived 说明）。
		{"spinner-arc", loadingArcIcon(), 24, 2, 1, ui.Point{X: 12, Y: 2.5}, "spinner(ongoing)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.icon.ViewBox != tc.viewBox {
				t.Fatalf("%s viewBox = %v，want %v（官方 sprite）", tc.iconName, tc.icon.ViewBox, tc.viewBox)
			}
			if tc.icon.Stroke != tc.stroke {
				t.Fatalf("%s stroke = %v，want %v", tc.iconName, tc.icon.Stroke, tc.stroke)
			}
			moves := 0
			for _, command := range tc.icon.Commands {
				if command.Kind == gfx.PathCommandMove {
					moves++
				}
			}
			if moves != tc.paths {
				t.Fatalf("%s path 段数 = %d，want %d（官方 sprite）", tc.iconName, moves, tc.paths)
			}
			first := tc.icon.Commands[0]
			if first.Kind != gfx.PathCommandMove || first.Point != tc.firstPt {
				t.Fatalf("%s 首命令 = %+v，want MoveTo(%v, %v)（官方 d 的起点）",
					tc.iconName, first, tc.firstPt.X, tc.firstPt.Y)
			}
		})
	}
}

// ⑦ warning 态是**可点按钮**：命中区存在，点击真的走 Actions.Reconnect。
//
// 官方 `ConnectionIndicator` 的 warning 两态是 `<button onClick={onReconnect}>`；
// 只断言"画出来了"不够——`Clicked()` 没接动作时界面完全正常，点了没反应，
// 那正是任务书要拆掉的"假动作"。
func TestConnectionWarningIsReconnectButton(t *testing.T) {
	for _, tc := range []struct {
		name string
		link pluginapi.LinkState
		aria string
	}{
		{"断开态", pluginapi.LinkDown, zhConnectionReconnect},
		{"重连中", pluginapi.LinkConnecting, zhConnectionRestart},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newConnHarness(t)
			reconnects := 0
			h.act = pluginapi.Actions{Reconnect: func() { reconnects++ }}
			// 先画一帧让命中区就位（viva 的事件 pass 需要上一帧的命中区）。
			h.frame(t, tc.link)
			h.hit = "connection-reconnect"
			canvas := h.frame(t, tc.link)
			if reconnects != 1 {
				t.Fatalf("点击 warning 胶囊应发起 1 次真实重连，实际 %d 次", reconnects)
			}
			var label string
			for _, hit := range canvas.Hits() {
				if hit.ID == h.tree.ID(h.feat, "connection-reconnect") {
					label = hit.Label
				}
			}
			if label != tc.aria {
				t.Fatalf("命中区 aria-label = %q，want %q（官方 connection.reconnect / connection.restart）",
					label, tc.aria)
			}
		})
	}
}

// ⑧ connected / unknown 两态**不可点**：官方只把 warning 画成 button。
func TestConnectionSuccessAndUnknownAreNotButtons(t *testing.T) {
	for _, tc := range []struct {
		name string
		link pluginapi.LinkState
	}{
		{"已连接", pluginapi.LinkConnected},
		{"未知", pluginapi.LinkUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newConnHarness(t)
			reconnects := 0
			h.act = pluginapi.Actions{Reconnect: func() { reconnects++ }}
			h.frame(t, tc.link)
			h.hit = "connection-reconnect"
			canvas := h.frame(t, tc.link)
			if reconnects != 0 {
				t.Fatalf("%s 态不该可点（官方只把 warning 画成按钮），却触发了 %d 次重连", tc.name, reconnects)
			}
			for _, hit := range canvas.Hits() {
				if hit.ID == h.tree.ID(h.feat, "connection-reconnect") {
					t.Fatalf("%s 态画了重连命中区", tc.name)
				}
			}
		})
	}
}

// ⑨ 没有重连动作（老壳 / 测试）时 warning 态**不画成按钮**：
// 画一个点了没反应的东西比不画更坏。
func TestConnectionWarningWithoutReconnectIsNotClickable(t *testing.T) {
	h := newConnHarness(t)
	h.frame(t, pluginapi.LinkDown)
	h.hit = "connection-reconnect"
	canvas := h.frame(t, pluginapi.LinkDown)
	for _, hit := range canvas.Hits() {
		if hit.ID == h.tree.ID(h.feat, "connection-reconnect") {
			t.Fatal("Actions.Reconnect == nil 时不该画重连命中区（假动作）")
		}
	}
}
