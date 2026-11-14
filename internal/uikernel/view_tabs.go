// view_tabs.go —— **多视图座位**的注册表与选择器绘制（rc.2）。
//
// rc.2 的界面上有两处「一个位置、多个视图轮流上」，官方各自是一个座位：
//
//	conversation.view        会话主区的视图（ui-conversation 的 list 槽）
//	sidebar.right.pane.tab   右栏面板的 tab（ui-sidebar-right 的 keyed 槽）
//
// 两处的**机制是同一套**，本仓因此只做一遍：
//
//	① 注册  插件在 `Panels()` 里往那个槽挂画，带 Key / Label / Order
//	② 投影  壳读内核的 painter 表，得到一张按 (Order, 装配顺序) 排好、同 key 去重的名录
//	③ 激活  名录 + 存储的偏好 → 本帧该画哪一个（官方的 resolveActiveView / defaultSeed）
//	④ 派发  只画命中的那一份（`Kernel.DrawSlotKeyed`），不再"全画一遍"
//	⑤ 选择  芯片行由壳画，点击走 `Actions.SelectView` / `SelectRightTab`
//
// 为什么机制必须在壳里而不是某个插件里：官方那两处座位分别由 ui-conversation
// 与 ui-sidebar-right 声明，**声明者同时是派发者**；本仓的对应物就是壳
// （official 档是 internal/uikernel 的骨架，legacy 档是 dsshell 三栏 +
// dstopbar 的 header）。放进插件会让「谁先装配谁说了算」，那正是要避免的。
package uikernel

