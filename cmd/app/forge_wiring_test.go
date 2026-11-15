// forge_wiring_test.go —— #64a 壳侧数据面的钉子（cmd/app/forge_wiring.go）。
//
// 对着**真 HTTP**（httptest + 真 apiclient）验五件事，全是最容易写错、也最难在真机
// 上看出来的：
//
//	checkout    branches 成功 → ready（分支名）；`not_a_repository` → none（不是 failed）
//	本地摘要    branch 范围的 localDiff 就是 `changes.local` 卡片的数字与 base
//	范围切换    切到 staged 只发 kind，且**回包晚到不影响新范围**（会话/范围双判）
//	工作树动作  request 的 `from` 与 cancel 的路径真的发出去了；失败要回滚勾选态
//	换会话     旧会话的回包**丢掉**（不把上一个会话的 diff 画进新会话）
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// forgeHost 是一个只认 forgePanel.* 的假宿主，记下每次请求的路径与平铺参数。
type forgeHost struct {
	mu       sync.Mutex
	calls    []string
	lastBody map[string]map[string]any
	// diffs 是按到达顺序记下的每次 localDiff 参数（晚到的旧会话回包也会记，
	// 所以断言"发过 s-2"要用它，而不是"最后一条"）。
	diffs []map[string]any

	// failRequest 让 requestWorktree 回四字段错误。
	failRequest bool
	// notRepo 让 branches 回 400 not_a_repository。
	notRepo bool
}

func (h *forgeHost) body(path string) map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastBody[path]
}

// sawDiffFor 报告有没有为某个会话发过 localDiff。
func (h *forgeHost) sawDiffFor(session string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, body := range h.diffs {
		if body["sessionId"] == session {
			return true
		}
	}
	return false
}

func (h *forgeHost) count(path string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, call := range h.calls {
		if call == path {
			n++
		}
	}
	return n
}

// newForgeHost 起假宿主：四个读方法与两个写方法都回**官方形状**。
func newForgeHost(t *testing.T) (*forgeHost, *httptest.Server) {
	t.Helper()
	host := &forgeHost{lastBody: map[string]map[string]any{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		host.mu.Lock()
		host.calls = append(host.calls, r.URL.Path)
		host.lastBody[r.URL.Path] = body
		if r.URL.Path == "/api/forgePanel.localDiff" {
			host.diffs = append(host.diffs, body)
		}
		host.mu.Unlock()
		switch r.URL.Path {
		case "/api/forgePanel.branches":
			if host.notRepo {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"code":"not_a_repository","message":"会话目录不在 git checkout 里","hint":"","retryable":false}}`))
				return
			}
			_, _ = w.Write([]byte(`{"current":"feat/base","branches":["main","feat/base"]}`))
		case "/api/forgePanel.localDiff":
			scope, _ := body["scope"].(map[string]any)
			kind, _ := scope["kind"].(string)
			files := `[{"path":"a.go","previousPath":null,"status":"modified","binary":false,"oversized":false,` +
				`"untracked":false,"additions":8,"deletions":2,` +
				`"hunks":[{"oldStart":1,"oldLines":2,"newStart":1,"newLines":3,` +
				`"lines":[" func main() {","-\told()","+\tnew()","+\tmore()"]}],` +
				`"layer":"implementation","digest":"d-a"}]`
			base := "null"
			if kind == "branch" {
				base = `"origin/main"`
			}
			_, _ = w.Write([]byte(`{"base":` + base + `,"files":` + files + `,"omitted":0}`))
		case "/api/forgePanel.diffBases":
			_, _ = w.Write([]byte(`{"automatic":{"name":"origin/main","worktree":false},"remote":[],"local":[]}`))
		case "/api/forgePanel.localCommits":
			_, _ = w.Write([]byte(`{"headSha":"abc123","commits":[{"sha":"abc123","parent":null,` +
				`"title":"一次提交","author":null,"authoredAt":"2026-11-15T00:00:00Z"}],"truncated":false}`))
		case "/api/forgePanel.requestWorktree":
			if host.failRequest {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"error":{"code":"busy","message":"已经有一次创建工作树在进行","hint":"","retryable":true}}`))
				return
			}
			_, _ = w.Write([]byte(`{}`))
		case "/api/forgePanel.cancelWorktree":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("意外路径：%s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return host, srv
}

