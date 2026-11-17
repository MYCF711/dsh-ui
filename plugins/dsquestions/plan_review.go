package dsquestions

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/planreview"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/ui"
)

// 官方 PlanReviewPanel.module.css:1-109 与 locales.ts:32-36。
const (
	planReviewHeader              = "计划待审"
	planReviewApprove             = "同意执行"
	planReviewDiscuss             = "要求修改"
	planReviewUnavailable         = "当前无法提交，请稍候再试。"
	planReviewStripPadY   float32 = 12
	planReviewStripPadX   float32 = 16
	planReviewStripGap    float32 = 8
	// StateDot 默认 size：warning=10、ongoing=14（index-ghOpC9uV.js:129）。
	planReviewStateDot      float32 = 10
	planReviewStateOngoing  float32 = 14
	planReviewSummaryPadTop float32 = 14
	planReviewSummaryPadX   float32 = 16
	planReviewSummaryPadBot float32 = 12
	planReviewTitleFont     float32 = 15
	planReviewTitleLine     float32 = 22
	planReviewDescFont      float32 = 14
	planReviewDescLine      float32 = 24
	planReviewDescGap       float32 = 8
	planReviewFooterPadTop  float32 = 8
	planReviewFooterPadX    float32 = 16
	planReviewFooterPadBot  float32 = 12
	planReviewFooterGap     float32 = 12
	planReviewActionsGap    float32 = 8
	// 共享 Button 默认 size="md"：index-CSAdOBx6.css:1 的
	// _button_1ulci_2 / _md_1ulci_23 / _icon_1ulci_78。
	planReviewButtonHeight float32 = 36
	planReviewButtonPadX   float32 = 14
	planReviewButtonGap    float32 = 4
	planReviewDiscussGap   float32 = 6
	planReviewButtonFont   float32 = 14
	planReviewButtonLine   float32 = 22
	planReviewIconSlot     float32 = 16
	planReviewIconGlyph    float32 = 14
	planReviewDisabled     float32 = 0.4
	// PlanReviewPanel.module.css:94-102 的整窗 ≤720px footer 值。
	planReviewCompactViewport float32 = 720
	planReviewCompactPadX     float32 = 12
	planReviewCompactPadBot   float32 = 10
	// 官方 PlanReviewPanel.module.css `.Nksazq_card{--dsw-elevation-stroke-color:
	// var(--dsw-alias-border-l2);box-shadow:var(--dsw-elevation-panel);border:0}`
	// （同名取值另见 gradient-shadow-text.css:33-35）。原来这里手抄了 panel 的五个数，
	// 已并进 `pluginapi.Shadows.Panel`；描边色由调用方按官方重绑传 `BorderL2`。
)

func planReviewIdentity(env *pluginapi.Env, review planreview.Review) string {
	key := review.CallID
	if key == "" {
		key = review.RequestKey
	}
	return env.Data.Cur + "\x00" + key
}

func (p *Plugin) reviewState(key string) (busy bool, failure string, hidden bool) {
	p.review.mu.Lock()
	defer p.review.mu.Unlock()
	return p.review.busy[key], p.review.err[key], p.review.answered[key] || p.review.discussed[key]
}

// drawPlanReviewOfficial 是 PlanReviewPanel.tsx:57-90 的 Go+Viva 结构：
// warn strip（含 plan-review.actions 子槽）→ 标题/首段 → 要求修改/同意执行。
func (p *Plugin) drawPlanReviewOfficial(f ui.C, env *pluginapi.Env, review planreview.Review) {
	key := planReviewIdentity(env, review)
	busy, failure, hidden := p.reviewState(key)
	if hidden {
		return
	}
	t := p.theme
	if env.Theme != nil {
		t = env.Theme
	}
	title, description := planreview.Summary(review.Plan)
	footerPadX, footerPadBot, footerAlign := planReviewFooterLayout(env)
	if env.Act.AnswerQuestion == nil || review.CallID == "" {
		failure = planReviewUnavailable
	}
	ui.Column(f).FillWidth().
		PadSides(questionFramePadTop, questionFramePadX, questionFramePadBottom, questionFramePadX).
		Children(func() {
			card := ui.Column(f).FillWidth().Radius(t.Radii.XL).Background(t.InputMajor).
				Draw(func(d *ui.DrawContext) { drawPlanReviewElevation(d, t.Radii.XL, t) })
			card.Children(func() {
				ui.Column(f).FillWidth().Radius(t.Radii.XL).Clip().Children(func() {
					// `.strip`：警告色整行；查看全文挂在官方 list/session 子槽。
					ui.Row(f).FillWidth().Items(ui.Center).Gap(planReviewStripGap).
						PadSides(planReviewStripPadY, planReviewStripPadX, planReviewStripPadY, planReviewStripPadX).
						Background(t.WarnTertiary).Children(func() {
						dotColor, dotSize := t.Warn, planReviewStateDot
						if busy {
							dotColor, dotSize = t.Info, planReviewStateOngoing
						}
						p.prims.Dot(f, dotColor, dotSize)
						ui.Text(f, planReviewHeader).FontSize(14).LineHeight(20).Color(t.Warn)
						ui.Spacer(f)
						if env.DrawSlot != nil {
							env.DrawSlot(f, uikernel.OffPlanReviewActions)
						}
					})
					ui.Column(f).FillWidth().
						PadSides(planReviewSummaryPadTop, planReviewSummaryPadX, planReviewSummaryPadBot, planReviewSummaryPadX).
						Children(func() {
							ui.Text(f, title).FontSize(planReviewTitleFont).LineHeight(planReviewTitleLine).
								Weight(500).Color(t.LabelPrimary).Ellipsis()
							if description != "" {
								ui.Text(f, description).FontSize(planReviewDescFont).LineHeight(planReviewDescLine).
									Color(t.LabelSecondary).MarginTop(planReviewDescGap).MaxLines(2)
							}
						})
					ui.Row(f).FillWidth().Items(footerAlign).Gap(planReviewFooterGap).
						PadSides(planReviewFooterPadTop, footerPadX, footerPadBot, footerPadX).
						Children(func() {
							ui.Row(f).Grow(1).MinHeight(16).Children(func() {
								if failure != "" {
									ui.Text(f, failure).FontSize(11).LineHeight(16).Color(t.Error)
								}
							})
							ui.Row(f).Items(ui.Center).Gap(planReviewActionsGap).Children(func() {
								p.drawReviewDiscuss(f, env, key, busy, t)
								p.drawReviewApprove(f, env, key, review, busy, failure == planReviewUnavailable, t)
							})
						})
				})
			})
		})
}

