// Package dsshortcuts 是一份**可查的键位说明**。
//
// 为什么需要它：键位散落在各插件里（侧栏吃 ↑↓/Home/End、composer 吃 Enter、
// 审批吃 Esc、ds-demo 吃 F12），而界面上**没有任何地方能查到"我还能按什么键"**。
// 用户只能靠读源码或者别人告诉。
//
// 这里做的事很窄：按 F1 弹一块面板列出**真实存在的绑定**，Esc 或再按 F1 收起。
// 不编：表里每一条都在本仓有对应的 `HandleKey`，注释里写了它在哪个文件。
//
// 它同时是「第三方插件只靠 ID/Requires/Register/Panels/HandleKey 就能接入」的第二个样例
// ——第一个是 dsdemo，那个更小，只有 Panels 没有 HandleKey。
package dsshortcuts

import (
	"sync"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Binding 是一条键位说明。Where 写的是**这条绑定真实存在于哪个包**，
// 不写"理论支持"——写了就有人会去试，试不通就是骗人。
type Binding struct {
	Key   string
	Does  string
	Where string
}

// bindings 是全部真实键位。改动这里之前先确认对应插件里那行 HandleKey 还在。
//
// **这张表必须覆盖代码里所有 HandleKey 吃掉的键**——不是"记得的键"。
// 这条规矩由 shortcuts_guard_test.go 卡住：从 plugins/ 下扫 `ui.KeyX`，
// 表里少一个就红。写这条守卫的原因见那个文件的开头。
var bindings = []Binding{
	{"↑ / ↓", "上下选会话（焦点在侧栏时）", "plugins/dssidebar"},
	{"Home / End", "跳到第一个 / 最后一个会话", "plugins/dssidebar"},
	{"Enter", "发送输入框里的内容", "plugins/dscomposer"},
	{"Esc", "拒绝当前审批请求（reject_once）", "plugins/dscomposer/approval.go"},
	{"Esc", "关闭搜索框（搜索打开时）", "plugins/dsconversation"},
	{"Esc / 停止键", "打断当前正在跑的 turn", "plugins/dscomposer（Cancel 接线）"},
	{"F1", "打开 / 关闭这份键位表", "plugins/dsshortcuts"},
	{"F2", "命令面板：模型 / 权限 / 反馈记录 / 栏宽", "plugins/dscommands"},
	{"F5", "打开 / 关闭终端面板", "plugins/dsterminal"},
	{"F6", "把终端输入框里的命令发出去", "plugins/dsterminal"},
	{"F7 / F8", "选第 1 / 第 2 个选项（提问条出现时）", "plugins/dsquestions"},
	{"F12", "收起 / 展开右上角的插件提示条", "plugins/dsdemo"},
	{"Tab / Shift+Tab", "设置面板打开时在面板内循环焦点（模态层陷阱）", "plugins/dssettings/official_modal.go"},
	{"Ctrl+O", "添加工作区（打开本次组合的目录选择器）", "cmd/app/dirpicker_wiring.go（OnShortcut 层，不是 HandleKey）"},
}

// Bindings 返回这份键位表的**只读拷贝**（设置面板「快捷键」行要显示条目数，
// 不能让它拿到内部切片去改）。
//
// 为什么设置行要用这张表而不是问宿主：官方 `shortcuts` 是**窗口本地**服务
// （`dsh-client-shortcuts/src/client/index.ts:31` 的 `ctx.shortcuts.catalog`），
// 键位来自 `DesktopShortcutsApi` 适配器，**不是 Host 设置**。本仓这张表就是
// 同等的真值来源，而且有 `shortcuts_guard_test.go` 卡着它覆盖所有 HandleKey。
func Bindings() []Binding {
	out := make([]Binding, len(bindings))
	copy(out, bindings)
	return out
}

// Plugin 只有「显示 / 收起」一个状态。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives

	// mu 保护 shown / notify：HandleKey 会从两个线程进来（窗口线程的 OnKey，
	// 以及 cmd/app `-keys` 的注入 goroutine），而 notify 是在绘制线程的 draw 里写的。
	mu     sync.Mutex
	shown  bool
	notify func(string)
	// blockedAddReason 是「添加工作区」当前被挡住的理由（空串 = 可用）。
	// 由 draw 每帧从 `env.Data.DirectoryPickerAddBlocked` 抄一份：绘制线程写，
	// 只有绘制线程读，所以不落在 mu 的保护区里（它和 notify 不同，后者会被
	// HandleKey 从另一个线程取）。
	blockedAddReason string
}

