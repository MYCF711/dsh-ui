# 三个区域的 More 菜单 + 面板固定态（rc.2 证据，#17 的两个真前置）

> 权威原文：官方 rc.2 随包 `src/**`（`OFF:` 前缀的锚点全部指向它）与安装态
> `dsh-web-frontend/dist/assets/index-ghOpC9uV.js`（图标 sprite）。
> 本文只写**已落地**的东西与**登记在案**的差异；没落地的部分是第 5 节。

## 1. 官方形状（逐处）

`dsh-client-ui-entry-customization` 不是独立可见面：它往**三个区域**各注册一份
菜单行，三个座位都是 list 槽，owner props 是同一组四个字段。

| 区域 | 座位 | 官方宿主（菜单那层） | 锚点 | 入口图标 |
|---|---|---|---|---|
| 左栏面板列表 | `sidebar.panellist.actions` | `SidebarRoot.tsx:107-133` `PanelMoreMenu` | `button.panelRow`（与面板行同几何，展开态同底色 `.panelRow[aria-expanded=true]`） | `apps-add-outline` 16/18 |
| 会话头 view 行 | `conversation.session.header.views.actions` | `ConversationSession.tsx:27-45` `ViewsMoreMenu` | `Button.size=sm` + `.viewsMore{22×22;margin-top:-3}` | `ellipsis-outline` 13 |
| 右栏 guide | `sidebar.right.tab.guide.actions` | `GuideBody.tsx:99-117` `GuideMoreMenu` | 一张 guide 卡片（`.entry` + `GuideMoreIcon`） | 本包私有 `GuideMoreIcon` |

owner props（三处逐字段相同，`ui-sidebar/src/client/contract/slots.ts:219` 等）：

```
entries: {id, label, pinned, active}[]   onOpen(id)   onPinnedChange(id, pinned)   onDismiss()
```

菜单面几何（可引用数值）：

- `.moreMenu{width:288px;max-width:calc(100vw - 24px)}`（`SidebarRoot.module.css:726-729`；
  会话头 `.viewsMoreMenu` 同值，`ConversationRoot.module.css:320-323`）
- 贴锚点 `calc(anchor(bottom) + 4px)`、窗口内留 12（dist CSS 的 `menu-below` /
  `menu-above` 规则）
- 面的内边距/圆角/底色/阴影与官方 composer 菜单面同一档
  （`ui-model-selection/.../ModelSelect.module.css:107-135`，本仓已在
  `internal/uikernel/official_model_menu.go` 落过同一套 token）

## 2. 本仓落点

| 东西 | 文件 | 说明 |
|---|---|---|
| 菜单面 + 座位派发 | `internal/uikernel/entry_menu.go` | `DrawEntryMoreMenu`：贴锚点 / 放不下翻面 / 288 夹宽 / 整窗遮罩关闭 / dispatch 期间挂 `Env.EntryMenu`、派发后恢复 |
| owner props 契约 | `pluginapi/entry_menu.go` | `EntryMenuProps` / `EntryMenuEntry` / `EntryRegion*` / `KeyEntryPins` / `EntryPins`；`Env.EntryMenu` 字段在 `pluginapi/plugin.go` |
| 固定态（纯内存） | `internal/entrypins/entrypins.go` | 官方三份 `*-pins.ts` 的等价物；壳在 `cmd/app/wave3_wiring.go` 的 `provide()` 里按 `ui.entryPins` 提供 |
| 左栏宿主 | `plugins/dssidebar/entry_menu.go` | More 行（`更多`）+ 菜单面 + 行尾取消固定按钮；可见行按固定态过滤 |
| 会话头宿主 | `plugins/dstopbar/entry_menu.go` | `viewsMore` 按钮（22×22 / 13px）+ 菜单面；tab 行按官方 `preferred` + 宽度预算切 |
| 图标 | `plugins/dssidebar/icons.go`（`pin-fill`）、`plugins/dstopbar/icons.go`（`ellipsis-outline`） | 按 rc.2 sprite 原文转写，见 `official-icon-sprite-register.md` |

固定态语义（`panel-pins.ts:24-31` 逐行）：

