# 终端光标四形态 + 焦点闪烁 + 程序改色（OSC 4/104、10/11/12）—— rc.2 证据

> 对应账本 `#54`（`dsh-client-ui-sidebar-terminal`）的 VT 面。任务：task-95。
> 官方基线 `0.2.2-rc.2`；上游 `charmbracelet/x/vt v0.1.0`。
> 本文件只写**取证**：每条结论都能指到 `file:line` 或官方产物里的原文。

---

## 1. 光标：官方到底有几种形态

### 1.1 四个 CSS 类（官方 module CSS）

`OFF:packages/client/ui-sidebar-terminal/src/client/terminal.module.css:11-19`：

| 行 | 原文 | 形态 |
|---|---|---|
| `:12` | `.screen :global(.xterm-cursor) { opacity: 1; }` | 光标本身不透明 |
| `:13` | `.xterm-cursor.xterm-cursor-block { background-color: var(--terminal-cursor); color: var(--terminal-cursor-accent); }` | **block**：实心块，格里的字换 accent 色 |
| `:14` | `.xterm-cursor.xterm-cursor-block:not(.xterm-cursor-blink) { … !important }` | 不闪的块光标固定用上面那组色 |
| `:15` | `.xterm-cursor.xterm-cursor-outline { outline-color: var(--terminal-cursor); }` | **outline**：只有框色 |
| `:16` | `.xterm-cursor.xterm-cursor-bar { box-shadow: 1px 0 0 var(--terminal-cursor) inset; }` | **bar**：格内左侧 1px |
| `:17` | `.xterm-cursor.xterm-cursor-underline { border-bottom-color: var(--terminal-cursor); }` | **underline**：格子底边 |
| `:18` | `.xterm-rows.xterm-focus .xterm-cursor.xterm-cursor-block.xterm-cursor-blink { animation: terminal-cursor-blink 1s step-end infinite; }` | 闪烁**要 `.xterm-focus`**，且这条只管 block |
| `:19` | `@keyframes terminal-cursor-blink { 0% {…cursor…} 50% {…cursor-cell-*…} }` | 1s step-end：亮 → 回到格子自己的色 |

### 1.2 谁决定画哪一个：xterm 渲染器（官方 asar 原文）

`outline` **不是** DECSCUSR 的第四个档，它是 xterm 的 `cursorInactiveStyle`。
官方把 xterm 打进 `app.asar`，`DEFAULT_OPTIONS` 原文（`grep -a` 于
`D:/DeepSeek Harness/Desktop-0.2.2-rc.2/resources/app.asar`）：

```js
t.DEFAULT_OPTIONS={cols:80,rows:24,cursorBlink:!1,cursorStyle:`block`,cursorWidth:1,
                   cursorInactiveStyle:`outline`,customGlyphs:!0,drawBoldTextInBrightColors:!0,…}
```

官方组件**没有**覆盖这两项（`OFF:terminal.tsx:91-95` 只传
`minimumContrastRatio / cursorBlink / fontSize / fontFamily / scrollback`），
所以 `cursorInactiveStyle` 就是默认值 `outline`。

渲染器按焦点二选一（同 asar，`createRow` 里那段）：

```js
if (I.push(`xterm-cursor`), this._coreBrowserService.isFocused)
    l && I.push(`xterm-cursor-blink`),
    I.push(`bar`===s?`xterm-cursor-bar`:`underline`===s?`xterm-cursor-underline`:`xterm-cursor-block`);
else if (r) switch (r) {
    case `outline`: I.push(`xterm-cursor-outline`); break;
    case `block`:   I.push(`xterm-cursor-block`);   break;
    case `bar`:     I.push(`xterm-cursor-bar`);     break;
    case `underline`: … }
```

`s` = `cursorStyle`（DECSCUSR 改它）、`r` = `cursorInactiveStyle`、`l` = `cursorBlink`。
**`xterm-cursor-blink` 这个类只在 `isFocused` 分支里加**——这是「闪烁看焦点」的第一条证据。

### 1.3 几何：xterm 生成的那段主题 CSS（同 asar）

