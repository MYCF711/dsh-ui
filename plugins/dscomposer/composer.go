// Package dscomposer 是「输入区」这个官方部件的移植。
//
// 官方对应物是 `@deepseek-ai/dsh-client-ui-conversation`（不是 ui-chat）：
//
//	InputBar.module.css   .GRgSLq_card     输入框那张大圆角卡片
//	                      .GRgSLq_input    可编辑区（active 36 / hero 52，padding 4/8/0/14）
//	                      .GRgSLq_row      底部控件行（padding 2/8/6、space-between）
//	                      .GRgSLq_add      命令键 28×28 圆形（无真实命令动作时不画）
//	                      ModelSelect / PermissionSelect trigger（高 28、radius sm、13px）
//	                      .GRgSLq_primary  发送键 34×34 圆形、button-info-fill、上移 2px
//
// 变量链（同包 ConversationRoot.module.css + editor 侧的 JS）：
//
//	--dsh-chat-content-width: clamp(680px, var(--dsh-conversation-column-width) * .64, 920px)
//	--dsh-composer-card-max-width: calc(<上一条> + 32px)
//	--dsh-composer-side-clearance: 16px
//	--dsh-composer-text-max-height: 336px
//
// 上面每个数值都在下面 const 块里注了出处，**没有一个是凭记忆写的**。
//
// 两条硬规矩（为此宁可少画一块）：
//
//  1. 模型名 / 审批策略只显示宿主真值（Snapshot.Model / Snapshot.Permission）。
//     空串 = 宿主没说 → 那一枚整段不画，绝不拿会话 mode 或自编默认值顶替。
//  2. 宿主没给动作的控件不画。官方 `.GRgSLq_add` 是命令键，附件还有独立文件入口；
//     pluginapi.Actions 两类入口都没有，画出来就是死按钮——所以这里**不画**，缺口写在报告里。
//
// 它还画审批条（见 approval.go）：待审批的工具调用画在输入框正上方，
// 因为这里就是"用户该做决定的地方"。
package dscomposer

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// 官方 composer 的几何常量。每一条后面都跟着原始 CSS，改之前先回去读那一行。
const (
	// .GRgSLq_root{padding:0 var(--dsh-composer-side-clearance) 4px}，clearance=16。
	composerClearance = 16
	// 壳（plugins/dsshell drawMain）给中栏的是 Pad(12)：composer 只补差值 4，
	// 净边距才等于官方的 16。壳那边一改这里要跟着改（同一个数值的第二个出处）。
	shellMainPad = 12

	// .GRgSLq_card{padding-top:8px;gap:12px;border-radius:var(--dsw-radius-panel)}
	cardPadTop = 8
	cardGap    = 12

	// .GRgSLq_input{min-height:36px;padding:4px 8px 0 14px}
	// 另一条 `.GRgSLq_hero .GRgSLq_input{min-height:52px}` **只在首屏（还没有会话）时**
	// 生效——它是后代选择器，不是"后面的规则覆盖前面的"。两种状态差 16px 高度，
	// 常驻取 52 会让有会话时的卡片高从 98 变成 114（实测 tmp/v3-1920-hero.png 对照）。
	inputMinHeight     = 36
	heroInputMinHeight = 52
	inputPadLeft       = 14
	inputPadRight      = 8

	// .GRgSLq_row{padding:2px 8px 6px;gap:12px;justify-content:space-between;align-items:center}
	rowPadTop    = 2
	rowPadSide   = 8
	rowPadBottom = 6

	// .GRgSLq_primary{width:34px;height:34px;border-radius:999px;transform:translateY(-2px)}
	sendSize = 34
	sendLift = 2

	// 两枚 trigger（ModelSelect ._9r0rkG_trigger / PermissionSelect ._86kJGG_trigger）：
	// height:28px、radius sm、padding:0 4px 0 8px、gap:4px、13px、颜色 label-secondary；
	// 模型 max-width:min(360px,45cqw) 且 font-weight:400，权限 max-width:220px 且 500。
	// 它们是**两个**组件、两份 CSS，不是同一个 .GRgSLq_select（那条是别的场景）。
	chipHeight       = 28
	chipPadLeft      = 8
	chipPadRight     = 8 // 只读态不画 chevron；保留 8px 右侧留白，不画 4+12 的假箭头位
	chipGap          = 4
	permissionIconWH = 14
	permissionChipW  = 220
	modelChipMaxW    = 360
	modelCardRatio   = 0.45
	primaryIconWH    = 16
	// .GRgSLq_pending{width:8px;height:8px;border-radius:50%}
	busyDotSize = 8

	// _9r0rkG_trigger{font-weight:400} / _86kJGG_trigger{font-weight:500}
	modelChipWeight      = ui.WeightRegular
	permissionChipWeight = ui.WeightMedium

	// --dsh-chat-content-width: clamp(680px, column * .64, 920px)
	// --dsh-composer-card-max-width: calc(content + 32px)
	contentMin   = 680
	contentMax   = 920
	contentRatio = 0.64
	cardExtra    = 32
	// 下方兜底：官方没有这一档，是中栏被挤到极窄时"至少还放得下一行字"的下限。
	cardMin = 240

	// viva 的 ui.Metrics.TextInsetX() = PadControlX-1 = 11：Field 自己从左边 11px
	// 起写字，并且强制自绘 1px 描边 + 底色（PaintInput，改不了；见 docs/official-specs.md §3
	// 同一处 viva 限制）。所以：左边只留 14-11，AfterDraw 再把 Field 表面统一成 InputMajor。
	fieldTextInset = 11
	// Field 高 32 时 inputLineBand = (32-min(24,24))/2 = 4 → 文字带 4..28，
	// 正好等于官方 padding-top:4 + line-height:24。高 36 就会变成 6..30（偏 2px）。
	fieldHeight = 32
)

