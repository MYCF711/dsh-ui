package uikernel

// entry_menu.go —— 官方三个「区域 More 菜单」的**那层菜单面**（区域宿主的公共一半）。
//
// # 官方事实（rc.2 随包 src/**）
//
// 三个区域各有一枚 More 入口，点开是一层菜单，菜单行由 `*.actions` list 座位贡献：
//
//	左栏面板列表   PanelMoreMenu   OFF:ui-sidebar/src/client/SidebarRoot.tsx:107-133
//	                锚点 = `button.panelRow`（与面板行同几何）+ 展开态同底色
//	会话头 view 行 ViewsMoreMenu   OFF:ui-conversation/src/client/skeleton/ConversationSession.tsx:27-45
//	                锚点 = `Button.size=sm` + `IconEllipsisOutlineRegular size=13`
//	右栏 guide     GuideMoreMenu   OFF:ui-sidebar-right/src/client/tabs/guide/GuideBody.tsx:99-117
//	                锚点 = 一张 guide 卡片（本仓 guide 面未落地，见 docs/official-parity.md #53）
//
// 三处 Menu 的**面**是同一个 primitives `MenuSurface`：portal 出去、贴着锚点、
// 放不下翻到另一侧、被夹在窗口内。可引用的官方数值：
//
//	.moreMenu{width:288px;max-width:calc(100vw - 24px)}   SidebarRoot.module.css:726-729
//	菜单贴锚点：`top:calc(anchor(bottom) + 4px)` / 窗口内留 12（dist CSS 的
//	  `menu-below` / `menu-above` 规则：`top:max(…, calc(anchor(bottom) + 4px))`、
//	  `bottom:12px`）
//	面的内边距/圆角/底色：与官方 composer 菜单面同一档（ModelSelect.module.css:107-135：
//	  padding 4 / radius md / elevation-prominent），本仓已在
//	  `official_model_menu.go` 按那一行落过（那一层也是 primitives MenuSurface）。
//
// # 画什么、不画什么
//
//   - **画**：菜单面（宽度/内边距/圆角/边框/阴影）+ 关掉用的整窗遮罩 + 把区域
//     `*.actions` 座位 dispatch 进面板里（带 owner props，见 pluginapi 的
//     `Env.EntryMenu`）。
//   - **不画**：入口本体与菜单行。入口几何三个区域各不相同（面板行 / sm 按钮 /
//     guide 卡片），归各自宿主；菜单行归 `dsh-client-ui-entry-customization` 的
//     等价物（本仓 #17），它读 `env.EntryMenu` 画自己那几行。
//
// # 为什么遮罩不是"模态"
//
// 官方 Menu 是 portal 出去的一层，点面外即关闭（`onClose`），**没有** backdrop、
// 也不抢焦点（primitives MenuSurface 的文件注释：「portal … no dismissal」那层
// 由 Menu 自己给）。这里用一个整窗透明命中区表达"点外面 = onClose"，
// 盖在菜单面之前声明——viva 同层内**后声明的赢**，所以点菜单面不会落到遮罩上。

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 三个区域 More 菜单的官方几何（出处见文件头）。
const (
	// EntryMoreMenuWidth 是 `.moreMenu{width:288px}`（SidebarRoot.module.css:726-729）。
	EntryMoreMenuWidth float32 = 288
	// EntryMoreMenuEdgeMargin 是菜单与窗口左右/上下沿的最小距离（12px）。
	EntryMoreMenuEdgeMargin float32 = 12
	// EntryMoreMenuAnchorGap 是菜单与锚点之间的间距（`calc(anchor(bottom) + 4px)`）。
	EntryMoreMenuAnchorGap float32 = 4
	// EntryMoreMenuPad 是菜单面的内边距（与官方 composer 菜单面同一档）。
	EntryMoreMenuPad float32 = 4
)

// EntryMoreMenuWidthFor 是官方 `.moreMenu{width:288px;max-width:calc(100vw - 24px)}`
// 的等价物：288 与"窗口宽减两侧 12"取小。<=0 表示窗口窄到放不下。
func EntryMoreMenuWidthFor(boundsWidth float32) float32 {
	return entryMenuWidthFor(EntryMoreMenuWidth, boundsWidth)
}

