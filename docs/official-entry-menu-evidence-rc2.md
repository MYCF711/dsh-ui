# 三个区域的 More 菜单 + 面板固定态（rc.2 证据，#17 的两个真前置）

> 权威原文：官方 rc.2 随包 `src/**`（`OFF:` 前缀的锚点全部指向它）与安装态
> `dsh-web-frontend/dist/assets/index-ghOpC9uV.js`（图标 sprite）。
> 本文只写**已落地**的东西与**登记在案**的差异；#17 本体落地与三项 HIDE 见第 5 节。

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
| **菜单行本体（#17）** | `plugins/dsentrycustom/entry_menu.go` | 两个座位各一份 painter：左栏 = 未固定条目行（行首面板图标 + 行尾「固定」按钮）+「浏览更多插件」；会话头 = 折起 view 行（active 带行尾对勾）+「浏览更多插件」。行几何取 rc.2 产物 CSS，见 §3.1 |
| **面板导航服务** | `plugins/dssidebar/panelapi`（`ui.pluginNavigation`，ds-sidebar 提供 `OpenList()`） | 官方 `pluginNavigation` 客户端服务（`dsh-client-ui-plugin-manager/src/client/index.ts:153-166`）的等价物；服务不在册 → 「浏览更多插件」整行不画（官方 `browsable`） |
| **owner props 两格补充** | `pluginapi/entry_menu.go`（`EntryMenuEntry.Glyph`）、`plugins/dstopbar/entry_menu.go`（会话头 entries 的 `active`） | `Glyph` = 官方 `renderFactorySlot('sidebar.panel.icon')` 的投影（同一枚面板字形）；`active` = 官方 `active: viewTab.id === active?.id`（`ConversationSession.tsx:244-247`） |
| **图标** | `plugins/dsentrycustom/icons.go`（`pin-outline` / `plugin-pinwheel-outline` / 复用 `check-outline`） | 按 rc.2 sprite 原文转写，见 `official-icon-sprite-register.md` |

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

### 3.1 菜单行的几何（rc.2 产物原文，逐条可查）

| 项 | 官方原文 | 本仓落点 |
|---|---|---|
| 行 | `_item_70kcw_173`：`display:flex;align-items:center;gap:6px;width:100%;min-height:34px;padding:6px 8px;border-radius:--dsw-radius-md;font-size:13px;line-height:20px;color:label-primary`；`:hover` = `interactive-bg-hover` | `entryRowMinHeight/…Gap/…PadX/…PadY` + `entryFontSize/…LineHeight`，`Hover(ui.Look{}.Background(t.Hover))` |
| 行首图标 | `_itemIcon_70kcw_233{width:14px;height:14px}` + `_itemIcon svg{width:14px;height:14px}`（属性 size=16 被 CSS 夹到 14） | 图标框 14×14，字形/图标一律 `.Size(14,14)` |
| active 对勾 | `_check_70kcw_265{flex:none;color:label-primary}` + `.activeMark{flex:none;margin-left:auto}` | 会话头行尾 `check-outline` 14，`LabelPrimary` |
| 分离线 | `_separator_70kcw_157{height:.5px;margin:3px 8px;background:border-l2}` | `PadSides(3,8,3,8)` 里一条 **0.5px**（`entrySeparatorHeight`，与 `dscontextmenu` 同值） |
| 左栏行 | `.sidebarEntry .entryName{padding-right:28px}` / `.pinAnchor{position:absolute;right:6px;top:0;bottom:0}` / `.pinButton{width:22px;height:22px;padding:0;color:label-tertiary}`（`EntryMenu.module.css:5-16`）+ 圆角吃 primitives `._button_1ulci_2{border-radius:var(--dsw-radius-md)}` | 条目名 `PadSides(0,28,0,0)`；固定按钮 `Absolute().Right(6).Top(0).Bottom(0)` **盖在行内**（不是行外一格），22×22、`Radii.MD`；hover 只把图标色换成 `LabelPrimary`、背景保持透明 |
| 左栏行首图标 | `renderFactorySlot('sidebar.panel.icon', { size: 16, active })`（`EntryMenuContent.tsx:43-45`） | 面板字形（`EntryMenuEntry.Glyph`），与面板行**同一个**字形 |
| 条目名 | `.entryTitle{min-width:0;overflow-wrap:anywhere;white-space:normal}` / `.entryName{gap:8px;display:flex;align-items:center}`（`EntryMenu.module.css:1-2`） | `.MaxLines(0)` 换行（viva 的 `Text` 默认单行，需显式开段），**不挂** `Ellipsis`；`.entryName` 的 gap 8 在扁平结构里由 `margin-left:auto` 等价吸收 |

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
6. ~~左栏行的图标色用 `LabelDimmed`~~ **已修（2026-11-17）**：本仓**有**这一档令牌——`pluginapi.Theme.MenuIcon`（`pluginapi/theme.go:65`，注释即 `--dsw-alias-menu-icon`），`plugins/dscore` 明暗两套都给了值（`:184` / `:273`），`dscontextmenu` 早就在用。原「没有令牌」的判断不成立，三处菜单图标色已改为 `t.MenuIcon`。
7. ~~左栏行尾的对勾不画、固定按钮不做「悬停才显形」~~ **已修（2026-11-17）**：官方那一块是 `@media (hover:hover)` 里的一对**反向**规则（`EntryMenu.module.css:20-31`），现在逐条实现且有绘制级断言：

