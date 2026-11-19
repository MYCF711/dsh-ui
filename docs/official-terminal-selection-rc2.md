# 终端 selection 与复制的官方面（rc.2）

> 2026-11-19，task-97。承接 [#54](../docs/official-parity.md) 的最后一条 VT 面缺口。
> 官方根：`DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`；
> 下文 `OFFT` = `dsh-client-ui-sidebar-terminal`。
>
> 选区**不是官方自己写的代码**，是 xterm 的 `SelectionService` / `SelectionModel`：
> 官方 `terminal.tsx` 只 `new Terminal({...})`，没有关掉、也没有改写选区。
> xterm 打进官方包里（`OFFT/lib/client.terminal.js`，509 421 字节，官方树里没有
> 独立的 `@xterm/xterm` 源码包），所以 xterm 内部一律给 **`grep -b` 可复现的字节偏移**。

## A. 官方怎么开 xterm（选区的唯一开关点）

| 面 | 官方 | 位置 |
|---|---|---|
| 构造 | `new Terminal({minimumContrastRatio: 4.5, cursorBlink: true, fontSize, fontFamily, scrollback: environment?.scrollback ?? 0})` | `OFFT/src/client/terminal.tsx:102-105` |
| 挂载 | `xterm.loadAddon(new FitAddon()); xterm.open(node)` | `terminal.tsx:106-108` |
| 选区选项 | **一个都没设**：没有 `selectionOn`、没有 `copyOnSelect`、没有 `macOptionClickForcesSelection`、没有 `rightClickSelectsWord` | `terminal.tsx:102-105`；`copyOnSelect` 在 509 421 字节的 bundle 里 **0 命中** |
| 主题 | `selectionBackground: foreground, selectionForeground: background, selectionInactiveBackground: foreground` | `OFFT/src/client/terminal-theme.ts:61` |
| 前景/背景的真值 | `getComputedStyle(节点)` 取 `.screen` 的 `color` / `backgroundColor`（= `label-primary` / `bg-base`） | `terminal.tsx:140-143`；`terminal.module.css:2-4` |

结论：**选区开箱即用**，官方没有任何「开选区」的调用；关掉它才需要写代码。

## B. 拖选几何（全部是 xterm 的默认行为）

| 交互 | 官方行为 | 证据（bundle 字节） |
|---|---|---|
| 按下 | `handleMouseDown`：`detail=1` 单选、`2` 双击、`3` 三击；Shift 按下走「只挪终点」 | byte 121098、122249 |
| 平常拖动 | **线性（流式）**：`_activeSelectionMode = shouldColumnSelect(e) ? 3 : 0` | byte 122249 |
| 矩形 | `shouldColumnSelect(e)` = `e.altKey && !(isMac && macOptionClickForcesSelection)` → **Windows/Linux 上是按住 Alt 拖** | byte 122685 |
| 双击 | 选**词**：`_activeSelectionMode = 1`，长度写进 `selectionStartLength` | byte 122685、122249 |
| 三击 | 选**整行**：`_activeSelectionMode = 2`，`selectionStart=[0,first]`、`selectionStartLength = cols` | byte 128691 |
| 起止格 | 坐标是**缓冲区绝对行**（`[列, 虚拟缓冲行]`），不是视口行；所以滚回里的行和屏幕行同一套坐标 | byte 120966 |
| 宽字 | 起点/终点落在全角字的**尾巴格**上要挪开：`hasWidth(col) === 0 && col < cols → col++` | byte 122249、123245 |
| 词分隔符 | `wordSeparator` 默认 `" ()[]{}',\"`"`；判据 `_isCharWordSeparator` = `width !== 0 && indexOf(getChars()) >= 0` | byte 278356、128691 |
| 反向拖 | `areSelectionValuesReversed()` 时 `finalSelectionStart/End` **互换两端** | byte 95563 |

## C. 选中态视觉

| 面 | 官方 | 证据 |
|---|---|---|
| 载体 | **独立一层** `_selectionContainer`（绝对定位 div），不是 `::selection`、不是改字形 | byte 80805（`handleSelectionChanged`）、byte 78634（CSS：`position:absolute; pointer-events:none; z-index:1`） |
| 有效底色 | `selectionBackgroundOpaque = blend(background, selectionBackgroundTransparent)`，且**不透明**的 `selectionBackground` 会被强制降到 `alpha .3` | byte 132041、132577 |
| 字色 | 选中格子的字换成 `selectionForeground`（官方设成 `background` = `bg-base`） | byte 88032 |
| 失焦 | 底色换成 `selectionInactiveBackground`——官方把它也设成 `foreground`，所以**焦点变化不改色** | `terminal-theme.ts:61`；byte 78634 |

所以官方选中态 = **`bg-base` 上盖 30% 的 `label-primary`，字用 `bg-base`**；
不是反显、不是 `::selection`、不是 50%。

## D. 复制：官方唯一的写剪贴板路径

| 环节 | 官方 | 证据 |
|---|---|---|
| xterm 只监听 `copy` 事件 | `addDisposableListener(element,'copy', e => hasSelection() && copyHandler(e, selectionService))` | byte 16481 |
| 写什么 | `clipboardData.setData('text/plain', selectionService.selectionText)` | byte 10034 |
| xterm **不绑**任何复制快捷键 | `_keyDown` 里 `evaluateKeyboardEvent` 的 default 分支把 Ctrl+字母算成 `String.fromCharCode(keyCode-64)`（Ctrl+C → `\x03`），随后 `cancel(e,true)`（preventDefault + stopPropagation） | byte 30324、231200 |
| 不自动复制 | 0 命中 `copyOnSelect`；选区变化只 fire `onSelectionChange` | byte 116835 |
| 右键 | `contextmenu` → `rightClickHandler`：把隐藏 textarea 挪到指针下、`rightClickSelectsWord` 默认 `isMac`（**Windows 上 false**），然后 `textarea.value = selectionText; textarea.select()` —— 只是**把内容准备好**，不写剪贴板 | byte 17140、10372、278356 |
| 真正的触发 | 桌面端右键弹出**原生编辑菜单**（撤销/重做/剪切/**复制**/粘贴/删除/全选），每项都带加速键，复制项点击走 `shortcuts.sendEditingKey('C',['control'])` → `webContents.sendInputEvent`，让 Chromium 对焦点元素执行编辑命令 | `app.asar` byte 207130469、207068569 |
| 快捷键表 | 官方 shortcuts catalog **没有复制项**（`dsh-client-ui-shortcuts` 里零命中） | `OFFT`/`dsh-client-ui-shortcuts/src/client/*` |
| Windows 菜单栏 | `Menu.setApplicationMenu` 在 win32 只有两条**不可见**的 devtools 项，没有 Edit 菜单 | `app.asar` byte 207128325 |

三条硬事实：

1. **Ctrl+C 永远是 `^C`**（xterm 自己把它转成 `\x03` 并 preventDefault），复制不可能是它；
2. **选完不会自动进剪贴板**；
3. 官方在 Windows 上真正能复制的那一条路是「**右键 → 原生编辑菜单 → 复制**」，
   而 xterm 的右键处理就是为这次 Copy 准备 `selectionText`。

## E. 与滚回的交互

选区坐标是缓冲区绝对行（B 节），所以**选到屏幕外就是选到滚回里**：
拖动时 xterm 自己在画布上下边缘滚动，并把终点贴到新的视口边界。

| 面 | 官方 | 证据 |
|---|---|---|
| 贴边滚动量 | `y 出画布 → 夹到 ±50px → amount = sign(y) + round(14 * y/50)`（1 格 ≈ 5 行/次，贴到底 15 行/次） | byte 120966 |
| 节拍 | `setInterval(_dragScroll, 50)` | byte 121584 |
| 滚动后终点 | 往下滚贴 `ydisp+rows-1`，往上滚贴 `ydisp`；非矩形模式下列为 `cols` / `0` | byte 124320 |
| 用户输入清选区 | `_coreService.onUserInput(() => hasSelection && clearSelection())` | byte 116835 |
| 缓冲区行被裁掉 | `handleTrim` 把坐标上移，整段掉出去就清空 | byte 95563 |

## F. 本仓的投影（`internal/vterm` + `plugins/dsterminal`）

| 官方 | 本仓 | 差异 |
|---|---|---|
| `SelectionModel`（起止格 → 文本） | `internal/vterm/selection.go`：`Selection` / `SelPos` / `SelMode` / `Normalized` / `Contains` / `SelectionText` | 无（逐条照 byte 95563、117677、219948、219185） |
| 行尾 `trimRight` | `Grid.TrimmedLength` | **一处登记差异**：xterm 判「有内容」是 `code !== 0`（未写过的格子没字），上游模拟器把没写过的格子填成空格字形（实测 `Content == " "`），所以这里退到「最后一个非空格格」——行尾真被打出来的空格也会被去掉 |
| 词选跨折行 | `Grid.WordAt` 在行边界收住 | **一处登记差异**：官方 `_getWordAt` 会用 `isWrapped` 把上一行/下一行接进来，本仓的 `Grid` 不带折行标志 |
| 指针状态机 | `plugins/dsterminal/selection.go`：`terminalSelection`，用 `canvas.PointerOn` + 双击/三击取 **`gfx.PointerEvent.ClickCount`** | 无（平台给的连击次数，Windows 由 `configureClicks` 按系统双击时间/距离算：`viva@v0.6.4/internal/platform/windows/window_windows.go:1137`） |
| 贴边滚动 | `advanceTerminalDragScroll` + `canvas.RequestFrameIn(50ms)` | 无（同一组数：50 / 50px / 14） |
| 选区层 | `paintTerminalSelection`：一行里同色相邻格合并成一块 `Fill`，画在格子底色之上、字形之下 | 无 |
| 底色/字色 | `terminalSelectionColors` = `blend(bg-base, label-primary, 0.3)` / `bg-base` | 无（xterm 的通道是 8 位整数，那句 `Math.round` 落到 Viva 的 0..1 浮点上变成不取整，差值 ≤ 1/255） |
| 复制载体 | **`app.SetClipboardText`**（`viva@v0.6.4/app/system.go:139`；Windows 实现 `internal/platform/windows/system_windows.go:317`；headless 走 `app/driver_system.go:79`） | 链路不同：官方是「Chromium 的 Copy 编辑命令 → xterm 的 `copy` 监听」，我们是直接调 Viva 的剪贴板 API。**Viva 有剪贴板能力**，不需要自建平台代码 |
| 复制的触发 | **右键有选区就复制** | **derived（登记）**：官方是「右键 → 原生编辑菜单 → 复制」三步；这条链路里两个环节本仓没有证据可实现——Viva 不向 `ui.C` 导出 Popup（`ui/page.go:72-119` 无此方法，见 `$DSH_HOME/outbox/vivaui-viva-popup-menu-shadow.md`），也没有原生菜单宿主。所以把「右键（准备 selectionText）→ 菜单里点复制」折叠成一次右键。**没有**绑 Ctrl+Shift+C 之类的自造快捷键 |
| 用户输入清选区 | `clearTerminalSelection`，挂在 F6 提交（本仓唯一人员输入口） | 挂点不同（xterm 挂原生键盘输入），语义相同 |

未做（**不是**这次的范围，已在账本 `#54` 里逐条登记）：鼠标上报（不向 PTY 发鼠标序列）、
`terminal.read` 之外的剪贴板相关动作。
