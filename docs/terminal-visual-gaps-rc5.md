# 终端视觉三处缺口 · faint / 对比度 / scrollback

> 2026-11-19。承接 dsh-ui#171（`plugins/dsterminal/grid.go` 网格渲染）。
> 官方根：`DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`。
> 下文 `OFFT` = `dsh-client-ui-sidebar-terminal`。xterm 是打进官方包里的
> bundle（`OFFT/lib/client.terminal.js`），官方树里没有独立的 `@xterm/xterm`
> 源码包，所以 xterm 内部的定位一律给**字节偏移**（`grep -b` 可复现）。

## A. faint（SGR 2）—— 做了

官方不是给整行加 opacity，而是**把前景色下成半透明**：

- `OFFT/lib/client.terminal.js` byte 491841 那一段是内联的 xterm.css，
  里面有 `.xterm-dim{opacity:1!important}` —— 显式关掉 opacity 方案；
- byte 76534：`` `${selector} .xterm-dim { color: ${multiplyOpacity(foreground,.5).css} }` ``；
- byte 78917：`` `${selector} .xterm-fg-N.xterm-dim { color: ${multiplyOpacity(ansi[N],.5).css} }` ``；
- byte 79214：反显默认色那一档同款。

所以官方值只有两个：**`0.5`** 与「作用在最终前景色上」。次序也与 xterm 一致：
先反显换色、粗体先按 `drawBoldTextInBrightColors` 提亮（byte 88698
`drawBoldTextInBrightColors&&($+=8)`，本仓在 `internal/vterm/cell.go:215-220` 已实现），
最后才降 alpha。

实现：`plugins/dsterminal/grid.go` 的 `terminalDimAlpha` 与 `paintOf`。
Viva 的 `gfx.Color` 带 Alpha（`viva@v0.6.4/gfx/canvas.go:14-19`，`ui.Color` 是它的
别名 `ui/vocab.go:10`），Windows 文本路径逐 run 取用
（`internal/platform/windows/direct_grid_windows.go:44` → `renderer_windows.go:687`），
底色由 `paintGridBackground` 先画好，与官方「浏览器把 rgba(fg,.5) 合成到格子底色」
是同一条链，因此**不需要我们自己算混色**。

测试：`plugins/dsterminal/grid_test.go` 的 `TestGridFaintDimsForeground`
（普通 / faint / faint+索引红 / faint+粗体红 / faint+反显 五段，逐段比色）。
断言里写的是字面量 `0.5` 而不是那个常量——引用常量的断言会跟着常量一起漂。

## B. `minimumContrastRatio: 4.5` —— 不做（不属于我们）

- 官方只做了一件事：`OFFT/src/client/terminal.tsx:103` 把 `4.5` 传给
  `new Terminal({…})`，官方包内**没有任何**对比度计算代码
  （`grep -r minimumContrastRatio` 只命中 `terminal.tsx` 与 bundle 自身）。
- 生效点在 xterm 的 DOM 渲染器里：`_applyMinimumContrast`（byte 89279）→
  `color.ensureContrastRatio`（byte 142427，内部走 `relativeLuminance` 的相对亮度，
  即 WCAG 那套）。它按**颜色模式**分三支调用（byte 88570 起的 `createRow`），
  对 `treatGlyphAsBackgroundColor` 的方块字符跳过，并且在 dim 格上把门槛
  除以 2（byte 89586 `minimumContrastRatio/(s.isDim()?2:1)`，配合
  `halfContrastCache`）。
- 结论：这是一条 **xterm 自带的渲染特性 + 官方只拨了开关**。照抄一个近似实现
  等于把 xterm vendored 的 `BlendForContrast` 常数搬进我们的渲染器——那些数字
  既不在官方视觉规格里，也不在 WCAG 里（WCAG 只规定「对比度怎么算」，不规定
  「不足时往哪个方向调多少」）。按硬边界「不自造数值/颜色」，这条不做。
- 什么时候该重开：只有当我们不再自己画网格、而是真嵌 xterm 时；或者用户明确
  要求「低对比色也要看得清」，那时应作为**设计决定**登记，而不是当成对官方的复刻。

## C. scrollback —— 模型层 + 渲染都已做（2026-11-19 补齐）

官方这边（全部是事实，不是推测）：

- 滚动缓冲来自宿主：`terminal.tsx:104` `scrollback: state.environment?.scrollback ?? 0`。
  **默认 0**，即宿主不给就等于没有滚回。我们打的宿主给真值
  （`DSH-GO/cmd/dshd/terminal_pty.go:723-731`，缺省 1000 行）。
- 呈现是 xterm 自己的 viewport：`.xterm-viewport{…overflow-y:scroll}`（byte 490908），
  滚轮 `scrollSensitivity:1`（DEFAULT_OPTIONS，byte 277845），官方 UI 侧**没有**一行
  滚动缓冲代码。
