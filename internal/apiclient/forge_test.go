// forge_test.go —— `forgePanel.*` A 档 14 个客户端的**形状**测试。
//
// 判据只有一条：**wire 与官方 strict codec 逐字段一致**。这条不是洁癖——
// 官方 `ForgeLocalScope` 的四支联合里，`branch` 那支 `base`/`commits` 是
// **required（可空）**，其余三支是 `additionalProperties:false`；错一个方向都是
// 参数错（少发 → branch 缺键；多发 → uncommitted 多键）。所以这里既断言"该有的
// 有"，也断言"不该有的**一个都没有**"。
//
// 另一半断言的是**结果形状**：`localText` 是裸字符串、`worktreeCleanup` 允许 null、
// `branches.current` 可以是 null（分离 HEAD）。写成结构体/非指针就会静默解成零值。
package apiclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// forgeServer 起一个只认**单一**方法的假宿主，把请求体交给回调并回给定 result。
func forgeServer(t *testing.T, wantPath string, result string, onBody func(map[string]any)) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			t.Errorf("路径 = %q，want %q", r.URL.Path, wantPath)
		}
		if onBody != nil {
			onBody(decodeBody(t, r))
		}
		_, _ = w.Write([]byte(result))
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL)
}

func TestForgeBranchesDecodesNullCurrent(t *testing.T) {
	var body map[string]any
	cli := forgeServer(t, "/api/forgePanel.branches",
		`{"result":{"current":null,"branches":["main","feat/x"]}}`,
		func(b map[string]any) { body = b })
	got, err := cli.ForgeBranches(context.Background(), "s-1", 40)
	if err != nil {
		t.Fatalf("branches 应当成功：%v", err)
	}
	if body["sessionId"] != "s-1" {
		t.Fatalf("sessionId = %v", body["sessionId"])
	}
	if body["maxBranches"] != float64(40) {
		t.Fatalf("maxBranches = %v", body["maxBranches"])
	}
	// 分离 HEAD 时官方是 `current: null`，不是空串——指针必须保持 nil。
	if got.Current != nil {
		t.Fatalf("current 应当是 nil（分离 HEAD），实得 %q", *got.Current)
	}
	if len(got.Branches) != 2 || got.Branches[0] != "main" {
		t.Fatalf("branches = %v", got.Branches)
	}
}

func TestForgeLocalDiffBranchScopeCarriesBaseAndCommits(t *testing.T) {
	var body map[string]any
	cli := forgeServer(t, "/api/forgePanel.localDiff",
		`{"result":{"base":"origin/main","files":[{"path":"a.go","previousPath":null,"status":"modified","binary":false,"oversized":false,"untracked":false,"additions":3,"deletions":1,"hunks":[{"oldStart":1,"oldLines":1,"newStart":1,"newLines":2,"lines":[" a","+b","+c"]}],"layer":"implementation","digest":"d1"}],"omitted":2}}`,
		func(b map[string]any) { body = b })
	scope := ForgeLocalScope{Kind: ForgeScopeBranch}
	got, err := cli.ForgeLocalDiff(context.Background(), "s-1", scope, 0)
	if err != nil {
		t.Fatalf("localDiff 应当成功：%v", err)
	}
	// branch 那支：base / commits 必须在，且都是 null。
	wire, ok := body["scope"].(map[string]any)
	if !ok {
		t.Fatalf("scope 不是对象：%v", body["scope"])
	}
	if wire["kind"] != ForgeScopeBranch {
		t.Fatalf("scope.kind = %v", wire["kind"])
	}
	if v, present := wire["base"]; !present || v != nil {
		t.Fatalf("branch 范围的 base 必须显式为 null（官方 required+nullable），实得 %v（present=%v）", v, present)
	}
	if v, present := wire["commits"]; !present || v != nil {
		t.Fatalf("branch 范围的 commits 必须显式为 null，实得 %v（present=%v）", v, present)
	}
	if _, present := body["maxDiffBytes"]; present {
		t.Fatalf("maxDiffBytes<=0 表示不传，实际发了：%v", body["maxDiffBytes"])
	}
	if got.Base == nil || *got.Base != "origin/main" {
		t.Fatalf("base = %v", got.Base)
	}
	if len(got.Files) != 1 || got.Files[0].Layer != ForgeLayerImplementation || got.Files[0].Digest != "d1" {
		t.Fatalf("files = %+v", got.Files)
	}
	if len(got.Files[0].Hunks) != 1 || len(got.Files[0].Hunks[0].Lines) != 3 {
		t.Fatalf("hunks = %+v", got.Files[0].Hunks)
	}
	if got.Omitted != 2 {
		t.Fatalf("omitted = %d", got.Omitted)
	}
}

