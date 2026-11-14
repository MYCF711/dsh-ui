package uikernel

// official_slots.go —— 官方首屏结构树的**槽位表**（rc.2 口径）。
//
// 为什么要单独一张表：官方壳的「结构相同」不是一句形容词，它由一份可枚举的
// 槽声明组成——每个槽有名字、kind（single / list / keyed / chain / hoc /
// factory）、scope（root / session / session-maybe）、父槽和**官方源码锚点**。
// 以前的界面线只有 `pluginapi.Slot` 这个扁平字符串，插件挂得上，但没人能回答
// 「官方这个槽我们到底有没有对应节点」；本文件把这个问题变成可断言的数据。
//
// 三条边界（对齐 AGENTS.md §1 的分层）：
//
//	① 不新造一套槽类型：名字仍然是 pluginapi.Slot，已存在的同名官方常量
//	   （pluginapi.SlotRightbar / SlotShellBottom / SlotShellOverlay /
//	   SlotShellLeading）直接复用，这里只补官方有、我们还没有的那批名字。
//	② 本表**只描述结构**，不画任何东西；画在 shell_skeleton.go。
//	③ 锚点必须是 rc.2 `src/**`（rc.2 把 lib/client.js 压成 3–27 行 bundle，
//	   rc.1 行号全部失效，见 tmp/GO8-RC2/reports/H2-shell-tree-reanchor.md）。
//
// 表是**有序**的：父槽永远排在子槽前面，装配/校验/绘制都按这个顺序走。

