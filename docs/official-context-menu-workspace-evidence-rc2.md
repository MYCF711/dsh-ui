# #10 context-menu 与 #62 workspace（rc.2 结构证据与落地台账）

- 日期：2026-11-14
- 分支：`feat/gui-ctx-workspace`（基于 `origin/main` `0d2cd8f`）
- 官方基线：`tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`（0.2.2-rc.2 随包 `src/**`）
- 上一轮审计把 #10 判 **HIDE**（`docs/official-ui-structure-evidence-rc2.md:41,96-108` 的理由是「GUI 里没有任何插件注册 `contextMenu.button` 的贡献；官方那两个消费方也没取证」）。本轮从官方 `dsh-client-ui-workspace/src/client/rows/Rows.tsx` 与
  `session-actions/context-menus.tsx` 取到消费方与文案后，**该 HIDE 结论作废**。

## 1. #10 `dsh-client-ui-context-menu`（11 文件 / 749 行）

### 1.1 官方结构（逐条锚点）

| 官方事实 | 锚点 |
|---|---|
| `ctx.contextMenus` 服务：active / registeredKinds / register / open / resolve / close / notify | `OFF:ui-context-menu/src/client/index.ts:98-134` |
| `contextMenu.button` 工厂槽；**目标 kind 没有贡献者时整枚按钮不渲染** | `OFF:ui-context-menu/src/client/index.ts:36-40` |
| `contextMenu.icon` keyed/root/inline 槽 | `OFF:ui-context-menu/src/client/index.ts:40` |
| `contextMenu.content` keyed/root 槽 | `OFF:ui-context-menu/src/client/index.ts:42` |
| `shell.overlay` 上的共享 `MenuOverlay` + 操作失败 Toast | `OFF:ui-context-menu/src/client/index.ts:170-181` |
| 注册表：同 target 内 id 去重、order 升序、卸载即撤回 | `OFF:ui-context-menu/src/client/registry.ts:30-47` |
| 右键 / Shift+F10 触发；已有 handler 可 preventDefault 取消 | `OFF:ui-context-menu/src/client/trigger.ts:11-43` |
| 菜单行：label / danger / disabled / separatorBefore / commandId 快捷键 / icon / 子菜单 | `OFF:ui-context-menu/src/client/View.tsx:16-33` |

官方消费方（本轮取证）：

| 消费形态 | 锚点 |
|---|---|
| workspace 行整行右键（`getTriggerProps`） | `OFF:ui-workspace/src/client/rows/Rows.tsx:298-325` |
| workspace 行尾 `contextMenu.button` | `OFF:ui-workspace/src/client/rows/Rows.tsx:327-334` |
| session 行尾 `contextMenu.button` | `OFF:ui-workspace/src/client/rows/Rows.tsx:746-758` |
| session 菜单内建动作（pin/rename/fork/copy id/archive） | `OFF:ui-workspace/src/client/session-actions/context-menus.tsx:18-69` |

### 1.2 本仓落地

| 层 | 落点 |
|---|---|
| 契约 | `pluginapi/context_menu.go`：`ContextMenus` / `ContextMenuContribution` / `ContextMenuAction` / `ContextMenuTarget`；服务键 `ui.contextMenus` |
| 服务 | `plugins/dscontextmenu/context_menu.go`：按 target kind 注册、同 kind 内 id 去重、order+id 排序、反注册、`HasTarget` |
| 渲染 | `plugins/dscontextmenu` 的 Overlay（右键 / Shift+F10 / 指针点开 + 省略号按钮锚定菜单）；按官方 primitives CSS 定制，不采用 Viva 内建菜单的 32px 行/16px 图标/ShadowMD 近似值 |
| 按钮 | `ContextMenus.Button`：只有 `HasTarget(kind)` 才创建 20×20 命中区，对齐官方「无贡献者不渲染」 |
| 结构账 | `plugins/dscontextmenu/official_slots.go` 用 `RegisterOfficialSlot` 登记 `contextMenu.button`（factory/root）、`contextMenu.icon`（keyed/root）、`contextMenu.content`（keyed/root），父槽 `root`，锚点 `index.ts:37/40/42` |
| 消费方 | `plugins/dssidebar`：workspace 组头右键 + 行尾省略号（`Rows.tsx:325/330` 的等价座位）；session 行尾省略号（`Rows.tsx:752`） |