// Plugin 实现 pluginapi.Plugin / Panels / Fields / KeyHandler。
type composerControlBoxes struct {
	Permission flex.Box
	Model      flex.Box
	Primary    flex.Box
}

type Plugin struct {
	theme *pluginapi.Theme
	// field 是值类型：用 &p.field 取地址。写成 *flex.Field 又忘了 new，
	// Fields() 就会返回一个 nil 元素，flex.Fields(...) 里直接 panic（踩过）。
	prims    pluginapi.Primitives
	field    flex.Field
	controls composerControlBoxes // 本帧真实布局矩形，供几何验收；零值矩形 = 控件未画

	// lastData / lastAct 是绘制期快照，按键时要用。
	// HandleKey 没有 env 参数，只能记住上一帧（与 ds-conversation 同一做法）。
	// 只读、不跨帧持有可变状态：壳每帧换一份新快照。
	lastData pluginapi.Snapshot
	lastAct  pluginapi.Actions

	// fieldBox 是本帧输入区的盒子。Draw 阶段量不到矩形（viva 的 Box.Rect 要等
	// End 之后才有效），所以把 Box 存下来，到 AfterDraw 再读——那一段跑在
	// End 之后、下一帧 Begin 之前，正是它有效的窗口（与 ds-conversation 同一做法）。
	fieldBox flex.Box
	// fieldBoxOn 表示这一帧真的画过输入区（没有会话/没装插件时不要拿旧矩形乱盖）。
	fieldBoxOn bool
	// fieldTheme 是 Draw 时 Viva 实际使用的控件主题；AfterDraw 用它重绘同一 Field，
	// 只把表面色改成 InputMajor，文字、光标、选区和 IME 状态全部保持原路径。
	fieldTheme ui.Theme

	// logf 是插件的日志出口（注册期拿到的 Context.Logf）。
	// 只用来记"用户点了哪一项"，一行一次，不进绘制热路径。
	logf func(format string, args ...any)

	// mentionRows / mentionRef 是上一帧算出来的 `@` 候选（见 mention.go）。
	//
	// 为什么按键路径要靠上一帧：HandleKey 没有 env，而候选要靠 FSList（在 Actions 里）。
	// 和 lastData / lastAct 同一个做法——只读上一帧的结果，不自己攒状态。
	// 空 = 没有菜单，Enter 必须原样让回去（不然消息发不出去）。
	mentionRows []pluginapi.FSEntry
	mentionRef  atRef
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-composer" }

// Provides 实现 pluginapi.Plugin。
func (*Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Register 实现 pluginapi.Plugin。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	p.logf = ctx.Logf
	return nil
}

// Fields 实现 pluginapi.Fields：壳靠它把输入框登记给 viva。
func (p *Plugin) Fields() []*flex.Field { return []*flex.Field{&p.field} }

// Panels 实现 pluginapi.Panels。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "composer", Slot: pluginapi.SlotMainComposer, Order: 10,
			Draw: p.draw, AfterDraw: p.afterDraw},
	}
}

