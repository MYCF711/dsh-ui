package main

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// 这个文件测"审批作废"那条路。
//
// # 为什么值得单独一个文件
//
// 宿主对一条审批有一条**超时**分支（`serve_approval.go`）：时间到了按拒绝结算，
// 并推一帧 `approval/timeout`，然后这条 requestId 就**作废**了——
// 再 approve 过去只会拿到 404 `no_such_request`。
//
// 界面这边原来不认识这一帧（全仓 0 个引用），于是超时之后审批条会**一直挂着**：
// 用户点它得到"请求已失效"，而更糟的是他一直以为"还有东西等着我处理"。
// 这叫僵尸审批条。

const timeoutPayload = `{"requestId":"perm-1","sessionId":"s1","toolCall":{"toolCallId":"c1","title":"bash","kind":"execute"},"options":[{"optionId":"allow_once","name":"允许这一次","kind":"allow"}]}`

func approvalModel(t *testing.T) *model {
	t.Helper()
	req := &approvalReq{RequestID: "perm-1", Tool: "bash", Title: "bash"}
	return &model{
		cur:       "s1",
		approval:  req,
		approvals: map[string]*approvalReq{"s1": req},
	}
}

// TestTimeoutClearsTheStalePrompt 是主路径：超时帧把那一条清掉。
func TestTimeoutClearsTheStalePrompt(t *testing.T) {
	m := approvalModel(t)
	m.applyEvent("s1", apiclient.Event{
		Kind: "approval/timeout", Seq: 5, Data: []byte(timeoutPayload),
	})
	if m.approval != nil {
		t.Error("前台还挂着审批条——用户会去点一个已经作废的请求")
	}
	if _, ok := m.approvals["s1"]; ok {
		t.Error("按会话存的审批没清掉——切走再切回来又会出现")
	}
	if m.status == "" {
		t.Error("没有给任何状态提示：用户只会看到审批条凭空消失，不知道是超时")
	}
}

// TestLateTimeoutDoesNotClearTheNextOne 是那条**必要的克制**。
//
// 迟到的超时帧（针对上一个请求）不该把后一个还没答的审批一起抹掉。
// 没有这条，把实现写成"收到 approval/timeout 就清"也能过上面那条测试。
func TestLateTimeoutDoesNotClearTheNextOne(t *testing.T) {
	m := approvalModel(t)
	m.applyEvent("s1", apiclient.Event{
		Kind: "approval/timeout", Seq: 6,
		// 说的是**另一个** requestId（之前那条的）
		Data: []byte(`{"requestId":"perm-0","sessionId":"s1"}`),
	})
	if m.approval == nil {
		t.Fatal("清了不属于这条超时帧的审批——后一个请求会被无声地丢掉")
	}
	if m.approval.RequestID != "perm-1" {
		t.Fatalf("留下的应当是 perm-1，实际 %q", m.approval.RequestID)
	}
}

// TestTimeoutWithNoRequestIDStillClears：宿主没说是哪一条时仍然清。
//
// 留一条点不动的审批条，比多清一次更糟——前者用户会一直等。
func TestTimeoutWithNoRequestIDStillClears(t *testing.T) {
	m := approvalModel(t)
	m.applyEvent("s1", apiclient.Event{Kind: "approval/timeout", Seq: 7, Data: []byte(`{}`)})
	if m.approval != nil {
		t.Error("载荷解不出 requestId 时也该清：一条点不动的审批条比多清一次更糟")
	}
}

// TestCancelBehavesLikeTimeout：`approval/cancel` 在契约白名单里，
// 语义也是"这条请求作废了"，处理必须一样。
func TestCancelBehavesLikeTimeout(t *testing.T) {
	m := approvalModel(t)
	m.applyEvent("s1", apiclient.Event{Kind: "approval/cancel", Seq: 8, Data: []byte(timeoutPayload)})
	if m.approval != nil {
		t.Error("approval/cancel 也该把审批条清掉")
	}
}

// TestTimeoutForAnotherSessionLeavesOursAlone：别的会话的超时不该动我们的。
func TestTimeoutForAnotherSessionLeavesOursAlone(t *testing.T) {
	m := approvalModel(t)
	other := &approvalReq{RequestID: "perm-other"}
	m.approvals["s2"] = other

	// 载荷要说的是 s2 **自己那一条**（perm-other）。用 s1 的载荷去测，
	// 测到的就不是"别的会话"，而是"别人的 requestId"——那是另一个判据。
	m.applyEvent("s2", apiclient.Event{Kind: "approval/timeout", Seq: 9,
		Data: []byte(`{"requestId":"perm-other","sessionId":"s2"}`)})

	if _, ok := m.approvals["s2"]; ok {
		t.Error("s2 自己的审批没清掉")
	}
	if m.approval == nil || m.approvals["s1"] == nil {
		t.Error("清了别的会话的审批——s1 那条还在等用户回答")
	}
}
