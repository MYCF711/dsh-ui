// forge_wiring.go —— #64a（官方 `dsh-experimental-client-ui-forge` 的 A 档）的壳侧数据面。
//
// # 数据从哪来（每条都指到宿主真实现）
//
//	checkout（分支 / 分离 HEAD）  ← forgePanel.branches      HOST:cmd/dshd/serve_forge_git.go:54
//	本地改动（文件/hunk/base）      ← forgePanel.localDiff     HOST:serve_forge_git.go:62
//	改动文件新侧全文                ← forgePanel.localText     HOST:serve_forge_git.go:74
//	自动分叉点与基准分支            ← forgePanel.diffBases     HOST:cmd/dshd/serve_forge_rc2.go:235
//	本分支提交                      ← forgePanel.localCommits  HOST:serve_forge_rc2.go:257
//	工作树请求                      ← forgePanel.requestWorktree / cancelWorktree
//	                                 HOST:cmd/dshd/serve_worktree_api.go:173
//	工作树可编辑/挡板/创建进度      ← forgePanel.follow 的 ForgePanelState
//	                                 HOST:cmd/dshd/serve_forge_state.go:239-300（dsh-go#84）
//
// 宿主自 dsh-go#84 起 `forgePanel.follow` 按官方形状发**整帧 ForgePanelState**
// （每行一帧），所以 `worktree.editable` / `blocker` / `creation` 有真读出口，
// 挡板与创建进度两张卡因此能画（见 `plugins/dsforge/hero.go` 与 `progress.go`）。
// 仍未消费的：`created` / `missing` / `restoration` / `failure` / `waiting`
// （MissingCard 与失败/恢复态整块不画，登记在账本 #64 行）。
//
// # 缓存与线程
//
// 插件在**绘制线程**上调读写，所以：
//
//	读  —— 同步返回缓存（`forgeFor`），首次/失效时**后台**拉一次，拉完 Invalidate；
//	写  —— 全部落地成 goroutine，回包后改缓存 + Invalidate。
//
// 每份结果回包时都核对 `session` 与 `generation`：换会话 / 换范围 / 又发了一次刷新
// 之后到的旧回包**直接丢掉**，不留"上一个会话的 diff 画在这个会话里"那一帧。
//
// # 默认范围是 branch（不是 uncommitted）
//
// 官方 changes-store 的首次 navigation 把 tab 初始化成
// `{scope:'branch', base:null, commits:null, lens:'all', hideTests:false,
// split:true, wrap:true}`（OFF:…/src/client/changes-store.ts:81-86）。同一份
// branch 范围也正是 `changes.local` 那张卡的输入（官方 `ForgeLocalChanges` 的注释：
// "Files the `branch` scope changes and the ref it compares with"），
// 所以本地摘要与默认视图共用**一次** localDiff，不重复打。
package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

const (
	// forgeReadTimeout 是一次 git 读的上限。大会话的分支范围 diff 可能很慢，
	// 但必须有界：宿主自己也带 deadline，客户端先超时会把"宿主还在跑"报成网络错。
	forgeReadTimeout = 25 * time.Second
	// forgeActionTimeout 是一次 worktree 动作的上限（`git worktree add` 会比读慢）。
	forgeActionTimeout = 60 * time.Second
	// forgeMaxBranches 是 `branches` 的上限（官方 `maxBranches?`；宿主默认 200，
	// 下拉菜单画不了那么多）。
	forgeMaxBranches = 40
	// forgeMaxDiffBytes 留给宿主默认（0 = 不传）。
	forgeMaxDiffBytes = 0
)