```js
@keyframes ${i} { 50% { border-bottom-style: hidden; }}                       // underline 闪
@keyframes ${s} { 50% { box-shadow: none; }}                                  // bar 闪
@keyframes ${r} { 0% { background-color: ${cursor}; color: ${cursorAccent}; }
                 50% { background-color: inherit; color: ${cursor}; }}        // block 闪
.xterm-rows.xterm-focus .xterm-cursor.xterm-cursor-blink.xterm-cursor-underline { animation: ${i} 1s step-end infinite; }
.xterm-rows.xterm-focus .xterm-cursor.xterm-cursor-blink.xterm-cursor-bar       { animation: ${s} 1s step-end infinite; }
.xterm-rows.xterm-focus .xterm-cursor.xterm-cursor-blink.xterm-cursor-block     { animation: ${r} 1s step-end infinite; }
.xterm-cursor.xterm-cursor-outline   { outline: 1px solid ${cursor}; outline-offset: -1px; }
.xterm-cursor.xterm-cursor-underline { border-bottom: 1px ${cursor}; … }
.xterm-cursor.xterm-cursor-bar       { box-shadow: ${cursorWidth}px 0 0 ${cursor} inset; … }
```

三条结论：
1. **1s step-end infinite**，三个形态都有（官方 module CSS 只是把 block 那条换成
   更准的 `--terminal-cursor-cell-*` 变量）；
2. **全部挂在 `.xterm-rows.xterm-focus` 下**——第二条独立证据，焦点不在就不闪；
3. `outline` 是 `1px solid` + `outline-offset:-1px`，即四条边**都在格子内沿**；
   `bar` 宽 = `cursorWidth`（默认 1）、`underline` 厚 1px。

「关」的那一相位官方画的是格子自己的颜色（`border-bottom-style:hidden` /
`box-shadow:none` / `background-color:inherit`），视觉等价于**不画光标**。

### 1.4 颜色与对比度（`terminal-cursor.ts`）

`OFF:terminal-cursor.ts:26-35`：光标色 `--terminal-cursor` 默认取前景色，与光标格
底色的 WCAG 对比度 **< 3:1** 时改黑或白；`--terminal-cursor-accent` 按光标色亮度取黑/白
（阈值 `0.179`）。`--terminal-cursor-cell-{background,foreground}` 是那一格自己的色。
这三条本仓早已实现（`plugins/dsterminal/grid.go` 的 `terminalCursorColors`）。

---

## 2. 本仓落点（A）

| 事 | 落点 |
|---|---|
| 第四形态常量 + 焦点二选一 | `internal/vterm/cell.go`：`CursorOutline`、`CursorInactiveShape`、`Cursor.ShapeAt`、`Cursor.BlinkAt` |
| 空心框几何 | `plugins/dsterminal/grid.go`：`terminalCursorOutlineWidth = 1`、`paintCursorOutline` |
| 焦点判据 | `plugins/dsterminal/grid.go`：`screenFocused`（`canvas.Focused() == 屏幕命中区`） |
| 命中区可聚焦 | `plugins/dsterminal/grid.go`：`rememberWheelTarget` 用 `RoleGroup + SkipTab` |

**为什么命中区要改成可聚焦**：官方 `.xterm-focus` 的来源是 xterm helper textarea 的
DOM 焦点，没有它就没有「失焦态」可言。本仓终端还没有原始键输入链路（`RoleTerminal`
会变成裸键盘、把用户按键吞掉），所以用 `RoleGroup + SkipTab`：**指针按下能拿焦点**
（viva 的 press 规则：命中区在指针下就接手焦点，`viva@v0.6.4/internal/interaction/focus.go:247-255`
`PointerDown` 的注释原文 "the hit under the pointer takes focus"），但**不占一个 Tab 站**。

---

## 3. 程序改色（B）：上游到底支不支持

### 3.1 判据

| 序列 | 语义 | `x/vt@v0.1.0` |
|---|---|---|
| OSC 4 `;i;c(;i;c)*` | 改 256 调色板第 i 项 | **无派发器** |
| OSC 104 `(;i)*` | 复位调色板 | **无派发器** |
| OSC 10/11/12 `;c…` | 默认前景/背景/光标色 | **有派发器** |
| OSC 110/111/112 | 复位上面三项 | **有派发器** |