| 官方 | 本仓 | 用例 |
|---|---|---|
| 初值 `unpinned = []`（全部固定） | `New()` 空表 | `TestPanelsKeepUnpinnedInMemoryAndInOrder` |
| `pinned === current.includes(id)` 直接返回（不重发） | `SetPinned` 返回 false、不改表 | `TestPanelsIgnoreFixedAndRepeatedChoices` |
| 固定 = 从表里删掉（保持其余顺序） | `current` 过滤 | 同上 |
| 取消固定 = 追加到表尾 | `append(current, id)` | 同上 |
| 固定条目不受理（`plugins` / `chat` / `files:*`） | `fixedInRegion` | `TestPanelsIgnoreFixedAndRepeatedChoices`、`TestViewPinsUseFixedViewChat`、`TestGuidePinsUseFixedKind` |
| 已注销的 id 留在表里 | 不清理 | `TestPanelsKeepIdsOfUnregisteredEntries` |
| 没有持久化（reload 即丢） | 只存在进程内存；新实例空表 | `TestPanelsKeepUnpinnedInMemoryAndInOrder` 末段 |

## 3. 可见性（宿主按固定态做的两件事）

- 左栏：`panels.filter(id === FIXED || !hasEntryActions || !unpinned.includes(id))`
  （`SidebarRoot.tsx:404-405`）→ `visiblePanelRoutes`；固定行 `plugins` 恒在，
  且**不进** More 的 entries（`:419-421`）。
- 会话头：`preferred = [固定 view] + [未固定的其余 view]`，
  `visibleViewLimit = max(1, floor((rowWidth - 48) / 120))`，
  `visible = preferred[:limit]`、`more = tabs - visible`（`ConversationSession.tsx:127-166`）。
  本仓行宽取 `env.MainWidth - 20 - 28`（官方 header 的左右内边距）。

## 4. 登记在案的差异（都不是"差不多就行"，每条都写清原因）

1. **左栏 More 行的图标不画**：`apps-add-outline` 由 3 个 `<rect rx=1>` + 1 条路径组成，
   圆角矩形需要椭圆弧，viva 的路径词汇只有 M/L/Q/C/Z（`gfx.PathCommandKind`），
   `plugins/dsassets` 对这类素材一律 `Unsupported`（同 `copy` / `branch`）。
   按仓规不画近似形状 → 这一行只有「更多」文案。
2. **行尾取消固定按钮恒显示**：官方 `.panelUnpin{opacity:0}` 只在 `.panelEntry:hover`
   淡入（`SidebarRoot.module.css:660-671`）；viva 的 `Box` 没有"本帧是否悬停"的读口，
   所以常显 caption 色（不冒充 hover 态）。
3. **行尾按钮占位而非覆盖**：官方 `.panelUnpin` 绝对定位在行右端（行 `padding-right:38px`）；
   本仓是同一行里 `[行(flex:1)] [按钮 28×28]`，行内容窄 28px。
4. **会话头 More 按钮与最后一个 tab 的间距 = `tabsGap`(20)**：官方 `.viewsMore` 与
   `.viewTabs` 是两个兄弟容器（间距由 `.tabs` 的 flex 排布决定，没有单一可引数值）。
5. **右栏 guide 的 More 卡片没有宿主**：本仓还没有 guide 面（`sidebar.right.tab.guide`
   无占用方，右栏是 dockkit tab 条 + 面板体），所以这一处 More **按 HIDE 登记**，
   不自造一枚"更多工具"卡片挂在别处。

## 5. 还差什么（#17 本身）

三个宿主的**入口**只在座位有占用方时出现（官方 `hasEntryActions`），今天没有占用方，
所以界面上看不到它们——这是官方行为，不是缺功能。要真正落地 #17，还需要它自己的内容插件：

1. 往三个座位注册菜单行：未固定条目的行 + 行尾「固定」按钮（`IconPinOutlineRegular` 14）、
   「浏览更多插件」（`pluginNavigation` 等价物 = 选中 `plugins` 面板）、
   「创建插件」（`IconPlusOutlineRegular` 16）；
2. 「创建插件」要走 `uiWorkspace.startSession(undefined, {agentPreset:'cordis', …})`
   ——**宿主今天不认 `agentPreset`**：`internal/transport/server.go` 的
   `handleSessionCreate` 只解 `Mode`，`opt.Create` 签名是 `func(mode string)`（DSH-GO 仓）。
   这一条不在 GUI 仓范围内，落地前 #17 的 Creator 行必须 HIDE 或等宿主补参数。

## 6. 断言（防回退）

| 断言 | 位置 |
|---|---|
| 固定态七条语义 | `internal/entrypins/entrypins_test.go` |
| 菜单面：关着不画/没 drawSlot 不画/派发一次/props 派发后恢复/288 夹宽 | `internal/uikernel/entry_menu_test.go` |
| 左栏：无占用方无「更多」行；点开→派发+props 行真的画出来；未固定行离场；行尾按钮真的改状态 | `plugins/dssidebar/entry_menu_test.go` |
| 会话头：无占用方/单 view 无按钮；点开→派发+entries=moreTabs；未固定 view 离场并进菜单 | `plugins/dstopbar/entry_menu_test.go` |
