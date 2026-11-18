package dsplan

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
)

// 右栏 tab 芯片的**标题座**（官方 `sidebar.right.pane.tab.title`）。
//
// 官方这个座位画两样东西（OFF:PlanPreview.tsx:47-53 的 `PlanTitle`）：
//
//	export function PlanTitle({ useTabInfo, useResource }: PropsRuntime<'sidebar.right.pane.tab.title'>) {
//	  const tab = useTabInfo()
//	  const resource = useResource<'plan'>(tab.tab.navigation.address)
//	  const params = tab.tab.navigation.params
//	  const plan = isReviewPreviewAddress(tab.tab.navigation.address)
//	    ? (params !== undefined && 'planReview' in params ? params.planReview : undefined) : resource.value
//	  return <><FileTypeIcon kind="other" size={16} className={css.titleIcon} />{plan?.title ?? tab.tab.title}</>
//	}
//
// 本仓没有"单独注册一个标题座位"的画法（骨架只派发 tab 体），所以拆成两条：
//
//	图标 → 注册期 `Painter.IconImage` + `Painter.IconSize`（DrawRightTabs 画）
//	文案 → `RightTabAddressTitleProvider`（壳投影名录时问一遍，覆盖 Tab.Label）
//
// 文案的取值顺序与官方一字不差：**计划的标题**，取不到才用类型标题
// （`plan?.title ?? tab.tab.title`，官方 `TabRecord.title` 就是这个类型注册的
// `title: () => t('preview.title')`，即本包的 `zhTitle` "计划"）。

// planTabIconSize 是官方 `size={16}`（OFF:PlanPreview.tsx:53）。
const planTabIconSize float32 = 16

// planTabIcon 建官方 `FileTypeIcon kind="other"` 的那一份（ViewBox 28）。
//
// 为什么可以在注册期预建、不用每帧重建：`other` 是官方唯一一个**两层都静态**的
// 文件类型分支（底 neutral-bluish-300、折角 neutral-400），既不跟 `currentColor`
// 走也没有 `--dsh-file-type-icon-color` 覆盖点；计划卡那枚 markdown 图标相反，
// 底板是 `currentColor` 才必须按帧现建。判据见 `dsassets.FileTypeOther` 的注释。
//
// 返回零值 `gfx.Icon` = 素材缺失（`ViewBox == 0`），此时 DrawRightTabs 不画图标，
// 芯片退成纯文字——不拿别的图形冒充。
func planTabIcon() gfx.Icon {
	asset, ok := dsassets.FileTypeOther()
	if !ok {
		return gfx.Icon{}
	}
	return asset.Sheet
}

// RightTabTitleFor 是官方 `PlanTitle` 的**文案那一半**，带地址问（见
// `pluginapi.RightTabAddressTitleProvider`）。
//
// 取值顺序：
//
//	① 临时审阅地址（`dsh-resource://plan-review/<session>/<request>`）
//	   → 该次审阅文档的标题（官方同一条 `params.planReview.title` 分支）
//	② 计划地址 → 本帧缓存里的计划标题（官方 `resource.value.title`，
//	   来自 `plan.ts:47` 对正文首个 `# 标题` 的提取）
//	③ 都没有 → 空串：壳保留注册期的类型标题（官方 `?? tab.tab.title`）
//
// 地址属于**别的会话**时返回空串：官方资源读的是地址里那个会话的 history，
// 本仓只有当前会话的帧，硬画会把别的会话的计划标题安在这枚芯片上。
//
// `frames` 是壳在本帧 Draw 之前递过来的原始帧（`Env.Data.Frames` 同一份）。
// 官方标题座位在**渲染期**读资源（`OFF:PlanPreview.tsx:47-53` 的 `useResource`），
// 所以它第一帧就有标题；本仓的 `planTitles` 是画的时候（`drawCards`/`drawPreview`）
// 才填的，只靠它就会「刚提交那一帧显示类型标题」。带上帧以后这里能像官方一样
// 现算：缓存命中就直接用（常态），没命中才扫一遍帧——只有新计划出现的那一帧会扫。
func (p *Plugin) RightTabTitleFor(sessionID, tabID, address string, frames []apiclient.Frame) string {
	if address == "" {
		return ""
	}
	if reviewSession, _, ok := parseReviewPreviewAddress(address); ok {
		if sessionID != "" && reviewSession != sessionID {
			return ""
		}
		if plan, exists := p.temporaryReviews[address]; exists {
			return plan.Title
		}
		return ""
	}
	planSessionID, callID, ok := parsePlanAddress(address)
	if !ok {
		return ""
	}
	if sessionID != "" && planSessionID != sessionID {
		return ""
	}
	if title := p.planTitles[callID]; title != "" {
		return title
	}
	// 缓存里还没有：用壳递来的帧现算（官方是渲染期同步读资源，同一条语义）。
	// 只认 callId 命中的那一份；扫到的标题顺手记进缓存，后面几帧不再扫。
	for _, plan := range SubmittedPlans(frames) {
		if plan.CallID != callID {
			continue
		}
		p.rememberPlanTitles([]Document{plan})
		return plan.Title
	}
	return ""
}

// rememberPlanTitles 把本帧所有已提交计划的标题记进缓存（供右栏芯片取用）。
//
// 只记不删：`callId` 是日志里的唯一身份，同一 callId 的标题不会变（计划正文是
// 不可变的调用参数），所以这条缓存与「本会话出现过哪些计划」等价。
func (p *Plugin) rememberPlanTitles(plans []Document) {
	for _, plan := range plans {
		if plan.CallID == "" || plan.Title == "" {
			continue
		}
		if p.planTitles == nil {
			p.planTitles = map[string]string{}
		}
		p.planTitles[plan.CallID] = plan.Title
	}
}

// 编译期断言：本插件提供带地址的实时标题（官方标题座位那一段）。
var _ pluginapi.RightTabAddressTitleProvider = (*Plugin)(nil)