// forgeState 是一个会话的 forge A 档缓存。字段全部由 m.mu 保护（与壳其余状态同一把锁）。
type forgeState struct {
	// session 是这份缓存描述的会话；与 m.cur 不同即整份作废。
	session string
	// generation 每换一次会话 +1：旧回包据此丢弃（不等同于 scope/seq，
	// 它管的是"这一整个会话的数据"）。
	generation int

	checkout pluginapi.ForgeCheckout
	branches []string
	// branchesKnown 区分"还没读到"与"这个仓库确实没有分支"。
	branchesKnown   bool
	branchesErr     string
	branchesPending bool

	// scope 是 Changes tab 当前范围（官方四选一），默认 branch。
	scope       string
	diff        pluginapi.ForgeDiff
	diffPending bool
	// basesPending 表示 diffBases 在飞（自动分叉点名下一帧才能显示）。
	basesPending bool
	textPending  bool
	// texts / textErrs 是按文件路径缓存的新侧全文与读取失败原因（gap 展开用，
	// 官方 `Expansion.texts` 同形）。键存在 = 读到/失败；切范围时整表清空
	// （官方换一份 `ChangesData` 就把 `texts`/`open` 一起丢掉，`ChangesTab.tsx:773`）。
	texts    map[string]string
	textErrs map[string]string

	worktree pluginapi.ForgeWorktree
	// branchRead 表示当前这份 diff 是**成功读到的** branch 范围结果。
	// 本地摘要（`changes.local` 卡片）只认它：读到过且 0 个文件 = 真的没有改动。
	branchRead bool

	// followCancel 停掉当前会话的 `forgePanel.follow` 订阅（换会话时调）。
	// 每个会话只有一条流；它把整帧官方 ForgePanelState 折进 worktree 那三格。
	followCancel context.CancelFunc
	// followStarted 表示这条会话的 follow 订阅已经起过（不重复起）。
	followStarted bool

	// cleanup 是归档清理的最近一次读取（官方结果允许 null）。
	cleanup *apiclient.ForgeWorktreeCleanup
}

// forgeFor 返回当前会话的 forge 数据面（同步；nil = 壳没接客户端）。
//
// 它在 Snapshot 组装时被调用，因此**必须**是可以每帧调的：会话变了就地重置并触发
// 一次后台刷新；没有客户端（mockd / 测试）时返回 nil，插件据此整块不画。
func (m *model) forgeFor() *pluginapi.ForgeView {
	if m.cli == nil {
		return nil
	}
	m.mu.Lock()
	cur := m.cur
	if cur == "" {
		m.mu.Unlock()
		return nil
	}
	if m.forge.session != cur {
		// 换会话：整份作废（不把上一个会话的 diff 画到这一个里），并停掉旧会话的
		// follow 流——否则上一个会话的状态帧会继续往这份缓存里写。
		oldCancel := m.forge.followCancel
		generation := m.forge.generation + 1
		m.forge = forgeState{
			session:    cur,
			generation: generation,
			scope:      apiclient.ForgeScopeBranch,
			checkout:   pluginapi.ForgeCheckout{Kind: pluginapi.ForgeCheckoutPending},
		}
		m.forge.diff = pluginapi.ForgeDiff{SessionID: cur, Scope: apiclient.ForgeScopeBranch}
		m.mu.Unlock()
		if oldCancel != nil {
			oldCancel()
		}
		m.forgeRefresh(cur, generation, "")
		go m.forgeFollow(cur, generation)
		return m.forgeViewLocked(cur)
	}
	start := m.forge.generation
	needsBranches := !m.forge.branchesKnown && !m.forge.branchesPending && m.forge.branchesErr == ""
	needsBases := m.forge.diff.Automatic == "" && !m.forge.basesPending && m.forge.diff.BasesErr == ""
	m.mu.Unlock()
	if needsBranches {
		m.forgeLoadBranches(cur, start)
	}
	if needsBases {
		m.forgeLoadBases(cur, start, "")
	}
	return m.forgeViewLocked(cur)
}

