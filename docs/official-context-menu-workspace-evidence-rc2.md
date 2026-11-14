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
| 渲染 | viva v0.6.0 原生 `ui.ContextMenu`（右键 / Shift+F10 / 指针点开）与 `ui.Menu`（省略号按钮锚定菜单）；官方 `MenuOverlay` 的等价物，不另造菜单视觉 |
| 按钮 | `ContextMenus.Button`：只有 `HasTarget(kind)` 才创建 20×20 命中区，对齐官方「无贡献者不渲染」 |
| 结构账 | `plugins/dscontextmenu/official_slots.go` 用 `RegisterOfficialSlot` 登记 `contextMenu.button`（factory/root）、`contextMenu.icon`（keyed/root）、`contextMenu.content`（keyed/root），父槽 `root`，锚点 `index.ts:37/40/42` |
| 消费方 | `plugins/dssidebar`：workspace 组头右键 + 行尾省略号（`Rows.tsx:325/330` 的等价座位）；session 行尾省略号（`Rows.tsx:752`） |

### 1.3 已知差异（不冒充官方）

1. **`contextMenu.icon` 不画图标**：官方图标来自 `dsh-client-ui-primitives`，该包**不在 rc.2 随包导出集**里；本仓没有官方矢量资产，菜单项只按文案渲染，不自画图标。
2. **`contextMenu.content` 不渲染自定义行**：官方是 React keyed slot（`View.tsx:56` 的 `renderSlot`）；本仓不是 React，业务动作全部走 `ContextMenuContribution` 数据面。
3. **`notify` Toast 未实现**：改名/删除的失败反馈留在各自对话框里（官方 `WorkspaceDialogs` 也是框内 error），不画一条没有官方几何依据的浮动 Toast。
4. **省略号按钮的字形**：官方是 `IconEllipsisOutlineRegular`（同一 primitives 包，未随包）；本仓用排版省略号 `⋯`，不是 SVG 图标。

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
