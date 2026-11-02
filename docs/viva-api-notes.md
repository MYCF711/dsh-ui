# Viva API 调研（v0.2.3，只读源码）

> 源码：`$(go env GOMODCACHE)/github.com/vivaui/viva@v0.2.3`
> 本文所有 `文件:行号` 均指该目录下的相对路径。**未实测运行时行为，只读代码。**
> 结论后面标 `【未找到】` 的，是源码里确实没有对应 API，不要照着编。

---

## 先纠正一个前提

> 任务书说：「`flex.Field` 只有 `Text()` / `SetText()`，没有 `Submitted()`（生成的 AGENTS.md 里写了但 API 不存在）」

**这个说法不准确。`Submitted()` 存在，只是挂在 `Box` 上而不是 `Field` 上。**

| 事实 | 证据 |
|---|---|
| `func (b Box) Submitted() bool` | `flex/field.go:75` |
| 官方文档示例就是推荐写法 | `flex/field.go:71` |
| 有单测钉住语义 | `flex/field_test.go:73`（Enter 只投递一次、点击不算）、`field_test.go:143`（零分配） |

`flex.Field` 上确实只有 `Text()` / `SetText()` / `Secure`（`flex/field.go:22-48`）——**字段的状态在上面，事件的返回值在 Box 上**。这是 Viva 的一贯分工（`Clicked()` 也是 Box 的方法）。

---

## Q1 · 输入框按 Enter 怎么捕获

**结论：`f.Field(...)` 返回的 `Box` 上调 `.Submitted()`。**

```go
// 最小可用片段（照 flex/field.go:71 的官方示例）
if f.Field(&m.draft, draftID, "Message", "Type a message").Submitted() {
    m.send()
    m.draft.SetText("")
}
```

| 项 | 说明 | 证据 |
|---|---|---|
| 返回 true 的时机 | **只有投递 Enter 的那一次 paint**；点击、打字符、长按都不算 | `flex/field.go:66-69` |
| 单行才有 | `Textarea` 保留 Enter 换行，**没有 Submitted** | `flex/field.go:73-74` |
| 实现方式 | 内部走 `canvas.ClickedByKey(inputID)` | `flex/field.go:80` |
| 分配 | 零分配（有测试） | `flex/field_test.go:143-159` |
| 和 `Config.OnKey` 的关系 | `OnKey` 在 `app/config` 层（`app/app.go:125`），是全局键通道；**Enter 提交不该走它**，否则要在 OnKey 里自己判断焦点在哪个 field | `app/app.go:125` |

**注意**：`.Submitted()` 必须紧跟在声明这个 field 的那行——它是「这次 paint 的事件结果」，**不能先存到变量里下一帧再问**。

---

## Q2 · `f.Field` 返回的 `flex.Box` 有哪些方法

`Box` 一共 **108 个方法**（`go doc -all github.com/vivaui/viva/flex.Box` 数出来的）。跟**焦点 / 键盘 / 提交 / 状态**相关的只有下面这些：

| 方法 | 签名 | 作用 | 证据 |
|---|---|---|---|
| `Submitted` | `func () bool` | 本帧是否投递了 Enter | `flex/field.go:75` |
| `Changed` | `func () bool` | 内容/值自上次以来变过 | `flex/box.go`（`go doc` 可见） |
| `Clicked` | `func () bool` | 本帧是否被点击 | `flex/box.go` |
| `Dismissed` | `func () bool` | 弹层是否被关闭请求 | `flex/box.go` |
| `Hit` | `func (id ui.ID, label string) Box` | 给容器一个点击目标与无障碍名 | `flex/box.go` |
| `HitWith` | `func (id ui.ID, label string, options ui.HitOptions) Box` | 同上，可指定 Cursor/Selected/Disabled/Role | `flex/box.go` |
| `Label` | `func (v string) Box` | 无障碍名（屏幕阅读器与测试用它找控件） | `flex/box.go` |
| `Disabled` | `func (off bool) Box` | 禁用（不接事件） | `flex/box.go` |
| `Placeholder` | `func (value string) Box` | 占位文本（`Field` 已内建） | `flex/box.go` |
| `Follow` | `func (on bool) Box` | 滚动区跟随末尾（日志/聊天场景） | `flex/scroll.go:195` |

**`Box` 上没有的方法（重要，别找）**：
- **没有 `Focus()` / `Blur()`** —— 聚焦是**事件返回值**，不是 Box 方法（见 Q4）。
- 没有 `OnKey` / `OnEnter` —— 没有逐 Box 的键盘回调，键盘只有 `Config.OnKey` 一条全局通道（`app/app.go:125`）。
- `Secure` 是 `flex.Field` 的字段，不是 Box 的方法（`flex/field.go:24`）。

---

## Q3 · 怎么读 ScrollArea 的滚动位置

