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
	"strings"

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
	// RightTabChipLineHeight 是 `.tabTitle{line-height:1.4}`，即 13×1.4=18.2px。
	RightTabChipLineHeight float32 = 18.2
	// RightTabChipGap 是 `.tabTitle{gap:5px}`（图标与文字之间）。
	RightTabChipGap float32 = 5
	// RightTabTitleIconSize 是 BrowserTitle 的 IconGlobeOutlineRegular 默认尺寸。
	RightTabTitleIconSize float32 = 14
	// RightTabCloseSize / Inset 是 `.tabClose{width:20px;height:20px;top:4px;right:4px}`。
	RightTabCloseSize  float32 = 20
	RightTabCloseInset float32 = 4
	// RightTabControlIconSize 是 dockkit 的 close-fill / plus-outline 显式 size={14}。
	RightTabControlIconSize float32 = 14

	// RightTabSeparatorWidth 之外，芯片上的就地改名框（官方 `.name`）：
	//
	//	OFF:dsh-client-ui-sidebar-terminal/src/client/TerminalTitle.module.css:2
	//	  `.name{width:120px;min-width:48px;max-width:100%;padding:0 4px;color:inherit;
	//	   font:inherit;background:var(--dsw-alias-bg-l1);border:0.5px solid
	//	   var(--dsw-alias-border-l3);border-radius:var(--dsw-radius-sm);outline:none}`
	//
	// 它替换的是**标题那一段**：图标与徽标仍在左边（TerminalTitle.tsx:40-59）。
	RightTabRenameWidth    float32 = 120
	RightTabRenameMinWidth float32 = 48
	RightTabRenamePadX     float32 = 4
	RightTabRenameBorder   float32 = 0.5
	// RightTabRenameHeight 是 `.tab{font-size:13px;line-height:1}` 下 `.name`（`font:inherit`，
	// 没有自己的 height）拿到的行盒高 = 13px。Viva 的输入框必须有明确高度才有可命中、
	// 可挂 IME 的矩形，所以把这条 inherit 出来的行盒高写出来——不是另编一个数。
	RightTabRenameHeight float32 = 13

	// RightTabSepWidth 是 `.slot{width:10px}`：两枚芯片之间那条分隔轨。
	RightTabSepWidth float32 = 10
	// RightTabSepRuleH / W 是 `.slot:before{width:.5px;height:14px}`。
	RightTabSepRuleH float32 = 14
	RightTabSepRuleW float32 = 0.5

	// ── 芯片徽标（官方 primitives 的 `Tag`，`.badge` 包着它）────────────
	//
	// 数据源是官方终端标题的实现（`sidebar.right.pane.tab.title` 的 keyed 占用方）：
	//
	//	OFF:dsh-client-ui-sidebar-terminal/src/client/TerminalTitle.tsx:44-50
	//	  <TerminalIcon /> {badge} {title}
	//	OFF:…/TerminalTitle.module.css:3  `.badge{display:inline-flex;flex:none}`
	//
	// Tag 本体的几何在编译产物 CSS 里（`dsh-web-frontend/dist/assets/index-CSAdOBx6.css`
	// 的 `._tag_brmue_4` 模块）：
	//
	//	._tag_brmue_4{display:inline-flex;align-items:center;border-radius:999px;
	//	  corner-shape:round;padding:1px 8px;font-size:11px;line-height:17px;
	//	  font-weight:500;white-space:nowrap}
	//	._tag_brmue_4[data-tone=neutral]{background:var(--dsw-alias-bg-module-platform);
	//	  color:var(--dsw-alias-label-secondary)}
	//	._tag_brmue_4[data-tone=info]{background:color-mix(in srgb,
	//	  var(--dsw-alias-state-business-primary) 10%,transparent);
	//	  color:var(--dsw-alias-state-business-primary)}
	//
	// RightTabBadgeHeight = line-height 17 + 2×padding-y 1 = 19。
	RightTabBadgeLineHeight float32 = 17
	RightTabBadgePadY       float32 = 1
	RightTabBadgePadX       float32 = 8
	RightTabBadgeFontSize   float32 = 11
	// RightTabBadgeRadius 是 `border-radius:999px`（胶囊，不是一个 radius token）。
	RightTabBadgeRadius float32 = 999
	// RightTabBadgeInfoTint 是 info 档底的 `color-mix(… 10%, transparent)`。
	RightTabBadgeInfoTint float32 = 0.10

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
			if out[at].IconImage.ViewBox == 0 {
				out[at].IconImage = p.IconImage
				out[at].IconMarginRight = p.IconMarginRight
				out[at].IconSize = p.IconSize
			}
			continue
		}
		seen[p.Key] = len(out)
		out = append(out, pluginapi.Tab{Key: p.Key, Kind: p.Key, Multiple: p.Multiple,
			Label: p.Label, Icon: p.Icon, IconImage: p.IconImage, IconMarginRight: p.IconMarginRight,
			IconSize: p.IconSize, Order: p.Order})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

