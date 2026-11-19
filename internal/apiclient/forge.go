// forge.go —— 官方 `forgePanel.*` 的**A 档 14 个方法 + 1 条面板状态流**的客户端（GUI 账本 #64）。
//
// 第 15 条是 `ForgePanelFollow`（`forgePanel.follow`，官方 `mode:'stream'`）：宿主
// dsh-go#84 起按官方形状发整帧 `ForgePanelState`（NDJSON 一行一帧），
// `worktree.editable` / `blocker` / `creation` 的唯一读出口就是它。
//
// # 契约唯一来源是官方 rc.2 源码（不是我们的宿主，也不是推理）
//
//	OFF:dsh-experimental-client-ui-forge/src/index.ts:222   @Remote branches(session, maxBranches?)
//	OFF:…/src/index.ts:301                                   @Remote localDiff(session, scope, maxDiffBytes?)
//	OFF:…/src/index.ts:347                                   @Remote localText(session, scope, path)
//	OFF:…/src/index.ts:236                                   @Remote diffBases(session, query)
//	OFF:…/src/index.ts:323                                   @Remote localCommits(session, base)
//	OFF:…/src/index.ts:729                                   @Remote requestWorktree(session, from)
//	OFF:…/src/index.ts:748 / 777 / 764 / 798 / 829            cancel / retry / stopCreation / restore / continueInProject
//	OFF:…/src/index.ts:253 / 268                             @Remote track / dismiss
//	OFF:…/src/index.ts:210                                   @Remote worktreeCleanup(session, revision)
//	形状逐字段见 `lib/typert.host.js` 的注册块（行号在各方法注释里）。
//
// # 两条形状硬事实（照官方，不照我们的方便）
//
//  1. **五个 worktree 生命周期方法的结果是 `z.void()`**（`requestWorktree` /
//     `cancelWorktree` / `retryWorktree` / `stopCreation` / `restoreWorktree` /
//     `continueInProject`，六个都是）。所以这里签名是 `error` 而不是某个结构体。
//     `track` / `dismiss` 官方也是 void，但**本仓宿主多回了一个 `{binding}`**——
//     这是已知宽容偏差（DSH-GO 的 gap 报告 G5）。客户端按官方 void 处理：
//     不解析结果，也不把它当成"宿主确认了绑定内容"的证据。
//  2. **`localText` 与 `localDiff` 的结果分别是裸字符串和对象**：`localText` 是
//     `z.string()`，写成结构体会静默解成零值（#63 的 `translate` 踩过同一坑）。
//
// # 宿主侧真实现落点（逐条核对过，2026-11-15）
//
//	../DSH-GO/cmd/dshd/serve_forge_git.go:54/62/74      branches / localDiff / localText
//	../DSH-GO/cmd/dshd/serve_forge_state.go             track / dismiss
//	../DSH-GO/cmd/dshd/serve_forge_rc2.go:235/257/330   diffBases / localCommits / worktreeCleanup
//	../DSH-GO/cmd/dshd/serve_worktree_api.go:173        worktree 6 方法（internal/worktree 状态机）
//
// # branch scope 的完整接线
//
// `base` / `commits` 由 forgeScopeWire 按官方 required+nullable 形状发送；宿主
// `serve_forge_git.go` 解成完整 `forge.Scope`，分别走 chosenForkPoint 与连续提交
// diff。当前 GUI 尚未实现 BaseMenu / CommitMenu，所以壳仍传 null（自动分叉点、
// 全部提交）；这是未做的菜单，不再是对端忽略字段的限制。
package apiclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// 官方 `ForgeLocalScope` 的四个 kind（OFF:…/src/types.ts 的 ForgeLocalScope 联合）。
const (
	// ForgeScopeBranch 是"本分支相对分叉点的全部改动"。
	ForgeScopeBranch = "branch"
	// ForgeScopeUncommitted 是"相对 HEAD，含未跟踪文件"（工作树 + 暂存）。
	ForgeScopeUncommitted = "uncommitted"
	// ForgeScopeUnstaged 是"相对暂存区，含未跟踪文件"。
	ForgeScopeUnstaged = "unstaged"
	// ForgeScopeStaged 是"暂存区相对 HEAD"。
	ForgeScopeStaged = "staged"
)

