# `#18` 图片面：真窗口复核（2026-11-20）

`docs/official-parity.md` 第 18 行长期挂着一条未验项：「headless 截图里该图元没有合成到
surface」。本文把它结掉。

## 结论

**图真的上屏了。** 旧登记是**测试假象** —— 既不是本仓缺陷，也不是 Viva 缺陷。

## 真窗口证据

独立窗口程序（`app.RunCanvas`，900×600）经 `scripts/shot.py` 截得 902×632：

| 图 | 内容 |
|---|---|
| [真窗口：带图片](evidence/message-image-real-window.png) | 用户消息「看这张图」+ 图片矩形 |
| [真窗口：不传附件（对照）](evidence/message-image-real-window-control.png) | 同一位置无图片 |

`scripts/visual-diff.py` 量出的数字：

```
图片矩形       [637, 97, 240, 180]
主色占比       1.0000（43200 px = 240 × 180）
与对照差异     100%
同图行上/左/下  0 像素差
```

## 假象的来源

旧夹具是渐变：

```go
R: uint8(x % 256), G: uint8(y % 256)
```

400×300 缩到 240×180 后，输出像素**全是邻域混合** —— 43200 个里只有 360 个（0.83%）
命中任何源色。**「数同色像素」在这个夹具上永远数不出全覆盖**，于是「整屏 0 像素」
是必然结果，与渲染无关。

实测那条 op 本身是健康的：

```
Bounds = {628.50, 66.00, 240.00, 180.00}
ClipOf = {628.00, 66.00, 240.00, 180.00}
HasClip = true   Layer = 0   Order = 101
headless 像素 = 43020 / 43200
```

`internal/framebuild/framebuild.go:342` 的三条丢弃路径一条都不成立。差的 180 像素是
`X=628.5` 与 `clip 628.0` 的**半像素列**。

## 顺带补的两条判据

1. **非周期两色夹具**（左半红 / 右半蓝）+ 「沿中线颜色翻转次数恰好 1」——
   纯色夹具检测不出「被平铺」，因为平铺纯色与画一次的像素统计完全相同。
2. **clip 账**：`HasClip` 为真时断言必与 `Bounds` 相交 —— Viva 的 `appendImage` 会
   **静默丢弃**不相交的 op，这类缺陷表面是「图没了」，根因在 clip。

详见 `plugins/dsconversation/official_message_image_headless_test.go`。
