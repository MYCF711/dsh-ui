# 代码块横向溢出：`mdlite.Overflow` 与实测

> 2026-11-03 · 起因：700px 宽时代码块第 3 行被右边缘硬切，没有横向滚动。

## 1. `components.CodeDiff` 是怎么做的（5 行以内）

1. `ScrollAreaGeometry` 里已有 `OffsetX / MaxOffsetX / HorizontalTrack / HorizontalThumb`，**几何是现成的**；
2. `BeginScrollArea(canvas, theme, bounds, contentSize).Offset(x,y)` 声明区域 → `BeginContent()` 推 `canvas.Clip(Viewport)` 并返回**已平移**的 `Content` 矩形 → 画内容 → `EndContent()` 弹 clip 并画两条滚动条；
3. CodeDiff 自己不实现这套，它只**持有 `scrollX/scrollY`**，在 `Paint` 里把 `code.X - OffsetX` 当原点画；
4. 拖拽是它自己接的：`horizontalDrag = event.Target == HorizontalScrollbarID && MaxOffsetX > 0`，按 `HorizontalOffsetForThumb(event.X - grab)` 换算；
5. **`components.ScrollArea` 只画不处理**——源码注释原话：「Scoped wheel routing, pointer capture, inertial scrolling, and scroll scheduling belong to the runtime and are intentionally not simulated here.」

## 2. `internal/mdlite/overflow.go` 做了什么

对 `components.ScrollArea` 的一层薄封装，把「Clip + 平移 + 两条滚动条」压成一个回调：

```go
slot := f.Slot().Height(150)              // flex 里占位
// Draw 返回之后布局已定：
o.Draw(canvas, theme, slot.Rect(), contentW, contentH, id, "代码块", func(c ui.Rect) {
    // c 已经按 OffsetX 平移过，直接从这里开始画
})
```

- `OffsetX` 由调用方持有（放 model 里），`Draw` 会**回写夹紧后的值**；
- `MaxOffset(bounds, contentW)` 可以在布局前问「会不会溢出」；
- **不处理滚轮和拖拽**——那部分按 CodeDiff 的做法在调用方接（约 100–200 行），本次没做，理由见 §4。

## 3. 实测（700×640，`spike/e2`）

| 截图 | offset | 现象 |
|---|---|---|
| `e2-overflow-zero.png` | **0** | 60 行代码从左边距开始，缩进完整；底部有横向滚动条 |
| `e2-overflow-offset.png` | **220** | 行内容整体左移，左侧缩进被裁掉，右侧能看到更远的 `// 尾巴故意` |
| `e2-overflow-end.png` | **99999** | 与 offset=220 **字节级完全相同**（`md5 b671ad3c…`）——说明被正确夹到 `MaxOffsetX`，没有越界 |

**三点结论**：

1. **溢出被检出**：横向滚动条出现 = `MaxOffsetX > 0`（内容比视口宽）；
2. **平移生效**：offset=0 与 offset=max 的截图 md5 不同（`4aa401dc…` vs `b671ad3c…`）；
3. **夹紧正确**：99999 与 220 渲染一致，说明 `Geometry` 把 offset 收进了 `[0, MaxOffsetX]`。

**行被拉长到 110+ 字符才触发溢出**：60 字符 × 7.2px ≈ 432px < 视口 620px，那时 `MaxOffsetX = 0`，滚动条不会出现——这是正确行为，不是 bug。

## 4. 结论：U1 该怎么排

| 场景 | 做法 | 理由 |
|---|---|---|
| **代码块** | 用 `mdlite.Overflow` + **不折行** | 折行会破坏代码的列对齐和缩进语义；横向滚动是编辑器的标准答案 |
| **diff** | 直接用 `components.CodeDiff` | 它是唯一「横向滚动 + 拖拽 + 高亮 + 行号」全都做好的件，别自己拼 |
| **普通正文** | `markdown` + `MarkdownView` | 它按宽度折行，本来就不需要横向滚动 |
| **滚轮/拖拽接线** | **还没做** | 需要拦 `Wheel` 事件 + 接 thumb 拖拽（照 CodeDiff 那 100–200 行）。U1 里再补，届时把 `Overflow` 扩成带 `Wheel`/`Drag` 的完整件 |

**一句话**：横向滚动在 flex 里做不到，但 `components.ScrollArea` 给了全套几何，**问题从「做不到」降级成「接线工作」**——而且只对代码块和 diff 需要，正文用 Markdown 折行即可。

## 5. 复现

```bash
cd "D:/DeepSeek Harness/default-workspace/dsh-ui/spike/e2"
viva build .
./build/e2.exe          # 默认 1100×860
# 用 PowerShell SetWindowPos 改成 700×640 再截图
```

`main.go` 里 `codeOv: mdlite.Overflow{OffsetX: N}` 可以改 N 看不同偏移下的渲染。
