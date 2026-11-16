// agent_actions.go —— dsplan / dsquestions 消费的官方命令与提问动作。
package apiclient

import "context"

// CommandOutcome 是 commands.execute 的归一化命令结果。
// kind=error 是宿主已经执行并结算的命令 outcome，不是 Transport 错误。
type CommandOutcome struct {
	Kind           string `json:"kind"`
	Text           string `json:"text,omitempty"`
	SourceEventSeq *int64 `json:"sourceEventSeq,omitempty"`
}

// CommandExecution 是 commands.execute 的非 null 结果。
type CommandExecution struct {
	CommandID string         `json:"commandId"`
	Result    CommandOutcome `json:"result"`
}

// CommandsExecute 在当前会话执行一条官方命令。
//
// 官方 undefined / null 表示命令名未解析，返回 (nil, nil)；调用方必须把它
// 显示成 unknown command，不能伪造成执行成功。HTTP/协议错误原样返回。
// GUI 当前不随命令提交附件，但 submittedAttachments 必须显式是 []，不是 null。
func (c *Client) CommandsExecute(ctx context.Context, sessionID, line string) (*CommandExecution, error) {
	params := struct {
		AgentID              string `json:"agentId"`
		Line                 string `json:"line"`
		SubmittedAttachments []any  `json:"submittedAttachments"`
	}{AgentID: sessionID, Line: line, SubmittedAttachments: []any{}}
	var out *CommandExecution
	if err := c.Call(ctx, "commands.execute", params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// QuestionAnswerItem 是一个问题的结构化答案，与 OfficialQuestionAnswerItem 同形。
type QuestionAnswerItem struct {
	ID       string   `json:"id"`
	Selected []string `json:"selected"`
	Custom   string   `json:"custom,omitempty"`
}

// QuestionAnswer 是 userQuestions.answer 的 answer 入参。
type QuestionAnswer struct {
	Answers []QuestionAnswerItem `json:"answers"`
}

// UserQuestionsAnswer 提交一批答案；布尔值是宿主返回的 still-continued 真值。
// false 是合法业务结果（问题已经关闭），不是 Transport 错误。
func (c *Client) UserQuestionsAnswer(ctx context.Context, sessionID, callID string, answer QuestionAnswer) (bool, error) {
	params := struct {
		AgentID string         `json:"agentId"`
		CallID  string         `json:"callId"`
		Answer  QuestionAnswer `json:"answer"`
	}{AgentID: sessionID, CallID: callID, Answer: answer}
	var out bool
	if err := c.Call(ctx, "userQuestions.answer", params, &out); err != nil {
		return false, err
	}
	return out, nil
}
