// official_seat.go —— 官方 `conversation.composer` 座位（rc.2 终态）的绘制。
//
// # 为什么单独一个文件
//
// `composer.go` 画的是 `main.composer`（旧三栏 / 官方骨架的**兼容桥**座位）；
// 本文件画的是官方 **chain** 槽 `conversation.composer`（`uikernel.OffConvComposer`）。
// 两个座位**同一帧只有一个活**（壳按 `ShellOptions.BridgeLegacyComposer` 二选一
// dispatch），所以卡片、命中区、输入框都只有一份实现：`drawInput` / `drawControls`
// 两个座位共用，这里只补官方**外层结构**。
//
// # 官方层级（rc.2 随包源码）
//
//	ConversationContent.tsx:147-186   renderSlotChain('conversation.composer',
//	                                     {fallback: composerBar, overlay: true})
//	composerBar                        composerStack（gap 6）
//	InputBar.root                     padding: 0 var(--dsh-composer-side-clearance) 4px
//	├─ .card                         radius-panel 28 / padding-top 8 / gap 12 / InputMajor
//	│  ├─ .input                     min-height 36（hero 52）
//	│  └─ .row                       padding 2px 8px 6px；`.add` 28×28 + 两个 trigger + 34 主钮
//	└─ .dock                         padding-top 4 / min-height 22（StatsPills / ContextMeter）
//
// `InputBar.module.css:2-66`（root/card/row）、`:336-348`（`.add`）、
// `:398-410`（`.primary`）、`StatsPills.module.css:13-60` + `ContextMeter.module.css:9-38`
// （dock 里的读数胶囊）。每条常量都注了出处，改之前先回去读那一行。
//
// # 侧边距由壳给，不在插件里判档位
//
// 官方 `InputBar.root` 的左右各 16 是**相对正文列**的。官方档里座位容器没有内边距
// （直接 16）；旧三栏兼容桥里外层已经有 12px 栏内边距，`uikernel.drawComposerSeat`
// 只补差值（`LegacyComposerSidePad` = 4）。两种档位净边距都是 16，插件只画内容。
package dscomposer

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const (
	// --dsh-composer-stack-gap: 6px（ConversationRoot.module.css:422-427 的
	// `.composerStack{gap:var(--dsh-composer-stack-gap)}`）。
	composerStackGap = 6

	// InputBar.root 的 padding-bottom（InputBar.module.css:10）。
	composerPadBottom = 4

	// InputBar.dock：padding-top 4（InputBar.module.css:19）、非 hero 的
	// min-height calc(22px + font-delta-secondary)（:22-52）。
	composerDockPadTop = 4
	composerDockMinH   = 22

	// InputBar.dock 里各条目之间的 gap：`justify-content:center;gap:12px`
	// （InputBar.module.css:13-19 的 `.dock`）。
	composerDockGap = 12

	// dock 里的读数胶囊：StatsPills `.pill` / ContextMeter `.trigger` 两个组件
	// 的几何一致（padding 1px 8px、gap 6、radius-sm、行高 20、图标 14、
	// 字号 calc(13px - 1px)）。
	composerPillPadY   = 1
	composerPillPadX   = 8
	composerPillGap    = 6
	composerPillLineH  = 20
	composerPillFont   = 12
	composerPillIconWH = 14

	// 控件行左端的「加号」入口（官方 `.add`，InputBar.module.css:336-348）：
	// 28×28 圆、背景 --dsw-specific-selector、图标 14、disabled opacity .5。
	composerAddSize = 28
	composerAddIcon = 14

	// 上下文计数缺失时的文案。规则与审批卡一致（approval.go 的 unknownField）：
	// 宿主没给就写「未知」，不留空、不拿 0 冒充。
	contextLabel = "上下文"
)

// drawOfficialComposer 是官方 `conversation.composer` 座位的绘制入口
// （**只给单测直调用**：没有内核就没有 `drawSlot`，所有子槽走只读回退）。
func (p *Plugin) drawOfficialComposer(f ui.C, env *pluginapi.Env) {
	p.drawOfficialComposerChildren(f, env, nil)
}