// newForgeModel 造一个"已经在会话 s-1 上"的壳。
func newForgeModel(t *testing.T, host *forgeHost, srv *httptest.Server) *model {
	t.Helper()
	_ = host
	m := &model{cli: apiclient.New(srv.URL), cur: "s-1"}
	return m
}

func TestForgeForLoadsCheckoutAndBranchSummary(t *testing.T) {
	host, srv := newForgeHost(t)
	m := newForgeModel(t, host, srv)

	view := m.forgeFor()
	if view == nil {
		t.Fatal("有客户端、有会话时必须给出数据面")
	}
	if view.Checkout.Kind != apiclientCheckoutPending {
		t.Fatalf("首帧应当是 pending，实得 %q", view.Checkout.Kind)
	}
	waitUntil(t, "checkout ready", func() bool {
		v := m.forgeFor()
		return v != nil && v.Checkout.Kind == apiclientCheckoutReady
	})
	waitUntil(t, "本地摘要", func() bool {
		v := m.forgeFor()
		return v != nil && v.Local.Known
	})
	view = m.forgeFor()
	if view.Checkout.Branch != "feat/base" {
		t.Fatalf("分支 = %q", view.Checkout.Branch)
	}
	if view.Checkout.Detached {
		t.Fatal("有分支时不该标成分离 HEAD")
	}
	if len(view.Branches) != 2 {
		t.Fatalf("候选分支 = %v", view.Branches)
	}
	// 本地摘要来自 branch 范围（官方 ForgeLocalChanges 的语义）。
	if view.Local.Files != 1 || view.Local.Additions != 8 || view.Local.Deletions != 2 {
		t.Fatalf("本地摘要 = %+v", view.Local)
	}
	if view.Local.Base != "origin/main" {
		t.Fatalf("本地摘要的 base = %q（应当是宿主真的用来比对的 ref）", view.Local.Base)
	}
	// 默认范围是 branch（官方 changes-store 首次 navigation）。
	if view.Diff.Scope != "branch" {
		t.Fatalf("默认范围 = %q，官方是 branch", view.Diff.Scope)
	}
	// branch 范围的 wire 形状：base / commits 必填且为 null。
	scope, _ := host.body("/api/forgePanel.localDiff")["scope"].(map[string]any)
	if scope["kind"] != "branch" {
		t.Fatalf("scope.kind = %v", scope["kind"])
	}
	if v, ok := scope["base"]; !ok || v != nil {
		t.Fatalf("branch 范围必须显式发 base:null，实得 %v（present=%v）", v, ok)
	}
	if v, ok := scope["commits"]; !ok || v != nil {
		t.Fatalf("branch 范围必须显式发 commits:null，实得 %v（present=%v）", v, ok)
	}
	// diffBases 的自动分叉点要落到数据面（官方 `diff.base.automatic`）。
	waitUntil(t, "自动分叉点", func() bool {
		v := m.forgeFor()
		return v != nil && v.Diff.Automatic == "origin/main"
	})
	// branch 范围还要拉提交（官方 localCommits）。
	waitUntil(t, "提交列表", func() bool {
		v := m.forgeFor()
		return v != nil && len(v.Diff.Commits) == 1
	})
	if got := m.forgeFor().Diff.Commits[0].Title; got != "一次提交" {
		t.Fatalf("提交标题 = %q", got)
	}
}

func TestForgeScopeSwitchSendsKindOnly(t *testing.T) {
	host, srv := newForgeHost(t)
	m := newForgeModel(t, host, srv)
	waitUntil(t, "首轮读", func() bool {
		v := m.forgeFor()
		return v != nil && v.Local.Known
	})

	m.forgeScopeAction("staged")
	// 等**回包**而不是等"范围名改了"：范围名是同步改的，回包才是真读到。
	waitUntil(t, "切到 staged", func() bool {
		v := m.forgeFor()
		return v != nil && v.Diff.Scope == "staged" && !v.Diff.Loading && len(v.Diff.Files) > 0
	})
	scope, _ := host.body("/api/forgePanel.localDiff")["scope"].(map[string]any)
	// `additionalProperties:false`：非 branch 只能有 kind 一个键。
	if len(scope) != 1 || scope["kind"] != "staged" {
		t.Fatalf("staged 范围的 scope 只能有 kind，实得 %v", scope)
	}
	// 未知范围**不发请求**（官方 scope 是封闭联合）。
	before := host.count("/api/forgePanel.localDiff")
	m.forgeScopeAction("future-scope")
	if after := host.count("/api/forgePanel.localDiff"); after != before {
		t.Fatalf("未知范围不该发请求（%d → %d）", before, after)
	}
}