- **`SerializeAddon` 在官方包里根本不存在**（`grep -c SerializeAddon` = 0），
  所以「SerializeAddon 的 scrollback:0」这个前提不成立：官方没有序列化附件，
  也没有 `scrollback: 0` 这样的调用。
- 一条容易漏的几何事实：FitAddon 在 `scrollback != 0` 时**预留 14px** 给滚动条，
  再从可用宽度算列数（`proposeDimensions`，byte 489675：
  `t = scrollback===0 ? 0 : overviewRuler?.width || 14`，随后 `width - padding - t`）。
  即官方有滚回时 cols 比 `terminalFitSize` 少 1~2 列。**这条已经接上**：扣宽与
  画滚动条现在出自同一个 `terminalScrollbarGutter()`（0 或 14），见下面的接线。

上游能力（`github.com/charmbracelet/x/vt@v0.1.0`）：

- API 齐全且并发安全：`SafeEmulator.Scrollback/ScrollbackLen/ScrollbackCellAt/SetScrollbackSize`
  （`safe_emulator.go:200-236`、`emulator.go:459-494`）。
- **正常换行滚屏就会填充**：`cc.go:29` 那句 `// XXX: Handle scrollback whenever we add it.`
  是过期注释，真正落盘在 `Screen.ScrollUp` → `Screen.DeleteLine` 的 `scrollback.PushN`
  （`screen.go:297-303`、`screen.go:359-366`）。探针（20 行写进 5 行屏）读到 16 行。

本单做了：`internal/vterm/cell.go` 新增 `Emulator.ScrollbackLen()` 与
`Emulator.Scrollback() *Grid`——滚回行的投影与 `Grid` 完全相同（同一套 `Cell`/`Style`，
颜色/反显/下划线/全角宽度同一份代码），行 0 最老、补格为零值、Cursor 不可见。
测试 `internal/vterm/cell_test.go` 的 `TestScrollbackKeepsScrolledOffLines` /
`TestScrollbackEmptyWithoutScroll`。

### 渲染接线（2026-11-19 补齐）

**先测成本，再定接口形状**（实测：`bench` 用真模拟器，90 列 × 1000 行滚回）：

| 取法 | 格数 | 单次耗时 | 每次的 Cells 切片 |
|---|---|---|---|
| `Scrollback()` 整块 | 90 000 | **1.725 ms** | ≈ 3.5 MiB |
| `Window(first, 24)` 可见窗口 | 2 160 | **0.041 ms**（41.9×） | ≈ 84 KiB |
| `ScrollbackLen()` | — | **0.0001 ms** | — |

所以接口按「长度每帧、窗口按需」切成两半，而不是往 `TerminalViewState` 塞一整块
`Scrollback *vterm.Grid`：

- `pluginapi.TerminalViewState.ScrollbackLen`：每帧带（一次读锁）。滚动条比例与
  「还能不能再往上滚」都靠它。
- `pluginapi.Actions.TerminalScrollbackWindow(tabID, first, lines) *vterm.Grid`：
  只在**用户往上滚了**的那一帧调，壳按坐标切一段（`internal/vterm` 的
  `Emulator.Window`，跨滚回与当前屏、光标恒不可见）。
- 容量按官方 `scrollback` 选项：`Emulator.SetScrollbackSize(environment.scrollback)`
  （上游默认 10000 行，比官方多一个数量级；`<=0` 由壳层投影成 0 行 = 官方
  「没有滚回」的语义）。
- 交互：`plugins/dsterminal` 实现 `pluginapi.ScrollHandler`，滚轮换算逐条照抄
  xterm `_getLinesFromWheelEvent`（`scrollSensitivity:1`、Alt/Ctrl/Shift ×5、
  像素 ÷ 格高、|Δ|<50 时 ×0.3 的小增量阻尼、小数余量跨事件累积）；滚到底恢复
  跟随（官方 `ydisp==ybase`），`scrollOnUserInput` 让 F6 输入回到贴底。
- 滚动条：`ScrollbarState._computeValues` 的几何（最小滑块 20px）+ 官方颜色
  `opacity(foreground,.2)`，画在 fit 已经扣掉的那 14px 槽里。

测试：`internal/vterm/scrollback_test.go`（窗口坐标/夹取/光标/容量/resize 不丢）、
`cmd/app/terminal_scrollback_test.go`（快照带长度、窗口动作、宿主容量真的生效）、
`plugins/dsterminal/scrollback_test.go`（14px 与 fit、滚轮换算、滚动条几何、
往上滚看到历史行、滚到底回到当前屏、resize 后仍停在历史）。