// 官方 `ForgeDiffFile.status` 的四个取值。
const (
	ForgeFileDeleted  = "deleted"
	ForgeFileAdded    = "added"
	ForgeFileModified = "modified"
	ForgeFileRenamed  = "renamed"
)

// 官方 `ForgeDiffFile.layer` 的三个取值（评审分层）。
const (
	ForgeLayerInterface      = "interface"
	ForgeLayerImplementation = "implementation"
	ForgeLayerTest           = "test"
)

// ForgeBranches 是 `forgePanel.branches` 的结果（官方 ForgeBranches）。
//
// Current 是**指针**：官方 `z.string().nullable()` —— 分离 HEAD 时是 null，
// 那不是"空分支名"，界面要据此走 `checkout.detached` 那一句。
type ForgeBranches struct {
	Current  *string  `json:"current"`
	Branches []string `json:"branches"`
}

// ForgeCommitRange 是官方 `branch` 范围里的 `commits:{from,to}`（可选）。
type ForgeCommitRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ForgeLocalScope 是官方 `ForgeLocalScope` 联合。
//
// Base / Commits 只在 Kind == ForgeScopeBranch 时有意义。别在 struct tag 上用
// omitempty 了事：官方 strict codec 对**非 branch** 的三种 kind 是
// `additionalProperties:false`，多发一个 `base:null` 就是参数错；而对 branch 那支
// `base` / `commits` 是**必填（可空）**，少发也是参数错。所以 wire 形状由
// forgeScopeWire 逐 kind 造，见那里的注释。
type ForgeLocalScope struct {
	Kind string `json:"kind"`
	// Base 只在 Kind == ForgeScopeBranch 时有意义；nil = 自动分叉点。
	Base *string `json:"-"`
	// Commits 只在 Kind == ForgeScopeBranch 时有意义；nil = 整个范围。
	Commits *ForgeCommitRange `json:"-"`
}

// forgeScopeWire 造官方 strict codec 认的那份 scope JSON。
//
//	kind=branch          {kind, base (string|null), commits ({from,to}|null)}   ← 两键必填
//	kind=uncommitted/…   {kind}                                                ← 只有 kind
//
// 依据：`OFF:…/lib/typert.host.js` 的 `forgePanel/localDiff` 参数 jsonSchema
// （branch 那支 `additionalProperties:true` 但 `required:[kind,base,commits]`；
// 其余三支 `additionalProperties:false`）。
func forgeScopeWire(scope ForgeLocalScope) map[string]any {
	wire := map[string]any{"kind": scope.Kind}
	if scope.Kind != ForgeScopeBranch {
		return wire
	}
	wire["base"] = nil
	if scope.Base != nil {
		wire["base"] = *scope.Base
	}
	wire["commits"] = nil
	if scope.Commits != nil {
		wire["commits"] = map[string]any{"from": scope.Commits.From, "to": scope.Commits.To}
	}
	return wire
}

// ForgeHunk 是官方 `ForgeHunk`：一段 hunk 的坐标 + patch 形式行。
//
// Lines 每行首字符是 ` ` / `+` / `-` / `\`（官方那句：a leading space for context,
// `+` for added, `-` for deleted, and `\` for a missing final newline）。
type ForgeHunk struct {
	OldStart int      `json:"oldStart"`
	OldLines int      `json:"oldLines"`
	NewStart int      `json:"newStart"`
	NewLines int      `json:"newLines"`
	Lines    []string `json:"lines"`
}

// ForgeDiffFile 是官方 `ForgeDiffFile`。
type ForgeDiffFile struct {
	Path         string      `json:"path"`
	PreviousPath *string     `json:"previousPath"`
	Status       string      `json:"status"`
	Binary       bool        `json:"binary"`
	Oversized    bool        `json:"oversized"`
	Untracked    bool        `json:"untracked"`
	Additions    int         `json:"additions"`
	Deletions    int         `json:"deletions"`
	Hunks        []ForgeHunk `json:"hunks"`
	Layer        string      `json:"layer"`
	Digest       string      `json:"digest"`
}