func TestForgeLocalDiffSelectedBaseCommitsAndFileStates(t *testing.T) {
	var body map[string]any
	cli := forgeServer(t, "/api/forgePanel.localDiff",
		`{"result":{"base":"origin/main","files":[{"path":"new.go","previousPath":"old.go","status":"renamed","binary":false,"oversized":false,"untracked":false,"additions":0,"deletions":0,"hunks":[],"layer":"test","digest":"digest-renamed"}],"omitted":3}}`,
		func(b map[string]any) { body = b })
	base := "refs/remotes/origin/main"
	from := "1111111111111111111111111111111111111111"
	to := "2222222222222222222222222222222222222222"
	scope := ForgeLocalScope{Kind: ForgeScopeBranch, Base: &base, Commits: &ForgeCommitRange{From: from, To: to}}
	got, err := cli.ForgeLocalDiff(context.Background(), "s-1", scope, 0)
	if err != nil {
		t.Fatalf("localDiff 应当成功：%v", err)
	}
	wire, _ := body["scope"].(map[string]any)
	if wire["base"] != base {
		t.Fatalf("选中 base 没有发给宿主：%v", wire)
	}
	commits, _ := wire["commits"].(map[string]any)
	if commits["from"] != from || commits["to"] != to {
		t.Fatalf("选中 commits 没有发给宿主：%v", wire)
	}
	if len(got.Files) != 1 {
		t.Fatalf("files = %+v", got.Files)
	}
	file := got.Files[0]
	if file.Status != ForgeFileRenamed || file.PreviousPath == nil || *file.PreviousPath != "old.go" ||
		file.Layer != ForgeLayerTest || file.Digest != "digest-renamed" || got.Omitted != 3 {
		t.Fatalf("官方文件字段/omitted 解码不完整：%+v omitted=%d", file, got.Omitted)
	}
}

func TestForgeLocalDiffOtherScopesCarryKindOnly(t *testing.T) {
	for _, kind := range []string{ForgeScopeUncommitted, ForgeScopeUnstaged, ForgeScopeStaged} {
		t.Run(kind, func(t *testing.T) {
			var body map[string]any
			cli := forgeServer(t, "/api/forgePanel.localDiff", `{"result":{"base":null,"files":[],"omitted":0}}`,
				func(b map[string]any) { body = b })
			if _, err := cli.ForgeLocalDiff(context.Background(), "s-1", ForgeLocalScope{Kind: kind}, 0); err != nil {
				t.Fatalf("localDiff 应当成功：%v", err)
			}
			wire, _ := body["scope"].(map[string]any)
			// `additionalProperties:false`：这三个 kind 只能有 kind 一个键。
			if len(wire) != 1 || wire["kind"] != kind {
				t.Fatalf("scope 只能有 kind 一个键（官方 additionalProperties:false），实得 %v", wire)
			}
		})
	}
}

func TestForgeLocalTextIsBareString(t *testing.T) {
	var body map[string]any
	cli := forgeServer(t, "/api/forgePanel.localText", `{"result":"line1\nline2\n"}`, func(b map[string]any) { body = b })
	got, err := cli.ForgeLocalText(context.Background(), "s-1", ForgeLocalScope{Kind: ForgeScopeStaged}, "a/b.go")
	if err != nil {
		t.Fatalf("localText 应当成功：%v", err)
	}
	if got != "line1\nline2\n" {
		t.Fatalf("localText 结果应当原样是字符串，实得 %q", got)
	}
	if body["path"] != "a/b.go" {
		t.Fatalf("path = %v", body["path"])
	}
	if wire, _ := body["scope"].(map[string]any); wire["kind"] != ForgeScopeStaged || len(wire) != 1 {
		t.Fatalf("staged 范围只能有 kind，实得 %v", wire)
	}
}

