// official_seat.go —— 提问卡的**官方座位**（rc.2 `@deepseek-ai/dsh-client-ui-user-questions`）。
//
// # 官方挂在哪里（逐字）
//
// `ui-user-questions/src/client/index.ts:383-395`：
//
//	ctx.slots.inject('conversation.composer', () => ctx.slots.register({
//	  name: 'conversation.composer',
//	  select: ({pendingInteraction}) =>
//	    pendingInteraction instanceof PendingQuestion ? pendingInteraction : null,
//	  children: {'conversation.plan-review.actions': {kind:'list', scope:'session'}},
//	}, QuestionComposer))
//
// 也就是官方把提问卡注册成 `conversation.composer`（**chain** 槽）的一个
// **selector 路由接管项**：有 pending 问题 → 它接管常住 composer；没有 → 不接管。
//
// # 本仓的落点（与官方的差异，已登记）
//
// 本仓的槽位内核没有 selector 路由（`uikernel.Kernel.drawSlot` 只按 Order 顺序
// 派发同一槽的全部 painter）。所以这里做**同一座位上的额外一块**：
//
//	conversation.composer
//	├─ questions-bar-official (ORDER 3)   ← 本文件：待答问题时画在常住 composer 上方
//	└─ composer-official      (ORDER 10)  ← dscomposer 的输入卡片
//
// 位置语义与官方一致（提问在最上面，用户先看到问题），差别是**不替换**输入卡片。
// 这与本仓审批卡的做法同源（审批卡也是 chain 座位里的 overlay，画在输入卡上方）。
// 等内核支持 selector，把这块改成真正的接管即可（本报告 §未实现项登记）。
//
// # 几何（`QuestionComposer.module.css` rc.2 逐条）
//
//	.frame  { padding: 6px calc(clearance + 16px) 10px }   clearance = 16
//	.card   { border-radius: radius-xl(20); background: --dsw-specific-input-major;
//	          padding: 0 0 10px; max-height: min(60vh, 520px) }
//	.header { padding: 20px 16px 0 24px }
//	.eyebrow{ font-size:11; line-height:16 }   .title { font-size:16; line-height:22 }
//	.options{ padding: 4px 12px }
//
// 座位已经给了 clearance(16)，所以 `.frame` 这里只补它自己的那 16。
package dsquestions

import (
	"github.com/MYCF711/dsh-ui/internal/planreview"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

const (
	// 官方 chain 座位上的顺序：3 < dscomposer 的 10（问题在上、输入卡在下）。
	questionsOrder = 3

	// `.frame{padding:6px calc(clearance+16px) 10px}` 里属于本插件的那一半。
	questionFramePadTop    float32 = 6
	questionFramePadBottom float32 = 10
	questionFramePadX      float32 = 16
	// `.card{padding:0 0 10px; border-radius:radius-xl}`。
	questionCardPadBottom float32 = 10
	// `.header{padding:20px 16px 0 24px}`。
	questionHeaderPadTop     float32 = 20
	questionHeaderPadRight   float32 = 16
	questionHeaderPadLeft    float32 = 24
	questionEyebrowGap       float32 = 5
	questionEyebrowFont      float32 = 11
	questionEyebrowLine      float32 = 16
	questionTitleFont        float32 = 16
	questionTitleLine        float32 = 22
	questionOptionsMarginTop float32 = 8
	questionOptionsPadTop    float32 = 4
	questionOptionsPadX      float32 = 12
	questionOptionsGap       float32 = 1
	questionOptionPadTop     float32 = 8
	questionOptionPadX       float32 = 8
	questionOptionPadRight   float32 = 12
	questionOptionGap        float32 = 8
	questionOptionLabelFont  float32 = 14
	questionOptionLabelLine  float32 = 24
	questionOptionDescFont   float32 = 13
	questionOptionDescLine   float32 = 20

	// 仅 legacy 座位保留既有等待文案；官方座位的 eyebrow 严格使用 question.header。
	questionEyebrowText = "在等你回答"
)

// composerSeatLive 报告**这一帧**官方 composer 座位是不是活的。
//
// 判据只有档位（与 dsgoal 同源、理由同）：legacy 档位 dsshell 只画旧槽；
// official 档位官方骨架一定 dispatch `conversation.composer`（本插件自己也注册在
// 它上面，所以 `BridgeLegacyComposer` 必为假）。不能用 `OfficialSlotsLive()`：
// `-shell official` 的产品形状里整列兼容桥还开着，但 composer 是分段开关。
func (p *Plugin) composerSeatLive() bool { return p.tier.Official() }

// drawOfficial 画官方 `conversation.composer` 座位上的提问卡。
// 通用问题与计划审阅共用同一座位；通用卡的绘制在 question_flow.go。
func (p *Plugin) drawOfficial(f ui.C, env *pluginapi.Env) {
	if !p.composerSeatLive() {
		return
	}
	if review, ok := planreview.Pending(env.Data.Msgs); ok {
		p.drawPlanReviewOfficial(f, env, review)
		return
	}
	qs := pending(env.Data.Msgs)
	if len(qs) == 0 {
		return
	}
	p.drawQuestionFlow(f, env, qs)
}

// itoa 只服务交互 id 的序号（不引 strconv：这一层保持极简依赖）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 && i > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg && i > 0 {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
