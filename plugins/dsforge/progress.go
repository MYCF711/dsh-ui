// progress.go —— `conversation.input.dock` 上的**创建进度卡**（官方 `WorktreeProgress`）。
//
// # 座位（官方就是这一格）
//
//	OFF:…/src/client/mount.ts:392-394
//	  conversation.input.dock, id 'forge-panel.worktree', order 5
//
// 本插件的本地变更卡占着同一个槽（座位不同是已登记的差异，见 dock.go 文件头），
// 两者用不同的 Order 并存：进度卡取 4，排在本地卡（5）**之上**。官方在该槽里
// 只有进度卡一张；本地卡是本仓的搬家件，官方没有"它该在第几位"的证据，
// 所以这条相对顺序按"先讲正在发生的事"定，并登记在账本行。
//
// # 官方形状（逐条）
//
//	OFF:…/src/client/WorktreeProgress.tsx:263-280  只画 `worktree.creation` 在跑的那一支
//	                                                （creationOf：creation 非 null → running）
//	OFF:…/src/client/WorktreeProgress.module.css
//	  :3-15   .dock{width:calc(100% - 2*clearance - 4*inset); margin:0 auto}
//	  :17-41  .card{flex column; gap:6px; max-width:calc(card-max - 4*inset);
//	          padding:10px 8px 6px 12px; radius:md; bg:--dsw-specific-menu;
//	          box-shadow:--dsw-elevation-panel; border:0;
//	          color:label-primary; 13px/20px}
//	  :54-60  .header{flex; items:center; gap:8px; padding-right:4px}
//	  :62-65  .glyph{color:label-tertiary}
//	  :72-75  .title{font-weight:500}
//	  :77-86  .branch{code 12px; label-tertiary; nowrap + ellipsis}
//	  :88-95  .steps{column; gap:2px; padding:0 0 0 22px}
//	  :97-111 .step{flex; items:center; gap:8px; min-height:24px; color:label-secondary}
//	          running → label-primary；failed → state-error-primary
//	  :117-134 .done{state-success-primary} / .failed{state-error-primary}
//	          .waiting{14×14; border:1.5px label-dimmed; radius:50%}
//	  :136-152 .details{grid auto/minmax(0,1fr); gap:2px 12px; margin:2px 0 0 22px; 12px/18px}
//	          dt → label-tertiary
//	  :161-167 .footer{flex; space-between; gap:8px; min-height:28px}
//	  :169-194 .disclosure{gap:4px; height:28px; padding:0 6px; radius:sm; font-size:12px;
//	          color:label-tertiary; hover → interactive-bg-hover + label-secondary}
//	  :196-204 .chevron 12px；展开时 rotate(90deg)
//	  :206-212 .actions{inline-flex; wrap; justify flex-end; gap:4px; margin-left:auto}
//
//	文案全部取 `OFF:…/src/client/locales.ts:319-332`（逐字）。
//
// # 今天**不画**的三支（在账本行登记，不画假卡）
//
//	failed / stopped（creationOf 的另外两支，要 `worktree.failure` + `waiting`）、
//	MissingCard（要 `worktree.missing` / `created` / `restoration`）。
//
// 宿主 follow 帧里这些字段已有真值，但本轮只接 `creation`（见任务 #64）。
package dsforge

