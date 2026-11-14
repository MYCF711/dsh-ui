package dsconversation

// 消息反馈（官方 `@deepseek-ai/dsh-client-ui-message-feedback`）。
//
// # 官方语义，逐条对上
//
//   - 每条**已持久化**的助手消息下面有 👍/👎（`FeedbackActions`）；
//   - 点一个**未记录**的评分 → 打开弹窗（分类 chips + 详情输入 + 提交）；
//   - 提交 = 宿主 `messageFeedback.put`，带 `ifVersion`（乐观并发）；
//   - `version-conflict` 用回包里的 authoritative `current` 就地收敛，
//     不重拉整条会话（见 `cmd/app/reference_wiring.go`）；
//   - 点**已记录**的那个评分 = 撤回（`messageFeedback.delete`），不弹窗；
//   - 成功 toast「感谢你的反馈」；失败 toast 按官方错误码；
//   - 弹窗开着时点遮罩 / Esc / 关闭 = dismiss（不提交）。
//
// # 本仓的两处适配，必须写在账本里
//
//  1. **消息身份**：官方用宿主持久 `messageId`；本仓宿主当前只给 user 消息
//     id（`HOST:cmd/dshd/message_identity.go:13-14` 明说 assistant 构造器
//     没有 id）。助手消息用**持久 v4 事件序号**（`v4Seq`，即 `Message.Seq`）
//     适配成 `v4:<seq>` 作为 messageId。这是真实、持久、可复算的标识，不是编造；
//     但如果宿主将来给出 messageId，旧记录需要一次迁移（见报告）。
//  2. **弹窗落点**：官方挂在 `conversation.input.overlay`（portal 到 body）。
//     本仓的等价"portal"是 `shell.overlay`——输入区里的 overlay 槽只有输入框
//     宽度，装不下官方的 488px 模态。
//
// 数据面全部由壳的动作提供（`Actions.MessageFeedback*`），插件不建 Transport：
// 这一层只负责状态机与画法。

