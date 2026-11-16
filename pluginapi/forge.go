// forge.go —— 官方 `dsh-experimental-client-ui-forge` 的 **A 档数据面与动作**（GUI 账本 #64a）。
//
// # 这一层为什么长这样
//
// 官方面板的全部状态来自一条流：`forgePanel.follow`（官方 `mode:'stream'`，
// 结果类型 `ForgePanelState`，见 `OFF:dsh-experimental-client-ui-forge/lib/typert.host.js:1752`）。
// 本仓宿主的 `forgePanel.follow` 流的是**绑定清单帧**
// （`{type:'baseline',value:{bindings}}` / `upsert` / `remove`，
// `HOST:internal/transport/handlers_forge_panel_state.go:300-345`），
// **不是** `ForgePanelState`——所以 checkout / worktree / local 这三块**没法**从
// follow 拿到（gap G6，已登记到 `../DSH-GO/docs/transport-contract.md`）。
//
// 于是 A 档的数据只能来自那些**有真实现、且形状对得上官方**的单点方法：
//
//	checkout（分支/提交/分离）  ← forgePanel.branches（current:null 即分离）
//	本地改动                    ← forgePanel.localDiff（四个 scope）
//	改动文件全文                ← forgePanel.localText
//	基准分支 / 分叉点           ← forgePanel.diffBases
//	本分支提交                  ← forgePanel.localCommits
//	工作树请求态                ← 本壳自己的乐观态（官方 WorktreeToggle 同样只有
//	                              `pending` 本地态，其余来自 follow）
//
// # 明确**没有**的东西（不要照着官方形状编）
//
//   - `worktree.creation`（创建进行到哪一步）、`worktree.created`、`worktree.missing`、
//     `worktree.restoration`、`worktree.editable` / `blocker`：宿主没有读出口。
//     官方这三块卡（WorktreeProgress / MissingCard）因此**整块不画**，见 #64 账本行。
//   - `changeRequests[]` 及一切 CR 内容：G1/G2/G3 未解，见 #64b。
package pluginapi

import "github.com/MYCF711/dsh-ui/internal/apiclient"

// 官方 `ForgeCheckoutState.state` 的四个取值。
const (
	// ForgeCheckoutPending 表示第一次 inspection 还没结束。
	ForgeCheckoutPending = "pending"
	// ForgeCheckoutNone 表示会话目录不在 Git checkout 里。
	ForgeCheckoutNone = "none"
	// ForgeCheckoutFailed 表示 git 跑不起来或报了错。
	ForgeCheckoutFailed = "failed"
	// ForgeCheckoutReady 表示拿到了 checkout 事实。
	ForgeCheckoutReady = "ready"
)

// ForgeCheckout 是官方 `ForgeCheckoutState` 的投影。
//
// Kind 取四个常量之一。**只有 Kind == ForgeCheckoutReady 时** Branch/Commit 才有
// 意义；Kind == ForgeCheckoutNone / Failed 时界面照官方画
// `checkout.none` / `checkout.failed` 那一句，不去猜分支。
type ForgeCheckout struct {
	Kind string
	// Branch 是当前检出的分支；分离 HEAD 时为空（官方是 null）。
	Branch string
	// Detached 报告 HEAD 是否不在分支上（官方 `checkout.branch === null`）。
	Detached bool
	// Commit 是 HEAD 指向提交的短名；分支还没有提交时为空。
	Commit string
	// Message 是 Kind == ForgeCheckoutFailed 时的原因（官方 `failed.message`）。
	Message string
}

// ForgeWorktree 是 `conversation.hero.options` 上那个勾选框需要的全部事实。
//
// 它**只覆盖"请求"这一层**：官方 WorktreeToggle 的 `requested` 也来自
// `state.worktree.request`（走的也是 follow），但"有没有请求"这件事本壳能自己
// 记住（请求由本壳发出、取消由本壳发出），所以这一格是**本壳的乐观态**，
// 与官方 `pending` 同一性质：成功回包前先照用户意图画，失败就退回并报错。
//
// ⚠ 官方那个勾选框在 `worktree.editable == false`（blocker: not-git /
// outside-writable-root / read-only）时**整块不画**。本仓没有 blocker 的读出口，
// 所以这里换成一个可证伪的判据：`Checkout.Kind == ready` 才画
// （官方同样要求 checkout ready）。这比"宿主说能建"弱，但它是真的：
// checkout 都读不到时请求一定建不出来。剩余差距（read-only / 外部可写根挡不住
// 请求）登记在账本行里。
type ForgeWorktree struct {
	// Requested 表示本壳发出过请求且用户没有撤回。
	Requested bool
	// From 是请求的基准分支；nil = 基于源 checkout 的 HEAD（官方 `from: null`）。
	From *string
	// Pending 表示有一次 worktree 动作还没回包（期间勾选框禁用，与官方 `pending` 同）。
	Pending bool
	// Err 是最近一次 worktree 动作的失败文本（空 = 没有失败）。
	Err string
}

