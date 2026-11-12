package dsconversation

// 消息反馈（官方 `@deepseek-ai/dsh-client-ui-message-feedback` 的位置）：
// 每条**定稿**助手消息动作行里的 👍/👎，点一下变实心，再点一下撤回。
//
// # 这里到底记录了什么，以及没记录什么
//
// 官方把评分提交给宿主（`messageFeedback.put/retract`，按 messageId + version 对账），
// 失败会回滚并弹 toast。**本仓的 Transport 没有这些方法**（见 docs/conversation-gap.md
// 第 23 项与第 94 行：不许用只存在内存的 👍/👎 冒充"已记录"）。
//
// 所以这里的边界写死：
//
//   - 记录走**界面配置**这一条真实通道：`Actions.ConfigGet/ConfigSet`
//     （壳把它们接到宿主 `config.get`/`config.set`，落 `<home>/state/ui-config.json`）。
//     键是 `feedback.<sessionID>.<msgIndex> = up|down` 一条扁平记录。
//     这是界面自己的配置表，不是会话日志，**不进模型上下文**。
//   - **没有任何上报**：宿主没有 feedback API，界面上也不出现"已上报/已反馈给官方"
//     这类词——点击只改图标状态（空心↔实心）。
//   - 壳没接 `ConfigSet`（`nil`）：只保留内存态。这一帧的图标照样切换
//     （那是一次真实的本地选择），但**重启即失**，代码不假装它落了盘。
//
// 为什么键里是 msgIndex 而不是 messageId：旧 `session.load` 投影不保证给
// messageId（docs/message-identity.md 第 2 节），而 msgIndex 是此时屏幕上这条消息
// 的位置，能和用户看到的那一条对上。代价是同一会话的窗口滚动/翻页会让下标漂移，
// 这也是为什么它只是一条本机备注、不被当成官方评分。
//
// 撤销为什么要写空串：`ConfigSet` 是**合并**语义，没有删除动作。所以撤回写 `""`
// 墓碑；读回来时只认 `up`/`down`，空串等于"没记录"。配置里留一个空值不会让界面
// 以为有评分——比留一个旧评分冒充"已记录"诚实。

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 评分的两个取值。空串 = 没记录（撤回时写的就是它）。
const (
	feedbackUp   = "up"
	feedbackDown = "down"
)

// feedbackKeyPrefix 是这条记录在界面配置表里的前缀。
const feedbackKeyPrefix = "feedback."

// 官方 `.Hyy-Wa_action`：28×28 命中区、radius sm、svg 15×15、padding 6。
// 这里逐项照抄，所以它和同一排 Ghost/Small 按钮（高 28）在同一基线上。
const (
	feedbackHitSize  = 28
	feedbackIconSize = 15
)

// feedbackKey 是配置表里的键：`feedback.<sessionID>.<msgIndex>`。
func feedbackKey(session string, index int) string {
	return feedbackKeyPrefix + session + "." + itoa(index)
}

// seedFeedback 从壳的配置缓存里读回本机记录，只做一次。
//
// 内存表是渲染时的唯一真相（每帧去问一次缓存没必要）；`ConfigGet` 只是它的来源。
func (p *Plugin) seedFeedback(act pluginapi.Actions) {
	if p.feedbackSeeded {
		return
	}
	p.feedbackSeeded = true
	if p.feedback == nil {
		p.feedback = map[string]string{}
	}
	if act.ConfigGet == nil {
		// 壳没接配置通道：这一进程里只存内存，重启即失（见文件头）。
		return
	}
	for k, v := range act.ConfigGet() {
		if !strings.HasPrefix(k, feedbackKeyPrefix) {
			continue
		}
		// 只认两个合法值：空串是墓碑，别的字符串不是我们写的，不猜它的意思。
		if v == feedbackUp || v == feedbackDown {
			p.feedback[k] = v
		}
	}
}

// feedbackRating 返回这条消息当前的评分（"" = 没记录）。
func (p *Plugin) feedbackRating(session string, index int) string {
	return p.feedback[feedbackKey(session, index)]
}

