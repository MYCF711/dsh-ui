// board.go —— TeamBoard：子代理目录 footer 里的只读共享任务板
// （官方 `TeamBoard.tsx` + `TeamBoard.module.css` 全量几何）。
//
// # 守卫（官方写死，顺序也一样）
//
//	!projection?.enabled            → null                        (TeamBoard.tsx:86)
//	非 deleted 任务为空             → 只有 failure 或 null        (:87,91)
//	展开互斥：expandedTaskId 单值，开一个就关上一个                (:85,105)
//
// # 几何（§4.1 逐条）
//
//	.board          padding 8px 7px；有成员目录时 margin-top 4px
//	.header         flex; gap 4px 7px; margin 0 0 4px
//	.title          13px/24px/500 label-primary
//	.progress       13px/20px/400 label-tertiary; margin-inline-start 4px
//	.taskRow        gap 7px; width(100%+12px); margin 0 -6px; padding 7px 6px;
//	                radius md; 13px/17px
//	.glyph          14×17
//	.chevron        28×17（展开时 rotate 180）
//	.details        padding 0 0 8px 21px; 12px/18px
//	.owner          flex; gap 4px; .description + .owner margin-top 6px
//	.error          flex; gap 7px; padding 8px 0; 12px/18px
package dsagentteam

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/ui"
)

// 官方几何常量（TeamBoard.module.css）。
const (
	boardPadV      = 8 // .board padding:8px 7px
	boardPadH      = 7
	boardMemberGap = 4 // .board[data-with-members='true']{margin-top:4px}
	headerGap      = 7 // .header gap:4px 7px（行间距 4、列间距 7）
	headerBottom   = 4 // .header margin:0 0 4px
	titleSize      = 13
	titleLine      = 24
	progressLine   = 20
	progressLead   = 4 // .progress margin-inline-start:4px
	rowGap         = 7 // .taskRow gap:7px
	rowPadV        = 7 // .taskRow padding:7px 6px
	rowPadH        = 6
	rowBleed       = 6 // width:calc(100%+12px); margin:0 -6px
	rowSize        = 13
	rowLine        = 17
	glyphW         = 14
	glyphH         = 17
	chevronW       = 28
	detailsPadB    = 8 // .details padding:0 0 8px 21px
	detailsPadL    = 21
	detailsSize    = 12
	detailsLine    = 18
	ownerGap       = 4
	descOwnerGap   = 6
	errGap         = 7
	errPadV        = 8
	dotSize        = 6
)

// dotColor 是官方 `StateDot` 四值映射（结构树 §2.1）：
//
//	pending → idle（中性） / in_progress → ongoing（品牌色） / completed → done（成功色）
//	被依赖阻塞的 pending → warning（覆盖上一条） / failure → error
//
// 颜色取自本仓令牌；官方 StateDot 的具体色值是 primitives 包的令牌，这里用同义令牌。
func dotColor(t *pluginapi.Theme, task TeamTask) ui.Color {
	if task.blocked() {
		return t.Warn
	}
	switch task.Status {
	case StatusInProgress:
		return t.BrandPrimary
	case StatusCompleted:
		return t.Success
	default:
		return t.LabelTertiary
	}
}

// drawBoard 画任务板。slot ① 的 painter。
func (p *Plugin) drawBoard(f ui.C, env *pluginapi.Env) {
	view := p.team.View()
	if !view.ProjectionKnown || !view.Projection.Enabled {
		return
	}
	t := p.themeFor(env)
	tasks := visibleTasks(view.Projection.Tasks)
	failure := view.Projection.Failure
	if len(tasks) == 0 && failure == "" {
		return
	}

	board := ui.Column(f).FillWidth().PadSides(boardPadV, boardPadH, boardPadV, boardPadH)
	if view.Members > 0 {
		// 有成员目录时与上方留 4px，**不画分割线**（官方 README）。
		board = board.MarginSides(boardMemberGap, 0, 0, 0)
	}
	board.Children(func() {
		if len(tasks) > 0 {
			ui.Row(f).FillWidth().Items(ui.Center).Gap(headerGap).
				MarginSides(0, 0, headerBottom, 0).Children(func() {
				ui.Icon(f, checklistIcon()).Size(14, 14).Color(t.LabelTertiary)
				ui.Text(f, zhTasks).FontSize(titleSize).LineHeight(titleLine).Weight(500).
					Color(t.LabelPrimary)
				ui.Text(f, progressLabel(tasks)).FontSize(titleSize).LineHeight(progressLine).
					Color(t.LabelTertiary).MarginSides(0, 0, 0, progressLead).Grow(1)
			})
		}
		if failure != "" {
			// `.error[role=alert]`：StateDot(error) + 文案。
			ui.Row(f).FillWidth().Items(ui.Start).Gap(errGap).
				PadSides(errPadV, 0, errPadV, 0).Children(func() {
				p.drawDot(f, t, t.Error)
				ui.Text(f, formatMessage(zhFailure, failure)).FontSize(detailsSize).
					LineHeight(detailsLine).Color(t.Error).Grow(1)
			})
		}
		if len(tasks) > 0 {
			ui.Column(f).FillWidth().Children(func() {
				for _, task := range tasks {
					p.drawTaskRow(f, env, t, task)
				}
			})
		}
	})
}

