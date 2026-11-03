// Package dsshell 是框架插件（L3）：三栏几何 + 底部整行 + 浮层。
//
// 它把「窗口里怎么分栏」从壳里搬了出来——壳只画一个 shell.root 槽，
// 这个插件在 root 里切出侧栏 / 中栏 / 右栏 / 底部整行，然后把 frame 交给
// 下一层（sidebar.*  /  main.* / rightbar / shell.bottom）。
//
// 三栏宽度**不是拍脑袋的比例**，是官方 `dsh-client-ui-layout` 的
// `computeColumns` 逐行移植（asar 里 `AppFrame.js` 的原函数）：
//
//	function computeColumns(viewport, sidebar, rightbar, collapsedWidth = 56) {
//	    const s = sidebar === 0 ? collapsedWidth : clampWidth(sidebar, 264, 420);
//	    const available = viewport - s - 400;
//	    const r = rightbar === 0 || available < 300 ? 0
//	            : Math.min(available, clampWidth(rightbar, 300, viewport * .7));
//	    return { sidebar: s, center: Math.max(0, viewport - s - r), rightbar: r };
//	}
//
// 三条规则值得单独记住（以前都踩过）：
//
//	侧栏   264–420 定宽、不参与压缩；没占用方时整列不占位
//	中栏   至少 400，**只有右栏整个不出现时**才允许被压到 400 以下
//	右栏   首次打开占视口 45%；放不下（available < 300）时官方是**整列消失**，
//	       不是压到 300 硬塞——硬塞会把中栏挤到 400 以下
//	底部   横跨三列的整行，占用方自己声明高度，三列按它的高度变矮
//
// 插件不需要知道自己挂在第几层：它只认「父级把 f 递给我」。
package dsshell

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
)

// 官方 ui-layout 的常量（asar：`@deepseek-ai/dsh-client-ui-layout`）。
const (
	sidebarMin = 264 // 官方 clampWidth(sidebar, 264, 420) 的下界
	sidebarMax = 420 // 同上界
	// sidebarRail 是官方「收起后保留 56px 控制栏」的轨道宽度。
	// 这一版没有折叠功能，solveColumns 里由调用方传 collapsedWidth；
	// 传 0 表示「没有占用方就整列不占位」（本插件既有约定）。
	sidebarRail = 56

	mainMin = 400 // 官方中栏下限：minmax(400px, 1fr)
	// rightbarMin / rightbarMaxFrac 是官方右栏的两端：clampWidth(rightbar, 300, viewport*.7)。
	rightbarMin      = 300
	rightbarMaxFrac  = 0.7
	rightbarOpenFrac = 0.45 // 官方首次打开：layoutInfo.rightbar ?? viewport * .45
)

// columnPad 是中栏/右栏由壳加的内边距。
//
// 官方 AppFrame 的 centerCol / rightbarCol 都是**裸列**（CSS 里没有 padding，
// 内容 inset 归占用方：ChatView scroll 32、composer clearance 16、右栏面板自画）。
// 我们这里保留 12，是因为它已经是**跨插件契约**：
// `plugins/dscomposer/composer.go` 的 `shellMainPad = 12` 就是按「12 + 4 = 官方 16」
// 校过的第二个出处，单方面把这里改成 0，composer 的净边距会从 16 掉到 4。
// 切到官方裸列模型要 dsshell + dscomposer + dsconversation + dstopbar 同一次改，
// 见 V4A 报告的「缺口」一节；到时候只改这个常量一处即可。
const columnPad = 12

// columns 是官方 computeColumns 的结果（像素）。
type columns struct {
	sidebar  float32
	center   float32
	rightbar float32
}

// clampWidth 对照官方 `clampWidth(px, min, max)`：
//
//	return Math.min(max, Math.max(min, Math.round(px)));
//
// 官方多一次 Math.round。这里保留取整：窗口宽度是整数，差值取整后
// 三列之和才能与视口严格相等（否则 0.5px 的余量会随机分给某一列）。
func clampWidth(px, lo, hi float32) float32 {
	if px < lo {
		px = lo
	}
	if px > hi {
		px = hi
	}
	return float32(round32(px))
}

