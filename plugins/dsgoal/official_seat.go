// official_seat.go —— 目标条的**官方座位**（rc.2 `@deepseek-ai/dsh-client-ui-goal`）。
//
// # 官方挂在哪里（逐字）
//
// `ui-goal/src/client/index.ts` 的 `apply` 里只有一处注册：
//
//	ctx.slots.inject('conversation.input.dock', () => ctx.slots.register({
//	  name: 'conversation.input.dock', id: 'goal', order: 10, …
//	}, GoalDock))
//
// 也就是说官方 GoalBar 是 **conversation.input.dock** 里的一条（list/session），
// 位置在输入卡片**上方**——ui-conversation 对该槽的声明原文是
// *"Full-width entries above the composer card"*（`contract/slots.ts:305`）。
//
// # 官方子槽今天**不可达**，所以挂同一个 chain 座位
//
// 官方的 GoalBar 是 `conversation.input.dock` 的一条，而这个槽由 composer 占用方
// （官方 `InputBar.root` 的父级 InputZone）转发。本仓的占用方是 `plugins/dscomposer`，
// 它**还没有转发自己的子槽**——GR 报告 `tmp/GO8-RC2/reports/GR-official-composer.md`
// §8 第 1 条记的就是这条。照着官方槽名注册会得到一个"登记了但永远画不到"的座位
// （不报错、界面空白，最难查的一类故障）。
//
// 所以本插件挂在**真的会被 dispatch 的那个官方座位**上：`conversation.composer`
// （chain，官方 ui-conversation 把它声明成"接管常住 composer 的入口"）。
//
//	conversation.composer
//	├─ questions-bar-official (ORDER 3)   ← 提问卡（最上面）
//	├─ goal-bar-official      (ORDER 5)   ← 本插件（卡片上方 = 官方 dock 的位置）
//	└─ composer-official      (ORDER 10)  ← dscomposer 的输入卡片
//
// 相对位置与官方一致（dock 在卡片上方），差别只是"父节点从 InputZone 换成了
// chain 座位"。等 dscomposer 转发 `.dock` 之后，把 painter 的 Slot 换成
// `uikernel.OffInputDock`、Order 换成官方的 10 即可（一行改动，报告里登记为剩余差距）。
package dsgoal

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const (
	// 官方目标条在 `conversation.input.dock` 里的 id 'goal' / order 10
	// （`ui-goal/src/client/index.ts:apply`）。本仓暂时挂在 chain 座位上，
	// 用**相对顺序**表达同一位置：3（提问）< goalDockOrder（目标）< 10（输入卡片）。
	goalDockOrder = 5

	// `.bar{height:36px}`。
	goalBarHeight float32 = 36
	// `.bar{gap:10px}`。
	goalBarGap float32 = 10
	// `.bar{padding:4px 5px 4px 12px}`（top/right/bottom/left）。
	goalBarPadTop    float32 = 4
	goalBarPadRight  float32 = 5
	goalBarPadBottom float32 = 4
	goalBarPadLeft   float32 = 12
	// `.dock{width:calc(100% - 2*clearance - 4*dock-inset)}`，inset=8；
	// 座位已扣 clearance，因此左右各再留 2*inset=16（GoalBar.module.css:5-17）。
	goalDockInset  float32 = 8
	goalDockMargin float32 = 2 * goalDockInset
	// `.label{font-size:13px; line-height:24px}`；活跃文案见 locales.ts:5。
	goalLabelFont float32 = 13
	goalLabelLine float32 = 24
	goalLabelText         = "进行中的目标"
	// `.objective{font-size:13px; line-height:20px}`。
	goalObjectiveFont float32 = 13
	goalObjectiveLine float32 = 20
	// 官方 `.goalGlyph` 是 sprite`goal-outline`（viewBox 16、size 14）。原注释
	// 「本仓没有那枚 symbol」是误判：官方 GoalBar.tsx:137 用的就是
	// `IconGoalOutlineRegular`，而素材库 `assets/icons/tools/goal.jsx` 与 rc.2
	// sprite 的 4 段 path **逐字节相同**。符号定义见 icons.go（goalGlyphSize）。

	// `.Gc24Bq_bar{--dsw-elevation-stroke-color:var(--dsw-alias-border-l1);
	//  box-shadow:var(--dsw-elevation-panel);border:0}`（GoalBar.module.css）。
	// 原来这里手抄了 elevation-panel 的五个数（0.5 / 3+8@.03 / 0+16@.02）与一个
	// stroke 常量：取值已并进 `pluginapi.Shadows.Panel`，描边色仍由调用方按官方
	// 那一处的重绑传（`BorderL1`），所以本地常量全部删掉。
)