// RightTabKind 取 occurrence 的类型 key，供 keyed 槽派发。
// 老调用方构造的 Tab 没填 Kind 时回退 Key，保持单实例兼容。
func RightTabKind(tabs []pluginapi.Tab, active string) string {
	for _, tab := range tabs {
		if tab.Key != active {
			continue
		}
		if tab.Kind != "" {
			return tab.Kind
		}
		return tab.Key
	}
	return ""
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

// RightTabRenameState 是芯片就地改名的**壳侧**状态：官方 TerminalTitle 的
// `editing` 布尔 + `<input>` 那两件东西。
//
//	OFF:dsh-client-ui-sidebar-terminal/src/client/TerminalTitle.tsx:25-36
//	  chip.addEventListener('dblclick', () => { cancelled=false; setEditing(true) })
//	OFF:…/TerminalTitle.tsx:51-59
//	  editing ? <input className={css.name} maxLength={120} …/> : <span className={css.title}>{title}</span>
//
// 为什么状态放在壳里而不是 uikernel 里：viva 的 `*ui.Field` 属于**应用**（它是编辑器
// 与 IME 的真实宿主），跨帧持有它的是壳的 model；uikernel 只在被调用的那一帧读它、
// 画它、按它的命中身份处理回车/失焦。这样既没有包级可变状态，也不用把 model 塞进来。
type RightTabRenameState struct {
	// TabID 是正在改名的 occurrence；空 = 没在改名。
	TabID string
	// Field 是编辑器（文本 + 光标 + 选区 + IME 都在里面）。
	Field ui.Field
	// focused 记这一轮编辑是否已经拿到过焦点。AutoFocus 有至少一帧的延迟，
	// 没有它就会「刚打开就被判成失焦 → 立刻提交空名字」。
	focused bool
}

// Begin 开始改名：把当前标题灌进编辑器（官方 `<input defaultValue={title}>` 同义）。
func (s *RightTabRenameState) Begin(tabID, title string) {
	if s == nil {
		return
	}
	s.TabID = tabID
	s.Field.SetText(title)
	s.focused = false
}

// End 收起编辑器。提交与取消都走它——差别只在调用方有没有先交出新名字。
func (s *RightTabRenameState) End() {
	if s == nil {
		return
	}
	s.TabID = ""
	s.focused = false
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
	DrawRightTabsEditable(f, env, tabs, active, onSelect, nil)
}

// DrawRightTabsEditable 是 DrawRightTabs 的**可改名**版本：`rename` 非 nil 时，
// 双击芯片标题进入就地编辑（官方 TerminalTitle 的行为）。
//
// 提交/取消的判据逐条照官方：
//
//	回车        → 提交（官方 Enter 只 blur，blur 那一支再提交，TerminalTitle.tsx:60-68）
//	失焦        → 提交（`onBlur`，:55-59）
//	提交前      → `trim()`；空串或与原值相同都不提交（`:55-58`）
//	Esc         → 取消（`:64-66` 先 `cancelled.current = true` 再 blur，于是不提交）。
//	              viva 把 Esc 交给窗口的 OnKey 而不是输入框，所以这一支在壳里
//	              （`cmd/app/right_tab_rename.go` 的 cancelRightTabRename）。
func DrawRightTabsEditable(
	f ui.C, env *pluginapi.Env, tabs []pluginapi.Tab, active string, onSelect func(string),
	rename *RightTabRenameState,
) {
	if env == nil || env.Theme == nil {
		return
	}
	// 正在改名的那枚 occurrence 不在这一帧的名录里（被 Agent 结束、被别处关掉，
	// 或会话切走）：编辑态在**绘制线程**收起，不去后台线程改 `ui.Field`。
	if rename != nil && rename.TabID != "" && !rightTabPresent(tabs, rename.TabID) {
		rename.End()
	}
	if len(tabs) == 0 {
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
			var id, closeID gfx.ID
			if env.Tree != nil {
				id = env.Tree.ID(env.Feat, "right-tab-"+tab.Key)
				closeID = env.Tree.ID(env.Feat, "right-tab-close-"+tab.Key)
			}
			isActive := tab.Key == active
			closeHovered := env.Canvas != nil && (env.Canvas.Hovered() == closeID || env.Canvas.Pressed() == closeID ||
				env.Canvas.Focused() == closeID || env.Canvas.FocusVisible() == closeID)
			chipHovered := closeHovered || env.Canvas != nil && (env.Canvas.Hovered() == id || env.Canvas.Pressed() == id ||
				env.Canvas.Focused() == id || env.Canvas.FocusVisible() == id)
			chip := ui.Row(f).Height(RightTabChipHeight).MinWidth(RightTabChipMinWidth).
				MaxWidth(RightTabChipMaxWidth).Radius(t.Radii.SM).
				PadSides(0, RightTabChipPadX, 0, RightTabChipPadX).
				Items(ui.Center).Hit(id, tab.Label)
			if isActive {
				chip = chip.Background(t.MarkdownTag)
			} else {
				chip = chip.Hover(ui.Look{}.Background(t.Hover))
			}
			// 官方 tab 标题里的图标是 `stroke="currentColor"`（TerminalIcon.tsx:9、
			// BrowserTitle.tsx:3,16），颜色跟着芯片文字走：选中 label-primary、
			// 未选中 label-secondary。图标和文字同色是官方形态，不能各挂一档。
			titleColor := t.LabelSecondary
			if isActive {
				titleColor = t.LabelPrimary
			}
			// 就地改名（官方 TerminalTitle 的 editing 分支）。只有「壳接了改名动作 +
			// 这一枚 occurrence 正在改名 + 有交互树能给出稳定身份」时才进这一支；
			// 否则就是普通的标题文字。
			var renameID gfx.ID
			editing := false
			if rename != nil && rename.TabID == tab.Key && env.Act.RenameRightTab != nil && env.Tree != nil {
				renameID = env.Tree.ID(env.Feat, "right-tab-rename-"+tab.Key)
				editing = true
			}
			chip.Children(func() {
				label := ui.Row(f).Grow(1).Items(ui.Center).Gap(RightTabChipGap)
				label.Children(func() {
					if tab.IconImage.ViewBox > 0 {
						iconSize := RightTabTitleIconSize
						if tab.IconSize > 0 {
							iconSize = tab.IconSize
						}
						ui.Row(f).Shrink(0).PadSides(0, tab.IconMarginRight, 0, 0).Children(func() {
							ui.Icon(f, tab.IconImage).Size(iconSize, iconSize).Color(titleColor)
						})
					} else if tab.Icon != "" {
						ui.Text(f, tab.Icon).FontSize(RightTabChipFontSize).Color(titleColor)
					}
					// 官方徽标**在标题之前**：TerminalTitle.tsx:44-45 的注释
					// 「The badge precedes the name because the tab's close button
					// covers the end of the title」。
					drawRightTabBadge(f, t, tab.Badge)
					if editing {
						// 官方 `<input className={css.name} maxLength={120}>`：替换标题那一段，
						// 图标与徽标留在左边（TerminalTitle.tsx:51-59）。
						editor := ui.TextFieldID(f, renameID, &rename.Field, "重命名标签", "").
							Width(RightTabRenameWidth).MinWidth(RightTabRenameMinWidth).
							MaxWidthPercent(100).
							PadSides(0, RightTabRenamePadX, 0, RightTabRenamePadX).
							FontSize(RightTabChipFontSize).Height(RightTabRenameHeight).
							Background(t.BgLayer1).
							Border(RightTabRenameBorder, t.BorderL3).Radius(t.Radii.SM).
							AutoFocus()
						commit := func() {
							next := strings.TrimSpace(rename.Field.Text())
							// 官方：`next !== '' && next !== title` 才调 rename；同值等于没改。
							if next != "" && next != tab.Label && env.Act.RenameRightTab != nil {
								env.Act.RenameRightTab(tab.Key, next)
							}
							rename.End()
						}
						// 回车提交。**Esc 不在这里**：viva 把 Esc 交给窗口的 OnKey
						// （`app.Config.OnKey`），不会送到输入框的 OnKey 上；而那一刻
						// 必须先把它从插件手里截下来（否则会顺带拒一条审批）。
						// 所以取消那一支归壳（`model.cancelRightTabRename`，见
						// cmd/app/right_tab_rename.go），这里只管提交。
						editor.OnKey(func(ev gfx.KeyEvent) gfx.EventResult {
							if ev.Key == gfx.KeyEnter {
								commit()
								return gfx.EventResult{Handled: true, Repaint: true}
							}
							return gfx.EventResult{}
						})
						if editor.Submitted() {
							commit()
						}
						// 失焦提交（官方 onBlur）：AutoFocus 在第一帧就拿到焦点，
						// 所以「拿到过焦点 → 这一帧焦点不在了」就是真的失焦。
						// 拿不到 Canvas 判据时不做这件事，宁可留着自己按回车。
						if env.Canvas != nil {
							if env.Canvas.Focused() == renameID {
								rename.focused = true
							} else if rename.focused {
								commit()
							}
						}
					} else {
						text := ui.Text(f, tab.Label).FontSize(RightTabChipFontSize).
							LineHeight(RightTabChipLineHeight).MaxLines(1).Ellipsis()
						if isActive {
							text.Color(t.LabelPrimary)
						} else {
							text.Color(t.LabelSecondary)
						}
					}
				})
				// 官方 .tabClose：active / hover / focus-within 才出现，绝对定位 20×20、top/right 4。
				if env.Act.CloseRightTab != nil && (isActive || chipHovered) {
					closeColor := t.LabelTertiary
					if env.Canvas != nil && (env.Canvas.Hovered() == closeID || env.Canvas.Pressed() == closeID) {
						closeColor = t.LabelPrimary
					}
					close := ui.Row(f).Width(RightTabCloseSize).Height(RightTabCloseSize).Shrink(0).
						Absolute().Top(RightTabCloseInset).Right(RightTabCloseInset).
						Items(ui.Center).Justify(ui.Center).Radius(t.Radii.SM).
						Hover(ui.Look{}.Background(t.Hover)).Hit(closeID, "关闭")
					close.Children(func() {
						ui.Icon(f, rightTabCloseIcon()).Size(RightTabControlIconSize, RightTabControlIconSize).Color(closeColor)
					})
					if close.Clicked() {
						env.Act.CloseRightTab(tab.Key)
					}
				}
			})
			// 官方把 `dblclick` 挂在**整枚芯片**上（`[data-dockkit-tab]`，
			// TerminalTitle.tsx:25-36「Dockkit captures the pointer on its drag
			// handle, which receives the double-click」），所以命中区是芯片本身。
			// 双击同时也选中（官方 Examples 里 setEditing 不拦点击）。
			if chip.DoubleClicked() && rename != nil && env.Act.RenameRightTab != nil && env.Tree != nil {
				if rename.TabID != tab.Key {
					rename.Begin(tab.Key, tab.Label)
				}
			}
			if chip.Clicked() && onSelect != nil {
				onSelect(tab.Key)
			}
		}
		// dockkit addTab 的落点。官方这里先打开 guide；本仓 guide 仍未实现，
		// 所以只对已声明 multiple:true 的类型开放，并走同一个 openTab 动作。
		for _, tab := range tabs {
			if !tab.Multiple || env.Act.OpenRightTab == nil {
				continue
			}
			kind := tab.Kind
			if kind == "" {
				kind = tab.Key
			}
			var addID gfx.ID
			if env.Tree != nil {
				addID = env.Tree.ID(env.Feat, "right-tab-add")
			}
			addColor := t.LabelSecondary
			if env.Canvas != nil && (env.Canvas.Hovered() == addID || env.Canvas.Pressed() == addID) {
				addColor = t.LabelPrimary
			}
			add := ui.Row(f).Width(RightTabChipHeight).Height(RightTabChipHeight).Shrink(0).
				Items(ui.Center).Justify(ui.Center).Radius(t.Radii.SM).
				Hover(ui.Look{}.Background(t.Hover)).
				Hit(addID, "新建浏览器标签页")
			add.Children(func() {
				ui.Icon(f, rightTabAddIcon()).Size(RightTabControlIconSize, RightTabControlIconSize).Color(addColor)
			})
			if add.Clicked() {
				env.Act.OpenRightTab(kind)
			}
			break
		}
	})
}