import (
	"math"
	"time"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方 `WorktreeProgress` 的中文文案（locales.ts:319-332，逐字）。
const (
	copyCreationTitle    = "正在创建工作树"
	copyCreationPrepare  = "创建分支"
	copyCreationCheckout = "检出文件"
	copyCreationDetails  = "更多详情"
	copyCreationBranch   = "分支"
	copyCreationBase     = "基于"
	copyCreationHead     = "当前 HEAD"
	copyCreationCancel   = "取消"
)

// 官方 `.card` 的几何（WorktreeProgress.module.css:17-41）。
const (
	progressCardGap      float32 = 6
	progressCardPadTop   float32 = 10
	progressCardPadRight float32 = 8
	progressCardPadBot   float32 = 6
	progressCardPadLeft  float32 = 12

	// `.dock{width:calc(100% - 2*clearance - 4*inset)}`，inset=8：座位已扣 clearance，
	// 所以这里左右各再留 2*inset（与 dsgoal 的官方 dock 卡同一算法）。
	progressDockInset  float32 = 8
	progressDockMargin float32 = 2 * progressDockInset

	// `.header{gap:8px; padding-right:4px}`。
	progressHeaderGap      float32 = 8
	progressHeaderPadRight float32 = 4
	// `.glyph` / `IconBranchOutlineRegular size={14}`（WorktreeProgress.tsx:221）。
	progressGlyphSize float32 = 14
	// `.branch,.code{font-size:12px}`，行高继承卡片 20px。
	progressCodeFont float32 = 12
	// 卡片正文 13px/20px。
	progressFont float32 = 13
	progressLine float32 = 20

	// `.steps{gap:2px; padding:0 0 0 22px}`。
	progressStepsPadLeft float32 = 22
	progressStepsGap     float32 = 2
	// `.step{gap:8px; min-height:24px}`。
	progressStepGap       float32 = 8
	progressStepMinHeight float32 = 24
	// `.waiting{width:14px; height:14px; border:1.5px}`。
	progressStepGlyph   float32 = 14
	progressWaitingRing float32 = 1.5

	// `.details{gap:2px 12px; margin:2px 0 0 22px; 12px/18px}`。
	progressDetailsGapRow float32 = 2
	progressDetailsGapCol float32 = 12
	progressDetailsMargin float32 = 2
	progressDetailsFont   float32 = 12
	progressDetailsLine   float32 = 18

	// `.footer{gap:8px; min-height:28px}`。
	progressFooterGap       float32 = 8
	progressFooterMinHeight float32 = 28
	// `.disclosure{gap:4px; height:28px; padding:0 6px; font-size:12px}`。
	progressDisclosureGap  float32 = 4
	progressDisclosureH    float32 = 28
	progressDisclosurePadX float32 = 6
	progressDisclosureFont float32 = 12
	// `.chevron` 12px（WorktreeProgress.tsx:166 `IconChevronRightOutlineRegular size={12}`）。
	progressChevronSize float32 = 12
	// `.actions{gap:4px; margin-left:auto}`。
	progressActionsGap float32 = 4
)

// drawWorktreeProgress 画官方创建进度卡。
//
// 可见性（官方 WorktreeProgress.tsx:263-268）：`worktree.creation === null` → 不画。
// 没有动作面时同样不画：卡上的"更多详情 / 取消"都是动作，画一张点不动的卡
// 与本地变更卡同一条判据（见 dock.go:85-88）。
func (p *Plugin) drawWorktreeProgress(f ui.C, env *pluginapi.Env) {
	view := forgeView(env)
	if view == nil {
		return
	}
	creation := view.Worktree.Creation
	if creation == nil {
		return
	}
	if env.Act.Forge == nil {
		return
	}
	t := p.themeOf(env)
	session := view.SessionID
	open := p.open.progressExpanded(session)

	ui.Row(f).FillWidth().MarginSides(0, progressDockMargin, 0, progressDockMargin).
		Justify(ui.Center).Children(func() {
		ui.Column(f).FillWidth().MaxWidth(progressContentWidth(env)).
			PadSides(progressCardPadTop, progressCardPadRight, progressCardPadBot, progressCardPadLeft).
			Gap(progressCardGap).Radius(t.Radii.MD).Background(t.Menu).
			// `.card` 的 `--dsw-elevation-panel` + 默认描边 `border-l1`
			// （WorktreeProgress.module.css:29-31 把 stroke 重绑成 border-l1）。
			Draw(func(d *ui.DrawContext) {
				dscore.PaintElevation(d, t.Radii.MD, t.Shadows.Panel, t.BorderL1)
			}).
			Children(func() {
				// ── .header ──
				ui.Row(f).FillWidth().Gap(progressHeaderGap).Items(ui.Center).
					PadSides(0, progressHeaderPadRight, 0, 0).Children(func() {
					// `.glyph` 是官方 `IconBranchOutlineRegular size={14}`
					// （WorktreeProgress.tsx:221）；那枚素材含两枚 `<circle>`，viva 路径
					// 词汇没有弧，素材库登记为 Unsupported（dsassets/registry.go），
					// 所以**不画残图**——与账本 #64 ④（分支图标）同一条差异。
					ui.Text(f, copyCreationTitle).FontSize(progressFont).LineHeight(progressLine).
						Weight(gfx.WeightMedium).Color(t.LabelPrimary).Shrink(0)
					ui.Text(f, creation.Branch).FontSize(progressCodeFont).
						LineHeight(progressLine).Mono().Color(t.LabelTertiary).Grow(1).Ellipsis()
				})

				// ── .steps：prepare / checkout ──
				ui.Column(f).FillWidth().Gap(progressStepsGap).
					PadSides(0, 0, 0, progressStepsPadLeft).Children(func() {
					drawCreationStep(f, t, env, creation, pluginapi.ForgeCreationPrepare, copyCreationPrepare)
					drawCreationStep(f, t, env, creation, pluginapi.ForgeCreationCheckout, copyCreationCheckout)
				})

				// ── .details（展开态；官方 `(open || failed)`，本卡只画 running 一支）──
				if open {
					drawCreationDetails(f, t, creation)
				}

				// ── .footer ──
				ui.Row(f).FillWidth().Gap(progressFooterGap).Items(ui.Center).
					MinHeight(progressFooterMinHeight).Children(func() {
					disclosure := ui.Row(f).Height(progressDisclosureH).Shrink(0).
						PadSides(0, progressDisclosurePadX, 0, progressDisclosurePadX).
						Gap(progressDisclosureGap).Items(ui.Center).Radius(t.Radii.SM).
						Hover(ui.Look{}.Background(t.Hover)).
						Hit(env.Tree.ID(env.Feat, "forge-creation-details"), copyCreationDetails)
					disclosure.Children(func() {
						// `.chevron`（12px）展开时 rotate(90deg)：右向箭头朝下。
						// viva 的图标不转（没有 transform），所以展开态直接画朝下的
						// 同一枚官方箭头（`chevron-down-outline`），几何等价。
						icon, ok := chevronRightIcon()
						if open {
							icon, ok = chevronDownIcon()
						}
						if ok {
							ui.Icon(f, icon).Size(progressChevronSize, progressChevronSize).
								Color(t.LabelTertiary)
						}
						ui.Text(f, copyCreationDetails).FontSize(progressDisclosureFont).
							LineHeight(progressLine).Color(t.LabelTertiary)
					})
					if disclosure.Clicked() {
						p.open.toggleProgress(session)
					}
					// 官方 `creation.held` 才画"取消"（WorktreeProgress.tsx:176-180）：
					// 停掉这次创建，消息继续等（stopCreation 的官方语义）。
					if creation.Held {
						ui.Spacer(f).Grow(1)
						cancel := ui.Button(f, copyCreationCancel).Ghost().Small()
						if icon, ok := closeIcon(t.LabelSecondary); ok {
							cancel = cancel.Icon(icon)
						}
						if cancel.Clicked() {
							env.Act.Forge.StopCreation()
						}
					}
				})
			})
	})
}

// drawCreationStep 画官方 `.step` 一行：14px 状态字形 + 步骤文案。
//
// 逐字照 `WorktreeProgress.tsx:59-80` 的 `stepState`：
//
//	phase === current            → running（failed 时才变红；本卡只画 running 一支）
//	phase === 'prepare'（更早）  → done
//	phase === 'checkout'（更晚） → waiting
func drawCreationStep(f ui.C, t *pluginapi.Theme, env *pluginapi.Env,
	creation *pluginapi.ForgeWorktreeCreation, phase, label string) {
	state := stepWaiting
	switch {
	case phase == creation.Phase:
		state = stepRunning
	case phase == pluginapi.ForgeCreationPrepare:
		state = stepDone
	}
	color := t.LabelSecondary
	switch state {
	case stepRunning:
		color = t.LabelPrimary
	case stepFailed:
		color = t.Error
	}
	row := ui.Row(f).FillWidth().Gap(progressStepGap).Items(ui.Center).
		MinHeight(progressStepMinHeight)
	row.Children(func() {
		drawStepGlyph(f, t, env, state)
		ui.Text(f, label).FontSize(progressFont).LineHeight(progressLine).Color(color)
	})
}

// 官方 `StepState` 的四值（WorktreeProgress.tsx:43）。本卡只会出现前三档，
// failed 留给同一张卡的失败态（今天不画）。
const (
	stepDone = iota
	stepRunning
	stepWaiting
	stepFailed
)

// drawStepGlyph 画步骤字形（WorktreeProgress.tsx:82-91）：
//
//	done    IconCheckOutlineRegular size=14（.done 绿色）
//	running StateDot state=ongoing size=14
//	waiting 空环：14×14、1.5px label-dimmed、radius 50%
//	failed  IconCloseOutlineRegular size=14（.failed 红色）
func drawStepGlyph(f ui.C, t *pluginapi.Theme, env *pluginapi.Env, state int) {
	switch state {
	case stepDone:
		if icon, ok := checkIcon(); ok {
			ui.Icon(f, icon).Size(progressStepGlyph, progressStepGlyph).Color(t.Success)
			return
		}
	case stepRunning:
		ui.Icon(f, stateDotOngoingIconAt(t.LabelTertiary, stateDotPhase(env))).
			Size(progressStepGlyph, progressStepGlyph).Color(t.LabelTertiary)
		return
	case stepFailed:
		if icon, ok := closeIcon(t.Error); ok {
			ui.Icon(f, icon).Size(progressStepGlyph, progressStepGlyph).Color(t.Error)
			return
		}
	}
	// waiting（以及素材编译不出来时的兜底）：官方那个空环。
	ui.Slot(f).Size(progressStepGlyph, progressStepGlyph).Shrink(0).
		Radius(progressStepGlyph/2).Border(progressWaitingRing, t.LabelDimmed)
}

// drawCreationDetails 画官方 `.details`：两列 grid（dt 第三级色 / dd 正文）。
//
// 官方 WorktreeProgress.tsx:109-128：只有 `branch !== null` 才出现分支那一行，
// 基准那一行恒在（`from ?? t('creation.head')`）。
func drawCreationDetails(f ui.C, t *pluginapi.Theme, creation *pluginapi.ForgeWorktreeCreation) {
	ui.Column(f).FillWidth().Gap(progressDetailsGapRow).
		PadSides(progressDetailsMargin, 0, 0, progressDetailsPadLeft).Children(func() {
		drawDetailRow := func(label, value string, mono bool) {
			ui.Row(f).FillWidth().Gap(progressDetailsGapCol).Items(ui.Center).Children(func() {
				ui.Text(f, label).FontSize(progressDetailsFont).LineHeight(progressDetailsLine).
					Color(t.LabelTertiary).Shrink(0)
				text := ui.Text(f, value).FontSize(progressDetailsFont).
					LineHeight(progressDetailsLine).Color(t.LabelSecondary).Grow(1).Ellipsis()
				if mono {
					text = text.Mono()
				}
			})
		}
		if creation.Branch != "" {
			drawDetailRow(copyCreationBranch, creation.Branch, true)
		}
		base := copyCreationHead
		if creation.From != nil && *creation.From != "" {
			base = *creation.From
		}
		drawDetailRow(copyCreationBase, base, true)
	})
}

// progressDetailsPadLeft 是 `.details{margin:2px 0 0 22px}` 的左边距。
const progressDetailsPadLeft = progressStepsPadLeft

// progressContentWidth 是本卡宽度上限。
//
// 官方 `.card{max-width:calc(var(--dsh-composer-card-max-width) - 4*dock-inset)}`，
// 而 `--dsh-composer-card-max-width` = 内容宽 + 32（dsgoal 文件头同一推导），
// 4*inset = 32，所以上限就是 `--dsh-chat-content-width`
// = clamp(680, 主列宽×0.64, 920)（与 dscomposer / dsgoal 同一条官方 clamp）。
func progressContentWidth(env *pluginapi.Env) float32 {
	column := env.MainWidth
	if env.Geometry.MainWidth > 0 {
		column = env.Geometry.MainWidth
	}
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

// ── StateDot(ongoing) 的官方几何（与 dsagentteam/icons.go 同一份取值）────────

// 官方 StateDot ongoing 的 SVG/CSS 几何（不是新造图标）：
// `index-ghOpC9uV.js:129`：viewBox=24、cx=cy=12、r=9.5、默认 14px；
// `index-CSAdOBx6.css:1`：stroke=2、linecap=round、track opacity=.25、dash=18、offset=-3。
// 一圈 1.5s linear infinite（`_dsh-state-dot-spin_1i3xo_1`）。
const (
	stateDotSpinnerViewBox = 24
	stateDotSpinnerCenter  = 12
	stateDotSpinnerRadius  = 9.5
	stateDotSpinnerStroke  = 2
	stateDotSpinnerDash    = 18
	stateDotSpinnerOffset  = -3
	stateDotTrackAlpha     = 0.25
)

// stateDotSpinnerPeriod 是官方 spinner 一圈的时长。
const stateDotSpinnerPeriod = 1500 * time.Millisecond

// stateDotPhase 返回这一帧的旋转相位 [0,1)，并让渲染器再排一帧。
//
// 与 dsagentteam 的 `stateDotPhase` 同一判据（`Canvas.RequestFrame()` 排帧，
// 相位取 `Canvas.Now()`；`ReduceMotion()` 时画静止弧）。
func stateDotPhase(env *pluginapi.Env) float32 {
	if env == nil || env.Canvas == nil {
		return 0
	}
	if env.Canvas.ReduceMotion() || !env.Canvas.HasClock() {
		return 0
	}
	env.Canvas.RequestFrame()
	now := env.Canvas.Now()
	if now <= 0 {
		return 0
	}
	return float32(now%stateDotSpinnerPeriod) / float32(stateDotSpinnerPeriod)
}

// stateDotOngoingIconAt 是带旋转相位的 StateDot ongoing（phase ∈ [0,1) 为一圈）。
func stateDotOngoingIconAt(tint gfx.Color, phase float32) gfx.Icon {
	circumference := 2 * math.Pi * stateDotSpinnerRadius
	start := -stateDotSpinnerOffset/circumference*360 + float64(phase)*360
	sweep := stateDotSpinnerDash / circumference * 360
	track := tint
	track.Alpha *= stateDotTrackAlpha
	return gfx.Icon{
		ViewBox: stateDotSpinnerViewBox,
		Stroke:  stateDotSpinnerStroke,
		Cap:     gfx.CapRound,
		Commands: stateDotArcCommands(stateDotSpinnerCenter, stateDotSpinnerCenter,
			stateDotSpinnerRadius, float32(start), float32(sweep)),
		Layers: []gfx.IconLayer{{
			Fill: track, FillRule: gfx.PathFillEvenOdd,
			Commands: stateDotRingCommands(stateDotSpinnerCenter, stateDotSpinnerCenter,
				stateDotSpinnerRadius+stateDotSpinnerStroke/2,
				stateDotSpinnerRadius-stateDotSpinnerStroke/2),
		}},
	}
}

// stateDotArcCommands 按官方 circle+dasharray 参数换算静态弧；每段 ≤90°。
func stateDotArcCommands(cx, cy, r, startDeg, sweepDeg float32) []gfx.PathCommand {
	if sweepDeg <= 0 {
		return nil
	}
	segments := int(math.Ceil(float64(sweepDeg / 90)))
	step := sweepDeg / float32(segments)
	angle := float64(startDeg) * math.Pi / 180
	commands := make([]gfx.PathCommand, 0, segments*4)
	commands = append(commands, gfx.MoveTo(
		cx+r*float32(math.Cos(angle)), cy+r*float32(math.Sin(angle))))
	for i := 0; i < segments; i++ {
		a0, a1 := angle, angle+float64(step)*math.Pi/180
		k := 4.0 / 3.0 * math.Tan((a1-a0)/4)
		p0x, p0y := float64(cx)+float64(r)*math.Cos(a0), float64(cy)+float64(r)*math.Sin(a0)
		p3x, p3y := float64(cx)+float64(r)*math.Cos(a1), float64(cy)+float64(r)*math.Sin(a1)
		commands = append(commands, gfx.CubicTo(
			float32(p0x-k*float64(r)*math.Sin(a0)), float32(p0y+k*float64(r)*math.Cos(a0)),
			float32(p3x+k*float64(r)*math.Sin(a1)), float32(p3y-k*float64(r)*math.Cos(a1)),
			float32(p3x), float32(p3y)))
		angle = a1
	}
	return commands
}

func stateDotRingCommands(cx, cy, outer, inner float32) []gfx.PathCommand {
	return append(stateDotCircleCommands(cx, cy, outer), stateDotCircleCommands(cx, cy, inner)...)
}

func stateDotCircleCommands(cx, cy, r float32) []gfx.PathCommand {
	k := 0.5522847498 * r
	return []gfx.PathCommand{
		gfx.MoveTo(cx+r, cy),
		gfx.CubicTo(cx+r, cy+k, cx+k, cy+r, cx, cy+r),
		gfx.CubicTo(cx-k, cy+r, cx-r, cy+k, cx-r, cy),
		gfx.CubicTo(cx-r, cy-k, cx-k, cy-r, cx, cy-r),
		gfx.CubicTo(cx+k, cy-r, cx+r, cy-k, cx+r, cy),
		gfx.ClosePath(),
	}
}