| 官方规则（行悬停态） | 数值 | 实现 |
|---|---|---|
| `.pinAnchor{opacity:0}`（:26） | 0 | `drawPinButton` 的 `anchor.Opacity(0)` |
| `.sidebarEntry:hover .pinAnchor{opacity:1}`（:28） | 1 | 同上，行或按钮被悬停时 |
| `.sidebarEntry .activeMark{opacity:1}`（:27，覆盖 :12 的 0） | 1 | `drawSidebarActiveMark` 静止值 |
| `.sidebarEntry:hover .activeMark{opacity:0}`（:30） | 0 | 同上，行或按钮被悬停时 |
| `.sidebarEntry .activeMark{position:absolute;right:9px;top:50%;transform:translateY(-50%)}`（:7-14） | right 9 / 14×14 | `.Absolute().Right(9).Top(0).Bottom(0)` + `Items(Center)` |
| `.pinButton:hover:not(:disabled){color:label-primary;background:0 0}`（:22-25） | label-primary / 透明 | `pinOutlineIcon(pinColor)` + `.Color(pinColor)`；**不铺底色** |
| `.sidebarEntry:hover>div:first-child>button{background:interactive-bg-hover}`（:21） | interactive-bg-hover | 指针在按钮上时整行底色同样点亮 |
| `Tooltip label={t('pinTooltip')} side="bottom"`（`EntryMenuContent.tsx:54`，文案 `locales.ts:14`） | 固定到左侧栏 | 按钮 `.Tooltip(zhPinTooltip).TooltipOn(ui.TooltipBottom)`（viva `ui/tooltip.go:12-45`） |
| `Button aria-label={t('pin',{name})}`（`EntryMenuContent.tsx:55`，文案 `locales.ts:15`） | 将{name}固定到侧边栏 | `Hit(pinID, zhPinPrefix+label+zhPinSuffix)` 的 label 就是无障碍名 |

两处判据写法（都由 `Canvas.Hovered()` 驱动，`gfx/canvas.go:1962` 只给最上面那一个命中身份）：
`:hover` 沿祖先链传播 ⇒ `Hovered()` 是**行**或**行尾按钮**任一即成立；`StateFrom` 只能跟一个来源，所以这一处与 `dsattachment` 的 `removeVisualActive`（`plugins/dsattachment/rail.go:1035-1041`）同法。
`pin-outline` 的尾 path 在 sprite 里是 `fill="currentColor"`，所以描边与填充层必须**同一个颜色变量**，否则悬停后只有描边变亮。会话头那一侧没有 `.sidebarEntry` 祖先，对勾恒画（`:12`/`:30` 都不适用），断言见 `TestConversationActiveMarkIgnoresHover`。

断言：`TestSidebarHoverSwapsPinAndActiveMark`（三态对翻）、`TestPinButtonHoverChangesIconColor`（换色 + 不铺底色 + 整行底色仍亮）、`TestSidebarActiveMarkGeometry`（right 9 / 居中）、`TestPinButtonKeepsOfficialTwoLineCopy`（可见浮条 + 无障碍名两层）、`TestConversationActiveMarkIgnoresHover`。
8. ~~固定按钮占位而非覆盖~~ **已修（2026-11-17）**：官方就是**绝对定位覆盖**——`.pinAnchor{position:absolute;right:6px;top:0;bottom:0}`，条目名靠 `.entryName{padding-right:28px}`（不是 38px）让位。本仓已改为 `ui.Row().Absolute().Right(6).Top(0).Bottom(0)` + 标签 `PadSides(0,28,0,0)`；viva 的绝对元素「painted after the in-flow children … wins pointer hits over them」（`viva/ui/absolute.go:11`），与官方 DOM 同序同几何。