func TestForgeDiffBasesDecodesAutomaticAndRefs(t *testing.T) {
	var body map[string]any
	cli := forgeServer(t, "/api/forgePanel.diffBases",
		`{"result":{"automatic":{"name":"origin/main","worktree":false},"remote":[{"ref":"refs/remotes/origin/main","name":"origin/main"}],"local":[{"ref":"refs/heads/main","name":"main"}]}}`,
		func(b map[string]any) { body = b })
	got, err := cli.ForgeDiffBases(context.Background(), "s-1", "ma")
	if err != nil {
		t.Fatalf("diffBases 应当成功：%v", err)
	}
	if body["query"] != "ma" {
		t.Fatalf("query = %v", body["query"])
	}
	if got.Automatic == nil || got.Automatic.Name != "origin/main" || got.Automatic.Worktree {
		t.Fatalf("automatic = %+v", got.Automatic)
	}
	if len(got.Remote) != 1 || got.Remote[0].Ref != "refs/remotes/origin/main" {
		t.Fatalf("remote = %+v", got.Remote)
	}
	if len(got.Local) != 1 || got.Local[0].Name != "main" {
		t.Fatalf("local = %+v", got.Local)
	}
}

func TestForgeLocalCommitsSendsExplicitNullBase(t *testing.T) {
	var body map[string]any
	cli := forgeServer(t, "/api/forgePanel.localCommits",
		`{"result":{"headSha":"abc","commits":[{"sha":"abc","parent":null,"title":"根提交","author":null,"authoredAt":"2026-11-15T00:00:00Z"}],"truncated":true}}`,
		func(b map[string]any) { body = b })
	got, err := cli.ForgeLocalCommits(context.Background(), "s-1", nil)
	if err != nil {
		t.Fatalf("localCommits 应当成功：%v", err)
	}
	// 官方 `base: string | null` 是必填：自动分叉点必须显式发 null。
	if v, present := body["base"]; !present || v != nil {
		t.Fatalf("base 必须显式为 null，实得 %v（present=%v）", v, present)
	}
	if got.HeadSHA != "abc" || len(got.Commits) != 1 || !got.Truncated {
		t.Fatalf("localCommits = %+v", got)
	}
	if got.Commits[0].Parent != nil || got.Commits[0].Author != nil {
		t.Fatalf("parent/author 官方都是 nullable，null 不该变成空串：%+v", got.Commits[0])
	}
}

func TestForgeWorktreeLifecycleSendsSessionOnly(t *testing.T) {
	cases := []struct {
		path   string
		call   func(cli *Client) error
		params func(t *testing.T, body map[string]any)
	}{
		{"/api/forgePanel.requestWorktree", func(cli *Client) error {
			return cli.ForgeRequestWorktree(context.Background(), "s-1", nil)
		}, func(t *testing.T, body map[string]any) {
			// 官方 `from: string | null` 是必填：基于 HEAD 时显式 null。
			if v, present := body["from"]; !present || v != nil {
				t.Fatalf("from 必须显式 null，实得 %v（present=%v）", v, present)
			}
		}},
		{"/api/forgePanel.cancelWorktree", func(cli *Client) error {
			return cli.ForgeCancelWorktree(context.Background(), "s-1")
		}, nil},
		{"/api/forgePanel.retryWorktree", func(cli *Client) error {
			return cli.ForgeRetryWorktree(context.Background(), "s-1")
		}, nil},
		{"/api/forgePanel.stopCreation", func(cli *Client) error {
			return cli.ForgeStopCreation(context.Background(), "s-1")
		}, nil},
		{"/api/forgePanel.restoreWorktree", func(cli *Client) error {
			return cli.ForgeRestoreWorktree(context.Background(), "s-1")
		}, nil},
		{"/api/forgePanel.continueInProject", func(cli *Client) error {
			return cli.ForgeContinueInProject(context.Background(), "s-1")
		}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			var body map[string]any
			cli := forgeServer(t, tc.path, `{}`, func(b map[string]any) { body = b })
			if err := tc.call(cli); err != nil {
				t.Fatalf("%s 应当成功：%v", tc.path, err)
			}
			if body["sessionId"] != "s-1" {
				t.Fatalf("sessionId = %v", body["sessionId"])
			}
			if tc.params != nil {
				tc.params(t, body)
			} else if len(body) != 1 {
				// 其余五个官方只有 sessionId 一个参数。
				t.Fatalf("%s 只该有 sessionId，实得 %v", tc.path, body)
			}
		})
	}
}

func TestForgeRequestWorktreeSendsNamedBase(t *testing.T) {
	var body map[string]any
	cli := forgeServer(t, "/api/forgePanel.requestWorktree", `{}`, func(b map[string]any) { body = b })
	base := "feat/base"
	if err := cli.ForgeRequestWorktree(context.Background(), "s-1", &base); err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if body["from"] != "feat/base" {
		t.Fatalf("from = %v", body["from"])
	}
}

