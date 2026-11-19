// Package dsforge 把官方 `dsh-experimental-client-ui-forge` 的 **A 档**（本地 git 与
// 工作树请求）画进本仓界面（GUI 账本 #64a）。
//
// # 它占哪几个座位（逐条对应官方注册）
//
//	conversation.hero.options      ← WorktreeToggle（勾选"新建工作树" + blocker 挡板）
//	                                  OFF:…/src/client/mount.ts:389-391
//	conversation.input.dock        ← 官方这一格是 WorktreeProgress（order 5）；本仓
//	                                  同一槽里有两张：创建进度卡（order 4，官方那
//	                                  张，progress.go）+ 本地变更卡（order 5，
//	                                  LocalChangesCard 的搬家件，理由见 dock.go 头）
//	                                  OFF:…/src/client/mount.ts:392-394
//	sidebar.right.pane.tab(.title) ← Changes 视图（本地那一半）
//	                                  OFF:…/src/client/mount.ts:364-369
//
// **不占**的座位与原因（都登记在账本 #64 行）：
//
//	conversation.session.header.utilities  官方那个按钮打开的是变更请求面板（HIDE）
//	shell.overlay                          ActionToast 的座位；本仓没有统一的 toast 座位
//	sidebar.session.row.trailing/.hover    会话行的变更请求字形（G1）
//
// # 它凭什么敢画
//
// 三个座位的数据全部来自**宿主真实现**（逐条见 `internal/apiclient/forge.go` 文件头），
// 没有一样来自猜测：`Forge == nil` 就一个字都不画（老壳 / 无会话 / 测试）。
package dsforge

import (
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 右侧变更视图的 tab 类型 key。
//
// 官方是 `CHANGES_ID = '@deepseek-ai/dsh-experimental-client-ui-forge/changes'`
// （`OFF:…/src/client/changes-address.ts:22`），kind 是 `forge-changes`。本仓的 tab
// key 是**注册表主键**（`uikernel.RightTabs` 按它去重、按它派发），用官方那个完整
// 包名当 key 既不会撞别的部件，也让人一眼看出它对应哪个官方包。
const changesTabKey = "@deepseek-ai/dsh-experimental-client-ui-forge/changes"

// changesTabLabel 是官方 `diff.tab` 的中文（locales.ts:198）。
const changesTabLabel = "变更"

// Plugin 是 forge 面板的 A 档。
type Plugin struct {
	theme *pluginapi.Theme
	// open 是本视图自己的界面态（官方放在 changes-store 的 per-tab 桶里）：
	// 文件卡展开、已读标记、范围菜单是否展开，以及 #64 ②③ 的树宽/对比/换行。
	//
	// 已读标记**落盘**（官方 viewed-files.ts 存 `dsh.forge-viewed.v1`，本仓经宿主
	// `uiState.*` 存同一份形状，见 viewed_store.go）；其余仍在内存。
	open changesOpen
	// viewedSynced 记"哪些 (会话, diff 源) 已经从宿主读过标记"——每格只读一次，
	// 避免每帧都问一遍。
	viewedSynced map[string]bool
	// viewedSource 是本帧在画的 diff 源 key（标记写回时用同一个）。
	viewedSource string
	// viewedDiff 是本帧在画的 diff（标记写回时取文件名与 digest 的集合）。
	viewedDiff *pluginapi.ForgeDiff
	// viewedPaths 是本帧 diff 里的路径集合：写回时丢掉已不在 diff 里的标记
	// （官方 viewed-files.ts:87-104 的 `present` 判据）。
	viewedPaths map[string]bool
	// changesWidth 是上一帧量到的变更视图正文宽度：官方 `@container (min-width:760px)`
	// 的容器宽度，immediate mode 下由 AfterDraw 从 root.Rect() 量回（见 changes_tree.go）。
	changesWidth float32
	// frame 是本帧 Draw 交给 AfterDraw 的几何（只在同一帧内有效）。
	frame changesFrame
	// diffTargets 是左右两列的横滚视口命中 id → (会话, 文件, 列) 的映射，
	// HandleScroll 据此把横向手势分给正确的那一列。
	diffTargets map[gfx.ID]diffScrollTarget
	// splitter 是本帧文件树分栏的状态；HandleKey 把方向键/Home/End 交给它
	// （viva SplitterState.Key 的约定：OFF:viva@v0.6.5/ui/splitter.go:37-61）。
	splitter *ui.SplitterState
}

func (*Plugin) ID() string { return "ds-forge" }

func (*Plugin) Provides() []string { return nil }

func (*Plugin) Requires() []string { return []string{pluginapi.KeyTheme} }

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.open = newChangesOpen()
	return nil
}

