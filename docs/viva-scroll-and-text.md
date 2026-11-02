# Viva 的横向滚动与文本能力（只读调研）

> 调研对象：`github.com/vivaui/viva@v0.2.3`，本机 `$(go env GOMODCACHE)/github.com/vivaui/viva@v0.2.3`
> 起因：E2 缩放实测发现 **700px 宽时代码块第 3 行被右边缘硬切、没有横向滚动条**。
> 结论先行：**`flex` 层确实只有纵向滚动，但 `components` 层有现成的横向滚动（`CodeDiff.ScrollBy`），而且 Viva 自带一个专为聊天设计的 Markdown 渲染器。**

---

## 1. flex 里的滚动：全部是纵向

把 `flex/scroll.go` 与 `flex/frame.go` 里所有 Scroll 相关的导出符号列全：

| 符号 | 位置 | 方向 | 说明 |
|---|---|---|---|
| `f.Scroll(offset float32) Box` | `flex/frame.go:413` | **纵向** | 注释原话：*"clips them and shows them **offset pixels up**"* |
| `f.ScrollArea(id ui.ID) Box` | `flex/scroll.go:181` | **纵向** | 带状态、自动响应滚轮 |
| `ScrollState.Max()` | `flex/scroll.go:83` | 纵向 | `Content - Viewport` |
| `ScrollState.AtEnd()` | `flex/scroll.go:86` | 纵向 | |
| `ScrollState.ScrollToEnd()` | `flex/scroll.go:91` | 纵向 | |
| `ScrollState.ScrollTo(offset)` | `flex/scroll.go:94` | 纵向 | 只有一个 float32 |
| `ScrollState.Wheel(event)` | `flex/scroll.go:119` | 纵向 | 滚轮事件 |
| `Box.Follow(on bool)` | `flex/scroll.go:195` | 纵向 | 贴底跟随（聊天用） |
| `Box.Clip()` | `flex/box.go:174` | — | **只有裁剪，没有位移** |

**`go doc github.com/vivaui/viva/flex` 全量扫过：没有任何 `ScrollX` / `Horizontal` / `dx` 的导出符号。**

### 三条替代方案

| # | 方案 | 可行性 | 改动量 | 坑 |
|---|---|---|---|---|
| **①** | 手动裁剪 + 位移（`Box.Clip()` + 自己算 X 偏移） | **需要自己实现** | 中（每行一个 Offset） | `Box` **没有** `Offset`/`Translate` 方法（`box.go` 里只有 `Clip`、`IconTransform`）。要给每行自己做 `PadSides` 或外层 `Row().Width(-dx)` 的负位移技巧，等于自己造一个滚动容器 |
| **②** | 文本自己折行（`MaxLines` / `Paragraph` / `Wrap`） | **立即可用** | **零** | 代码块折行后丢失缩进视觉，长命令行会很难读；但**不会溢出、不用滚动** |
| **③** | 用 `components` 层现成的横向滚动 | **可行，最优** | 小 | 见 §1.1 |

### 1.1 横向滚动其实存在于 components 层（这是关键发现）

`internal/componentdemo/code_diff.go` 里用了带横向滚动条的组件：

```go
codeDiffHorizontal = tree.ID(codeDiffUI, "horizontal-scrollbar")   // code_diff.go:13
...
VerticalScrollbarID: vertical, HorizontalScrollbarID: horizontal    // code_diff.go:72-73
```

`components.CodeDiff` 的方法（`go doc`）：

```
func (diff *CodeDiff) Scroll(event ui.ScrollEvent) ui.EventResult
func (diff *CodeDiff) ScrollBy(dx, dy float32)      ← 二维
func (diff *CodeDiff) ScrollTo(x, y float32)        ← 二维
```

**结论：横向滚动不是一个"框架缺失"，而是"`flex` 只做纵向，`components` 的复合组件自带二维"。**
要做普通代码块的横向滚动，得参考 `CodeDiff.ScrollBy` 的实现自己写（约 100–200 行），或者直接用它。

**还有一条更硬的证据**：`components` 里已经有一套完整的二维滚动几何类型（`go doc components.ScrollAreaGeometry`）：

```go
type ScrollAreaGeometry struct {
    Bounds, Viewport, Content         ui.Rect
    VerticalTrack, VerticalThumb      ui.Rect
    HorizontalTrack, HorizontalThumb  ui.Rect   ← 横向轨道与滑块都有
    Corner                            ui.Rect
    OffsetX                           float32   ← 横向偏移
}
```

`CodeDiffSnapshot` 的两个滚动条字段也是明文的：
```go
VerticalScrollbarID   ui.ID
HorizontalScrollbarID ui.ID
```

**也就是说：横向滚动的"轮子"在 `components` 里已经造好了，缺的只是"给普通代码块用"的那层封装。**

---

## 2. 现成的富文本 / Markdown / 代码组件：**全都有**

`components` 包里有 200+ 文件，与本项目相关的是这些：

| 能力 | 组件 | 位置 | 说明 |
|---|---|---|---|
| **Markdown 解析** | `markdown` **包** | `markdown/markdown.go`、`markdown/inline.go` | 独立包，**专为 chat 助手写的** |
| **Markdown 排版** | `components.MarkdownView` | `components/` | `Layout` / `Paint` / `Codes()` / `Link(i)` / `OffsetAt` / `SelectedText` |
| **代码 diff（带横向滚动条）** | `components.CodeDiff` | `components/` | 双栏、行对齐、可编辑右侧 |
| **代码编辑器** | `components.CodeEditor` | `components/` | 有 `TextareaLayout`、语法高亮 scratch |
| **多行文本框** | `f.Textarea(snapshot)` | `flex/frame.go:563` | 注意：**没有 `Submitted`**（要留 Enter 换行，`flex/field.go:73`） |
| **表格** | `f.Table(state, spec)` | `flex/table.go:153` | |
| **输入框组 / 分段** | `f.Input` / `f.InputGroup` / `f.Segmented` | `flex/frame.go:544/554/573` | |

