// approval.go —— 审批帧（WS `approval/request`）的解析与回传（H13 客户端侧）。
//
// 线上形状（宿主 docs/h13-approval.md 第 22 行起，逐字）：
//
//	{"seq":101,"type":"approval/request","time":1793683941007,
//	 "data":{"requestId":"perm-8f3c…","sessionId":"…",
//	   "toolCall":{"toolCallId":"…","title":"bash: go test ./...","kind":"execute",
//	               "status":"pending","rawInput":{"command":"go test ./..."}},
//	   "options":[{"optionId":"allow_once","name":"允许这一次","kind":"allow_once"},
//	              {"optionId":"allow_always",…},{"optionId":"reject_once","name":"拒绝"}]}}
//
// 回传：POST /api/session.approve {"requestId":"…","optionId":"allow_once"}
//
// 两条纪律（都写进宿主 §11.7）：
//  1. **requestId 必须原样回传**，界面不许自己造 id；
//  2. **本地气泡消失不等于已批准** —— 只有 Approve 真的 200 了才算，
//     失败要把错误显示给用户，不能"点了就当成功"。
package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
)

// ApprovalOption 是宿主给出的一个可选项。**不要自己编**：
// 界面显示的名称与 optionId 一律原样用宿主的，我们只按 Kind 归类。
type ApprovalOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

// ApprovalRequest 是 approval/request 帧的 data。
type ApprovalRequest struct {
	RequestID string `json:"requestId"`
	SessionID string `json:"sessionId"`
	ToolCall  struct {
		ToolCallID string          `json:"toolCallId"`
		Title      string          `json:"title"`
		Kind       string          `json:"kind"`
		Status     string          `json:"status"`
		RawInput   json.RawMessage `json:"rawInput"`
	} `json:"toolCall"`
	Options []ApprovalOption `json:"options"`
}

// ParseApprovalRequest 解一帧 approval/request 的 data。
// 缺 requestId 视为坏帧（回传时会对不上），直接报错而不是猜。
func ParseApprovalRequest(data json.RawMessage) (*ApprovalRequest, error) {
	var a ApprovalRequest
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("approval/request 帧不是合法 JSON：%w", err)
	}
	if a.RequestID == "" {
		return nil, fmt.Errorf("approval/request 帧缺 requestId（回传会对不上，拒收）")
	}
	return &a, nil
}

// Tool 是工具名。优先取 rawInput 里能不能看出命令，其次退回 title。
//
// 为什么不解析 toolCallId：那不是给界面的语义字段（宿主用它配对），
// 拿它当工具名是自造。
func (a *ApprovalRequest) Tool() string {
	if a.ToolCall.Kind != "" {
		return a.ToolCall.Kind // execute / edit / write …（宿主给的分类）
	}
	return a.ToolCall.Title
}

// Command 是要给人看的那一行。
//
// 顺序：rawInput.command（bash 类）→ rawInput 原文（其它工具的入参）→ title。
// **完整不截断**：用户要看的就是这一行；截断交给插件按宽度做。
func (a *ApprovalRequest) Command() string {
	var p struct {
		Command string `json:"command"`
	}
	if len(a.ToolCall.RawInput) > 0 {
		if json.Unmarshal(a.ToolCall.RawInput, &p) == nil && p.Command != "" {
			return p.Command
		}
		return string(a.ToolCall.RawInput)
	}
	return a.ToolCall.Title
}

// OptionID 按 host 的 kind 找一个可选项（allow_once / allow_always / reject_once）。
//
// 找不到返回空串——调用方必须据此报错，**不许**退回一个写死的 optionId。
func (a *ApprovalRequest) OptionID(kind string) string {
	for _, o := range a.Options {
		if o.Kind == kind {
			return o.OptionID
		}
	}
	return ""
}

// Approve 调 session.approve，把用户的选择原样回传。
//
// 返回 nil 表示宿主真的收下了（200 + accepted）；错误必须冒泡到界面，
// 因为"点了按钮但宿主没收到"是最坏的情况：用户以为批了，工具其实没跑。
func (c *Client) Approve(ctx context.Context, requestID, optionID string) error {
	if requestID == "" || optionID == "" {
		return fmt.Errorf("approve 需要 requestId 与 optionId（收到 %q / %q）", requestID, optionID)
	}
	var out struct {
		Accepted bool   `json:"accepted"`
		OptionID string `json:"optionId"`
	}
	err := c.Call(ctx, "session.approve", map[string]string{
		"requestId": requestID,
		"optionId":  optionID,
	}, &out)
	if err != nil {
		return err
	}
	if !out.Accepted {
		// 宿主回了 200 但没 accepted：不能当成功，否则就是"气泡消失冒充完成"。
		return fmt.Errorf("宿主没有接受这次审批（accepted=false）")
	}
	return nil
}