func (p *Plugin) drawReviewDiscuss(f ui.C, env *pluginapi.Env, key string, busy bool, t *pluginapi.Theme) {
	button := ui.Row(f).Height(planReviewButtonHeight).Items(ui.Center).Gap(planReviewDiscussGap).
		PadSides(0, planReviewButtonPadX, 0, planReviewButtonPadX).
		Border(0.5, t.BorderL3).Radius(t.Radii.MD).
		Focus(ui.Look{}.Ring(2, t.Info)).
		Hit(env.Tree.ID(env.Feat, "plan-review-discuss"), planReviewDiscuss)
	if busy {
		button = button.Opacity(planReviewDisabled)
	} else {
		button = button.Hover(ui.Look{}.Background(t.Hover))
		if button.Clicked() {
			p.review.mu.Lock()
			if p.review.discussed == nil {
				p.review.discussed = map[string]bool{}
			}
			p.review.discussed[key] = true
			p.review.mu.Unlock()
			if env.Act.Invalidate != nil {
				env.Act.Invalidate()
			}
		}
	}
	button.Children(func() {
		if icon, ok := dsassets.PlanReviewEdit(t.LabelPrimary); ok {
			ui.Row(f).Width(planReviewIconSlot).Height(planReviewIconSlot).Center().Children(func() {
				ui.Icon(f, icon).Width(planReviewIconGlyph).Height(planReviewIconGlyph).Color(t.LabelPrimary)
			})
		}
		ui.Text(f, planReviewDiscuss).FontSize(planReviewButtonFont).LineHeight(planReviewButtonLine).Color(t.LabelPrimary)
	})
}

func (p *Plugin) drawReviewApprove(f ui.C, env *pluginapi.Env, key string, review planreview.Review, busy, unavailable bool, t *pluginapi.Theme) {
	button := ui.Row(f).Height(planReviewButtonHeight).Items(ui.Center).Gap(planReviewButtonGap).
		PadSides(0, planReviewButtonPadX, 0, planReviewButtonPadX).
		Radius(t.Radii.MD).Background(t.ButtonPrimaryFill).
		Focus(ui.Look{}.Ring(2, t.Info)).
		Hit(env.Tree.ID(env.Feat, "plan-review-approve"), review.Approve.Description)
	if busy || unavailable {
		button = button.Opacity(planReviewDisabled)
	} else {
		button = button.Hover(ui.Look{}.Background(t.ButtonPrimaryHover))
		if button.Clicked() {
			p.answerPlanReview(env, key, review)
		}
	}
	button.Children(func() {
		ui.Text(f, planReviewApprove).FontSize(planReviewButtonFont).LineHeight(planReviewButtonLine).Color(t.ButtonPrimaryFg)
	})
}

func planReviewFooterLayout(env *pluginapi.Env) (padX, padBottom float32, align ui.Align) {
	viewport := env.Bounds.Width
	if viewport <= 0 {
		viewport = env.Geometry.Width
	}
	if viewport > 0 && viewport <= planReviewCompactViewport {
		return planReviewCompactPadX, planReviewCompactPadBot, ui.End
	}
	return planReviewFooterPadX, planReviewFooterPadBot, ui.Center
}

// drawPlanReviewElevation 画官方 `--dsw-elevation-panel`（PlanReviewPanel.module.css
// 的 `.card`），不拿 Viva 预设阴影档位替代。数值一处住在 `Shadows.Panel`。
func drawPlanReviewElevation(d *ui.DrawContext, radius float32, t *pluginapi.Theme) {
	dscore.PaintElevation(d, radius, t.Shadows.Panel, t.BorderL2)
}

func (p *Plugin) answerPlanReview(env *pluginapi.Env, key string, review planreview.Review) {
	answer := env.Act.AnswerQuestion
	if answer == nil || review.CallID == "" {
		return
	}
	p.review.mu.Lock()
	if p.review.busy == nil {
		p.review.busy = map[string]bool{}
	}
	if p.review.busy[key] {
		p.review.mu.Unlock()
		return
	}
	p.review.busy[key] = true
	if p.review.err == nil {
		p.review.err = map[string]string{}
	}
	p.review.err[key] = ""
	p.review.mu.Unlock()
	invalidate := env.Act.Invalidate
	go func() {
		_, err := answer(review.CallID, apiclient.QuestionAnswer{Answers: []apiclient.QuestionAnswerItem{{
			ID: review.ID, Selected: []string{review.Approve.Label},
		}}})
		p.review.mu.Lock()
		p.review.busy[key] = false
		if err != nil {
			p.review.err[key] = err.Error()
		} else {
			// true=宿主已接受并投递；false=问题已被别的客户端回答。两者都不能再点。
			if p.review.answered == nil {
				p.review.answered = map[string]bool{}
			}
			p.review.answered[key] = true
		}
		p.review.mu.Unlock()
		if invalidate != nil {
			invalidate()
		}
	}()
}