// rightTabPresent 报某枚 occurrence 是否还在名录里（就地改名的收尾判据）。
func rightTabPresent(tabs []pluginapi.Tab, key string) bool {
	for _, tab := range tabs {
		if tab.Key == key {
			return true
		}
	}
	return false
}

// drawRightTabBadge 画官方芯片徽标：`.badge{display:inline-flex;flex:none}` 包着一枚
// primitives 的 `Tag`。
//
// 数据与形态来自官方终端标题（`sidebar.right.pane.tab.title` 的 keyed 占用方）：
//
//	OFF:dsh-client-ui-sidebar-terminal/src/client/TerminalTitle.tsx:44-50
//	  Agent 控制   → <Tag tone="info">Agent</Tag>
//	  沙箱（未接管）→ <Tag tone="neutral">沙箱</Tag>
//	几何 OFF:dsh-web-frontend/dist/assets/index-CSAdOBx6.css 的 `._tag_brmue_4`
//	  （11px / line-height 17 / padding 1px 8px / 圆角 999 / 500 字重）
//
// nil 或空文案 = 这一帧没有徽标（官方两个分支都不渲染）。
func drawRightTabBadge(f ui.C, t *pluginapi.Theme, badge *pluginapi.TabBadge) {
	if badge == nil || badge.Text == "" || t == nil {
		return
	}
	// `[data-tone=neutral]{background:bg-module-platform;color:label-secondary}`
	background, foreground := t.BgModulePlatform, t.LabelSecondary
	if badge.Tone == pluginapi.TabBadgeInfo {
		// `[data-tone=info]`：底是业务状态色的 10%（color-mix 与 transparent 混 -> 同色不同透明度），
		// 字是同一个业务状态色。
		foreground = t.Info
		background = t.Info.WithAlpha(RightTabBadgeInfoTint)
	}
	// `.badge{display:inline-flex;flex:none}`：宽度由内容定，不参与拉伸。
	ui.Row(f).Height(RightTabBadgeLineHeight+2*RightTabBadgePadY).Shrink(0).
		Radius(RightTabBadgeRadius).Background(background).
		PadSides(RightTabBadgePadY, RightTabBadgePadX, RightTabBadgePadY, RightTabBadgePadX).
		Items(ui.Center).Children(func() {
		ui.Text(f, badge.Text).FontSize(RightTabBadgeFontSize).
			LineHeight(RightTabBadgeLineHeight).Weight(gfx.WeightMedium).
			MaxLines(1).Color(foreground)
	})
}