func (*Plugin) ID() string { return "ds-shortcuts" }

func (*Plugin) Provides() []string { return nil }

// Requires 与 dsdemo 一致：颜色令牌 + 基础图元。装配器据此保证 ds-core 先就位。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	return nil
}

// Panels 挂在 shell.overlay：它盖住整个框架，正好是弹层该在的地方。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID:    "shortcuts-sheet",
		Slot:  pluginapi.SlotShellOverlay,
		Order: 95, // 比 dsdemo 的提示条（Order 10）后画：两者同时出现时面板在上面
		Draw:  p.draw,
	}}
}

// HandleKey 实现 pluginapi.KeyHandler（**返回 bool**，不是 EventResult——
// 我第一次写成 EventResult，类型断言过不了，插件装上了却永远收不到键）。
//
// **返回 true 是有代价的**：事件不会再传给后面的插件。所以只在
// ①按 F1（是我的键）②面板开着时按 Esc（先关面板）两种情况下吃掉。
// 面板没开时按 Esc 必须放行——那是审批拒绝 / 取消 turn 的键，被我吞掉就出事了。
//
// 状态切换之后**必须请求一次重绘**：HandleKey 跑在绘制之外，改完 shown 之后
// 没有任何一帧会自动发生。pluginapi 没有「请重绘」这个动作，插件手里唯一的
// 公开入口是 Actions.Notify——它的宿主实现写状态栏 + Invalidate（cmd/app 的
// setStatus）。少了这一步，`-keys f1` 的日志是 `key 16 handled=true`、
// 屏幕上却什么都没有：上一轮「面板画不出来」卡的就是这里，不是 Absolute 布局
// （布局在 overlay_test.go 里用真 kernel + 真 dsshell 验过）。
func (p *Plugin) HandleKey(ev gfx.KeyEvent) bool {
	switch ev.Key {
	case gfx.KeyF1:
		p.mu.Lock()
		p.shown = !p.shown
		shown, notify := p.shown, p.notify
		p.mu.Unlock()
		if notify != nil {
			if shown {
				notify("键位表已打开 · F1 或 Esc 关闭")
			} else {
				notify("键位表已关闭")
			}
		}
		return true
	case gfx.KeyEscape:
		p.mu.Lock()
		shown := p.shown
		p.mu.Unlock()
		if !shown {
			return false
		}
		// 与面板里的关闭按钮走同一条收起路径（hide 里带重绘请求）。
		p.hide()
		return true
	}
	return false
}

// hide 收起面板并请求一帧重绘。
//
// 抽出来是因为两个入口都要它：HandleKey（Esc / F1）与面板自己的关闭按钮
// （官方 Reference.tsx:164 是个真的 `<button className={css.close}>`）。
func (p *Plugin) hide() {
	p.mu.Lock()
	p.shown = false
	notify := p.notify
	p.mu.Unlock()
	if notify != nil {
		notify("键位表已关闭")
	}
}