// ForgeLocalDiff 是官方 `ForgeLocalDiff`。
//
// Base 是 `branch` 范围比对的 ref（例如 `origin/master`），其余范围是 null。
// Omitted 是"超出 maxUntrackedFiles 被略掉的未跟踪文件数"——界面照官方
// `diff.omitted` 那句如实说，不把它算进 files 长度。
type ForgeLocalDiff struct {
	Base    *string         `json:"base"`
	Files   []ForgeDiffFile `json:"files"`
	Omitted int             `json:"omitted"`
}

// ForgeBaseRef 是 `diffBases` 的一个分支（full ref + 简称）。
type ForgeBaseRef struct {
	Ref  string `json:"ref"`
	Name string `json:"name"`
}

// ForgeBasePoint 是 `diffBases.automatic`：没有选基准时用的分叉点。
//
// Worktree 为 true 表示这个分叉点是本会话创建的工作树的起点提交
// （官方 `forkPoint`：created.path 就是会话 checkout 时用 created.baseCommit）。
type ForgeBasePoint struct {
	Name     string `json:"name"`
	Worktree bool   `json:"worktree"`
}

// ForgeBaseBranches 是官方 `ForgeBaseBranches`。
type ForgeBaseBranches struct {
	Automatic *ForgeBasePoint `json:"automatic"`
	Remote    []ForgeBaseRef  `json:"remote"`
	Local     []ForgeBaseRef  `json:"local"`
}

// ForgeCommit 是官方 `ForgeCommit`（`localCommits` 的一项）。
type ForgeCommit struct {
	SHA        string  `json:"sha"`
	Parent     *string `json:"parent"`
	Title      string  `json:"title"`
	Author     *string `json:"author"`
	AuthoredAt string  `json:"authoredAt"`
}

// ForgeLocalCommits 是官方 `ForgeLocalCommits`：fork point 到 HEAD 之间最老的在前。
type ForgeLocalCommits struct {
	HeadSHA   string        `json:"headSha"`
	Commits   []ForgeCommit `json:"commits"`
	Truncated bool          `json:"truncated"`
}

// ForgeWorktreeCleanup 是官方 `ForgeWorktreeCleanup`（`worktreeCleanup` 的 null 分支之外）。
//
// Status 四个取值（官方 eviction 状态机）：`scheduled` / `retained` / `removed` / `failed`。
type ForgeWorktreeCleanup struct {
	Revision int64  `json:"revision"`
	Status   string `json:"status"`
}

// 官方 `ForgeWorktreeCleanup.status` 的四个取值。
const (
	ForgeCleanupScheduled = "scheduled"
	ForgeCleanupRetained  = "retained"
	ForgeCleanupRemoved   = "removed"
	ForgeCleanupFailed    = "failed"
)

// ── 本地 git 三条 ────────────────────────────────────────────────────────

// ForgeBranches 读会话 checkout 的本地分支（官方 `forgePanel/branches`）。
//
// maxBranches <= 0 表示"不传"，由宿主按自己的上限取（官方 `maxBranches?`）。
func (c *Client) ForgeBranches(ctx context.Context, sessionID string, maxBranches int) (ForgeBranches, error) {
	params := struct {
		SessionID   string `json:"sessionId"`
		MaxBranches int    `json:"maxBranches,omitempty"`
	}{SessionID: sessionID, MaxBranches: maxBranches}
	var out ForgeBranches
	if err := c.Call(ctx, "forgePanel.branches", params, &out); err != nil {
		return ForgeBranches{}, err
	}
	return out, nil
}

// ForgeLocalDiff 读一个范围的本地改动（官方 `forgePanel/localDiff`）。
//
// maxDiffBytes <= 0 表示"不传"（宿主自己的上限）。返回的 Base 是**宿主真的
// 拿来做比对的那个 ref**，界面显示它而不是自己猜基准。
func (c *Client) ForgeLocalDiff(ctx context.Context, sessionID string, scope ForgeLocalScope, maxDiffBytes int) (ForgeLocalDiff, error) {
	params := struct {
		SessionID    string         `json:"sessionId"`
		Scope        map[string]any `json:"scope"`
		MaxDiffBytes int            `json:"maxDiffBytes,omitempty"`
	}{SessionID: sessionID, Scope: forgeScopeWire(scope), MaxDiffBytes: maxDiffBytes}
	var out ForgeLocalDiff
	if err := c.Call(ctx, "forgePanel.localDiff", params, &out); err != nil {
		return ForgeLocalDiff{}, err
	}
	return out, nil
}

