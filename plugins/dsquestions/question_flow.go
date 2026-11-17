package dsquestions

import (
	"fmt"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// QuestionComposer.module.css 的通用问题卡几何。共享 md Button 的 36/14/14/22
// 与计划审阅相同，直接复用 planReviewButton*，不另造一套按钮档位。
const (
	questionCardMaxHeight        float32 = 520
	questionCardViewportRatio    float32 = 0.60
	questionCompactViewport      float32 = 720
	questionHeaderCompactTop     float32 = 10
	questionHeaderCompactRight   float32 = 12
	questionHeaderCompactLeft    float32 = 18
	questionTitleCompactFont     float32 = 15
	questionTitleCompactLine     float32 = 21
	questionTitleMarginTop       float32 = 5
	questionTitleMarginX         float32 = 24
	questionTitleMaxLines                = 3
	questionDetailMarginX        float32 = 24
	questionDetailMarginBottom   float32 = 8
	questionOptionsCompactPadX   float32 = 8
	questionOptionCompactPadX    float32 = 6
	questionOptionMinHeight      float32 = 40
	questionOptionBorder         float32 = 0.5
	questionIndicatorSeat        float32 = 20
	questionIndicatorMarginTop   float32 = 2
	questionNumberFont           float32 = 12
	questionNumberLine           float32 = 18
	questionCheckboxSize         float32 = 14
	questionCheckGlyph           float32 = 12
	questionIconButtonSize       float32 = 24
	questionIconButtonRadius     float32 = 999
	questionIconButtonGap        float32 = 4
	questionIconGlyph            float32 = 14
	questionHeaderGap            float32 = 16
	questionPagerGap             float32 = 6
	questionProgressPadX         float32 = 4
	questionProgressFont         float32 = 14
	questionProgressLine         float32 = 24
	questionCustomBlockMinHeight float32 = 64
	questionCustomBlockMarginX   float32 = 12
	questionCustomBlockPadY      float32 = 8
	questionCustomBlockPadX      float32 = 12
	questionFieldFont            float32 = 14
	questionFieldLine            float32 = 24
	questionFieldMaxHeight       float32 = 144
	questionFieldFocusBorder     float32 = 1
	questionFocusRing            float32 = 2
	questionFooterMarginTop      float32 = 12
	questionFooterPadRight       float32 = 10
	questionFooterPadLeft        float32 = 18
	questionFooterCompactPadX    float32 = 10
	questionFooterGap            float32 = 12
	questionFooterActionsGap     float32 = 12
	questionFeedbackMinHeight    float32 = 16
	questionFeedbackFont         float32 = 11
	questionFeedbackLine         float32 = 16
	questionCardMinimizedBottom  float32 = 14

	questionCustomPlaceholder = "输入你的答案" // locales.ts:29
	questionSkipText          = "跳过"     // locales.ts:30
	questionNextText          = "下一题"    // locales.ts:31
	questionSubmitText        = "提交"     // dsh-client-locale/src/locales/zh.ts:21
	questionSubmittingText    = "正在提交…"  // dsh-client-locale/src/locales/zh.ts:22
	questionMinimizeText      = "收起问题卡片" // locales.ts:22
	questionMaximizeText      = "展开问题卡片" // locales.ts:23
	questionCloseText         = "放弃整组问题" // locales.ts:24
	questionPrevText          = "上一题"    // locales.ts:20
	questionPagerNextText     = "下一题"    // locales.ts:21
	questionErrorIncomplete   = "请先完成这道问题。"
	questionErrorUnanswered   = "请选择一个选项或填写自定义答案。"
)

type questionFlowState struct {
	key       string
	index     int
	selected  map[string][]string
	drafts    map[string]string
	skipped   map[string]bool
	fieldKey  string
	minimized bool
}

// Fields 把自定义答案交给壳的 TextInput；不实现这个接口时中文与键盘输入都进不来。
func (p *Plugin) Fields() []*ui.Field { return []*ui.Field{&p.custom} }

func questionFlowKey(env *pluginapi.Env, qs []Question) string {
	var b strings.Builder
	b.WriteString(env.Data.Cur)
	b.WriteByte(0)
	if len(qs) != 0 {
		b.WriteString(qs[0].CallID)
	}
	for _, q := range qs {
		b.WriteByte(0)
		b.WriteString(q.ID)
	}
	return b.String()
}

func questionDraftKey(q Question, index int) string {
	if q.ID != "" {
		return q.ID
	}
	return fmt.Sprintf("question-%d", index)
}

func (p *Plugin) prepareQuestionFlow(env *pluginapi.Env, qs []Question) {
	key := questionFlowKey(env, qs)
	if p.flow.key != key {
		p.flow = questionFlowState{key: key, selected: map[string][]string{}, drafts: map[string]string{}, skipped: map[string]bool{}}
		p.custom.SetText("")
	}
	if p.flow.index >= len(qs) {
		p.flow.index = 0
	}
	if p.flow.selected == nil {
		p.flow.selected = map[string][]string{}
	}
	if p.flow.drafts == nil {
		p.flow.drafts = map[string]string{}
	}
	if p.flow.skipped == nil {
		p.flow.skipped = map[string]bool{}
	}
}

func (p *Plugin) syncQuestionField(qs []Question) {
	key := questionDraftKey(qs[p.flow.index], p.flow.index)
	if p.flow.fieldKey == key {
		return
	}
	if p.flow.fieldKey != "" {
		p.flow.drafts[p.flow.fieldKey] = p.custom.Text()
	}
	p.custom.SetText(p.flow.drafts[key])
	p.flow.fieldKey = key
}

func (p *Plugin) moveQuestion(qs []Question, index int) {
	if index < 0 || index >= len(qs) || index == p.flow.index {
		return
	}
	p.syncQuestionField(qs)
	p.flow.drafts[p.flow.fieldKey] = p.custom.Text()
	p.flow.index = index
	p.syncQuestionField(qs)
	p.setQuestionError("")
}

func (p *Plugin) setQuestionError(value string) {
	p.review.mu.Lock()
	defer p.review.mu.Unlock()
	if p.review.err == nil {
		p.review.err = map[string]string{}
	}
	p.review.err[p.flow.key] = value
}

func questionContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (p *Plugin) chooseQuestionOption(qs []Question, optionIndex int) {
	q := qs[p.flow.index]
	if optionIndex < 0 || optionIndex >= len(q.Options) {
		return
	}
	p.syncQuestionField(qs)
	key := questionDraftKey(q, p.flow.index)
	label := q.Options[optionIndex].Label
	selected := p.flow.selected[key]
	if q.MultiSelect {
		if questionContains(selected, label) {
			next := selected[:0]
			for _, value := range selected {
				if value != label {
					next = append(next, value)
				}
			}
			selected = next
		} else {
			selected = append(append([]string(nil), selected...), label)
		}
	} else {
		selected = []string{label}
		p.custom.SetText("")
		p.flow.drafts[key] = ""
	}
	p.flow.selected[key] = selected
	p.flow.skipped[key] = false
	p.setQuestionError("")
	if !q.MultiSelect && p.flow.index < len(qs)-1 {
		p.moveQuestion(qs, p.flow.index+1)
	}
}

func (p *Plugin) currentQuestionAnswered(qs []Question) bool {
	p.syncQuestionField(qs)
	q := qs[p.flow.index]
	key := questionDraftKey(q, p.flow.index)
	custom := p.custom.Text()
	p.flow.drafts[key] = custom
	if strings.TrimSpace(custom) != "" {
		if !q.MultiSelect {
			p.flow.selected[key] = nil
		}
		return true
	}
	return len(p.flow.selected[key]) != 0
}

func (p *Plugin) skipCurrentQuestion(env *pluginapi.Env, qs []Question) {
	p.syncQuestionField(qs)
	q := qs[p.flow.index]
	key := questionDraftKey(q, p.flow.index)
	p.flow.selected[key] = nil
	p.flow.drafts[key] = ""
	p.flow.skipped[key] = true
	p.custom.SetText("")
	if p.flow.index < len(qs)-1 {
		p.moveQuestion(qs, p.flow.index+1)
		return
	}
	p.submitQuestionFlow(env, qs)
}

func (p *Plugin) submitQuestionFlow(env *pluginapi.Env, qs []Question) {
	p.syncQuestionField(qs)
	p.flow.drafts[p.flow.fieldKey] = p.custom.Text()
	for i, q := range qs {
		key := questionDraftKey(q, i)
		if len(p.flow.selected[key]) == 0 && strings.TrimSpace(p.flow.drafts[key]) == "" && !p.flow.skipped[key] {
			p.moveQuestion(qs, i)
			p.setQuestionError(questionErrorIncomplete)
			return
		}
	}
	answer := apiclient.QuestionAnswer{Answers: make([]apiclient.QuestionAnswerItem, 0, len(qs))}
	var fallback []string
	for i, q := range qs {
		key := questionDraftKey(q, i)
		selected := append([]string(nil), p.flow.selected[key]...)
		custom := strings.TrimSpace(p.flow.drafts[key])
		if !q.MultiSelect && custom != "" {
			selected = nil
		}
		if p.flow.skipped[key] {
			selected, custom = nil, ""
		}
		answer.Answers = append(answer.Answers, apiclient.QuestionAnswerItem{ID: q.ID, Selected: selected, Custom: custom})
		values := append(append([]string(nil), selected...), custom)
		if custom == "" {
			values = selected
		}
		fallback = append(fallback, strings.Join(values, "，"))
	}
	key := p.flow.key
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
	finish := func(err error) {
		p.review.mu.Lock()
		p.review.busy[key] = false
		if err != nil {
			p.review.err[key] = err.Error()
		} else {
			if p.review.answered == nil {
				p.review.answered = map[string]bool{}
			}
			p.review.answered[key] = true
		}
		p.review.mu.Unlock()
		if invalidate != nil {
			invalidate()
		}
	}
	if env.Act.AnswerQuestion != nil && len(qs) != 0 && qs[0].CallID != "" {
		go func() {
			_, err := env.Act.AnswerQuestion(qs[0].CallID, answer)
			finish(err)
		}()
		return
	}
	if env.Act.Send == nil {
		finish(fmt.Errorf("%s", planReviewUnavailable))
		return
	}
	env.Act.Send(strings.Join(fallback, "\n"))
	finish(nil)
}

func questionCompact(env *pluginapi.Env) bool {
	width := env.Bounds.Width
	if width <= 0 {
		width = env.Geometry.Width
	}
	return width > 0 && width <= questionCompactViewport
}

func questionContentWidth(env *pluginapi.Env) float32 {
	column := env.Geometry.MainWidth
	if column <= 0 {
		column = env.MainWidth
	}
	width := column * uikernel.ContentWidthRatio
	if width < uikernel.ContentWidthMin {
		width = uikernel.ContentWidthMin
	}
	if width > uikernel.ContentWidthMax {
		width = uikernel.ContentWidthMax
	}
	return width
}

func questionMaxHeight(env *pluginapi.Env) float32 {
	height := env.Bounds.Height
	if height <= 0 {
		height = env.Geometry.Height
	}
	height *= questionCardViewportRatio
	if height <= 0 || height > questionCardMaxHeight {
		return questionCardMaxHeight
	}
	return height
}

func (p *Plugin) drawQuestionFlow(f ui.C, env *pluginapi.Env, qs []Question) {
	p.prepareQuestionFlow(env, qs)
	busy, failure, hidden := p.reviewState(p.flow.key)
	if hidden {
		return
	}
	p.syncQuestionField(qs)
	q := qs[p.flow.index]
	p.cur, p.act = q, env.Act
	t := p.theme
	if env.Theme != nil {
		t = env.Theme
	}
	compact := questionCompact(env)
	ui.Row(f).FillWidth().Justify(ui.Center).
		PadSides(questionFramePadTop, questionFramePadX, questionFramePadBottom, questionFramePadX).
		Children(func() {
			card := ui.Column(f).FillWidth().MaxWidth(questionContentWidth(env)).MaxHeight(questionMaxHeight(env)).
				Radius(t.Radii.XL).Background(t.InputMajor).
				PadSides(0, 0, questionCardPadBottom, 0).
				Draw(func(d *ui.DrawContext) { drawPlanReviewElevation(d, t.Radii.XL, t.BorderL2) })
			card.Children(func() {
				ui.Column(f).FillWidth().Radius(t.Radii.XL).Clip().Children(func() {
					p.drawQuestionHeader(f, env, t, q, compact, busy)
					if p.flow.minimized {
						return
					}
					ui.Scroll(f).FillWidth().Grow(1).MinHeight(0).Children(func() {
						p.drawQuestionBody(f, env, t, qs, compact, busy)
					})
					p.drawQuestionFooter(f, env, t, qs, compact, busy, failure)
				})
			})
		})
}

func (p *Plugin) drawQuestionHeader(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, q Question, compact, busy bool) {
	top, right, left := questionHeaderPadTop, questionHeaderPadRight, questionHeaderPadLeft
	bottom := float32(0)
	if compact {
		top, right, left = questionHeaderCompactTop, questionHeaderCompactRight, questionHeaderCompactLeft
	}
	if p.flow.minimized {
		bottom = questionCardMinimizedBottom
	}
	ui.Row(f).FillWidth().Items(ui.Start).Gap(questionHeaderGap).
		PadSides(top, right, bottom, left).Children(func() {
		if q.Header != "" || p.flow.minimized {
			ui.Column(f).Grow(1).MinWidth(0).Gap(questionEyebrowGap).Children(func() {
				if q.Header != "" {
					ui.Text(f, q.Header).FontSize(questionEyebrowFont).LineHeight(questionEyebrowLine).Color(t.LabelTertiary)
				}
				if p.flow.minimized {
					titleFont, titleLine := questionTitleFont, questionTitleLine
					if compact {
						titleFont, titleLine = questionTitleCompactFont, questionTitleCompactLine
					}
					ui.Text(f, q.Question).FontSize(titleFont).LineHeight(titleLine).
						Weight(500).Color(t.LabelPrimary).MaxLines(2)
				}
			})
		} else {
			ui.Spacer(f)
		}
		ui.Row(f).Items(ui.Center).Gap(questionIconButtonGap).Children(func() {
			minimizeIcon, _ := questionChevronDownIcon()
			minimizeLabel := questionMinimizeText
			if p.flow.minimized {
				minimizeIcon, _ = questionChevronUpIcon()
				minimizeLabel = questionMaximizeText
			}
			p.drawQuestionIconButton(f, env, "question-minimize", minimizeLabel, minimizeIcon, questionIconGlyph, busy, func() {
				p.flow.minimized = !p.flow.minimized
			})
			closeIcon, _ := questionCloseIcon()
			p.drawQuestionIconButton(f, env, "question-close", questionCloseText, closeIcon, questionIconGlyph, busy, func() {
				p.review.mu.Lock()
				if p.review.discussed == nil {
					p.review.discussed = map[string]bool{}
				}
				p.review.discussed[p.flow.key] = true
				p.review.mu.Unlock()
			})
		})
	})
}

func (p *Plugin) drawQuestionBody(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, qs []Question, compact, busy bool) {
	q := qs[p.flow.index]
	titleFont, titleLine := questionTitleFont, questionTitleLine
	optionsPadX := questionOptionsPadX
	if compact {
		titleFont, titleLine = questionTitleCompactFont, questionTitleCompactLine
		optionsPadX = questionOptionsCompactPadX
	}
	ui.Text(f, q.Question).FillWidth().MarginSides(questionTitleMarginTop, questionTitleMarginX, 0, questionTitleMarginX).
		FontSize(titleFont).LineHeight(titleLine).Weight(500).Color(t.LabelPrimary).MaxLines(questionTitleMaxLines)
	if q.Detail != "" {
		ui.Text(f, q.Detail).FillWidth().MarginSides(0, questionDetailMarginX, questionDetailMarginBottom, questionDetailMarginX).
			FontSize(t.Fonts.Body).LineHeight(t.Fonts.BodyLine).Color(t.LabelPrimary)
	}
	ui.Column(f).FillWidth().MarginTop(questionOptionsMarginTop).
		PadSides(questionOptionsPadTop, optionsPadX, questionOptionsPadTop, optionsPadX).Gap(questionOptionsGap).Children(func() {
		for i, opt := range q.Options {
			p.drawQuestionOption(f, env, t, qs, i, opt, compact, busy)
		}
		if len(q.Options) == 0 {
			p.drawQuestionBlockField(f, t, busy)
		} else {
			p.drawQuestionInlineField(f, env, t, qs, compact, busy)
		}
	})
}

func (p *Plugin) drawQuestionOption(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, qs []Question, index int, opt Option, compact, busy bool) {
	q := qs[p.flow.index]
	key := questionDraftKey(q, p.flow.index)
	selected := questionContains(p.flow.selected[key], opt.Label)
	left, right := questionOptionPadX, questionOptionPadRight
	if compact {
		left, right = questionOptionCompactPadX, questionOptionCompactPadX
	}
	border := gfx.Color{}
	if selected && !q.MultiSelect {
		border = t.BorderL2
	}
	row := ui.Row(f).FillWidth().MinHeight(questionOptionMinHeight).Items(ui.Start).Gap(questionOptionGap).
		PadSides(questionOptionPadTop, right, questionOptionPadTop, left).Radius(t.Radii.MD).
		Border(questionOptionBorder, border).Focus(ui.Look{}.Ring(questionFocusRing, t.Link)).
		Hit(env.Tree.ID(env.Feat, "question-option-"+itoa(index)), opt.Label)
	if selected && !q.MultiSelect {
		row = row.Background(t.Hover)
	}
	if !busy {
		row = row.Hover(ui.Look{}.Background(t.Hover))
		if row.Clicked() {
			p.chooseQuestionOption(qs, index)
		}
	}
	row.Children(func() {
		if q.MultiSelect {
			p.drawQuestionCheckbox(f, t, selected)
		} else {
			p.drawQuestionNumber(f, t, index+1)
		}
		ui.Column(f).Grow(1).MinWidth(0).Children(func() {
			ui.Text(f, opt.Label).FillWidth().FontSize(questionOptionLabelFont).LineHeight(questionOptionLabelLine).
				Weight(500).Color(t.LabelPrimary)
			if strings.TrimSpace(opt.Description) != "" {
				ui.Text(f, opt.Description).FillWidth().FontSize(questionOptionDescFont).LineHeight(questionOptionDescLine).
					Color(t.LabelTertiary).MaxLines(2)
			}
		})
	})
}

func (p *Plugin) drawQuestionNumber(f ui.C, t *pluginapi.Theme, number int) {
	ui.Row(f).Width(questionIndicatorSeat).Height(questionIndicatorSeat).MarginTop(questionIndicatorMarginTop).
		Center().Radius(t.Radii.XS).Background(t.BgOverlay).Children(func() {
		ui.Text(f, fmt.Sprintf("%d", number)).FontSize(questionNumberFont).LineHeight(questionNumberLine).
			Weight(500).Color(t.LabelSecondary)
	})
}

func (p *Plugin) drawQuestionCheckbox(f ui.C, t *pluginapi.Theme, checked bool) {
	border, background := t.BorderL4, gfx.Color{}
	if checked {
		border, background = t.LabelPrimary, t.LabelPrimary
	}
	ui.Row(f).Width(questionIndicatorSeat).Height(questionIndicatorSeat).MarginTop(questionIndicatorMarginTop).Center().Children(func() {
		ui.Row(f).Width(questionCheckboxSize).Height(questionCheckboxSize).Center().Radius(t.Radii.XS).
			Border(questionOptionBorder, border).Background(background).Children(func() {
			if checked {
				if icon, ok := questionCheckIcon(); ok {
					// label-primary-foreground 与 button-primary-foreground 在官方两套主题中同值
					// （design-platform.css:257,409），复用已有令牌，不另造近似色。
					ui.Icon(f, icon).Width(questionCheckGlyph).Height(questionCheckGlyph).Color(t.ButtonPrimaryFg)
				}
			}
		})
	})
}

func (p *Plugin) drawQuestionInlineField(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, qs []Question, compact, busy bool) {
	q := qs[p.flow.index]
	custom := p.custom.Text()
	left, right := questionOptionPadX, questionOptionPadRight
	if compact {
		left, right = questionOptionCompactPadX, questionOptionCompactPadX
	}
	border := gfx.Color{}
	if custom != "" {
		border = t.BorderL2
	}
	row := ui.Row(f).FillWidth().MinHeight(questionOptionMinHeight).Items(ui.Start).Gap(questionOptionGap).
		PadSides(questionOptionPadTop, right, questionOptionPadTop, left).Radius(t.Radii.MD).
		Border(questionOptionBorder, border)
	if custom != "" {
		row = row.Background(t.Hover)
	}
	row.Children(func() {
		if q.MultiSelect {
			p.drawQuestionCheckbox(f, t, custom != "")
		} else {
			ui.Row(f).Width(questionIndicatorSeat).Height(questionIndicatorSeat).MarginTop(questionIndicatorMarginTop).
				Center().Radius(t.Radii.XS).Background(t.BgOverlay).Children(func() {
				if icon, ok := dsassets.PlanReviewEdit(t.LabelSecondary); ok {
					ui.Icon(f, icon).Width(questionCheckGlyph).Height(questionCheckGlyph).Color(t.LabelSecondary)
				}
			})
		}
		ui.TextFieldID(f, env.Tree.ID(env.Feat, "question-custom"), &p.custom, questionCustomPlaceholder, questionCustomPlaceholder).
			Grow(1).Height(questionFieldLine).MaxHeight(questionFieldMaxHeight).PadSides(0, 0, 0, 0).
			FontSize(questionFieldFont).LineHeight(questionFieldLine).Color(t.LabelPrimary).Disabled(busy)
	})
}

func (p *Plugin) drawQuestionBlockField(f ui.C, t *pluginapi.Theme, busy bool) {
	ui.TextArea(f, &p.custom, questionCustomPlaceholder, questionCustomPlaceholder).
		FillWidth().MinHeight(questionCustomBlockMinHeight).MaxHeight(questionFieldMaxHeight).
		MarginSides(0, questionCustomBlockMarginX, 0, questionCustomBlockMarginX).
		PadSides(questionCustomBlockPadY, questionCustomBlockPadX, questionCustomBlockPadY, questionCustomBlockPadX).
		Border(questionOptionBorder, t.BorderL4).Radius(t.Radii.LG).Background(t.BgModulePlatform).
		Focus(ui.Look{}.Border(t.Link)).
		FontSize(questionFieldFont).LineHeight(questionFieldLine).Color(t.LabelPrimary).Disabled(busy)
}

func (p *Plugin) drawQuestionFooter(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, qs []Question, compact, busy bool, failure string) {
	align, left, right := ui.Center, questionFooterPadLeft, questionFooterPadRight
	if compact {
		align, left, right = ui.End, questionFooterCompactPadX, questionFooterCompactPadX
	}
	ui.Row(f).FillWidth().Items(align).Gap(questionFooterGap).MarginTop(questionFooterMarginTop).
		PadSides(0, right, 0, left).Children(func() {
		ui.Row(f).Items(ui.Center).Gap(questionPagerGap).Children(func() {
			leftIcon, _ := questionChevronLeftIcon()
			p.drawQuestionIconButton(f, env, "question-prev", questionPrevText, leftIcon, questionIconGlyph, busy || p.flow.index == 0, func() {
				p.moveQuestion(qs, p.flow.index-1)
			})
			ui.Text(f, fmt.Sprintf("%d / %d", p.flow.index+1, len(qs))).PadSides(0, questionProgressPadX, 0, questionProgressPadX).
				FontSize(questionProgressFont).LineHeight(questionProgressLine).Weight(500).Color(t.LabelSecondary)
			rightIcon, _ := questionChevronRightIcon()
			p.drawQuestionIconButton(f, env, "question-next", questionPagerNextText, rightIcon, questionIconGlyph, busy || p.flow.index == len(qs)-1, func() {
				p.moveQuestion(qs, p.flow.index+1)
			})
		})
		ui.Row(f).Grow(1).MinHeight(questionFeedbackMinHeight).Justify(ui.End).Children(func() {
			if failure != "" {
				ui.Text(f, failure).FontSize(questionFeedbackFont).LineHeight(questionFeedbackLine).Color(t.Error)
			}
		})
		ui.Row(f).Items(ui.Center).Gap(questionFooterActionsGap).Children(func() {
			p.drawQuestionActionButton(f, env, t, "question-skip", questionSkipText, false, busy, func() { p.skipCurrentQuestion(env, qs) })
			answered := p.currentQuestionAnswered(qs)
			canSubmit := env.Act.Send != nil || (env.Act.AnswerQuestion != nil && len(qs) != 0 && qs[0].CallID != "")
			label := questionSubmitText
			if busy {
				label = questionSubmittingText
			} else if p.flow.index < len(qs)-1 {
				label = questionNextText
			}
			disabled := busy || !answered || (p.flow.index == len(qs)-1 && !canSubmit)
			p.drawQuestionActionButton(f, env, t, "question-submit", label, true, disabled, func() {
				if p.flow.index < len(qs)-1 {
					p.moveQuestion(qs, p.flow.index+1)
					return
				}
				p.submitQuestionFlow(env, qs)
			})
		})
	})
}

func (p *Plugin) drawQuestionIconButton(f ui.C, env *pluginapi.Env, id, label string, icon gfx.Icon, size float32, disabled bool, action func()) ui.Box {
	color := p.theme.LabelTertiary
	if env.Theme != nil {
		color = env.Theme.LabelTertiary
	}
	if disabled {
		color = p.theme.LabelDimmed
		if env.Theme != nil {
			color = env.Theme.LabelDimmed
		}
	}
	button := ui.Row(f).Width(questionIconButtonSize).Height(questionIconButtonSize).Center().
		Radius(questionIconButtonRadius).Focus(ui.Look{}.Ring(2, color)).
		Hit(env.Tree.ID(env.Feat, id), label)
	if !disabled {
		t := p.theme
		if env.Theme != nil {
			t = env.Theme
		}
		button = button.Hover(ui.Look{}.Background(t.Hover))
		if button.Clicked() {
			action()
		}
	}
	button.Children(func() {
		if len(icon.Commands) != 0 || len(icon.Layers) != 0 {
			ui.Icon(f, icon).Width(size).Height(size).Color(color)
		}
	})
	return button
}

func (p *Plugin) drawQuestionActionButton(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, id, label string, primary, disabled bool, action func()) {
	button := ui.Row(f).Height(planReviewButtonHeight).Items(ui.Center).
		PadSides(0, planReviewButtonPadX, 0, planReviewButtonPadX).Radius(t.Radii.MD).
		Focus(ui.Look{}.Ring(2, t.Info)).Hit(env.Tree.ID(env.Feat, id), label)
	foreground := t.LabelPrimary
	if primary {
		foreground = t.ButtonPrimaryFg
		button = button.Background(t.ButtonPrimaryFill)
	} else {
		button = button.Border(questionOptionBorder, t.BorderL3)
	}
	if disabled {
		button = button.Opacity(planReviewDisabled)
	} else {
		button = button.Hover(ui.Look{}.Background(t.Hover))
		if primary {
			button = button.Hover(ui.Look{}.Background(t.ButtonPrimaryHover))
		}
		if button.Clicked() {
			action()
		}
	}
	button.Children(func() {
		ui.Text(f, label).FontSize(planReviewButtonFont).LineHeight(planReviewButtonLine).Color(foreground)
	})
}
