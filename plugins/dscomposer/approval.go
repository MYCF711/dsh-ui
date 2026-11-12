// 审批条：待审批的工具调用画在输入框正上方。
//
// 为什么归 composer：这里是"用户该做决定的地方"——决定"允许/拒绝"和
// "接着说什么"发生在同一个位置，审批条不会跑到消息流里被滚走。
//
// 这一文件分两层，刻意分开：
//
//	计划层（approvalVisible / planApproval / chooseOption / oneLine）  纯函数，喂 Snapshot 就能算
//	绘制层（drawApproval / decide）                                    碰 flex.Frame 与 Actions
//
// 单测只打计划层：界面对不对，先看"该不该画、画什么、回传什么"算得对不对，
// 再看像素。
package dscomposer

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

const (
	// approvalArgsMax 是参数摘要最多留多少个**字符**（rune，不是字节）。
	// 按字节截会把"审批"切成半个字，屏幕上出现 �（UTF-8 被切断）。
	// 这只是逻辑上限；放不下的由 flex 的 Ellipsis() 再按像素收一次。
	approvalArgsMax = 120
	// approvalTitleMax 是标题行的字符上限（宿主给的 title 已经是单行，这里只防超长）。
	approvalTitleMax = 160
	// unknownField 是**宿主没给的字段**在界面上的显示值（跨线 §12 H13）。
	//
	// 为什么必须有一个具体词而不是留空：审批卡上"工具名/标题"是**身份**字段，
	// 留空等于画一张没有任何信息的卡——用户看到一条审批，却不知道在批准什么。
	// 留空还有一种更坏的读法："这个字段不存在"，而真相是"宿主没说"。
	//
	// 官方 ApprovalPanel 的 title/description/subject 里 title 是必填
	// （`ApprovalCard.tsx` 直接 `{title}`），所以"未知"是本仓对
	// "宿主缺字段"的如实标注，不是编造内容。
	unknownField = "未知"
)

// approvalVisible 决定整条审批条画不画。
//
// Approval == nil 就是"宿主没在等审批"：**整条不画**，连外边距都不留——
// 留一个空框等于告诉用户"这里本来有个东西"，比没有更糟。
func approvalVisible(s pluginapi.Snapshot) bool {
	return s.Approval != nil
}

// clipRunes 按字符截断，超长补一个省略号。
//
// 用 []rune 而不是 s[:n]：rune 边界天然是 UTF-8 边界，中文/emoji 都不会被切成半个字。
// 组合字符（e + ́）可能被拆开，这是可接受的：比切坏字节好得多，也不影响读。
func clipRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// oneLine 把任意文本压成一行再截断。
//
// 参数是 JSON 时会带 \n、\t 的转义（两个字符，不是真换行），但 title/detail
// 里可能是真换行——审批条只有一行的高度，真换行会把布局撑开，所以统一压成空格。
func oneLine(s string, max int) string {
	var b strings.Builder
	pendingSpace := false
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r':
			pendingSpace = true
		default:
			if pendingSpace && b.Len() > 0 {
				b.WriteRune(' ')
			}
			pendingSpace = false
			b.WriteRune(r)
		}
	}
	return clipRunes(b.String(), max)
}

// approvalTool 取显示用的工具名。
//
// 顺序：宿主直接给的 Tool → title 冒号前那一段（宿主 approvalTitle 的约定）。
// 都不成立就返回空串：界面只画"需要审批"，**不编一个"未知工具"以外的名字**。
// （"写入 <路径>" / "修改 <路径>" 没有冒号，解析不出工具名，也不猜。）
func approvalTool(ap *pluginapi.Approval) string {
	if ap == nil {
		return ""
	}
	if ap.Tool != "" {
		return ap.Tool
	}
	title := strings.TrimSpace(ap.Title)
	for _, sep := range []string{": ", "："} {
		if before, after, ok := strings.Cut(title, sep); ok {
			name := strings.TrimSpace(before)
			// 只认"一个词"的前缀：带空格的句子不是工具名（"这个命令会删除…"）。
			if name != "" && !strings.ContainsAny(name, " \t") && len([]rune(name)) <= 24 && after != "" {
				return name
			}
		}
	}
	return ""
}

