package dsquestions

import (
	"errors"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/planreview"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type recordedQuestionAnswer struct {
	callID string
	answer apiclient.QuestionAnswer
}

func planReviewQuestionFixture(t *testing.T, answer func(string, apiclient.QuestionAnswer) (bool, error)) (*Plugin, *pluginapi.Env, *gfx.Canvas, *[]pluginapi.Slot, chan struct{}) {
	t.Helper()
	tier := uikernel.NewShellTier()
	tier.Set(true)
	kernel := uikernel.New("plan-review-test")
	kernel.Root().Provide(uikernel.KeyShellTier, tier)
	p := &Plugin{}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, p); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len([]rune(value))) * size * 0.55 })
	slots := []pluginapi.Slot{}
	invalidated := make(chan struct{}, 4)
	env := &pluginapi.Env{
		Theme: kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme),
		Data: pluginapi.Snapshot{Cur: "s1", Msgs: []apiclient.Message{{
			Role: "tool", Tool: planreview.ToolName, CallID: "call-plan",
			View: &apiclient.ViewSpec{AwaitingAnswer: true, Questions: []apiclient.QuestionItem{{
				ID: "plan-review", Question: "Approve this plan and leave plan mode?", Detail: "# 审阅方案\n\n先执行第一步。",
				Options: []apiclient.QuestionOption{
					{Label: "Approve", Description: "Leave plan mode"},
					{Label: "Keep planning", Description: "Stay in plan mode"},
				},
				Intent: &apiclient.QuestionIntent{Kind: "plan-review", Approve: "Approve", CallID: "call-plan"},
			}}},
		}}},
		Act: pluginapi.Actions{
			AnswerQuestion: answer,
			Invalidate: func() {
				select {
				case invalidated <- struct{}{}:
				default:
				}
			},
		},
		Tree: tree, Feat: tree.Feature("plan-review-test"), Canvas: canvas,
		Bounds: gfx.Rect{Width: 1182, Height: 832}, MainWidth: 902,
		DrawSlot: func(f ui.C, slot pluginapi.Slot) { slots = append(slots, slot) },
	}
	return p, env, canvas, &slots, invalidated
}

func drawPlanReviewQuestion(t *testing.T, p *Plugin, env *pluginapi.Env, canvas *gfx.Canvas) {
	t.Helper()
	canvas.Reset()
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) { p.drawOfficial(f, env) })
}

func TestPlanReviewApproveUsesVerbatimOfficialAnswer(t *testing.T) {
	answers := make(chan recordedQuestionAnswer, 1)
	p, env, canvas, slots, invalidated := planReviewQuestionFixture(t, func(callID string, answer apiclient.QuestionAnswer) (bool, error) {
		answers <- recordedQuestionAnswer{callID: callID, answer: answer}
		return true, nil
	})
	drawPlanReviewQuestion(t, p, env, canvas)
	for _, want := range []string{planReviewHeader, "审阅方案", "先执行第一步。", planReviewDiscuss, planReviewApprove} {
		if !hasExactText(canvas, want) {
			t.Fatalf("计划审阅卡缺 %q：%v", want, canvasTextValues(canvas))
		}
	}
	if len(*slots) != 1 || (*slots)[0] != uikernel.OffPlanReviewActions {
		t.Fatalf("strip 未派发官方 plan-review.actions：%v", *slots)
	}
	canvas.BeginEventPass(env.Tree.ID(env.Feat, "plan-review-approve"), gfx.ClickInfo{})
	drawPlanReviewQuestion(t, p, env, canvas)
	canvas.EndEventPass()
	select {
	case got := <-answers:
		if got.callID != "call-plan" || len(got.answer.Answers) != 1 {
			t.Fatalf("回答身份不对：%+v", got)
		}
		item := got.answer.Answers[0]
		if item.ID != "plan-review" || len(item.Selected) != 1 || item.Selected[0] != "Approve" || item.Custom != "" {
			t.Fatalf("批准必须逐字回传 Approve：%+v", item)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("点击同意执行没有提交 userQuestions.answer")
	}
	select {
	case <-invalidated:
	case <-time.After(2 * time.Second):
		t.Fatal("回答结束后没有重绘")
	}
	drawPlanReviewQuestion(t, p, env, canvas)
	if len(canvasTextValues(canvas)) != 0 {
		t.Fatalf("宿主接受后审阅卡应收起：%v", canvasTextValues(canvas))
	}
}

func TestPlanReviewFailureStaysVisibleAndDiscussDoesNotAnswer(t *testing.T) {
	p, env, canvas, _, invalidated := planReviewQuestionFixture(t, func(string, apiclient.QuestionAnswer) (bool, error) {
		return false, errors.New("send failed")
	})
	canvas.BeginEventPass(env.Tree.ID(env.Feat, "plan-review-approve"), gfx.ClickInfo{})
	drawPlanReviewQuestion(t, p, env, canvas)
	canvas.EndEventPass()
	select {
	case <-invalidated:
	case <-time.After(2 * time.Second):
		t.Fatal("失败没有重绘")
	}
	drawPlanReviewQuestion(t, p, env, canvas)
	if !hasExactText(canvas, "send failed") || !hasExactText(canvas, planReviewApprove) {
		t.Fatalf("失败应保留卡片并显示宿主错误：%v", canvasTextValues(canvas))
	}

	calls := 0
	p2, env2, canvas2, _, _ := planReviewQuestionFixture(t, func(string, apiclient.QuestionAnswer) (bool, error) {
		calls++
		return true, nil
	})
	canvas2.BeginEventPass(env2.Tree.ID(env2.Feat, "plan-review-discuss"), gfx.ClickInfo{})
	drawPlanReviewQuestion(t, p2, env2, canvas2)
	canvas2.EndEventPass()
	drawPlanReviewQuestion(t, p2, env2, canvas2)
	if calls != 0 {
		t.Fatalf("要求修改只关闭卡片交回 composer，不应替用户回答；calls=%d", calls)
	}
	if len(canvasTextValues(canvas2)) != 0 {
		t.Fatalf("要求修改后应关闭审阅卡：%v", canvasTextValues(canvas2))
	}
}

func hasExactText(canvas *gfx.Canvas, want string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == want {
			return true
		}
	}
	return false
}

func canvasTextValues(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}