// draw 画一列：审批条（有才画）+ 输入卡片。
//
// 位置照官方 .GRgSLq_root：左右各留 --dsh-composer-side-clearance(16)。壳已经给
// 中栏 12 的 padding，所以这里只补差值——补整 16 会让卡片比官方窄 8。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	p.lastData = env.Data
	p.lastAct = env.Act
	// 中栏内容宽度：壳算好的 Geometry.MainWidth 优先，退到 Env.MainWidth。
	column := env.Geometry.MainWidth
	if column <= 0 {
		column = env.MainWidth
	}
	extra := float32(composerClearance - shellMainPad)
	if extra < 0 {
		extra = 0
	}
	// ConversationRoot .composerStack 的 --dsh-composer-stack-gap 是 6px。
	f.Column().FillWidth().PadX(extra).Gap(6).Children(func() {
		// 审批条：Approval == nil 时这里什么都不画（不留空框）。
		p.drawApproval(f, env)
		// `@` 候选菜单：位置在输入卡片**上方**，和 GoalBar / 问题条同一条约定
		// （越靠近输入框的东西越紧急）。没有候选就整块不画。
		p.drawMention(f, env, column)
		p.drawCard(f, env, column)
	})
}

// drawCard 画输入卡片：一个居中、定宽、圆角 panel(28) 的面板。
//
// 官方的 max-width 是「会话栏宽 * .64 收在 680…920，再加 32」，不是百分比——
// 所以这里必须先算出像素宽度再定宽，交给 flex 的 MaxWidth 只会按整窗解析。
func (p *Plugin) drawCard(f *flex.Frame, env *pluginapi.Env, column float32) {
	t := p.theme
	w := cardWidth(column)
	// 这里**不加**底 padding：官方 `.GRgSLq_root` 的 4px 是相对**窗口底部**的，
	// 而壳已经给中栏 12px padding——再加 4 会把卡片整体顶上 4px，
	// 卡片底距窗口底就从 8px 变成 12px（正好顶在 ≤8px 这条线的外面）。
	// 卡片自身 border:0，柔和的 elevation 由 Shadow 表达，不能塞一圈占布局的 Border
	//（否则宽、高和圆角各偏 1–2px）。
	f.Row().FillWidth().Justify(flex.Center).Children(func() {
		f.Column().Width(w).Radius(t.Radii.Panel).Background(t.InputMajor).
			Shadow(ui.ShadowXS).
			PadSides(cardPadTop, 0, 0, 0).Gap(cardGap).Children(func() {
			p.drawInput(f, env, w)
			p.drawControls(f, env, w)
		})
	})
}

// drawInput 画可编辑区那一行。
//
// 官方 `.input` 是 contenteditable，铺满卡片宽度、无底无边；我们用的是 viva 的
// Field，它**强制**自绘底色 + 1px 描边（PaintInput → paintTextEntrySurface）。
// AfterDraw 用同一 InputSnapshot 把它的表面重绘成 InputMajor，于是"框里套框"消失。
func (p *Plugin) drawInput(f *flex.Frame, env *pluginapi.Env, cardW float32) {
	t := p.theme
	// 文字要从卡片外沿 +14 起：Field 自己缩进 11，所以外层左边补 3。
	left := float32(inputPadLeft - fieldTextInset)
	// 右边缘落在卡片外沿 -8（官方 padding-right:8）。
	fieldW := cardW - inputPadLeft + fieldTextInset - inputPadRight
	// 首屏（没有会话）用官方 hero 档 52，有会话时是 36。
	inputH := float32(inputMinHeight)
	if env.Data.Cur == "" {
		inputH = heroInputMinHeight
	}
	f.Row().FillWidth().Height(inputH).PadSides(0, 0, 0, left).
		Items(flex.Start).Children(func() {
		field := f.Field(&p.field, env.Tree.ID(env.Feat, "composer"), "输入", placeholderText).
			Width(fieldW).Height(fieldHeight).FontSize(t.Fonts.Body)
		p.fieldBox, p.fieldBoxOn = field, true
		p.fieldTheme = f.Theme()
		if field.Submitted() {
			p.submit(env)
		}
	})
}