import (
	"fmt"
	"sync"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// SlotKind 是官方槽的挂载语义（`dsh-client-ui-slots` 的类型契约）。
type SlotKind string

const (
	// KindSingle 一个实例；重复注册是装配错误（官方「注册在这里 = 整列替换」）。
	KindSingle SlotKind = "single"
	// KindList 多个实例，按 order 排序。
	KindList SlotKind = "list"
	// KindKeyed 按 key 取一（官方 `main` 按面板 id 取）。
	KindKeyed SlotKind = "keyed"
	// KindChain 链式合成（官方 composer 的 takeover 链）。
	KindChain SlotKind = "chain"
	// KindHOC 包装层（官方 `conversation.hero.frame`）。
	KindHOC SlotKind = "hoc"
	// KindFactory 组件工厂：宿主外观由调用方决定，自己声明子槽。
	KindFactory SlotKind = "factory"
)

// SlotScope 是官方槽的作用域：root 与会话无关，session 绑当前会话，
// session-maybe 允许「还没有选中会话」时挂载。
type SlotScope string

const (
	ScopeRoot         SlotScope = "root"
	ScopeSession      SlotScope = "session"
	ScopeSessionMaybe SlotScope = "session-maybe"
)

// SlotSpec 是官方结构树里的一个槽位。
type SlotSpec struct {
	// Name 是官方槽名字面量（`main` / `conversation.session.header.lineage` …）。
	Name pluginapi.Slot
	// Kind / Scope 是官方契约里的两个维度。
	Kind  SlotKind
	Scope SlotScope
	// Parent 是结构树上的父槽（root 自己为 ""）。
	Parent pluginapi.Slot
	// Host 是本仓**实现节点**的名字（首屏骨架里的容器节点，见 shell_skeleton.go）。
	// 断言「官方每个槽都有对应实现节点」就是在查这个字段能在骨架树里找到。
	Host string
	// Anchor 是官方 rc.2 源码锚点 `pkg/src/…:line`。不许留空、不许写 rc.1 行号。
	Anchor string
	// Note 记容易搞错的语义（空槽不占位、keyed 的 key 等）。
	Note string
}

// 官方根槽。这七个是 ui-layout 在运行时注册 `root` 时声明的 children
// （ui-layout/src/client/index.ts:192-206），也是 AppFrame 的全部座位。
const (
	// OffRoot 是运行时内置的根槽（AppFrame 挂在它上面）。
	OffRoot pluginapi.Slot = "root"
	// OffSidebar 是整条左栏：**注册在这里 = 整列替换**，不是追加。
	OffSidebar pluginapi.Slot = "sidebar"
	// OffMain 是按面板 id 取一的 keyed 槽，保留 key 固定为 conversation。
	OffMain pluginapi.Slot = "main"
)

// 复用的官方同名常量（pluginapi 里已有，不重复定义，避免出现两套字符串）：
//
//	pluginapi.SlotRightbar      = "rightbar"
//	pluginapi.SlotShellBottom   = "shell.bottom"
//	pluginapi.SlotShellOverlay  = "shell.overlay"
//	pluginapi.SlotShellLeading  = "shell.leading"

// 左栏内部座位（ui-sidebar/src/client/index.ts:94-104）。
const (
	OffSidebarBrandMark     pluginapi.Slot = "sidebar.brand.mark"
	OffSidebarBrandName     pluginapi.Slot = "sidebar.brand.name"
	OffSidebarToggleBadge   pluginapi.Slot = "sidebar.toggle.badge"
	OffSidebarPanelList     pluginapi.Slot = "sidebar.panellist"
	OffSidebarPanelActions  pluginapi.Slot = "sidebar.panellist.actions"
	OffSidebarWorkspaces    pluginapi.Slot = "sidebar.workspaces"
	OffSidebarSettings      pluginapi.Slot = "sidebar.settings"
	OffSidebarFooterActions pluginapi.Slot = "sidebar.footer.action"
)

// 主区 conversation 槽（ui-conversation/src/client/contract/slots.ts）。
const (
	OffMainConversation    pluginapi.Slot = "main.conversation"
	OffConversationHeader  pluginapi.Slot = "conversation.header"
	OffHeaderLeading       pluginapi.Slot = "conversation.header.leading"
	OffConversationSession pluginapi.Slot = "conversation.session"
	OffConvView            pluginapi.Slot = "conversation.view"
	OffConvContent         pluginapi.Slot = "conversation.content"
	OffConvComposer        pluginapi.Slot = "conversation.composer"
	OffConvComposerBar     pluginapi.Slot = "conversation.composer.bar"
	OffConvComposerDock    pluginapi.Slot = "conversation.composer.dock"
)

// Session header 的六个座位（conversation/src/client/contract/slots.ts:239-283）。
// rc.2 里 `.crumbSubagent` 类已删除（rc.1 有），面包屑只剩 lineage + 当前层级；
// 展开按钮走 corner，动作/工具区是 list，空时 `:empty{display:none}`。
const (
	OffSessionHeader           pluginapi.Slot = "conversation.session.header"
	OffSessionHeaderLineage    pluginapi.Slot = "conversation.session.header.lineage"
	OffSessionHeaderActions    pluginapi.Slot = "conversation.session.header.actions"
	OffSessionHeaderUtilities  pluginapi.Slot = "conversation.session.header.utilities"
	OffSessionHeaderCorner     pluginapi.Slot = "conversation.session.header.corner"
	OffSessionHeaderViewAction pluginapi.Slot = "conversation.session.header.views.actions"
)

// Composer 控件行与 dock（conversation/src/client/contract/slots.ts:305-362）。
const (
	OffInputDock        pluginapi.Slot = "conversation.input.dock"
	OffInputOverlay     pluginapi.Slot = "conversation.input.overlay"
	OffInputLeft        pluginapi.Slot = "conversation.input.left"
	OffInputRight       pluginapi.Slot = "conversation.input.right"
	OffInputActivity    pluginapi.Slot = "conversation.input.activity"
	OffInputAttachments pluginapi.Slot = "conversation.input.attachments"
	OffInputPlan        pluginapi.Slot = "conversation.input.plan"
	OffInputPermission  pluginapi.Slot = "conversation.input.permission"
	OffInputModel       pluginapi.Slot = "conversation.input.model"
)

// Hero（空白会话首屏）与文本/图片工厂槽。
const (
	OffHeroFrame          pluginapi.Slot = "conversation.hero.frame"
	OffHeroBrandMark      pluginapi.Slot = "conversation.hero.brand.mark"
	OffHeroWorkspace      pluginapi.Slot = "conversation.hero.workspace"
	OffHeroAgentPreset    pluginapi.Slot = "conversation.hero.agentPreset"
	OffHeroOptions        pluginapi.Slot = "conversation.hero.options"
	OffUserTextFactory    pluginapi.Slot = "conversation.user.text"
	OffUserTextNode       pluginapi.Slot = "conversation.user.text.node"
	OffImagesContentPlant pluginapi.Slot = "conversation.images.content"
)

// transcript 节点级座位：动作卡的两个官方槽（rc.2 的 ui-chat / ui-tool 子槽）。
//
// 官方出处（Kind / Scope / Parent 三个维度）：
//
//	conversation.chat.turnTail  list  / session —— ui-chat/src/client/contract/slots.ts:492
//	tool.call.toolview          keyed / session —— ui-tool/src/client/contract/slots.ts:23-27
//
// 两者的官方**父级**都是 `conversation.chat.node`（ui-chat 的节点工厂）：
// turnTail 是 `turn-tail` 节点的 child（ui-chat/src/client/chat/register-node-renderers.ts:93-97），
// toolview 是 `tool-call` 节点的 child（ui-tool/src/client/apply.ts:37-42）。
// 那个中间工厂槽不在首屏槽表里，所以按本表"挂最近的已登记祖先"的惯例
// 挂在 `conversation.session`（transcript 宿主）下。
//
// 本仓的界面侧占用方：`plugins/dsactioncards`（卡片框 + propose_action 过程行）。
const (
	// OffChatTurnTail 是答案下面的动作卡座位（官方 list/session）。
	OffChatTurnTail pluginapi.Slot = "conversation.chat.turnTail"
	// OffToolCallView 是工具调用行座位（官方 keyed/session，key 是工具 wire 名）。
	OffToolCallView pluginapi.Slot = "tool.call.toolview"
)

// 右栏 DockKit 座位（ui-sidebar-right/src/client/contract/slots.ts）。
const (
	OffRightbarSession   pluginapi.Slot = "rightbar.session"
	OffRightPaneTab      pluginapi.Slot = "sidebar.right.pane.tab"
	OffRightPaneTabTitle pluginapi.Slot = "sidebar.right.pane.tab.title"
	OffRightGuide        pluginapi.Slot = "sidebar.right.tab.guide"
	OffRightGuideActions pluginapi.Slot = "sidebar.right.tab.guide.actions"
	OffRightGuideEntry   pluginapi.Slot = "sidebar.right.tab.guide.entry"
	OffRightTabMenuItem  pluginapi.Slot = "sidebar.right.tab.menu.item"
)

// builtinOfficialSlots 是首屏结构树的**官方槽全集**（rc.2）。
//
// 顺序 = 结构树的自上而下顺序，父槽在前。锚点一律指 rc.2 随包 `src/**`：
//   - ui-layout：AppFrame 的七个 root children + root 自身
//   - ui-sidebar：左栏内部七个座位 + panellist 列表
//   - ui-conversation：conversation.* 全家 + 两个工厂
//   - ui-sidebar-right：右栏 session / pane tab / guide
var builtinOfficialSlots = []SlotSpec{
	// ── AppFrame 的根与七个 root children ──────────────────────────────
	{
		Name: OffRoot, Kind: KindSingle, Scope: ScopeRoot, Host: "AppFrame.frame",
		Anchor: "ui-layout/src/client/index.ts:193",
		Note:   "运行时内置根槽；AppFrame 注册在它上面，children 是下面七个",
	},
	{
		Name: OffSidebar, Kind: KindSingle, Scope: ScopeRoot, Parent: OffRoot, Host: "AppFrame.sidebarCol",
		Anchor: "ui-layout/src/client/index.ts:196",
		Note:   "整条左栏；注册在这里是整列替换，追加内容要挂到 ui-sidebar 声明的内层座位",
	},
	{
		Name: OffMain, Kind: KindKeyed, Scope: ScopeRoot, Parent: OffRoot, Host: "AppFrame.mainPanel",
		Anchor: "ui-layout/src/client/index.ts:197",
		Note:   "keyed 槽；保留 key 是 conversation",
	},
	{
		Name: pluginapi.SlotRightbar, Kind: KindSingle, Scope: ScopeRoot, Parent: OffRoot, Host: "AppFrame.rightbarCol",
		Anchor: "ui-layout/src/client/index.ts:198",
		Note:   "轨道宽度由 computeColumns 求解；占用方面板绝对锚定框架右缘",
	},
	{
		Name: pluginapi.SlotShellBottom, Kind: KindSingle, Scope: ScopeRoot, Parent: OffRoot, Host: "AppFrame.bottomRow",
		Anchor: "ui-layout/src/client/index.ts:199",
		Note:   "横跨三列；高度由占用方决定，空内容不预留（官方默认组合不占用）",
	},
	{
		Name: pluginapi.SlotShellOverlay, Kind: KindList, Scope: ScopeRoot, Parent: OffRoot, Host: "AppFrame.overlayLayer",
		Anchor: "ui-layout/src/client/index.ts:200",
		Note:   "整框浮层，pointer-events:none，子项恢复 auto",
	},
	{
		Name: pluginapi.SlotShellLeading, Kind: KindSingle, Scope: ScopeRoot, Parent: OffRoot, Host: "AppFrame.leadingSeat",
		Anchor: "ui-layout/src/client/index.ts:201",
		Note:   "窗口 chrome 左上座位；仅窄抽屉 / macOS 收起时挂载，Windows 不造同名座",
	},

	// ── 左栏内部座位（SidebarRoot 声明）──────────────────────────────
	{
		Name: OffSidebarBrandMark, Kind: KindSingle, Scope: ScopeRoot, Parent: OffSidebar, Host: "SidebarRoot.logoRow.mark",
		Anchor: "ui-sidebar/src/client/index.ts:97",
		Note:   "鱼标；mark 与 name 是两枚独立座位，外壳管基线/间距/高度",
	},
	{
		Name: OffSidebarBrandName, Kind: KindSingle, Scope: ScopeRoot, Parent: OffSidebar, Host: "SidebarRoot.logoRow.name",
		Anchor: "ui-sidebar/src/client/index.ts:98",
	},
	{
		Name: OffSidebarToggleBadge, Kind: KindSingle, Scope: ScopeRoot, Parent: OffSidebar, Host: "SidebarRoot.logoRow.toggleBadge",
		Anchor: "ui-sidebar/src/client/index.ts:99",
	},
	{
		Name: OffSidebarPanelList, Kind: KindList, Scope: ScopeRoot, Parent: OffSidebar, Host: "SidebarRoot.panelList",
		Anchor: "ui-sidebar/src/client/index.ts:90-91",
		Note:   "官方由 `sidebar.panel.icon` 工厂承载（每个入口一个图标行），这里按列表槽建模",
	},
	{
		Name: OffSidebarPanelActions, Kind: KindList, Scope: ScopeRoot, Parent: OffSidebar, Host: "SidebarRoot.panelListActions",
		Anchor: "ui-sidebar/src/client/index.ts:100",
	},
	{
		Name: OffSidebarWorkspaces, Kind: KindSingle, Scope: ScopeRoot, Parent: OffSidebar, Host: "SidebarRoot.regionArea",
		Anchor: "ui-sidebar/src/client/index.ts:101",
		Note:   "工作区树/会话列表都归这个占用方，不是 sidebar 自己硬编码",
	},
	{
		Name: OffSidebarSettings, Kind: KindSingle, Scope: ScopeRoot, Parent: OffSidebar, Host: "SidebarRoot.footArea.settings",
		Anchor: "ui-sidebar/src/client/index.ts:102",
	},
	{
		Name: OffSidebarFooterActions, Kind: KindList, Scope: ScopeRoot, Parent: OffSidebar, Host: "SidebarRoot.footArea.actions",
		Anchor: "ui-sidebar/src/client/index.ts:103",
		Note:   "官方默认可为空，不放固定「N 个会话」",
	},

	// ── 主区：conversation 外壳与 header ─────────────────────────────
	{
		Name: OffMainConversation, Kind: KindSingle, Scope: ScopeSessionMaybe, Parent: OffMain, Host: "ConversationRoot.root",
		Anchor: "ui-conversation/src/client/contract/slots.ts:229",
	},
	{
		Name: OffConversationHeader, Kind: KindSingle, Scope: ScopeSessionMaybe, Parent: OffMainConversation, Host: "ConversationRoot.header",
		Anchor: "ui-conversation/src/client/contract/slots.ts:237",
		Note:   "常驻导航容器；有 tabs 时 min-height 76，无 tabs 时 min-height 0 + padding-bottom 10",
	},
	{
		Name: OffHeaderLeading, Kind: KindSingle, Scope: ScopeRoot, Parent: OffConversationHeader, Host: "ConversationHeader.leading",
		Anchor: "ui-conversation/src/client/contract/slots.ts:266",
	},
	{
		Name: OffSessionHeader, Kind: KindSingle, Scope: ScopeSession, Parent: OffConversationHeader, Host: "ConversationHeader.titleRow",
		Anchor: "ui-conversation/src/client/contract/slots.ts:239",
	},
	{
		Name: OffSessionHeaderLineage, Kind: KindSingle, Scope: ScopeSession, Parent: OffSessionHeader, Host: "ConversationHeader.lineage",
		Anchor: "ui-conversation/src/client/contract/slots.ts:248",
		Note:   "rc.2 已无 `.crumbSubagent` 类：父子血缘只走这一枚 lineage + 当前层级 crumb",
	},
	{
		Name: OffSessionHeaderActions, Kind: KindList, Scope: ScopeSession, Parent: OffSessionHeader, Host: "ConversationHeader.actions",
		Anchor: "ui-conversation/src/client/contract/slots.ts:254",
		Note:   "标题旁动作，从右往左裁切后渐隐；gap 10px（rc.2，rc.1 是 8px）",
	},
	{
		Name: OffSessionHeaderUtilities, Kind: KindList, Scope: ScopeSession, Parent: OffSessionHeader, Host: "ConversationHeader.utilities",
		Anchor: "ui-conversation/src/client/contract/slots.ts:260",
		Note:   "工具区 `:empty{display:none}`；gap 8px + margin-left 20px",
	},
	{
		Name: OffSessionHeaderCorner, Kind: KindSingle, Scope: ScopeSession, Parent: OffSessionHeader, Host: "ConversationHeader.corner",
		Anchor: "ui-conversation/src/client/contract/slots.ts:277",
		Note:   "右栏展开按钮；margin-left 8px / margin-right -16px，空时不布局",
	},
	{
		Name: OffSessionHeaderViewAction, Kind: KindList, Scope: ScopeRoot, Parent: OffConversationHeader, Host: "ConversationHeader.tabs",
		Anchor: "ui-conversation/src/client/contract/slots.ts:283",
		Note:   "view tab 行的 More 菜单；有注册项才显示 More",
	},

	// ── 主区：content / session / view / composer ────────────────────
	{
		Name: OffConvContent, Kind: KindFactory, Scope: ScopeSessionMaybe, Parent: OffMainConversation, Host: "ConversationRoot.body",
		Anchor: "ui-conversation/src/client/contract/slots.ts:367",
		Note:   "工厂槽：子槽 conversation.session / conversation.composer / hero.* 由本表其余条目承载",
	},
	{
		Name: OffConversationSession, Kind: KindSingle, Scope: ScopeSession, Parent: OffConvContent, Host: "ChatView.frame",
		Anchor: "ui-conversation/src/client/contract/slots.ts:231",
		Note:   "transcript 宿主：滚动与 composer seat 共享外层 scrollBody",
	},
	{
		Name: OffConvView, Kind: KindList, Scope: ScopeSession, Parent: OffConversationSession, Host: "ChatView.column",
		Anchor: "ui-conversation/src/client/contract/slots.ts:285",
		Note:   "一次只渲染一个 view（Chat / Trajectory）；Trajectory 是主区 view 不是右栏",
	},
	{
		Name: OffConvComposer, Kind: KindChain, Scope: ScopeSession, Parent: OffConvContent, Host: "ConversationRoot.composerSeat",
		Anchor: "ui-conversation/src/client/contract/slots.ts:290",
		Note:   "composer 链；active 时 composerSeat z-index 9 + sticky bottom",
	},
	{
		Name: OffConvComposerBar, Kind: KindSingle, Scope: ScopeSessionMaybe, Parent: OffConvComposer, Host: "InputBar.root",
		Anchor: "ui-conversation/src/client/contract/slots.ts:336",
	},
	{
		Name: OffInputDock, Kind: KindList, Scope: ScopeSession, Parent: OffConvContent, Host: "InputBar.dockTop",
		Anchor: "ui-conversation/src/client/contract/slots.ts:305",
		Note:   "composer 卡片上方整行条目",
	},
	{
		Name: OffInputOverlay, Kind: KindList, Scope: ScopeSession, Parent: OffConvComposerBar, Host: "InputBar.overlayLayer",
		Anchor: "ui-conversation/src/client/contract/slots.ts:310",
	},
	{
		Name: OffInputLeft, Kind: KindList, Scope: ScopeSession, Parent: OffConvComposerBar, Host: "InputBar.row.left",
		Anchor: "ui-conversation/src/client/contract/slots.ts:320",
		Note:   "控件行左：命令 + modes(permission/plan) + input.left",
	},
	{
		Name: OffInputRight, Kind: KindList, Scope: ScopeSession, Parent: OffConvComposerBar, Host: "InputBar.row.right",
		Anchor: "ui-conversation/src/client/contract/slots.ts:325",
		Note:   "控件行右：input.right/model/activity/主按钮",
	},
	{
		Name: OffInputActivity, Kind: KindSingle, Scope: ScopeSession, Parent: OffConvComposerBar, Host: "InputBar.row.activity",
		Anchor: "ui-conversation/src/client/contract/slots.ts:331",
	},
	{
		Name: OffInputAttachments, Kind: KindSingle, Scope: ScopeSessionMaybe, Parent: OffConvComposerBar, Host: "InputBar.attachments",
		Anchor: "ui-conversation/src/client/contract/slots.ts:341",
		Note:   "附件 rail：空 rail 不占位（图片 64×64 / 文件卡 240×64）",
	},
	{
		Name: OffInputPlan, Kind: KindSingle, Scope: ScopeSession, Parent: OffConvComposerBar, Host: "InputBar.row.plan",
		Anchor: "ui-conversation/src/client/contract/slots.ts:350",
	},
	{
		Name: OffInputPermission, Kind: KindSingle, Scope: ScopeSession, Parent: OffConvComposerBar, Host: "InputBar.row.permission",
		Anchor: "ui-conversation/src/client/contract/slots.ts:355",
	},
	{
		Name: OffInputModel, Kind: KindSingle, Scope: ScopeSession, Parent: OffConvComposerBar, Host: "InputBar.row.model",
		Anchor: "ui-conversation/src/client/contract/slots.ts:362",
	},
	{
		Name: OffConvComposerDock, Kind: KindList, Scope: ScopeSession, Parent: OffConvComposer, Host: "InputBar.dock",
		Anchor: "ui-conversation/src/client/contract/slots.ts:315",
		Note:   "卡片下方环境条目（Activity/Usage/ContextMeter 不在这里，它们在 dock 内）",
	},

	// ── Hero（空白会话）─────────────────────────────────────────────
	{
		Name: OffHeroFrame, Kind: KindHOC, Scope: ScopeRoot, Parent: OffConvContent, Host: "Hero.frame",
		Anchor: "ui-conversation/src/client/contract/slots.ts:294",
		Note:   "hero 阶段的包装层；Hero 不被 brand-official 替换",
	},
	{
		Name: OffHeroBrandMark, Kind: KindSingle, Scope: ScopeRoot, Parent: OffConvContent, Host: "Hero.brandMark",
		Anchor: "ui-conversation/src/client/contract/slots.ts:296",
	},
	{
		Name: OffHeroWorkspace, Kind: KindSingle, Scope: ScopeRoot, Parent: OffConvContent, Host: "Hero.workspace",
		Anchor: "ui-conversation/src/client/contract/slots.ts:292",
	},
	{
		Name: OffHeroAgentPreset, Kind: KindSingle, Scope: ScopeSessionMaybe, Parent: OffConvContent, Host: "Hero.agentPreset",
		Anchor: "ui-conversation/src/client/contract/slots.ts:298",
	},
	{
		Name: OffHeroOptions, Kind: KindList, Scope: ScopeSession, Parent: OffConvContent, Host: "Hero.options",
		Anchor: "ui-conversation/src/client/contract/slots.ts:300",
	},

	// ── 文本/图片工厂槽 ─────────────────────────────────────────────
	{
		Name: OffUserTextFactory, Kind: KindFactory, Scope: ScopeSession, Parent: OffConversationSession, Host: "ChatView.flow",
		Anchor: "ui-conversation/src/client/contract/slots.ts:202",
		Note:   "工厂：解析一段原文并把它自己的 keyed 子槽渲染成业务节点",
	},
	{
		Name: OffUserTextNode, Kind: KindKeyed, Scope: ScopeSession, Parent: OffUserTextFactory, Host: "ChatView.flow.markdown",
		Anchor: "ui-conversation/src/client/contract/slots.ts:221",
	},
	{
		Name: OffImagesContentPlant, Kind: KindFactory, Scope: ScopeRoot, Parent: OffConversationSession, Host: "ChatView.flow.images",
		Anchor: "ui-conversation/src/client/contract/slots.ts:211",
	},

	// ── transcript 节点级座位（动作卡 / 工具视图）───────────────────
	{
		Name: OffChatTurnTail, Kind: KindList, Scope: ScopeSession, Parent: OffConversationSession,
		Host:   "ChatView.flow.turnTail",
		Anchor: "ui-chat/src/client/contract/slots.ts:492",
		Note: "list/session；官方声明原文 `'conversation.chat.turnTail': { kind: 'list'; scope: 'session'; owner: TurnTailOwnerProps }`。" +
			"官方父级是 `conversation.chat.node` 的 `turn-tail` 节点（ui-chat/src/client/chat/register-node-renderers.ts:93-97 的 children 声明），" +
			"该中间工厂槽不在首屏槽表，故挂最近的已登记祖先 conversation.session。" +
			"空槽不占位：官方 entries 为空时整块返回 null（OFF:dsh-client-ui-action-cards/src/client/Card.tsx:87-92）",
	},
	{
		Name: OffToolCallView, Kind: KindKeyed, Scope: ScopeSession, Parent: OffConversationSession,
		Host:   "ChatView.flow.toolview",
		Anchor: "ui-tool/src/client/contract/slots.ts:23",
		Note: "keyed/session；官方声明原文 `'tool.call.toolview': { kind: 'keyed'; scope: 'session'; owner: ToolCallOwnerProps }`（ui-tool/src/client/contract/slots.ts:23-27）。" +
			"key 是工具 wire 名（占用方用 `key: 'propose_action'`，OFF:dsh-client-ui-action-cards/src/client/mount.ts:60-62），" +
			"未占用的 key 用通用行。官方父级是 `conversation.chat.node` 的 `tool-call` 节点（ui-tool/src/client/apply.ts:37-42 children 声明），" +
			"该中间工厂槽不在首屏槽表，故挂 conversation.session",
	},

	// ── 右栏 DockKit ───────────────────────────────────────────────
	{
		Name: OffRightbarSession, Kind: KindSingle, Scope: ScopeSession, Parent: pluginapi.SlotRightbar, Host: "SidebarRight.panel",
		Anchor: "ui-sidebar-right/src/client/contract/slots.ts:43",
		Note:   "每会话独立 surface；后台已初始化的 view 保活隐藏",
	},
	{
		Name: OffRightPaneTab, Kind: KindKeyed, Scope: ScopeSession, Parent: OffRightbarSession, Host: "SidebarRight.tabStrip",
		Anchor: "ui-sidebar-right/src/client/contract/slots.ts:59",
	},
	{
		Name: OffRightPaneTabTitle, Kind: KindKeyed, Scope: ScopeSession, Parent: OffRightbarSession, Host: "SidebarRight.tabStrip.title",
		Anchor: "ui-sidebar-right/src/client/contract/slots.ts:73",
	},
	{
		Name: OffRightGuide, Kind: KindChain, Scope: ScopeSession, Parent: OffRightbarSession, Host: "SidebarRight.guide",
		Anchor: "ui-sidebar-right/src/client/contract/slots.ts:84",
		Note:   "只有一个可注册页时直接播种该页，否则显示 guide",
	},
	{
		Name: OffRightGuideActions, Kind: KindList, Scope: ScopeRoot, Parent: OffRightGuide, Host: "SidebarRight.guide.actions",
		Anchor: "ui-sidebar-right/src/client/contract/slots.ts:91",
	},
	{
		Name: OffRightGuideEntry, Kind: KindKeyed, Scope: ScopeSession, Parent: OffRightGuide, Host: "SidebarRight.guide.entry",
		Anchor: "ui-sidebar-right/src/client/contract/slots.ts:93",
	},
	{
		Name: OffRightTabMenuItem, Kind: KindList, Scope: ScopeSession, Parent: OffRightPaneTab, Host: "SidebarRight.tabMenu",
		Anchor: "ui-sidebar-right/src/client/contract/slots.ts:105",
	},
}

// officialSlotMu 保护运行期追加（rc.2 新插件的槽由后续任务用 RegisterOfficialSlot 加）。
var (
	officialSlotMu    sync.RWMutex
	officialSlots     = append([]SlotSpec(nil), builtinOfficialSlots...)
	officialSlotIndex = buildSlotIndex(officialSlots)
)

func buildSlotIndex(specs []SlotSpec) map[pluginapi.Slot]SlotSpec {
	out := make(map[pluginapi.Slot]SlotSpec, len(specs))
	for _, spec := range specs {
		out[spec.Name] = spec
	}
	return out
}

// OfficialSlots 返回官方首屏槽表的只读副本（父槽在前，顺序稳定）。
func OfficialSlots() []SlotSpec {
	officialSlotMu.RLock()
	defer officialSlotMu.RUnlock()
	return append([]SlotSpec(nil), officialSlots...)
}

// LookupOfficialSlot 按官方槽名查声明；没有返回 false（不是错误：官方还有
// 本表之外的包自己的槽，这类槽由各自的后续任务登记）。
func LookupOfficialSlot(name pluginapi.Slot) (SlotSpec, bool) {
	officialSlotMu.RLock()
	defer officialSlotMu.RUnlock()
	spec, ok := officialSlotIndex[name]
	return spec, ok
}

// RegisterOfficialSlot 追加一条官方槽声明（rc.2 新增插件的槽靠这个进来）。
//
// 三条 fail-loud（与 uikernel.Load 的「不静默」同源）：
//   - 名字为空 / 重复 → 错误
//   - kind 或 scope 不认识 → 错误
//   - 父槽既不是根槽、也没登记过 → 错误（挂不到一棵不存在的树上）
//
// 返回的错误要原样交给装配方，不许在调用点吞掉：一个挂不上的槽在界面上
// 表现为「这块什么都不显示」，那是最难查的一类 bug。
func RegisterOfficialSlot(spec SlotSpec) error {
	if spec.Name == "" {
		return fmt.Errorf("uikernel: 官方槽声明没有名字")
	}
	if spec.Anchor == "" {
		return fmt.Errorf("uikernel: 官方槽 %q 没有 rc.2 源码锚点", spec.Name)
	}
	if !validSlotKind(spec.Kind) {
		return fmt.Errorf("uikernel: 官方槽 %q 的 kind %q 不认识", spec.Name, spec.Kind)
	}
	if !validSlotScope(spec.Scope) {
		return fmt.Errorf("uikernel: 官方槽 %q 的 scope %q 不认识", spec.Name, spec.Scope)
	}

	officialSlotMu.Lock()
	defer officialSlotMu.Unlock()
	if _, dup := officialSlotIndex[spec.Name]; dup {
		return fmt.Errorf("uikernel: 官方槽 %q 重复登记", spec.Name)
	}
	if spec.Parent != "" {
		if _, ok := officialSlotIndex[spec.Parent]; !ok {
			return fmt.Errorf("uikernel: 官方槽 %q 的父槽 %q 没有登记", spec.Name, spec.Parent)
		}
	}
	officialSlots = append(officialSlots, spec)
	officialSlotIndex[spec.Name] = spec
	return nil
}

func validSlotKind(kind SlotKind) bool {
	switch kind {
	case KindSingle, KindList, KindKeyed, KindChain, KindHOC, KindFactory:
		return true
	}
	return false
}

func validSlotScope(scope SlotScope) bool {
	switch scope {
	case ScopeRoot, ScopeSession, ScopeSessionMaybe:
		return true
	}
	return false
}