// entryMenuWidthFor 是三个区域共用的宽度规则：`max-width:calc(100vw - 24px)`
// 三处逐字相同（`SidebarRoot.module.css:726-729`；右栏 guide 那处
// `GuideBody.module.css:128` 是 `.moreMenu{width:max-content;max-width:min(480px,calc(100vw - 24px))}`），
// 所以先夹声明的上限再夹窗口——窗口窄到放不下时窗口优先，否则菜单会伸到窗口外。
func entryMenuWidthFor(declared, boundsWidth float32) float32 {
	cap := boundsWidth - 2*EntryMoreMenuEdgeMargin
	if cap <= 0 {
		return cap
	}
	if declared > 0 && cap > declared {
		return declared
	}
	return cap
}

// entryMenuAnchorWidth 是右栏 guide 那层菜单的实宽：官方 `width:max-content` +
// `minWidthFromAnchor`（GuideBody.tsx:104）。
//
// 本仓那两行（「浏览更多插件」/「创建你的插件」）的 max-content 比锚点窄，所以
// `minWidthFromAnchor` 那一半就是最终宽：**卡片多宽，菜单就多宽**（官方桌面档下
// 380 宽）。Viva 这边没有"一段文字在菜单行里的固有宽"这个读口，所以不去近似
// max-content，而是按官方在这一档的**实宽**取锚点宽；窄栏时锚点自己先被
// `.entryCell{max-width:100%}` 收窄，菜单跟着窄，最后再被 `min(480, 100vw-24)` 夹住。
func entryMenuAnchorWidth(declared, anchorWidth, boundsWidth float32) float32 {
	cap := entryMenuWidthFor(declared, boundsWidth)
	if anchorWidth > 0 && anchorWidth < cap {
		return anchorWidth
	}
	return cap
}

// EntryMenuOptions 是画一层区域 More 菜单面需要的一帧输入。
type EntryMenuOptions struct {
	// Theme 是主题令牌；nil 时用 env.Theme。
	Theme *pluginapi.Theme
	// Anchor 是入口按钮的**窗口坐标**矩形（宿主在 Draw 里量到的真实矩形）。
	Anchor ui.Rect
	// Bounds 是整窗矩形（覆层坐标）。
	Bounds ui.Rect
	// Open 是菜单展开态（宿主持有，入口点击时切换）。
	Open bool
	// Slot 是本区域的内容座位（`sidebar.panellist.actions` 等），由宿主传。
	Slot pluginapi.Slot
	// Region 是区域身份，写进 owner props（`pluginapi.EntryRegion*`）。
	Region string
	// Entries 是本区域**全部**条目（含已固定；筛不筛由占用方按官方决定）。
	Entries []pluginapi.EntryMenuEntry
	// OpenEntry / SetPinned / Dismiss 是官方 owner props 的 onOpen /
	// onPinnedChange / onDismiss。nil = 宿主没接（占用方据此不画对应按钮）。
	OpenEntry func(id string)
	SetPinned func(id string, pinned bool)
	Dismiss   func()
	// Dispatch 是内核的 drawSlot（`Painter.Children` 拿到的那个）。
	Dispatch func(pluginapi.Slot)
	// MenuWidth 是这层菜单面的宽度上限（官方 `.moreMenu{max-width:…}`）。
	//
	// 0 = 跟档次走：`AnchorMinWidth` 为假时是左栏/会话头的 288，为真时是 guide 的 480。
	// 两处都是官方声明值，调用方不必自己再写一遍。
	MenuWidth float32
	// AnchorMinWidth 让菜单宽跟锚点走（官方 guide 那层 Menu 的 `minWidthFromAnchor`，
	// GuideBody.tsx:104；实宽见 `entryMenuAnchorWidth`）。只有 guide 那处为真。
	AnchorMinWidth bool
	// PanelHeight 是**上一帧量到的**菜单面高度（0 = 还没量过 → 按"放得下"处理）。
	PanelHeight float32
	// OnPanel 把这一帧声明的菜单面 Box 交给调用方，供它在 AfterDraw 里量高度。
	OnPanel func(ui.Box)
}

