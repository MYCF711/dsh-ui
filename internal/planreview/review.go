// Package planreview 把宿主持久的 plan-review 待答问题 narrowing 成官方
// PlanReviewPanel 所需的决策卡数据。判据逐条对应
// dsh-client-ui-user-questions/src/client/contract/slots.ts:69-101。
package planreview

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/vivaui/viva/markdown"
)

// ToolName 是提交计划供审阅的模型工具。
const ToolName = "exit_plan_mode"

// Review 是一条可绘制的计划审阅。Approve/Decline 保留宿主原始选项，
// 回答必须逐字回传 label，不能在客户端改写。
type Review struct {
	ID         string
	Question   string
	Plan       string
	CallID     string
	RequestKey string
	Approve    apiclient.QuestionOption
	Decline    *apiclient.QuestionOption
}

// Of 按官方 planReviewOf 收窄一批问题；不符合专用卡形状时交回通用提问流。
func Of(questions []apiclient.QuestionItem, requestKey string) (Review, bool) {
	if len(questions) != 1 {
		return Review{}, false
	}
	q := questions[0]
	if q.Intent == nil || q.Intent.Kind != "plan-review" || q.Detail == "" || q.MultiSelect || len(q.Options) > 2 {
		return Review{}, false
	}
	var approve *apiclient.QuestionOption
	var decline *apiclient.QuestionOption
	for i := range q.Options {
		option := q.Options[i]
		if option.Label == q.Intent.Approve && approve == nil {
			approve = &option
			continue
		}
		if decline == nil {
			decline = &option
		}
	}
	if approve == nil {
		return Review{}, false
	}
	return Review{
		ID: q.ID, Question: q.Question, Plan: q.Detail,
		CallID: q.Intent.CallID, RequestKey: requestKey,
		Approve: *approve, Decline: decline,
	}, true
}

// Pending 找最近一条尚未有用户后续消息的 exit_plan_mode 待审问题。
// 之后的任意 user/message 都表示已经回答（与 dsquestions 的通用问题同口径）。
func Pending(messages []apiclient.Message) (Review, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message.Role == "user" {
			return Review{}, false
		}
		if message.Role != "tool" || message.Tool != ToolName || message.View == nil || !message.View.AwaitingAnswer {
			continue
		}
		requestKey := message.CallID
		if requestKey == "" {
			requestKey = message.MessageID
		}
		return Of(message.View.Questions, requestKey)
	}
	return Review{}, false
}

// Summary 取官方 PlanReviewPanel 的标题与首段摘要，经真实 Markdown parser
// 去掉标题记号、行内强调和链接目的地，不把 Markdown 标记当成可见摘要。
func Summary(source string) (title, description string) {
	var document markdown.Document
	document.Parse(source)
	if len(document.Blocks) == 0 {
		return "", ""
	}
	plain := func(block *markdown.Block) string {
		var b strings.Builder
		for _, span := range document.Spans(block) {
			b.WriteString(strings.ReplaceAll(span.Text, "\n", " "))
		}
		return strings.TrimSpace(b.String())
	}
	title = plain(&document.Blocks[0])
	for i := 1; i < len(document.Blocks); i++ {
		if document.Blocks[i].Kind != markdown.Paragraph {
			continue
		}
		description = plain(&document.Blocks[i])
		break
	}
	if description == title {
		description = ""
	}
	return title, description
}