// ForgeLocalSummary 是 `changes.local` 那张卡的全部数字（来自默认 `branch` 范围）。
//
// 官方卡片的数字是 `panel.local`（follow 给的 `ForgeLocalChanges`）；本仓默认视图
// 与卡片共用一次 `localDiff{kind:'branch'}` 的结果。`Known == false` 表示还没读到
// （或读失败）：卡片**不画**，不画成 0 个文件。
type ForgeLocalSummary struct {
	Known     bool
	Files     int
	Additions int
	Deletions int
	// Base 是宿主真的用来比对的 ref（官方 `changes.local.refs` 的 `{base}`）。
	Base string
}

// ForgeCommit 是 `localCommits` 的一项（官方 ForgeCommit 的投影）。
type ForgeCommit struct {
	SHA        string
	Title      string
	Author     string
	AuthoredAt string
}

// ForgeDiff 是 Changes tab（本地那一半）的数据面。
//
// Scope 是当前选中的官方 `ForgeLocalScope.kind`（`branch` / `uncommitted` /
// `unstaged` / `staged`）。Loading 期间 Files 保留上一次的结果（切范围时不闪空）。
type ForgeDiff struct {
	// SessionID 是这份数据所属的会话；与 Snapshot.Cur 不一致时界面**不画**
	// （壳换会话时旧数据还没被替换掉的那一帧）。
	SessionID string
	Scope     string
	// Base 是宿主回的分支范围比对基准；其余范围为空。
	Base  string
	Files []apiclient.ForgeDiffFile
	// Omitted 是被略掉的未跟踪文件数（官方 `diff.omitted`）。
	Omitted int
	// Commits 是 `localCommits` 的结果（只有 branch 范围会拉）。
	Commits   []ForgeCommit
	HeadSHA   string
	Truncated bool
	// Automatic 是 `diffBases.automatic` 的名字（官方 `diff.base.automatic`）。
	Automatic string
	// AutomaticWorktree 表示那个分叉点是本会话工作树的起点提交。
	AutomaticWorktree bool
	// Text / TextPath 是 `localText` 的结果（选中文件的新侧全文）。
	Text     string
	TextPath string
	Loading  bool
	// Err 是最近一次读的失败文本（空 = 没有失败）。界面照官方 `diff.error` 画。
	Err string
	// BasesErr 是 `diffBases` 的失败文本（它不该让整份 diff 变成错误）。
	BasesErr string
}

// ForgeView 是 #64a 的整份数据面；`Snapshot.Forge` 为 nil = 壳没接这条能力
// （老壳 / 测试 / 插件单独渲染），插件这时**一个字都不画**。
type ForgeView struct {
	// SessionID 是这份数据描述的那个会话。
	SessionID string
	Checkout  ForgeCheckout
	// Branches 是 checkout 的本地分支（官方 ForgeBranches.branches）。
	Branches []string
	// BranchesKnown 区分"还没读到"与"这个仓库确实没有分支"。
	BranchesKnown bool
	// BranchesErr 是 branches 读失败的原因（含 not-a-checkout）。
	BranchesErr string
	Worktree    ForgeWorktree
	Local       ForgeLocalSummary
	Diff        ForgeDiff
}

// ForgeActions 是 #64a 的动作面；`Actions.Forge` 为 nil = 壳没接。
//
// 读动作一律**异步**（后台拉 + Invalidate），因为插件的 Draw 在绘制线程上：
// 同步等 HTTP 会卡首帧。读结果下一帧从 `Snapshot.Forge` 里取。
type ForgeActions struct {
	// Refresh 重读 checkout / 分支 / 当前范围的 diff / 本地摘要。
	Refresh func()
	// LoadBranches 重读本地分支（hero 选项展开基准菜单时用）。
	LoadBranches func()
	// LoadScope 切到某个官方 scope 并读它的 diff（branch 范围同时读提交）。
	LoadScope func(scope string)
	// LoadBases 读基准分支清单与自动分叉点。
	LoadBases func(query string)
	// LoadText 读一个改动文件的新侧全文（选中文件时用）。
	LoadText func(path string)
	// RequestWorktree 请求"本会话在独立工作树里跑"；from 为 nil = 基于 HEAD。
	RequestWorktree func(from *string)
	// CancelWorktree 撤回还没建的工作树请求。
	CancelWorktree func()
}