// DrawEntryMoreMenu 画一层区域 More 菜单面并派发 `opts.Slot`，返回是否画了。
//
// 不画的三条（都不产生任何像素）：
//
//	!opts.Open            菜单没展开
//	opts.Dispatch == nil  壳这一帧没有 drawSlot（最小装配/单测：没人能画行）
//	宽度算不出来（窗口太窄到 288 也放不下）
func DrawEntryMoreMenu(f ui.C, env *pluginapi.Env, opts EntryMenuOptions) bool {
	if f == nil || !opts.Open || opts.Dispatch == nil || opts.Slot == "" {
		return false
	}
	t := opts.Theme
	if t == nil && env != nil {
		t = env.Theme
	}
	if t == nil {
		return false
	}
	var width float32
	if opts.AnchorMinWidth {
		// 右栏 guide：官方 `width:max-content;max-width:min(480px,…)` + `minWidthFromAnchor`
		// ⇒ 这一档的实宽就是锚点宽（见 `entryMenuAnchorWidth`）。
		declared := opts.MenuWidth
		if declared <= 0 {
			declared = GuideMoreMenuMax
		}
		width = entryMenuAnchorWidth(declared, opts.Anchor.Width, opts.Bounds.Width)
	} else {
		desired := opts.MenuWidth
		if desired <= 0 {
			desired = EntryMoreMenuWidth // 左栏/会话头两处仍是官方 288
		}
		width = entryMenuWidthFor(desired, opts.Bounds.Width)
	}
	if width <= 0 {
		return false
	}

	layer := ui.Overlay(f, ui.OverlaySpec{Band: ui.LayerPaneChrome, Bounds: opts.Bounds})
	layer.Children(func() {
		// 点菜单面之外 = 官方 Menu 的 onClose。遮罩先声明，菜单面后声明
		// （同层内后声明的赢），所以点菜单面本身不会落到遮罩上。
		if opts.Dismiss != nil && env != nil && env.Tree != nil {
			scrim := ui.Row(f).FillWidth().FillHeight().
				Hit(env.Tree.ID(env.Feat, "entry-more-scrim"), "关闭菜单")
			if scrim.Clicked() {
				opts.Dismiss()
			}
		}
		// 横向：右缘贴锚点右缘，夹在窗口内（官方 portal 菜单的贴边语义）。
		right := opts.Bounds.Width - (opts.Anchor.X + opts.Anchor.Width)
		if right < EntryMoreMenuEdgeMargin {
			right = EntryMoreMenuEdgeMargin
		}
		if right+width > opts.Bounds.Width-EntryMoreMenuEdgeMargin {
			right = opts.Bounds.Width - EntryMoreMenuEdgeMargin - width
		}
		if right < EntryMoreMenuEdgeMargin {
			right = EntryMoreMenuEdgeMargin
		}
		menu := ui.Column(f).Absolute().Width(width).Right(right)
		// 纵向：默认贴锚点下方 gap 处；放不下翻到上方（与模型菜单同一判据）。
		if PlaceAbove(opts.Anchor, opts.PanelHeight, EntryMoreMenuAnchorGap,
			opts.Bounds.Height, EntryMoreMenuEdgeMargin) == PopupBelow {
			menu = menu.Top(opts.Anchor.Y + opts.Anchor.Height + EntryMoreMenuAnchorGap)
		} else {
			menu = menu.Bottom(opts.Bounds.Height - opts.Anchor.Y + EntryMoreMenuAnchorGap)
		}
		menu.MaxHeight(opts.Bounds.Height - 2*EntryMoreMenuEdgeMargin)
		menu.Radius(t.Radii.MD).Background(t.BgLayer3).Border(1, t.BorderL1).
			Shadow(gfx.ShadowMD).Pad(EntryMoreMenuPad).Gap(0).Children(func() {
			// owner props 只在本帧这一层里有效：dispatch 前挂上，dispatch 后恢复
			// （与 `Env.ViewportOffset` 同一约定：写者负责恢复）。
			var saved *pluginapi.EntryMenuProps
			if env != nil {
				saved = env.EntryMenu
				env.EntryMenu = &pluginapi.EntryMenuProps{
					Region:    opts.Region,
					Entries:   opts.Entries,
					Open:      opts.OpenEntry,
					SetPinned: opts.SetPinned,
					Dismiss:   opts.Dismiss,
				}
			}
			opts.Dispatch(opts.Slot)
			// 派发结束立刻恢复：占用方的行只在 dispatch 期间看到 props，
			// 之后同一帧的其它画（正文/状态栏）不该还能读到它。
			if env != nil {
				env.EntryMenu = saved
			}
		})
		if opts.OnPanel != nil {
			opts.OnPanel(menu)
		}
	})
	return true
}
