package dsplan

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

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

	// chip 是 /plan off 的在飞与错误状态。
	chip planChipState
	// planTitles 是「callId → 计划标题」的本帧缓存，供右栏芯片的**资源标题**用。
	//
	// 为什么要有这一层：官方标题座位 `useResource<'plan'>(tab.tab.navigation.address)`
	// 在渲染期就能拿到资源（OFF:PlanPreview.tsx:47-53），本仓的资源是
	// `env.Data.Frames`，而壳问标题（`rightTabsFor`）时**没有 env**。所以计划正文
	// 每帧照常从帧里算（`drawCards`），顺手把标题记下来，壳下一帧问标题时就有了。
	// 代价是「刚提交那一帧的芯片仍显示类型标题『计划』」，下一帧起才是计划标题。
	planTitles map[string]string
	// openedReviews 记录本会话已自动打开过的审阅，手动关右栏后不被下一帧重新打开。
	openedReviews map[string]bool
	// reviewWindow 是官方 reviewPreviewAddress 的浏览器生命周期随机前缀。
	reviewWindow string
	// temporaryReviews 只存临时 plan-review 文档，不进入会话历史或右栏布局持久化。
	temporaryReviews map[string]Document
}

func (*Plugin) ID() string { return ServiceID }

func (*Plugin) Provides() []string { return nil }

func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return fmt.Errorf("dsplan: 创建 plan-review 浏览窗口身份失败: %w", err)
	}
	p.reviewWindow = hex.EncodeToString(entropy[:])
	return nil
}

// Panels 注册官方四个可见座位：
//
//	conversation.chat.turnTail          Turn 尾计划卡
//	sidebar.right.pane.tab (plan)       右栏计划全文
//	conversation.input.plan             计划模式 chip（真实 plan 投影 + /plan off）
//	conversation.plan-review.actions    待审计划的自动/手动「查看全文」
//
// 右栏 tab 芯片上的**类型**文案来自 `Painter.Label`（官方 `title: () => t('preview.title')`，
// OFF:index.ts:65-69）；芯片上的**资源**文案与图标走官方的标题座位
// `sidebar.right.pane.tab.title`（OFF:index.ts:112-114 + OFF:PlanPreview.tsx:47-53）。
// 本仓 official 档骨架不派发那个座位（只派发 tab 体），所以这里把它拆成两半接：
// 图标进 `Painter.IconImage`/`IconSize`，文案进 `RightTabAddressTitleProvider`
// （见 tab_chip.go），两条都由壳在投影名录时用上。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{
			ID:    "plan-cards",
			Slot:  offChatTurnTail,
			Order: 20, // dsactioncards 是 0；官方 Turn 尾也是"动作卡在上、计划卡在下"
			Draw:  p.drawCards,
		},
		{
			ID:    "plan-mode-chip",
			Slot:  offInputPlan,
			Order: 20,
			Draw:  p.drawPlanChip,
		},
		{
			ID:    "plan-review-open",
			Slot:  offPlanReviewActions,
			Order: 20,
			Draw:  p.drawReviewAction,
		},
		{
			ID:    "plan-preview",
			Slot:  pluginapi.SlotRightPaneTab,
			Key:   RightTabPlan,
			Label: zhTitle,
			Order: 30, // dsterminal 是 20：终端在左、计划在右
			// 芯片标题前的图标是官方 `FileTypeIcon kind="other" size={16}`
			// （OFF:PlanPreview.tsx:53，PlanTitle 那一段）。
			//
			// 它是**静态**的两层色板（底 neutral-bluish-300、折角 neutral-400），
			// 不跟 currentColor 走，详见 dsassets.FileTypeOther 的注释；所以这里
			// 预建一份就与官方同色，不需要按帧重建（与计划卡那枚 markdown 图标不同）。
			IconImage: planTabIcon(),
			IconSize:  planTabIconSize,
			Draw:      p.drawPreview,
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

const (
	offInputPlan         pluginapi.Slot = "conversation.input.plan"
	offPlanReviewActions pluginapi.Slot = "conversation.plan-review.actions"
)
