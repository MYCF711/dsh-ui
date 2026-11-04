package dsconversation

import (
	"math"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

var wpc3StatusLabels = []string{"准备中", "运行中", "完成", "失败", "已停止"}

func wpc3TextAt(texts []ui.Text, value string) (ui.Text, bool) {
	for _, text := range texts {
		if text.Value == value {
			return text, true
		}
	}
	return ui.Text{}, false
}

func requireWPC3HeaderStatus(t *testing.T, texts []ui.Text, tool, status string) {
	t.Helper()
	title, ok := wpc3TextAt(texts, tool)
	if !ok {
		t.Fatalf("画面缺少工具标题 %q：%q", tool, sessionTextValues(texts))
	}
	label, ok := wpc3TextAt(texts, status)
	if !ok {
		t.Fatalf("工具 %q 缺少有真值的状态 %q：%q", tool, status, sessionTextValues(texts))
	}
	if math.Abs(float64(title.Bounds.Y-label.Bounds.Y)) > 1 {
		t.Fatalf("状态 %q 不在工具标题行：titleY=%v statusY=%v", status, title.Bounds.Y, label.Bounds.Y)
	}
}

func TestWPC3ToolStatusOnlyComesFromTruth(t *testing.T) {
	stoppedKey := pluginapi.MessageKey("s-status", apiclient.Message{Role: "tool", Tool: "bash", CallID: "call-stopped"}, 0)
	tests := []struct {
		name string
		msg  apiclient.Message
		data func(*pluginapi.Snapshot)
		want string
	}{
		{name: "显式 preparing", msg: apiclient.Message{Role: "tool", Tool: "bash", Args: "sleep 1", View: &apiclient.ViewSpec{Badge: "preparing"}}, want: "准备中"},
		{name: "活动 Turn 无结果是 running", msg: apiclient.Message{Role: "tool", Tool: "bash", TurnID: "turn-live", Args: "go test ./..."}, data: func(data *pluginapi.Snapshot) {
			data.Busy, data.ActiveTurnID = true, "turn-live"
		}, want: "运行中"},
		{name: "显式结果是 ok", msg: apiclient.Message{Role: "tool", Tool: "bash", Args: "go test ./...", Summary: "exit 0 · 12 passed"}, want: "完成"},
		{name: "View.Badge ok", msg: apiclient.Message{Role: "tool", Tool: "read", Args: "a.go", View: &apiclient.ViewSpec{Badge: "ok"}}, want: "完成"},
		{name: "View.Badge error", msg: apiclient.Message{Role: "tool", Tool: "read", Args: "a.go", Summary: "permission denied", View: &apiclient.ViewSpec{Badge: "error"}}, want: "失败"},
		{name: "非零退出码是 error", msg: apiclient.Message{Role: "tool", Tool: "bash", Args: "go test ./...", Summary: "[exit 1] 1 failed"}, want: "失败"},
		{name: "原始 stopped 真值", msg: apiclient.Message{Role: "tool", Tool: "bash", CallID: "call-stopped", Args: "sleep 30"}, data: func(data *pluginapi.Snapshot) {
			data.MessageMeta = map[string]pluginapi.MessageMetadata{stoppedKey: {Stopped: true}}
		}, want: "已停止"},
		{name: "stopped interrupted Turn 真值", msg: apiclient.Message{Role: "tool", Tool: "bash", TurnID: "turn-stopped", Args: "sleep 30"}, data: func(data *pluginapi.Snapshot) {
			data.TurnEvents = []pluginapi.TurnEvent{{Key: "stopped-real", Kind: "stopped", TurnID: "turn-stopped"}}
		}, want: "已停止"},
		{name: "未知保持 neutral", msg: apiclient.Message{Role: "tool", Tool: "bash", Args: "sleep 1"}},
		{name: "Busy 但不是这个活动 Turn", msg: apiclient.Message{Role: "tool", Tool: "bash", TurnID: "turn-old", Args: "sleep 1"}, data: func(data *pluginapi.Snapshot) {
			data.Busy, data.ActiveTurnID = true, "turn-other"
		}},
		{name: "活动 Turn 已有结果不是 running", msg: apiclient.Message{Role: "tool", Tool: "bash", TurnID: "turn-live", Args: "go test ./...", Summary: "exit 0 · done"}, data: func(data *pluginapi.Snapshot) {
			data.Busy, data.ActiveTurnID = true, "turn-live"
		}, want: "完成"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := pluginapi.Snapshot{Cur: "s-status", Msgs: []apiclient.Message{tc.msg}}
			if tc.data != nil {
				tc.data(&data)
			}
			canvas := drawStreamingMarkdownTest(t, newStreamingMarkdownTestPlugin(), data)
			texts := canvas.Texts()
			if tc.want != "" {
				requireWPC3HeaderStatus(t, texts, tc.msg.Tool, tc.want)
				return
			}
			for _, label := range wpc3StatusLabels {
				if _, found := wpc3TextAt(texts, label); found {
					t.Fatalf("没有状态真值却显示 %q：%q", label, sessionTextValues(texts))
				}
			}
		})
	}
}

func TestWPC3InterleavedIdenticalCallsKeepIdentityResultAndStatus(t *testing.T) {
	args := `{"command":"printf same"}`
	msgs := []apiclient.Message{
		{Role: "tool", Tool: "bash", TurnID: "turn-interleaved", CallID: "call-A", Args: args, Summary: "exit 0 · RESULT-A"},
		{Role: "tool", Tool: "bash", TurnID: "turn-interleaved", CallID: "call-B", Args: args, Summary: "[exit 1] RESULT-B"},
	}
	data := pluginapi.Snapshot{Cur: "s-interleaved", Msgs: msgs, Busy: true, ActiveTurnID: "turn-interleaved"}
	p := newStreamingMarkdownTestPlugin()
	canvas := drawStreamingMarkdownTest(t, p, data)
	texts := canvas.Texts()

	keyA := messageRenderKey(&pluginapi.Env{Data: data}, msgs[0], 0)
	keyB := messageRenderKey(&pluginapi.Env{Data: data}, msgs[1], 1)
	if keyA == keyB || p.toolRenderID(keyA) == p.toolRenderID(keyB) {
		t.Fatalf("同 Tool+Args 的两次 CallID 调用共享身份：%q / %q", keyA, keyB)
	}
	resultA, okA := wpc3TextAt(texts, "exit 0 · RESULT-A")
	resultB, okB := wpc3TextAt(texts, "[exit 1] RESULT-B")
	if !okA || !okB || resultA.Bounds.Y >= resultB.Bounds.Y {
		t.Fatalf("交错结果被最近调用覆盖或顺序错位：A=%+v B=%+v texts=%q", resultA, resultB, sessionTextValues(texts))
	}
	okStatus, ok := wpc3TextAt(texts, "完成")
	errorStatus, failed := wpc3TextAt(texts, "失败")
	if !ok || !failed || math.Abs(float64(okStatus.Bounds.Y-resultA.Bounds.Y)) > 1 || math.Abs(float64(errorStatus.Bounds.Y-resultB.Bounds.Y)) > 1 {
		t.Fatalf("两个同参调用的状态串用：ok=%+v error=%+v A=%+v B=%+v", okStatus, errorStatus, resultA, resultB)
	}
	if strings.Count(sessionTextValues(texts), "RESULT-A") != 1 || strings.Count(sessionTextValues(texts), "RESULT-B") != 1 {
		t.Fatalf("同参调用结果被复制或丢失：%q", sessionTextValues(texts))
	}
}
