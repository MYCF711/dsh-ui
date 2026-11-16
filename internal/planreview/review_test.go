package planreview

import (
	"encoding/json"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func reviewQuestions() []apiclient.QuestionItem {
	return []apiclient.QuestionItem{{
		ID: "plan-review", Header: "Plan review", Question: "Approve this plan and leave plan mode?",
		Detail: "# 迁移方案\n\n先补宿主。\n\n再补 GUI。",
		Options: []apiclient.QuestionOption{
			{Label: "Approve", Description: "Leave plan mode"},
			{Label: "Keep planning", Description: "Stay in plan mode"},
		},
		Intent: &apiclient.QuestionIntent{Kind: "plan-review", Approve: "Approve", CallID: "call-plan"},
	}}
}

func TestOfMatchesOfficialNarrowing(t *testing.T) {
	review, ok := Of(reviewQuestions(), "request-1")
	if !ok {
		t.Fatal("合法 plan-review 没有收窄成功")
	}
	if review.ID != "plan-review" || review.CallID != "call-plan" || review.RequestKey != "request-1" {
		t.Fatalf("身份不对：%+v", review)
	}
	if review.Approve.Label != "Approve" || review.Decline == nil || review.Decline.Label != "Keep planning" {
		t.Fatalf("批准/继续规划选项不对：%+v", review)
	}
	title, description := Summary(review.Plan)
	if title != "迁移方案" || description != "先补宿主。" {
		t.Fatalf("摘要 = %q / %q", title, description)
	}
}

func TestOfRejectsGenericQuestionShapes(t *testing.T) {
	cases := map[string]func([]apiclient.QuestionItem) []apiclient.QuestionItem{
		"多问题":      func(q []apiclient.QuestionItem) []apiclient.QuestionItem { return append(q, q[0]) },
		"无 intent": func(q []apiclient.QuestionItem) []apiclient.QuestionItem { q[0].Intent = nil; return q },
		"缺 detail": func(q []apiclient.QuestionItem) []apiclient.QuestionItem { q[0].Detail = ""; return q },
		"多选":       func(q []apiclient.QuestionItem) []apiclient.QuestionItem { q[0].MultiSelect = true; return q },
		"超过两个选项": func(q []apiclient.QuestionItem) []apiclient.QuestionItem {
			q[0].Options = append(q[0].Options, apiclient.QuestionOption{Label: "third"})
			return q
		},
		"批准标签不匹配": func(q []apiclient.QuestionItem) []apiclient.QuestionItem { q[0].Intent.Approve = "yes"; return q },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			questions := reviewQuestions()
			if _, ok := Of(mutate(questions), "r"); ok {
				t.Fatalf("%s 不应进入计划专用卡", name)
			}
		})
	}
}

func TestOfficialViewWireUsesMultiSelectCamelCase(t *testing.T) {
	raw := []byte(`{"kind":"card","awaiting_answer":true,"questions":[{"id":"plan-review","question":"Approve?","detail":"# P","multiSelect":true,"options":[{"label":"Approve"}],"intent":{"kind":"plan-review","approve":"Approve","callId":"c1"}}]}`)
	var view apiclient.ViewSpec
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Questions) != 1 || !view.Questions[0].MultiSelect {
		t.Fatalf("官方 multiSelect 没有解码：%+v", view.Questions)
	}
	if _, ok := Pending([]apiclient.Message{{Role: "tool", Tool: ToolName, CallID: "c1", View: &view}}); ok {
		t.Fatal("multiSelect=true 必须退回通用提问流，不能进计划专用卡")
	}
}

func TestPendingRequiresExitReviewAndNoLaterUserMessage(t *testing.T) {
	message := apiclient.Message{
		Role: "tool", Tool: ToolName, CallID: "call-plan",
		View: &apiclient.ViewSpec{AwaitingAnswer: true, Questions: reviewQuestions()},
	}
	review, ok := Pending([]apiclient.Message{message})
	if !ok || review.CallID != "call-plan" {
		t.Fatalf("待审问题未识别：%+v %v", review, ok)
	}
	for name, messages := range map[string][]apiclient.Message{
		"已经回答":           {message, {Role: "user", Text: "Approve"}},
		"不是计划工具":         {{Role: "tool", Tool: "ask_user_question", View: message.View}},
		"没有 awaiting 标记": {{Role: "tool", Tool: ToolName, View: &apiclient.ViewSpec{Questions: reviewQuestions()}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := Pending(messages); ok {
				t.Fatal("不该显示待审计划")
			}
		})
	}
}