// ForgeLocalText 读一个改动文件的**新侧全文**（官方 `forgePanel/localText`）。
//
// 结果是裸字符串（官方 `z.string()`）：staged 读 index，branch 的选定提交读那个
// 提交，其余读工作树。
func (c *Client) ForgeLocalText(ctx context.Context, sessionID string, scope ForgeLocalScope, path string) (string, error) {
	params := struct {
		SessionID string         `json:"sessionId"`
		Scope     map[string]any `json:"scope"`
		Path      string         `json:"path"`
	}{SessionID: sessionID, Scope: forgeScopeWire(scope), Path: path}
	var out string
	if err := c.Call(ctx, "forgePanel.localText", params, &out); err != nil {
		return "", err
	}
	return out, nil
}

// ForgeDiffBases 列可选的基准分支与自动分叉点（官方 `forgePanel/diffBases`）。
//
// Automatic 与 remote/local 都按官方形状解码；当前 Changes 视图只显示 Automatic，
// BaseMenu 选择交互仍未实现（见 #64 账本），但 `localDiff` 客户端已能发送选中 ref。
func (c *Client) ForgeDiffBases(ctx context.Context, sessionID, query string) (ForgeBaseBranches, error) {
	params := struct {
		SessionID string `json:"sessionId"`
		Query     string `json:"query"`
	}{SessionID: sessionID, Query: query}
	var out ForgeBaseBranches
	if err := c.Call(ctx, "forgePanel.diffBases", params, &out); err != nil {
		return ForgeBaseBranches{}, err
	}
	return out, nil
}

// ForgeLocalCommits 列 fork point 到 HEAD 的提交（官方 `forgePanel/localCommits`）。
//
// base 为 nil 时发 `null`（官方 `base: string | null`，null = 自动分叉点）。
func (c *Client) ForgeLocalCommits(ctx context.Context, sessionID string, base *string) (ForgeLocalCommits, error) {
	params := struct {
		SessionID string  `json:"sessionId"`
		Base      *string `json:"base"`
	}{SessionID: sessionID, Base: base}
	var out ForgeLocalCommits
	if err := c.Call(ctx, "forgePanel.localCommits", params, &out); err != nil {
		return ForgeLocalCommits{}, err
	}
	return out, nil
}

// ── worktree 生命周期六条（官方结果都是 z.void()）─────────────────────────

// ForgeRequestWorktree 请求"本会话在独立工作树里跑"（官方 `forgePanel/requestWorktree`）。
//
// from 为 nil 表示基于源 checkout 的 HEAD；否则是基准分支名。**成功 = 请求被记录**，
// 不是"工作树已建好"——创建发生在会话的第一步之前，界面要如实画成"等待创建"。
func (c *Client) ForgeRequestWorktree(ctx context.Context, sessionID string, from *string) error {
	params := struct {
		SessionID string  `json:"sessionId"`
		From      *string `json:"from"`
	}{SessionID: sessionID, From: from}
	return c.Call(ctx, "forgePanel.requestWorktree", params, nil)
}

// ForgeCancelWorktree 撤回未创建的工作树请求（官方 `forgePanel/cancelWorktree`）。
func (c *Client) ForgeCancelWorktree(ctx context.Context, sessionID string) error {
	return c.Call(ctx, "forgePanel.cancelWorktree", forgeSessionOnly(sessionID), nil)
}

// ForgeRetryWorktree 再试一次被失败/停下挡住的那次创建（官方 `forgePanel/retryWorktree`）。
func (c *Client) ForgeRetryWorktree(ctx context.Context, sessionID string) error {
	return c.Call(ctx, "forgePanel.retryWorktree", forgeSessionOnly(sessionID), nil)
}

