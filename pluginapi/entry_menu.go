package pluginapi

// entry_menu.go —— 官方三个「区域 More 菜单」座位与它外面的那层固定态（rc.2）。
//
// # 官方结构
//
// `dsh-client-ui-entry-customization` 往**三个区域**各注册一份菜单行：
//
//	sidebar.panellist.actions                 左栏面板列表的 More
//	conversation.session.header.views.actions 会话头 view tab 行的 More
//	sidebar.right.tab.guide.actions           右栏 guide 的 More 卡片
//	OFF:dsh-client-ui-entry-customization/src/client/index.ts:60-72
//
// 三个座位都是 **list** 槽，owner props 是同一组四个字段
// （OFF:dsh-client-ui-sidebar/src/client/contract/slots.ts:219 的
// `SidebarPanelActionsOwnerProps`，另两处 `ConversationViewActionsOwnerProps` /
// `SidebarRightGuideActionsOwnerProps` 逐字段相同）：
//
//	entries       本区域**全部**条目的 `{id, label, pinned, active}`
//	onOpen        选中一条（区域宿主自己的"切过去"）
//	onPinnedChange 把一条固定/取消固定
//	onDismiss     关掉这层菜单
//
// 宿主一侧（region owner）还负责三件事，本仓同样如此：
//
//  1. **入口只在座位有占用方时出现**（官方 `hasEntryActions`：
//     `slots.entriesOfSlot(slot).length > 0`，OFF:ui-sidebar/src/client/index.ts:96-98）
//  2. 未被固定的条目从区域里**隐藏**、改由 More 菜单列出（官方
//     `SidebarRoot.tsx:404-410` 的 `panels.filter(... !unpinned.includes(id))`）
//  3. 固定态**只存在内存里**，reload 即丢（官方 `panel-pins.ts:9-33` 没有任何
//     持久化通道——`EntryPins` 因此也不提供 Set 之外的任何写路径）
//
// # 本仓的形状
//
// 本仓的插件拿不到"owner props"这种每帧入参：`Painter.Draw` 只有 `*Env`。
// 所以宿主在派发那三个座位之前，把 owner props 挂到 `Env.EntryMenu` 上，
// **派发完必须恢复原值**（与 `Env.ViewportOffset` 同一约定：写者负责恢复）。
// 占用方读 `env.EntryMenu` 画自己的行；它不存在时说明本帧不在菜单里。
//
// 谁写：三个区域宿主（左栏 `plugins/dssidebar`、会话头 `plugins/dstopbar`、
// 右栏 guide——右栏 guide 面本仓尚未落地，见 `docs/official-parity.md` #53 的缺口）。
// 谁读：贡献菜单行的插件（`dsh-client-ui-entry-customization` 的等价物，即 #17）。

// 三个区域的 region 身份（官方 `EntryRegion`：`'sidebar' | 'conversation' | 'rightbar'`，
// OFF:dsh-client-ui-entry-customization/src/client/EntryMenuContent.tsx:7）。
const (
	// EntryRegionSidebar 是左栏面板列表。
	EntryRegionSidebar = "sidebar"
	// EntryRegionConversation 是会话头的 view tab 行。
	EntryRegionConversation = "conversation"
	// EntryRegionRightbar 是右栏 guide。
	EntryRegionRightbar = "rightbar"
)

// KeyEntryPins 是固定态服务键。值是 EntryPins。
//
// 为什么做成服务而不是每个宿主各自存一份：官方这三处固定态分别由三个**区域
// 包**持有（ui-sidebar 的 panel-pins / ui-conversation 的 view-pins /
// ui-sidebar-right 的 guide-pins），但写它的是同一个菜单内容插件——菜单里那条
// "固定"要改的就是区域宿主的显示。本仓把三份内存表放在壳里提供，宿主读、
// 菜单内容写，两边看到同一份值。
const KeyEntryPins = "ui.entryPins"

// EntryPins 是三个区域的**纯内存**固定态（官方三个 `*-pins.ts` 的等价物）。
//
// 语义逐条对齐官方（`panel-pins.ts:24-31` / `view-pins.ts:24-31` /
// `guide-pins.ts:44-51`），三条都有用例钉住：
//
//	① 只有"未固定"的 id 被记下来；固定 = 从表里删掉，取消固定 = 追加到表尾
//	② 表里**没有的 id 就是固定**（默认值）；已不存在的 id 留在表里不清理
//	③ 固定 id（左栏 `plugins`、会话头 `chat`、右栏 `files:*`）不受理
type EntryPins interface {
	// Pinned 报告某个条目是否处于固定态（不在未固定表里 = 固定）。
	Pinned(region, id string) bool
	// Unpinned 返回某个区域当前未固定的 id（顺序 = 取消固定的先后）。
	Unpinned(region string) []string
	// SetPinned 设置固定态；返回是否真的改了（重复设置同一值不算改）。
	SetPinned(region, id string, pinned bool) bool
}

// EntryMenuEntry 是官方 owner props 里的一条条目（`{id, label, pinned, active}`，
// OFF:dsh-client-ui-sidebar/src/client/SidebarRoot.tsx:404-410 的 entries 投影）。
type EntryMenuEntry struct {
	// ID 是区域内的条目身份（面板 id / view id / guide 的 `kind:id`）。
	ID string
	// Label 是已本地化的显示名（区域宿主给，菜单不查字典）。
	Label string
	// Glyph 是条目的图标字形，只给**左栏**菜单行用。
	//
	// 官方那一格不是 owner props，而是渲染工厂槽
	// `renderFactorySlot('sidebar.panel.icon', { id, size: 16, active })`
	// （OFF:dsh-client-ui-entry-customization/src/client/EntryMenuContent.tsx:43-45），
	// 由 ui-sidebar 按面板 id 出一枚 16px 图标。本仓的面板行本来就用一个字形
	// （`dssidebar` 的 `panelRoute.Glyph`，同一枚字形画在面板行上），所以这里把它
	// 投影进 owner props——菜单行与面板行因此永远是**同一个图标**，不会两处各画一套。
	//
	// 空 = 没有图标（会话头区域官方本来就不给这一格，那一侧恒为空）。
	Glyph string
	// Pinned 表示这条当前是固定态（= 没进未固定表）。
	Pinned bool
	// Active 表示这条是区域里当前选中的那一条。
	Active bool
}

// EntryMenuProps 是三个 `*.actions` 座位的 owner props（本仓经 `Env.EntryMenu` 传）。
type EntryMenuProps struct {
	// Region 是区域身份，取 EntryRegionSidebar / EntryRegionConversation /
	// EntryRegionRightbar。官方用它选 Creator 的提示词与分析属性。
	Region string
	// Entries 是**本区域全部**条目（含已固定的）：官方菜单自己只画未固定的那些
	// （`EntryMenuContent` 的 `moreEntries = entries.filter(entry => !entry.pinned)`），
	// 所以这里给全量，筛不筛由占用方按官方那一行决定。
	Entries []EntryMenuEntry
	// Open 是"选中一条"（官方 onOpen）。nil = 宿主没接。
	Open func(id string)
	// SetPinned 是"固定/取消固定一条"（官方 onPinnedChange）。nil = 宿主没接
	// （这时不该画固定按钮——画一个点了没反应的按钮比不画更糟）。
	SetPinned func(id string, pinned bool)
	// Dismiss 是"关掉这层菜单"（官方 onDismiss）。nil = 宿主没接。
	Dismiss func()
}
