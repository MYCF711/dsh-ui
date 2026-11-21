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
| `navigator.clipboard.write(ClipboardItem)`（复制图片） | `cmd/app/copy_image.go`（壳侧，走 `pluginapi.Actions.CopyWorkspaceImage` + viva `app.SetClipboardImage`） |

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
| 图片文件 | 复制图片 · 下载 · 另存为… | **复制图片 · 下载 · 另存为…**（2026-11-21 起） |
| 非图片文件 | 另存为… | **另存为…** |

### 2.2 `image`（`OFF:fa/src/client/context-menus.tsx:85-104`）

官方三项：`copy` / `download` / `canSaveAs` 时的 `saveAs`。本仓 `#18` ① 闭合（2026-11-21）后
**同样三项**，id 也逐字照官方（`copy`，不是 file 族的 `copyImage`）；
但仍然**只在 (sessionId, path) 都在时给动作**（没有它们就只能靠 `source.url` fetch，
本仓没有浏览器那一层，§4 第 3 条）。

### 2.3 文案（`OFF:fa/src/client/locales.ts:5-16` 的 zh 字典，逐字）

| key | zh | 本仓出现处 |
|---|---|---|
| `menu.download` | `下载` | `plugins/dsfileactions/plugin.go` 的 `zhDownload` |
| `menu.saveAs` | `另存为…` | 同上 `zhSaveAs` |
| `menu.copyImage` | `复制图片` | `plugins/dsfileactions/plugin.go` 的 `zhCopyImage`（两族共用同一文案，§4 第 1 条） |
| `menu.saved` | `已保存` | `cmd/app/file_actions.go` 的 `zhFileSaved` |
| `menu.downloaded` | `下载完成` | 同上 `zhFileDownloaded` |
| `menu.saveFailed` | `无法保存文件，请检查源文件和目标位置后重试` | 同上 `zhFileSaveFailed` |

---

## 3. 数据面：`workspaceFiles.readBytes` / `workspaceFiles.stat` 都是**真的**

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
counts.total 76 → 77，与 methods 条数一致）；**2026-11-17 再加 `workspaceFiles.stat`**
（`batch 24`，counts.total 121 → 122，`snapshot_methods_test.go` 的方法表/计数两条一起守）。

`workspaceFiles.stat` 的形状（本仓 `internal/apiclient.WorkspaceFileStat`）：

| 环节 | 锚点 |
|---|---|
| 方法名 | `HOST:internal/transport/handlers_workspace_official.go:81` |
| handler（**只有** scope/path，无 options） | `HOST:internal/transport/handlers_workspace_official.go:1212-1242` |
| 参数取值 | `HOST:internal/transport/handlers_workspace_official.go:820-843`（`sessionId` 别名 `workspaceFileScopeId`/`agentId`；`path` 必填、可空串） |
| 只 stat 普通文件（目录回 `workspace-file/not-regular-file`） | 同上 `:1233-1240` |
| 本仓客户端 | `internal/apiclient/workspace_files.go`（`WorkspaceFileStatOf` → `WorkspaceFileStat`） |

**为什么它必须存在**：官方 file 菜单在列出动作**之前**先 stat，失败就把三条全标
`disabled`（`OFF:fa/src/client/context-menus.tsx:50-60` + `:71/:75/:79` 的 `disabled: !available`）。
少了这条，我们只能在"画成可点、点下去必失败"和"整张菜单不弹"之间二选一，两种都不是官方行为。

---

## 4. 与官方的差异（照实登记，不假装搬了）

