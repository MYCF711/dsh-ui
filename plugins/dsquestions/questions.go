// Package dsquestions 在 composer 上方显示 agent 正在等回答的问题。
//
// 官方 `dsh-client-ui-user-questions`：*Web ask_user_question composer takeover
// and plan-review presentation UI*。
//
// # 它是怎么工作的（宿主那边的契约决定了这里能做什么）
//
// `ask_user_question` 是**唯一一个"结果不是结果"的工具**：它设 `Result.Stop`，
// agent 循环在这一步收尾、把控制权交回人。所以：
//   - 它**不阻塞**等一个答案（没有可以回写的通道）
//   - 用户回答 = **下一条普通 prompt**
//
// 于是界面能做的是：把问题**显示出来**、把选项变成**可点的按钮**，
// 点了就发那句话。这和"接管 composer"在观感上一样，实现上简单得多。
//
// # 什么时候不显示
//
// 看消息流里 `ask_user_question` 之后**有没有 user 消息**：
//   - 没有 → 正在等（显示）
//   - 有   → 已经答过了（不显示）
//
// 不这么判断的话，一个答完的问题会永远挂在输入框上方。
package dsquestions

import (
	"encoding/json"
	"strconv"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// askTool 是宿主那个工具的名字（`modules/tool/ask`）。
const askTool = "ask_user_question"

// Option 是一个候选答案。
type Option struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// Question 是一条待答问题（宿主 schema 的投影）。
type Question struct {
	ID          string   `json:"id"`
	Question    string   `json:"question"`
	Header      string   `json:"header"`
	Options     []Option `json:"options"`
	MultiSelect bool     `json:"multi_select"`
}

type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives
	// tier 是壳档位（nil = 按 official 处理）。两个座位靠它二选一。
	tier *uikernel.ShellTier
	// act 由 draw 每帧刷新：HandleKey 里没有 env，只能靠它。
	act pluginapi.Actions
	// cur 是当前显示的那条问题（draw 填、HandleKey 读）。
	cur Question
}

func (*Plugin) ID() string { return "ds-questions" }

func (*Plugin) Provides() []string { return nil }

func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	// 可选注入：没有这个能力键（单测 / 旧壳）时按 official（官方槽是终态）。
	if v := ctx.Inject(uikernel.KeyShellTier); v != nil {
		if tier, ok := v.(*uikernel.ShellTier); ok {
			p.tier = tier
		}
	}
	return nil
}

// Panels 把提问卡挂到**两个座位**，同一帧只有一个活（判据 `composerSeatLive`）：
//
//	conversation.composer  官方 chain 座位（ui-user-questions index.ts:383）—— rc.2 终态
//	main.composer          dsshell 三栏 / 官方骨架兼容桥 —— legacy 档位的旧路径
//
// 官方是 selector 路由接管；本仓内核没有 selector，所以官方座位上的这一块
// 画在常住 composer 上方（见 official_seat.go 文件头的差异登记）。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID:   "questions-bar",
		Slot: pluginapi.SlotMainComposer,
		// Order 3 < GoalBar 的 5 < composer 的 10：最上面是问题，然后是目标，然后是输入框。
		// **等回答的时候问题比目标更紧急**——所以它在更上面。
		Order: 3,
		Draw: func(f *flex.Frame, env *pluginapi.Env) {
			if p.composerSeatLive() {
				return // 官方座位活着：旧槽这一帧不画（不双画）
			}
			p.draw(f, env)
		},
	}, {
		ID:    "questions-bar-official",
		Slot:  uikernel.OffConvComposer,
		Order: questionsOrder,
		Draw:  p.drawOfficial,
	}}
}

// pending 找出"最后一条提问之后还没被回答"的问题。
//
// 从后往前扫：`ask_user_question` 是最近的、且它之后没有 user 消息 → 正在等。
func pending(msgs []apiclient.Message) []Question {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role == "user" {
			// 提问之后已经有人说过话 → 答过了（或本来就是更早的会话）
			return nil
		}
		if m.Role != "tool" || m.Tool != askTool {
			continue
		}
		var args struct {
			Questions []Question `json:"questions"`
		}
		if json.Unmarshal([]byte(m.Args), &args) != nil {
			return nil // 参数不是 JSON：宁可不显示，也不画半截
		}
		return args.Questions
	}
	return nil
}