// draw 画官方快捷键面板：`.dialog` 外框 → `.header`（标题 + 关闭）→ `.list`（滚动，
// 逐条键位）。结构与官方 Reference.tsx:155-205 同构；数值全在 official_geometry.go，
// 逐项带官方 file:line，这里只负责"摆"。
func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
	// 记住「请求重绘」的入口，给 HandleKey 用（理由见 HandleKey 的注释）。
	// 每帧都写：Actions 是壳每帧新建的，缓存一份旧的不如跟着帧走。
	if env.Act.Notify != nil {
		p.mu.Lock()
		p.notify = env.Act.Notify
		p.mu.Unlock()
	}
	// 官方 `workspace.add` 的禁用理由（逐字来自 locales 的 shortcut.noPicker /
	// shortcut.directoryBusy）。它由壳算好交过来，本插件不自己判——判据只有一个
	// 真值来源（cmd/app/dirpicker_wiring.go 的 directoryAddBlockedReason）。
	p.blockedAddReason = env.Data.DirectoryPickerAddBlocked
	if !p.shown {
		return
	}
	t := p.theme
	// **必须 Absolute，并且必须显式 Width/Height**：
	//   - overlay 槽是"自由画"的区域，普通流式布局里它只是浮层行的一个孩子；
	//   - 只给 Left/Top 的绝对盒子，宽度取自内容的 pref；内容若是 FillWidth，
	//     Fill 又相对父盒子的宽度解析，于是父 0 → 子 0，整块退化成 0 宽（实测）。
	//   - 给两条边（Left+Right）会拉伸到容器宽；这里要的是官方设计宽度，所以给 Width。
	// 居中用的是 env.Bounds：overlay 槽的边界就是整窗（dsshell 用同一对数给
	// 浮层行定宽高，dsdemo 的提示条也靠它贴在右上角）。
	w := panelWidth(env.Bounds.Width)
	h := panelHeight(env.Bounds.Height)
	left := (env.Bounds.Width - w) / 2
	if left < 0 {
		left = 0
	}
	// Clip：官方 `.dialog` 是圆角+内部滚动容器，内容不该溢出圆角（CSS 里由
	// overflow 与 border-radius 的裁剪保证）。
	panel := ui.Column(f).Absolute().Left(left).Top(panelTop(env.Bounds.Height, h)).
		Width(w).Height(h).
		Radius(sheetRadius).Background(t.InputMajor).Shadow(gfx.ShadowLG).Clip()
	// 面板本体吃掉点击：官方这是 Modal，点框内不该落到下面的会话/侧栏上。
	// 先声明、后声明的子元素（关闭按钮）在它上面——viva 的命中按声明顺序取最后命中。
	if env.Tree != nil {
		panel = panel.HitWith(env.Tree.ID(env.Feat, "shortcuts-sheet"), shortcutsTitle,
			gfx.HitOptions{Role: gfx.RoleGroup})
	}
	panel.Children(func() {
		p.drawHeader(f, env, t)
		listH := h - sheetHeaderH
		if listH < 0 {
			listH = 0
		}
		p.drawList(f, t, listH)
	})
}

// drawHeader 画 `.header`（46 高、padding 22/54/0/24）与绝对定位的 `.close`。
//
// `flex:0 0 46px` 在 viva 里就是固定的 Height(46)：头部不参与伸缩，
// 剩下的高度全给 `.list`。
func (p *Plugin) drawHeader(f ui.C, env *pluginapi.Env, t *pluginapi.Theme) {
	ui.Row(f).FillWidth().Height(sheetHeaderH).
		PadSides(sheetHeaderPadTop, sheetHeaderPadRight, sheetHeaderPadBottom, sheetHeaderPadLeft).
		Items(ui.Start).Children(func() {
		// `.title{font-size:16px;line-height:24px;font-weight:500}`：
		// 22(padding-top) + 24(line-height) = 46，正好填满头部，不多不少。
		ui.Text(f, shortcutsTitle).FontSize(sheetTitleSize).LineHeight(sheetTitleLine).
			Weight(gfx.WeightMedium).Color(t.LabelPrimary)
	})
	// `.close{position:absolute;top:20px;right:14px;width:28px;height:28px;
	//  padding:6px;border-radius:28px}`。viva 的 Absolute 就是"相对容器 padding box
	//  按边定位"（ui/absolute.go:12-21），与 CSS position:absolute 同义。
	closeBtn := ui.Row(f).Absolute().Top(sheetCloseTop).Right(sheetCloseRight).
		Size(sheetCloseSize, sheetCloseSize).Radius(sheetCloseRadius).Center().
		Hover(ui.Look{}.Background(t.Hover))
	if env.Tree != nil {
		closeBtn = closeBtn.HitWith(env.Tree.ID(env.Feat, "shortcuts-close"),
			shortcutsCloseLabel, gfx.HitOptions{})
	}
	closeBtn.Children(func() {
		// 官方 `.close` 的字色是 label-primary，图标颜色跟着 currentColor 走。
		ui.Icon(f, closeOutlineIcon()).Size(sheetCloseIconSize, sheetCloseIconSize).
			Color(t.LabelPrimary)
	})
	if closeBtn.Clicked() {
		p.hide()
	}
}