func TestForgeTrackDismissSendUrlAndIgnoreBinding(t *testing.T) {
	for _, tc := range []struct {
		path string
		call func(cli *Client) error
	}{
		{"/api/forgePanel.track", func(cli *Client) error {
			return cli.ForgeTrack(context.Background(), "s-1", "https://github.com/o/r/pull/7")
		}},
		{"/api/forgePanel.dismiss", func(cli *Client) error {
			return cli.ForgeDismiss(context.Background(), "s-1", "https://github.com/o/r/pull/7")
		}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			var body map[string]any
			// 宿主多回了一个 binding（已知宽容偏差 G5）：客户端仍按官方 void 处理。
			cli := forgeServer(t, tc.path, `{"result":{"binding":{"sessionId":"s-1","url":"https://github.com/o/r/pull/7","tracked":true}}}`,
				func(b map[string]any) { body = b })
			if err := tc.call(cli); err != nil {
				t.Fatalf("%s 应当成功：%v", tc.path, err)
			}
			if body["url"] != "https://github.com/o/r/pull/7" {
				t.Fatalf("url = %v", body["url"])
			}
			if body["sessionId"] != "s-1" {
				t.Fatalf("sessionId = %v", body["sessionId"])
			}
		})
	}
}

func TestForgeWorktreeCleanupNullAndObject(t *testing.T) {
	var body map[string]any
	cli := forgeServer(t, "/api/forgePanel.worktreeCleanup", `{"result":null}`, func(b map[string]any) { body = b })
	got, err := cli.ForgeWorktreeCleanup(context.Background(), "s-1", nil)
	if err != nil {
		t.Fatalf("cleanup 应当成功：%v", err)
	}
	// 官方结果允许 null：没有服务/没尝试/rev 不匹配都是 null，不是"空对象"。
	if got != nil {
		t.Fatalf("null 应当解成 nil，实得 %+v", got)
	}
	if v, present := body["revision"]; !present || v != nil {
		t.Fatalf("revision 必须显式 null（官方 oneOf null|number），实得 %v（present=%v）", v, present)
	}

	body = nil
	cli2 := forgeServer(t, "/api/forgePanel.worktreeCleanup", `{"result":{"revision":4,"status":"retained"}}`,
		func(b map[string]any) { body = b })
	wait := int64(4)
	got2, err := cli2.ForgeWorktreeCleanup(context.Background(), "s-1", &wait)
	if err != nil {
		t.Fatalf("cleanup 应当成功：%v", err)
	}
	if got2 == nil || got2.Revision != 4 || got2.Status != ForgeCleanupRetained {
		t.Fatalf("cleanup = %+v", got2)
	}
	if body["revision"] != float64(4) {
		t.Fatalf("revision = %v", body["revision"])
	}
}

func TestForgeErrorsKeepOfficialCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"not_a_repository","message":"会话目录不在 git checkout 里","hint":"换一个会话目录","retryable":false}}`))
	}))
	defer srv.Close()
	_, err := New(srv.URL).ForgeBranches(context.Background(), "s-1", 0)
	if err == nil {
		t.Fatal("非 git 目录必须报错")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("错误应当是 *APIError，实得 %T: %v", err, err)
	}
	if apiErr.Code != "not_a_repository" {
		t.Fatalf("官方码要原样留下（界面据此画 checkout.none），实得 %q", apiErr.Code)
	}
}

// TestForgePanelFollowStreamsWholeStates 钉住 follow 的**流形状**：
//
//	每行一个完整 `ForgePanelState`（不是 `{type,value}` 那种差异帧）
//	worktree 三格（editable / blocker / creation）逐字段解出来
//	body 只有 sessionId 一个参数（官方 `forgePanel.follow(sessionId)`）
//
// 这条判据的意义：宿主 dsh-go#84 之前流的是绑定清单帧，界面因此读不到
// editable/creation（gap G6）。形状一旦回退，这条必红。
func TestForgePanelFollowStreamsWholeStates(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/forgePanel.follow" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		body = decodeBody(t, r)
		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		frames := []string{
			// 第一帧：能建（editable=true）但没有创建在跑；blocker 是只读。
			`{"directory":"/repo","checkout":{"state":"none"},"repository":null,"readiness":null,` +
				`"changeRequests":[],"local":null,` +
				`"worktree":{"editable":true,"blocker":"read-only","request":null,"created":null,` +
				`"creation":null,"failure":null,"missing":null,"restoration":null,"waiting":null}}`,
			// 第二帧：创建在跑（prepare，held），editable 变成 false、blocker 清空。
			`{"directory":"/repo","checkout":{"state":"none"},"repository":null,"readiness":null,` +
				`"changeRequests":[],"local":null,` +
				`"worktree":{"editable":false,"blocker":null,"request":{"from":"main"},"created":null,` +
				`"creation":{"phase":"prepare","branch":"dsh/feat","from":"main","held":true},` +
				`"failure":null,"missing":null,"restoration":null,"waiting":"creation"}}`,
			// 第三帧：前进到 checkout。
			`{"directory":"/repo","checkout":{"state":"none"},"repository":null,"readiness":null,` +
				`"changeRequests":[],"local":null,` +
				`"worktree":{"editable":false,"blocker":null,"request":{"from":"main"},"created":null,` +
				`"creation":{"phase":"checkout","branch":"dsh/feat","from":null,"held":true},` +
				`"failure":null,"missing":null,"restoration":null,"waiting":"creation"}}`,
		}
		for _, frame := range frames {
			_, _ = w.Write([]byte(frame + "\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var got []ForgePanelState
	err := New(srv.URL).ForgePanelFollow(ctx, "s-1", func(state ForgePanelState) error {
		got = append(got, state)
		if len(got) == 3 {
			cancel()
			return context.Canceled
		}
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("follow 不该报错：%v", err)
	}
	if body["sessionId"] != "s-1" || len(body) != 1 {
		t.Fatalf("follow 的参数只该有 sessionId，实得 %v", body)
	}
	if len(got) != 3 {
		t.Fatalf("应当收到 3 帧，实得 %d", len(got))
	}
	if !got[0].Worktree.Editable || got[0].Worktree.Blocker == nil || *got[0].Worktree.Blocker != "read-only" {
		t.Fatalf("第一帧 worktree = %+v", got[0].Worktree)
	}
	if got[0].Worktree.Creation != nil {
		t.Fatalf("第一帧没有创建，creation 必须是 nil：%+v", got[0].Worktree.Creation)
	}
	second := got[1].Worktree
	if second.Editable || second.Blocker != nil {
		t.Fatalf("第二帧应当 editable=false、blocker=null：%+v", second)
	}
	if second.Creation == nil || second.Creation.Phase != "prepare" || !second.Creation.Held ||
		second.Creation.Branch != "dsh/feat" || second.Creation.From == nil || *second.Creation.From != "main" {
		t.Fatalf("第二帧 creation = %+v", second.Creation)
	}
	third := got[2].Worktree
	if third.Creation == nil || third.Creation.Phase != "checkout" {
		t.Fatalf("第三帧应当前进到 checkout：%+v", third.Creation)
	}
	if third.Creation.From != nil {
		t.Fatalf("from 为 null 时必须是 nil（基于 HEAD），实得 %q", *third.Creation.From)
	}
}

// TestForgePanelFollowReportsStreamErrorFrame：宿主在头已发出之后只能在流里报错，
// 客户端必须把它当四字段错误返回，不能静默当成"流正常结束"。
func TestForgePanelFollowReportsStreamErrorFrame(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":{"code":"internal","message":"序列化 ForgePanelState 失败","hint":"","retryable":false}}` + "\n"))
	}))
	defer srv.Close()
	err := New(srv.URL).ForgePanelFollow(context.Background(), "s-1", func(ForgePanelState) error { return nil })
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("流里的错误帧要解成 *APIError，实得 %T: %v", err, err)
	}
	if apiErr.Code != "internal" || apiErr.Message == "" {
		t.Fatalf("错误必须原样保留四字段：%+v", apiErr)
	}
}

// TestForgePanelFollowRequiresNdjson：宿主回 200 但不是 NDJSON 时立刻报错——
// 否则一行都解不出来，界面会以为"宿主一直没有状态"。
func TestForgePanelFollowRequiresNdjson(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	defer srv.Close()
	err := New(srv.URL).ForgePanelFollow(context.Background(), "s-1", func(ForgePanelState) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "application/x-ndjson") {
		t.Fatalf("非 NDJSON 应答必须报错并点明内容类型，实得 %v", err)
	}
}
