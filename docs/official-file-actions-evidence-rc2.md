# 官方文件动作（#18 `dsh-client-ui-file-actions`）的结构证据与实现台账

> 建立时间：2026-11-15 · 对应 [official-parity.md](official-parity.md) 第 18 行（❌ MISSING → 🟡 PARTIAL）。
>
> **这份文件回答四个问题**：
> 1. 官方 rc.2 的 `dsh-client-ui-file-actions` 是什么结构（服务 / 两段菜单 / 文案 / 判据）？
> 2. 它消费哪个宿主方法，数据形状是什么？
> 3. 本仓实现落在哪些 `file:line`，哪些值是从官方原文学来的？
> 4. 哪几处**与官方不同 / 没做**，各自缺的是什么？

**证据口径**：rc.2 起官方**随包发 `src/**` 原件**。锚点 `OFF:<包简称>/<src 内相对路径>:<行号>`，
`OFF` 根 = `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`；宿主锚点 `HOST:`。

---

## 0. 官方包是什么（**不是一张页面，而是一族右键动作 + 一个服务**）

| 文件 | 行数 | 作用 |
|---|---|---|
| `OFF:fa/src/client/index.ts` | 38 | 浏览器半侧入口：`provide('fileActions', new BrowserFileActions(desktop))`，再 `ctx.inject([...], registerFileMenus)` |
| `OFF:fa/src/client/files.ts` | 140 | `BrowserFileActions`：`save`（download / saveAs）与 `copyImage`（`navigator.clipboard.write`） |
| `OFF:fa/src/client/context-menus.tsx` | 104 | **本包唯一的可见行为**：两段 context menu 贡献（`file` order 200 / `image` order 100） |
| `OFF:fa/src/client/locales.ts` | 175 | 8 种语言的字典；zh 12 条（本文第 2 节的文案逐字来自它） |
| `OFF:fa/src/types.ts` | 45 | `FileActions` / `DesktopFileActions` / `FileSaveResult` 三个接口 |
| `OFF:fa/src/index.ts` | 4 | Host 侧空 `apply()`：这个包在宿主侧没有行为 |

**座位归属**：它**不占任何 slot**，只往 `ctx.contextMenus` 注册两段贡献
（`OFF:fa/src/client/context-menus.tsx:49,85`）+ 往 `contextMenu.icon` 注册三枚图标
（`OFF:fa/src/client/index.ts:29-33`）。所以本仓的对应物是四件：

| 官方那层 | 本仓落点 |
|---|---|
| `file` / `image` target 的判别键 | `pluginapi/context_menu.go`（`TargetFile` / `TargetImage` + 每类 target 的字段） |
| 两段菜单贡献 | `plugins/dsfileactions/plugin.go` |
| 原始字节（`workspaceFiles.readBytes`） | `internal/apiclient/workspace_files.go` |
| `desktop.save`（原生另存 / 下载） | `cmd/app/file_actions.go`（壳侧，走 `pluginapi.Actions.SaveWorkspaceFile`） |

---

## 1. target 形状（官方逐字段 → 本仓字段）

官方 `ContextMenuTargetMap`（`OFF:ui-context-menu/src/client/types.ts:29-40`）：

```ts
file:  { sessionId: SessionId; path: string; line?: number; paneId?: PaneId }
image: { sessionId?: SessionId; url: string; name?: string; mediaType?: string; path?: string }
```

本仓 `pluginapi.ContextMenuTarget` 是平面结构（`Kind` 是判别键），字段一一对上：
`SessionID` / `Path` / `Line` / `URL` / `Name` / `MediaType`（`pluginapi/context_menu.go` 的注释逐个写了它是官方哪一族的哪个字段）。

**生产方**（谁把 target 挂到元素上）：

| target | 本仓生产方 | 官方同族 |
|---|---|---|
| `file` | `plugins/dsfiles/files.go` 的文件行（`Attach`，带**会话 + 工作区相对路径**） | 侧栏文件行 / 文档预览 / 引用 chip 都挂 `file` |
| `image` | **暂无**（消息区图片是 64×64 占位瓦片，没有 URL/路径可取；见 §4 第 4 条） | 图片消息、附件缩略图 |
| `workspace` | `plugins/dssidebar/grouping.go`（既有） | `open-in-app` 扩展出来的族 |