// approvalArgs 取参数摘要：Args 优先，没有就回退兼容字段 Cmd，都空返回空串。
func approvalArgs(ap *pluginapi.Approval) string {
	if ap == nil {
		return ""
	}
	if ap.Args != "" {
		return oneLine(ap.Args, approvalArgsMax)
	}
	return oneLine(ap.Cmd, approvalArgsMax)
}

// chooseOption 从宿主给的 Options 里挑"允许 / 拒绝"该用哪一项。
//
// 规则只有一条：**只认宿主给的 kind**，不猜、不造。
// 优先选只生效一次的那一项（allow_once / reject_once），没有才退到
// 同方向的其它项（allow_always），并把它的名字一并返回——
// 这样界面能把"以后都允许"如实写出来，而不是用"允许"两个字骗用户。
func chooseOption(ap *pluginapi.Approval, allow bool) (pluginapi.ApprovalOption, bool) {
	if ap == nil {
		return pluginapi.ApprovalOption{}, false
	}
	exact, prefix := "reject_once", "reject"
	if allow {
		exact, prefix = "allow_once", "allow"
	}
	var loose *pluginapi.ApprovalOption
	for i := range ap.Options {
		opt := ap.Options[i]
		if opt.OptionID == "" || opt.Kind == "" {
			continue // 没 id 或没 kind 的项不能用：不知道它是干什么的
		}
		if opt.Kind == exact {
			return opt, true
		}
		if loose == nil && strings.HasPrefix(opt.Kind, prefix) {
			loose = &ap.Options[i]
		}
	}
	if loose != nil {
		return *loose, true
	}
	return pluginapi.ApprovalOption{}, false
}

// chooseOtherAllow 找与主允许项**不同**的另一个 allow 类选项。
//
// 判据是 optionId 不同（不是 kind 不同）：宿主可能给 allow_once / allow_always
// 两个不同 kind，也可能给两个同 kind 但不同 id 的（比如"允许这一次"和"允许这个命令"）。
// 只要 id 不同就是两个选择，界面都该让用户看得到。
func chooseOtherAllow(ap *pluginapi.Approval, main pluginapi.ApprovalOption) (pluginapi.ApprovalOption, bool) {
	if ap == nil {
		return pluginapi.ApprovalOption{}, false
	}
	for _, opt := range ap.Options {
		if opt.OptionID == "" || opt.Kind == "" {
			continue
		}
		if !strings.HasPrefix(opt.Kind, "allow") {
			continue
		}
		if opt.OptionID == main.OptionID {
			continue // 就是主按钮那一项
		}
		return opt, true
	}
	return pluginapi.ApprovalOption{}, false
}

// approvalPlan 是"这条审批条此刻画什么、点了回传什么"的纯计算结果。
//
// 不碰 env、不画东西：喂一个 Snapshot 就能算，单测直接断言它。
type approvalPlan struct {
	Visible bool
	Tool    string // 空 = 宿主没说工具名，不画
	Title   string
	Args    string // 已压成一行并截断
	Detail  string
	// Note 是按钮旁边/底部的一行说明："宿主没给 requestId，回传不出去"这类。
	// 空 = 没有要解释的（正常可用状态）。
	Note  string
	Allow approvalButton
	Deny  approvalButton
	// AllowAlways 是宿主给的第二个"允许"类选项（典型是「这个会话里以后都允许」）。
	//
	// 为什么单独一个字段而不是把 Allow 变成列表：宿主只给一个时（最常见），
	// 界面不该多画一个空按钮；给了两个时用户必须能选得到——否则连续几次工具调用
	// 就得点 N 次「允许这一次」，而宿主明明提供了"以后都允许"。
	// Option 为空 = 宿主没给这一项，不画。
	AllowAlways approvalButton
}

// approvalButton 是一个按钮：画什么字、回传哪一个 optionId、点不点得动。
type approvalButton struct {
	Label  string
	Option string // 要回传的 optionId；空 = 宿主没给这一项
	Usable bool   // false = 画成禁用态，点了什么也不会发生
}