**结论：`view.ScrollState(id)` 拿到 `*ScrollState`，`Offset` 是公开字段，可读可写；`ScrollTo` 是带边界钳制的设定器。**

```go
// 读
st := m.view.ScrollState(listID)
offset := st.Offset      // float32，像素
atEnd := st.AtEnd()

// 恢复
m.view.ScrollState(listID).ScrollTo(savedOffset)
```

| 项 | 说明 | 证据 |
|---|---|---|
| 拿状态 | `func (view *View) ScrollState(id ui.ID) *ScrollState` | `flex/scroll.go:80` |
| 偏移字段 | `Offset float32` —— **导出的结构体字段** | `flex/scroll.go:24` |
| 内容高度 | `Max() float32` = `Content - Viewport` | `flex/scroll.go:83` |
| 是否到底 | `AtEnd() bool`（容差 0.5 px） | `flex/scroll.go:86` |
| 跳到末尾 | `ScrollToEnd()` | `flex/scroll.go:91` |
| **带钳制地跳** | `ScrollTo(offset float32)`，下一次 layout 会把它夹回 `[0, Max]` | `flex/scroll.go:94-98` |
| 跟随模式 | `Box.Follow(true)`，日志/流式场景自动贴底 | `flex/scroll.go:195`、`flex/doc.go:59` |

**两个坑（源码注释明说的）**：

1. **不要长期持有 `*ScrollState`**——`flex/scroll.go:15-18` 原话：*"ask for it when you need it and do not keep (its offset starts again at the top when the area comes back, as for a closed dialog)"*。
   → **会话切换时存的是 `Offset` 这个 float32，不是指针。**
2. `ScrollArea` 的状态跟着 `id` 走（`flex/scroll.go:174-175`：*offset the View keeps under id*），所以**每个会话要用不同的 `ui.ID`**，否则切回来读到的是另一个会话的偏移。

**直接赋值 `st.Offset = x` 能不能用？** 结构体字段可写，但它绕过 `toEnd/detached` 的簿记（`flex/scroll.go:101-112`），**建议一律用 `ScrollTo`**。

---

## Q4 · 怎么让某个控件获得焦点

**结论：没有「设置焦点」的命令式 API；焦点是 `ui.EventResult.Focus` 的返回值。**

```go
// 在任意返回 ui.EventResult 的回调里（OnKey / OnPointer / OnScroll / OnFileDrop）
return ui.EventResult{Handled: true, Repaint: true, Focus: draftID, FocusVisible: true}
```

| 项 | 说明 | 证据 |
|---|---|---|
| 设置焦点 | `EventResult.Focus ui.ID` | `ui/input.go:252` |
| 配套 | `FocusVisible bool`（是否画焦点环）、`Blur bool`（清焦点）、`ContinueTraversal bool`（Tab 继续走） | `ui/input.go:255-261` |
| 读当前焦点 | `canvas.Focused() ui.ID` | `ui/canvas.go:1473`；用法见 `flex/field.go:57` |
| 真实用例 | 下拉搜索选中一项后把焦点交回去 | `flex/select.go:113` |
| 真实用例 | 表格行点击后聚焦该行 | `flex/table.go:135` |
| 对话框打开时聚焦 | `flex.DialogSpec.InitialFocusID` | `flex/dialog.go:21-22`、`dialog.go:130` |

### 「窗口打开时自动聚焦 draft」怎么实现

**【未找到】直接的启动聚焦 API。** 源码里没有 `Config.InitialFocus` 这类字段（`app/app.go:46` 起整个 `Config` 结构我都过了一遍，没有）。

可行的三条路，按推荐顺序：

1. **首帧后用一次性标志 + 一次 `Invalidator` 触发的事件**——最省事。在 `OnWake` 里把 `m.needInitialFocus` 置位，然后在**第一次** `OnKey`/`OnPointer`（用户随便动一下就触发）时返回 `Focus: draftID`。缺点是用户不动键盘就永不聚焦。
2. **把它做成对话框**：用 `flex.DialogSpec{InitialFocusID: draftID}`（`flex/dialog.go:21`）——如果输入框本来就要在弹层里，这条最正统。
3. **宿主层补**：壳（`cmd/dshui`）在窗口激活后合成一个事件喂给 `OnKey`——脏，但确定能做。

**建议**：先按 1 做，把「启动即聚焦」列为已知缺口；如果真卡体验，再评估 2。

---

## Q5 · 后台数据怎么到达界面（`Invalidator` 之外）