// placeholderText 是空输入时的提示。
//
// 官方把 "Enter 发送" 用 ::after 接在**正文末尾**（--dsh-composer-hint），viva 的
// Field 只有 placeholder 一个位置，所以两者合并写在这里——不然这条提示在单行
// 输入框里根本没地方放。
const placeholderText = "说点什么…（Enter 发送）"

// afterDraw 用同一个 InputSnapshot 重绘 Field 的表面。
//
// Viva 的 PaintInput 固定给 Field 加底色与 1px 边框；只盖边框会留下比 InputMajor
// 更暗的一块内框。这里保留原控件主题中的文字/光标/选区参数，只把背景、边框和
// 焦点边框统一成 InputMajor，再调用同一个 components.PaintInput。交互仍由 Draw 阶段
// 登记的 flex.Field 负责，因此输入、Enter、IME 和点击行为都没有第二条实现。
func (p *Plugin) afterDraw(canvas *ui.Canvas, env *pluginapi.Env) {
	if !p.fieldBoxOn {
		return
	}
	r := p.fieldBox.Rect()
	if r.Width <= 0 || r.Height <= 0 {
		return
	}
	p.fieldBoxOn = false // 只在画过的那一帧盖一次；下一帧会重新量
	id := env.Tree.ID(env.Feat, "composer")
	value := p.field.Text()
	focused := canvas.Focused() == id
	theme := p.fieldTheme
	theme.Background = p.theme.InputMajor
	theme.Input = ui.Color{}
	theme.Primary = p.theme.InputMajor
	theme.Danger = p.theme.InputMajor
	components.PaintInput(canvas, theme, r, components.InputSnapshot{
		ID: id, Label: "输入", Value: value, Placeholder: placeholderText,
		Secure: p.field.Secure, Focused: focused,
		PlatformEditing: focused && value != "",
		ShowCaret:       focused && value == "",
	})
}

// drawControls 按官方层级画底部控件行：permission 在左 tools；model、activity、
// primary 依次在右 trailing。命令/附件没有真实 Action，不画加号；model/permission
// 只有当前真值、没有目录与切换 Action，因此都是只读文本而不是假下拉。
func (p *Plugin) drawControls(f *flex.Frame, env *pluginapi.Env, cardW float32) {
	p.controls = composerControlBoxes{}
	model, permission := hostLabels(env.Data)
	status := activityLabel(env.Data)
	running := env.Data.Stream != "" || env.Data.Busy
	t := p.theme
	// 官方 `.GRgSLq_primary{transform:translateY(-2px)}`：viva 的 MarginY 不吃负值，
	// 这里用**同样的视觉位移**实现——行高不变（2+34+6 == 0+34+8），把 2px 从
	// 上内边距挪到下内边距，发送键就整体上移 2px，底距卡片底 = 6+2 = 8（官方值）。
	f.Row().FillWidth().Height(rowPadTop+sendSize+rowPadBottom).
		PadSides(rowPadTop-sendLift, rowPadSide, rowPadBottom+sendLift, rowPadSide).
		Items(flex.Center).Justify(flex.Between).Gap(cardGap).Children(func() {
		// 官方 DOM 的左 modes 只有 permission/plan；model 属于右 standardControls。
		if permission != "" {
			icon, hasIcon := permissionIconFor(strings.TrimSpace(env.Data.Permission), t.LabelSecondary)
			p.controls.Permission = p.hostChip(f, permission, permissionChipWeight, permissionChipW, icon, hasIcon)
		}
		// 即使左边为空，Spacer 也把右侧 trailing 固定到卡片右缘。
		f.Spacer()
		f.Row().Items(flex.Center).Gap(cardGap).Children(func() {
			if model != "" {
				maxW := cardW * modelCardRatio
				if maxW > modelChipMaxW {
					maxW = modelChipMaxW
				}
				p.controls.Model = p.hostChip(f, model, modelChipWeight, maxW, ui.Icon{}, false)
			}
			if status != "" {
				p.hostChip(f, status, modelChipWeight, permissionChipW, ui.Icon{}, false)
			}
			if running {
				p.busyDot(f)
			}
			// 保留原有发送/停止判据：Stream 或 Busy 任一成立时主按钮只走 Cancel。
			p.sendButton(f, env, running)
		})
	})
}

