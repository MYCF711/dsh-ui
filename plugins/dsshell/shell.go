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
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	f.Row().FillWidth().FillHeight().Background(p.theme.BgBase)
}

// children 把 root 切三层，再把 frame 交给下一层。
func (p *Plugin) children(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	// 注意：这里不要再包一层 Column。draw 已经建了一个填满的 Row，
	// 再套 Column 会多消耗一次剩余高度，实测整窗白屏。
	// 主体用 Grow(1)，不重新开 Row：f 本身已经是 draw 建好的那一行。
	f.Row().FillWidth().Grow(1).Children(func() {
		p.drawSidebar(f, env, drawSlot)
		p.drawMain(f, env, drawSlot)
		p.drawRightbar(f, env, drawSlot)
	})
	p.drawBottom(f, env, drawSlot)
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
	f.Column().Width(sw).FillHeight().Pad(12).Gap(8).
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
// 占位行 + 绝对定位两步走：只绝对定位的话，主体和侧栏会画到窗口底边被压住（踩过）。
func (p *Plugin) drawBottom(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	if !env.Has(pluginapi.SlotShellBottom) {
		return
	}
	h := env.Geometry.BottomHeight
	if h <= 0 {
		h = 24
	}
	f.Row().FillWidth().Height(h)
	f.Row().Absolute().Left(0).Bottom(0).Width(env.Bounds.Width).Height(h).
		Children(func() {
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
