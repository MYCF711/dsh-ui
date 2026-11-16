package dsquestions

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/planreview"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// 官方 PlanReviewPanel.module.css:1-109 与 locales.ts:32-36。
const (
	planReviewHeader                = "计划待审"
	planReviewApprove               = "同意执行"
	planReviewDiscuss               = "要求修改"
	planReviewUnavailable           = "当前无法提交，请稍候再试。"
	planReviewStripPadY     float32 = 12
	planReviewStripPadX     float32 = 16
	planReviewStripGap      float32 = 8
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
	planReviewButtonHeight  float32 = 32
	planReviewButtonPadX    float32 = 12
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
	if env.Act.AnswerQuestion == nil || review.CallID == "" {
		failure = planReviewUnavailable
	}
	ui.Column(f).FillWidth().
		PadSides(questionFramePadTop, questionFramePadX, questionFramePadBottom, questionFramePadX).
		Children(func() {
			ui.Column(f).FillWidth().Radius(t.Radii.XL).Background(t.InputMajor).Children(func() {
				// `.strip`：警告色整行；查看全文挂在官方 list/session 子槽。
				ui.Row(f).FillWidth().Items(ui.Center).Gap(planReviewStripGap).
					PadSides(planReviewStripPadY, planReviewStripPadX, planReviewStripPadY, planReviewStripPadX).
					Background(t.WarnTertiary).Children(func() {
					dotColor := t.WarnLabel
					if busy {
						dotColor = t.Info
					}
					p.prims.Dot(f, dotColor, 5)
					ui.Text(f, planReviewHeader).FontSize(14).LineHeight(20).Color(t.WarnLabel)
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
								Color(t.LabelSecondary).MarginTop(planReviewDescGap)
						}
					})
				ui.Row(f).FillWidth().Items(ui.Center).Gap(planReviewFooterGap).
					PadSides(planReviewFooterPadTop, planReviewFooterPadX, planReviewFooterPadBot, planReviewFooterPadX).
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
}

func (p *Plugin) drawReviewDiscuss(f ui.C, env *pluginapi.Env, key string, busy bool, t *pluginapi.Theme) {
	button := ui.Row(f).Height(planReviewButtonHeight).Items(ui.Center).
		PadSides(0, planReviewButtonPadX, 0, planReviewButtonPadX).
		Border(0.5, t.BorderL3).Radius(t.Radii.SM).
		Hit(env.Tree.ID(env.Feat, "plan-review-discuss"), planReviewDiscuss)
	if busy {
		button = button.Opacity(0.6)
	} else if button.Clicked() {
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
	button.Children(func() {
		ui.Text(f, planReviewDiscuss).FontSize(13).LineHeight(20).Color(t.LabelPrimary)
	})
}

func (p *Plugin) drawReviewApprove(f ui.C, env *pluginapi.Env, key string, review planreview.Review, busy, unavailable bool, t *pluginapi.Theme) {
	button := ui.Row(f).Height(planReviewButtonHeight).Items(ui.Center).
		PadSides(0, planReviewButtonPadX, 0, planReviewButtonPadX).
		Radius(t.Radii.SM).Background(t.ButtonPrimaryFill).
		Hit(env.Tree.ID(env.Feat, "plan-review-approve"), review.Approve.Description)
	if busy || unavailable {
		button = button.Opacity(0.6)
	} else if button.Clicked() {
		p.answerPlanReview(env, key, review)
	}
	button.Children(func() {
		ui.Text(f, planReviewApprove).FontSize(13).LineHeight(20).Color(t.ButtonPrimaryFg)
	})
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
