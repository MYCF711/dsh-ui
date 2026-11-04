package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

// WP-C1 先红回归：旧实现发送失败后既清空草稿，又留下永久 Seq:-1 的本地 echo。
func TestWPC1SendFailureKeepsDraftAndDoesNotBorrowTransportIdentity(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"code":"prompt_failed","message":"injected offline"}`)
	}))
	defer ts.Close()

	m := &model{cli: apiclient.New(ts.URL), cur: "session-local"}
	m.draft.SetText("不要丢的草稿")
	m.sendText(m.draft.Text())

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		status := m.status
		m.mu.Unlock()
		if strings.Contains(status, "发送失败") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !strings.Contains(m.status, "发送失败") {
		t.Fatalf("没有等到发送失败状态：%q", m.status)
	}
	if got := m.draft.Text(); got != "不要丢的草稿" {
		t.Fatalf("发送失败后草稿 = %q，期望保留原文", got)
	}
	outgoing := m.outgoing["session-local"]
	if len(outgoing) != 1 || outgoing[0].State != pluginapi.OutgoingFailed || outgoing[0].Text != "不要丢的草稿" || outgoing[0].Key == "" {
		t.Fatalf("失败发送没有保留可重试状态：%+v", outgoing)
	}
	for _, msg := range m.msgs {
		if msg.Seq == -1 {
			t.Fatal("本地 user echo 仍借用永久 Seq:-1；应使用独立的会话内发送状态")
		}
	}
}

func TestWPC1OutgoingDoesNotReconcileAgainstHistoricalSameText(t *testing.T) {
	m := &model{
		msgs: []apiclient.Message{{Seq: 1, Role: "user", Text: "same"}},
		outgoing: map[string][]pluginapi.OutgoingMessage{
			"s": {{Key: "new", Text: "same", State: pluginapi.OutgoingSent}},
		},
	}
	m.reconcileOutgoingLocked("s", "")
	if len(m.outgoing["s"]) != 1 {
		t.Fatal("历史同文本被误当成本次 durable echo")
	}
	m.reconcileOutgoingLocked("s", "same")
	if len(m.outgoing["s"]) != 0 {
		t.Fatal("本次 durable user event 到达后本地 echo 未对账")
	}
}