// activityLabel 是 activity 槽的文本。优先原样显示壳/宿主给的 Status；只有 turn
// 在跑而状态文本为空时才由 Busy/Stream 推导"正在生成"。完全未知就整段隐藏。
func activityLabel(s pluginapi.Snapshot) string {
	if status := strings.TrimSpace(s.Status); status != "" {
		return status
	}
	if s.Busy || s.Stream != "" {
		return "正在生成"
	}
	return ""
}

// hostChip 画一枚只读的宿主真值标签。
//
// 几何照官方两个 trigger（它们不是同一份 CSS）：
//
//	ModelSelect      ._9r0rkG_trigger{height:28px;border-radius:var(--dsw-radius-sm);
//	                 padding:0 4px 0 8px;gap:4px;font:400 13px/20px;color:label-secondary;
//	                 max-width:min(360px,45cqw)}
//	PermissionSelect ._86kJGG_trigger{height:28px;border-radius:var(--dsw-radius-sm);
//	                 padding:0 4px 0 8px;gap:4px;font:500 13px/20px;color:label-secondary;
//	                 max-width:220px}
//
// **不接点击、不画 chevron、没有 hover 底**：宿主只给了"现在是哪一个"，既没有候选
// 列表也没有切换动作，画成能点的样子就是骗人（更别说那个下拉箭头）。只有官方
// ModelSelect 的 catalog + selection mutation 都存在时，才可另实现按钮与 chevron；
// 当前 pluginapi 没有这两条契约，所以这里始终是只读真值。
func (p *Plugin) hostChip(f *flex.Frame, label string, weight, maxW float32, icon ui.Icon, hasIcon bool) flex.Box {
	t := p.theme
	textMax := maxW - chipPadLeft - chipPadRight
	if hasIcon {
		textMax -= permissionIconWH + chipGap
	}
	if textMax < 0 {
		textMax = 0
	}
	return f.Row().Height(chipHeight).MaxWidth(maxW).Radius(t.Radii.SM).
		PadSides(0, chipPadRight, 0, chipPadLeft).Items(flex.Center).Gap(chipGap).Children(func() {
		if hasIcon {
			f.Icon(icon).Size(permissionIconWH, permissionIconWH).Color(t.LabelSecondary)
		}
		f.Row().MaxWidth(textMax).Children(func() {
			f.Text(label).FontSize(t.Fonts.Secondary).LineHeight(20).Weight(weight).
				Color(t.LabelSecondary).Ellipsis()
		})
	})
}

// busyDot 画官方 `.GRgSLq_pending` 那个 8px 圆点：宿主说 turn 在跑才有。
//
// 官方 `.pending{width:8px;height:8px;border-radius:50%;background:var(--dsw-alias-state-business-primary)}`
// （还有一条 1s 明暗动画）。这里不抖：无头截图拍不到动画，反复重绘只会让整窗脏区
// 变大；"在跑"这件事已经由发送键变■说清楚了。
func (p *Plugin) busyDot(f *flex.Frame) {
	f.Row().Size(busyDotSize, busyDotSize).Radius(ui.RadiusFull).
		Background(p.theme.Info).Children(func() {})
}

