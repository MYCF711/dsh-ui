package uikernel

// view_guide_more.go —— 右栏向导页的 **More 卡**（官方 `GuideMoreMenu`，rc.5）。
//
// # 官方形状（逐行）
//
//	OFF:dsh-client-ui-sidebar-right/src/client/tabs/guide/GuideBody.tsx:99-117
//	  <Menu open={open} onClose portal autoFocus minWidthFromAnchor align="end"
//	        className={css.moreAnchor} listClassName={css.moreMenu}
//	        anchor={<button type="button" className={css.entry} aria-label={label}
//	                        aria-haspopup="menu" aria-expanded={open}
//	                        onClick={() => setOpen(v => !v)}>
//	                  <span className={css.entryIcon}><GuideMoreIcon /></span>
//	                  <span className={css.entryText}>
//	                    <span className={css.entryTitle}>{label}</span>
//	                  </span>
//	                </button>}>
//	    {renderSlot('sidebar.right.tab.guide.actions', { entries, onOpen, onPinnedChange, onDismiss })}
//	  </Menu>
//	OFF:.../GuideBody.tsx:160-172  这张卡**只在 `hasEntryActions` 时出现**（座位有占用方），
//	  entries 是 `entries.filter(kind !== FIXED_GUIDE_KIND)`、id 是 `guideEntryId`
//	  （`${kind}:${id}`，guide-pins.ts:15-17）、`pinned: !unpinned.includes(id)`。
//	OFF:.../GuideBody.module.css:121-128
//	  `.entryCell{width:380px;max-width:100%}`（与入口卡同一格）
//	  `.moreAnchor{display:flex;width:380px;max-width:100%}`（锚点包一层，宽同卡片）
//	  `.moreAnchor .entry[aria-expanded='true']{background:interactive-bg-hover}`（展开时底色）
//	  `.moreMenu{width:max-content;max-width:min(480px,calc(100vw - 24px))}`
//	卡片本体就是入口卡那枚胶囊（`.entry`：380/56/14·20/gap14/layer-1/0.5 border-l3/xl），
//	图标格 `.entryIcon{width:26px;height:26px}` 里画 **22×22** 的 `GuideMoreIcon`
//	（`GuideMoreIcon.tsx:8`：viewBox 22、stroke 1.38181、`stroke="#A6B2C7"` 写死在素材里），
//	文案 `t('tab.guide.more')`（sidebar-right 的 zh 字典 = 「更多工具」，locales.ts:47）。
//
// # 本仓的形状（以及为什么状态在壳里）
//
// 官方的卡片与菜单都长在向导页的体里，所以它们的宿主就是**画向导页的那一层**——
// 本仓是骨架自己（`view_guide.go` 的 `DrawRightGuideMore`），不是插件。
// `open` 是官方那个 `useState`，本仓落在 `ShellOptions.GuideMore` 指向的
// `GuideMoreState`（与芯片就地改名的 `RightTabRenameState` 同一手法：壳持有、跨帧保存）。
//
// 锚点与菜单面高度是"上一帧量到的"：viva 先声明整棵树、再 `End()` 解算，所以
// Draw 回调里 `Box.Rect()` 还是零值（`plugins/dsconversation/official_transcript_wiring.go:118`
// 同一句）。壳在 `m.view.Draw(...)` 返回之后调一次 `AfterDraw()`（与插件
// `AfterDraw` 同一时机：布局已定），把本帧的卡片矩形 / 菜单面高度留给下一帧。
//
// 菜单面**不新画一份**：走的还是 `internal/uikernel/entry_menu.go` 的
// `DrawEntryMoreMenu`（三个区域共用的那一层：面 + 遮罩关闭 + owner props 生命周期）。
// 只把宽度规则按官方 guide 那处传进去：`MenuWidth=480`（`max-width:min(480px,…)`）
// 且 `AnchorMinWidth=true`（`minWidthFromAnchor`）。
//
// # 登记在案的差异
//
//  1. **`onPinnedChange` 不给**（`SetPinned: nil`）：官方 `guide-pins.ts` 的
//     `setPinned` 传进了这层菜单，但 rc.5 的 `EntryMenuContent` 在 `region==='rightbar'`
//     时 `moreEntries` 恒空（`EntryMenuContent.tsx:38`）⇒ **没有任何行会调用它**，
//     也就是说官方这个入口点今天没有可用者。本仓的固定态服务（`ui.entryPins`）在
//     壳里也没有引用，所以这里不编一个"点了会改但没人看得见"的回调。
//  2. **点外面关闭、Esc 不关**：`DrawEntryMoreMenu` 只有整窗遮罩那一条关闭路径，
//     左栏与会话头两处同样如此（登记在 `docs/official-parity.md` #17）。官方的
//     primitives `Menu` 会吃 Esc，本仓三处一致地不吃——补齐属于菜单面的公共行为，
//     不在这张卡上单独造。