// planApproval 把快照算成审批条要画的东西。
//
// wired 表示壳有没有接 Actions.Approve：
//   - 接了（wired=true）：每个按钮必须同时有 requestId 和 optionId 才点得动，
//     少一个就画成禁用 + 一行说明——**不许编 id 让它看起来能点**。
//   - 没接（wired=false，mockd 旧路径）：退回 Allow/Deny，不要 id 也能点。
func planApproval(s pluginapi.Snapshot, wired bool) approvalPlan {
	ap := s.Approval
	if ap == nil {
		return approvalPlan{}
	}
	plan := approvalPlan{
		Visible: true,
		Tool:    approvalTool(ap),
		Title:   oneLine(ap.Title, approvalTitleMax),
		Args:    approvalArgs(ap),
		Detail:  oneLine(ap.Detail, approvalTitleMax),
	}
	// 兼容旧字段：只有 Reason 没有 Detail 时，把它标成"模型自述（不可信）"再显示。
	// 模型说的话**不可信**这条标注不能省——它是审批界面上唯一一句"别人告诉你的话"。
	if plan.Detail == "" && ap.Reason != "" {
		plan.Detail = oneLine("模型自述（不可信）："+ap.Reason, approvalTitleMax)
	}
	// 标题和参数说的是同一件事时只留一行：省一行高度，也不重复念两遍。
	if plan.Title != "" && plan.Args != "" && strings.Contains(plan.Title, plan.Args) {
		plan.Title = ""
	}
	// **宿主没给的字段显示「未知」**（只针对身份字段，见 unknownField 注释）。
	//
	// 工具名：宿主没给 `toolCall.title` 的冒号前缀、也没有 `Tool` 时，
	// 条上仍要有"批的是谁"这一格——留空等于把"不知道"画成"没有"。
	// 标题：只有在**连参数摘要都没有**时才补「未知」；有参数摘要说明宿主
	// 已经交代了在批什么，再补一行"未知"是噪声。
	if plan.Tool == "" {
		plan.Tool = unknownField
	}
	if plan.Title == "" && plan.Args == "" {
		plan.Title = unknownField
	}

	allowOpt, allowOK := chooseOption(ap, true)
	rejectOpt, rejectOK := chooseOption(ap, false)
	// 再找**另一个** allow 类选项（kind 不同即可）。宿主这次给了 3 个选项时
	// （allow_once / allow_always / reject_once），只画一个允许按钮就等于替用户
	// 丢掉了"以后都允许"——而它恰恰是连续工具调用时最该点的那个。
	alwaysOpt, alwaysOK := chooseOtherAllow(ap, allowOpt)
	plan.Allow = approvalButton{Label: "允许", Option: allowOpt.OptionID}
	plan.Deny = approvalButton{Label: "拒绝", Option: rejectOpt.OptionID}
	if allowOK && allowOpt.Name != "" {
		plan.Allow.Label = allowOpt.Name // 宿主给的措辞优先（"允许这一次"/"以后都允许"）
	}
	if rejectOK && rejectOpt.Name != "" {
		plan.Deny.Label = rejectOpt.Name
	}

	if alwaysOK {
		plan.AllowAlways = approvalButton{Label: alwaysOpt.Name, Option: alwaysOpt.OptionID}
		if plan.AllowAlways.Label == "" {
			plan.AllowAlways.Label = "以后都允许"
		}
		plan.AllowAlways.Usable = ap.RequestID != ""
	}
	if wired {
		plan.Allow.Usable = ap.RequestID != "" && allowOK
		plan.Deny.Usable = ap.RequestID != "" && rejectOK
		plan.Note = approvalNote(ap, allowOK, rejectOK)
	} else {
		// 旧路径：Allow/Deny 不带 id，只要壳接了就能点。
		plan.Allow.Usable = true
		plan.Deny.Usable = true
	}
	return plan
}