// sendButton 画 34px 的圆形发送/停止键（官方 `.GRgSLq_primary`）。
//
// 官方 fill = --dsw-alias-button-info-fill = deepseek-400/500，正好是主题里的 Info
// （深色 deepseek-400、浅色 deepseek-500，逐档一致）；前景色官方写死 #fff，
// 我们没有 "on-info" 令牌，取对比度最高的那一档，见 onInfo()。
// 图标是官方 16 viewBox 的填充路径，不再依赖 ↑/■ 的字体字形。
func (p *Plugin) sendButton(f *flex.Frame, env *pluginapi.Env, running bool) {
	t := p.theme
	id := env.Tree.ID(env.Feat, "send")
	icon, label := primarySendIcon, "发送"
	disabled := strings.TrimSpace(p.field.Text()) == "" || env.Act.Send == nil
	if running {
		id = env.Tree.ID(env.Feat, "stop")
		icon, label = primaryStopIcon, "停止"
		disabled = env.Act.Cancel == nil
	}
	fill, foreground := t.Info, onInfo(t)
	if disabled {
		// .GRgSLq_primary:disabled{opacity:.4}；底色和矢量一起降 alpha。
		fill.Alpha *= 0.4
		foreground.Alpha *= 0.4
	}
	button := f.Row().Size(sendSize, sendSize).Radius(ui.RadiusFull).Background(fill).
		Items(flex.Center).Justify(flex.Center).MarginY(-sendLift)
	if !disabled {
		button = button.HoverBackground(t.Info)
	}
	p.controls.Primary = button
	clicked := button.HitWith(id, label, ui.HitOptions{
		Role: ui.RoleButton, Disabled: disabled,
		// 官方 onMouseDown=keepDraftFocus；鼠标动作不抢编辑器/IME 焦点。
		// 键盘仍走输入框 Enter 发送、Esc 停止，审批按钮保持自己的焦点语义。
		SkipFocus: true,
	}).Children(func() {
		f.Icon(icon).Size(primaryIconWH, primaryIconWH).Color(foreground)
	}).Clicked()
	if clicked && !disabled {
		p.clickSend(env, running)
	}
}

// clickSend 是发送键的唯一出口：真发（Send）或真停（Cancel）。动作没接就返回 false。
func (p *Plugin) clickSend(env *pluginapi.Env, running bool) bool {
	if running {
		if env.Act.Cancel == nil {
			return false
		}
		env.Act.Cancel()
		return true
	}
	return p.submit(env)
}

// submit 把非空文本交给壳，然后清空自己。
//
// 空文本不提交；Send 没接时既不调动作也不清草稿，避免一次无效点击把输入弄丢。
func (p *Plugin) submit(env *pluginapi.Env) bool {
	text := strings.TrimSpace(p.field.Text())
	if text == "" || env.Act.Send == nil {
		return false
	}
	env.Act.Send(text)
	p.field.SetText("")
	return true
}

// composerColumn 取本帧主列宽度；审批与输入卡必须共用同一个官方 column 语义。
func composerColumn(env *pluginapi.Env) float32 {
	if env.Geometry.MainWidth > 0 {
		return env.Geometry.MainWidth
	}
	return env.MainWidth
}

// contentWidth 是官方 --dsh-chat-content-width 的取值：clamp(680, column*.64, 920)。
func contentWidth(column float32) float32 {
	w := column * contentRatio
	if w < contentMin || column <= 0 {
		return contentMin
	}
	if w > contentMax {
		return contentMax
	}
	return w
}

// cardWidth 是官方卡片宽度：`width:100%; max-width: content+32`，
// 而 100% 是「中栏 - 两侧 clearance」。
func cardWidth(column float32) float32 {
	w := contentWidth(column) + cardExtra
	if avail := column - 2*composerClearance; avail > 0 && w > avail {
		w = avail
	}
	if w < cardMin {
		w = cardMin
	}
	return w
}

// hostLabels 把宿主真值翻成两枚标签的文字。空串 = 宿主没说 = 这一枚不画。
//
// 官方 preset 的中文产品名逐字来自 permission-presets/locales。当前宿主 H5 给的
// 却是进程 policy（deny/ask/allow/auto），不是 preset：allow 表示审批闸全部自动放行，
// 对应官方“完全权限”的可读文案；ask/deny 没有官方 preset 等价物，因此使用明确的
// 审批语义“逐次询问/全部拒绝”，不把它们冒充“工作区内修改/仅可查看”。
// host auto 是“终端询问、非终端拒绝”（cmd/dshd makeAskFunc），不是官方 auto-review，
// 因而写“自动审批”，不写“自动审查”。未知非空值原样只读，不拿 Mode 顶替。
func hostLabels(s pluginapi.Snapshot) (model, permission string) {
	model = strings.TrimSpace(s.Model)
	switch value := strings.TrimSpace(s.Permission); value {
	case "":
	case "read-only":
		permission = "仅可查看"
	case "workspace-write":
		permission = "工作区内修改"
	case "danger-full-access", "allow":
		permission = "完全权限"
	case "ask":
		permission = "逐次询问"
	case "deny":
		permission = "全部拒绝"
	case "auto":
		permission = "自动审批"
	default:
		permission = value
	}
	return model, permission
}

