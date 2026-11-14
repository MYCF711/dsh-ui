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
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
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
	modelChipWeight      = gfx.WeightRegular
	permissionChipWeight = gfx.WeightMedium

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
	Permission ui.Box
	Model      ui.Box
	Primary    ui.Box
}

type Plugin struct {
	theme *pluginapi.Theme
	// field 是值类型：用 &p.field 取地址。写成 *flex.Field 又忘了 new，
	// Fields() 就会返回一个 nil 元素，flex.Fields(...) 里直接 panic（踩过）。
	prims    pluginapi.Primitives
	field    ui.Field
	controls composerControlBoxes // 本帧真实布局矩形，供几何验收；零值矩形 = 控件未画

	// lastData / lastAct 是绘制期快照，按键时要用。
	// HandleKey 没有 env 参数，只能记住上一帧（与 ds-conversation 同一做法）。
	// 只读、不跨帧持有可变状态：壳每帧换一份新快照。
	lastData pluginapi.Snapshot
	lastAct  pluginapi.Actions

	// fieldBox 是本帧输入区的盒子。Draw 阶段量不到矩形（viva 的 Box.Rect 要等
	// End 之后才有效），所以把 Box 存下来，到 AfterDraw 再读——那一段跑在
	// End 之后、下一帧 Begin 之前，正是它有效的窗口（与 ds-conversation 同一做法）。
	fieldBox ui.Box
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
	// triggerRows / triggerHit 是统一输入触发管道（`@` 文件/会话、`/` 技能）
	// 上一帧算出来的扁平候选，见 trigger.go。按键路径同样只读上一帧。
	triggerRows  []triggerCandidate
	triggerHit   triggerHit
	triggerIndex int
	// suppressed 是"刚 pick 完的那份草稿"：同一份文本不再触发菜单，
	// 否则补出来的 `@path` 会立刻被当成新 token 再弹一次（永远关不掉）。
	// 用户一改文本就失效。
	suppressed string
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
func (p *Plugin) Fields() []*ui.Field { return []*ui.Field{&p.field} }

// Panels 实现 pluginapi.Panels。
//
// # 两个座位（同一帧只有一个活）
//
//	main.composer          旧三栏（`-shell legacy`）/ 纯旧插件集的**兼容桥**
//	conversation.composer  官方 `InputBar`（rc.2 终态，`uikernel.OffConvComposer`）
//
// 为什么两个都挂：壳按 `ShellOptions.BridgeLegacyComposer` **一帧一个值**决定
// dispatch 哪一边（`uikernel/shell_skeleton.go` 的 `drawComposerSeat`）。
// 判据是"官方 composer 座位有没有占用方"——本插件注册了官方座位，所以在
// `-shell official` 下**官方座位真的被 dispatch**（GP 报告 §5 第 1 条），旧槽
// 这一帧没人画。反过来 `-shell legacy`（dsshell 三栏）只 dispatch 旧槽，
// 官方座位没人画。两个座位互斥，**不会双画** —— 与 transcript 的双座位同一条判据。
//
// 输入框本体（`Fields()` 登记的 `flex.Field`）两个座位共用一份：`drawInput` 在
// 两个 Draw 里都调用它，但只有活的那个座位会执行。
//
// 审批卡的座位归属：官方审批走 **composer 接管**（`ChatView.tsx:293-295`
// 的注释：approvals take over the composer），不是消息流里的一段。所以两个
// 座位上都只有 **composer 侧**画审批卡，transcript 侧一条都不画
// （dsconversation 的 Panels 注释写了为什么）。
// 两个座位都是**父容器**：全部内容在 `Children` 里画，因为官方
// `conversation.input.*` 子槽只有在那里才能拿到内核的 `drawSlot`（见
// input_slots.go 的文件头）。`Draw` 留空是有意的，不是漏写：viva 的
// `Painter.Draw` 拿不到 `drawSlot`，而内核的装配校验只要求「Draw 或 AfterDraw
// 有一个」——这里的 AfterDraw 是真实的（重绘 Field 表面），所以两半都是实事。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "composer", Slot: pluginapi.SlotMainComposer, Order: 10,
			Draw: p.draw, AfterDraw: p.afterDraw},
		// 官方座位（rc.2 终态）：`conversation.composer` 是 **chain** 槽，
		// 占用方接管整个 composer 区（审批卡 overlay + InputBar fallback）。
		// 没有审批请求时它是**输入卡片本体**，不是"只画审批卡"——GP 报告 §5
		// 第 2 条记的就是这个缺口。
		{ID: "composer-official", Slot: pluginapi.Slot(offConversationComposer), Order: 10,
			AfterDraw: p.afterDraw, Children: p.drawOfficialComposerChildren},
	}
}

