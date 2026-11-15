package dsplan

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
)

// RightTabPlan 是右栏 tab **类型**的 key。
//
// 官方把 `id`（包名，逐实现唯一）与 `kind`（类型判别 `'plan'`）分开，是为了让别的
// 扩展顶掉同名 builtin（`sidebarRightTabs.register({id: previewId, kind: 'plan'})`，
// OFF:index.ts:104-108）。那条 `priority` 覆盖机制本仓还没建，所以这里只有一个
// key——与 `plugins/dsterminal` 的 `RightTabTerminal` 同一条理由（那一处也有登记）。
const RightTabPlan = "plan"

// Plugin 持有两样跨帧状态：**当前打开的计划**（左键点了哪张卡）与右栏全文的排版结果。
//
// 没有跨帧状态的部分（帧里有哪些计划）每帧从 `env.Data.Frames` 现算：journal 帧
// 是壳给的只读快照，不需要本包缓存。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives

	// openAddress 是右栏正在看的那份计划的地址（`dsh-resource://plan/…`）。
	// 空 = 还没点过任何计划卡（此时右栏全文退到"本会话最新一份计划"）。
	openAddress string

	// preview 是右栏排版缓存（见 preview.go 的 layout）。
	preview previewState

	// boxes 是上一帧的几何（同一 paint 内有效，跨帧只抄值，见 cards.go 的 snapnote）。
	boxes previewBoxes
}

func (*Plugin) ID() string { return ServiceID }

func (*Plugin) Provides() []string { return nil }

func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	return nil
}

// Panels 注册两个**官方真有的**座位：
//
//	conversation.chat.turnTail（list/session）       Turn 尾计划卡
//	sidebar.right.pane.tab     （keyed/session）      右栏计划全文（key = plan）
//
// 官方另外两个座位（`conversation.input.plan` 的 chip、`conversation.plan-review.actions`
// 的「查看全文」）本包**不注册**：缺的数据/前置在 plan.go 包注释里逐条写了，
// 注册上去只会是永不出现的死代码。
//
// 右栏 tab 芯片上的文案来自 `Painter.Label`（官方 `title: () => t('preview.title')`，
// OFF:index.ts:104-108）；官方还注册了 `sidebar.right.pane.tab.title` 画"图标 + 标题"，
// 而本仓 official 档骨架不派发那个槽（`internal/uikernel/shell_skeleton.go:1067-1082`
// 只派发 tab 体），所以这里不注册标题座位——注册了也永远不画。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{
			ID:    "plan-cards",
			Slot:  offChatTurnTail,
			Order: 20, // dsactioncards 是 0；官方 Turn 尾也是"动作卡在上、计划卡在下"
			Draw:  p.drawCards,
		},
		{
			ID:    "plan-preview",
			Slot:  pluginapi.SlotRightPaneTab,
			Key:   RightTabPlan,
			Label: zhTitle,
			Order: 30, // dsterminal 是 20：终端在左、计划在右
			Draw:  p.drawPreview,
			// 正文是「布局后覆画」：Markdown 的 Paint 必须等布局定下来才知道画在哪
			// （与 dsconversation 的 MarkdownView 同一条理由）。
			AfterDraw: p.afterDrawPreview,
		},
	}
}

// offChatTurnTail 是官方 `conversation.chat.turnTail` 的字面量。
//
// 单独写一遍而不是引 `uikernel.OffChatTurnTail`：插件不 import uikernel
// （与 dsactioncards 的 `SlotTurnTail` 同一条做法，值必须与官方逐字一致）。
const offChatTurnTail pluginapi.Slot = "conversation.chat.turnTail"