// onInfo 是蓝底（button-info-fill）上的前景色。
//
// 官方 `.GRgSLq_primary{color:#fff}`。我们没有 "on-info" 令牌，也不许在插件里
// 写十六进制，所以取与那块蓝对比最大的既有令牌：深色主题用 LabelPrimary
// （bluish-50，接近白），浅色主题用 BgBase（bluish-00，就是纯白）。
func onInfo(t *pluginapi.Theme) ui.Color {
	if t.Dark {
		return t.LabelPrimary
	}
	return t.BgBase
}

// HandleKey 实现 pluginapi.KeyHandler：**只在审批条可见时**接管两个键。
//
//	Enter  允许（回传宿主给的 allow 项）
//	Esc    拒绝（同理）
//
// 审批条不在（Approval == nil）就直接返回 false —— 输入框的 Enter 发送
// 不能被抢掉：viva 的分发顺序是「先 OnKey → 没被处理才交给聚焦控件」，
// 这里返回 true 就等于把 Enter 从 TextInput 手里拿走，所以门槛必须卡在
// approvalVisible 上（见 approval_test.go 的 TestHandleKeyTakesEnterOnlyWhenVisible）。
func (p *Plugin) HandleKey(ev ui.KeyEvent) bool {
	if ev.Release || ev.Modifiers != 0 {
		return false
	}
	if !approvalVisible(p.lastData) {
		// `@` 候选菜单开着时，Enter 是**补全**，不是发送（见 mention.go）。
		//
		// 门禁卡死在"这一帧确实有候选"上：没有候选就原样让回去。
		// 让回去的代价是零；吃掉的代价是**这条消息再也发不出去**，
		// 而且用户看到的是"我按了 Enter 什么都没发生"——没有任何提示。
		if ev.Key == ui.KeyEnter && p.acceptMention() {
			return true
		}
		// 没有审批时 Esc 退而求其次 = **停止当前 turn**（官方也是这个直觉）。
		// 不做这一条的话，无头环境里根本没法触发停止键——它是个按钮，
		// 注入不了鼠标事件；而"能停"是产品级链路的必选项（宿主 §0.6）。
		if ev.Key == ui.KeyEscape && p.lastAct.Cancel != nil {
			// 不判 Stream/Busy：这两个字段在"只有思考流"或快照延迟时都不可靠
			// （实测 turn 明明在跑，Busy 却还是 false）。宿主对没有 turn 在跑的
			// cancel 是幂等的（回 cancelled=true），多按一次不会出错。
			p.lastAct.Cancel()
			return true
		}
		return false
	}
	plan := planApproval(p.lastData, p.lastAct.Approve != nil)
	switch ev.Key {
	case ui.KeyEnter:
		if !plan.Allow.Usable {
			return false
		}
		p.decideKey(true)
		return true
	case ui.KeyEscape:
		if !plan.Deny.Usable {
			return false
		}
		p.decideKey(false)
		return true
	}
	return false
}

// decideKey 是按键路径的出入口：与按钮走同一个 decide()，只是没有 env。
func (p *Plugin) decideKey(allow bool) {
	ap := p.lastData.Approval
	if !decide(ap, p.lastAct, allow) {
		return
	}
	if p.logf != nil {
		opt, _ := chooseOption(ap, allow)
		verdict := "拒绝"
		if allow {
			verdict = "允许"
		}
		p.logf("审批（键盘）：%s → %s", verdict, opt.OptionID)
	}
}

var (
	_ pluginapi.Plugin     = (*Plugin)(nil)
	_ pluginapi.Panels     = (*Plugin)(nil)
	_ pluginapi.Fields     = (*Plugin)(nil)
	_ pluginapi.KeyHandler = (*Plugin)(nil)
)