// drawOfficialComposerChildren 画官方座位，并把它作为**父容器**派发
// `conversation.input.*` 子槽（见 input_slots.go 的文件头：为什么必须在这里派发）。
//
// 官方这一格是 **chain**：fallback 是 InputBar（本函数画的 composerStack），
// overlay 是接管项（审批卡）。两者同帧共存，审批卡在上、输入卡在下，
// 与旧座位的相对位置一致（用户不会因为切档位而换个地方找按钮）。
//
// 顺序（官方 ConversationContent / InputBar 的结构）：
//
//	composerStack（gap 6）
//	├─ 审批卡（接管项，Approval == nil 时整条不画）
//	├─ `@` 候选菜单（输入卡上方）
//	├─ conversation.composer.dock    Queue/Todo dock（贴卡片上方；空则不产生节点）
//	└─ InputBar.root（padding-bottom 4）
//	   ├─ .card（见 drawCard：附件 rail / 输入区 / 控件行）
//	   └─ .dock（conversation.input.dock + 上下文读数）
func (p *Plugin) drawOfficialComposerChildren(f ui.C, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	p.lastData = env.Data
	p.lastAct = env.Act
	column := composerColumn(env)
	ui.Column(f).FillWidth().Gap(composerStackGap).Children(func() {
		// 接管项：审批卡（Approval == nil 时 drawApproval 自己整条不画）。
		p.drawApproval(f, env)
		// `@` 候选菜单：在输入卡片上方（与旧座位同一条约定）。
		p.drawMention(f, env, column)
		// 官方 `conversation.composer.dock`（Queue/Todo dock）：贴在输入卡片上方。
		// 直接派发在 stack 里（不套空容器）：没有占用方就不产生节点、不多吃一个 gap。
		dispatchComposerSlot(drawSlot, uikernel.OffConvComposerDock)
		// fallback：InputBar 本体。
		p.drawInputBar(f, env, column, drawSlot)
	})
}

// drawInputBar 画官方 `InputBar.root`：卡片 + 卡片下方 dock。
//
// 高度是**内容驱动**的（卡片 98 + dock 26 + 底边距 4 = 128），不是钉死 122：
// 官方 `.root` 也没有 height，它有 padding-bottom 4、子元素按内容排。座位容器
// 给的高度只用于「没有占用方」的空壳结构（见 uikernel.drawComposerSeat）。
func (p *Plugin) drawInputBar(f ui.C, env *pluginapi.Env, column float32, drawSlot func(pluginapi.Slot)) {
	ui.Column(f).FillWidth().PadSides(0, 0, composerPadBottom, 0).Children(func() {
		// .card：居中、定宽、圆角 panel(28)（见 drawCard）。
		p.drawCard(f, env, column, drawSlot)
		// .dock：卡片下方的读数行（官方 StatsPills / ContextMeter）。
		p.drawDock(f, env, cardWidth(column), drawSlot)
	})
}

// drawDock 画官方 `InputBar.dock`：一行居中的读数胶囊。
//
// 今天只有一枚真值可画——上下文占用（`Snapshot.ContextTokens/ContextWindow`）。
// 官方 ContextMeter 在宿主没给 used/contextWindow 时**不渲染**；本仓按跨线 §12
// 的「未知显式化」规则改成显示「未知」：这一格是**结构位置**，空着会让用户以为
// 这一版没有上下文读数，而真相是宿主没给。
//
// `conversation.input.dock`（list/session，官方 composer 底部 dock）在读数前面
// 派发：占用方（Activity/Usage 胶囊等）画出来的节点与本胶囊同一行、同一 gap。
//
// 其余读数（Activity / Usage / 缓存率）归 dsstatusbar 的 dock，不在这里重复画：
// 同一行两组数字会让人分不清哪个是"当前上下文"。
func (p *Plugin) drawDock(f ui.C, env *pluginapi.Env, cardW float32, drawSlot func(pluginapi.Slot)) {
	t := p.theme
	text := contextMeterText(env.Data)
	ui.Row(f).FillWidth().Height(composerDockPadTop+composerDockMinH).
		PadSides(composerDockPadTop, 0, 0, 0).Justify(ui.Center).Gap(composerDockGap).Children(func() {
		dispatchComposerSlot(drawSlot, uikernel.OffInputDock)
		ui.Row(f).Height(composerPillLineH+2*composerPillPadY).MaxWidth(cardW).
			Radius(t.Radii.SM).PadSides(composerPillPadY, composerPillPadX, composerPillPadY, composerPillPadX).
			Items(ui.Center).Gap(composerPillGap).Children(func() {
			ui.Text(f, contextLabel).FontSize(composerPillFont).LineHeight(composerPillLineH).
				Color(t.LabelTertiary)
			ui.Text(f, text).FontSize(composerPillFont).LineHeight(composerPillLineH).
				Color(t.LabelTertiary).Ellipsis()
		})
	})
}

// contextMeterText 把上下文占用翻成一行读数。
//
// 三种真值状态（**不编数字**）：
//
//	ContextWindow <= 0          → 「未知」（宿主没给容量，百分比算不出来）
//	ContextTokens <  0          → 「未知」（宿主没给已用量）
//	都有                        → 「12.3k / 200k（6%）」
//
// 为什么不用 `Snapshot.Usage.TotalTokens` 顶替：那是**全会话累计**（一直涨），
// 上下文会随压缩回落——两者混用会让用户看到"上下文越用越大"的假象
// （pluginapi.Snapshot 的 ContextTokens 注释写死了这条）。
func contextMeterText(s pluginapi.Snapshot) string {
	if s.ContextWindow <= 0 || s.ContextTokens < 0 {
		return unknownField
	}
	used := formatTokenCount(s.ContextTokens)
	window := formatTokenCount(s.ContextWindow)
	percent := float64(s.ContextTokens) / float64(s.ContextWindow) * 100
	return used + " / " + window + "（" + formatPercent(percent) + "）"
}