// offConversationComposer 是官方 composer 链的槽位字面量
// （`ui-conversation/src/client/contract/slots.ts:290`，`uikernel.OffConvComposer`
// 的同一个值）。**不许改写成 main.composer 之类的旧槽名**：那就又回到兼容桥了。
const offConversationComposer = "conversation.composer"

// draw 画一列：审批条（有才画）+ 输入卡片。
//
// **只给单测/调试直调用**：生产路径一律走 `Panels()` 的 `Children`
// （`drawLegacyComposerChildren` / `drawOfficialComposerChildren`），那里才有
// 内核的 `drawSlot`。这里传 nil 等于"没有子槽占用方"，所有位置走只读回退。
//
// 位置照官方 .GRgSLq_root：左右各留 --dsh-composer-side-clearance(16)。壳已经给
// 中栏 12 的 padding，所以这里只补差值——补整 16 会让卡片比官方窄 8。
func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
	p.drawLegacyComposerChildren(f, env, nil)
}

// drawLegacyComposerChildren 画旧三栏座位（`main.composer`）。
//
// 与官方座位的差别只有外壳：旧三栏的中栏已经有 12px 内边距（dsshell 的
// `columnPad`），本插件只补到官方的 16；dock（上下文读数）仍只在官方座位画，
// 保持旧外观不变。卡片、控件行、`conversation.input.*` 派发两侧共用。
func (p *Plugin) drawLegacyComposerChildren(f ui.C, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	p.lastData = env.Data
	p.lastAct = env.Act
	// 中栏内容宽度：壳算好的 Geometry.MainWidth 优先，退到 Env.MainWidth。
	column := composerColumn(env)
	extra := float32(composerClearance - shellMainPad)
	if extra < 0 {
		extra = 0
	}
	// ConversationRoot .composerStack 的 --dsh-composer-stack-gap 是 6px。
	ui.Column(f).FillWidth().PadX(extra).Gap(composerStackGap).Children(func() {
		// 审批条：Approval == nil 时这里什么都不画（不留空框）。
		p.drawApproval(f, env)
		// `@` 候选菜单：位置在输入卡片**上方**，和 GoalBar / 问题条同一条约定
		// （越靠近输入框的东西越紧急）。没有候选就整块不画。
		p.drawMention(f, env, column)
		p.drawCard(f, env, column, drawSlot)
	})
}

// drawCard 画输入卡片：一个居中、定宽、圆角 panel(28) 的面板。
//
// 官方的 max-width 是「会话栏宽 * .64 收在 680…920，再加 32」，不是百分比——
// 所以这里必须先算出像素宽度再定宽，交给 flex 的 MaxWidth 只会按整窗解析。
//
// 卡片内部按官方 InputBar 的子槽位置排：
//
//	.card
//	├─ conversation.input.attachments   附件 rail（占用方在 → 它占高；空 rail 不占位）
//	├─ .input                           真 flex.Field + conversation.input.overlay（绝对定位覆盖层）
//	└─ .row                             控件行（见 drawControls 的子槽派发）
func (p *Plugin) drawCard(f ui.C, env *pluginapi.Env, column float32, drawSlot func(pluginapi.Slot)) {
	t := p.theme
	w := cardWidth(column)
	// 这里**不加**底 padding：官方 `.GRgSLq_root` 的 4px 是相对**窗口底部**的，
	// 而壳已经给中栏 12px padding——再加 4 会把卡片整体顶上 4px，
	// 卡片底距窗口底就从 8px 变成 12px（正好顶在 ≤8px 这条线的外面）。
	// 卡片自身 border:0，柔和的 elevation 由 Shadow 表达，不能塞一圈占布局的 Border
	//（否则宽、高和圆角各偏 1–2px）。
	ui.Row(f).FillWidth().Justify(ui.Center).Children(func() {
		ui.Column(f).Width(w).Radius(t.Radii.Panel).Background(t.InputMajor).
			Shadow(gfx.ShadowXS).
			PadSides(cardPadTop, 0, 0, 0).Gap(cardGap).Children(func() {
			// `.input` 区：附件 rail 在真 Field 上方；覆盖层绝对定位盖在整块输入区上。
			// 这一列没有 gap：空 rail 不产生节点，卡片高仍是 8+36+12+42。
			ui.Column(f).FillWidth().Children(func() {
				dispatchComposerSlot(drawSlot, uikernel.OffInputAttachments)
				p.drawInput(f, env, w)
				ui.Row(f).Absolute().Inset(0).Children(func() {
					dispatchComposerSlot(drawSlot, uikernel.OffInputOverlay)
				})
			})
			p.drawControls(f, env, w, drawSlot)
		})
	})
}