// forgeViewLocked 把内部缓存投影成一份**不可变**快照交给插件。
func (m *model) forgeViewLocked(cur string) *pluginapi.ForgeView {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.forge.session != cur {
		return nil
	}
	view := &pluginapi.ForgeView{
		SessionID:     cur,
		Checkout:      m.forge.checkout,
		Branches:      append([]string(nil), m.forge.branches...),
		BranchesKnown: m.forge.branchesKnown,
		BranchesErr:   m.forge.branchesErr,
		Worktree:      m.forge.worktree,
		Diff:          m.forge.diff,
	}
	view.Diff.SessionID = cur
	view.Diff.Scope = m.forge.scope
	view.Diff.Loading = m.forge.diffPending
	// 快照必须是**不可变**的（插件在绘制线程读）：两张 map 每帧整份复制，
	// 不让后台的 `forgeLoadText` 与绘制线程共享同一个 map。
	view.Diff.Texts = cloneForgeTexts(m.forge.texts)
	view.Diff.TextErrors = cloneForgeTexts(m.forge.textErrs)
	view.Local = forgeLocalSummary(m.forge.diff, m.forge.branchRead)
	return view
}

// cloneForgeTexts 复制一张只读快照用的字符串表（nil 保持 nil）。
func cloneForgeTexts(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	out := make(map[string]string, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

// forgeLocalSummary 从 branch 范围的 diff 算出 `changes.local` 卡片的数字。
//
// 官方那张卡的输入是 `panel.local`（`ForgeLocalChanges`：branch 范围的文件与比对 ref），
// 与 Changes tab 的默认范围**同一份**。所以这里只在 `branchRead == true`（那次读
// 真的成功了）时给值：`Known=false` 表示还没读到，卡片不画，而不是画成"0 个文件"。
func forgeLocalSummary(diff pluginapi.ForgeDiff, branchRead bool) pluginapi.ForgeLocalSummary {
	if !branchRead || diff.Err != "" {
		return pluginapi.ForgeLocalSummary{}
	}
	out := pluginapi.ForgeLocalSummary{Known: true, Base: diff.Base, Files: len(diff.Files)}
	for _, f := range diff.Files {
		out.Additions += f.Additions
		out.Deletions += f.Deletions
	}
	return out
}

// ── 面板状态流（`forgePanel.follow`）─────────────────────────────────────

// forgeFollowRetry 是 follow 断线后的重连间隔。
//
// 宿主的 follow 是长连接（NDJSON 不关），断了只有两种可能：宿主重启 / 连接被
// 掐断。两种情况都该重连，但不能空转，所以按固定间隔；会话换掉或壳退出时
// ctx 取消，立刻返回。
const forgeFollowRetry = 2 * time.Second

// forgeFollow 订阅一个会话的官方 `ForgePanelState` 流，把每一帧折进界面态。
//
// 这是 `worktree.editable` / `blocker` / `creation` 的**唯一读出口**
// （HOST:cmd/dshd/serve_forge_state.go:239-300 的 `forgePanelHost.state`）。
// 每来一帧就叫一次 Invalidate：勾选框的挡板与创建进度卡下一帧即更新。
func (m *model) forgeFollow(sessionID string, generation int) {
	if m.cli == nil || sessionID == "" {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.mu.Lock()
	if m.forge.session != sessionID || m.forge.generation != generation || m.forge.followStarted {
		m.mu.Unlock()
		return
	}
	m.forge.followCancel = cancel
	m.forge.followStarted = true
	m.mu.Unlock()

	for {
		if ctx.Err() != nil {
			return
		}
		// 返回值只有"流结束/出错"两种；两种情况都按断线处理并重连（帧内容已经
		// 在回调里落地了）。第一帧是宿主的当前完整状态，所以断开前一定收到过值。
		err := m.cli.ForgePanelFollow(ctx, sessionID, func(state apiclient.ForgePanelState) error {
			m.applyForgePanel(sessionID, generation, state)
			return nil
		})
		if ctx.Err() != nil {
			return
		}
		if forgeFollowFatal(err) {
			// 老宿主没有这条流（not_implemented）/ 参数错：重连也不会好。
			// 停下来，界面按 `PanelKnown == false` 退回弱判据，不刷屏重试。
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(forgeFollowRetry):
		}
	}
}

// forgeFollowFatal 报告"重连也不会好"的错误：宿主没接这条流（not_implemented）
// 或参数错（bad_request）。
func forgeFollowFatal(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *apiclient.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Code == "not_implemented" || apiErr.Code == "bad_request"
}

// applyForgePanel 把一帧官方状态折进缓存（会话/世代对不上就丢掉）。
func (m *model) applyForgePanel(sessionID string, generation int, state apiclient.ForgePanelState) {
	m.mu.Lock()
	if m.forge.session != sessionID || m.forge.generation != generation {
		m.mu.Unlock()
		return
	}
	forgeWorktreePanel(state, &m.forge.worktree)
	m.mu.Unlock()
	m.inv.Invalidate()
}

// forgeWorktreePanel 把官方 `ForgePanelState` 的工作树三格折进界面态。
//
// 纯函数（不碰 model），所以"值不是常量"可以被单测直接钉住：
// 同一份 state 换成 `editable=false` / 换一个 creation.phase，输出必须跟着变。
func forgeWorktreePanel(state apiclient.ForgePanelState, out *pluginapi.ForgeWorktree) {
	if out == nil {
		return
	}
	out.PanelKnown = true
	out.Editable = state.Worktree.Editable
	out.Blocker = ""
	if state.Worktree.Blocker != nil {
		out.Blocker = *state.Worktree.Blocker
	}
	out.Creation = nil
	if creation := state.Worktree.Creation; creation != nil {
		view := pluginapi.ForgeWorktreeCreation{
			Phase:  creation.Phase,
			Branch: creation.Branch,
			Held:   creation.Held,
		}
		if creation.From != nil {
			from := *creation.From
			view.From = &from
		}
		out.Creation = &view
	}
}

// ── 后台读 ───────────────────────────────────────────────────────────────

// forgeRefresh 读一遍这个会话的 A 档全部只读数据（分支 / 摘要 / 当前范围 diff）。
//
// scope 为空 = 保留当前范围；非空 = 顺便切过去（"本地变更"卡片点进 Changes 时用）。
func (m *model) forgeRefresh(sessionID string, generation int, scope string) {
	if m.cli == nil || sessionID == "" {
		return
	}
	m.mu.Lock()
	if m.forge.session != sessionID || m.forge.generation != generation {
		m.mu.Unlock()
		return
	}
	if scope != "" {
		m.forge.scope = scope
		m.forge.diff.Scope = scope
		m.forge.diff.Files = nil
		m.forge.diff.Base = ""
		m.forge.diff.Omitted = 0
		m.forge.diff.Err = ""
		m.forge.diff.Commits = nil
		m.forge.diff.HeadSHA = ""
		m.forge.diff.Truncated = false
	}
	activeScope := m.forge.scope
	if activeScope == "" {
		activeScope = apiclient.ForgeScopeBranch
		m.forge.scope = activeScope
	}
	if activeScope != apiclient.ForgeScopeBranch {
		m.forge.branchRead = false
	}
	m.forge.diffPending = true
	m.forge.textPending = false
	m.forge.diff.Text = ""
	m.forge.diff.TextPath = ""
	// 重新读 diff = 换了一份数据：按路径的未变更行正文全部作废（官方
	// `ChangesTab.tsx:773` 在 `previous.data !== owner` 时把 texts/open 清空）。
	m.forge.texts = nil
	m.forge.textErrs = nil
	m.mu.Unlock()
	m.inv.Invalidate()

	go m.forgeLoadBranches(sessionID, generation)
	go m.forgeLoadScope(sessionID, generation, activeScope)
	go m.forgeLoadBases(sessionID, generation, "")
}

// forgeLoadBranches 读本地分支并顺手确定 checkout 状态。
func (m *model) forgeLoadBranches(sessionID string, generation int) {
	m.mu.Lock()
	if m.forge.session != sessionID || m.forge.generation != generation {
		m.mu.Unlock()
		return
	}
	m.forge.branchesPending = true
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), forgeReadTimeout)
	defer cancel()
	branches, err := m.cli.ForgeBranches(ctx, sessionID, forgeMaxBranches)

	m.mu.Lock()
	if m.forge.session != sessionID || m.forge.generation != generation {
		m.mu.Unlock()
		return
	}
	m.forge.branchesPending = false
	if err != nil {
		// 拿不到答案 = 不知道是不是 git 仓库。官方四态里 `none` 只在宿主明确说
		// "不是 checkout" 时用；别的失败一律 `failed`，消息原样留着。
		if apiErr := (*apiclient.APIError)(nil); errors.As(err, &apiErr) && apiErr.Code == "not_a_repository" {
			m.forge.checkout = pluginapi.ForgeCheckout{
				Kind:    pluginapi.ForgeCheckoutNone,
				Message: apiErr.Message,
			}
			m.forge.branches = nil
			m.forge.branchesKnown = true
			m.forge.branchesErr = apiErr.Message
		} else {
			m.forge.checkout = pluginapi.ForgeCheckout{
				Kind:    pluginapi.ForgeCheckoutFailed,
				Message: forgeErrText(err),
			}
			m.forge.branchesErr = forgeErrText(err)
		}
		m.mu.Unlock()
		m.inv.Invalidate()
		return
	}
	m.forge.branches = append([]string(nil), branches.Branches...)
	m.forge.branchesKnown = true
	m.forge.branchesErr = ""
	checkout := pluginapi.ForgeCheckout{Kind: pluginapi.ForgeCheckoutReady}
	if branches.Current == nil {
		// 官方的分离 HEAD：`checkout.branch === null`。
		checkout.Detached = true
	} else {
		checkout.Branch = *branches.Current
	}
	m.forge.checkout = checkout
	m.mu.Unlock()
	m.inv.Invalidate()
}

// forgeLoadScope 读一个范围的本地改动（branch 范围同时读提交）。
func (m *model) forgeLoadScope(sessionID string, generation int, scope string) {
	ctx, cancel := context.WithTimeout(context.Background(), forgeReadTimeout)
	defer cancel()
	diff, err := m.cli.ForgeLocalDiff(ctx, sessionID, apiclient.ForgeLocalScope{Kind: scope}, forgeMaxDiffBytes)

	m.mu.Lock()
	if m.forge.session != sessionID || m.forge.generation != generation || m.forge.scope != scope {
		// 回包时范围已经被切走了：丢掉（否则会把另一个范围的文件画在标题下）。
		m.mu.Unlock()
		return
	}
	m.forge.diffPending = false
	m.forge.diff.SessionID = sessionID
	m.forge.diff.Scope = scope
	if err != nil {
		if scope == apiclient.ForgeScopeBranch {
			m.forge.branchRead = false
		}
		m.forge.diff.Err = forgeErrText(err)
		m.forge.diff.Files = nil
		m.forge.diff.Omitted = 0
		m.forge.diff.Commits = nil
		m.mu.Unlock()
		m.inv.Invalidate()
		return
	}
	m.forge.diff.Err = ""
	if scope == apiclient.ForgeScopeBranch {
		m.forge.branchRead = true
	}
	m.forge.diff.Files = append([]apiclient.ForgeDiffFile(nil), diff.Files...)
	m.forge.diff.Omitted = diff.Omitted
	if diff.Base != nil {
		m.forge.diff.Base = *diff.Base
	} else {
		m.forge.diff.Base = ""
	}
	m.mu.Unlock()
	m.inv.Invalidate()

	if scope == apiclient.ForgeScopeBranch {
		m.forgeLoadCommits(sessionID, generation)
	}
}

// forgeLoadCommits 读 fork point 到 HEAD 的提交（只有 branch 范围显示）。
//
// base 传 nil：BaseMenu 尚未实现，当前视图没有选中分支，因此 diff 与提交列表
// 都明确使用自动分叉点；两者仍共用同一基准，不会出现各按一套的比较。
func (m *model) forgeLoadCommits(sessionID string, generation int) {
	ctx, cancel := context.WithTimeout(context.Background(), forgeReadTimeout)
	defer cancel()
	commits, err := m.cli.ForgeLocalCommits(ctx, sessionID, nil)

	m.mu.Lock()
	if m.forge.session != sessionID || m.forge.generation != generation || m.forge.scope != apiclient.ForgeScopeBranch {
		m.mu.Unlock()
		return
	}
	if err != nil {
		// 提交读不到不该让整份 diff 变成错误：官方把两者分开显示，这里也分开。
		m.forge.diff.Commits = nil
		m.forge.diff.HeadSHA = ""
		m.mu.Unlock()
		m.inv.Invalidate()
		return
	}
	out := make([]pluginapi.ForgeCommit, 0, len(commits.Commits))
	for _, c := range commits.Commits {
		commit := pluginapi.ForgeCommit{SHA: c.SHA, Title: c.Title, AuthoredAt: c.AuthoredAt}
		if c.Author != nil {
			commit.Author = *c.Author
		}
		out = append(out, commit)
	}
	m.forge.diff.Commits = out
	m.forge.diff.HeadSHA = commits.HeadSHA
	m.forge.diff.Truncated = commits.Truncated
	m.mu.Unlock()
	m.inv.Invalidate()
}

// forgeLoadBases 读基准分支清单与自动分叉点。
func (m *model) forgeLoadBases(sessionID string, generation int, query string) {
	ctx, cancel := context.WithTimeout(context.Background(), forgeReadTimeout)
	defer cancel()
	bases, err := m.cli.ForgeDiffBases(ctx, sessionID, query)

	m.mu.Lock()
	if m.forge.session != sessionID || m.forge.generation != generation {
		m.mu.Unlock()
		return
	}
	m.forge.basesPending = false
	if err != nil {
		m.forge.diff.BasesErr = forgeErrText(err)
		m.mu.Unlock()
		m.inv.Invalidate()
		return
	}
	m.forge.diff.BasesErr = ""
	if bases.Automatic != nil {
		m.forge.diff.Automatic = bases.Automatic.Name
		m.forge.diff.AutomaticWorktree = bases.Automatic.Worktree
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// forgeLoadText 读一个改动文件的新侧全文。
func (m *model) forgeLoadText(sessionID string, generation int, path string) {
	ctx, cancel := context.WithTimeout(context.Background(), forgeReadTimeout)
	defer cancel()
	text, err := m.cli.ForgeLocalText(ctx, sessionID, apiclient.ForgeLocalScope{Kind: m.forgeScopeOf(sessionID, generation)}, path)

	m.mu.Lock()
	if m.forge.session != sessionID || m.forge.generation != generation {
		m.mu.Unlock()
		return
	}
	m.forge.textPending = false
	if m.forge.texts == nil {
		m.forge.texts = map[string]string{}
	}
	if m.forge.textErrs == nil {
		m.forge.textErrs = map[string]string{}
	}
	if err != nil {
		// 失败必须留下**按路径**的原因：插件据此把 gap 退回可点态
		// （官方 `toast.expandFailed` 之后同样清 open/text，`ChangesTab.tsx:787-795`）。
		delete(m.forge.texts, path)
		m.forge.textErrs[path] = forgeErrText(err)
		m.forge.diff.Text = ""
		m.forge.diff.TextPath = ""
		m.mu.Unlock()
		m.inv.Invalidate()
		return
	}
	m.forge.texts[path] = text
	delete(m.forge.textErrs, path)
	m.forge.diff.Text = text
	m.forge.diff.TextPath = path
	m.mu.Unlock()
	m.inv.Invalidate()
}

// forgeScopeOf 读当前范围（调用方已确认会话）。
func (m *model) forgeScopeOf(sessionID string, generation int) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.forge.session != sessionID || m.forge.generation != generation || m.forge.scope == "" {
		return apiclient.ForgeScopeBranch
	}
	return m.forge.scope
}

// ── 动作（插件调的入口）──────────────────────────────────────────────────

// forgeRefreshAction 是插件"刷新"按钮：重读当前会话的一切。
func (m *model) forgeRefreshAction() {
	m.mu.Lock()
	sessionID, generation := m.forge.session, m.forge.generation
	m.mu.Unlock()
	if sessionID == "" || sessionID != m.cur {
		m.needForgeSession()
		return
	}
	m.forgeRefresh(sessionID, generation, "")
}

// needForgeSession 在还没有 forge 会话时先把会话建起来（插件在首帧就点了刷新）。
func (m *model) needForgeSession() {
	if m.forgeFor() == nil {
		m.setStatus("forge：当前没有可读的会话")
	}
}

// forgeScopeAction 切范围并读它。
func (m *model) forgeScopeAction(scope string) {
	switch scope {
	case apiclient.ForgeScopeBranch, apiclient.ForgeScopeUncommitted,
		apiclient.ForgeScopeUnstaged, apiclient.ForgeScopeStaged:
	default:
		// 未知范围不猜：静默忽略（官方 scope 是封闭联合）。
		return
	}
	m.mu.Lock()
	sessionID, generation := m.forge.session, m.forge.generation
	m.mu.Unlock()
	if sessionID == "" || sessionID != m.cur {
		return
	}
	m.mu.Lock()
	m.forge.scope = scope
	m.forge.diff.Scope = scope
	m.forge.diff.Files = nil
	m.forge.diff.Base = ""
	m.forge.diff.Omitted = 0
	m.forge.diff.Err = ""
	m.forge.diff.Commits = nil
	m.forge.diff.HeadSHA = ""
	m.forge.diff.Truncated = false
	// 在飞标记：插件据此不画"没有变更"（还没读到 ≠ 没有改动）。
	m.forge.diffPending = true
	m.forge.textPending = false
	// 切范围 = 换一份 diff：按路径的正文全部作废，别把上一个范围的行画到这个范围。
	m.forge.texts = nil
	m.forge.textErrs = nil
	m.mu.Unlock()
	m.inv.Invalidate()
	go m.forgeLoadScope(sessionID, generation, scope)
}

// forgeTextAction 读选中文件的新侧全文。
func (m *model) forgeTextAction(path string) {
	if path == "" {
		return
	}
	m.mu.Lock()
	sessionID, generation := m.forge.session, m.forge.generation
	m.forge.textPending = true
	// 重读之前先清掉这个路径上一次的失败：否则插件会在"新请求在飞"的这几帧里
	// 仍看到旧错误，把 gap 当成失败态反复退回（官方重试是一次干净的读）。
	if m.forge.textErrs != nil {
		delete(m.forge.textErrs, path)
	}
	m.mu.Unlock()
	go m.forgeLoadText(sessionID, generation, path)
}

// forgeBranchesAction 重读本地分支（基准菜单展开时用）。
func (m *model) forgeBranchesAction() {
	m.mu.Lock()
	sessionID, generation := m.forge.session, m.forge.generation
	m.forge.branchesErr = ""
	m.mu.Unlock()
	if sessionID == "" {
		return
	}
	go m.forgeLoadBranches(sessionID, generation)
}

// forgeBasesAction 重读基准分支清单（搜索框输入时用）。
func (m *model) forgeBasesAction(query string) {
	m.mu.Lock()
	sessionID, generation := m.forge.session, m.forge.generation
	m.forge.basesPending = true
	m.mu.Unlock()
	if sessionID == "" {
		return
	}
	go m.forgeLoadBases(sessionID, generation, query)
}

// forgeRequestWorktreeAction 请求工作树。
//
// **乐观态**：先照用户意图画（官方 WorktreeToggle 的 `pending` 也是本地态），回包
// 失败就退回并报状态栏——界面不会出现"勾上了但其实没发出去"。
func (m *model) forgeRequestWorktreeAction(from *string) {
	m.mu.Lock()
	sessionID := m.forge.session
	if sessionID == "" || m.forge.worktree.Pending {
		m.mu.Unlock()
		return
	}
	m.forge.worktree.Pending = true
	m.forge.worktree.Requested = true
	m.forge.worktree.From = from
	m.forge.worktree.Err = ""
	m.mu.Unlock()
	m.inv.Invalidate()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), forgeActionTimeout)
		defer cancel()
		err := m.cli.ForgeRequestWorktree(ctx, sessionID, from)
		m.mu.Lock()
		if m.forge.session != sessionID {
			m.mu.Unlock()
			return
		}
		m.forge.worktree.Pending = false
		if err != nil {
			// 回滚到"没有请求"：宿主没接受，界面就不能留着勾。
			m.forge.worktree.Requested = false
			m.forge.worktree.From = nil
			m.forge.worktree.Err = forgeErrText(err)
		}
		m.mu.Unlock()
		m.inv.Invalidate()
		if err != nil {
			m.setStatus("请求工作树失败: " + forgeErrText(err))
		}
	}()
}

