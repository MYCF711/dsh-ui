package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

func TestStartSessionCreatesCordisAndAppendsDraftWithoutSending(t *testing.T) {
	var mu sync.Mutex
	var createMode string
	var createAgentPreset string
	var createHadTitle, createHadCwd bool
	var promptCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/config.get" {
			// 防止 refreshList 在测试里启动周期 jobs ticker；失败语义与产品相同。
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/session.create":
			mu.Lock()
			createMode, _ = body["mode"].(string)
			createAgentPreset, _ = body["agentPreset"].(string)
			_, createHadTitle = body["title"]
			_, createHadCwd = body["cwd"]
			mu.Unlock()
			_, _ = w.Write([]byte(`{"result":{"sessionId":"creator-session"}}`))
		case "/api/session.prompt", "/api/session.promptParts":
			mu.Lock()
			promptCalls++
			mu.Unlock()
			_, _ = w.Write([]byte(`{"result":{"turnId":"must-not-happen"}}`))
		case "/api/session.load":
			_, _ = w.Write([]byte(`{"result":{"id":"creator-session","messages":[],"total":0}}`))
		default:
			_, _ = w.Write([]byte(`{"result":{}}`))
		}
	}))
	defer srv.Close()

	m := &model{
		cli: apiclient.New(srv.URL), mux: apiclient.NewMux(srv.URL),
		cache: map[string]*cachedSession{}, seqs: map[string]int{}, epochs: map[string]int64{},
	}
	m.draft.SetText("保留我的草稿")
	t.Cleanup(func() {
		m.mu.Lock()
		for _, sub := range m.subs {
			if sub.cancel != nil {
				sub.cancel()
			}
		}
		m.mu.Unlock()
	})
	m.startSession(pluginapi.StartSessionOptions{
		AgentPreset: "cordis", Prompt: "创建一个带左侧栏入口的 DSH 插件。", WhenDraftExists: pluginapi.DraftAppend,
	})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		cur, pending := m.cur, m.pendingDraft
		m.mu.Unlock()
		if cur == "creator-session" && pending != nil {
			m.applyPendingDraft()
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	m.mu.Lock()
	cur, pending := m.cur, m.pendingDraft
	m.mu.Unlock()
	if cur != "creator-session" || pending != nil {
		t.Fatalf("Creator 会话/草稿没有落地：cur=%q pending=%#v", cur, pending)
	}
	if got, want := m.draft.Text(), "保留我的草稿\n\n创建一个带左侧栏入口的 DSH 插件。"; got != want {
		t.Fatalf("append 草稿 = %q，want %q", got, want)
	}
	mu.Lock()
	defer mu.Unlock()
	// 官方字段（types.ts:268-277）：Creator 点名的 preset 必须落在 agentPreset 上；
	// mode 是旧名字，同一份值也照发（老宿主只解它，见 apiclient/session_create.go）。
	if createAgentPreset != "cordis" {
		t.Fatalf("session.create.agentPreset = %q，want cordis", createAgentPreset)
	}
	if createMode != "cordis" {
		t.Fatalf("session.create.mode = %q，want cordis", createMode)
	}
	// 官方没有 title；cwd 只有"没有工作区时会话自己拥有目录"时才发，Creator 这条不发。
	if createHadTitle {
		t.Fatal("session.create 不该再发 title（官方 SessionCreateRequest 没有这个字段）")
	}
	if createHadCwd {
		t.Fatal("Creator 这条不该发 cwd（官方只有 workspaceId/cwd 二选一的目录语义）")
	}
	if promptCalls != 0 {
		t.Fatalf("Creator 只初始化草稿，却自动发了 %d 次 prompt", promptCalls)
	}
}

func TestStartSessionCreateFailureKeepsExistingDraft(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":"injected","message":"create failed"}`))
	}))
	defer srv.Close()
	m := &model{cli: apiclient.New(srv.URL)}
	m.draft.SetText("不能丢")
	m.startSession(pluginapi.StartSessionOptions{AgentPreset: "cordis", Prompt: "新提示", WhenDraftExists: pluginapi.DraftReplace})

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		status := m.status
		m.mu.Unlock()
		if strings.Contains(status, "新建失败") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	m.mu.Lock()
	status, pending := m.status, m.pendingDraft
	m.mu.Unlock()
	if !strings.Contains(status, "新建失败") || pending != nil {
		t.Fatalf("失败没有可见反馈/错误排了草稿：status=%q pending=%#v", status, pending)
	}
	if got := m.draft.Text(); got != "不能丢" {
		t.Fatalf("创建失败改了用户草稿：%q", got)
	}
}

func TestAppendDraftParagraphUsesNewParagraph(t *testing.T) {
	for _, tc := range []struct{ old, prompt, want string }{
		{"", "new", "new"},
		{"old", "new", "old\n\nnew"},
		{"old\n\n", "new", "old\n\nnew"},
		{"old", "", "old"},
	} {
		if got := appendDraftParagraph(tc.old, tc.prompt); got != tc.want {
			t.Fatalf("appendDraftParagraph(%q,%q)=%q，want %q", tc.old, tc.prompt, got, tc.want)
		}
	}
}