// formatTokenCount 把 token 数压成 k/M 档读数。
//
//	999        → 999
//	1234       → 1.2k
//	15537      → 15k
//	200000     → 200k
//	1000000    → 1M
//	1050000    → 1.05M
//
// 为什么 k 档一位小数、M 档两位：容量量级（200k / 1M）要一眼可读，而"用掉多少"
// 在 M 档上差 0.01M 就是一万 token——一位小数会把 1.04M 和 1.05M 抹平。
// k 档的 0.1k = 100 token，够用了。**尾零一律去掉**：正好 1M 写「1M」。
func formatTokenCount(n int64) string {
	switch {
	case n < 1000:
		return itoa(n)
	case n < 10000:
		return trimTrailingZero(itoa(n/1000)+"."+itoa(n%1000/100)) + "k"
	case n < 1000000:
		return itoa(n/1000) + "k"
	case n < 10000000:
		// 两位小数：1050000 → 1.05M。
		return trimTrailingZero(itoa(n/1000000)+"."+itoa(n%1000000/100000)+itoa(n%100000/10000)) + "M"
	default:
		return itoa(n/1000000) + "M"
	}
}

// trimTrailingZero 去掉小数部分的尾零（1.00 → 1，1.20 → 1.2，1.05 不变）。
func trimTrailingZero(text string) string {
	return strings.TrimSuffix(strings.TrimRight(text, "0"), ".")
}

// formatPercent 是官方 ContextMeter 的百分比：`Math.min(100, Math.round(used/window*100))`
// （`ui-conversation/src/client/context-occupancy.ts:21`）。
//
// 两点照抄官方，不自己发明：
//   - **四舍五入**到整数：0.63% 显示 1%，不是 0%（截断会把"用了一点"画成"没用"）。
//   - **上限 100**：宿主数据异常（used > window）时官方也显示 100%，不显示 200%。
func formatPercent(p float64) string {
	percent := int64(p + 0.5)
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return itoa(percent) + "%"
}

// itoa 是 strconv.Itoa 的最小等价物：这个文件只做整数拼接，不为它引一个包。
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// drawAddEntry 画控件行左端的「加号」入口（官方 `.add`）。
//
// **只在真有内容可加时画**：本仓今天唯一能"加"的东西是 `@` 文件引用
// （`Actions.FSList` + `Snapshot.CWD`）。没有它就不画——画一个点了没反应的
// 加号等于告诉用户"这里能加东西"，而实际上什么都不会发生
// （composer.go 文件头第 2 条硬规矩：宿主没给动作的控件不画）。
//
// 与官方的差异（写进报告）：官方 `.add` 打开的是**命令菜单**（`aria-label`
// 是 `input.commands`，`InputBar.tsx:505-520`），我们还没有命令注册表；
// 文件引用是当前唯一真实的"添加"来源，所以这里让它打开 `@` 候选。
// 等命令注册表落地，这一枚改成"加号 → 菜单（命令 / 附件 / 文件引用）"。
func (p *Plugin) drawAddEntry(f ui.C, env *pluginapi.Env) bool {
	canReference := (env.Act.FileReferences != nil && env.Data.Cur != "") ||
		(env.Act.FSList != nil && strings.TrimSpace(env.Data.CWD) != "")
	if !canReference {
		return false
	}
	t := p.theme
	id := env.Tree.ID(env.Feat, "composer-add")
	button := ui.Row(f).Size(composerAddSize, composerAddSize).Radius(gfx.RadiusFull).
		Background(t.BgLayer3).Items(ui.Center).Justify(ui.Center).
		Hover(ui.Look{}.Background(t.Hover))
	clicked := button.HitWith(id, "添加文件引用", gfx.HitOptions{
		Role: gfx.RoleButton,
		// 官方 onMouseDown=keepFocus：加号不抢输入框/IME 焦点。
		SkipFocus: true,
	}).Children(func() {
		ui.Icon(f, addPlusIcon).Size(composerAddIcon, composerAddIcon).Color(t.LabelPrimary)
	}).Clicked()
	if clicked {
		// 插入一个 `@` 触发点：下一帧 computeMention 就会按 CWD 列候选。
		// 不直接拼路径——拼什么由用户在候选里选（见 mention.go）。
		p.field.SetText(p.field.Text() + "@")
	}
	return true
}