### `markdown` 包的原话（`go doc`）

> *"Package markdown parses the Markdown that **chat assistants** and release notes are written in: headings, paragraphs, bulleted and numbered lists, block quotes, fenced code, rules and pipe tables, with bold, italic, strike, inline code and links inside them."*
>
> *"A newline inside a paragraph is a line break, as it is in chat. Text that is not closed (a `**` with no partner, a fence nobody ended yet) **stays as typed, so half a message that is still arriving reads sensibly**."*
>
> *"Links are kept only when they point to http, https or mailto. HTML, images, footnotes and indented code are not interpreted."*

**这两段几乎就是为我们写的**：为聊天、为流式、为"半条消息"设计。

### `components.MarkdownView` 的 API（`go doc`）

```
func (v *MarkdownView) Layout(canvas, style MarkdownStyle, doc *markdown.Document, width float32) float32
func (v *MarkdownView) Paint(canvas, p MarkdownPaint)
func (v *MarkdownView) Codes() []MarkdownCode      ← 拿每个代码块的位置，可以放「复制」按钮
func (v *MarkdownView) Link(i int) string
func (v *MarkdownView) Height() float32
func (v *MarkdownView) OffsetAt(point) (int, bool)  ← 可以做选中
func (v *MarkdownView) SelectedText(sel) string
func (v *MarkdownView) CaretBounds() (ui.Rect, bool)
```

原话：*"It keeps the arrays it builds, so laying out the next version of a message that is still being typed **allocates nothing once the view is warm**."*

**官方注释还直接承认了一个缺陷**：*"Proportional text has no italic face yet, so emphasis is drawn a weight heavier than the text around it."* —— 斜体暂时用加粗代替。

---

## 3. `f.Text` 的富文本能力：**只有整段级别，做不了"部分加粗"**

`Box` 上与文本有关的方法（全量，`grep` 源码）：

| 方法 | 位置 | 作用域 |
|---|---|---|
| `FontSize(v)` | `flex/box.go:179` | **整段** |
| `Weight(v)` | `flex/box.go:187` | 整段 |
| `Bold()` | `flex/box.go:192` | 整段（`WeightSemibold`） |
| `Color(c)` | `flex/box.go:193` | 整段 |
| `Muted()` | `flex/box.go:199` | 整段 |
| `Mono()` | `flex/box.go:200` | 整段 |
| `MonoFace(face)` | `flex/box.go:214` | 整段 |
| `LineHeight(v)` | `flex/box.go:191` | 整段 |
| `MaxLines(n)` | `flex/text_wrap.go:58` | 整段 |
| `Ellipsis()` | `flex/box.go:218` | 整段 |
| `TextCenter()` / `TextEnd()` / `TextDirection()` | `flex/box.go:224/230/236` | 整段 |

**没有任何 `Box` 级 API 能在"一段话里"切换样式。** `f.Text("a **b** c")` 只会把字面星号画出来。

### 那 Markdown 行内样式怎么做的？

靠 **`markdown` 包 + `components.MarkdownView`** —— 它在内部把一段拆成多个 run 再逐段画。`markdown/inline.go` 这个文件名本身就说明它专门处理行内。

**所以"一段话里部分加粗 / 部分是等宽"是能做到的，但路径不是 `f.Text`，是 `MarkdownView`。**

---

## 4. `RichText` / 按 run 渲染：底层有，但只给等宽网格

| 符号 | 位置 | 用途 |
|---|---|---|
| `ui.TextRun` | `ui/canvas.go:219` | 字段：`Column` / `Bytes` / `Cells` / `Color` / `Weight` / `Italic` / `Underline` / `Strike` / `Ligatures` |
| `ui.Text` | `ui/canvas.go:234` | 带 `RunFirst` / `RunCount` / `CellWidth` |
| `canvas.LabelRuns(value, bounds, style, cellWidth, runs)` | `ui/canvas.go:1146` | **唯一的 run 渲染入口** |
| `ui.TextSpan` | `ui/canvas.go:1092` | `{X, Width}` 命中测试用 |

官方注释限定死：*"A text with `RunCount > 0` is a **fixed-pitch grid line**"*。

**结论：`LabelRuns` 是"等宽网格 + 分段着色"，不是通用富文本。** 文件名 `label_runs` 而非 `richtext` 也印证了。想在比例字体里做任意 run，没有现成 API。

---

## 5. 给 U1 的取舍建议（不引入新依赖）

1. **消息正文直接上 `markdown` + `components.MarkdownView`，不要再自己写 Markdown 渲染。**
   它已经处理了行内粗体/行内代码/表格/围栏代码/未闭合标记，且"流式中每次重排零分配"。这一步能省掉我们原本估的 5,679 行（schemastery 那类）。

2. **代码块横向溢出：先折行，别急着做横向滚动。**
   `MaxLines` + `Paragraph` 零成本；等真的有人抱怨"代码看不清"再上二维滚动。理由是 `flex` 没有 `Box.Offset`，自己造二维滚动要 100–200 行且容易在缩放时出错（E2 已经证明这个方向有坑）。

3. **diff 视图直接用 `components.CodeDiff`，它自带 `HorizontalScrollbarID`。**
   这是唯一一个"横向滚动已经做好"的现成件，别重复造。

> 如果后面非要做普通代码块的横向滚动，抄 `CodeDiff.ScrollBy` 的 dx/dy 实现，不要从 `flex.Scroll` 下手——它是单轴的，改造它等于重写。
