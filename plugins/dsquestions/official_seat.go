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
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
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
	questionHeaderPadTop    float32 = 20
	questionHeaderPadRight  float32 = 16
	questionHeaderPadLeft   float32 = 24
	questionHeaderGap       float32 = 4
	questionEyebrowFont     float32 = 11
	questionEyebrowLine     float32 = 16
	questionTitleFont       float32 = 16
	questionTitleLine       float32 = 22
	questionOptionsPadTop   float32 = 4
	questionOptionsPadX     float32 = 12
	questionOptionsGap      float32 = 6
	questionOptionPadTop    float32 = 8
	questionOptionPadX      float32 = 8
	questionOptionGap       float32 = 8
	questionOptionLabelFont float32 = 14
	questionOptionDescFont  float32 = 12

	// 官方 `.eyebrow` 的文案域是 `question` 命名空间；本仓没有 locale 座位，
	// 用旧座位已经对外说过的同一句话（不新造文案）。
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
//
// 可见性：**只在有待答问题时显示**（`pending(Msgs)` 为空 → 整块不画）。
func (p *Plugin) drawOfficial(f *flex.Frame, env *pluginapi.Env) {
	if !p.composerSeatLive() {
		return
	}
	qs := pending(env.Data.Msgs)
	if len(qs) == 0 {
		return
	}
	q := qs[0]
	p.cur = q
	p.act = env.Act
	t := p.theme
	if env.Theme != nil {
		t = env.Theme
	}
	send := env.Act.Send

	// `.frame`：座位给了 clearance 16，这里再补 16（官方是 clearance + 16）。
	f.Column().FillWidth().
		PadSides(questionFramePadTop, questionFramePadX, questionFramePadBottom, questionFramePadX).
		Children(func() {
			// `.card`
			f.Column().FillWidth().Radius(t.Radii.XL).Background(t.InputMajor).
				PadSides(0, 0, questionCardPadBottom, 0).Children(func() {
				// `.header`：eyebrow（在等你回答）+ title（问题本身）
				f.Column().FillWidth().
					PadSides(questionHeaderPadTop, questionHeaderPadRight, 0, questionHeaderPadLeft).
					Gap(questionHeaderGap).Children(func() {
					f.Row().Items(flex.Center).Gap(6).Children(func() {
						p.prims.Dot(f, t.LabelTertiary, 5)
						f.Text(questionEyebrowText).FontSize(questionEyebrowFont).
							LineHeight(questionEyebrowLine).Color(t.LabelTertiary)
					})
					f.Text(q.Question).FontSize(questionTitleFont).
						LineHeight(questionTitleLine).Color(t.LabelPrimary)
				})
				// `.options`：一行一个候选答案（点位 = 同一句 prompt）
				f.Column().FillWidth().
					PadSides(questionOptionsPadTop, questionOptionsPadX, 0, questionOptionsPadX).
					Gap(questionOptionsGap).Children(func() {
					for i, opt := range q.Options {
						p.drawOptionRow(f, env, t, i, opt, send)
					}
				})
			})
		})
}

// drawOptionRow 画一个候选答案行（两个座位共用同一套命中/发送语义）。
func (p *Plugin) drawOptionRow(f *flex.Frame, env *pluginapi.Env, t *pluginapi.Theme, index int, opt Option, send func(string)) {
	key := ""
	if index == 0 {
		key = "F7"
	} else if index == 1 {
		key = "F8"
	}
	row := f.Row().FillWidth().Items(flex.Center).Gap(questionOptionGap).
		PadSides(questionOptionPadTop, questionOptionPadX, questionOptionPadTop, questionOptionPadX).
		Radius(t.Radii.MD).Background(t.BgLayer2).
		Hit(env.Tree.ID(env.Feat, "opt-"+itoa(index)), opt.Label)
	if row.Clicked() && send != nil {
		// 点选项 = 发那句话。不带问题文本：agent 看得见自己问过什么。
		send(opt.Label)
	}
	row.Children(func() {
		if key != "" {
			// 能按的键要写出来；第三项之后没有键就不写，也不假装能选。
			f.Text(key).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		}
		f.Column().Gap(2).Grow(1).Children(func() {
			f.Text(opt.Label).FontSize(questionOptionLabelFont).Color(t.LabelPrimary)
			if opt.Description != "" {
				f.Text(opt.Description).FontSize(questionOptionDescFont).Color(t.LabelTertiary)
			}
		})
	})
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