1. **「复制图片」——原先是唯一平台阻断项，2026-11-21 已闭合**。
   **当时的阻断**：官方走 `navigator.clipboard.write([new ClipboardItem({'image/png': …})])`
   （`OFF:fa/src/client/files.ts:65-89`，非 PNG 还要经 `<canvas>` 转码），而 viva v0.6.4/0.6.5
   的剪贴板**只有读图片**（`app.ClipboardImagePNG` / `HasClipboardImage`），图片的**写**
   只活在测试 Driver 上（`(*Driver).SetClipboardImage`，仅 headless 驱动）。
   `SetClipboardFiles` 是"复制文件"，语义不同，**不许冒充**——当时宁可不画。
   **怎么解封的**：`dsh-ui/go.mod` 把 viva 指到 `github.com/MYCF711/viva v0.6.10-clip`
   （`replace github.com/vivaui/viva => …v0.6.10-clip`），上游导出了生产级写入口——
   `app.SetClipboardImage(png []byte) error`（`app/clipboard_image.go:74-90`）与
   `app.SetClipboardImageAsync`；校验复用 `platform.CheckClipboardPNG`
   （`internal/platform/clipboard_image.go:32-42`：空 / 非 PNG 魔数 / 超
   `MaxClipboardImageBytes`=32 MiB 一律报错，且**不动剪贴板**）。Windows 侧
   `internal/platform/windows/clipboard_image_windows.go:31` 同时写注册的 `"PNG"` 格式与
   `CF_DIB`（透明区合成白底），并带 5×10 ms 的剪贴板占用重试——比"只写 PNG"更兼容
   只认位图的阅读器。
   **本仓落点**（三层，与保存链同构）：

   | 层 | 落点 | 官方依据 |
   |---|---|---|
   | 菜单项 | `plugins/dsfileactions/plugin.go` 的 `copyAction`；file 族 `copyImage`（带 `separatorBefore`）、image 族 `copy` | `OFF:context-menus.tsx:69-72`、`:99` |
   | 壳动作 | `pluginapi.Actions.CopyWorkspaceImage` → `cmd/app/copy_image.go` | `OFF:context-menus.tsx:44-47` 的 `copy` |
   | 转码 | `internal/imgdecode.ToPNG`（PNG 原样透传、其它格式解码重编） | `OFF:files.ts:67-86` 的 canvas 那一步 |
   | 写入口 | `app.SetClipboardImage`（替换点 `clipboardSetImage`，测试不碰真剪贴板） | — |

   **一处刻意的优于官方**：官方按 `blob.type === 'image/png'` 判要不要转码，而 blob 的
   type 来自**路径扩展名**——一个改了后缀的 JPEG 会被它原样塞进 `ClipboardItem`，
   粘出来是坏图。本仓按 **PNG 魔数**判（`imgdecode.IsPNG`），错后缀的图会被**转码**
   成正牌 PNG 再写。零代价，所以做（`AGENTS.md` 铁律 13）。
   **反馈**：官方成功后 `ctx.contextMenus.notify(t('menu.copied'), true)`（"已复制"，浮条），
   失败由菜单项的 `failureLabel: t('menu.copyFailed')` 显示。本仓没有浮条服务（本节第 2 条），
   两种都落状态栏：成功 `已复制`、失败 `无法复制，请检查内容是否可用后重试：<原因>`
   ——文案逐字来自 `OFF:locales.ts:8,12`。
   **没做的边界（照实登记）**：① 超过 32 MiB 的 PNG 被写入口拒（上游常量
   `MaxClipboardImageBytes`），状态栏如实报错；② WebP 没有解码器（`internal/imgdecode`
   只注册 png/jpeg/gif），复制 WebP 会失败并带原因；③ 动图 GIF 只复制第一帧（官方 canvas
   `drawImage` 同样只有第一帧，**不是**我们的简化）。
