package dsplan

import (
	"time"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 文案：**只有这些**，逐字来自 OFF:locales.ts:4-21 的 zh 字典。
//
// 与 dsactioncards 同一条规矩：界面上出现这 16 条以外的字样，就不是官方那个界面了。
// 未用到的几条（chip.*）也留在这里：将来补上 `plan` 投影时只接数据、不新写文案。
const (
	zhTitle          = "计划"                         // preview.title
	zhDocument       = "计划 · Markdown"              // preview.document
	zhAction         = "打开"                         // preview.action
	zhOpen           = "在侧边栏打开计划"                   // preview.open
	zhFull           = "查看全文"                       // preview.full
	zhOpenNamedFmt   = "打开计划：%s"                    // preview.openNamed（{title}）
	zhLoading        = "正在读取计划…"                    // preview.loading
	zhFailed         = "无法读取计划"                     // preview.failed
	zhInvalidAddress = "计划地址无效"                     // preview.invalidAddress
	zhNotFound       = "未找到这份计划"                    // preview.notFound
	zhUnavailable    = "计划预览不可用"                    // preview.unavailable
	zhExpired        = "临时计划预览已失效，请从仍在等待审批的卡片重新打开。" // preview.expired
	zhChipLabel      = "计划"                         // chip.label
	zhChipTitle      = "计划模式已开启 — 点击关闭（/plan off）"  // chip.on.title
	zhChipExitFailed = "退出计划模式失败"                   // chip.exitFailed
	// zhChipAria 是官方 chip 的 **无障碍名**（`aria-label`，OFF:locales.ts:18）。
	//
	// 官方那个 button 同时有 `aria-label={t('chip.on.aria')}` 与
	// `title={t('chip.on.title')}`（OFF:PlanModeControl.tsx:56-57）：前者是
	// 辅助技术的名字，后者是**原生** title 的悬停提示。Viva 的 `Hit(id, label)`
	// 承载的是前者，没有原生 title 属性这一层，所以这个座位上只有 aria 文案有
	// 落点——用它，而不是拿 title 文案顶替（后者已登记为差异）。
	zhChipAria = "计划模式已开启，按下关闭"
)

// 卡片几何：OFF:PlanPreview.module.css:1-21 逐条。
//
//	.cards  { display:flex; flex-direction:column; gap:10px }
//	.card   { align-items:center; gap:10px; width:100%; height:60px; padding:8px 10px;
//	          border:0.5px solid border-l1; border-radius:radius-lg }
//	.cardIcon { width:40px; height:40px; border:0.5px solid border-l1; border-radius:10px }
//	.cardDetails { flex:1; flex-direction:column; gap:2px; min-width:0 }
//	.cardTitle { font-size:13px; font-weight:500; line-height:20px; ellipsis }
//	.cardDescription { font-size:10px; line-height:16px; ellipsis }
//	.cardOpen { height:28px; padding:4px 8px; border:0.5px solid border-l3;
//	            border-radius:radius-sm; font-size:12px; line-height:18px }
const (
	cardsGap       float32 = 10
	cardHeight     float32 = 60
	cardPadY       float32 = 8
	cardPadX       float32 = 10
	cardGap        float32 = 10
	cardBorder     float32 = 0.5
	cardIconSize   float32 = 40
	cardIconRadius float32 = 10
	cardTitleFont  float32 = 13
	cardTitleLine  float32 = 20
	cardTitleWght  float32 = 500
	cardDescFont   float32 = 10
	cardDescLine   float32 = 16
	cardDetailsGap float32 = 2
	cardOpenHeight float32 = 28
	cardOpenPadY   float32 = 4
	cardOpenPadX   float32 = 8
	cardOpenFont   float32 = 12
	cardOpenLine   float32 = 18
	// PlanCard.tsx:51：FileTypeIcon kind="markdown" size={20}。
	cardIconGlyphSize float32 = 20
	// focus.css:3：--dsw-focus-ring-width:2px。
	planFocusRing float32 = 2
)

var planCardMotion = gfx.Motion{Duration: 120 * time.Millisecond, Easing: gfx.EaseCSS}

// 官方 FileTypeIcon(kind="markdown") 的**两个颜色都来自继承**，不是烘死的品牌色：
//
//	底板  fill="currentColor"  → `.card` 的 color（PlanPreview.module.css:17）
//	                              即 --dsw-alias-label-primary
//	折角  var(--dsw-static-neutral-00) fillOpacity .7（design-platform.css:40）
//	字    <g color="var(--dsw-static-neutral-00)">（同一处 neutral-00 = 白）
//
// 所以这里**每帧按主题色现建**（原先是包级 var 烘了 deepseek-500：
// design-platform.css:28 那个 `--dsw-static-deepseek-500` 是静态色板，
// 官方这个部件没有引用它）。gfx 的 tint 只把 Color() 的 alpha 乘进图层 Fill
// （viva v0.6.4 gfx/icon.go:526-529），烘死底色会让深/浅主题各错一边。
func planMarkdownIcon(t *pluginapi.Theme) (gfx.Icon, bool) {
	return dsassets.FileTypeMarkdown(t.LabelPrimary, gfx.RGB24(0xffffff))
}

// drawCards 画这一轮的 Turn 尾计划卡（官方 `PlanCards`，OFF:PlanCard.tsx:43-62）。
//
// **一轮一次**：官方座位 `conversation.chat.turnTail` 是 `turn-tail` 节点的
// children（OFF:.../ui-chat/src/client/chat/register-node-renderers.ts:88-97），
// owner props 带这一轮的 turn（OFF:.../contract/slots.ts:60-64），`PlanCards` 拿它
// 只取这一轮的 `submitted-plan` 数据源（OFF:PlanCard.tsx:43-45 +
// OFF:index.ts:88-92）。本仓壳同样按轮派发（`internal/uikernel/shell_skeleton.go`
// 的 `dispatchTurnTails`），所以这里读 `env.Turn` 取**那一轮**的计划。
//
// 三个"不画"的判据，都是官方同一条"空槽不占位"：
//   - 没有帧基线（`len(Frames)==0`：老宿主 / mockd）→ 不画
//   - 本会话没有任何 `exit_plan_mode` 提交 → 不画
//   - 这一轮没有提交 → 不画（卡片属于**提交它的那一轮**）
func (p *Plugin) drawCards(f ui.C, env *pluginapi.Env) {
	t := p.theme
	if t == nil || env == nil {
		return
	}
	plans := turnPlans(env, SubmittedPlans(env.Data.Frames))
	if len(plans) == 0 {
		return
	}
	session := env.Data.Cur
	ui.Column(f).FillWidth().Gap(cardsGap).Children(func() {
		for _, plan := range plans {
			p.drawCard(f, env, t, session, plan)
		}
	})
}

// drawCard 画一张卡：图标槽 + 标题/说明 + 「打开」，整块可点（官方是 `<button>`）。
func (p *Plugin) drawCard(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, session string, plan Document) {
	label := replaceTitle(zhOpenNamedFmt, plan.Title)
	card := ui.Row(f).FillWidth().Height(cardHeight).Items(ui.Center).Gap(cardGap).
		PadSides(cardPadY, cardPadX, cardPadY, cardPadX).
		Border(cardBorder, t.BorderL1).Radius(t.Radii.LG).
		Background(t.CardFill).
		Hover(ui.Look{}.Background(t.CardFillHover)).
		DrawOver(func(d *ui.DrawContext) { drawPlanFocusRing(d, planCardFocusOffset, t.Radii.LG, t.Info) }).
		TransitionWith(planCardMotion).
		Hit(env.Tree.ID(env.Feat, "plan-card-"+plan.CallID), label)
	// 先 Hit 再读 Clicked：v0.6.0 的 `.Clicked()` 只在有身份时才可能为真
	// （cmd/app/hits_static_guard_test.go 守着这条）。
	if card.Clicked() {
		p.open(env, session, plan)
	}
	card.Children(func() {
		// `.cardIcon`：40×40 圆角 10 的描边块，里面一个 20px 图标位。
		icon := ui.Column(f).Width(cardIconSize).Height(cardIconSize).
			Center().Border(cardBorder, t.BorderL1).Radius(cardIconRadius).
			Background(t.CardTileFill)
		icon.Children(func() {
			if markdown, ok := planMarkdownIcon(t); ok {
				ui.Icon(f, markdown).Width(cardIconGlyphSize).Height(cardIconGlyphSize)
			}
		})
		// `.cardDetails`：标题 + 说明两行，flex:1、min-width:0（用 Ellipsis 表达）。
		ui.Column(f).Grow(1).Gap(cardDetailsGap).Children(func() {
			ui.Text(f, plan.Title).FontSize(cardTitleFont).LineHeight(cardTitleLine).
				Weight(cardTitleWght).Color(t.LabelPrimary).Ellipsis()
			ui.Text(f, zhDocument).FontSize(cardDescFont).LineHeight(cardDescLine).
				Color(t.LabelTertiary).Ellipsis()
		})
		// `.cardOpen`：28 高、4/8 内边距、radius-sm、border-l3 的胶囊。
		open := ui.Row(f).Height(cardOpenHeight).Items(ui.Center).
			PadSides(cardOpenPadY, cardOpenPadX, cardOpenPadY, cardOpenPadX).
			Border(cardBorder, t.BorderL3).Radius(t.Radii.SM).Background(t.BgLayer2)
		open.Children(func() {
			ui.Text(f, zhAction).FontSize(cardOpenFont).LineHeight(cardOpenLine).Color(t.LabelPrimary)
		})
	})
}

// open 是「点了一张计划卡」：把这份计划交给右栏全文，并切到它的 tab。
//
// 官方的对等动作是 `ctx.sidebarRight.openResource(planAddress(...))`（OFF:index.ts:71-78）：
// ① 记住地址 ② 右栏切到 plan 类型 ③ 右栏收起时展开。本仓这三个动作分别落在
// 插件状态、`Act.SelectRightTab`、`Act.ToggleRightbar` 上（内核没有"按地址打开
// tab"这一层，右栏 tab 只有类型没有 navigation——差异登记在 preview.go 文件头）。
func (p *Plugin) open(env *pluginapi.Env, session string, plan Document) {
	address := planAddress(session, plan.CallID)
	if address == "" {
		return
	}
	p.openAddress = address
	if env.Act.SelectRightTab != nil {
		env.Act.SelectRightTab(RightTabPlan)
	}
	if !env.Data.ShowRightbar && env.Act.ToggleRightbar != nil {
		env.Act.ToggleRightbar()
	}
}

// replaceTitle 把官方 `{title}` 插值填进去。
//
// 官方 key 是 `preview.openNamed`（`打开计划：{title}`）；这里只做**单占位**
// 替换，不做通用模板引擎——只有一条文案带占位。
func replaceTitle(format, title string) string {
	placeholder := "{title}"
	for i := 0; i+len(placeholder) <= len(format); i++ {
		if format[i:i+len(placeholder)] == placeholder {
			return format[:i] + title + format[i+len(placeholder):]
		}
	}
	return format
}