### 1.3 已知差异（不冒充官方）

1. **`contextMenu.icon` 已复用官方 sprite**：`plugins/dsassets.ContextMenuIcon` 覆盖当前生产项的 `workspace.rename/delete/copy` 与 `file-actions.download/save`；官方映射见 `workspace/src/client/index.ts:317-320`、`file-actions/src/client/index.ts:33-34`。`copy` 的 rect 与 `save` 的 arc 由 `viva icons` 编译，未知 key 不画替代形状。
2. **`contextMenu.content` 不渲染自定义行**：官方是 React keyed slot（`View.tsx:56` 的 `renderSlot`）；本仓不是 React，业务动作全部走 `ContextMenuContribution` 数据面。
3. **`notify` Toast 未实现**：改名/删除的失败反馈留在各自对话框里（官方 `WorkspaceDialogs` 也是框内 error），不画一条没有官方几何依据的浮动 Toast。
4. **省略号按钮已复用官方素材**：`plugins/dsassets.ContextMenuButtonIcon` 是 `Button.tsx:49` 的 `IconEllipsisOutlineRegular` 三条填充 path，不再用排版字符 `⋯`。

### 1.4 2026-11-17 视觉复刻数值

官方 `context-menu/src/**` 没有 `*.module.css`；`View.tsx:2,50-57` 把实际绘制交给 primitives `Menu`。本仓从官方 `dsh-web-frontend/dist/assets/index-CSAdOBx6.css:1` 提取 `_70kcw_` 与 `_1w4el_` 原始规则到 `DSH-GO/tmp/GO8-RC2/_vccm/` 核对：

- 菜单面：padding **4**、min/max width **144/360**、viewport margin **12**（`primitives-menu-official.css:2-5`）；圆角 **radius-lg=16**（`menu-material-official.css:1` + `base.css:21`）；材质 **menu-surface-fill**（`menu-material-official.css:5`）。
- 菜单项：**34px** 高、**6×8** padding、**6** gap、**13/20** 字、**radius-md=12**（`primitives-menu-official.css:19` + `base.css:20`）；图标 **14×14**（同文件 `:30-31`）；禁用 opacity **0.4**（`:29`）。
- 分隔线：**0.5px / margin 3px 8px / border-l2**（同文件 `:44`）。
- 阴影：0.5px border-l1 + **0/3/8/.04** + **0/0/20/.05**（`gradient-shadow-text.css:33-37`）。菜单材质的 blur(40px)/saturate(150%) 仍是平台未验项，不拿别的东西冒充。

`TestOfficialMenuGeometryAndStyles` 用真实 Viva 帧逐项钉住上述面/行/分隔线/阴影与矢量图元。

## 2. #62 `dsh-client-ui-workspace` 的 rename / delete（依赖 #10）

### 2.1 官方事实

| 官方事实 | 锚点 |
|---|---|
| workspace 菜单：rename(order 100, icon workspace.rename, `rename`「重命名」)，delete(order 300, danger, separatorBefore, `delete.workspace`) | `OFF:ui-workspace/src/client/session-actions/context-menus.tsx:39-68` |
| 重命名模态：`rename.workspace.title`「重命名工作区」/ `field.workspaceName`「工作区名称」/ `conflict.named` / 空与未改标题禁用 | `OFF:ui-workspace/src/client/rows/WorkspaceDialogs.tsx:16-113`；zh 文案 `locales.ts:50-58` |
| 删除模态：`delete.workspace`「删除工作区」/ `delete.desc`（只移除工作区列表，保留文件夹与会话，会话落“未分组”）/ `delete.pending`「正在删除工作区…」 | `OFF:ui-workspace/src/client/rows/WorkspaceDialogs.tsx:46-139`；zh 文案 `locales.ts:59-60` |
| 数据面：`workspace.registry`（Order 是唯一顺序真相） | `HOST:internal/transport/handlers_workspace_registry.go:12-40`；实现 `HOST:cmd/dshd/serve_workspace_registry.go:34-60` |
| 写面：`workspace.rename` / `workspace.delete` | `HOST:internal/transport/handlers_workspace_official.go:58-59,748,773`；真实现挂在同一份 `<home>/state/workspaces.json` |