import (
	"github.com/MYCF711/dsh-ui/internal/entrypins"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方 guide More 卡的读数（出处见文件头）。
const (
	// GuideMoreTitle 是 `t('tab.guide.more')` 的 zh（sidebar-right 的 locales.ts:47）。
	GuideMoreTitle = "更多工具"
	// GuideMoreMenuMax 是 `.moreMenu{max-width:min(480px,…)}`（GuideBody.module.css:128）。
	GuideMoreMenuMax float32 = 480
	// guideMoreStroke 是 `GuideMoreIcon` 里写死的 `stroke="#A6B2C7"`
	// （GuideMoreIcon.tsx:9）。素材自带色，不跟主题令牌（同一档做法见
	// `plugins/dsterminal/guide.go` 的 `PluginArtworkTerminal`）。
	guideMoreStroke uint32 = 0xA6B2C7
)

// GuideMoreState 是右栏向导页 More 卡的跨帧状态（官方 `GuideMoreMenu` 的 `useState`
// 加上"上一帧量到的"两个矩形）。
//
// 只由绘制线程读写（与 `RightTabRenameState` 同一约定），零值可用。
type GuideMoreState struct {
	// Open 是展开态（官方 `open`）。
	Open bool
	// Anchor 是卡片**上一帧**的窗口坐标矩形（菜单贴它开）。
	Anchor gfx.Rect
	// PanelHeight 是菜单面**上一帧**的高度（0 = 还没量过 → 按"放得下"处理）。
	PanelHeight float32
	// card / cardDrawn 是本帧声明的卡片盒；AfterDraw 量它的矩形给下一帧。
	card      ui.Box
	cardDrawn bool
	// panel / panelDrawn 同上，量菜单面高度。
	panel      ui.Box
	panelDrawn bool
}

// Toggle 切展开态（官方 `onClick={() => setOpen(value => !value)}`）。
func (s *GuideMoreState) Toggle() {
	if s == nil {
		return
	}
	s.Open = !s.Open
}

// Close 关掉菜单（官方 `onClose` / `onDismiss`）。
func (s *GuideMoreState) Close() {
	if s == nil {
		return
	}
	s.Open = false
}

// AfterDraw 把本帧卡片 / 菜单面的矩形留给下一帧。
//
// 必须在整帧 Draw 返回之后调（布局已定，`Box.Rect()` 才有值）。卡片这一帧**没画**
// （向导页不在、右栏关着、或者座位没占用方）时按官方 unmount 处理：展开态归零，
// 不留一个"下次出现就已经展开"的幽灵菜单。
func (s *GuideMoreState) AfterDraw() {
	if s == nil {
		return
	}
	if !s.cardDrawn {
		s.Open = false
		s.Anchor = gfx.Rect{}
	} else if rect := s.card.Rect(); rect.Width > 0 && rect.Height > 0 {
		s.Anchor = rect
	}
	if s.panelDrawn {
		if rect := s.panel.Rect(); rect.Height > 0 {
			s.PanelHeight = rect.Height
		}
	}
	s.cardDrawn, s.panelDrawn = false, false
}

// GuideMoreOptions 是向导页画 More 卡要的那一帧输入。
type GuideMoreOptions struct {
	// State 是跨帧状态；nil = 这张卡不画（最小装配 / 单测里没人持有它）。
	State *GuideMoreState
	// Dispatch 是内核的 drawSlot（骨架的 `dispatch`）；nil = 菜单排不出行，
	// 这时卡片照画、点了不展开（与左栏/会话头「菜单面无 drawSlot → 只画入口」同一条边界）。
	Dispatch func(pluginapi.Slot)
	// HasActions 是官方 `hasEntryActions`：`sidebar.right.tab.guide.actions`
	// 座位有占用方时才有这张卡（GuideBody.tsx:160）。
	HasActions bool
}

// drawGuideMoreCard 在向导页那一列里画 More 卡，返回是否画了。
//
// 官方这张卡是 `.moreAnchor` 里那枚 `.entry` 胶囊：与入口卡同一格几何（380 /
// min-height 56 / padding 14·20 / gap 14 / layer-1 / 0.5 border-l3 / radius xl），
// 图标格 26×26 里画 22×22 的 `GuideMoreIcon`，文案只有一行标题。
func drawGuideMoreCard(f ui.C, env *pluginapi.Env, opts GuideMoreOptions) bool {
	if f == nil || env == nil || env.Theme == nil || opts.State == nil || !opts.HasActions {
		return false
	}
	t := env.Theme
	state := opts.State
	// `.moreAnchor .entry[aria-expanded='true']{background:interactive-bg-hover}`
	// （GuideBody.module.css:126）：展开时底色恒亮，不看指针。
	background := t.BgLayer1
	if state.Open {
		background = t.Hover
	}
	var id gfx.ID
	if env.Tree != nil {
		id = env.Tree.ID(env.Feat, "guide-more")
	}
	var card ui.Box
	// `.entryCell{width:380px;max-width:100%}`（:121）：撑满再夹到 380。
	ui.Column(f).FillWidth().MaxWidth(GuideEntryCellWidth).Shrink(0).Children(func() {
		row := ui.Row(f).FillWidth().MinHeight(GuideEntryMinHeight).
			PadSides(GuideEntryPadY, GuideEntryPadX, GuideEntryPadY, GuideEntryPadX).
			Gap(GuideEntryGap).Items(ui.Center).
			Background(background).Border(GuideEntryBorder, t.BorderL3).Radius(t.Radii.XL).
			// `@media (hover:hover){.entry:hover{background:interactive-bg-hover}}`（:61-65）。
			Hover(ui.Look{}.Background(t.Hover)).
			// 官方 aria-label 就是文案（`aria-label={label}`，GuideBody.tsx:107）。
			Hit(id, GuideMoreTitle)
		row.Children(func() {
			// `.entryIcon{width:26px;height:26px}`（:70-78）：固定 26 格，图标本身 22。
			ui.Row(f).Width(GuideEntryIconBox).Height(GuideEntryIconBox).Shrink(0).
				Items(ui.Center).Justify(ui.Center).Children(func() {
				ui.Icon(f, guideMoreIcon()).Size(GuideEntryIconBareSize, GuideEntryIconBareSize).
					Color(gfx.RGB24(guideMoreStroke))
			})
			// `.entryText` 只有一行标题（官方这张卡没有 description）。
			ui.Row(f).Grow(1).Shrink(1).Children(func() {
				ui.Text(f, GuideMoreTitle).FontSize(GuideEntryTitleSize).
					LineHeight(GuideEntryTitleLine).MaxLines(1).Ellipsis().Color(t.LabelPrimary)
			})
		})
		card = row
	})
	state.card, state.cardDrawn = card, true
	if card.Clicked() {
		state.Toggle()
	}
	return true
}

// drawGuideMoreMenu 画贴在 More 卡上的那层菜单面，并把
// `sidebar.right.tab.guide.actions` 派发进去。
//
// 官方同一张 `Menu` 的 `listClassName={css.moreMenu}`（width:max-content /
// max-width:min(480px, 100vw-24px)）与 `minWidthFromAnchor` 都转成
// `EntryMenuOptions` 的两个字段；面本身、遮罩关闭、owner props 生命周期仍是
// `DrawEntryMoreMenu` 的（三个区域共用一份）。
func drawGuideMoreMenu(f ui.C, env *pluginapi.Env, entries []GuideEntry, opts GuideMoreOptions) {
	if f == nil || env == nil || opts.State == nil || !opts.State.Open || opts.Dispatch == nil {
		return
	}
	state := opts.State
	DrawEntryMoreMenu(f, env, EntryMenuOptions{
		Theme:          env.Theme,
		Anchor:         state.Anchor,
		Bounds:         env.Bounds,
		Open:           true,
		Slot:           OffRightGuideActions,
		Region:         pluginapi.EntryRegionRightbar,
		Entries:        guideMoreEntries(entries),
		OpenEntry:      func(id string) { guideMoreOpen(env, entries, id) },
		SetPinned:      nil, // 见文件头「登记在案的差异」第 1 条
		Dismiss:        state.Close,
		Dispatch:       opts.Dispatch,
		MenuWidth:      GuideMoreMenuMax,
		AnchorMinWidth: true,
		PanelHeight:    state.PanelHeight,
		OnPanel: func(box ui.Box) {
			state.panel, state.panelDrawn = box, true
		},
	})
}

// guideMoreEntries 是官方传给那层菜单的 entries
// （GuideBody.tsx:163-164）：去掉固定 kind，id 是 `kind:id`。
//
// `Pinned` 恒真：官方默认值就是「全部固定」（`createGuidePins` 的初值空表），
// 而 rc.5 里也没有任何一行会调 `onPinnedChange` 改它（见文件头差异 1）。
func guideMoreEntries(entries []GuideEntry) []pluginapi.EntryMenuEntry {
	out := make([]pluginapi.EntryMenuEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Kind == entrypins.FixedGuideKind {
			continue
		}
		out = append(out, pluginapi.EntryMenuEntry{
			ID: guideEntryID(entry), Label: entry.Title, Pinned: true,
		})
	}
	return out
}

// guideEntryID 是官方 `guideEntryId`（guide-pins.ts:15-17）：`${kind}:${id}`。
func guideEntryID(entry GuideEntry) string {
	id := entry.ID
	if id == "" {
		id = entry.Kind
	}
	return entry.Kind + ":" + id
}

// guideMoreOpen 是官方 onOpen（GuideBody.tsx:165-168 + `open(kind)` :138-141）：
// 按 id 找回那张卡，打开它声明的类型。官方还顺手 `tab.actions.close()` 关掉
// 自己这个 guide 页；本仓的向导只在右栏没有 tab 时出现，所以只有「打开」这一半
// （与 `drawGuideFallbackEntry` 同一句话）。
func guideMoreOpen(env *pluginapi.Env, entries []GuideEntry, id string) {
	if env == nil {
		return
	}
	for _, entry := range entries {
		if guideEntryID(entry) != id {
			continue
		}
		if env.Act.OpenRightTab != nil {
			env.Act.OpenRightTab(entry.Kind)
		}
		return
	}
}

// guideMoreIcon 是官方 `GuideMoreIcon`（OFF:GuideMoreIcon.tsx）的逐条转写：
// viewBox 22、stroke 1.38181、三个圆角方块（同一枚方块的三个平移）+ 一枚加号。
//
// 三个圆角方块在素材里写成**同一条** `d` 的三份平移（第二份 dy 7.93948、
// 第三份 dx 7.94047），所以这里也只写一份几何再按官方那份偏移复制：
//
//	M5.45873 5.04102 H7.90157 C8.57615 5.04102 9.123 5.58787 9.123 6.26244
//	V8.70528 C9.123 9.37986 8.57615 9.92671 7.90157 9.92671 H5.45873
//	C4.78415 9.92671 4.2373 9.37986 4.2373 8.70528 V6.26244
//	C4.2373 5.58787 4.78415 5.04102 5.45873 5.04102 Z
//
// 加号是第 4 条 path（`M15.0692 3.86523V6.91879M15.0692 6.91879V9.97235
// M15.0692 6.91879H12.0156M15.0692 6.91879H18.1227`）：一条竖线两条横线，
// 全是直线，原样转写。
func guideMoreIcon() gfx.Icon {
	commands := make([]gfx.PathCommand, 0, 34)
	for _, offset := range [][2]float32{
		{0, 0},
		{0, 7.93948},
		{7.94047, 7.93948},
	} {
		commands = append(commands, guideMoreSquare(offset[0], offset[1])...)
	}
	commands = append(commands,
		gfx.MoveTo(15.0692, 3.86523), gfx.LineTo(15.0692, 6.91879),
		gfx.MoveTo(15.0692, 6.91879), gfx.LineTo(15.0692, 9.97235),
		gfx.MoveTo(15.0692, 6.91879), gfx.LineTo(12.0156, 6.91879),
		gfx.MoveTo(15.0692, 6.91879), gfx.LineTo(18.1227, 6.91879),
	)
	return gfx.Icon{ViewBox: 22, Stroke: 1.38181, Commands: commands}
}

// guideMoreSquare 是官方那枚圆角方块（四个角各一段三次贝塞尔）平移 (dx, dy)。
func guideMoreSquare(dx, dy float32) []gfx.PathCommand {
	return []gfx.PathCommand{
		gfx.MoveTo(5.45873+dx, 5.04102+dy), gfx.LineTo(7.90157+dx, 5.04102+dy),
		gfx.CubicTo(8.57615+dx, 5.04102+dy, 9.123+dx, 5.58787+dy, 9.123+dx, 6.26244+dy),
		gfx.LineTo(9.123+dx, 8.70528+dy),
		gfx.CubicTo(9.123+dx, 9.37986+dy, 8.57615+dx, 9.92671+dy, 7.90157+dx, 9.92671+dy),
		gfx.LineTo(5.45873+dx, 9.92671+dy),
		gfx.CubicTo(4.78415+dx, 9.92671+dy, 4.2373+dx, 9.37986+dy, 4.2373+dx, 8.70528+dy),
		gfx.LineTo(4.2373+dx, 6.26244+dy),
		gfx.CubicTo(4.2373+dx, 5.58787+dy, 4.78415+dx, 5.04102+dy, 5.45873+dx, 5.04102+dy),
		gfx.ClosePath(),
	}
}
