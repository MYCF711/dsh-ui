// Package dsshell 是框架插件（L3）：三栏几何 + 底部整行 + 浮层。
//
// 它把「窗口里怎么分栏」从壳里搬了出来——壳只画一个 shell.root 槽，
// 这个插件在 root 里切出侧栏 / 中栏 / 右栏 / 底部整行，然后把 frame 交给
// 下一层（sidebar.*  / main.* / rightbar / shell.bottom）。
//
// 几何照官方 dsh-client-ui-layout：
//
//	侧栏   280（官方 264–420，默认 280）
//	中栏   至少 400（官方为保中栏先压右栏到 300，再报空间不足）
//	右栏   宽度由占用方给；壳算 MainWidth 时扣掉
//	底部   横跨三列的整行，占用方自己声明高度，空着不占位
//
// 插件不需要知道自己挂在第几层：它只认「父级把 f 递给我」。
package dsshell

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
)

// Plugin 实现 pluginapi.Plugin / Panels。
type Plugin struct {
	theme pluginapi.Theme
	prims pluginapi.Primitives

	// SidebarWidth 侧栏宽度（官方默认 280）。
	SidebarWidth float32
	// RightbarWidth 右栏宽度；<=0 时按官方首次打开的口径算（视口 45%）。
	RightbarWidth float32
	// RightbarOn 右栏当前是否展开（壳在快照里传进来，这里只镜像）。
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
	f.Column().FillWidth().FillHeight().Children(func() {
		f.Row().FillWidth().Grow(1).Children(func() {
			p.drawSidebar(f, env, drawSlot)
			p.drawMain(f, env, drawSlot)
			p.drawRightbar(f, env, drawSlot)
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
func (p *Plugin) drawSidebar(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	if !env.Has(pluginapi.SlotSidebarHeader) && !env.Has(pluginapi.SlotSidebarList) &&
		!env.Has(pluginapi.SlotSidebarFooter) {
		return
	}
	sw := env.Geometry.SidebarWidth
	if sw <= 0 {
		sw = p.SidebarWidth
	}
	// 内边距照官方侧栏根：`.wtg9ga_root{padding:6px var(--dsh-sidebar-inline-padding)}`，
	// 其中 `--dsh-sidebar-inline-padding:12px`。所以上下 6、左右 12，不是四边 12。
	//
	// Shrink(0) 是必须的：侧栏是**定宽**（官方 264–420，默认 280），不参与压缩。
	// 不写它的话，中间栏 + 右栏一起超宽时 flex 会把侧栏压到 239，
	// 而侧栏内部的滚动区是按 280 算的（280-24=256），于是每一行都往中栏里
	// 探出 41px——悬停/选中底色会横穿到会话正文上（这次截图里量出来的：
	// 行背景 x=20…276，而侧栏列只到 x=247）。
	f.Column().Width(sw).Shrink(0).FillHeight().PadX(12).PadY(6).Gap(8).
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
func (p *Plugin) drawMain(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	w := env.Geometry.MainWidth
	if w <= 0 {
		w = env.MainWidth
	}
	if w < 400 {
		w = 400 // 官方为保中栏，宁可挤别的
	}
	f.Column().Width(w).Grow(1).FillHeight().Pad(12).Gap(8).Children(func() {
		if env.Has(pluginapi.SlotMainHeader) {
			drawSlot(pluginapi.SlotMainHeader)
		}
		drawSlot(pluginapi.SlotMainBody)
		drawSlot(pluginapi.SlotMainComposer)
	})
}

// drawRightbar 右栏：没有占用方、或壳说收起，就整列不占位。
func (p *Plugin) drawRightbar(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	if !env.Data.ShowRightbar || !env.Has(pluginapi.SlotRightbar) {
		return
	}
	w := env.Geometry.RightWidth
	if w <= 0 {
		w = env.Bounds.Width * 0.45 // 官方：首次打开占视口 45%
	}
	f.Column().Width(w).FillHeight().Pad(12).Gap(8).
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