| # | 机制 | 怎么用 | 适用场景 | 证据 |
|---|---|---|---|---|
| 1 | **`Invalidator` + `OnWake`** | 后台 goroutine 存好数据 → `inv.Invalidate()` → 窗口线程先跑 `OnWake()` 再 paint | **网络/WS 数据到达**（我们的 session/prompt 流式） | `app/app.go:186`、`app/app.go:190`、`app/invalidator.go:11-33` |
| 2 | **输入事件天然重绘** | `OnPointer` / `OnKey` / `OnScroll` / `OnFileDrop` 每处理一个就重绘 | 用户交互 | `app/app.go:122/125/136/143` |
| 3 | **窗口事件** | `OnResize`（内容尺寸变）、`OnFrameChange`（位置/尺寸）、`OnMinimize`、`OnMaximize` | 布局重算（终端行列数、pty 尺寸） | `app/app.go:150/159/165/171` |
| 4 | **动画（`ui.Motion`）** | `ui.Motion` / `Easing` / `Spring` / `CubicBezier` / `Lerp` | 入场出场、状态变化曲线 | `ui/motion.go:32/39/93/125/150/174` |
| 5 | **`Snapshot` / `Restore`** | 状态槽的存/取 | 重启恢复，**不是推进重绘** | `app/app.go:206-207` |

**关键事实：`Invalidator` 本身不拥有 goroutine / timer / channel**（`app/invalidator.go:8-10` 原话）。所以：

- **没有内置的 tick**。要做「每 100ms 检查一次」得自己起 `time.Ticker`，然后在回调里 `Invalidate()`。
- **`OnWake` 在每次重绘前跑**（`app/invalidator.go:30-37` 的 `wakeHandler`：先 `onWake()` 再 `repaint()`），所以在 `OnWake` 里把后台写进缓冲区的数据搬进 model 是**正确**的落点。
- **没有空闲帧循环**——不 Invalid 就不画（这也解释了 E3 量到的静置 CPU 0.0%）。

---

## 给 `cmd/app/main.go` 的直接建议

1. **Enter 提交就用 `.Submitted()`，别绕 `Config.OnKey`。**
   ```go
   if f.Field(&m.draft, draftID, "Message", "输入…").Submitted() { m.send(); m.draft.SetText("") }
   ```
   单行 field 才免费拿到 Enter；如果以后要「Shift+Enter 换行」，就得换成 `Textarea` 并自己在 `OnKey` 里判 —— 那时 `Submitted` 就没了。

2. **滚动位置按会话存 `float32`，不存指针。**
   ```go
   m.scrollOf[sessionID] = m.view.ScrollState(listID).Offset   // 切走时
   m.view.ScrollState(listID).ScrollTo(m.scrollOf[sessionID])  // 切回来
   ```
   并且**每个会话用不同的 `ui.ID`**，否则 `ScrollArea` 的 offset 会串（`flex/scroll.go:174-175`）。`ScrollState` 指针不要跨帧持有（`flex/scroll.go:15-18`）。

3. **流式输出期间 `Follow(true)`，用户手动上滚就 `Follow(false)`。**
   `flex/scroll.go:195` 的 Follow 和 `AtEnd()`（`scroll.go:86`）配合，就是"贴底但不打断用户回看"的标准做法。

4. **后台数据只走一个入口：`Invalidator.Invalidate()` + `OnWake` 里搬数据。**
   不要在 goroutine 里直接改 model（`Paint` 只读 state 的约定会被破坏）。`Invalidator` 零值时可用、且不拥有任何 goroutine（`app/invalidator.go:8-13`）。

5. **启动自动聚焦目前没有官方 API**（Q4）。先用「一次性标志 + 第一个输入事件」的写法顶上，并在 `docs/` 里记成已知缺口；不要把 `EventResult.Focus` 硬塞进 `Paint`——`Paint` 没有返回值。

---

## 实测补充（2026-11-03 08:10，由父代理验证）

`components.MarkdownView` + `markdown.Document` **在本机真机跑通**（`spike/md`，4.02 MB exe）：

| 元素 | 结果 |
|---|---|
| 标题（`###`） | ✅ 放大 + 加粗 |
| 粗体 / 斜体 / 行内代码 / 链接 | ✅ 行内代码带底色，链接蓝色 |
| 无序列表 | ✅ 项目符号 |
| 引用（`>`） | ✅ 左侧竖线 |
| 围栏代码块 | ✅ 等宽 + 背景 + 圆角 |
| 表格 | ✅ 表头加粗 + 行线 |
| **未闭合的 `**`** | ✅ 原样显示（流式关键） |

集成方式（在 `flex` 里）：
```go
w := f.Bounds().Width - 300
h := mdView.Layout(canvas, markdownStyle, &doc, w)
slot = f.Slot().Height(h)      // 占位，拿 Rect
// Draw 返回之后：
r := slot.Rect()
mdView.Paint(canvas, components.MarkdownPaint{Look: look, X: r.X, Y: r.Y})
```

**结论：`flex` 的声明式布局 + `MarkdownView` 的命令式绘制可以共存**，`Box.Rect()`（`flex/box.go:30`）是桥。