// composerSeatLive 报告**这一帧**官方 composer 座位是不是活的。
//
// 判据只有档位一条，理由：
//
//	legacy 档位                 → dsshell 只 dispatch 旧槽（main.composer）→ false
//	official 档位               → 官方骨架一定 dispatch `conversation.composer`
//	                              （本插件自己也注册在它上面，所以
//	                              `BridgeLegacyComposer` 必为假，见
//	                              `cmd/app/shell.go` 的 shellOptionsFor）
//
// **不能**用 `OfficialSlotsLive()`：`-shell official` 的产品形状里整列兼容桥
// 还开着（sidebar / main.body 仍是旧槽），那一帧它返回假；但 composer 是
// **分段开关**，官方座位照样被 dispatch——按整列判会把目标条挂到一个不画的座上。
func (p *Plugin) composerSeatLive() bool { return p.tier.Official() }

// goalText 读这一帧的目标文本（闭包，每帧都读；空 = 没有活跃目标）。
func goalText(env *pluginapi.Env) string {
	if env == nil || env.Act.Goal == nil {
		return ""
	}
	return env.Act.Goal()
}

// goalColumn 是本帧主列宽（壳算好给插件，插件不自己猜）。
func goalColumn(env *pluginapi.Env) float32 {
	if env == nil {
		return 0
	}
	if env.Geometry.MainWidth > 0 {
		return env.Geometry.MainWidth
	}
	return env.MainWidth
}

// goalContentWidth 是官方 `--dsh-chat-content-width`：clamp(680, 列宽×.64, 920)。
//
// 与 dscomposer 的 `contentWidth` 同源（`plugins/dscomposer/composer.go:507-517`），
// 两处都抄官方同一行 `clamp(680px, 主列宽×.64, 920px)`；不是各编一个数。
func goalContentWidth(column float32) float32 {
	if column <= 0 {
		return 680
	}
	w := column * 0.64
	if w < 680 {
		return 680
	}
	if w > 920 {
		return 920
	}
	return w
}

// drawOfficial 画官方座位上的目标条；legacy 与 official 共用同一套 CSS 几何。
func (p *Plugin) drawOfficial(f ui.C, env *pluginapi.Env) {
	if !p.composerSeatLive() {
		return
	}
	p.drawGoalBar(f, env)
}

// drawGoalBar 对照 GoalBar.module.css:5-84 画唯一一张 36px 目标条。
// 可见性：只在有活跃 goal 时显示；官方 complete 同样不画（GoalBar.tsx:86-87）。
func (p *Plugin) drawGoalBar(f ui.C, env *pluginapi.Env) {
	goal := goalText(env)
	if goal == "" {
		return
	}
	t := p.theme
	if env.Theme != nil {
		t = env.Theme
	}
	// card-max-width=content+32，bar max-width=card-max-width-4*8，因此就是 content。
	maxW := goalContentWidth(goalColumn(env))
	// `.dock{margin:0 auto; width:calc(100% - 2*clearance - 4*inset)}`：
	// 座位已扣 2×clearance，这里左右各扣 2×inset，总计官方 4×inset。
	ui.Row(f).FillWidth().MarginSides(0, goalDockMargin, 0, goalDockMargin).
		Justify(ui.Center).Children(func() {
		ui.Row(f).FillWidth().MaxWidth(maxW).Height(goalBarHeight).
			PadSides(goalBarPadTop, goalBarPadRight, goalBarPadBottom, goalBarPadLeft).
			Items(ui.Center).Gap(goalBarGap).
			Radius(t.Radii.MD).Background(t.Menu).
			Draw(func(d *ui.DrawContext) { drawGoalPanelElevation(d, t.Radii.MD, t) }).
			Children(func() {
				// `.goalGlyph{color:label-tertiary}` + GoalBar.tsx:137 的 size=14。
				ui.Icon(f, goalOutlineIcon()).Size(goalGlyphSize, goalGlyphSize).Color(t.LabelTertiary)
				// rc.2 `GoalBar.module.css` `.label{…flex:none…}`：固定标签**不许收缩**。
				// 不写 Shrink(0) 时 viva 默认 flex-shrink:1，长目标会把
				// 「进行中的目标」压成 1 个字（本轮 vr3 实测 78px → 12px）。
				ui.Text(f, goalLabelText).FontSize(goalLabelFont).LineHeight(goalLabelLine).
					Weight(gfx.WeightMedium).Color(t.LabelPrimary).Shrink(0)
				// `.objective`：13/20、label-primary-dimmed、一行省略。
				ui.Text(f, goal).FontSize(goalObjectiveFont).LineHeight(goalObjectiveLine).
					Color(t.LabelPrimaryDimmed).Grow(1).Ellipsis()
			})
	})
}

// drawGoalPanelElevation 画官方 `--dsw-elevation-panel`（GoalBar.module.css 的
// `.bar`），不拿 ShadowXS 的 0/1/2/5% 替代。数值一处住在 `Shadows.Panel`。
func drawGoalPanelElevation(d *ui.DrawContext, radius float32, t *pluginapi.Theme) {
	dscore.PaintElevation(d, radius, t.Shadows.Panel, t.BorderL1)
}