// forgeCancelWorktreeAction 撤回工作树请求。
func (m *model) forgeCancelWorktreeAction() {
	m.mu.Lock()
	sessionID := m.forge.session
	if sessionID == "" || m.forge.worktree.Pending {
		m.mu.Unlock()
		return
	}
	m.forge.worktree.Pending = true
	m.mu.Unlock()
	m.inv.Invalidate()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), forgeActionTimeout)
		defer cancel()
		err := m.cli.ForgeCancelWorktree(ctx, sessionID)
		m.mu.Lock()
		if m.forge.session != sessionID {
			m.mu.Unlock()
			return
		}
		m.forge.worktree.Pending = false
		if err != nil {
			m.forge.worktree.Err = forgeErrText(err)
		} else {
			m.forge.worktree.Requested = false
			m.forge.worktree.From = nil
			m.forge.worktree.Err = ""
		}
		m.mu.Unlock()
		m.inv.Invalidate()
		if err != nil {
			m.setStatus("撤回工作树请求失败: " + forgeErrText(err))
		}
	}()
}

// forgeStopCreationAction 停掉一条消息正在等的那次创建（官方 `stopCreation`）。
//
// 与 cancel 不同：请求仍然开着、消息继续等（官方注释原文 "the message keeps
// waiting"），所以这里**不动** Requested，只把在飞标记立起来并在回包后清掉。
func (m *model) forgeStopCreationAction() {
	m.mu.Lock()
	sessionID := m.forge.session
	if sessionID == "" || sessionID != m.cur || m.cli == nil {
		m.mu.Unlock()
		return
	}
	m.forge.worktree.Pending = true
	m.mu.Unlock()
	m.inv.Invalidate()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), forgeActionTimeout)
		defer cancel()
		err := m.cli.ForgeStopCreation(ctx, sessionID)
		m.mu.Lock()
		if m.forge.session != sessionID {
			m.mu.Unlock()
			return
		}
		m.forge.worktree.Pending = false
		if err != nil {
			m.forge.worktree.Err = forgeErrText(err)
		} else {
			m.forge.worktree.Err = ""
		}
		m.mu.Unlock()
		m.inv.Invalidate()
		if err != nil {
			m.setStatus("停止创建工作树失败: " + forgeErrText(err))
		}
	}()
}

