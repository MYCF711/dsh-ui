// view_guide.go —— 右栏向导页（guide）的**入口卡**名录与绘制（rc.2）。
//
// # 官方事实
//
// 官方 ui-sidebar-right 自己注册了一个 `kind:'guide'` 的 tab 类型
// （OFF:src/client/tabs/guide/definition.ts:20-27，`id` = 包名 + "/guide"），它的体是
// 一个 **chain** 座位（`sidebar.right.tab.guide`），页面里逐张入口卡再各走一个
// **keyed** 座位（`sidebar.right.tab.guide.entry`，key = 提供方 id）：
//
//	OFF:src/client/tabs/guide/GuideBody.tsx:142-176
//	  visibleEntries.map(entry => renderSlot('sidebar.right.tab.guide.entry',
//	      { entryId, kind, title, description? },
//	      { entryKey: entry.providerId, fallback: <EntryBox …/> }))
//
// 也就是说：**每一张卡片是一个 keyed 座位**，没有占用方时由向导页自己画官方那枚
// 胶囊（`EntryBox`）。本文件就是那条「没有占用方时的胶囊」加上名录投影。
//
// # 本仓的边界（写清楚，不冒充）
//
// 已落地：入口卡名录投影、每卡 keyed 派发（带 owner props）、官方胶囊 fallback、
// 官方列的几何（GuideBody.module.css 的 `.guide` / `.entryCell` / `.entry` 三块）。
//
// 未落地（仍在 docs/official-parity.md 的 #53 行）：页首那枚 56px 罗盘（`.hero`，
// `GuideBody.module.css:32-36`）、More 卡片（`.moreAnchor`，需要 `sidebar.right.tab.guide.actions`
// 的行实现）、卡片的固定/取消固定（`dsh.entryPins`）。它们不是入口卡本身的一部分。
package uikernel