// approvalNote 说明"为什么点不动"。能用的时候返回空串（不占地方）。
func approvalNote(ap *pluginapi.Approval, allowOK, rejectOK bool) string {
	switch {
	case ap.RequestID == "":
		return "宿主没给 requestId，这次审批回传不出去"
	case !allowOK && !rejectOK:
		return "宿主没给可选项，按钮点不动"
	case !allowOK || !rejectOK:
		return "宿主只给了一个方向的可选项"
	}
	return ""
}

// decide 把"用户点了哪一边"翻成一次真实回传，返回是否发出去了。
//
// 这是整个审批条唯一的出口：所有 id 都从快照里取，**一个都不自己造**。
func decide(ap *pluginapi.Approval, act pluginapi.Actions, allow bool) bool {
	if ap == nil {
		return false
	}
	if act.Approve != nil {
		opt, ok := chooseOption(ap, allow)
		if !ok || opt.OptionID == "" || ap.RequestID == "" {
			return false // 缺 id 就不发：发一个假的比不发更糟
		}
		act.Approve(ap.RequestID, opt.OptionID)
		return true
	}
	// 兼容旧壳：只清本地气泡。注意这**不是**审批完成（H13）。
	if allow {
		if act.Allow == nil {
			return false
		}
		act.Allow()
		return true
	}
	if act.Deny == nil {
		return false
	}
	act.Deny()
	return true
}

// pressOption 是 press 的"指定项"版本：用户点的就是这一项，id 已确定。
func (p *Plugin) pressOption(env *pluginapi.Env, optionID, label string) {
	ap := env.Data.Approval
	if !decideOption(ap, env.Act, optionID) {
		return
	}
	if p.logf != nil {
		p.logf("审批：%s → %s", label, optionID)
	}
}

// decideOption 用**指定**的 optionId 回传（allow_always 这类非主按钮走这里）。
//
// 和 decide 的区别只有一点：decide 按 allow/reject 二选一去"找"选项，
// 而这里用户点的就是某个具体项，id 已经确定——不能再找一次（那会点"以后都允许"
// 却回传"允许这一次"）。
func decideOption(ap *pluginapi.Approval, act pluginapi.Actions, optionID string) bool {
	if ap == nil || optionID == "" {
		return false
	}
	if act.Approve == nil || ap.RequestID == "" {
		return false // 没有回传通道就不假装成功
	}
	act.Approve(ap.RequestID, optionID)
	return true
}

// approvalTextWidth 取官方审批卡内容宽：chat content 减去 body 左右各 16px。
// 不再从整窗、侧栏和右栏反推；它与输入卡共用 composerColumn/contentWidth，
// 因而右栏开合和两种验收尺寸下都与实际卡片一致。
func approvalTextWidth(env *pluginapi.Env) float32 {
	w := contentWidth(composerColumn(env)) - 32
	if w < 120 {
		w = 120 // 极窄兜底不是官方档位，只保证文本不会被压成 0 宽
	}
	return w
}

// oneLineText 画一行"放不下就省略号"的文本。
//
// 两件事都要做对，缺一个省略号就不出现：
//   - **给一个确定的宽度**（MaxWidth）。Text 默认按内在宽度排版，宽到超出卡片时
//     是被裁掉，Ellipsis 根本来不及生效；
//   - **不要 Mono()**。WrapLines 是按比例字宽量的（canvas.TextWidthWeight 不带字面），
//     等宽字实际更宽，量出来"放得下"于是不省略，屏幕上还是硬切
//     （同一份截图里比例字那行有省略号、等宽那行没有，对比出来的）。
func oneLineText(f *flex.Frame, text string, width, size, leading float32, color ui.Color) {
	f.Row().FillWidth().Children(func() {
		f.Text(text).MaxWidth(width).FontSize(size).LineHeight(leading).Color(color).Ellipsis()
	})
}