### 2.2 本仓落地

| 层 | 落点 |
|---|---|
| 客户端 | `internal/apiclient/workspaces.go`：`WorkspaceRegistryView` / `WorkspaceRename` / `WorkspaceDelete` |
| 壳缓存与动作 | `cmd/app/workspace_registry_wiring.go`：`Snapshot.Workspaces` 缓存；`Actions.WorkspaceRename/Delete` 同步写宿主并回填错误 |
| 契约 | `pluginapi/plugin.go` 的 `Workspace`、`Snapshot.Workspaces`、`Actions.WorkspaceRename/Delete` |
| 菜单贡献 | `plugins/dssidebar/workspace_actions.go`：`workspace.rename`(order 100) / `workspace.delete`(order 300, danger, separatorBefore) |
| 对话框 | 同文件：官方中文逐字（标题/字段/描述/删除中），空标题、未改标题、重名禁用；写操作在途不可关 |
| 行入口 | `plugins/dssidebar/sidebar.go` 组头：registry 命中时用 registry 标题、挂右键 + 省略号；未命中保持旧 cwd 分组 |

### 2.3 #62 仍缺什么（所以仍是 PARTIAL）

1. **分组源**：官方 WorkspaceBrowser 用 `registry.SessionIDs` 分组并支持 `workspace.insertSessionBefore` / 拖拽重排；本仓仍按会话 `cwd` 分组（rename/delete 只对 cwd 能对上 registry path 的组生效）。
2. **WorkspaceBrowser 其余面**：`menu.addWorkspace`「添加工作区…」入口、默认工作区标记、workspace 拖拽排序、hover card（createdAt/cwd/copy）、`workspace.follow` 实时流（本仓用启动+动作后重拉，不是 stream）、workspace 行 folder/chevron 图标与官方 32px projectRow 几何（本仓复用 `SectionHeader`）。
3. **Session 菜单其余动作**：官方 pin/archive/fork/rename 在 `context-menus.tsx:39-68`；本仓只接了 `menu.sessionId`「复制 Session ID」，其余没有宿主写面（不做死按钮）。
4. **contextMenu.button 的 workspace 按钮**用绝对定位叠在组头上，比官方 projectRow 的 `rowActions` 布局更粗糙一格（功能可用，几何未逐像素对齐）。

## 3. 验证

```text
python scripts/parity-count.py --check                         PASS（66 包逐 id）
CGO_ENABLED=0 go build ./...                                   0
go vet ./...                                                   0
CGO_ENABLED=0 go test ./... -count=1                           0（40 包 / 0 FAIL）
gofmt -l cmd internal plugins pluginapi                        空
python scripts/doc-links.py                                    PASS
python scripts/scripts-index.py                                PASS
```

快照：`testdata/api.snapshot.json` batch12 = `workspace.registry` / `workspace.rename` / `workspace.delete`，
`counts.total=64`，各批之和 64（守卫 `TestSnapshotCountsMatchMethods`）。

## 4. 键盘路由审计（2026-11-18，`ui.Page.Key` 接上之后）

### 4.1 背景

`ui.Box.Shortcut` / `ui.Shortcut` 在 `ui.Page.Key` 接上之前是**死代码**（本壳是
`Page.Draw` 形态，宿主自己不送键进页面路由），所以本插件当时用
`pluginapi.KeyHandler` 自己处理 Shift+F10 / ↑↓ / Home / End / Enter / Space / Esc。
dsh-ui#166（`cmd/app/main.go` 的 `cfg.OnKey` 先送 `m.view.Key(ev)`，页面接了立刻 return）
接上之后两套机制会并存，这一节是收口后的口径。