// toggleFeedback 处理一次点击：同值再点 = 撤回，否则改成新值。
//
// 官方也是这个语义（"Clicking the recorded rating retracts it"）。撤回要写墓碑
// 空串，不能把旧评分留在配置里冒充"已记录"。
func (p *Plugin) toggleFeedback(act pluginapi.Actions, session string, index int, rating string) {
	if session == "" {
		// 没有会话就没有可归属的记录：宁可什么都不写，也不造一个空 session 键。
		return
	}
	if p.feedback == nil {
		// 正常路径上 drawFeedback 已经 seeding 过；这里兜底，免得调用方
		// 先 toggle 后 draw 时往 nil map 里写（一个 panic 换一个 if，值）。
		p.feedback = map[string]string{}
	}
	key := feedbackKey(session, index)
	next := rating
	if p.feedback[key] == rating {
		next = ""
	}
	if next == "" {
		delete(p.feedback, key)
	} else {
		p.feedback[key] = next
	}
	// 写失败由壳报（它的 ConfigSet 失败会写状态栏，插件不等待也不重试）：
	// 这一帧的图标该显示成实心——点击确实发生在本机界面里；
	// 落盘是另一件事，不拿"已记录"去假装它是证据。
	if act.ConfigSet != nil {
		act.ConfigSet(map[string]string{key: next})
	}
}

// drawFeedback 画 👍/👎 这一对，位置在"复制"与"重试"之间（官方 IconActions
// 的次序就是 copy → feedback → branch）。
//
// 用自绘命中区而不是 f.Button：按钮的标签只能是一段文字，而官方这里是没有文字的
// 15px 图标，且要用空心/实心表达评分状态（viva 的按钮没有按状态改标签颜色的口子）。
// 为了和同一排的文字按钮对齐，命中区按 Ghost/Small 的盒高 28、radius sm、
// 悬停底色与之一致（t.Hover == viva 主题的 Muted，Ghost 按钮悬停用的就是它）。
// 颜色取 t.LabelPrimary：官方这里是 label-tertiary，但本排的"复制/重试"是
// Ghost 按钮（前景 = t.LabelPrimary），同一排用同一个颜色才不会一半深一半浅。
func (p *Plugin) drawFeedback(f *flex.Frame, env *pluginapi.Env, index int) {
	t := p.theme
	session := env.Data.Cur
	if session == "" {
		// 没有会话就没有可归属的记录：不画一对点了也不知道记到哪去的按钮。
		return
	}
	p.seedFeedback(env.Act)
	rating := p.feedbackRating(session, index)

	button := func(up bool, value string) {
		active := rating == value
		// 文案逐字取官方 zh 资源：action.like「好的回答」、action.dislike
		// 「有问题的回答」、两者 active 都是「取消标记」。没有 tooltip 原语，
		// 它同时是命中区的无障碍标签（读屏与测试都靠它定位）。
		label := "有问题的回答"
		if up {
			label = "好的回答"
		}
		if active {
			label = "取消标记"
		}
		id := env.Tree.ID(env.Feat, "feedback-"+value+"-"+itoa(index))
		if f.Row().Width(feedbackHitSize).Height(feedbackHitSize).
			Radius(t.Radii.SM).Items(ui.Center).Justify(ui.Center).
			Hover(ui.Look{}.Background(t.Hover)).Hit(id, label).Children(func() {
			f.Icon(thumbIcon(up, active)).Width(feedbackIconSize).
				Height(feedbackIconSize).Color(t.LabelPrimary)
		}).Clicked() {
			p.toggleFeedback(env.Act, session, index, value)
		}
	}
	button(true, feedbackUp)
	button(false, feedbackDown)
}

// thumbIcon 是 16 viewBox 的拇指字形：`filled` 决定描边（空心）还是填充（实心）。
//
// 官方用 IconLikeOutlineRegular / IconLikeFillRegular 这两枚图标表达"没评/已评"，
// 颜色不变。这里用同一份路径的两种画法达到同样的读法——不引入彩色 emoji：
// 一排单色次要按钮里突然两个黄色 emoji，不是同一个视觉语言。
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
//
// 两条独立子路径（和 Feather 那类线性图标的做法一致）：掌根是圆角矩形，
// 手背从掌根右侧起笔，拇指向上抬起并在指尖收圆，指背落回手背顶边。
// 路径同时用于描边与填充，所以两种状态的法线一致，切换时不会"跳形"。
func thumbCommands() []gfx.PathCommand {
	return []gfx.PathCommand{
		// 掌根（左侧圆角矩形）。
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

		// 手背 + 拇指。
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
