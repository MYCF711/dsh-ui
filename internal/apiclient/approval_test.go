// approval_test.go —— 审批帧解析与回传的自测。
//
// 为什么必须用**构造帧**：真宿主当前 policy=ask 时是在子进程的**终端 stdin** 上
// 问 `允许？[y/N]`，占位应答直接算拒绝，`approval/request` 帧还没有经 WS 推出来
// （宿主 §12 H13：handler 已有，执行链未闭环）。所以客户端这一侧只能对着
// `docs/h13-approval.md` 的帧形状做契约测试，等宿主推帧那天直接接上。
package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 宿主 docs/h13-approval.md 第 22 行起的帧，逐字抄下来当夹具。
const sampleApprovalFrame = `{
  "requestId": "perm-8f3c",
  "sessionId": "s-1",
  "toolCall": {
    "toolCallId": "perm-8f3c",
    "title": "bash: go test ./...",
    "kind": "execute",
    "status": "pending",
    "rawInput": {"command": "go test ./..."}
  },
  "options": [
    {"optionId": "allow_once", "name": "允许这一次", "kind": "allow_once"},
    {"optionId": "allow_always", "name": "这个会话里以后都允许", "kind": "allow_always"},
    {"optionId": "reject_once", "name": "拒绝", "kind": "reject_once"}
  ]
}`

func TestParseApprovalRequest(t *testing.T) {
	req, err := ParseApprovalRequest(json.RawMessage(sampleApprovalFrame))
	if err != nil {
		t.Fatalf("合法帧不该报错：%v", err)
	}
	if req.RequestID != "perm-8f3c" {
		t.Fatalf("requestId 要原样保留，得到 %q", req.RequestID)
	}
	if got := req.Tool(); got != "execute" {
		t.Fatalf("Tool 应取 toolCall.kind，得到 %q", got)
	}
	if got := req.Command(); got != "go test ./..." {
		t.Fatalf("Command 应取 rawInput.command，得到 %q", got)
	}
	if got := req.OptionID("allow_once"); got != "allow_once" {
		t.Fatalf("OptionID(allow_once) 得到 %q", got)
	}
	if got := req.OptionID("reject_once"); got != "reject_once" {
		t.Fatalf("OptionID(reject_once) 得到 %q", got)
	}
	if got := req.OptionID("no_such_kind"); got != "" {
		t.Fatalf("找不到的 kind 必须返回空串（不许猜），得到 %q", got)
	}
}

func TestParseApprovalRequestRejectsBadFrames(t *testing.T) {
	cases := map[string]string{
		"缺 requestId": `{"sessionId":"s-1","options":[]}`,
		"不是对象":        `[1,2,3]`,
		"不是 JSON":     `not-json`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseApprovalRequest(json.RawMessage(raw)); err == nil {
				t.Fatalf("坏帧 %q 应当报错（requestId 丢了就无法原样回传）", raw)
			}
		})
	}
}

func TestApprovalCommandFallsBackToRawInput(t *testing.T) {
	// 非 bash 类工具：rawInput 里没有 command，就把 rawInput 原文给人看。
	req := &ApprovalRequest{}
	req.ToolCall.RawInput = json.RawMessage(`{"path":"a/b.go","line":3}`)
	if got := req.Command(); !strings.Contains(got, `"path":"a/b.go"`) {
		t.Fatalf("没有 command 时应回退到 rawInput 原文，得到 %q", got)
	}
	// 连 rawInput 都没有：退回 title。
	req2 := &ApprovalRequest{}
	req2.ToolCall.Title = "edit: a/b.go"
	if got := req2.Command(); got != "edit: a/b.go" {
		t.Fatalf("rawInput 也缺时应回退到 title，得到 %q", got)
	}
}

func TestApproveSendsRequestIDVerbatim(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"result":{"accepted":true,"optionId":"allow_once"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.Approve(context.Background(), "perm-8f3c", "allow_once"); err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotBody["requestId"] != "perm-8f3c" || gotBody["optionId"] != "allow_once" {
		t.Fatalf("回传必须原样：拿到 %v", gotBody)
	}
}

func TestApproveRejectsNotAccepted(t *testing.T) {
	// 宿主回 200 但 accepted=false：不能当成功，否则就是"气泡消失冒充完成"。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"accepted":false,"optionId":"allow_once"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.Approve(context.Background(), "perm-8f3c", "allow_once"); err == nil {
		t.Fatal("accepted=false 必须报错")
	}
}

func TestApproveRequiresBothIDs(t *testing.T) {
	c := New("http://127.0.0.1:1") // 不会真的发请求：参数校验先失败
	if err := c.Approve(context.Background(), "", "allow_once"); err == nil {
		t.Fatal("空 requestId 必须报错（不能编一个发过去）")
	}
	if err := c.Approve(context.Background(), "perm-1", ""); err == nil {
		t.Fatal("空 optionId 必须报错（不能退回写死的值）")
	}
}

func TestApproveSurfacesHostError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"no_such_request","message":"没有这个请求","hint":"可能已超时","retryable":false}}`))
	}))
	defer srv.Close()

	err := New(srv.URL).Approve(context.Background(), "perm-gone", "allow_once")
	if err == nil {
		t.Fatal("宿主 404 必须冒泡（气泡不能悄悄收掉）")
	}
	if !strings.Contains(err.Error(), "no_such_request") {
		t.Fatalf("错误要带宿主 code，得到 %v", err)
	}
}
