// session_delete_test.go —— 删会话必须**停掉那条流**并丢掉它的壳侧状态。
//
// 为什么值得单独测：订阅是 per-session 一直活着的（切走不退订）。会话删掉之后
// 如果还留着 `m.subs[id]`，`runSubscription` 会拿一个已经不存在的 topic 无限重连
// （1s→…→30s 退避），每次尝试都改写全局连接指示器——**当前会话明明连着，指示器
// 却隔一会儿翻成「重新连接中」**。而且这条 goroutine 与 socket 再没人收得回来：
// `m.subs` 里没有第二个删它的入口。
//
// 失败面同样要钉：宿主回错时会话还在，订阅**不能**停（停了就是"删失败，流也没了"）。
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// deleteOKHost 造一个 session.delete 成功的假宿主。
func deleteOKHost(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/session.delete") {
			_, _ = fmt.Fprint(w, `{"result":{"deleted":true}}`)
			return
		}
		http.Error(w, `{"error":{"code":"internal","message":"test host"}}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// deleteFailHost 造一个 session.delete 失败的假宿主。
func deleteFailHost(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":{"code":"not_found","message":"no such session"}}`, http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// subbedModel 造一个"那条会话正在订"的 model，并返回一个可观测的 cancel。
func subbedModel(t *testing.T, base string, id string) (*model, *cancelFlag) {
	t.Helper()
	flag := &cancelFlag{}
	m := &model{
		cli:          apiclient.New(base),
		cur:          id,
		subs:         map[string]*muxSub{id: {cancel: flag.cancel, lastSeq: 42}},
		seqs:         map[string]int{id: 42},
		epochs:       map[string]int64{id: 3},
		frames:       map[string]*frameJournal{id: {}},
		streamStates: map[string]*sessionStreamState{id: {}},
		cache:        map[string]*cachedSession{id: {}},
		approvals:    map[string]*approvalReq{id: {RequestID: "req-1"}},
		lastSeq:      42,
		approval:     &approvalReq{RequestID: "req-1"},
	}
	return m, flag
}

// cancelFlag 记录"订阅的 cancel 被调过没有"。
type cancelFlag struct {
	mu     sync.Mutex
	called bool
}

func (f *cancelFlag) cancel() {
	f.mu.Lock()
	f.called = true
	f.mu.Unlock()
}

func (f *cancelFlag) Cancelled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.called
}

func TestDeleteSessionStopsSubscriptionAndDropsState(t *testing.T) {
	const id = "s-del"
	srv := deleteOKHost(t)
	m, flag := subbedModel(t, srv.URL, id)

	m.deleteSession(id)

	waitUntil(t, "subs[s-del] 被清掉", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.subs[id] == nil
	})
	if !flag.Cancelled() {
		t.Fatal("会话删掉之后订阅的 cancel 没被调：runSubscription 会拿一个不存在的 topic 无限重连")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cur != "" {
		t.Fatalf("cur = %q，想要空（删的是当前会话）", m.cur)
	}
	if m.lastSeq != 0 {
		t.Fatalf("前台水位 lastSeq = %d，想要 0（源已清，副本不能留着）", m.lastSeq)
	}
	if m.approval != nil {
		t.Fatalf("前台审批还挂着 %+v：删掉的会话不该再画审批条", m.approval)
	}
	if _, ok := m.seqs[id]; ok {
		t.Fatal("seqs[s-del] 没删")
	}
	if _, ok := m.epochs[id]; ok {
		t.Fatal("epochs[s-del] 没删")
	}
	if _, ok := m.frames[id]; ok {
		t.Fatal("frames[s-del] 没删")
	}
	if _, ok := m.streamStates[id]; ok {
		t.Fatal("streamStates[s-del] 没删")
	}
	if _, ok := m.cache[id]; ok {
		t.Fatal("cache[s-del] 没删")
	}
	if _, ok := m.approvals[id]; ok {
		t.Fatal("approvals[s-del] 没删")
	}
}

func TestDeleteSessionFailureKeepsSubscription(t *testing.T) {
	const id = "s-keep"
	srv := deleteFailHost(t)
	m, flag := subbedModel(t, srv.URL, id)

	m.deleteSession(id)

	// 失败时唯一可观测的终态是状态栏那句原文（宿主原因不折叠）。
	waitUntil(t, "状态栏报出删除失败", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return strings.Contains(m.status, "删除会话失败")
	})
	if flag.Cancelled() {
		t.Fatal("删除失败却把订阅停了：宿主那边会话还在，界面的流不该断")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.subs[id] == nil || m.cur != id {
		t.Fatalf("失败路径动了本地状态：subs=%v cur=%q", m.subs[id], m.cur)
	}
	if m.seqs[id] != 42 {
		t.Fatalf("失败路径把水位清了：%d", m.seqs[id])
	}
}

// 编译期断言：cancelFlag 的 cancel 形状就是订阅要的 context.CancelFunc。
var _ context.CancelFunc = (&cancelFlag{}).cancel