// drawInput 画可编辑区那一行。
//
// 官方 `.input` 是 contenteditable，铺满卡片宽度、无底无边；我们用的是 viva 的
// Field，它**强制**自绘底色 + 1px 描边（PaintInput → paintTextEntrySurface）。
// AfterDraw 用同一 InputSnapshot 把它的表面重绘成 InputMajor，于是"框里套框"消失。
func (p *Plugin) drawInput(f ui.C, env *pluginapi.Env, cardW float32) {
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
	ui.Row(f).FillWidth().Height(inputH).PadSides(0, 0, 0, left).
		Items(ui.Start).Children(func() {
		field := ui.TextFieldID(f, env.Tree.ID(env.Feat, "composer"), &p.field, "输入", placeholderText).
			Width(fieldW).Height(fieldHeight).FontSize(t.Fonts.Body)
		p.fieldBox, p.fieldBoxOn = field, true
		p.fieldTheme = *f.Theme()
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
func (p *Plugin) afterDraw(canvas *gfx.Canvas, env *pluginapi.Env) {
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
// primary 依次在右 trailing。
//
// # 子槽派发（每帧一次，位置照官方 DOM）
//
//	左 .tools ：.add（本插件）→ conversation.input.permission → .plan → .left
//	右 trailing：conversation.input.right → .model → .activity → .pending → .primary
//
// `permission` / `model` / `activity` 三枚是**占用方优先**：本座位有内核
// `drawSlot`（官方座位）且官方槽这一帧有占用方（`env.Has`）→ 由占用方画，本插件
// 不画；否则（没有派发通道，或槽空着）退回本插件的只读 chip。判据与壳判
// `BridgeLegacyComposer` 用的是同一个内核事实，所以**同帧只有一个活**，不会出现
// "两个模型标签"（顺序理由见 input_slots.go 的 composerDispatchTo）。
//
// `.left` / `.right` / `.plan` 本插件没有对应实现（官方也是空的默认组合），
// 只派发、不画回退。
func (p *Plugin) drawControls(f ui.C, env *pluginapi.Env, cardW float32, drawSlot func(pluginapi.Slot)) {
	p.controls = composerControlBoxes{}
	model, permission := hostLabels(env.Data)
	status := activityLabel(env.Data)
	running := env.Data.Stream != "" || env.Data.Busy
	t := p.theme
	// 官方 `.GRgSLq_primary{transform:translateY(-2px)}`：viva 的 MarginY 不吃负值，
	// 这里用**同样的视觉位移**实现——行高不变（2+34+6 == 0+34+8），把 2px 从
	// 上内边距挪到下内边距，发送键就整体上移 2px，底距卡片底 = 6+2 = 8（官方值）。
	ui.Row(f).FillWidth().Height(rowPadTop+sendSize+rowPadBottom).
		PadSides(rowPadTop-sendLift, rowPadSide, rowPadBottom+sendLift, rowPadSide).
		Items(ui.Center).Justify(ui.Between).Gap(cardGap).Children(func() {
		// 官方 `.tools`：命令/附件入口在**最左**（`.add` 28×28 圆钮），
		// 然后是 modes（permission/plan）。这一枚只在真有内容可加时出现
		// （见 official_seat.go 的 drawAddEntry：没有 FSList/CWD 就不画）。
		p.drawAddEntry(f, env)
		// 官方 DOM 的左 modes 只有 permission/plan；model 属于右 standardControls。
		// 三个槽都是"有占用方就派发、否则走本插件的回退/什么都不画"。
		ui.Row(f).Items(ui.Center).Gap(cardGap).Children(func() {
			if composerDispatchTo(drawSlot, env, uikernel.OffInputPermission) {
				dispatchComposerSlot(drawSlot, uikernel.OffInputPermission)
			} else if permission != "" {
				icon, hasIcon := permissionIconFor(strings.TrimSpace(env.Data.Permission), t.LabelSecondary)
				p.controls.Permission = p.hostChip(f, permission, permissionChipWeight, permissionChipW, icon, hasIcon)
			}
			dispatchComposerSlot(drawSlot, uikernel.OffInputPlan)
			dispatchComposerSlot(drawSlot, uikernel.OffInputLeft)
		})
		// 即使左边为空，Spacer 也把右侧 trailing 固定到卡片右缘。
		ui.Spacer(f)
		ui.Row(f).Items(ui.Center).Gap(cardGap).Children(func() {
			dispatchComposerSlot(drawSlot, uikernel.OffInputRight)
			if composerDispatchTo(drawSlot, env, composerInputSlotModel) {
				// 官方 `conversation.input.model`（single）：官方模型菜单（dsagentteam）
				// 就是它的占用方——trigger + 菜单都由它画。
				dispatchComposerSlot(drawSlot, composerInputSlotModel)
			} else if model != "" {
				p.controls.Model = p.drawModelChip(f, env, cardW, model)
			}
			if composerDispatchTo(drawSlot, env, uikernel.OffInputActivity) {
				dispatchComposerSlot(drawSlot, uikernel.OffInputActivity)
			} else if status != "" {
				p.hostChip(f, status, modelChipWeight, permissionChipW, gfx.Icon{}, false)
			}
			if running {
				p.busyDot(f)
			}
			// 保留原有发送/停止判据：Stream 或 Busy 任一成立时主按钮只走 Cancel。
			p.sendButton(f, env, running)
		})
	})
}

// drawModelChip 画只读的模型 chip（旧 chip 的回退绘制）。
//
// 只在官方 `conversation.input.model` 这一帧**没有占用方**时用：有占用方时由它
// 画（官方模型菜单的 trigger / 降级 chip）。几何与官方 ModelSelect trigger 同档：
// 高 28、radius sm、padding 左 8 右 8（不画 chevron，所以右边不留 4）、13/20/400、
// max-width min(360, 45cqw)。
func (p *Plugin) drawModelChip(f ui.C, env *pluginapi.Env, cardW float32, model string) ui.Box {
	maxW := cardW * modelCardRatio
	if maxW > modelChipMaxW {
		maxW = modelChipMaxW
	}
	return p.hostChip(f, model, modelChipWeight, maxW, gfx.Icon{}, false)
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
func (p *Plugin) hostChip(f ui.C, label string, weight, maxW float32, icon gfx.Icon, hasIcon bool) ui.Box {
	t := p.theme
	textMax := maxW - chipPadLeft - chipPadRight
	if hasIcon {
		textMax -= permissionIconWH + chipGap
	}
	if textMax < 0 {
		textMax = 0
	}
	return ui.Row(f).Height(chipHeight).MaxWidth(maxW).Radius(t.Radii.SM).
		PadSides(0, chipPadRight, 0, chipPadLeft).Items(ui.Center).Gap(chipGap).Children(func() {
		if hasIcon {
			ui.Icon(f, icon).Size(permissionIconWH, permissionIconWH).Color(t.LabelSecondary)
		}
		ui.Row(f).MaxWidth(textMax).Children(func() {
			ui.Text(f, label).FontSize(t.Fonts.Secondary).LineHeight(20).Weight(weight).
				Color(t.LabelSecondary).Ellipsis()
		})
	})
}

// busyDot 画官方 `.GRgSLq_pending` 那个 8px 圆点：宿主说 turn 在跑才有。
//
// 官方 `.pending{width:8px;height:8px;border-radius:50%;background:var(--dsw-alias-state-business-primary)}`
// （还有一条 1s 明暗动画）。这里不抖：无头截图拍不到动画，反复重绘只会让整窗脏区
// 变大；"在跑"这件事已经由发送键变■说清楚了。
func (p *Plugin) busyDot(f ui.C) {
	ui.Row(f).Size(busyDotSize, busyDotSize).Radius(gfx.RadiusFull).
		Background(p.theme.Info).Children(func() {})
}

// sendButton 画 34px 的圆形发送/停止键（官方 `.GRgSLq_primary`）。
//
// 官方 fill = --dsw-alias-button-info-fill = deepseek-400/500，正好是主题里的 Info
// （深色 deepseek-400、浅色 deepseek-500，逐档一致）；前景色官方写死 #fff，
// 我们没有 "on-info" 令牌，取对比度最高的那一档，见 onInfo()。
// 图标是官方 16 viewBox 的填充路径，不再依赖 ↑/■ 的字体字形。
func (p *Plugin) sendButton(f ui.C, env *pluginapi.Env, running bool) {
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
	button := ui.Row(f).Size(sendSize, sendSize).Radius(gfx.RadiusFull).Background(fill).
		Items(ui.Center).Justify(ui.Center).MarginY(-sendLift)
	if !disabled {
		button = button.Hover(ui.Look{}.Background(t.Info))
	}
	p.controls.Primary = button
	clicked := button.HitWith(id, label, gfx.HitOptions{
		Role: gfx.RoleButton, Disabled: disabled,
		// 官方 onMouseDown=keepDraftFocus；鼠标动作不抢编辑器/IME 焦点。
		// 键盘仍走输入框 Enter 发送、Esc 停止，审批按钮保持自己的焦点语义。
		SkipFocus: true,
	}).Children(func() {
		ui.Icon(f, icon).Size(primaryIconWH, primaryIconWH).Color(foreground)
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

// submit 把草稿交给壳，然后清空自己。
//
// 空文本也交给壳：官方允许附件-only 消息（OFF:service.ts:302），壳会在
// 没有附件时把空文本挡回去（`sendLocal`）。Send 没接时既不调动作也不清草稿，
// 避免一次无效点击把输入弄丢。
func (p *Plugin) submit(env *pluginapi.Env) bool {
	text := strings.TrimSpace(p.field.Text())
	if env.Act.Send == nil {
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
//
// # Shift+Enter 必须被吃掉
//
// 官方 composer 是多行编辑器：Enter 发送、**Shift+Enter 换行**。viva 的
// `flex.Field` 是单行（`field.go` 注释写死「A field is one line, so Enter is
// free to mean done」），没有多行编辑态，所以"插换行"这一半做不了——但
// **"不发送"这一半必须做**：不拦的话 Shift+Enter 会落到驱动器的提交路径
// （`painter.key` → `ClickedByKey`），用户以为在换行，草稿已经发出去了。
//
// 所以这里无条件吃掉 Shift+Enter（不调 Send、不清草稿）。真正的换行编辑是
// Viva 上游缺口（flex 没有 Textarea），最小复现与后续见报告 §未实现项。
func (p *Plugin) HandleKey(ev gfx.KeyEvent) bool {
	if ev.Release {
		return false
	}
	if ev.Key == gfx.KeyEnter && ev.Modifiers&gfx.ModifierShift != 0 {
		return true
	}
	if ev.Modifiers != 0 {
		return false
	}
	if !approvalVisible(p.lastData) {
		// 触发菜单开着时，按键归菜单（见 trigger.go / mention.go）：
		//
		//	Enter    选中当前行（文件/会话/技能）；目录行 = drill
		//	Tab      目录 drill（官方 drill.key = Tab）
		//	↑ / ↓    移动选中行（官方 move 事件）
		//	Esc      关掉菜单（同一份草稿不再重开）
		//
		// 门禁卡死在"这一帧确实有候选"上：没有候选就原样让回去。
		// 让回去的代价是零；吃掉的代价是**这条消息再也发不出去**，
		// 而且用户看到的是"我按了 Enter 什么都没发生"——没有任何提示。
		if len(p.triggerRows) > 0 || len(p.mentionRows) > 0 {
			switch ev.Key {
			case gfx.KeyEnter:
				if p.acceptMention() {
					return true
				}
			case gfx.KeyTab:
				if p.drillTrigger() {
					return true
				}
			case gfx.KeyUp:
				if p.moveTrigger(-1) {
					return true
				}
			case gfx.KeyDown:
				if p.moveTrigger(1) {
					return true
				}
			case gfx.KeyEscape:
				if p.dismissTrigger() {
					return true
				}
			}
		}
		// 没有审批时 Esc 退而求其次 = **停止当前 turn**（官方也是这个直觉）。
		// 不做这一条的话，无头环境里根本没法触发停止键——它是个按钮，
		// 注入不了鼠标事件；而"能停"是产品级链路的必选项（宿主 §0.6）。
		if ev.Key == gfx.KeyEscape && p.lastAct.Cancel != nil {
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
	case gfx.KeyEnter:
		if !plan.Allow.Usable {
			return false
		}
		p.decideKey(true)
		return true
	case gfx.KeyEscape:
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