2. **反馈落在状态栏，不是"带后续动作的浮条"**：官方成功后
   `ctx.contextMenus.notify(text, success, {label, failureLabel, run})`
   （`OFF:fa/src/client/context-menus.tsx:30-42`）。本仓 context menu 服务**没有** notify
   （`pluginapi.ContextMenus` 只有 RegisterContribution/HasTarget/Attach/Button），壳里的
   本地浮条入口 `pluginapi.Actions.Notify` 只收一个字符串、**没有后续动作**这一格，而
   `dsnotifications` 的卡片其实**能画动作**（`plugins/dsnotifications/card.go:152-160`，
   动作来自宿主 `notifications` 数据）——缺的是"壳侧发一条带 run 的本地通知"这条通路。
   所以"已保存 / 下载完成 / 失败"写在壳的状态栏。官方文案**一字不改**，只在冒号后面补
   落点路径——官方靠浮条里那个 reveal 动作回答"存哪了"，本仓没有那个动作。
   **「在文件夹显示」今天不接**，但**卡点已换**：宿主其实**已有**
   `session.openWorkspacePath({action:'reveal'})`
   （`DSH-GO/cmd/dshd/serve_native_api.go:79-90` 的 `svc.Reveal`、
   `DSH-GO/internal/transport/handlers_native_official.go:233` 只接受 `reveal` 或缺省），
   所以缺的是两件本仓的事：①`internal/apiclient` 还没有这个方法（`WorkspaceFileStat` 是
   本轮新加的第一条 session 侧原生调用，可照同一形状加）；
   ②**没有可放这个动作的浮条**（上一条）。两条都补上之后才画，绝不拿
   `Actions.OpenFile` 打开目录冒充"在文件夹里选中这个文件"（语义不同）。
3. **`image` target 只在有 (sessionId, path) 时给动作**：官方那一族可以只靠
   `source.url` 取字节——`read` 没有时 `fetch(source.url)`，且只认
   `http: / https: / blob: / data: / dsh-app:` 五种协议
   （`OFF:fa/src/client/files.ts:23-36`；跨域 http(s) 还要走桌面壳的
   `readClipboardImage`）。本仓 UI 侧没有这条 URL→字节链（不是权限问题，是没有那一层），
   所以只有 URL 的 image target **不给动作**（菜单不弹），而不是画一个点了必失败的项。
   能补的条件：图片字节能从"URL"取到——要么宿主给一条 `workspaceFiles` 之外的取数
   方法，要么 `dsh-app:` 这类 URL 在本仓有解析链。今天两条都没有。
4. **`image` target 今天没有生产方**（本行仍是 🟡 的直接原因之一）：官方的两个生产方都是
   **渲染出来的图片面**——
   `OFF:dsh-client-ui-attachment/src/MessageImage.tsx:115-118`（消息里的图片附件：
   `{kind:'image', value:{url, name, mediaType}}`，**没有 sessionId/path**）与
   `OFF:dsh-client-ui-chat/src/client/chat/ChatView.tsx:75-79`（markdown 图片：
   `{kind:'image', value:{sessionId, url, name?, mediaType?, path?}}`）。
   本仓消息区对 `image` 附件画的是**空 64×64 占位**（`plugins/dsconversation/official_transcript_view.go:357-364`
   的 `ui.Slot(...).Border(...)`，没有像素），也没有 markdown 图片渲染，所以**没有可绑的
   图片面**。补它的前提是先让图片真的上屏：viva 侧 `ui.Image(c, gfx.Bitmap)` 是现成的
   （头像在用：`plugins/dssettings/official_account.go:266`），缺的是"取字节 + 解码"链
   （宿主附件 URL 或工作区路径 → `[]byte` → `gfx.Bitmap`），属 #6/#19 范围。
   在没有真图片之前**不造一个"点了必然取不到字节"的 image target**。
5. ~~没有 `stat` 前置的置灰态~~ **已修（2026-11-17）**：官方 `resolve` 先 stat，
   失败时**照样列出**三条、逐条 `disabled: !available`
   （`OFF:fa/src/client/context-menus.tsx:50-60` + `:71/:75/:79`）。本仓现在同一条判据：
   壳的 `pluginapi.Actions.WorkspaceFileStat`（`cmd/app/file_actions.go` 的
   `workspaceFileStat`，1 秒上限）→ 插件把它压成 `Disabled`。
   三档口径都钉住了：stat 成功→可点；stat 失败→**仍然列出、全灰**；壳没接这条探测
   （老宿主）→ 按可点画（差异登记，不是静默降级）。
   **剩余差异只有一处且是架构性的**：`Resolve` 是同步签名（官方是 async Promise），
   所以这次 stat 是**在 UI 线程上同步 HTTP**（带 1 秒上限）。与保存链同一条已知差异：
   官方的 await 能把 HTTP 留给下一次事件循环，viva 的绘制帧里没有那层，异步反而要把
   Box 生命周期拖过帧边界。