// Panels 注册三个座位，顺序与官方注册顺序无关（三个槽互不相交）。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{
			// 官方 `conversation.hero.options` id 'forge-panel.worktree'，无 order。
			ID: "forge-worktree-option", Slot: uikernel.OffHeroOptions,
			Draw: p.drawWorktreeToggle,
		},
		{
			// 官方 `conversation.input.dock` id 'forge-panel.worktree' **order 5**
			// （mount.ts:392-394）上的官方卡片就是创建进度卡。order 5 已被本地变更卡
			// 占着（本仓历史，见 dock.go），所以进度卡取 4 = 排在它之上；官方在该槽
			// 只有一张卡，没有"本地卡在第几位"的证据，这条相对顺序已登记在账本行。
			ID: "forge-creation-progress", Slot: uikernel.OffInputDock, Order: 4,
			Draw: p.drawWorktreeProgress,
		},
		{
			// 同槽的本地变更卡（官方在 ForgeTab 里，本仓搬到这里，见 dock.go 文件头）。
			ID: "forge-local-card", Slot: uikernel.OffInputDock, Order: 5,
			Draw: p.drawLocalCard,
		},
		{
			// 官方 `sidebar.right.pane.tab` key = CHANGES_ID（mount.ts:364-366）。
			//
			// Order 1（不是 0）：官方 tab 类型**没有** order 字段，名录顺序 = 注册顺序；
			// 本仓把 Order 当那个注册顺序用，而 0 已经被 ds-browser 占着
			// （`TestNoTwoPanelsShareOneSlotAndOrder` 必须互不相同）。取 1 = 排在浏览器之后。
			ID: "forge-changes-tab", Slot: pluginapi.SlotRightPaneTab, Key: changesTabKey,
			Label: changesTabLabel, Order: 1, Draw: p.drawChangesBody,
			// 左右对比正文是在 canvas 上画的（每列自己的 BeginScrollArea），
			// 必须等布局解完拿到 slot 矩形才能画：viva 的绘制顺序由布局树决定，
			// 在 Draw 里直接画会被后画的节点盖住（pluginapi.Painter.AfterDraw 的约定）。
			AfterDraw: p.afterDrawChanges,
		},
		{
			// 同 key 的标题槽（mount.ts:367-369），Order 与体一致。
			ID: "forge-changes-tab-title", Slot: pluginapi.SlotRightPaneTabTitle, Key: changesTabKey,
			Order: 1, Draw: p.drawChangesTitle,
		},
	}
}

// forgeView 取本帧的 A 档数据面；nil = 不该画任何东西。
//
// 三条判据都与官方同一语义：
//
//	Forge == nil                  壳没接这条能力（老壳/测试）→ 整块不画
//	Forge.SessionID != Data.Cur   这份数据属于别的会话（换会话那一帧）→ 不画，
//	                              免得把上一个会话的改动画进这个会话
//	Diff.SessionID != Data.Cur    同上，单独再判一次（diff 是另一路异步读）
func forgeView(env *pluginapi.Env) *pluginapi.ForgeView {
	if env == nil || env.Data.Forge == nil {
		return nil
	}
	view := env.Data.Forge
	if view.SessionID == "" || view.SessionID != env.Data.Cur {
		return nil
	}
	return view
}

// forgeDiffOf 取本帧可画的 diff（会话对不上就是 nil）。
func forgeDiffOf(env *pluginapi.Env) *pluginapi.ForgeDiff {
	view := forgeView(env)
	if view == nil || view.Diff.SessionID != view.SessionID {
		return nil
	}
	return &view.Diff
}

// themeOf 取本帧主题（壳每帧都会给，测试里有兜底）。
func (p *Plugin) themeOf(env *pluginapi.Env) *pluginapi.Theme {
	if env != nil && env.Theme != nil {
		return env.Theme
	}
	return p.theme
}