import (
	"sort"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 官方几何（GuideBody.module.css / tab-registry）────────────────────
const (
	// GuideColumnPad 是 `.guide{padding:0 24px}`（GuideBody.module.css:7-17）。
	GuideColumnPad float32 = 24
	// GuideEntryGap 是 `.guide{gap:14px}`，同时也是 `.entry{gap:14px}`
	// （GuideBody.module.css:11、:45）。
	GuideEntryGap float32 = 14
	// GuideEntryCellWidth 是 `.entryCell{width:380px;max-width:100%}`（:121）。
	GuideEntryCellWidth float32 = 380
	// GuideEntryMinHeight 是 `.entry{min-height:56px}`（:50）。
	GuideEntryMinHeight float32 = 56
	// GuideEntryPadY / PadX 是 `.entry{padding:14px 20px}`（:51）。
	GuideEntryPadY float32 = 14
	GuideEntryPadX float32 = 20
	// GuideEntryBorder 是 `.entry{border:.5px solid border-l3}`（:56）。
	GuideEntryBorder float32 = 0.5
	// GuideEntryIconBox 是 `.entryIcon{width:26px;height:26px}`（:70-78）。
	GuideEntryIconBox float32 = 26
	// GuideEntryIconBareSize 是 `icon` 在**没有描述**时的显示边长 22，有描述时 26
	// （GuideBody.tsx:81-84 的 `size={description === undefined ? 22 : 26}`）。
	GuideEntryIconBareSize float32 = 22
	// GuideEntryTextGap 是 `.entryText{gap:3px}`（:88-94）。
	GuideEntryTextGap float32 = 3
	// GuideEntryTitleSize / Line 是 `.entryTitle{font-size:14px;line-height:1.4}`（:96-102）。
	GuideEntryTitleSize float32 = 14
	GuideEntryTitleLine float32 = 14 * 1.4
	// GuideEntryDescSize / Line 是 `.entryDescription{font-size:11px;line-height:1.4}`（:105-112）。
	GuideEntryDescSize float32 = 11
	GuideEntryDescLine float32 = 11 * 1.4
)

// GuideEntry 是向导页上的一张入口卡：官方 `SidebarRightGuideBox`
// （OFF:dsh-client-ui-sidebar-right/src/client/tab-registry.ts:88-95）
// —— `SidebarRightGuideEntry` 再加 `providerId` 与 `kind` 两个字段。
type GuideEntry struct {
	pluginapi.TabGuide
	// ProviderID 是派发用的实现身份（官方 `providerId`，也是 keyed 座位的 key）。
	//
	// 官方那个值是 `SidebarRightTabDefinition.id`（包名）；本仓取类型注册的
	// `Painter.ID`——同一层「实现身份」，插件内唯一。空则退回 `Kind`。
	ProviderID string
	// Kind 是选中这张卡要打开的 tab 类型（官方 `kind`）。
	Kind string
}

// GuideEntries 把内核里的 tab 类型注册投影成向导页入口卡名录。
//
// 只有 `sidebar.right.pane.tab` 上**声明了 `Painter.Guide`** 的类型才上向导页
// （官方 `SidebarRightTabDefinition.guide`，tab-registry.ts:66-135）。
// 排序 = 官方 `guide[].order` 升序，同序按注册顺序（`sort.SliceStable`）。
func GuideEntries(k *Kernel) []GuideEntry {
	if k == nil {
		return nil
	}
	var out []GuideEntry
	for _, p := range k.painters {
		if p.Slot != pluginapi.SlotRightPaneTab || len(p.Guide) == 0 {
			continue
		}
		// 官方 `providerId` 是 `SidebarRightTabDefinition.id`（允许 extension 顶掉
		// builtin）；本仓还没有 priority 覆盖机制（docs/official-view-tabs-evidence-rc2.md
		// 的 G2），类型的实现身份就是它的 kind，所以 keyed 座位的 key 也取 kind。
		kind := p.Key
		provider := kind
		if provider == "" {
			provider = p.ID
		}
		for _, guide := range p.Guide {
			entry := GuideEntry{TabGuide: guide, ProviderID: provider, Kind: kind}
			if entry.ID == "" {
				entry.ID = kind
			}
			out = append(out, entry)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

// DrawRightGuide 画向导页的入口卡列（官方 `.guide` 那个居中竖列）。
//
// 每张卡按官方 `renderSlot('sidebar.right.tab.guide.entry', …, {entryKey: providerId})`
// 派发：**只有这个 provider 注册了卡片**才派发，否则画官方胶囊 fallback
// （`fallback: <EntryBox …/>`，GuideBody.tsx:150-157）。
//
// 选中一张卡 = 打开它声明的类型（官方 GuideBody.tsx:138-141 的 `open(kind)`：
// `tab.actions.openTab(kind, {replaceTab:true})` 然后 `tab.actions.close()`）。
// 本仓的向导只在右栏没有 tab 时出现，所以只有「打开」这一半，没有要关掉的自身 tab。
func DrawRightGuide(f ui.C, env *pluginapi.Env, entries []GuideEntry) {
	if env == nil || env.Theme == nil || len(entries) == 0 {
		return
	}
	t := env.Theme
	ui.Column(f).FillWidth().FillHeight().PadX(GuideColumnPad).
		Items(ui.Center).Justify(ui.Center).Gap(GuideEntryGap).Children(func() {
		for _, entry := range entries {
			entry := entry
			// `.entryCell{width:380px;max-width:100%}`：撑满再夹到 380。
			cell := ui.Column(f).FillWidth().MaxWidth(GuideEntryCellWidth).Shrink(0)
			cell.Children(func() {
				if entry.ProviderID != "" && env.HasKey != nil &&
					env.HasKey(pluginapi.SlotRightGuideEntry, entry.ProviderID) && env.DrawKeyed != nil {
					props := &pluginapi.GuideEntryProps{
						EntryID: entry.ID, Kind: entry.Kind,
						Title: entry.Title, Description: entry.Description,
					}
					previous := env.GuideEntry
					env.GuideEntry = props
					env.DrawKeyed(f, pluginapi.SlotRightGuideEntry, entry.ProviderID)
					env.GuideEntry = previous
					return
				}
				drawGuideFallbackEntry(f, env, t, entry)
			})
		}
	})
}

// drawGuideFallbackEntry 画官方 `EntryBox` 那枚胶囊（GuideBody.tsx:64-92 + `.entry` 几何）。
//
// 没有占用方的入口卡走这里；有占用方时卡片长什么样由占用方决定。
func drawGuideFallbackEntry(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, entry GuideEntry) {
	var id gfx.ID
	if env.Tree != nil {
		id = env.Tree.ID(env.Feat, "guide-entry-"+entry.Kind+"-"+entry.ID)
	}
	described := entry.Description != ""
	box := ui.Row(f).FillWidth().MinHeight(GuideEntryMinHeight).Shrink(0).
		PadSides(GuideEntryPadY, GuideEntryPadX, GuideEntryPadY, GuideEntryPadX).
		Gap(GuideEntryGap).Items(ui.Center).
		Background(t.BgLayer1).Border(GuideEntryBorder, t.BorderL3).Radius(t.Radii.XL).
		// `@media (hover:hover){.entry:hover{background:interactive-bg-hover}}`（:61-65）。
		Hover(ui.Look{}.Background(t.Hover)).
		Hit(id, entry.Title)
	box.Children(func() {
		// 官方的 `.placeholderInk` 是 `CubeGlyph` 占位（GuideBody.tsx:71,83）。本仓没有
		// 登记那枚方块素材，所以**宁可不画图标也不手画一个**；有图标时盒子仍是 26×26，
		// 保证每张卡的文字起点对齐（`.entryIcon` 的注释就是为这一点）。
		if entry.IconImage.ViewBox > 0 {
			size := GuideEntryIconBareSize
			if described {
				size = GuideEntryIconBox
			}
			if entry.IconSize > 0 {
				size = entry.IconSize
			}
			ui.Row(f).Width(GuideEntryIconBox).Height(GuideEntryIconBox).Shrink(0).
				Items(ui.Center).Justify(ui.Center).Children(func() {
				ui.Icon(f, entry.IconImage).Size(size, size).Color(t.LabelSecondary)
			})
		}
		ui.Column(f).Grow(1).Shrink(1).Gap(GuideEntryTextGap).Children(func() {
			ui.Text(f, entry.Title).FontSize(GuideEntryTitleSize).
				LineHeight(GuideEntryTitleLine).MaxLines(1).Ellipsis().Color(t.LabelPrimary)
			if described {
				ui.Text(f, entry.Description).FontSize(GuideEntryDescSize).
					LineHeight(GuideEntryDescLine).MaxLines(1).Ellipsis().Color(t.LabelTertiary)
			}
		})
	})
	if box.Clicked() && env.Act.OpenRightTab != nil {
		env.Act.OpenRightTab(entry.Kind)
	}
}