---

## 2. 两段菜单的判据（官方原文 → 本仓结果）

### 2.1 `file`（`OFF:fa/src/client/context-menus.tsx:49-84`）

官方的三段展开条件是（`:68-80`）：

```ts
...mediaType.startsWith('image/') ? [{ id:'copyImage', separatorBefore:true, … }] : [],
...mediaType.startsWith('image/') || !actions.canSaveAs ? [{ id:'download', … }] : [],
...actions.canSaveAs ? [{ id:'saveAs', … }] : [],
```

本仓 `canSaveAs = true`（viva 有原生另存对话框，见 §3），`copyImage` 不画（§4 第 1 条）：

| 目标 | 官方 | 本仓 |
|---|---|---|
| 图片文件 | 复制图片 · 下载 · 另存为… | **下载 · 另存为…** |
| 非图片文件 | 另存为… | **另存为…** |

### 2.2 `image`（`OFF:fa/src/client/context-menus.tsx:85-104`）

官方三项：`copy` / `download` / `canSaveAs` 时的 `saveAs`。本仓同样少 `copy`；
并且**只在 (sessionId, path) 都在时给动作**（没有它们就只能靠 `source.url` fetch，
本仓没有浏览器那一层，§4 第 3 条）。

### 2.3 文案（`OFF:fa/src/client/locales.ts:5-16` 的 zh 字典，逐字）

| key | zh | 本仓出现处 |
|---|---|---|
| `menu.download` | `下载` | `plugins/dsfileactions/plugin.go` 的 `zhDownload` |
| `menu.saveAs` | `另存为…` | 同上 `zhSaveAs` |
| `menu.copyImage` | `复制图片` | **不画**（§4 第 1 条） |
| `menu.saved` | `已保存` | `cmd/app/file_actions.go` 的 `zhFileSaved` |
| `menu.downloaded` | `下载完成` | 同上 `zhFileDownloaded` |
| `menu.saveFailed` | `无法保存文件，请检查源文件和目标位置后重试` | 同上 `zhFileSaveFailed` |

---

## 3. 数据面：`workspaceFiles.readBytes` 是**真的**

| 环节 | 锚点 |
|---|---|
| 方法名 | `HOST:internal/transport/handlers_workspace_official.go:70` |
| handler（scope/path/options 三个位置参数） | `HOST:internal/transport/handlers_workspace_official.go:997-1026` |
| 生产接线 | `HOST:cmd/dshd/serve_workspace_api.go:570`（`FilesReadBytes: h.filesReadBytes`） |
| 真实现（工作区根 + 符号链接拒绝） | `HOST:cmd/dshd/serve_workspace_api.go:1208`、`:1360-1390` |
| `data` 的 L1 差异（Uint8Array → base64 + `encoding`） | `HOST:internal/transport/handlers_workspace_official.go:208-220` |
| 本仓客户端（解码成 `[]byte`） | `internal/apiclient/workspace_files.go` |

参数是**位置参数**：`sessionId`（宿主别名 `workspaceFileScopeId` / `agentId`）+
`path`（工作区**相对**路径）+ `options`（官方必填，读整个文件给 `{}`）。
所以界面**不许**自己拼绝对路径——工作区根只在宿主侧解析。

契约快照：`testdata/api.snapshot.json` 新增 `workspaceFiles.readBytes`（`batch 17`，
counts.total 76 → 77，与 methods 条数一致）。

---

## 4. 与官方的差异（照实登记，不假装搬了）

1. **「复制图片」不画**：官方走 `navigator.clipboard.write([new ClipboardItem({'image/png': …})])`
   （`OFF:fa/src/client/files.ts:105-138`，非 PNG 还要经 `<canvas>` 转码）。
   viva v0.6.4 只导出 `app.ClipboardImagePNG`（**读**）与 `app.SetClipboardText`；
   图片的**写**只存在于测试 Driver（`driver.SetClipboardImage`）。`SetClipboardFiles`
   是"复制文件"，语义不同，**不许冒充**。