### 4.2 官方怎么做（`dsh-client-ui-context-menu` rc.2 源码）

| 键 | 官方落点 | 语义 |
|---|---|---|
| Shift+F10 / ContextMenu | `src/client/trigger.ts:29-37` | 挂在**触发器元素自己的 `onKeyDown`**，读 `event.currentTarget`；`show()` 里 `preventDefault`+`stopPropagation`（一个键只处理一次）；`getTriggerProps` 还给了 `tabIndex:0`，所以键成立的前提是"触发器有焦点" |
| Esc / ↑↓ / Home / End / Enter / Space | `src/client/View.tsx:50-57` | 全交给 primitives 的 `Menu`（`autoFocus` / `contextual` / `onClose` / `onSelect`），**插件不声明任何全局键** |
| Esc（开菜单过渡期） | `src/client/index.ts` 的 `window.addEventListener('keydown', escape, true)` | 只在 `active !== undefined && target === null`（还没上屏）时成立，且 `stopImmediatePropagation()` |

结论：官方**没有**插件级全局按键路由；每个键只有一个所有者。

### 4.3 本仓落点（Viva 的两条官方形状 + 一条原生激活）

| 键 | 本仓落点 | 为什么是这条 |
|---|---|---|
| Shift+F10 | `plugins/dscontextmenu/context_menu.go` 的 `Attach`：`ui.Shortcut(c, ui.Shift, "f10")` +「焦点正好是这个目标」判据 | 官方是"触发器元素自己的键"；viva `ui.ContextMenu` 的规则逐字相同——"at the target when it has the focus and Shift+F10 is pressed"（`ui/menu.go:57-59`），判据 `target.id == event.Target`（`:631-640`），容器只要 `Hit` 过也算（`:75`）。**旧写法 `target.Shortcut(gfx.ModifierShift,"f10")` 在真装配下是死声明**：`Box.Shortcut` 对容器按作用域覆盖匹配（`ui/shortcut.go:69-71,120-123,162-165`），而本仓交互 id 是 `env.Tree.ID(env.Feat,…)` 扁平命名，永远不落在 `ui.Row` 按位置命名的作用域里 |
| Esc、↑、↓、Home、End | `drawMenu` 的容器级 `menu.Shortcut(...)`（只有菜单开着才声明） | 官方把菜单键给菜单原语；viva 的对应形状是容器级 `Box.Shortcut`（`ui/shortcut.go:53-71`），派发者 `ui.Page.Key`（`ui/page.go:502-514` → `matchShortcut`） |
| Enter、Space | **插件不声明**，由 Viva 输入路由器把"焦点区域上的一击"兑现成点击（`internal/input/input.go:308-315`；`gfx.RoleMenuItem` 两键都算，`:344-345`；painter 兑现 `app/painter.go:554`） | 官方 Menu 的"选中一项"就是这件事；插件再声明一遍即两套机制 |

`pluginapi.KeyHandler` 已整条删除（`context_menu.go` 里不再有 `HandleKey`、
`stepHighlight`、`runHighlighted`）。

### 4.4 全仓 Shortcut 声明盘点

`grep -rn "\.Shortcut(" --include="*.go" .` 的非测试命中只有 dscontextmenu 一处
（`Attach` 的 Shift+F10、`drawMenu` 的 Esc + 四个导航键）；页面级 `ui.Shortcut`
在插件里只有本插件 `Attach` 的这一条。改动前后：

- **死声明（已删）**：`target.Shortcut(gfx.ModifierShift,"f10")` —— 容器作用域匹配不到扁平 id，真装配下永不触发；
- **活的（保留）**：`drawMenu` 的 `escape` / `up` / `down` / `home` / `end`；
- **新接上的**：`Attach` 的页面级 `ui.Shortcut(c, ui.Shift, "f10")`（按焦点判定目标）；
- **改成原生激活（不是 Shortcut）**：Enter / Space。

### 4.5 测试与变异