6. **`canSaveAs` 恒 true**：官方 Web 侧是 false（浏览器只能 download）；本仓是原生窗口，
   有 `app.SaveFile`（`SaveFileOptions{SuggestedName}`，取消返回 `app.ErrCanceled`），
   所以走的是 Desktop 那一档。
7. **没有 `line` / `paneId` 的消费方**：`file` target 的这两个可选字段已进结构体，
   但现有三条动作（复制图片 / 下载 / 另存为）都不用它们；「在侧栏打开并定位到第 N 行」属于
   `open-in-app` 的 `openIn('sidebar')`，本仓右栏还没有文件视图，故不下发。

---

## 5. 落点与门禁证据

| 层 | 文件 | 钉住它的测试 |
|---|---|---|
| 契约（kind + 字段） | `pluginapi/context_menu.go` | `plugins/dsfileactions/plugin_test.go`（两段贡献的 id/target/order） |
| 菜单判据 | `plugins/dsfileactions/plugin.go` | 同上：图片 file → `[copyImage download saveAs]`（copyImage 带 `separatorBefore`）、非图片 file → `[saveAs]`、image → `[copy download saveAs]`、缺身份/缺壳动作 → 0 项、Run 交出的是**捕获值** |
| 复制项的两族差异 | 同上（`copyAction`） | 同上：file 族 id `copyImage` + 分隔线 + 吃 stat 置灰；image 族 id `copy`、无分隔线、永不置灰 |
| target 生产方 | `plugins/dsfiles/files.go`（`Attach`）、`plugins/dsfiles/paths.go`（`relPath`） | `plugins/dsfiles/file_target_test.go`（真布局渲染 + 记录 Attach；目录行不挂、无会话不挂） |
| 数据面 | `internal/apiclient/workspace_files.go` | `cmd/app/file_actions_test.go`（假宿主的真实 wire 形状） |
| 壳侧动作 | `cmd/app/file_actions.go`、`cmd/app/main.go`（`SaveWorkspaceFile` 接线） | 同上：另存 / 下载 / 取消静默 / 参数不全；写盘字节是**原始字节** |
| 复制图片 | `cmd/app/copy_image.go`、`cmd/app/main.go`（`CopyWorkspaceImage` 接线）、`internal/imgdecode/to_png.go` | `cmd/app/copy_image_test.go`（字节**逐字节**交给写入口 / 非 PNG 转码 / 写入口失败报原因 / 读失败不碰剪贴板 / 0 字节算失败 / 参数不全）＋ `internal/imgdecode/to_png_test.go`（魔数判据 / PNG 透传 / JPEG 转码保尺寸） |
| 装配顺序 | `cmd/app/main.go` 的 import 与 `pluginapi.Builtin()` | `cmd/app/file_actions_assembly_test.go`（产品装配路径下两个 target 都挂上） |
| 真渲染 | `plugins/dsfileactions/vr3_capture_test.go` | 右键展开 → 数菜单行：图片 file = `[复制图片 下载 另存为…]`（stat 探测一次）、image = `[复制图片 下载 另存为…]`（不 stat、不置灰）；点第一行交出捕获的 (sessionId, path) |

装配顺序是本行唯一的"看不见的坑"：`dsfileactions` **不把** `KeyContextMenus` 写进
`Requires`（用户可以在插件页停用 `ds-contextmenu`，那时整棵树不该起不来），所以
"服务还没就位"只能是**日志 + 装配测试**守住，不能靠硬依赖。