// forgeCleanupAction 读一次归档清理状态（官方 `forgePanel/worktreeCleanup`，revision 传 nil）。
//
// GUI 今天没有"归档会话"这条交互（宿主有 `workspace.archiveSession`，界面从未接），
// 所以官方那个 `toast.archiveCleanup*` 座位**没有投递点**：这里只保留读取能力，
// 供 `-keys`/测试与将来的归档入口用，界面不画它。
func (m *model) forgeCleanupAction() {
	m.mu.Lock()
	sessionID := m.forge.session
	m.mu.Unlock()
	if sessionID == "" || m.cli == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), forgeReadTimeout)
		defer cancel()
		cleanup, err := m.cli.ForgeWorktreeCleanup(ctx, sessionID, nil)
		if err != nil {
			m.setStatus("读归档清理状态失败: " + forgeErrText(err))
			return
		}
		m.mu.Lock()
		if m.forge.session == sessionID {
			m.forge.cleanup = cleanup
		}
		m.mu.Unlock()
		m.inv.Invalidate()
	}()
}

// forgeErrText 把客户端错误折成一句人话（**保留宿主原文**，不自己编解释）。
func forgeErrText(err error) string {
	if err == nil {
		return ""
	}
	var apiErr *apiclient.APIError
	if errors.As(err, &apiErr) {
		if msg := strings.TrimSpace(apiErr.Message); msg != "" {
			return msg
		}
		if apiErr.Code != "" {
			return apiErr.Code
		}
	}
	return err.Error()
}