func TestForgeRequestWorktreePostsFromAndRollsBackOnFailure(t *testing.T) {
	host, srv := newForgeHost(t)
	m := newForgeModel(t, host, srv)
	waitUntil(t, "checkout ready", func() bool {
		v := m.forgeFor()
		return v != nil && v.Checkout.Kind == apiclientCheckoutReady
	})

	from := "main"
	m.forgeRequestWorktreeAction(&from)
	// 乐观态立刻可见（官方 WorktreeToggle 的 pending 同样是本地态）。
	if view := m.forgeFor(); !view.Worktree.Requested || !view.Worktree.Pending {
		t.Fatalf("请求发出后应当立刻是(已请求, 在飞)，实得 %+v", view.Worktree)
	}
	waitUntil(t, "请求回包", func() bool {
		v := m.forgeFor()
		return v != nil && !v.Worktree.Pending
	})
	if view := m.forgeFor(); !view.Worktree.Requested || view.Worktree.Err != "" {
		t.Fatalf("成功回包后应当保持已请求且无错，实得 %+v", view.Worktree)
	}
	if got := host.body("/api/forgePanel.requestWorktree")["from"]; got != "main" {
		t.Fatalf("from = %v，want main（官方 from:string|null）", got)
	}

	// 取消：请求撤回。
	m.forgeCancelWorktreeAction()
	waitUntil(t, "取消回包", func() bool {
		v := m.forgeFor()
		return v != nil && !v.Worktree.Requested && !v.Worktree.Pending
	})

	// 失败回滚：宿主拒绝时勾选态必须退回去（不许留下"勾着但其实没发出去"）。
	host.mu.Lock()
	host.failRequest = true
	host.mu.Unlock()
	m.forgeRequestWorktreeAction(nil)
	waitUntil(t, "失败回包", func() bool {
		v := m.forgeFor()
		return v != nil && !v.Worktree.Pending
	})
	view := m.forgeFor()
	if view.Worktree.Requested {
		t.Fatalf("失败后勾选态没回滚：%+v", view.Worktree)
	}
	if view.Worktree.Err == "" {
		t.Fatal("失败必须留下原因")
	}
}

func TestForgeNonRepositoryBecomesCheckoutNone(t *testing.T) {
	host, srv := newForgeHost(t)
	host.notRepo = true
	m := newForgeModel(t, host, srv)
	waitUntil(t, "checkout none", func() bool {
		v := m.forgeFor()
		return v != nil && v.Checkout.Kind == apiclientCheckoutNone
	})
	view := m.forgeFor()
	if view.Checkout.Message == "" {
		t.Fatal("none 态要带着宿主原话（界面照官方画 checkout.none，不自己编原因）")
	}
}

func TestForgeDropsResultsAfterSessionChange(t *testing.T) {
	host, srv := newForgeHost(t)
	m := newForgeModel(t, host, srv)
	m.forgeFor() // 触发 s-1 的后台读
	// 立刻换会话：s-1 的回包到达时必须被丢掉。
	m.mu.Lock()
	m.cur = "s-2"
	m.mu.Unlock()
	waitUntil(t, "s-2 数据面读起来", func() bool {
		v := m.forgeFor()
		return v != nil && v.SessionID == "s-2" && v.Local.Known
	})
	view := m.forgeFor()
	if view.SessionID != "s-2" {
		t.Fatalf("数据面会话 = %q，want s-2", view.SessionID)
	}
	if view.Diff.SessionID != "s-2" {
		t.Fatalf("diff 会话 = %q，want s-2（旧会话的慢回包必须丢掉）", view.Diff.SessionID)
	}
	// 换会话后发出的请求必须带新会话 id（旧会话的数据不再被请求/写入）。
	if !host.sawDiffFor("s-2") {
		t.Fatalf("换会话后没有为 s-2 发过 localDiff：%v", host.diffs)
	}
}

// apiclient 的常量别名（本文件断言里用得最多，避免反复写长前缀）。
const (
	apiclientCheckoutPending = "pending"
	apiclientCheckoutReady   = "ready"
	apiclientCheckoutNone    = "none"
)