```text
go test ./cmd/app/ -run 'TestShellContextMenu|TestShellPageKey|TestShellPageWide|TestShellKeyHandler'   ok
go test ./plugins/dscontextmenu/                                                                       ok
go test ./plugins/dsshortcuts/                                                                          ok
```

新增/改动的判据：

- `cmd/app/page_shortcut_test.go`
  - `newShellPageKeyHarnessShaped`：目标按**真实调用点形状**（`ui.Row` + 子元素 + `.Hit(id)`）摆；
  - `TestShellContextMenuShiftF10OnContainerTarget`：真实形状下 Shift+F10 由页面打开菜单，且不进程探针；
  - `TestShellContextMenuKeysHandledOnce`：**一个键只处理一次**——↓ 一次只走一格（不是"焦点变了"）、
    Enter/Space 动作只跑一次、Esc 只关一次；页面接掉的键一个都不进 `kernel.HandleKey`，
    Enter/Space 必须原样进一次（Viva 的原生激活靠这条）；
  - `TestShellContextMenuClosesLeavesKeysToPlugins`：菜单没开时这五个键原样进插件链（不偷键）。
- `plugins/dscontextmenu/keyboard_routing_test.go`
  - `TestPluginHasNoGlobalKeyHandler`：本插件不得再实现 `pluginapi.KeyHandler`（"不要两套并存"）；
  - `TestShiftF10OpensOnlyTheFocusedTrigger`：焦点在哪一行就开哪一行的菜单（旧 KeyHandler 取的是"第一个登记过的目标"）；
  - `TestShiftF10WithoutFocusOpensNothing`：焦点不在任何目标上时什么都不开。
- `plugins/dsshortcuts/shortcuts.go` + `shortcuts_guard_test.go`：F1 表新增 `Binding.Route`
  （`routePluginKey` / `routeShortcut` / `routeVivaActivate`），反向核对按路线分派——
  `routeShortcut` 行必须能在插件里找到真实 chord 声明，`routeVivaActivate` 行要求该包
  **没有** `HandleKey`。这不是放宽：对这几行它比"扫 `ui.KeyXxx` 常量"更严。

变异（逐条实跑，红 → 还原绿；日志在 `DSH-GO/tmp/GO8-RC2/_sc/`）：

| 变异 | 结果 |
|---|---|
| 壳把 `m.view.Key(ev)` 的早退去掉（同键进两条链） | 红：`Shift+F10 由页面接掉后又进了 kernel.HandleKey` |
| Shift+F10 改回 `target.Shortcut(...)` | 红：`真实形状（容器目标）下 Shift+F10 没打开菜单` |
| 把 KeyHandler 加回来 + 壳不早退 | 红：菜单根本打不开（页面 Activate 被吞） |
| 插件在 Enter 上自己跑一遍动作但不认领 | 红：`Enter 之后动作跑了 3 次（[one one one]），want 1` |
| 同一个 ↓ 处理者在同一帧应用两次 | 绿（说明"同帧重复请求焦点"本身是 no-op；双处理只能在**跨机制**时发生，由上面的探针判据抓） |
| F1 表把 Shift+F10 的 `Route` 去掉 | 红：`表里有 Shift+F10 这一行，但 plugins/ 下没有任何文件在处理它` |
| 把 `HandleKey` 加回 dscontextmenu | 红：`Enter（KeyEnter）记成 Viva 原生激活，但 plugins/dscontextmenu 又实现了 HandleKey` |
| 删掉 `menu.Shortcut(0,"home")` | 红：`表里把 Home（KeyHome）记成 ui.Shortcut 派发，但 plugins/ 下没有任何文件声明它` |

### 4.6 门禁（本轮，worktree `feat/hs-shortcut-audit`）

```text
python scripts/parity-count.py --check                         0
CGO_ENABLED=0 go build ./...                                   0
go vet ./...                                                   0
CGO_ENABLED=0 go test ./... -count=1                           0（55 包 / 0 FAIL）
test -z "$(gofmt -l cmd internal plugins pluginapi)"            0（空）
python scripts/doc-links.py                                    0
python scripts/scripts-index.py                                0
```
