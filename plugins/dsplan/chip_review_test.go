package dsplan

import (
	"errors"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func TestPlanChipUsesProjectionAndExecutesPlanOff(t *testing.T) {
	p, env, act, canvas := fixture(t, nil, 360)
	act.commands = make(chan string, 1)
	act.invalidated = make(chan struct{}, 1)
	env.Data.Plan = &apiclient.PlanProjection{Active: true}
	id := env.Tree.ID(env.Feat, "plan-mode-off")
	click(t, p, env, canvas, 520, 80, id, func() { drawChipOnce(t, p, env, canvas) })
	select {
	case line := <-act.commands:
		if line != "/plan off" {
			t.Fatalf("执行命令 = %q，want /plan off", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("点击计划 chip 没有执行 /plan off")
	}
	select {
	case <-act.invalidated:
	case <-time.After(2 * time.Second):
		t.Fatal("命令结束后没有请求重绘")
	}
}

func TestPlanChipVisibilityAndFailure(t *testing.T) {
	for name, projection := range map[string]*apiclient.PlanProjection{
		"未知":   nil,
		"已关闭":  {Active: false},
		"进入排队": {Active: false, Pending: true},
		"退出排队": {Active: true, Pending: true},
	} {
		t.Run(name, func(t *testing.T) {
			p, env, _, canvas := fixture(t, nil, 360)
			env.Data.Plan = projection
			drawChipOnce(t, p, env, canvas)
			want := name == "进入排队"
			if got := hasText(canvas, zhChipLabel); got != want {
				t.Fatalf("chip 可见 = %v，want %v（texts=%v）", got, want, textValues(canvas))
			}
		})
	}
	p, env, act, canvas := fixture(t, nil, 360)
	act.commands = make(chan string, 1)
	act.invalidated = make(chan struct{}, 1)
	act.commandErr = errors.New("command failed (internal)")
	env.Data.Plan = &apiclient.PlanProjection{Active: true}
	click(t, p, env, canvas, 520, 80, env.Tree.ID(env.Feat, "plan-mode-off"), func() { drawChipOnce(t, p, env, canvas) })
	select {
	case <-act.invalidated:
	case <-time.After(2 * time.Second):
		t.Fatal("失败没有结束 leaving 状态")
	}
	drawChipOnce(t, p, env, canvas)
	if !hasText(canvas, zhChipExitFailed) {
		t.Fatalf("失败应显示官方错误行：%v", textValues(canvas))
	}
}

func TestPlanReviewOpensLoggedPlanAutomaticallyOnce(t *testing.T) {
	p, env, act, canvas := fixture(t, []apiclient.Frame{
		callFrame(t, 1, 1, "call-plan", "exit_plan_mode", planArgs(t, "# 审阅方案\n\n完整正文。")),
	}, 360)
	env.Data.Msgs = []apiclient.Message{reviewMessage("call-plan")}
	drawReviewActionOnce(t, p, env, canvas)
	if got, want := p.openAddress, planAddress("session-1", "call-plan"); got != want {
		t.Fatalf("自动打开地址 = %q，want %q", got, want)
	}
	if act.toggled != 1 || len(act.tabs) != 1 {
		t.Fatalf("首次应自动展开右栏一次：toggle=%d tabs=%v", act.toggled, act.tabs)
	}
	drawReviewActionOnce(t, p, env, canvas)
	if act.toggled != 1 || len(act.tabs) != 1 {
		t.Fatalf("同一审阅不应反复自动打开：toggle=%d tabs=%v", act.toggled, act.tabs)
	}
	click(t, p, env, canvas, 520, 80, env.Tree.ID(env.Feat, "plan-review-open-call-plan"), func() {
		drawReviewActionOnce(t, p, env, canvas)
	})
	if act.toggled != 2 || len(act.tabs) != 2 {
		t.Fatalf("手动查看全文应始终可用：toggle=%d tabs=%v", act.toggled, act.tabs)
	}
	drawPreviewOnce(t, p, env, canvas, 360, 600)
	if body := joinedTexts(canvas); !contains(body, "完整正文") {
		t.Fatalf("右栏没有计划全文：%q", body)
	}
}

func TestTemporaryPlanReviewPreviewAndExpiry(t *testing.T) {
	p, env, _, canvas := fixture(t, nil, 360)
	message := reviewMessage("")
	message.CallID = "request-temp"
	env.Data.Msgs = []apiclient.Message{message}
	drawReviewActionOnce(t, p, env, canvas)
	wantAddress := reviewPreviewAddress("session-1", "test-window:request-temp")
	if p.openAddress != wantAddress {
		t.Fatalf("临时地址 = %q，want %q", p.openAddress, wantAddress)
	}
	if session, request, ok := parseReviewPreviewAddress(wantAddress); !ok || session != "session-1" || request != "test-window:request-temp" {
		t.Fatalf("临时地址往返失败：%q %q %v", session, request, ok)
	}
	drawPreviewOnce(t, p, env, canvas, 360, 600)
	if body := joinedTexts(canvas); !contains(body, "完整正文") {
		t.Fatalf("临时全文没有画出来：%q", body)
	}
	delete(p.temporaryReviews, wantAddress)
	drawPreviewOnce(t, p, env, canvas, 360, 600)
	if !hasText(canvas, zhExpired) {
		t.Fatalf("临时参数丢失后应显示 expired：%v", textValues(canvas))
	}
}

func reviewMessage(callID string) apiclient.Message {
	return apiclient.Message{
		Role: "tool", Tool: "exit_plan_mode", CallID: callID,
		View: &apiclient.ViewSpec{
			AwaitingAnswer: true,
			Questions: []apiclient.QuestionItem{{
				ID: "plan-review", Question: "Approve this plan and leave plan mode?", Detail: "# 审阅方案\n\n完整正文。",
				Options: []apiclient.QuestionOption{{Label: "Approve"}, {Label: "Keep planning"}},
				Intent:  &apiclient.QuestionIntent{Kind: "plan-review", Approve: "Approve", CallID: callID},
			}},
		},
	}
}