import (
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方 `.Hyy-Wa_action`：28×28 命中区、radius sm、svg 15×15。
const (
	feedbackHitSize  = 28
	feedbackIconSize = 15
)

// 官方文案（`dsh-client-ui-message-feedback/src/client/locales.ts` zh 段逐字）。
const (
	zhFeedbackLike        = "好的回答"
	zhFeedbackDislike     = "有问题的回答"
	zhFeedbackRetract     = "取消标记"
	zhFeedbackDialogTitle = "提交反馈"
	zhFeedbackCategories  = "反馈分类"
	zhFeedbackDetail      = "反馈详情"
	zhFeedbackHint        = "填写详情以帮助我们改进体验，提交内容会包括当前对话的日志"
	zhFeedbackSubmit      = "提交"
	zhFeedbackSubmitting  = "正在提交…"
	zhFeedbackRecorded    = "感谢你的反馈"
	zhFeedbackConflict    = "这条反馈已在别处改动，已显示最新状态"
	zhFeedbackLoad        = "反馈状态加载失败"
	zhFeedbackGeneric     = "反馈保存失败"
	zhFeedbackNoteLarge   = "描述太长，请缩短后再提交"
	zhFeedbackClose       = "关闭" // 官方 common.close
)

// 分类 id → 官方中文文案。
func feedbackCategoryLabel(id string) string {
	switch id {
	case apiclient.FeedbackCategoryTaskResult:
		return "任务结果"
	case apiclient.FeedbackCategoryInstructionFollowing:
		return "指令理解与遵循"
	case apiclient.FeedbackCategoryProductInteraction:
		return "产品功能与交互"
	case apiclient.FeedbackCategoryServiceStability:
		return "稳定性和速度"
	case apiclient.FeedbackCategoryResourceCost:
		return "资源使用与费用"
	case apiclient.FeedbackCategorySecurityPrivacyPermission:
		return "安全隐私与权限"
	case apiclient.FeedbackCategoryOther:
		return "其他"
	}
	return id
}

// feedbackDialogState 是一次打开的弹窗。
type feedbackDialogState struct {
	// MessageID 是写入宿主时用的消息身份（见文件头适配 1）。
	MessageID string
	// Rating 是用户点的那一侧（positive / negative）。
	Rating string
	// Category 是已选分类；空 = 没选（官方允许空）。
	Category string
	// field 是详情输入框（官方 textarea）。
	field ui.Field
	// submitting 表示已经发出 put、还没收到终态。
	submitting bool
	// failure 是上一次提交失败的文案（官方保持弹窗打开）。
	failure string
}

// feedbackMessageID 把一条消息映成宿主可接受的消息身份。
//
// 顺序：宿主 messageId（user/steering 有）→ v4Seq → load 的 Seq。
// 空 = 没有稳定身份：这条消息不画反馈按钮（不拿下标冒充 id）。
func feedbackMessageID(msg apiclient.Message) string {
	if msg.MessageID != "" {
		return msg.MessageID
	}
	if msg.V4Seq != nil {
		return "v4:" + itoa(int(*msg.V4Seq))
	}
	if msg.Seq > 0 {
		return "v4:" + itoa(msg.Seq)
	}
	return ""
}

// feedbackItems 取某会话的反馈表；壳没接这条动作时返回 nil,false。
func feedbackItems(act pluginapi.Actions, session string) (map[string]pluginapi.FeedbackItem, bool) {
	if act.MessageFeedback == nil || session == "" {
		return nil, false
	}
	return act.MessageFeedback(session)
}

// feedbackRating 返回这条消息当前的评分（"" = 没记录）。
func feedbackRating(act pluginapi.Actions, session string, msg apiclient.Message) string {
	id := feedbackMessageID(msg)
	if id == "" {
		return ""
	}
	items, _ := feedbackItems(act, session)
	return items[id].Rating
}

// openFeedbackDialog 打开弹窗（点未记录的一侧）。
func (p *Plugin) openFeedbackDialog(msg apiclient.Message, rating string) {
	id := feedbackMessageID(msg)
	if id == "" {
		return
	}
	p.feedbackDialog = &feedbackDialogState{MessageID: id, Rating: rating}
}

// dismissFeedbackDialog 关闭弹窗（不提交）。
func (p *Plugin) dismissFeedbackDialog() {
	p.feedbackDialog = nil
}

// submitFeedback 提交当前弹窗里的草稿。
func (p *Plugin) submitFeedback(act pluginapi.Actions, session string) {
	d := p.feedbackDialog
	if d == nil || d.submitting || act.MessageFeedbackPut == nil {
		return
	}
	// 官方 dialog.submitDraft：`const text = s.text.trim()`，空串就省略 note。
	note := strings.TrimSpace(d.field.Text())
	d.submitting = true
	d.failure = ""
	act.MessageFeedbackPut(session, d.MessageID, d.Rating, note, d.Category)
}

// retractFeedback 撤回一条已记录的评分。
func (p *Plugin) retractFeedback(act pluginapi.Actions, session string, msg apiclient.Message) {
	id := feedbackMessageID(msg)
	if id == "" || act.MessageFeedbackDelete == nil {
		return
	}
	p.feedbackRetract = true
	act.MessageFeedbackDelete(session, id)
}

// pollFeedback 在每帧开头消费壳的终态、并让 toast 过期。
//
// 为什么要"取走"：壳把终态放在一个单槽里，插件读一次就该清掉，
// 否则同一句 toast 会在每一帧重放（见 Actions.FeedbackTakeResult）。
func (p *Plugin) pollFeedback(env *pluginapi.Env) {
	if now := time.Now(); p.feedbackToast != "" && !now.Before(p.feedbackToastUntil) {
		p.feedbackToast = ""
	}
	if env.Act.FeedbackTakeResult == nil || env.Data.Cur == "" {
		return
	}
	result, ok := env.Act.FeedbackTakeResult(env.Data.Cur)
	if !ok {
		return
	}
	d := p.feedbackDialog
	// 只处理与当前弹窗/最近一次动作对应的结果；别的会话/消息不串。
	if d != nil && d.MessageID != result.MessageID {
		return
	}
	if result.OK {
		if d != nil {
			p.feedbackDialog = nil
		}
		if p.feedbackRetract {
			// 撤回不需要 acknowledgement toast（官方撤回也不弹「感谢反馈」）。
			p.feedbackRetract = false
			return
		}
		p.feedbackToast = zhFeedbackRecorded
		p.feedbackToastUntil = time.Now().Add(3 * time.Second)
		return
	}
	p.feedbackRetract = false
	text := feedbackFailureText(result.Code)
	if d != nil {
		d.submitting = false
		d.failure = text
		return
	}
	p.feedbackToast = text
	p.feedbackToastUntil = time.Now().Add(5 * time.Second)
}

// feedbackFailureText 把宿主业务码翻成官方文案。
func feedbackFailureText(code string) string {
	switch code {
	case "version-conflict":
		return zhFeedbackConflict
	case "note-too-large":
		return zhFeedbackNoteLarge
	case "load":
		return zhFeedbackLoad
	}
	return zhFeedbackGeneric
}

// drawFeedback 画 👍/👎 这一对，位置在"复制"与"重试"之间（官方 IconActions
// 的次序就是 copy → feedback → branch）。
//
// 壳没接 `MessageFeedback` / 这条消息没有稳定身份 → 一对都不画：
// 画一个点了记不下来的按钮，比不画更让人以为"已经反馈过了"。
func (p *Plugin) drawFeedback(f ui.C, env *pluginapi.Env, msg apiclient.Message) {
	t := p.theme
	session := env.Data.Cur
	if session == "" || env.Act.MessageFeedback == nil || feedbackMessageID(msg) == "" {
		return
	}
	rating := feedbackRating(env.Act, session, msg)

	button := func(up bool, value string) {
		active := rating == value
		label := zhFeedbackDislike
		if up {
			label = zhFeedbackLike
		}
		if active {
			label = zhFeedbackRetract
		}
		id := env.Tree.ID(env.Feat, "feedback-"+value+"-"+feedbackMessageID(msg))
		if ui.Row(f).Width(feedbackHitSize).Height(feedbackHitSize).
			Radius(t.Radii.SM).Items(ui.Center).Justify(ui.Center).
			Hover(ui.Look{}.Background(t.Hover)).HitWith(id, label, gfx.HitOptions{Role: gfx.RoleButton}).Children(func() {
			ui.Icon(f, thumbIcon(up, active)).Width(feedbackIconSize).
				Height(feedbackIconSize).Color(t.LabelPrimary)
		}).Clicked() {
			if active {
				p.retractFeedback(env.Act, session, msg)
			} else {
				p.openFeedbackDialog(msg, value)
			}
		}
	}
	button(true, apiclient.FeedbackRatingPositive)
	button(false, apiclient.FeedbackRatingNegative)
}

// —— 弹窗几何（官方 `FeedbackDialog.module.css` 逐值）——
const (
	feedbackDialogWidth  = 488 // .dialog.dialog{width:min(488px,100%)}
	feedbackDialogMargin = 32
	feedbackDialogGap    = 18 // 分类到详情 textarea 的距离 .detail{margin-top:18px}
	feedbackDialogPad    = 20
	feedbackChipHeight   = 28  // .chip{height:28px}
	feedbackChipPadX     = 12  // .chip{padding:0 12px}
	feedbackChipRadius   = 8   // radius-sm
	feedbackChipFont     = 13  // .chip{font-size:13px;line-height:18px}
	feedbackChipGap      = 8   // .categories{gap:8px}
	feedbackDetailHeight = 116 // .detail{min-height:116px}
	feedbackSubmitHeight = 44  // .submit{height:44px}
	feedbackTitleFont    = 16
)

// drawFeedbackOverlay 是 `shell.overlay` 的 painter：弹窗 + toast。
func (p *Plugin) drawFeedbackOverlay(f ui.C, env *pluginapi.Env) {
	p.pollFeedback(env)
	p.drawFeedbackToast(f, env)
	if p.feedbackDialog == nil {
		return
	}
	p.drawFeedbackDialog(f, env)
}

// drawFeedbackDialog 画官方反馈弹窗。
func (p *Plugin) drawFeedbackDialog(f ui.C, env *pluginapi.Env) {
	t := p.theme
	d := p.feedbackDialog
	width := float32(feedbackDialogWidth)
	if env.Bounds.Width > 0 && width > env.Bounds.Width-feedbackDialogMargin {
		width = env.Bounds.Width - feedbackDialogMargin
	}
	if width < 260 {
		width = 260
	}
	left := (env.Bounds.Width - width) / 2
	top := env.Bounds.Height * 0.16
	if top < 0 {
		top = 0
	}
	// 遮罩：点它 = dismiss（官方 Modal onClose）。
	mask := t.BgBase
	mask.Alpha = 0.55
	backdrop := ui.Row(f).Absolute().Inset(0).Background(mask).
		Hit(env.Tree.ID(env.Feat, "feedback-backdrop"), zhFeedbackClose)
	card := ui.Column(f).Absolute().Left(left).Top(top).Width(width).
		Hit(env.Tree.ID(env.Feat, "feedback-dialog"), zhFeedbackDialogTitle).
		Radius(t.Radii.Panel).Background(t.InputMajor).Border(1, t.BorderL2).
		Shadow(gfx.ShadowMD).Pad(feedbackDialogPad).Gap(feedbackDialogGap)
	card.Children(func() {
		// 标题行：标题 + 关闭。
		ui.Row(f).FillWidth().Items(ui.Center).Gap(8).Children(func() {
			ui.Text(f, zhFeedbackDialogTitle).FontSize(feedbackTitleFont).Weight(500).
				Color(t.LabelPrimary).Grow(1)
			if ui.Button(f, zhFeedbackClose).Ghost().Small().Clicked() {
				p.dismissFeedbackDialog()
			}
		})
		// 分类 chips：官方 flex-wrap；这里按 canvas 量宽排成多行。
		ui.Column(f).FillWidth().Gap(feedbackChipGap).Children(func() {
			rowWidth := float32(0)
			var row []string
			flush := func() {
				if len(row) == 0 {
					return
				}
				labels := row
				ui.Row(f).FillWidth().Gap(feedbackChipGap).Children(func() {
					for _, id := range labels {
						p.drawFeedbackChip(f, env, d, id)
					}
				})
				row = nil
				rowWidth = 0
			}
			for _, id := range apiclient.FeedbackCategoryValues() {
				w := env.Canvas.TextWidth(feedbackCategoryLabel(id), feedbackChipFont) + feedbackChipPadX*2
				if rowWidth+w+feedbackChipGap > width-float32(feedbackDialogPad*2) && len(row) > 0 {
					flush()
				}
				row = append(row, id)
				rowWidth += w + feedbackChipGap
			}
			flush()
		})
		// 详情 textarea（官方 min-height 116）。
		ui.TextArea(f, &d.field, zhFeedbackDetail, zhFeedbackHint).
			Height(feedbackDetailHeight).FillWidth()
		if d.failure != "" {
			ui.Text(f, d.failure).FillWidth().FontSize(t.Fonts.Caption).Color(t.Error)
		}
		// 提交按钮（官方 width:100%、height:44、radius lg）。
		label := zhFeedbackSubmit
		if d.submitting {
			label = zhFeedbackSubmitting
		}
		submit := ui.Button(f, label).FillWidth().Height(feedbackSubmitHeight).Radius(t.Radii.LG)
		if d.submitting {
			submit = submit.Disabled(true)
		}
		if submit.Clicked() {
			p.submitFeedback(env.Act, env.Data.Cur)
		}
	})
	if backdrop.Clicked() {
		p.dismissFeedbackDialog()
	}
}

// drawFeedbackChip 画一枚分类 chip（官方 `.chip` / `.chipActive`）。
func (p *Plugin) drawFeedbackChip(f ui.C, env *pluginapi.Env, d *feedbackDialogState, id string) {
	t := p.theme
	active := d.Category == id
	bg := t.InputMajor // 透明不适用：用卡片底色（与官方 transparent 视觉等价）
	border := t.BorderL4
	fg := t.LabelPrimary
	if active {
		bg = t.ButtonPrimaryFill
		border = t.ButtonPrimaryFill
		fg = t.ButtonPrimaryFg
	}
	hit := env.Tree.ID(env.Feat, "feedback-category-"+id)
	chip := ui.Row(f).Height(feedbackChipHeight).PadX(feedbackChipPadX).
		Radius(feedbackChipRadius).Background(bg).Border(1, border).Items(ui.Center).
		HitWith(hit, feedbackCategoryLabel(id), gfx.HitOptions{Role: gfx.RoleButton})
	chip.Children(func() {
		ui.Text(f, feedbackCategoryLabel(id)).FontSize(feedbackChipFont).Color(fg)
	})
	if chip.Clicked() && !d.submitting {
		if active {
			d.Category = "" // 再点一次取消选择（官方 `category: null`）
		} else {
			d.Category = id
		}
	}
}

// drawFeedbackToast 画官方 acknowledgement / failure toast。
func (p *Plugin) drawFeedbackToast(f ui.C, env *pluginapi.Env) {
	if p.feedbackToast == "" {
		return
	}
	t := p.theme
	width := env.Canvas.TextWidth(p.feedbackToast, t.Fonts.Secondary) + 32
	if width < 160 {
		width = 160
	}
	left := (env.Bounds.Width - width) / 2
	top := env.Bounds.Height - 120
	if top < 0 {
		top = 0
	}
	ui.Row(f).Absolute().Left(left).Top(top).Width(width).Height(40).Radius(t.Radii.MD).
		Background(t.BgLayer3).Border(1, t.BorderL2).Items(ui.Center).Justify(ui.Center).
		Children(func() {
			ui.Text(f, p.feedbackToast).FontSize(t.Fonts.Secondary).Color(t.LabelPrimary)
		})
}

// thumbIcon 是 16 viewBox 的拇指字形：`filled` 决定描边（空心）还是填充（实心）。
func thumbIcon(up, filled bool) gfx.Icon {
	commands := thumbCommands()
	if !up {
		commands = mirrorY(commands)
	}
	icon := gfx.Icon{
		ViewBox:  16,
		Cap:      gfx.CapRound,
		Join:     gfx.JoinRound,
		Commands: commands,
	}
	if !filled {
		icon.Stroke = 1.35
	}
	return icon
}

// mirrorY 把 16 高的字形上下翻过来：👎 就是 👍 的镜像，不另画一份。
func mirrorY(commands []gfx.PathCommand) []gfx.PathCommand {
	out := make([]gfx.PathCommand, len(commands))
	for i, c := range commands {
		if c.Kind == gfx.PathCommandClose {
			out[i] = c
			continue
		}
		c.Point.Y = 16 - c.Point.Y
		c.Control1.Y = 16 - c.Control1.Y
		c.Control2.Y = 16 - c.Control2.Y
		out[i] = c
	}
	return out
}

// thumbCommands 是拇指的轮廓：左边一小块掌根 + 右边带拇指的手背。
func thumbCommands() []gfx.PathCommand {
	return []gfx.PathCommand{
		gfx.MoveTo(2.6, 8.3),
		gfx.LineTo(3.9, 8.3),
		gfx.QuadTo(4.9, 8.3, 4.9, 9.3),
		gfx.LineTo(4.9, 13.4),
		gfx.QuadTo(4.9, 14.4, 3.9, 14.4),
		gfx.LineTo(2.6, 14.4),
		gfx.QuadTo(1.6, 14.4, 1.6, 13.4),
		gfx.LineTo(1.6, 9.3),
		gfx.QuadTo(1.6, 8.3, 2.6, 8.3),
		gfx.ClosePath(),

		gfx.MoveTo(5.6, 14.4),
		gfx.LineTo(5.6, 8.9),
		gfx.LineTo(6.3, 5.6),
		gfx.LineTo(7.1, 3.2),
		gfx.QuadTo(8.0, 1.9, 8.9, 3.2),
		gfx.LineTo(9.2, 5.0),
		gfx.LineTo(9.9, 8.0),
		gfx.LineTo(12.6, 8.0),
		gfx.QuadTo(13.8, 8.0, 13.8, 9.2),
		gfx.LineTo(13.8, 13.2),
		gfx.QuadTo(13.8, 14.4, 12.6, 14.4),
		gfx.ClosePath(),
	}
}