// solveColumns 是官方 computeColumns 的逐行移植。
//
// 参数：
//
//	viewport      整窗宽（客户区，不是含标题栏的整窗）
//	sidebarPref   侧栏偏好宽；<=0 表示「没有占用方」，此时用 collapsedWidth
//	rightbarPref  右栏偏好宽（首次打开用 viewport*0.45）；<=0 表示没有偏好
//	rightbarOn    右栏这一帧要不要出现（占用方在 + 壳说展开）
//	collapsedWidth 侧栏折叠后保留的轨道宽；0 = 整列不占位
//
// 右栏的三条下落顺序照官方写死：
//  1. 没有占用方/没收起 → 0；
//  2. 扣掉侧栏和中栏下限后 available < 300 → 0（整列消失，不是压窄）；
//  3. 否则 min(available, clamp(pref, 300, viewport*0.7))。
func solveColumns(viewport, sidebarPref, rightbarPref float32, rightbarOn bool, collapsedWidth float32) columns {
	if viewport <= 0 {
		return columns{}
	}
	var s float32
	if sidebarPref <= 0 {
		s = collapsedWidth
	} else {
		s = clampWidth(sidebarPref, sidebarMin, sidebarMax)
	}
	var r float32
	if rightbarOn && rightbarPref > 0 {
		if available := viewport - s - mainMin; available >= rightbarMin {
			r = min(available, clampWidth(rightbarPref, rightbarMin, viewport*rightbarMaxFrac))
		}
	}
	c := viewport - s - r
	if c < 0 {
		c = 0
	}
	return columns{sidebar: s, center: c, rightbar: r}
}

// round32 把像素值取整（官方 clampWidth 里的 Math.round）。
// 自己写是为了不引 math 包：这里只处理 0..1e4 的正数，加 0.5 再截断即可。
func round32(v float32) float32 {
	if v < 0 {
		return float32(-int32(-v + 0.5))
	}
	return float32(int32(v + 0.5))
}

// Plugin 实现 pluginapi.Plugin / Panels。
type Plugin struct {
	theme pluginapi.Theme
	prims pluginapi.Primitives

	// SidebarWidth 侧栏默认宽度（官方默认 280）。
	// 壳在 Geometry 里给了宽度就以壳为准，这里只是「壳没说」时的兜底。
	SidebarWidth float32
	// RightbarWidth 右栏宽度；<=0 时按官方首次打开的口径算（视口 45%）。
	RightbarWidth float32
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-shell" }

// Provides 实现 pluginapi.Plugin。
func (*Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Register 实现 pluginapi.Plugin。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = *ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	if p.SidebarWidth <= 0 {
		p.SidebarWidth = 280
	}
	return nil
}

// Panels 实现 pluginapi.Panels。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "shell", Slot: pluginapi.SlotShellRoot, Order: 10, Draw: p.draw, Children: p.children},
	}
}

// layout 把「这一帧的三栏该多宽」算成官方 computeColumns 的结果。
//
// 输入优先级：壳给的 Geometry（它知道侧栏偏好、右栏开合、白名单里有没有占用方）
// → 插件自己的默认值 → 官方比例。绝不拿 f.Bounds() 当列宽：槽位里那是整窗宽。
func (p *Plugin) layout(env *pluginapi.Env) columns {
	vw := env.Geometry.Width
	if vw <= 0 {
		vw = env.Bounds.Width
	}
	if vw <= 0 {
		// 连视口都不知道时不编宽度：返回全 0，让各列按「没有」处理。
		return columns{}
	}

	// 侧栏：没有 sidebar.* 占用方就整列不占位（本插件的既有约定）。
	var sidebarPref float32
	if env.Has(pluginapi.SlotSidebarHeader) || env.Has(pluginapi.SlotSidebarList) ||
		env.Has(pluginapi.SlotSidebarFooter) {
		sidebarPref = env.Geometry.SidebarWidth
		if sidebarPref <= 0 {
			sidebarPref = p.SidebarWidth
		}
	}

	rightbarOn := env.Data.ShowRightbar && env.Has(pluginapi.SlotRightbar)
	rightbarPref := env.Geometry.RightWidth
	if rightbarPref <= 0 {
		rightbarPref = p.RightbarWidth
	}
	if rightbarPref <= 0 {
		rightbarPref = vw * rightbarOpenFrac // 官方：首次打开占视口 45%
	}

	return solveColumns(vw, sidebarPref, rightbarPref, rightbarOn, 0)
}