// drawTaskRow 画一行任务 + 可展开的详情（官方 TaskRow）。
func (p *Plugin) drawTaskRow(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, task TeamTask) {
	expanded := p.team.ExpandedTaskID() == task.ID
	rowID := env.Tree.ID(env.Feat, "team-task-"+task.ID)

	ui.Column(f).FillWidth().Children(func() {
		row := ui.Row(f).Items(ui.Center).Gap(rowGap).
			FillWidth().MarginSides(0, -rowBleed, 0, -rowBleed).
			PadSides(rowPadV, rowPadH, rowPadV, rowPadH).
			Radius(t.Radii.MD)
		// 整行是命中区（官方 `<button className=.taskRow>`）。
		if row.Hit(rowID, task.Subject).Clicked() {
			p.team.ToggleExpanded(task.ID)
		}
		row.Children(func() {
			// .glyph 14×17：圆点 + aria-label（被阻塞 → t('blocked')）。
			glyphLabel := statusLabel(task.Status)
			if task.blocked() {
				glyphLabel = zhBlocked
			}
			ui.Row(f).Width(glyphW).Height(glyphH).Center().
				Hit(env.Tree.ID(env.Feat, "team-task-glyph-"+task.ID), glyphLabel).Children(func() {
				p.drawDot(f, t, dotColor(t, task))
			})
			// .subject：13/17，未展开时单行省略；展开时整段可换行。
			subject := ui.Text(f, task.Subject).FontSize(rowSize).LineHeight(rowLine).
				Color(t.LabelPrimary).Grow(1)
			if !expanded {
				subject.Ellipsis()
			}
			// .chevron 28×17（展开时官方 rotate(180deg)；viva 没有 transform，
			// 这里展开态不旋转——可见差异登记在报告里）。
			ui.Row(f).Width(chevronW).Height(glyphH).Center().Children(func() {
				ui.Icon(f, chevronDownIcon()).Size(14, 14).Color(t.LabelTertiary)
			})
		})
		if expanded {
			ui.Column(f).FillWidth().PadSides(0, 0, detailsPadB, detailsPadL).Children(func() {
				if task.Description != "" {
					ui.Text(f, task.Description).FillWidth().FontSize(detailsSize).
						LineHeight(detailsLine).Color(t.LabelSecondary)
				}
				owner := ui.Row(f).FillWidth().Items(ui.Start).Gap(ownerGap)
				if task.Description != "" {
					owner = owner.MarginSides(descOwnerGap, 0, 0, 0)
				}
				owner.Children(func() {
					ui.Text(f, zhOwner+"：").FontSize(detailsSize).LineHeight(detailsLine).
						Color(t.LabelSecondary)
					ui.Text(f, task.ownerLabel()).FontSize(detailsSize).LineHeight(detailsLine).
						Color(t.LabelSecondary).Grow(1).Ellipsis()
				})
			})
		}
	})
}

// drawDot 画官方 StateDot 的等价物（6px 实心圆）。
func (p *Plugin) drawDot(f ui.C, t *pluginapi.Theme, color ui.Color) {
	_ = t
	ui.Row(f).Width(dotSize).Height(dotSize).Radius(dotSize / 2).Background(color)
}