// ForgeStopCreation 停下一条消息正在等的那次创建（官方 `forgePanel/stopCreation`）。
//
// 官方语义：消息**继续等**，请求仍然开着（与 cancel 不同）。
func (c *Client) ForgeStopCreation(ctx context.Context, sessionID string) error {
	return c.Call(ctx, "forgePanel.stopCreation", forgeSessionOnly(sessionID), nil)
}

// ForgeRestoreWorktree 把被移除的工作树重新检出来（官方 `forgePanel/restoreWorktree`）。
func (c *Client) ForgeRestoreWorktree(ctx context.Context, sessionID string) error {
	return c.Call(ctx, "forgePanel.restoreWorktree", forgeSessionOnly(sessionID), nil)
}

// ForgeContinueInProject 让会话回到工作树的源 checkout 继续（官方 `forgePanel/continueInProject`）。
func (c *Client) ForgeContinueInProject(ctx context.Context, sessionID string) error {
	return c.Call(ctx, "forgePanel.continueInProject", forgeSessionOnly(sessionID), nil)
}

// ── 面板状态两条（track / dismiss）──────────────────────────────────────

// ForgeTrack 把一个变更请求按 URL 绑到会话上（官方 `forgePanel/track`）。
//
// 官方结果是 void；宿主回 `{binding}`（已知宽容偏差 G5），这里**不解析**——
// 绑定内容的真相仍由 `follow` 的帧给出，不由这个返回值给出。
func (c *Client) ForgeTrack(ctx context.Context, sessionID, url string) error {
	return c.Call(ctx, "forgePanel.track", forgeRef(sessionID, url), nil)
}

// ForgeDismiss 隐藏一个已绑定的变更请求（官方 `forgePanel/dismiss`）。
func (c *Client) ForgeDismiss(ctx context.Context, sessionID, url string) error {
	return c.Call(ctx, "forgePanel.dismiss", forgeRef(sessionID, url), nil)
}

// ── rc.2 归档清理 ────────────────────────────────────────────────────────