2. **反馈落在状态栏，不是 toast**：官方成功后 `ctx.contextMenus.notify(...)`，还带一个
   「在文件夹显示」的后续动作（`OFF:fa/src/client/context-menus.tsx:41-47`）。本仓
   context menu 服务没有 toast（那里已登记同一条理由），所以"已保存 / 下载完成 / 失败"
   写在壳的状态栏。官方文案**一字不改**，只在冒号后面补上落点路径——官方靠 toast 里那个
   reveal 动作回答"存哪了"，本仓没有那个动作，不把落点写出来就没法回答。**「在文件夹显示」没接**：官方走
   `remote.session.openWorkspacePath({action:'reveal'})`，宿主没有这个方法
   （`HOST` 全仓 grep 为空）；用 `Actions.OpenFile` 打开目录是另一种语义，不冒充。
3. **`image` target 只在有 (sessionId, path) 时给动作**：官方可以只靠 `source.url`
   fetch 原字节（`OFF:fa/src/client/files.ts:41-52` 的 `fetch(source.url)`）。本仓没有
   浏览器 HTTP 栈，取不到远端图片字节，所以那一档**不给动作**（菜单不弹），
   而不是画一个点了必失败的项。
4. **`image` target 今天没有生产方**：消息区的图片是占位瓦片（无像素解码路径，见
   [official-parity.md](official-parity.md) 第 6 行 #6 的"剩余差异"），拿不到 URL/路径。
   所以 `image` 贡献**已注册且有测试**，但要等图片生产方（附件缩略图/灯箱）出现才会
   真弹。这一条是本行仍是 🟡 而不是 ✅ 的直接原因之一。
5. **没有 `stat` 前置的置灰态**：官方 `resolve` 先 `workspaceFiles.stat`，失败时把动作
   置灰（`OFF:fa/src/client/context-menus.tsx:54-58`）。本仓的 `Resolve` 是同步的
   （不能在一次求值里发 HTTP），所以动作总是可点，失败由壳用官方 `menu.saveFailed`
   文案报出来。差异是"能不能点"，不是"会不会静默成功"。
6. **`canSaveAs` 恒 true**：官方 Web 侧是 false（浏览器只能 download）；本仓是原生窗口，
   有 `app.SaveFile`（`SaveFileOptions{SuggestedName}`，取消返回 `app.ErrCanceled`），
   所以走的是 Desktop 那一档。
7. **没有 `line` / `paneId` 的消费方**：`file` target 的这两个可选字段已进结构体，
   但今天唯一的动作（保存字节）不用它们；「在侧栏打开并定位到第 N 行」属于
   `open-in-app` 的 `openIn('sidebar')`，本仓右栏还没有文件视图，故不下发。

---

## 5. 落点与门禁证据

| 层 | 文件 | 钉住它的测试 |
|---|---|---|
| 契约（kind + 字段） | `pluginapi/context_menu.go` | `plugins/dsfileactions/plugin_test.go`（两段贡献的 id/target/order） |
| 菜单判据 | `plugins/dsfileactions/plugin.go` | 同上：图片 → `[download saveAs]`、非图片 → `[saveAs]`、缺身份/缺壳动作 → 0 项、Run 交出的四个字段是**捕获值** |
| target 生产方 | `plugins/dsfiles/files.go`（`Attach`）、`plugins/dsfiles/paths.go`（`relPath`） | `plugins/dsfiles/file_target_test.go`（真布局渲染 + 记录 Attach；目录行不挂、无会话不挂） |
| 数据面 | `internal/apiclient/workspace_files.go` | `cmd/app/file_actions_test.go`（假宿主的真实 wire 形状） |
| 壳侧动作 | `cmd/app/file_actions.go`、`cmd/app/main.go`（`SaveWorkspaceFile` 接线） | 同上：另存 / 下载 / 取消静默 / 参数不全；写盘字节是**原始字节** |
| 装配顺序 | `cmd/app/main.go` 的 import 与 `pluginapi.Builtin()` | `cmd/app/file_actions_assembly_test.go`（产品装配路径下两个 target 都挂上） |

装配顺序是本行唯一的"看不见的坑"：`dsfileactions` **不把** `KeyContextMenus` 写进
`Requires`（用户可以在插件页停用 `ds-contextmenu`，那时整棵树不该起不来），所以
"服务还没就位"只能是**日志 + 装配测试**守住，不能靠硬依赖。