// HandleKey 绑 F7 / F8：直接选第 1 / 第 2 个选项。
//
// # 为什么用键盘而不是点击
//
// `flex` 那层没有点击 API（全仓 grep 确认过 `OnClick`/`OnPress`/`OnTap` 都没有），
// 而 `ui.Canvas.ClickedIn` 需要把每一行注册进一个 `ui.Scope`——
// 那不是"给一个问题条加个按钮"，是改 flex 的行模型。
//
// 键盘选择是**真的能按**的：F7/F8 在别处没被占用（F1 快捷键 / F2 命令 /
// F5 终端 / F6 终端发送 / F12 demo）。等 flex 有了点击，再补鼠标那条路。
func (p *Plugin) HandleKey(ev ui.KeyEvent) bool {
	if len(p.cur.Options) == 0 || p.act.Send == nil {
		return false
	}
	idx := -1
	switch ev.Key {
	case ui.KeyF7:
		idx = 0
	case ui.KeyF8:
		idx = 1
	default:
		return false
	}
	if idx >= len(p.cur.Options) {
		return false // 没那么多选项：不消费，让键冒泡
	}
	p.act.Send(p.cur.Options[idx].Label)
	return true
}

// draw 画问题条。没有待答问题就整块不画。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	qs := pending(env.Data.Msgs)
	if len(qs) == 0 {
		return
	}
	t := p.theme
	// **一次只显示第一条**：宿主 schema 说"一次问全，别挤成多轮"，
	// 但面板空间有限；把三个问题叠起来会谁都看不清。
	// 用户答完第一条，下一帧会显示剩下的（因为 Msgs 变了）。
	q := qs[0]
	p.cur = q
	p.act = env.Act

	f.Column().FillWidth().PadSides(16, 16, 8, 4).Gap(6).Children(func() {
		f.Row().Items(flex.Center).Gap(6).Children(func() {
			p.prims.Dot(f, t.LabelPrimary, 5)
			if q.Header != "" {
				f.Text(q.Header).FontSize(t.Fonts.Caption).Color(t.LabelPrimary)
			}
			f.Text("在等你回答").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		})
		f.Text(q.Question).FontSize(t.Fonts.Body).Color(t.LabelPrimary)
		for i, opt := range q.Options {
			opt := opt
			send := env.Act.Send
			key := ""
			if i == 0 {
				key = "F7"
			} else if i == 1 {
				key = "F8"
			}
			// **`.Hit(id, label)` 之后 `.Clicked()` 才有效**——viva 的点击模型是
			// "先给元素一个 id，再问它这一帧有没有被点"。我第一版用 `.OnClick(...)`
			// 搜不到就以为 flex 没有点击 API，**那是搜错了名字**：
			// 真正的入口是 `Hit`/`Clicked`，`dscore.ListRow` 也是这么用的。
			row := f.Row().FillWidth().Items(flex.Center).Gap(8).PadSides(8, 8, 7, 7).
				Radius(t.Radii.MD).Background(t.InputMajor).
				Hit(env.Tree.ID(env.Feat, "opt-"+strconv.Itoa(i)), opt.Label)
			if row.Clicked() {
				if send != nil {
					// 点选项 = 发那句话。**不带问题文本**：agent 能看到自己问过什么，
					// 再复述一遍只会让对话更啰嗦。
					send(opt.Label)
				}
			}
			row.Children(func() {
				if key != "" {
					// 把键位写在选项上：**能按的键要写出来**，否则只有读代码的人知道。
					// 第三项之后没有键——所以就**不写**，也不假装能选。
					f.Text(key).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
				}
				f.Column().Gap(2).Grow(1).Children(func() {
					f.Text(opt.Label).FontSize(t.Fonts.Body).Color(t.LabelPrimary)
					if opt.Description != "" {
						f.Text(opt.Description).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
					}
				})
			})
		}
	})
}