## 5. #17 本体：落地了什么、HIDE 了什么（2026-11-15）

落地（`plugins/dsentrycustom`，逐条对 `EntryMenuContent.tsx`）：

| 官方行 | 做了什么 | 断言 |
|---|---|---|
| 未固定条目行（`entries.filter(!pinned)`，左栏） | 行首面板字形 + 标题 + 行尾 22×22「固定」按钮（`pin-outline` 14）；点了 `onDismiss()+onOpen(id)` / `onDismiss()+onPinnedChange(id,true)` | `TestSidebarRowsListOnlyUnpinnedEntries`、`TestSidebarRowClickOpensEntry`、`TestPinButtonPinsUnpinnedEntry` |
| 折起 view 行（`moreEntries = entries`，会话头） | 标题 + active 行尾对勾；无图标、无固定按钮 | `TestConversationRowsShowEveryFoldedViewWithActiveMark` |
| 「浏览更多插件」 | `plugin-pinwheel-outline` 行首；**服务在册才画**；点了 `onDismiss()+openList()` | `TestBrowsePluginsRowOpensList`、`TestBrowsePluginsRowHiddenWithoutNavigation` |
| 不在菜单里 | `Env.EntryMenu == nil` → 一行都不画 | `TestEntryRowsHiddenOutsideMenu` |
| 装配 | Builtin 里真的在册、两个座位都有占用方、导航能力可注入 | `cmd/app/entry_customization_assembly_test.go` |

Creator 已接（2026-11-17）：

- 官方调用是 `uiWorkspace.startSession(undefined, {agentPreset:'cordis', prompt, whenDraftExists:'append'})`
  （`OFF:src/client/index.ts:47-52`）。本仓 `pluginapi.Actions.StartSession` 接收同一组三字段；
  壳经真实 `session.create{mode:'cordis'}` 建会话（宿主写 `header.agentPreset`），打开后按官方
  新段落规则 append 草稿，**不自动 prompt**。区域提示词逐字取 `OFF:locales.ts:18-19`；
  创建失败保留原草稿并显示错误。断言：`TestCreatePluginRowStartsCordisDraftByRegion`、
  `TestStartSessionCreatesCordisAndAppendsDraftWithoutSending`、
  `TestStartSessionCreateFailureKeepsExistingDraft`。

HIDE 两项（都不自造）：

1. **右栏 guide 的 More 卡片不注册**：`sidebar.right.tab.guide.actions` 没有宿主
   （guide 面未落地，账本 #53），注册了也没有派发方。
2. **键盘模态那半边（`html[data-input-modality='keyboard'] … :focus-within`）不画**：
   它挂在同一串 CSS 里（`EntryMenu.module.css:29,31`），但①本仓没有"全局输入模态"这一位
   （官方是 `document.documentElement.dataset` 上的一个属性），②它是 `:focus-within`
   （行内**任一后代**有焦点），而 viva 只有 `FocusVisible()==id`（单个身份），
   两者不是同一个判据，不冒充。补的前提：viva 给出"行内焦点"或输入模态位。

## 6. 断言（防回退）

| 断言 | 位置 |
|---|---|
| 固定态七条语义 | `internal/entrypins/entrypins_test.go` |
| 菜单面：关着不画/没 drawSlot 不画/派发一次/props 派发后恢复/288 夹宽 | `internal/uikernel/entry_menu_test.go` |
| 左栏：无占用方无「更多」行；点开→派发+props 行真的画出来；未固定行离场；行尾按钮真的改状态 | `plugins/dssidebar/entry_menu_test.go` |
| 会话头：无占用方/单 view 无按钮；点开→派发+entries=moreTabs；未固定 view 离场并进菜单 | `plugins/dstopbar/entry_menu_test.go` |
| 内容行：不在菜单里不画 / 左栏只列未固定且行首有图标行尾有固定按钮 / 点行=onDismiss+onOpen / 点固定=onDismiss+onPinnedChange / 「浏览更多插件」服务在册才画且点了 openList / Creator 按区域启动 cordis append 草稿 / 会话头 active 多一枚对勾 / 四枚图标按 sprite 转写 | `plugins/dsentrycustom/entry_menu_test.go` |
| 装配：Builtin 在册 + 两个座位都有占用方 + `ui.pluginNavigation` 可注入可调用 | `cmd/app/entry_customization_assembly_test.go` |
| Creator：真实 `session.create{mode:'cordis'}` + 新段落 append + 不自动发送；失败保留原草稿 | `cmd/app/session_start_test.go` |