import (
	"sort"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 官方锚定的几何 ──────────────────────────────────────────────────────
//
// 右栏 tab 芯片的全部数值来自官方 **dockkit** 的编译产物：
// `@deepseek-ai/dsh-web-frontend/dist/assets/index-CSAdOBx6.css` 里 `_123xe_` 模块
// （dockkit 的源码包不在随包 `src/**` 里，只有这一份编译后的 CSS；
// 会话侧 CSS 的注释也独立佐证了 `.tabStrip` 38px 这个数：
// `dsh-client-ui-conversation/src/client/skeleton/ConversationRoot.module.css:17-23`）。
//
// 下面每个常量后面都写了原始声明，改数值前先回去核那一行。
const (
	// RightTabStripRowHeight 是 `.tabStrip{height:28px}`。
	RightTabStripRowHeight float32 = 28
	// RightTabStripPadTop 是 `.tabStrip{padding:10px 6px 0 …}` 的上内边距；
	// 28 + 10 = 38，就是会话侧注释里说的 `.tabStrip 38px`。
	RightTabStripPadTop float32 = 10
	// RightTabStripGap 是 `.tabStrip{gap:4px}`。
	RightTabStripGap float32 = 4
	// RightTabStripPadLeft / Right 是 `.tabStrip{padding:… 6px 0 var(--dsh-dockkit-strip-inline-start,10px)}`。
	RightTabStripPadLeft  float32 = 10
	RightTabStripPadRight float32 = 6

	// RightTabChipMinWidth / MaxWidth 是 `.tab{min-width:80px;max-width:170px}`。
	RightTabChipMinWidth float32 = 80
	RightTabChipMaxWidth float32 = 170
	// RightTabChipHeight 是 `.tab{height:28px}`。
	RightTabChipHeight float32 = 28
	// RightTabChipPadX 是 `.tab{padding:0 10px}`。
	RightTabChipPadX float32 = 10
	// RightTabChipFontSize 是 `.tab{font-size:var(--dsh-content-font-size-secondary,13px)}`。
	RightTabChipFontSize float32 = 13
	// RightTabChipGap 是 `.tabTitle{gap:5px}`（图标与文字之间）。
	RightTabChipGap float32 = 5

	// RightTabSepWidth 是 `.slot{width:10px}`：两枚芯片之间那条分隔轨。
	RightTabSepWidth float32 = 10
	// RightTabSepRuleH / W 是 `.slot:before{width:.5px;height:14px}`。
	RightTabSepRuleH float32 = 14
	RightTabSepRuleW float32 = 0.5

	// ViewTabRowHeight 是官方 `.viewTabs{min-height:25px}`。
	ViewTabRowHeight float32 = ViewTabsMinH
	// ViewTabGap 是 `.viewTabs{gap:32px}`。
	ViewTabGap float32 = ViewTabsGap
	// ViewTabActiveBarHeight 是 `.tab::after{height:2px}`。
	ViewTabActiveBarHeight float32 = TabBarHeight
	// ViewTabFontSize / LineHeight 是 `.tab{font-size:13px;line-height:16px}`。
	ViewTabFontSize   float32 = TabFontSize
	ViewTabLineHeight float32 = TabLineHeight
)

// ── 注册表投影 ──────────────────────────────────────────────────────────

// DefaultViewKey 是官方 `DEFAULT_VIEW_ID`：没有可用偏好时选中的视图。
//
// OFF:dsh-client-ui-conversation/src/client/view-selection.ts:4
const DefaultViewKey = pluginapi.ViewChat

// ViewTabs 把内核里的 `conversation.view` 占用方投影成会话视图名录。
//
// 同 key 的多块画（本仓 chat 视图由 ds-conversation 与 ds-agent-team 各画一段）
// 合成**一条**名录项；Label 取第一条非空的（官方 `ViewTab` 每条注册一个 label，
// 本仓把它放宽成"同 key 共用一个 label"）。
func ViewTabs(k *Kernel) []pluginapi.Tab {
	return tabRoster(k, pluginapi.SlotConvView)
}

// RightTabs 把内核里的 `sidebar.right.pane.tab` 占用方投影成右栏 tab 名录。
func RightTabs(k *Kernel) []pluginapi.Tab {
	return tabRoster(k, pluginapi.SlotRightPaneTab)
}

func tabRoster(k *Kernel, slot pluginapi.Slot) []pluginapi.Tab {
	if k == nil {
		return nil
	}
	seen := map[string]int{} // key -> out 里的下标
	var out []pluginapi.Tab
	for _, p := range k.painters { // 装配顺序 = 官方「同 order 按注册顺序」的次序
		if p.Slot != slot || p.Key == "" {
			continue
		}
		if at, ok := seen[p.Key]; ok {
			if out[at].Label == "" {
				out[at].Label = p.Label
			}
			if out[at].Icon == "" {
				out[at].Icon = p.Icon
			}
			continue
		}
		seen[p.Key] = len(out)
		out = append(out, pluginapi.Tab{Key: p.Key, Label: p.Label, Icon: p.Icon, Order: p.Order})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

// ResolveActiveView 是官方 `resolveActiveView` 的逐字移植。
//
// OFF:dsh-client-ui-conversation/src/client/view-selection.ts:15-21
//
//	存着的偏好命中名录 → 用它
//	否则                → 找 `chat`
//	再没有              → 没有可画的视图（false）
//
// 注意它**不会**落到"名录里的第一条"：官方明文说「without choosing another View」，
// 因为任意挑一个会让「我明明选的是轨迹」变成打开另一个东西。
func ResolveActiveView(tabs []pluginapi.Tab, selected string) (pluginapi.Tab, bool) {
	if selected != "" {
		for _, t := range tabs {
			if t.Key == selected {
				return t, true
			}
		}
	}
	for _, t := range tabs {
		if t.Key == DefaultViewKey {
			return t, true
		}
	}
	return pluginapi.Tab{}, false
}

// ResolveActiveRightTab 解析右栏该画哪个 tab。
//
// 官方 `defaultSeed`（OFF:dsh-client-ui-sidebar-right/src/client/contract/seed.ts:26-33）
// 的规则是「名录里**恰好一个**条目就直接上它，否则上 guide」。
//
// 本仓的差异必须说清：**guide tab 还没有实现**（`sidebar.right.tab.guide` 的骨架
// 座位在 official_slots.go 已登记，但没有占用方），所以"多个条目"这一支退化成
// 「按 (Order, 装配顺序) 取第一条」——那是名录里第一条真实存在的 tab，不是 guide。
// 这一条登记在 docs/official-parity.md 的缺口里，等 guide 面落地后改回官方规则。
func ResolveActiveRightTab(tabs []pluginapi.Tab, selected string) (pluginapi.Tab, bool) {
	if selected != "" {
		for _, t := range tabs {
			if t.Key == selected {
				return t, true
			}
		}
	}
	if len(tabs) == 0 {
		return pluginapi.Tab{}, false
	}
	return tabs[0], true
}

// ── 选择器绘制 ──────────────────────────────────────────────────────────

// DrawViewTabs 画会话 header 里的 view tab 行（官方 `.tabs` 里的 `.viewTabs`）。
//
// 几何全部来自官方 CSS（`.tab`：min-height 35 / margin -10px -10px 0 /
// padding 10px 10px 9px / font 13/16 / active 条 2px），与 dstopbar 现有实现同源；
// 这里抽成一处是为了让**官方骨架档**也能画同一行（骨架自己没有 header 插件）。
func DrawViewTabs(
	f ui.C, env *pluginapi.Env, tabs []pluginapi.Tab, active string, onSelect func(string),
) {
	if len(tabs) == 0 || env == nil || env.Theme == nil {
		return
	}
	t := env.Theme
	// Grow(1) 而不是 FillWidth：这一行永远画在 header 的 Row 里，后面还要跟
	// More 座位（`conversation.session.header.views.actions`）。FillWidth 会把
	// 兄弟节点挤出去——那正是官方 `.tabs{display:flex}` 里 `.viewTabs` 与
	// `.viewsMoreAnchor` 的关系（前者 flex 收缩、后者 flex:none）。
	ui.Row(f).Grow(1).Height(ViewTabRowHeight).Gap(ViewTabGap).Children(func() {
		for _, tab := range tabs {
			tab := tab
			var id gfx.ID
			if env.Tree != nil {
				id = env.Tree.ID(env.Feat, "view-"+tab.Key)
			}
			isActive := tab.Key == active
			box := ui.Column(f).Height(ViewTabRowHeight).
				Hit(id, tab.Label).Children(func() {
				ui.Row(f).Grow(1).Items(ui.Center).Children(func() {
					text := ui.Text(f, tab.Label).FontSize(ViewTabFontSize).
						LineHeight(ViewTabLineHeight).MaxLines(1).Ellipsis()
					if isActive {
						text.Color(t.LabelPrimary).Weight(gfx.WeightMedium)
					} else {
						text.Color(t.LabelTertiary)
					}
				})
				// 官方 `.tab::after`：2px 圆角条，只有选中项有底色；
				// 本仓沿用 dstopbar 既有的 BorderL4（见 dstopbar/drawTab）。
				rule := ui.Row(f).FillWidth().Height(ViewTabActiveBarHeight)
				if isActive {
					rule.Background(t.BorderL4)
				}
			})
			if box.Clicked() && onSelect != nil {
				onSelect(tab.Key)
			}
		}
	})
}

// DrawRightTabs 画右栏的 tab 条（官方 dockkit `.tabStrip` + `.tab`）。
//
// 逐值照 `index-CSAdOBx6.css` 的 `_123xe_` 模块：
//
//	.tabStrip  gap:4px; height:28px; padding:10px 6px 0 10px   → 外高 38
//	.tab       min-width:80px; max-width:170px; height:28px; padding:0 10px;
//	           font-size:13px; line-height:1; border-radius:radius-sm;
//	           color:label-secondary；选中 label-primary + 底 markdown-tag
//	.slot      宽 10px；:before 一条 .5px × 14px 的 border-l4 竖线，
//	           紧邻选中项的那一条**不画**（`.tabActive + .slot` 规则）
func DrawRightTabs(
	f ui.C, env *pluginapi.Env, tabs []pluginapi.Tab, active string, onSelect func(string),
) {
	if len(tabs) == 0 || env == nil || env.Theme == nil {
		return
	}
	t := env.Theme
	ui.Row(f).FillWidth().Height(RightTabStripRowHeight+RightTabStripPadTop).
		PadSides(RightTabStripPadTop, RightTabStripPadRight, 0, RightTabStripPadLeft).
		Gap(RightTabStripGap).Items(ui.Center).Children(func() {
		for i, tab := range tabs {
			tab := tab
			// 分隔轨：第一枚之前不画；紧邻选中项的那一条（左或右）也不画。
			if i > 0 && tabs[i-1].Key != active && tab.Key != active {
				ui.Row(f).Width(RightTabSepWidth).Height(RightTabStripRowHeight).
					Items(ui.Center).Justify(ui.Center).Children(func() {
					ui.Row(f).Width(RightTabSepRuleW).Height(RightTabSepRuleH).Background(t.BorderL4)
				})
			}
			var id gfx.ID
			if env.Tree != nil {
				id = env.Tree.ID(env.Feat, "right-tab-"+tab.Key)
			}
			isActive := tab.Key == active
			chip := ui.Row(f).Height(RightTabChipHeight).MinWidth(RightTabChipMinWidth).
				MaxWidth(RightTabChipMaxWidth).Radius(t.Radii.SM).
				PadSides(0, RightTabChipPadX, 0, RightTabChipPadX).
				Items(ui.Center).Hit(id, tab.Label)
			if isActive {
				chip = chip.Background(t.MarkdownTag)
			} else {
				chip = chip.Hover(ui.Look{}.Background(t.Hover))
			}
			box := chip.Children(func() {
				ui.Row(f).Grow(1).Items(ui.Center).Gap(RightTabChipGap).Children(func() {
					if tab.Icon != "" {
						ui.Text(f, tab.Icon).FontSize(RightTabChipFontSize).Color(t.LabelSecondary)
					}
					text := ui.Text(f, tab.Label).FontSize(RightTabChipFontSize).
						LineHeight(RightTabChipFontSize).MaxLines(1).Ellipsis()
					if isActive {
						text.Color(t.LabelPrimary)
					} else {
						text.Color(t.LabelSecondary)
					}
				})
			})
			if box.Clicked() && onSelect != nil {
				onSelect(tab.Key)
			}
		}
	})
}