- `handlers.go:304-337` 的 `registerDefaultOscHandlers` 只注册
  `0,1,2,7,8,10,11,12,110,111,112`——**4 与 104 不在名单里**，落到
  `osc.go:14-18` 的 `logf("unhandled sequence: OSC %q")`。
- 但调色板本身是**公开 API**：`emulator.go:422-446`
  （`IndexedColor(i)` / `SetIndexedColor(i,c)`，字段 `e.colors [256]color.Color`），
  `SafeEmulator` 各有带锁包装（`safe_emulator.go:113-127`）。
- `RegisterOscHandler(cmd, fn)`（`handlers.go:62-68`）是公开挂载点；派发**逆序**
  （`handlers.go:146-157`，后注册的先跑）。
- 10/11/12 的实现：`osc.go:52-104` 的 `handleDefaultColor` + `emulator.go:339-419`。

**结论**：4/104 是「**能力有、派发缺**」，不是真不支持；用上游 API 接上即可，
**不算兜底、不需要 issue 卡住我们**。

### 3.2 最小复现（`tmp/GO8-RC5/_curs/probe/main.go`，2026-11-19 实跑）

```
-- 1/3: 派发探测 --
  "\x1b]4;1;#ff0000\x1b\\" -> 4:"4;1;#ff0000"
  "\x1b]104\x1b\\"         -> 104:"104"
  "\x1b]10;#ff0000\x1b\\"  -> 10:"10;#ff0000"
-- 2: 不挂 handler 时 OSC 4 对 SGR 31 单元格的影响 --
  OSC 4 之前 SGR31 的 Fg = 0x1
  OSC 4 之后 SGR31 的 Fg = 0x1        ← 没变
  IndexedColor(1) = 0x1               ← 调色板没被改
-- 4: SetIndexedColor / IndexedColor --
  SetIndexedColor(1,#ff0000) 后 IndexedColor(1) = ansi.RGBColor{R:0xff, G:0x0, B:0x0}
-- 5: OSC 10/11/12 对上游默认色的影响（不挂 handler）--
  初始  fg=color.Gray16{Y:0xffff} bg=color.Gray16{Y:0x0} cursor=color.Gray16{Y:0xffff}
  之后  fg=colorful.Color{R:1,…} bg=colorful.Color{R:0.066,…} cursor=colorful.Color{R:0,G:1,B:0}
  reset fg=color.Gray16{Y:0xffff} …
  （defaultFg 与 reset 后相同 → 光看 getter 分不出「被改过」）
```

两条要点：
- **data 带命令号**：上游给 handler 的 `data` 是 `"4;1;#ff0000"` / `"104"`，
  与 xterm.js 的 `registerOscHandler`（给的是命令号之后的原文）不同，下标不能照抄；
- 上游 `ForegroundColor()` 复位后回的是 `defaultFg`，与全新模拟器一模一样——
  所以「有没有被程序改过」必须自己记一份（见 3.3）。

### 3.3 处置

1. **OSC 4 / 104**：`internal/vterm/osc.go` 用 `RegisterOscHandler(4/104)` 接上，
   调 `inner.Emulator.SetIndexedColor`（**不能用 `SafeEmulator` 的带锁版**：
   handler 在 `SafeEmulator.Write` 持写锁时被派发，二次加同一把 `sync.RWMutex`
   会死锁——实测测试挂到 298s 超时）。
   渲染层每次都问上游那张表（`cell.go` 的 `indexed`：`IndexedColor` 回
   `ansi.IndexedColor` = 没设置过 → 才落回官方 `DEFAULT_ANSI_COLORS` 静态表），
   **不存在第二份调色板副本**；OSC 104 复位就是把该项设回 `nil`。
2. **OSC 10/11/12 / 110/111/112**：上游自己派发，本仓只按官方
   `terminal-theme.ts:20-21` 的**观察者**形状（`registerOscHandler(…, data => { change(data); return false })`，
   返回 false 不吃掉）记一份 `vterm.Defaults`，再由 `grid.go` 画到
   默认前景 / 整屏背景 / 光标色。
