# 超宽代码块：结论是「不硬切，也不会横滚」——清单 3.3 那条是误判

> 2026-11-03 11:50 · 实测环境 `cmd/app -demo`（注入了一条 129 字符单行代码的消息）· 截图 `docs/u1-codeblock.png`

## 一、先纠正一个前提

`docs/u1-checklist.md` 的 3.3 写的是「60+ 字符单行代码块会被横向硬切」。**这条不成立。**

实测（`docs/u1-codeblock.png`）：129 字符那行被 **折成两行**，不是被裁掉。

```
func veryLongFunctionName(alpha int, beta string, gamma float64, delta []byte, epsilon map[string]any) (int
, error) {
	return alpha, nil
}
```

`markdown` 包的代码块本来就是**折行**渲染的。所以"不再硬切"这个验收目标，**在改动之前就已经满足了**。

## 二、方案 A（Fill + 重画）做了什么，为什么放弃

思路：`MarkdownView.Codes()` 能拿到每个代码块的位置和原文，那就
**先用卡片底色盖掉它画的，再用 `mdlite.Overflow` 重画一遍**，这样就有横向滚动了。

代码写出来了，也编译过了，但**结果不能用**：

| 步骤 | 结果 |
|---|---|
| `Codes()` 拿 bounds | ✅ `{X:0 Y:31 Width:880 Height:80}`，3 行、138 字符，数据是对的 |
| `canvas.Fill(cr, theme.Background)` 盖住 | ❌ **盖不住** |
| `Overflow.Draw` + `canvas.Label` 重画 | ✅ 画出来了，滚动条也在 |
| 最终显示 | ❌ **两层文字叠在一起** |

**根因**：`MarkdownView.Paint` 和我的 `Fill`/`Label` 在**同一个 canvas 上**，但绘制顺序不受调用方控制——
我 Fill 之后它又画了一遍（调试时把 Fill 染成蓝色，蓝色区域里同时有我的文字和它的文字，清楚看到两层）。

**试过的三个补救都对不上**：
- 按 `code.Bounds.Height` 算高度 → 它报 80，我按行数算 64，取 max 后 80，**仍不够**
- 把 Fill 提前 → 更糟（整块被它覆盖）
- 不用 Fill 只画滚动条 → 文字照样重叠

## 三、方案 B 为什么也不合适

把整条消息卡套 `mdlite.Overflow`（横向）：消息里的**普通段落会跟着横滚**，
而正文本来是该折行的——为了几十行代码牺牲所有正文的阅读方式，不划算。

## 四、所以现在的状态

| 项 | 状态 |
|---|---|
| 超宽代码行 | **折行显示**，不丢内容 |
| 横向滚动 | 没有 |
| 代价 | 折行处代码列对不齐（`(int` / `, error) {` 这种断法影响阅读） |

**这不是 bug，是 Viva 的渲染边界**：一旦交给 `MarkdownView.Paint`，代码块的画法就不归调用方管了。

## 五、真要做横向滚动，只有一条路

**自己画 Markdown**：用 `markdown` 包解析（`doc.Parse` + 遍历 span），代码块自己用
`mdlite.Overflow` + `canvas.Label` 渲染，正文用 `canvas.LabelRuns`。

成本：`MarkdownView` 的 `Layout`/`Paint` 约 400 行（含行内样式拆 run、链接命中区、表格、
引用、列表缩进）。**为几十行代码重写整个渲染器，成本远大于收益。**

**建议**：接受折行。如果以后真要改，优先级应该排在「U0 直连」（真实宿主接入）之后。

## 六、这一轮真正的产出

- 确认了 `MarkdownView` 对超宽行**折行不硬切**（清单 3.3 那条要改）
- 确认了 `Codes()` 的 bounds 数据可用（将来真做横滚时能直接用）
- 确认了**在 MarkdownView 上叠画这条路走不通**（Fill 盖不住），下次不用再试