// drawList 画 `.list`：官方是 `flex:1;min-height:0;overflow-y:auto` 的滚动容器，
// `padding:0 16px 18px`；行是 `.row{min-height:42px;padding:8px;border-radius:12px}`。
//
// 高度由调用方按"面板高 - 头部高"给定（等价 CSS 的 flex:1 剩余空间），
// 内容超出就在这一层滚动，而不是把面板撑高。
func (p *Plugin) drawList(f ui.C, t *pluginapi.Theme, height float32) {
	viewport := ui.Scroll(f).FillWidth().MinHeight(0).Height(height).Scrollbars()
	viewport.Children(func() {
		ui.Column(f).FillWidth().
			PadSides(0, sheetListPadX, sheetListPadBottom, sheetListPadX).Children(func() {
			// 本仓补充的一行提示（官方面板没有这一行）：字号/行高/颜色/外边距全用
			// 官方 `.hint` 与 `p.hint{margin:6px 0}`，不另造一套。
			ui.Text(f, shortcutsHint).FontSize(sheetHintSize).LineHeight(sheetHintLine).
				Color(t.LabelTertiary).
				MarginTop(sheetHintMarginY).MarginBottom(sheetHintMarginY)
			for _, bd := range bindings {
				p.drawBindingRow(f, t, bd)
			}
			// 「添加工作区」当前若被挡住，把官方那句理由原样列出来
			// （`Snapshot.DirectoryPickerAddBlocked`，逐字取自
			// `ui-workspace/src/client/locales.ts:16-17` 的 `shortcut.noPicker` /
			// `shortcut.directoryBusy`）。空串 = 可用，这一行不出现——
			// 「没话说时多画一行」会让这份表看起来像在报错。
			if reason := p.blockedAddReason; reason != "" {
				ui.Text(f, "Ctrl+O 当前不可用："+reason).FontSize(sheetHintSize).
					LineHeight(sheetHintLine).Color(t.LabelTertiary).
					MarginTop(sheetHintMarginY).MarginBottom(sheetHintMarginY)
			}
		})
	})
}

// drawBindingRow 画一行：`.commandLabel` 在左（长文本省略号）、`.keyBadge` 贴右。
//
// 官方 `.row{display:flex;justify-content:space-between;gap:8px}`：命令名在左、
// 键位徽标在右。旧的"键名列定宽 120 + 键名在左"没有官方出处，已按官方顺序摆正。
func (p *Plugin) drawBindingRow(f ui.C, t *pluginapi.Theme, bd Binding) {
	ui.Row(f).FillWidth().MinHeight(sheetRowMinH).Pad(sheetRowPad).Gap(sheetRowGap).
		Radius(sheetRowRadius).Items(ui.Center).Children(func() {
		// `.commandLabel{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;min-width:0}`
		ui.Text(f, bd.Does).FontSize(sheetRowFont).LineHeight(sheetRowLine).
			Color(t.LabelPrimary).Grow(1).MinWidth(0).Ellipsis()
		// `.keyBadge{height:26px;padding:0 12px;border-radius:10px;
		//  background:var(--dsw-alias-bg-module-platform)}` +
		// `.binding .keyBadge{color:var(--dsw-alias-label-secondary)}`。
		// 徽标字号继承 `.row` 的 13/18（官方 .keyBadge 不自带 font-size）。
		ui.Row(f).Height(sheetKeyH).
			PadSides(0, sheetKeyPadX, 0, sheetKeyPadX).Radius(sheetKeyRadius).
			Background(t.BgModulePlatform).Center().Self(ui.Center).Children(func() {
			ui.Text(f, bd.Key).FontSize(sheetRowFont).LineHeight(sheetRowLine).
				Color(t.LabelSecondary)
		})
	})
}
