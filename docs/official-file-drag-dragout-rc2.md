# `#19 dsh-client-ui-file-drag`：拖出的精确阻断点（2026-11-17 重核）

> 结论：**仍不可做，但阻断点不在"我们没接"，而在 Viva 的窗口层**。
> 旧登记说的"Viva 无 `DataTransfer`、`SetClipboardFiles` 是复制不是拖拽"**对**；
> 本轮补上的是另一半：**官方这个包有两条拖出通道，本仓两条都缺**，
> 且**即使我们自己写 `DoDragDrop` 也拿不到窗口句柄**。

## 1. 官方这个包到底怎么拖出（逐行）

官方包 `@deepseek-ai/dsh-client-ui-file-drag` 的 `src/client/transfers.ts` 里有**两条**互斥通道，
按 `globalThis.dshDesktop.fileDrag` 在不在选一条：

| 通道 | 位置 | 机制 | 依赖 |
|---|---|---|---|
| 桌面桥（Electron） | `transfers.ts:32-41`（`prepareNative`）、`:102-118`（`native()` 的 `start/release`） | 走 `desktop.prepare/start/release/cancel`，`start` 才真正发起 OS 拖拽 | Electron preload 暴露的 `dshDesktop.fileDrag`（`index.ts:32-33` 判 `protocolVersion === 1`）→ 主进程 `DoDragDrop` |
| 浏览器 | `transfers.ts:120-155`（`browser()`） | `URL.createObjectURL(file)` + `transfer.setData(FILE_DRAG_TOKEN_TYPE, token)`（应用内识别）+ `transfer.setData('DownloadURL', '<mime>:<name>:<url>')`（`transfers.ts:150`） | **Chromium 的 `DataTransfer` + `DownloadURL` 约定**（拖到资源管理器时由 Chromium 落盘） |

两条都不是"复制到剪贴板"：`DownloadURL` 是 Chromium 专有的拖放载荷类型，`dataTransfer.effectAllowed = 'copy'`（`:148`）
表达的是拖拽下场效果，不是剪贴板写入。

## 2. Viva v0.6.4 侧四项实证（源码路径 `C:/Users/Administrator/go/pkg/mod/github.com/vivaui/viva@v0.6.4`）

| 检查 | 结果 | 证据 |
|---|---|---|
| OLE 拖出源 | **零命中** | `grep -rni "DoDragDrop\|IDropSource\|OleSetClipboard\|IDataObject\|OleDataObject"` 无输出 |
| 文件剪贴板 | 有，但是**复制** | `app/clipboard_files.go:43-67` `SetClipboardFiles` → CF_HDROP 写剪贴板 |
| 文件拖入 | 有，方向相反 | `app/app.go:154-160` `OnFileDrop`（系统文件拖**进**窗口） |
| 拖拽预览窗 | 有，但不是拖出源 | `app/drag_preview.go:20-36` `OpenDragPreview`：跟着指针的置顶穿透窗，给应用内拖放用 |
| 原生窗口句柄 | **不暴露** | `grep -rn "HWND\|NativeHandle\|WindowHandle" app/ internal/platform/contract.go` 无输出 |

最后一项是决定性的：Windows 的拖出必须由**持有窗口**的一方调
`DoDragDrop(hwnd, pDataObject, DROPEFFECT_COPY, …)`（或 `SHDoDragDrop`）。
`viva/internal/` 是 Go 的内部包，本仓 import 不了；`app` 包既不导出句柄、
也没有任何 drag-source 入口——**插件侧没有任何绕法**。

## 3. 判定

- **档位**：C 档（上游缺前置），不是 A 档（"宿主已在只差界面"）。
- **能做的前提**（缺一不可）：
  1. Viva 暴露窗口原生句柄（`Window.Handle()` 之类）；
  2. Viva 提供一个 drag-source 入口（例如 `app.Application.OpenDragSource(hwnd, []FileItem)` 或直接在 `Config` 上加 `OnDragOut` 钩子），
     由它在窗口线程跑 OLE 拖拽循环并在循环里正确处理鼠标捕获与 `MsgWaitForMultipleObjects`。