// syncMainWidth 把 env.MainWidth 校正成**本帧真正画出来的中栏宽**。
//
// 为什么插件要动这个字段：壳里另有一份 `geomRightWidth`（cmd/app/main.go），
// 它算右栏时和官方在「中栏下限」这一步不同——官方是「放不下就让右栏整列
// 消失」，壳是「把右栏压到 300 硬塞」。两者只在视口 < ~984（侧栏 280 时）分叉，
// 而那正是窄窗口最常见的样子：壳说主区 380，这里按官方画 680，
// 下游若照 380 排版，内容就缩在左半边（和 main.go 注释里踩过的
// 「按 600 排版、真主列只有 495」是同一类错，只是方向反过来）。
//
// 两档验收尺寸（1920×1032 / 1182×832）下两份算法结果恒等，
// 所以这一步不改变任何现有像素，只补上「窄窗口下画的和说的不一致」。
func syncMainWidth(env *pluginapi.Env, cols columns) {
	if cols.center > 0 {
		env.MainWidth = cols.center
	}
}

// draw 只画底色：分栏几何在 children 里。
//
// 底色的画法照官方 AppFrame：`.mP3x-q_frame{background:var(--dsw-alias-bg-base)}`
// 铺满整框。这里直接落在画布上而不是开一个 Box——开 Box 会在 root 列里
// 多占一个孩子，把后面的行挤出视口（踩过：底部整行跑到窗口外面）。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	env.Canvas.Fill(env.Bounds, p.theme.BgBase)
}

// children 把 root 切成「上：三列」+「下：底部整行」两行。
//
// 官方 AppFrame 是 grid：
//
//	.mP3x-q_frame{grid-template-rows:minmax(0,1fr) auto;display:grid}
//	.mP3x-q_bottomRow{grid-column:1/-1;background:var(--dsw-alias-bg-base)}
//
// 也就是**底部条横跨三列整行**，且三列因为它的高度而变矮。
// 这里用等价的 Column(主体 Grow(1) + 底部定高) 表达，
// 而不是把底部条绝对定位——绝对定位的元素不参与父级布局，
// 先前那一版状态栏因此既没从 x=0 开始，也没把三列压矮。
func (p *Plugin) children(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	// 一帧只算一次：三列宽度必须来自同一次求解，
	// 否则右栏被判定「放不下」而中栏还按有右栏算，中间会空一条或被挤出窗口。
	cols := p.layout(env)
	syncMainWidth(env, cols)
	f.Column().FillWidth().FillHeight().Children(func() {
		f.Row().FillWidth().Grow(1).Children(func() {
			p.drawSidebar(f, env, drawSlot, cols)
			p.drawMain(f, env, drawSlot, cols)
			p.drawRightbar(f, env, drawSlot, cols)
		})
		p.drawBottom(f, env, drawSlot)
		// 浮层放**最后**画：viva 按绘制顺序叠，靠后的盖在靠前的上面。
		// 这一步漏了整整一轮——`ds-demo` 装配日志里明明有，界面上却什么都没有，
		// 因为没人画 shell.overlay 这个槽。
		//
		// 官方对应 `.mP3x-q_overlayLayer{position:absolute;inset:0;pointer-events:none}`：
		// 覆盖整个框架的绝对定位层。这里同样从 (0,0) 铺满整窗，而不是铺满某一列。
		if env.Has(pluginapi.SlotShellOverlay) {
			f.Row().Absolute().Left(0).Top(0).
				Width(env.Bounds.Width).Height(env.Bounds.Height).
				Children(func() {
					drawSlot(pluginapi.SlotShellOverlay)
				})
		}
	})
}

// drawSidebar 左栏：只有挂了 sidebar.* 的插件才出现。
func (p *Plugin) drawSidebar(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot), cols columns) {
	if cols.sidebar <= 0 {
		return
	}
	// 内边距照官方侧栏根：`.wtg9ga_root{padding:6px var(--dsh-sidebar-inline-padding)}`，
	// 其中 `--dsh-sidebar-inline-padding:12px`。所以上下 6、左右 12，不是四边 12。
	//
	// Shrink(0) 是必须的：侧栏是**定宽**（官方 264–420，默认 280），不参与压缩。
	// 不写它的话，中间栏 + 右栏一起超宽时 flex 会把侧栏压到 239，
	// 而侧栏内部的滚动区是按 280 算的（280-24=256），于是每一行都往中栏里
	// 探出 41px——悬停/选中底色会横穿到会话正文上（这次截图里量出来的：
	// 行背景 x=20…276，而侧栏列只到 x=247）。
	f.Column().Width(cols.sidebar).Shrink(0).FillHeight().PadX(12).PadY(6).Gap(8).
		Background(p.theme.SidebarFill).Children(func() {
		drawSlot(pluginapi.SlotSidebarHeader)
		p.prims.Divider(f)
		f.Column().Grow(1).MinHeight(120).Children(func() {
			drawSlot(pluginapi.SlotSidebarList)
		})
		if env.Has(pluginapi.SlotSidebarFooter) {
			p.prims.Divider(f)
			drawSlot(pluginapi.SlotSidebarFooter)
		}
	})
}