// ForgeWorktreeCleanup 读（revision 为 nil）或等（给定 revision）归档时的 checkout 清理。
//
// 官方结果允许 **null**：没有工作树服务、没有尝试过、或 revision 不是当前那一个。
// 所以这里返回指针——`nil` 与"status 是空串"是两件事，不能合并。
func (c *Client) ForgeWorktreeCleanup(ctx context.Context, sessionID string, revision *int64) (*ForgeWorktreeCleanup, error) {
	params := struct {
		SessionID string `json:"sessionId"`
		Revision  *int64 `json:"revision"`
	}{SessionID: sessionID, Revision: revision}
	var out *ForgeWorktreeCleanup
	if err := c.Call(ctx, "forgePanel.worktreeCleanup", params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ── 面板状态流（官方 mode:'stream' 的 `forgePanel.follow`）──────────────

// ForgePanelState 是官方 `ForgePanelState`（`OFF:…/lib/typert.host.js:1941-1942`）的投影。
//
// 只声明**本仓界面真的消费**的那几格：整帧七个键里，`directory` / `checkout` /
// `repository` / `readiness` / `changeRequests` / `local` 都另有单点方法或整块 HIDE，
// 这里不重复读（多解一份就多一处可能对不上的形状）。`worktree` 只取
// editable / blocker / creation 三样，其余子字段（request/created/failure/missing/
// restoration/waiting）宿主也会给真值，但本仓界面今天不消费（见 #64 账本行）。
type ForgePanelState struct {
	Directory string             `json:"directory"`
	Worktree  ForgePanelWorktree `json:"worktree"`
}

// ForgePanelWorktree 是官方 `ForgePanelWorktree`（`OFF:…/lib/typert.host.js:1945-1946`）的三格投影。
type ForgePanelWorktree struct {
	// Editable 是官方 `worktree.editable`：还能请求/撤回工作树。
	Editable bool `json:"editable"`
	// Blocker 是官方 `worktree.blocker`（三值联合或 null）。
	Blocker *string `json:"blocker"`
	// Creation 是官方 `worktree.creation`：此刻正在跑的那次创建，或 null。
	Creation *ForgeWorktreeCreation `json:"creation"`
}

// ForgeWorktreeCreation 是官方 `ForgeWorktreeCreation`（`OFF:…/src/types.ts:89-102`）。
type ForgeWorktreeCreation struct {
	// Phase 只有 prepare / checkout 两个官方词。
	Phase string `json:"phase"`
	// Branch 是这次尝试要创建的分支。
	Branch string `json:"branch"`
	// From 是基准分支；null = 基于 HEAD。
	From *string `json:"from"`
	// Held 表示有消息在等这次创建（只有这种创建可以停）。
	Held bool `json:"held"`
}

// ForgePanelFollow 订阅 `forgePanel.follow`：每来一帧完整 `ForgePanelState` 调一次 fn。
//
// 这是官方 `mode:'stream'` 方法（`OFF:…/lib/typert.host.js:936` 的语义原文：
// "the current state first, then each later state. Intermediate states may be
// skipped when the reader is slower than the updates."），宿主按 NDJSON 一行一帧写
// （`HOST:internal/transport/handlers_forge_panel_state.go:574-631`）。所以不能走
// `Call`（它把整个 body 读完才返回）——这里自己起请求、逐行解。
//
// 首帧一定是**当前完整状态**（宿主 `OpenOfficialForgePanelFollowStream` 的首帧无条件发），
// 所以 fn 至少被调用一次才可能返回 nil；`ctx` 取消即结束。
//
// 流里的错误帧（宿主在头已发出之后只能这样报错）解成四字段错误返回，
// 不静默当成"没有更多状态"。
func (c *Client) ForgePanelFollow(ctx context.Context, sessionID string, fn func(ForgePanelState) error) error {
	body, err := json.Marshal(forgeSessionOnly(sessionID))
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Base+"/api/"+MethodForgePanelFollow, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/x-ndjson")
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}
	// 长连接不能继承普通一元调用的整体超时；取消只由 ctx 决定。
	httpClient := *c.HTTP
	httpClient.Timeout = 0
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("连不上 %s：%w", c.Base, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var env envelope
		if json.Unmarshal(raw, &env) == nil && len(env.Error) > 0 {
			if apiErr := parseError(env.Error); apiErr != nil {
				return apiErr
			}
		}
		return fmt.Errorf("%s 返回 HTTP %d: %.200s", MethodForgePanelFollow, resp.StatusCode, raw)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/x-ndjson") {
		return fmt.Errorf("%s 必须返回 application/x-ndjson，拿到 %q", MethodForgePanelFollow, got)
	}
	sc := bufio.NewScanner(resp.Body)
	// 一帧状态是几百字节到几 KB；给足余量（大仓库的 changeRequests 会长一点）。
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// 错误帧：宿主在流里写 `{"error":{code,message}}`（头已经发出）。
		var errFrame struct {
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &errFrame); err != nil {
			return fmt.Errorf("forgePanel.follow 帧不是合法 JSON: %.200s", line)
		}
		if len(errFrame.Error) > 0 {
			if apiErr := parseError(errFrame.Error); apiErr != nil {
				return apiErr
			}
			return fmt.Errorf("forgePanel.follow 流里报告了错误: %.200s", line)
		}
		var state ForgePanelState
		if err := json.Unmarshal([]byte(line), &state); err != nil {
			return fmt.Errorf("forgePanel.follow 帧不是 ForgePanelState: %.200s", line)
		}
		if err := fn(state); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

// MethodForgePanelFollow 是 follow 的方法名（官方 `forgePanel.follow`）。
const MethodForgePanelFollow = "forgePanel.follow"

// forgeSessionOnly 是"只有 sessionId 一个参数"的官方形状。
func forgeSessionOnly(sessionID string) any {
	return struct {
		SessionID string `json:"sessionId"`
	}{SessionID: sessionID}
}

// forgeRef 是 track / dismiss 的官方形状 `{sessionId, url}`。
func forgeRef(sessionID, url string) any {
	return struct {
		SessionID string `json:"sessionId"`
		URL       string `json:"url"`
	}{SessionID: sessionID, URL: url}
}