- **为什么不能在本仓"兜底"**：拖拽循环必须在窗口线程上跑，本仓没有窗口线程的控制权；
  即使拿到句柄，`IDropSource`/`IDataObject` 的 COM vtable 也要在窗口线程内建，
  和 Viva 自己的指针/捕获状态机直接冲突。这不是"加个 shim"能解决的。

## 4. 最小复现

`tmp/GO8-RC2/_miss2/` 下（临时产物，不进仓）：一个只做两件事的 Go 程序——
① 用 `app.Config` 建窗；② 试着从公开 API 取一次原生窗口句柄、再找一次 drag-source 入口。
两者都拿不到，编译期就不可能写出一行 `DoDragDrop` 调用。复现要点已写进上游 issue 草稿
（`$DSH_HOME/outbox/vivaui-viva-file-drag-source.md`，**未发送**，等 Lead 过目）。

## 5. 与 `#6 attachment` 的边界（沿用 2026-11-16 的纠正）

`plugins/dsattachment/overlay.go` 是官方 `attachment` 包的 `ComposerAttachments → DropOverlay`
（`OFF:attachment/src/client/ComposerAttachments.tsx:45-60`），那是**拖入**，仍归 `#6`；
本行不拿它凑 PARTIAL。

## 6. 本仓还缺的另一半（等 Viva 有了拖出源之后才谈得上）

`ctx.fileDrag.actions.prepare/read`（`index.ts:41-44`）、`file-drag.feedback` Toast
（`feedback.tsx:18-25`）、以及 `prepare` 在无桌面桥时的 `readSource` 分支
（`transfers.ts:89-100`：URL/blob → `File`）都还没有对应实现。
今天的结论是：**没有拖拽手势，这一整套没有触发入口**，所以不做半截。

## 7. 2026-11-20 更新：阻断点已消失（本文 §3 的两个前提都做到了）

§3 开出的两个前提 —— ① Viva 暴露窗口层的 drag-source 入口、② 由它在窗口线程
跑 OLE 拖拽循环 —— **已经自己做出来了**，不再等上游：

- Viva 侧：`app.StartFileDrag(paths, effects)`（`viva-ole/app/drag_files.go`）+
  `internal/platform/windows/drag_windows.go`（`IDataObject` 以 CF_HDROP 交出文件、
  `IDropSource` 管松手/Esc 与光标、`DoDragDrop` 在窗口线程跑）。公开 API 直接收
  路径列表，**不需要**暴露 HWND —— 句柄留在窗口层内部，插件侧只给路径。
  macOS / Linux / Web 三档是**明确不支持**桩（返回 `ErrUnsupported` 并写明缺什么），
  不是缺席。PR：**vivaui/viva#360**。
- 本仓侧：`plugins/dsfiles/drag_files.go`（press=prepare → 越 4px=start →
  松手/离开=release/cancel）+ 壳 `cmd/app/main.go` 的 `cfg.OnPointer` 发起
  （窗口线程、页面绘制之外）。

**真机验证到什么程度**（不夸大）：Windows 上注入真鼠标按下→移动→松手，
同完整级跨进程实测 `DoDragDrop` 返回 `DROPEFFECT_COPY`，目标进程收到真实路径；
**把文件拖进 Explorer 窗口那一步没验成功** —— 本会话进程是 elevated、Explorer 是
medium，OLE 不跨完整级送 drop（与 `drop_windows.go` 记录的入向限制同一条），
在非提权会话里应能补齐。§6 列的三项（浏览器 DownloadURL 通道、`file-drag.feedback`
的 Toast 形态、从聊天附件拖出）仍未做，所以本包记 🟡 而不是 ✅。