// drawApproval 按官方 ApprovalPanel 画在输入框上方：root padding 8/12，卡片宽
// chat-content、radius-xl 20、InputMajor；strip → body → actionRow 三层。
// 按钮仍调用原来的 press/pressOption，requestId/optionId 回传路径不变。
func (p *Plugin) drawApproval(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	plan := planApproval(env.Data, env.Act.Approve != nil)
	if !plan.Visible {
		return
	}
	f.Column().FillWidth().PadSides(8, 0, 12, 0).Children(func() {
		f.Row().FillWidth().Justify(flex.Center).Children(func() {
			f.Column().Width(contentWidth(composerColumn(env))).Radius(t.Radii.XL).
				Background(t.InputMajor).Border(1, t.Warn).Clip().Children(func() {
				stripFill := t.Warn
				stripFill.Alpha = 0.12 // theme 没有 warn-tertiary；由 Warn 派生，不写新色号
				f.Row().FillWidth().Background(stripFill).PadSides(10, 16, 10, 16).
					Gap(8).Items(flex.Center).Children(func() {
					f.Text("需要审批").FontSize(t.Fonts.Secondary).LineHeight(18).Color(t.Warn)
					if plan.Tool != "" {
						f.Text(plan.Tool).FontSize(t.Fonts.Secondary).LineHeight(18).Color(t.Warn)
					}
					f.Spacer()
				})
				textW := approvalTextWidth(env)
				f.Column().FillWidth().PadSides(12, 16, 0, 16).Gap(6).Children(func() {
					if plan.Title != "" {
						oneLineText(f, plan.Title, textW, t.Fonts.Title, 24, t.LabelPrimary)
					}
					// 参数摘要：宿主原样给的，只压成一行、超长省略号。
					if plan.Args != "" {
						oneLineText(f, plan.Args, textW, t.Fonts.Secondary, 20, t.LabelTertiary)
					}
					if plan.Detail != "" {
						oneLineText(f, plan.Detail, textW, t.Fonts.Secondary, 20, t.LabelCaption)
					}
					if plan.Note != "" {
						oneLineText(f, plan.Note, textW, t.Fonts.Tiny, 16, t.LabelCaption)
					}
				})
				f.Row().Gap(8).Items(flex.Center).Justify(flex.End).
					PadSides(14, 16, 14, 16).Children(func() {
					// 顺序 = 官方 `.actionRow`（`ApprovalCard.tsx:87-97`）：
					// 拒绝（outline）→ 允许一次（宿主只给一个允许项时是 primary）
					// → 始终允许（primary）。**不重排**：官方把"破坏性最小的
					// 拒绝"放在最左、"以后都允许"放在最右，位置本身就是风险提示。
					if f.Button(env.Tree.ID(env.Feat, "approval-deny"), plan.Deny.Label).Outline().Small().
						Disabled(!plan.Deny.Usable).Clicked() && plan.Deny.Usable {
						p.press(env, false)
					}
					// 官方：宿主给了"始终允许"时，中间那个是 outline（primary 让给
					// 最右的始终允许）；只给一个允许项时它自己是 primary。
					allow := f.Button(env.Tree.ID(env.Feat, "approval-allow"), plan.Allow.Label).Small()
					if plan.AllowAlways.Option != "" {
						allow = allow.Outline()
					}
					if allow.Disabled(!plan.Allow.Usable).Clicked() && plan.Allow.Usable {
						p.press(env, true)
					}
					if plan.AllowAlways.Option != "" {
						if f.Button(env.Tree.ID(env.Feat, "approval-allow-always"), plan.AllowAlways.Label).Small().
							Disabled(!plan.AllowAlways.Usable).Clicked() && plan.AllowAlways.Usable {
							p.pressOption(env, plan.AllowAlways.Option, plan.AllowAlways.Label)
						}
					}
				})
			})
		})
	})
}

// press 是绘制层对 decide() 的薄封装：按钮点击走这里（键盘走 decideKey）。
func (p *Plugin) press(env *pluginapi.Env, allow bool) {
	ap := env.Data.Approval
	act := env.Act
	if !decide(ap, act, allow) {
		return
	}
	if p.logf != nil {
		opt, _ := chooseOption(ap, allow)
		verdict := "拒绝"
		if allow {
			verdict = "允许"
		}
		p.logf("审批：%s → %s", verdict, opt.OptionID)
	}
}