// drawMain 中栏：宽度必须钉死（百分比在槽位里按整窗解析，内容会跑到窗口外）。
//
// 宽度用 solveColumns 的 center，而不是「窗体减一遍侧栏右栏」——
// 官方只在没有右栏时才允许中栏低于 400，这条规则只写在求解器里。
// Grow(1) 留着当安全网：三列之和按构造等于视口，正常情况下它不起作用。
func (p *Plugin) drawMain(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot), cols columns) {
	w := cols.center
	if w <= 0 {
		w = env.MainWidth // 视口未知时的兜底（壳算好的主区宽）
	}
	f.Column().Width(w).Grow(1).FillHeight().Pad(columnPad).Gap(8).Children(func() {
		if env.Has(pluginapi.SlotMainHeader) {
			drawSlot(pluginapi.SlotMainHeader)
		}
		// body 是可伸展轨道：composer 的位置由主区几何决定，
		// 不能依赖空态/消息流占用方自己记得 Grow。
		f.Column().FillWidth().Grow(1).MinHeight(0).Children(func() {
			drawSlot(pluginapi.SlotMainBody)
		})
		drawSlot(pluginapi.SlotMainComposer)
	})
}

// drawRightbar 右栏：没有占用方、壳说收起、或**扣完中栏下限放不下**时，
// 整列不占位（官方 computeColumns 的 available < 300 → 右栏宽度 0）。
//
// 放不下时不是压到 300 硬塞：那样中栏会被挤到 400 以下，
// 而官方中栏只有在右栏整个不出现时才允许低于 400。
//
// 底色这里画 SidebarFill，和官方有一处**已知差异**：官方 `.mP3x-q_rightbarCol`
// 在 Windows 上不设背景（透出 frame 的 bg-base），面板底色归 dockkit pane 自己画。
// 我们的右栏占用方（plugins/dsrightbar）没有自己的面板框，直接画在列里，
// 所以这层底色只能由列来给——否则右栏和主区同色、边界消失。
// 真正的面板底色（pane 的 CSS）没在打包产物里找到，等 V4B 定右栏面板形态时一起收。
func (p *Plugin) drawRightbar(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot), cols columns) {
	if cols.rightbar <= 0 {
		return
	}
	f.Column().Width(cols.rightbar).FillHeight().Pad(columnPad).Gap(8).
		Background(p.theme.SidebarFill).Children(func() {
		drawSlot(pluginapi.SlotRightbar)
	})
}

// drawBottom 底部整行：占用方自己声明高度；没占用方就一行都不占。
//
// 几何照官方 `.mP3x-q_bottomRow{background:var(--dsw-alias-bg-base);grid-column:1/-1}`：
// **横跨整行**（从窗口左边缘到右边缘，不是从中栏开始），底色 bg-base，
// 高度由占用方决定（官方 slot 文档：「Its occupant controls its own height
// and visibility; the columns shrink by that height」）。
//
// 用 Column 不是 Row：底部槽里通常是「一条分隔线 + 一行内容」，
// 竖着排。用 Row 会让分隔线和内容并排，背景只剩右半截。
func (p *Plugin) drawBottom(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	if !env.Has(pluginapi.SlotShellBottom) {
		return
	}
	h := env.Geometry.BottomHeight
	if h <= 0 {
		h = 24
	}
	f.Column().FillWidth().Height(h).Background(p.theme.BgBase).Children(func() {
		drawSlot(pluginapi.SlotShellBottom)
	})
}

var (
	_ pluginapi.Plugin = (*Plugin)(nil)
	_ pluginapi.Panels = (*Plugin)(nil)
)

// _ 保证 draw / children 被引用（调试期临时）。
var _ = func() bool {
	println("DBG dsshell funcs wired")
	return true
}