3. **什么条件下能删**：上游在 `registerDefaultOscHandlers` 里自己派发 4/104 之后，
   删掉 `registerPaletteOSC` 那两个 handler 即可；10/11/12 的观察者可以一直留着
   （它回答的是上游 getter 回答不了的「改过没有」）。

### 3.4 建议给上游的 issue（**未发送**，等确认）

- 标题：`vt: dispatch OSC 4 / OSC 104 (palette set/reset) although SetIndexedColor exists`
- 要点：
  1. `Emulator.SetIndexedColor/IndexedColor` 已经是公开 API（`emulator.go:422-446`），
     但 `registerDefaultOscHandlers`（`handlers.go:304-337`）不派发 4/104，
     `\e]4;1;#ff0000\e\\` 只打一行 `unhandled sequence` 日志；
  2. 最小复现：`NewEmulator(20,3)` + 写上面那条序列 → `IndexedColor(1)` 仍是
     `ansi.IndexedColor(1)`；挂 `RegisterOscHandler(4, …)` 后同一句才生效；
  3. 期望：默认派发 4（成对 `index;color`，`?` 查询回 `OSC 4;i;rgb:…`）与 104
     （空参数复位全表），语义按 xterm ctlseqs；
  4. 影响：任何把 PTY 输出喂给 `x/vt` 的宿主，`ls --color` / `bat` / `vim` 主题
     之类依赖 OSC 4 的程序改色都不生效。

---

## 4. 测试与变异

测试：
- `internal/vterm/cell_test.go`：`TestCursorShapeAtFourShapes`、`TestCursorBlinkAtOnlyWhenFocused`、`TestCursorShapeStrings`
- `internal/vterm/osc_test.go`：`TestOSC4SetsIndexedColor`、`TestOSC4MultiplePairs`、`TestOSC4AffectsTruecolorCube`、`TestOSC4IgnoresQueryAndBadColor`、`TestOSC104ResetsPalette`、`TestOSC4BrightUsedByBold`、`TestOSCDefaultColors`、`TestOSCDefaultColorResets`、`TestOSC10WritesAllThreeDefaults`、`TestOSCDefaultsAbsentByDefault`
- `plugins/dsterminal/grid_test.go`：`TestGridCursorOutlineWhenUnfocused`、`TestGridCursorShapeFollowsFocus`、`TestGridCursorBlinkOnlyWhenFocused`、`TestGridProgramDefaultColors`
- `plugins/dsterminal/scrollback_test.go`：命中区断言改为「**必须可聚焦 + 必须 SkipTab + Role 仍是 group**」

变异（`tmp/GO8-RC5/_curs/mutate.sh`，每条都是「改坏 → 跑 → 必须红 → 还原」）：

```
OK=19 FAIL=0
[ShapeAt 忽略焦点] [BlinkAt 忽略焦点] [screenFocused 恒真] [screenFocused 恒假]
[光标只画三条边] [outline 宽度 2px] [outline 落到块光标分支]
[屏幕命中区不能聚焦] [屏幕命中区占 Tab 站]
[indexed 不问上游] [OSC 4 派发器摘掉] [OSC 104 复位不生效] [OSC 104 单条复位不生效]
[OSC 10/11/12 不观察] [OSC 110/111/112 不复位] [OSC 11 槽位错一档]
[默认色不参与绘制] [OSC 11 不铺整屏底色] [光标色不看 OSC 12]
```

其中 `outline 宽度 2px` 第一次**没杀掉**：期望值当时由 `terminalCursorOutlineWidth`
算出来，常量一改两遍——改成字面量 `1`（来自官方 `outline:1px`）之后才红。
这条记在这里，因为它正是「测试自己验自己」的实例。

---

## 5. 仍是 🟡 的剩余差异

- **selection**（只读文本选区）没有：官方 xterm 的选择层 + `onSelectionChange`。
- scrollback / rows 自适应仍由界面按窗口切（`vterm.Window`），不是 xterm 的 viewport。
- OSC 4 的**查询**（`\e]4;i;?\e\\`）不答：回话要写上游应答流，本包只做「看屏幕」，
  应答由壳丢弃；官方那边也是 xterm 回的，不在 `terminal-theme.ts` 的观察范围。
